package app

import (
	"context"
	"fmt"
	"net/http"

	database "database.orly"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
)

func Run(
	ctx context.Context, cfg *config.C, db *database.D,
) (quit chan struct{}) {
	// shutdown handler
	go func() {
		select {
		case <-ctx.Done():
			log.I.F("shutting down")
			close(quit)
		}
	}()
	// start listener
	l := &Server{
		Ctx:    ctx,
		Config: cfg,
		D:      db,
	}
	addr := fmt.Sprintf("%s:%d", cfg.Listen, cfg.Port)
	log.I.F("starting listener on http://%s", addr)
	go func() {
		chk.E(http.ListenAndServe(addr, l))
	}()
	quit = make(chan struct{})
	return
}
