package app

import (
	"context"

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

	quit = make(chan struct{})
	return
}
