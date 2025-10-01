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
	DefaultMaxMessageSize = 1 * units.Mb

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
	log.T.F("handling websocket connection from %s", remote)
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
	ctx, cancel := context.WithCancel(s.Ctx)
	defer cancel()
	var err error
	var conn *websocket.Conn
	if conn, err = websocket.Accept(
		w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}},
	); chk.E(err) {
		log.E.F("websocket accept failed from %s: %v", remote, err)
		return
	}
	log.T.F("websocket accepted from %s path=%s", remote, r.URL.String())
	conn.SetReadLimit(DefaultMaxMessageSize)
	defer conn.CloseNow()
	listener := &Listener{
		ctx:    ctx,
		Server: s,
		conn:   conn,
		remote: remote,
		req:    r,
	}
	chal := make([]byte, 32)
	rand.Read(chal)
	listener.challenge.Store([]byte(hex.Enc(chal)))
	// If admins are configured, immediately prompt client to AUTH (NIP-42)
	if len(s.Config.Admins) > 0 {
		// log.D.F("sending initial AUTH challenge to %s", remote)
		log.D.F("sending AUTH challenge to %s", remote)
		if err = authenvelope.NewChallengeWith(listener.challenge.Load()).
			Write(listener); chk.E(err) {
			log.E.F("failed to send AUTH challenge to %s: %v", remote, err)
			return
		}
		log.D.F("AUTH challenge sent successfully to %s", remote)
	}
	ticker := time.NewTicker(DefaultPingWait)
	go s.Pinger(ctx, conn, ticker, cancel)
	defer func() {
		log.D.F("closing websocket connection from %s", remote)
		
		// Cancel context and stop pinger
		cancel()
		ticker.Stop()
		
		// Cancel all subscriptions for this connection
		log.D.F("cancelling subscriptions for %s", remote)
		listener.publishers.Receive(&W{Cancel: true})
		
		// Log detailed connection statistics
		log.D.F("ws connection closed %s: msgs=%d, REQs=%d, EVENTs=%d, duration=%v", 
			remote, listener.msgCount, listener.reqCount, listener.eventCount, 
			time.Since(time.Now())) // Note: This will be near-zero, would need start time tracked
		
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
		var typ websocket.MessageType
		var msg []byte
		// log.T.F("waiting for message from %s", remote)

		// Block waiting for message; rely on pings and context cancellation to detect dead peers
		typ, msg, err = conn.Read(ctx)

		if err != nil {
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
				log.E.F("failed to send PONG to %s after %v: %v", remote, pongDuration, err)
				if writeCtx.Err() != nil {
					log.E.F("PONG write timeout to %s after %v (limit=%v)", remote, pongDuration, DefaultWriteTimeout)
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
		// log.T.F("received message from %s: %s", remote, string(msg))
		listener.HandleMessage(msg, remote)
	}
}

func (s *Server) Pinger(
	ctx context.Context, conn *websocket.Conn, ticker *time.Ticker,
	cancel context.CancelFunc,
) {
	defer func() {
		log.D.F("pinger shutting down")
		cancel()
		ticker.Stop()
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
				log.E.F("PING #%d FAILED after %v: %v", pingCount, pingDuration, err)
				
				if pingCtx.Err() != nil {
					log.E.F("PING #%d timeout after %v (limit=%v)", pingCount, pingDuration, DefaultWriteTimeout)
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
			log.D.F("pinger context cancelled after %d pings", pingCount)
			return
		}
	}
}
