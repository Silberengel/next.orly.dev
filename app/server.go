package app

import (
	"context"
	"net/http"

	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
)

type Server struct {
	mux    *http.ServeMux
	Config *config.C
	Ctx    context.Context
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.T.F("path %v header %v", r.URL, r.Header)
	if r.Header.Get("Upgrade") == "websocket" {
		s.HandleWebsocket(w, r)
	} else if r.Header.Get("Accept") == "application/nostr+json" {
		s.HandleRelayInfo(w, r)
	} else {
		http.Error(w, "Upgrade required", http.StatusUpgradeRequired)
	}
}
