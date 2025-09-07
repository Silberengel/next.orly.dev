package app

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
	"lol.mleku.dev/chk"
	"utils.orly/atomic"
)

type Listener struct {
	*Server
	conn         *websocket.Conn
	ctx          context.Context
	remote       string
	req          *http.Request
	challenge    atomic.Bytes
	authedPubkey atomic.Bytes
}

func (l *Listener) Write(p []byte) (n int, err error) {
	if err = l.conn.Write(l.ctx, websocket.MessageText, p); chk.E(err) {
		return
	}
	n = len(p)
	return
}
