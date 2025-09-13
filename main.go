package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/pkg/profile"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/version"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() * 4)
	var err error
	var cfg *config.C
	if cfg, err = config.New(); chk.T(err) {
	}
	log.I.F("starting %s %s", cfg.AppName, version.V)
	switch cfg.Pprof {
	case "cpu":
		if cfg.PprofPath != "" {
			prof := profile.Start(profile.CPUProfile, profile.ProfilePath(cfg.PprofPath))
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.CPUProfile)
			defer prof.Stop()
		}
	case "memory":
		if cfg.PprofPath != "" {
			prof := profile.Start(profile.MemProfile, profile.ProfilePath(cfg.PprofPath))
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.MemProfile)
			defer prof.Stop()
		}
	case "allocation":
		if cfg.PprofPath != "" {
			prof := profile.Start(profile.MemProfileAllocs, profile.ProfilePath(cfg.PprofPath))
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.MemProfileAllocs)
			defer prof.Stop()
		}
	}
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
		// Optional shutdown endpoint to gracefully stop the process so profiling defers run
		if cfg.EnableShutdown {
			mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("shutting down"))
				log.I.F("shutdown requested via /shutdown; sending SIGINT to self")
				go func() {
					p, _ := os.FindProcess(os.Getpid())
					_ = p.Signal(os.Interrupt)
				}()
			})
		}
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
