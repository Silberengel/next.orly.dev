package app

import (
	"context"

	"github.com/coder/websocket"
	"lol.mleku.dev/chk"
)

type Listener struct {
	*Server
	conn   *websocket.Conn
	ctx    context.Context
	remote string
}

func (l *Listener) Write(p []byte) (n int, err error) {
	if err = l.conn.Write(l.ctx, websocket.MessageText, p); chk.E(err) {
		return
	}
	n = len(p)
	return
}
