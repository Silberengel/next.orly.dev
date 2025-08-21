package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/version"
)

func main() {
	var err error
	var cfg *config.C
	if cfg, err = config.New(); chk.T(err) {
	}
	log.I.F("starting %s %s", cfg.AppName, version.V)
	startProfiler(cfg.Pprof)
	ctx, cancel := context.WithCancel(context.Background())
	quit := app.Run(ctx, cfg)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	for {
		select {
		case <-sigs:
			fmt.Printf("\r")
			cancel()
		case <-quit:
			cancel()
			return
		}
	}

}
