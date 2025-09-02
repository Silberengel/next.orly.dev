package app

import (
	"context"
	"fmt"
	"net/http"

	"database.orly"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	"protocol.orly/publish"
)

type Server struct {
	mux    *http.ServeMux
	Config *config.C
	Ctx    context.Context
	remote string
	*database.D
	publishers *publish.S
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.T.C(
		func() string {
			return fmt.Sprintf("path %v header %v", r.URL, r.Header)
		},
	)
	if r.Header.Get("Upgrade") == "websocket" {
		s.HandleWebsocket(w, r)
	} else if r.Header.Get("Accept") == "application/nostr+json" {
		s.HandleRelayInfo(w, r)
	} else {
		if s.mux == nil {
			http.Error(w, "Upgrade required", http.StatusUpgradeRequired)
		} else {
			s.mux.ServeHTTP(w, r)
		}
	}
}
