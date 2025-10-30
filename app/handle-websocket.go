package app

import (
	"context"
	"crypto/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/encoders/envelopes/authenvelope"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/utils/units"
)

const (
	DefaultWriteWait      = 10 * time.Second
	DefaultPongWait       = 60 * time.Second
	DefaultPingWait       = DefaultPongWait / 2
	DefaultWriteTimeout   = 3 * time.Second
	DefaultMaxMessageSize = 100 * units.Mb
	// ClientMessageSizeLimit is the maximum message size that clients can handle
	// This is set to 100MB to allow large messages
	ClientMessageSizeLimit = 100 * 1024 * 1024 // 100MB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for proxy compatibility
	},
}

func (s *Server) HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	remote := GetRemoteFromReq(r)

	// Log comprehensive proxy information for debugging
	LogProxyInfo(r, "WebSocket connection from "+remote)
	if len(s.Config.IPWhitelist) > 0 {
		for _, ip := range s.Config.IPWhitelist {
			log.T.F("checking IP whitelist: %s", ip)
			if strings.HasPrefix(remote, ip) {
				log.T.F("IP whitelisted %s", remote)
				goto whitelist
			}
		}
		log.T.F("IP not whitelisted: %s", remote)
		return
	}
whitelist:
	// Create an independent context for this connection
	// This context will be cancelled when the connection closes or server shuts down
	ctx, cancel := context.WithCancel(s.Ctx)
	defer cancel()
	var err error
	var conn *websocket.Conn

	// Configure upgrader for this connection
	upgrader.ReadBufferSize = int(DefaultMaxMessageSize)
	upgrader.WriteBufferSize = int(DefaultMaxMessageSize)

	if conn, err = upgrader.Upgrade(w, r, nil); chk.E(err) {
		log.E.F("websocket accept failed from %s: %v", remote, err)
		return
	}
	log.T.F("websocket accepted from %s path=%s", remote, r.URL.String())

	// Set read limit immediately after connection is established
	conn.SetReadLimit(DefaultMaxMessageSize)
	log.D.F("set read limit to %d bytes (%d MB) for %s", DefaultMaxMessageSize, DefaultMaxMessageSize/units.Mb, remote)
	defer conn.Close()
	listener := &Listener{
		ctx:       ctx,
		Server:    s,
		conn:      conn,
		remote:    remote,
		req:       r,
		startTime: time.Now(),
	}

	// Check for blacklisted IPs
	listener.isBlacklisted = s.isIPBlacklisted(remote)
	if listener.isBlacklisted {
		log.W.F("detected blacklisted IP %s, marking connection for timeout", remote)
		listener.blacklistTimeout = time.Now().Add(time.Minute) // Timeout after 1 minute
	}
	chal := make([]byte, 32)
	rand.Read(chal)
	listener.challenge.Store([]byte(hex.Enc(chal)))
	if s.Config.ACLMode != "none" {
		log.D.F("sending AUTH challenge to %s", remote)
		if err = authenvelope.NewChallengeWith(listener.challenge.Load()).
			Write(listener); chk.E(err) {
			log.E.F("failed to send AUTH challenge to %s: %v", remote, err)
			return
		}
		log.D.F("AUTH challenge sent successfully to %s", remote)
	}
	ticker := time.NewTicker(DefaultPingWait)
	// Set pong handler
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(DefaultPongWait))
		return nil
	})
	// Set ping handler
	conn.SetPingHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(DefaultPongWait))
		return conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(DefaultWriteTimeout))
	})
	// Don't pass cancel to Pinger - it should not be able to cancel the connection context
	go s.Pinger(ctx, conn, ticker)
	defer func() {
		log.D.F("closing websocket connection from %s", remote)

		// Cancel context and stop pinger
		cancel()
		ticker.Stop()

		// Cancel all subscriptions for this connection
		log.D.F("cancelling subscriptions for %s", remote)
		listener.publishers.Receive(&W{
			Cancel: true,
			Conn:   listener.conn,
			remote: listener.remote,
		})

		// Log detailed connection statistics
		dur := time.Since(listener.startTime)
		log.D.F(
			"ws connection closed %s: msgs=%d, REQs=%d, EVENTs=%d, duration=%v",
			remote, listener.msgCount, listener.reqCount, listener.eventCount,
			dur,
		)

		// Log any remaining connection state
		if listener.authedPubkey.Load() != nil {
			log.D.F("ws connection %s was authenticated", remote)
		} else {
			log.D.F("ws connection %s was not authenticated", remote)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check if blacklisted connection has timed out
		if listener.isBlacklisted && time.Now().After(listener.blacklistTimeout) {
			log.W.F("blacklisted IP %s timeout reached, closing connection", remote)
			return
		}

		var typ int
		var msg []byte
		log.T.F("waiting for message from %s", remote)

		// Set read deadline for context cancellation
		deadline := time.Now().Add(DefaultPongWait)
		if ctx.Err() != nil {
			return
		}
		conn.SetReadDeadline(deadline)

		// Block waiting for message; rely on pings and context cancellation to detect dead peers
		typ, msg, err = conn.ReadMessage()

		if err != nil {
			// Check if the error is due to context cancellation
			if err == context.Canceled || strings.Contains(err.Error(), "context canceled") {
				log.T.F("connection from %s cancelled (context done): %v", remote, err)
				return
			}
			if strings.Contains(
				err.Error(), "use of closed network connection",
			) {
				return
			}
			// Handle EOF errors gracefully - these occur when client closes connection
			// or sends incomplete/malformed WebSocket frames
			if strings.Contains(err.Error(), "EOF") ||
				strings.Contains(err.Error(), "failed to read frame header") {
				log.T.F("connection from %s closed: %v", remote, err)
				return
			}
			// Handle message too big errors specifically
			if strings.Contains(err.Error(), "message too large") ||
				strings.Contains(err.Error(), "read limited at") {
				log.D.F("client %s hit message size limit: %v", remote, err)
				// Don't log this as an error since it's a client-side limit
				// Just close the connection gracefully
				return
			}
			// Check for websocket close errors
			if websocket.IsCloseError(err, websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived,
				websocket.CloseAbnormalClosure,
				websocket.CloseUnsupportedData,
				websocket.CloseInvalidFramePayloadData) {
				log.T.F("connection from %s closed: %v", remote, err)
			} else if websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
				log.D.F("client %s sent message too big: %v", remote, err)
			} else {
				log.E.F("unexpected close error from %s: %v", remote, err)
			}
			return
		}
		if typ == websocket.PingMessage {
			log.D.F("received PING from %s, sending PONG", remote)
			// Create a write context with timeout for pong response
			deadline := time.Now().Add(DefaultWriteTimeout)
			conn.SetWriteDeadline(deadline)
			pongStart := time.Now()
			if err = conn.WriteControl(websocket.PongMessage, msg, deadline); chk.E(err) {
				pongDuration := time.Since(pongStart)
				log.E.F(
					"failed to send PONG to %s after %v: %v", remote,
					pongDuration, err,
				)
				return
			}
			pongDuration := time.Since(pongStart)
			log.D.F("sent PONG to %s successfully in %v", remote, pongDuration)
			if pongDuration > time.Millisecond*50 {
				log.D.F("SLOW PONG to %s: %v (>50ms)", remote, pongDuration)
			}
			continue
		}
		// Log message size for debugging
		if len(msg) > 1000 { // Only log for larger messages
			log.D.F("received large message from %s: %d bytes", remote, len(msg))
		}
		// log.T.F("received message from %s: %s", remote, string(msg))
		listener.HandleMessage(msg, remote)
	}
}

func (s *Server) Pinger(
	ctx context.Context, conn *websocket.Conn, ticker *time.Ticker,
) {
	defer func() {
		log.D.F("pinger shutting down")
		ticker.Stop()
		// DO NOT call cancel here - the pinger should not be able to cancel the connection context
		// The connection handler will cancel the context when the connection is actually closing
	}()
	var err error
	pingCount := 0
	for {
		select {
		case <-ticker.C:
			pingCount++
			log.D.F("sending PING #%d", pingCount)

			// Set write deadline for ping operation
			deadline := time.Now().Add(DefaultWriteTimeout)
			conn.SetWriteDeadline(deadline)
			pingStart := time.Now()

			if err = conn.WriteControl(websocket.PingMessage, []byte{}, deadline); err != nil {
				pingDuration := time.Since(pingStart)
				log.E.F(
					"PING #%d FAILED after %v: %v", pingCount, pingDuration,
					err,
				)
				chk.E(err)
				return
			}

			pingDuration := time.Since(pingStart)
			log.D.F("PING #%d sent successfully in %v", pingCount, pingDuration)

			if pingDuration > time.Millisecond*100 {
				log.D.F("SLOW PING #%d: %v (>100ms)", pingCount, pingDuration)
			}
		case <-ctx.Done():
			log.T.F("pinger context cancelled after %d pings", pingCount)
			return
		}
	}
}
