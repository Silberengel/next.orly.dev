package app

import (
	"context"
	"fmt"
	"net/http"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	database "next.orly.dev/pkg/database"
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
	addr := fmt.Sprintf("%s:%d", cfg.Listen, cfg.Port)
	log.I.F("starting listener on http://%s", addr)
	go func() {
		chk.E(http.ListenAndServe(addr, l))
	}()
	quit = make(chan struct{})
	return
}
