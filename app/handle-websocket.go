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
	DefaultReadTimeout    = 3 * time.Second // Read timeout to detect stalled connections
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
		return
	}
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
		if err = authenvelope.NewChallengeWith(listener.challenge.Load()).
			Write(listener); chk.E(err) {
			return
		}
	}
	ticker := time.NewTicker(DefaultPingWait)
	go s.Pinger(ctx, conn, ticker, cancel)
	defer func() {
		// log.D.F("closing websocket connection from %s", remote)
		cancel()
		ticker.Stop()
		listener.publishers.Receive(&W{Cancel: true})
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		var typ websocket.MessageType
		var msg []byte
		log.T.F("waiting for message from %s", remote)

		// Create a read context with timeout to prevent indefinite blocking
		readCtx, readCancel := context.WithTimeout(ctx, DefaultReadTimeout)
		typ, msg, err = conn.Read(readCtx)
		readCancel()

		if err != nil {
			if strings.Contains(
				err.Error(), "use of closed network connection",
			) {
				return
			}
			// Handle timeout errors - occurs when client becomes unresponsive
			if strings.Contains(err.Error(), "context deadline exceeded") {
				log.T.F(
					"connection from %s timed out after %v", remote,
					DefaultReadTimeout,
				)
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
			// Create a write context with timeout for pong response
			writeCtx, writeCancel := context.WithTimeout(
				ctx, DefaultWriteTimeout,
			)
			if err = conn.Write(writeCtx, PongMessage, msg); chk.E(err) {
				writeCancel()
				return
			}
			writeCancel()
			continue
		}
		log.T.F("received message from %s: %s", remote, string(msg))
		go listener.HandleMessage(msg, remote)
	}
}

func (s *Server) Pinger(
	ctx context.Context, conn *websocket.Conn, ticker *time.Ticker,
	cancel context.CancelFunc,
) {
	defer func() {
		cancel()
		ticker.Stop()
	}()
	var err error
	for {
		select {
		case <-ticker.C:
			// Create a write context with timeout for ping operation
			pingCtx, pingCancel := context.WithTimeout(ctx, DefaultWriteTimeout)
			if err = conn.Ping(pingCtx); chk.E(err) {
				pingCancel()
				return
			}
			pingCancel()
		case <-ctx.Done():
			return
		}
	}
}
