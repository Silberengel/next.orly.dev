package app

import (
	"net/http"

	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
)

type Listener struct {
	mux    *http.ServeMux
	Config *config.C
}

func (l *Listener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.I.F("path %v header %v", r.URL, r.Header)
	if r.Header.Get("Upgrade") == "websocket" {
		l.HandleWebsocket(w, r)
	} else if r.Header.Get("Accept") == "application/nostr+json" {
		l.HandleRelayInfo(w, r)
	} else {
		http.Error(w, "Upgrade required", http.StatusUpgradeRequired)
	}
}

func (l *Listener) HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	log.I.F("websocket")
	return
}
