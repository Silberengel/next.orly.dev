package app

import (
	"context"
	"fmt"
	"net/http"

	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
)

func Run(ctx context.Context, cfg *config.C) (quit chan struct{}) {
	// shutdown handler
	go func() {
		select {
		case <-ctx.Done():
			log.I.F("shutting down")
			close(quit)
		}
	}()
	// start listener
	l := &Listener{
		Config: cfg,
	}
	addr := fmt.Sprintf("%s:%d", cfg.Listen, cfg.Port)
	log.I.F("starting listener on %s", addr)
	go http.ListenAndServe(addr, l)
	quit = make(chan struct{})
	return
}
