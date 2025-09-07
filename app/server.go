package app

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"database.orly"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	"protocol.orly/publish"
)

type Server struct {
	mux        *http.ServeMux
	Config     *config.C
	Ctx        context.Context
	remote     string
	publishers *publish.S
	Admins     [][]byte
	*database.D
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
func (s *Server) ServiceURL(req *http.Request) (st string) {
	host := req.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = req.Host
	}
	proto := req.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if host == "localhost" {
			proto = "ws"
		} else if strings.Contains(host, ":") {
			// has a port number
			proto = "ws"
		} else if _, err := strconv.Atoi(
			strings.ReplaceAll(
				host, ".",
				"",
			),
		); chk.E(err) {
			// it's a naked IP
			proto = "ws"
		} else {
			proto = "wss"
		}
	} else if proto == "https" {
		proto = "wss"
	} else if proto == "http" {
		proto = "ws"
	}
	return proto + "://" + host
}
