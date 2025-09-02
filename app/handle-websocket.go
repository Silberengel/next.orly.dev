package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"protocol.orly/publish"
)

const (
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
	var cancel context.CancelFunc
	s.Ctx, cancel = context.WithCancel(s.Ctx)
	defer cancel()
	var err error
	var conn *websocket.Conn
	if conn, err = websocket.Accept(
		w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}},
	); chk.E(err) {
		return
	}
	defer conn.CloseNow()
	listener := &Listener{
		ctx:    s.Ctx,
		Server: s,
		conn:   conn,
		remote: remote,
	}
	listener.publishers = publish.New(NewPublisher())
	go s.Pinger(s.Ctx, conn, time.NewTicker(time.Second*10), cancel)
	for {
		select {
		case <-s.Ctx.Done():
			return
		default:
		}
		var typ websocket.MessageType
		var msg []byte
		if typ, msg, err = conn.Read(s.Ctx); err != nil {
			if strings.Contains(
				err.Error(), "use of closed network connection",
			) {
				return
			}
			status := websocket.CloseStatus(err)
			switch status {
			case websocket.StatusNormalClosure,
				websocket.StatusGoingAway,
				websocket.StatusNoStatusRcvd,
				websocket.StatusAbnormalClosure,
				websocket.StatusProtocolError:
			default:
				log.E.F("unexpected close error from %s: %v", remote, err)
			}
			return
		}
		if typ == PingMessage {
			if err = conn.Write(s.Ctx, PongMessage, msg); chk.E(err) {
				return
			}
			continue
		}
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
			if err = conn.Write(ctx, PingMessage, nil); err != nil {
				log.E.F("error writing ping: %v; closing websocket", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
