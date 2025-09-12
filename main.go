package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
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
	var db *database.D
	if db, err = database.New(
		ctx, cancel, cfg.DataDir, cfg.DBLogLevel,
	); chk.E(err) {
		os.Exit(1)
	}
	acl.Registry.Active.Store(cfg.ACLMode)
	if err = acl.Registry.Configure(cfg, db, ctx); chk.E(err) {
		os.Exit(1)
	}
	acl.Registry.Syncer()

	// Start health check HTTP server if configured
	var healthSrv *http.Server
	if cfg.HealthPort > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc(
			"/healthz", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
				log.I.F("health check ok")
			},
		)
		healthSrv = &http.Server{
			Addr: fmt.Sprintf(
				"%s:%d", cfg.Listen, cfg.HealthPort,
			), Handler: mux,
		}
		go func() {
			log.I.F("health check server listening on %s", healthSrv.Addr)
			if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.E.F("health server error: %v", err)
			}
		}()
		go func() {
			<-ctx.Done()
			shutdownCtx, cancelShutdown := context.WithTimeout(
				context.Background(), 2*time.Second,
			)
			defer cancelShutdown()
			_ = healthSrv.Shutdown(shutdownCtx)
		}()
	}

	quit := app.Run(ctx, cfg, db)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	for {
		select {
		case <-sigs:
			fmt.Printf("\r")
			cancel()
			chk.E(db.Close())
			return
		case <-quit:
			cancel()
			chk.E(db.Close())
			return
		}
	}

}
