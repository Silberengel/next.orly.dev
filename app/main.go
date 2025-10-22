package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/crypto/keys"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/bech32encoding"
	"next.orly.dev/pkg/policy"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/spider"
)

func Run(
	ctx context.Context, cfg *config.C, db *database.D,
) (quit chan struct{}) {
	quit = make(chan struct{})
	var once sync.Once

	// shutdown handler
	go func() {
		<-ctx.Done()
		log.I.F("shutting down")
		once.Do(func() { close(quit) })
	}()
	// get the admins
	var err error
	var adminKeys [][]byte
	for _, admin := range cfg.Admins {
		if len(admin) == 0 {
			continue
		}
		var pk []byte
		if pk, err = bech32encoding.NpubOrHexToPublicKeyBinary(admin); chk.E(err) {
			continue
		}
		adminKeys = append(adminKeys, pk)
	}
	// get the owners
	var ownerKeys [][]byte
	for _, owner := range cfg.Owners {
		if len(owner) == 0 {
			continue
		}
		var pk []byte
		if pk, err = bech32encoding.NpubOrHexToPublicKeyBinary(owner); chk.E(err) {
			continue
		}
		ownerKeys = append(ownerKeys, pk)
	}
	// start listener
	l := &Server{
		Ctx:        ctx,
		Config:     cfg,
		D:          db,
		publishers: publish.New(NewPublisher(ctx)),
		Admins:     adminKeys,
		Owners:     ownerKeys,
	}

	// Initialize sprocket manager
	l.sprocketManager = NewSprocketManager(ctx, cfg.AppName, cfg.SprocketEnabled)

	// Initialize policy manager
	l.policyManager = policy.NewWithManager(ctx, cfg.AppName, cfg.PolicyEnabled)

	// Initialize spider manager based on mode
	if cfg.SpiderMode != "none" {
		if l.spiderManager, err = spider.New(ctx, db, l.publishers, cfg.SpiderMode); chk.E(err) {
			log.E.F("failed to create spider manager: %v", err)
		} else {
			// Set up callbacks for follows mode
			if cfg.SpiderMode == "follows" {
				l.spiderManager.SetCallbacks(
					func() []string {
						// Get admin relays from follows ACL if available
						for _, aclInstance := range acl.Registry.ACL {
							if aclInstance.Type() == "follows" {
								if follows, ok := aclInstance.(*acl.Follows); ok {
									return follows.AdminRelays()
								}
							}
						}
						return nil
					},
					func() [][]byte {
						// Get followed pubkeys from follows ACL if available
						for _, aclInstance := range acl.Registry.ACL {
							if aclInstance.Type() == "follows" {
								if follows, ok := aclInstance.(*acl.Follows); ok {
									return follows.GetFollowedPubkeys()
								}
							}
						}
						return nil
					},
				)
			}

			if err = l.spiderManager.Start(); chk.E(err) {
				log.E.F("failed to start spider manager: %v", err)
			} else {
				log.I.F("spider manager started successfully in '%s' mode", cfg.SpiderMode)
			}
		}
	}

	// Initialize the user interface
	l.UserInterface()

	// Ensure a relay identity secret key exists when subscriptions and NWC are enabled
	if cfg.SubscriptionEnabled && cfg.NWCUri != "" {
		if skb, e := db.GetOrCreateRelayIdentitySecret(); e != nil {
			log.E.F("failed to ensure relay identity key: %v", e)
		} else if pk, e2 := keys.SecretBytesToPubKeyHex(skb); e2 == nil {
			log.I.F("relay identity loaded (pub=%s)", pk)
			// ensure relay identity pubkey is considered an admin for ACL follows mode
			found := false
			for _, a := range cfg.Admins {
				if a == pk {
					found = true
					break
				}
			}
			if !found {
				cfg.Admins = append(cfg.Admins, pk)
				log.I.F("added relay identity to admins for follow-list whitelisting")
			}
			// also ensure relay identity pubkey is considered an owner for full control
			found = false
			for _, o := range cfg.Owners {
				if o == pk {
					found = true
					break
				}
			}
			if !found {
				cfg.Owners = append(cfg.Owners, pk)
				log.I.F("added relay identity to owners for full control")
			}
		}
	}

	if l.paymentProcessor, err = NewPaymentProcessor(ctx, cfg, db); err != nil {
		log.E.F("failed to create payment processor: %v", err)
		// Continue without payment processor
	} else {
		if err = l.paymentProcessor.Start(); err != nil {
			log.E.F("failed to start payment processor: %v", err)
		} else {
			log.I.F("payment processor started successfully")
		}
	}
	addr := fmt.Sprintf("%s:%d", cfg.Listen, cfg.Port)
	log.I.F("starting listener on http://%s", addr)

	// Create HTTP server for graceful shutdown
	srv := &http.Server{
		Addr:    addr,
		Handler: l,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.E.F("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown handler
	go func() {
		<-ctx.Done()
		log.I.F("shutting down HTTP server gracefully")

		// Stop spider manager if running
		if l.spiderManager != nil {
			l.spiderManager.Stop()
			log.I.F("spider manager stopped")
		}

		// Create shutdown context with timeout
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()

		// Shutdown the server gracefully
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.E.F("HTTP server shutdown error: %v", err)
		} else {
			log.I.F("HTTP server shutdown completed")
		}

		once.Do(func() { close(quit) })
	}()

	return
}
