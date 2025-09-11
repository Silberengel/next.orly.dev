package app

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"lol.mleku.dev/chk"
	"utils.orly/atomic"
)

const WriteTimeout = 10 * time.Second

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
	// Use a separate context with timeout for writes to prevent race conditions
	// where the main connection context gets cancelled while writing events
	writeCtx, cancel := context.WithTimeout(context.Background(), WriteTimeout)
	defer cancel()
	
	if err = l.conn.Write(writeCtx, websocket.MessageText, p); chk.E(err) {
		return
	}
	n = len(p)
	return
}
