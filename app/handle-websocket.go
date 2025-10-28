package app

import (
	"context"
	"crypto/rand"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
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

	// CloseMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	CloseMessage = 8

	// PingMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	PingMessage = 9

	// PongMessage denotes a pong control message. The optional message payload
	// is UTF-8 encoded text.
	PongMessage = 10
)

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
	// Configure WebSocket accept options for proxy compatibility
	acceptOptions := &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // Allow all origins for proxy compatibility
		// Don't check origin when behind a proxy - let the proxy handle it
		InsecureSkipVerify: true,
		// Try to set a higher compression threshold to allow larger messages
		CompressionMode: websocket.CompressionDisabled,
	}

	if conn, err = websocket.Accept(w, r, acceptOptions); chk.E(err) {
		log.E.F("websocket accept failed from %s: %v", remote, err)
		return
	}
	log.T.F("websocket accepted from %s path=%s", remote, r.URL.String())

	// Set read limit immediately after connection is established
	conn.SetReadLimit(DefaultMaxMessageSize)
	log.D.F("set read limit to %d bytes (%d MB) for %s", DefaultMaxMessageSize, DefaultMaxMessageSize/units.Mb, remote)
	defer conn.CloseNow()
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

		var typ websocket.MessageType
		var msg []byte
		log.T.F("waiting for message from %s", remote)

		// Block waiting for message; rely on pings and context cancellation to detect dead peers
		typ, msg, err = conn.Read(ctx)

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
			if strings.Contains(err.Error(), "MessageTooBig") ||
				strings.Contains(err.Error(), "read limited at") {
				log.D.F("client %s hit message size limit: %v", remote, err)
				// Don't log this as an error since it's a client-side limit
				// Just close the connection gracefully
				return
			}
			status := websocket.CloseStatus(err)
			switch status {
			case websocket.StatusNormalClosure,
				websocket.StatusGoingAway,
				websocket.StatusNoStatusRcvd,
				websocket.StatusAbnormalClosure,
				websocket.StatusProtocolError:
				log.T.F(
					"connection from %s closed with status: %v", remote, status,
				)
			case websocket.StatusMessageTooBig:
				log.D.F("client %s sent message too big: %v", remote, err)
			default:
				log.E.F("unexpected close error from %s: %v", remote, err)
			}
			return
		}
		if typ == PingMessage {
			log.D.F("received PING from %s, sending PONG", remote)
			// Create a write context with timeout for pong response
			writeCtx, writeCancel := context.WithTimeout(
				ctx, DefaultWriteTimeout,
			)
			pongStart := time.Now()
			if err = conn.Write(writeCtx, PongMessage, msg); chk.E(err) {
				pongDuration := time.Since(pongStart)
				log.E.F(
					"failed to send PONG to %s after %v: %v", remote,
					pongDuration, err,
				)
				if writeCtx.Err() != nil {
					log.E.F(
						"PONG write timeout to %s after %v (limit=%v)", remote,
						pongDuration, DefaultWriteTimeout,
					)
				}
				writeCancel()
				return
			}
			pongDuration := time.Since(pongStart)
			log.D.F("sent PONG to %s successfully in %v", remote, pongDuration)
			if pongDuration > time.Millisecond*50 {
				log.D.F("SLOW PONG to %s: %v (>50ms)", remote, pongDuration)
			}
			writeCancel()
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

			// Create a write context with timeout for ping operation
			pingCtx, pingCancel := context.WithTimeout(ctx, DefaultWriteTimeout)
			pingStart := time.Now()

			if err = conn.Ping(pingCtx); err != nil {
				pingDuration := time.Since(pingStart)
				log.E.F(
					"PING #%d FAILED after %v: %v", pingCount, pingDuration,
					err,
				)

				if pingCtx.Err() != nil {
					log.E.F(
						"PING #%d timeout after %v (limit=%v)", pingCount,
						pingDuration, DefaultWriteTimeout,
					)
				}

				chk.E(err)
				pingCancel()
				return
			}

			pingDuration := time.Since(pingStart)
			log.D.F("PING #%d sent successfully in %v", pingCount, pingDuration)

			if pingDuration > time.Millisecond*100 {
				log.D.F("SLOW PING #%d: %v (>100ms)", pingCount, pingDuration)
			}

			pingCancel()
		case <-ctx.Done():
			log.T.F("pinger context cancelled after %d pings", pingCount)
			return
		}
	}
}
