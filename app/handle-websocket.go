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
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/utils/units"
)

const (
	DefaultWriteWait      = 10 * time.Second
	DefaultPongWait       = 60 * time.Second
	DefaultPingWait       = DefaultPongWait / 2
	DefaultWriteTimeout   = 3 * time.Second
	DefaultMaxMessageSize = 512000 // Match khatru's MaxMessageSize
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
	
	// Set initial read deadline - pong handler will extend it when pongs are received
	conn.SetReadDeadline(time.Now().Add(DefaultPongWait))
	
	defer conn.Close()
	listener := &Listener{
		ctx:            ctx,
		Server:         s,
		conn:           conn,
		remote:         remote,
		req:            r,
		startTime:      time.Now(),
		writeChan:      make(chan publish.WriteRequest, 100), // Buffered channel for writes
		writeDone:      make(chan struct{}),
		messageQueue:   make(chan messageRequest, 100), // Buffered channel for message processing
		processingDone: make(chan struct{}),
	}

	// Start write worker goroutine
	go listener.writeWorker()

	// Start message processor goroutine
	go listener.messageProcessor()

	// Register write channel with publisher
	if socketPub := listener.publishers.GetSocketPublisher(); socketPub != nil {
		socketPub.SetWriteChan(conn, listener.writeChan)
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
	go s.Pinger(ctx, listener, ticker)
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
			"ws connection closed %s: msgs=%d, REQs=%d, EVENTs=%d, dropped=%d, duration=%v",
			remote, listener.msgCount, listener.reqCount, listener.eventCount,
			listener.DroppedMessages(), dur,
		)

		// Log any remaining connection state
		if listener.authedPubkey.Load() != nil {
			log.D.F("ws connection %s was authenticated", remote)
		} else {
			log.D.F("ws connection %s was not authenticated", remote)
		}

		// Close message queue to signal processor to exit
		close(listener.messageQueue)
		// Wait for message processor to finish
		<-listener.processingDone

		// Close write channel to signal worker to exit
		close(listener.writeChan)
		// Wait for write worker to finish
		<-listener.writeDone
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

		// Don't set read deadline here - it's set initially and extended by pong handler
		// This prevents premature timeouts on idle connections with active subscriptions
		if ctx.Err() != nil {
			return
		}

		// Block waiting for message; rely on pings and context cancellation to detect dead peers
		// The read deadline is managed by the pong handler which extends it when pongs are received
		typ, msg, err = conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseNormalClosure,    // 1000
				websocket.CloseGoingAway,        // 1001
				websocket.CloseNoStatusReceived, // 1005
				websocket.CloseAbnormalClosure,  // 1006
				4537,                            // some client seems to send many of these
			) {
				log.I.F("websocket connection closed from %s: %v", remote, err)
			}
			cancel() // Cancel context like khatru does
			return
		}
		if typ == websocket.PingMessage {
			log.D.F("received PING from %s, sending PONG", remote)
			// Send pong directly (like khatru does)
			if err = conn.WriteMessage(websocket.PongMessage, nil); err != nil {
				log.E.F("failed to send PONG to %s: %v", remote, err)
				return
			}
			continue
		}
		// Log message size for debugging
		if len(msg) > 1000 { // Only log for larger messages
			log.D.F("received large message from %s: %d bytes", remote, len(msg))
		}
		// log.T.F("received message from %s: %s", remote, string(msg))

		// Queue message for asynchronous processing
		if !listener.QueueMessage(msg, remote) {
			log.W.F("ws->%s message queue full, dropping message (capacity=%d)", remote, cap(listener.messageQueue))
		}
	}
}

func (s *Server) Pinger(
	ctx context.Context, listener *Listener, ticker *time.Ticker,
) {
	defer func() {
		log.D.F("pinger shutting down")
		ticker.Stop()
	}()
	pingCount := 0
	for {
		select {
		case <-ctx.Done():
			log.T.F("pinger context cancelled after %d pings", pingCount)
			return
		case <-ticker.C:
			pingCount++
			// Send ping request through write channel - this allows pings to interrupt other writes
			select {
			case <-ctx.Done():
				return
			case listener.writeChan <- publish.WriteRequest{IsPing: true, MsgType: pingCount}:
				// Ping request queued successfully
			case <-time.After(DefaultWriteTimeout):
				log.E.F("ping #%d channel timeout - connection may be overloaded", pingCount)
				return
			}
		}
	}
}
