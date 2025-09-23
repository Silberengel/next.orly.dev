package app

import (
	"context"
	"fmt"
	"net/http"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/crypto/keys"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/bech32encoding"
	"next.orly.dev/pkg/protocol/publish"
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
	// start listener
	l := &Server{
		Ctx:        ctx,
		Config:     cfg,
		D:          db,
		publishers: publish.New(NewPublisher(ctx)),
		Admins:     adminKeys,
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
	go func() {
		chk.E(http.ListenAndServe(addr, l))
	}()
	quit = make(chan struct{})
	return
}
