package main

import (
	"context"
	"fmt"
	"net/http"
	pp "net/http/pprof"
	"os"
	"os/exec"
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
	"next.orly.dev/pkg/spider"
	"next.orly.dev/pkg/version"
)

// openBrowser attempts to open the specified URL in the default browser.
// It supports multiple platforms including Linux, macOS, and Windows.
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command(
			"rundll32", "url.dll,FileProtocolHandler", url,
		).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		log.W.F("unsupported platform for opening browser: %s", runtime.GOOS)
		return
	}

	if err != nil {
		log.E.F("failed to open browser: %v", err)
	} else {
		log.I.F("opened browser to %s", url)
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() * 4)
	var err error
	var cfg *config.C
	if cfg, err = config.New(); chk.T(err) {
	}
	log.I.F("starting %s %s", cfg.AppName, version.V)

	// If OpenPprofWeb is true and profiling is enabled, we need to ensure HTTP profiling is also enabled
	if cfg.OpenPprofWeb && cfg.Pprof != "" && !cfg.PprofHTTP {
		log.I.F("enabling HTTP pprof server to support web viewer")
		cfg.PprofHTTP = true
	}
	switch cfg.Pprof {
	case "cpu":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.CPUProfile, profile.ProfilePath(cfg.PprofPath),
			)
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.CPUProfile)
			defer prof.Stop()
		}
	case "memory":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.MemProfile, profile.MemProfileRate(32),
				profile.ProfilePath(cfg.PprofPath),
			)
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.MemProfile)
			defer prof.Stop()
		}
	case "allocation":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.MemProfileAllocs, profile.MemProfileRate(32),
				profile.ProfilePath(cfg.PprofPath),
			)
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.MemProfileAllocs)
			defer prof.Stop()
		}
	case "heap":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.MemProfileHeap, profile.ProfilePath(cfg.PprofPath),
			)
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.MemProfileHeap)
			defer prof.Stop()
		}
	case "mutex":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.MutexProfile, profile.ProfilePath(cfg.PprofPath),
			)
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.MutexProfile)
			defer prof.Stop()
		}
	case "threadcreate":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.ThreadcreationProfile,
				profile.ProfilePath(cfg.PprofPath),
			)
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.ThreadcreationProfile)
			defer prof.Stop()
		}
	case "goroutine":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.GoroutineProfile, profile.ProfilePath(cfg.PprofPath),
			)
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.GoroutineProfile)
			defer prof.Stop()
		}
	case "block":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.BlockProfile, profile.ProfilePath(cfg.PprofPath),
			)
			defer prof.Stop()
		} else {
			prof := profile.Start(profile.BlockProfile)
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

	// Initialize and start spider functionality if enabled
	spiderCtx, spiderCancel := context.WithCancel(ctx)
	spiderInstance := spider.New(db, cfg, spiderCtx, spiderCancel)
	spiderInstance.Start()
	defer spiderInstance.Stop()

	// Start HTTP pprof server if enabled
	if cfg.PprofHTTP {
		pprofAddr := fmt.Sprintf("%s:%d", cfg.Listen, 6060)
		pprofMux := http.NewServeMux()
		pprofMux.HandleFunc("/debug/pprof/", pp.Index)
		pprofMux.HandleFunc("/debug/pprof/cmdline", pp.Cmdline)
		pprofMux.HandleFunc("/debug/pprof/profile", pp.Profile)
		pprofMux.HandleFunc("/debug/pprof/symbol", pp.Symbol)
		pprofMux.HandleFunc("/debug/pprof/trace", pp.Trace)
		for _, p := range []string{
			"allocs", "block", "goroutine", "heap", "mutex", "threadcreate",
		} {
			pprofMux.Handle("/debug/pprof/"+p, pp.Handler(p))
		}
		ppSrv := &http.Server{Addr: pprofAddr, Handler: pprofMux}
		go func() {
			log.I.F("pprof server listening on %s", pprofAddr)
			if err := ppSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.E.F("pprof server error: %v", err)
			}
		}()
		go func() {
			<-ctx.Done()
			shutdownCtx, cancelShutdown := context.WithTimeout(
				context.Background(), 2*time.Second,
			)
			defer cancelShutdown()
			_ = ppSrv.Shutdown(shutdownCtx)
		}()

		// Open the pprof web viewer if enabled
		if cfg.OpenPprofWeb && cfg.Pprof != "" {
			pprofURL := fmt.Sprintf("http://localhost:6060/debug/pprof/")
			go func() {
				// Wait a moment for the server to start
				time.Sleep(500 * time.Millisecond)
				openBrowser(pprofURL)
			}()
		}
	}

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
			mux.HandleFunc(
				"/shutdown", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("shutting down"))
					log.I.F("shutdown requested via /shutdown; sending SIGINT to self")
					go func() {
						p, _ := os.FindProcess(os.Getpid())
						_ = p.Signal(os.Interrupt)
					}()
				},
			)
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
			log.I.F("exiting")
			return
		case <-quit:
			cancel()
			chk.E(db.Close())
			log.I.F("exiting")
			return
		}
	}
	log.I.F("exiting")
}
