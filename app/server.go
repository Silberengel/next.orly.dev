package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"lol.mleku.dev/chk"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/protocol/auth"
	"next.orly.dev/pkg/protocol/publish"
)

type Server struct {
	mux        *http.ServeMux
	Config     *config.C
	Ctx        context.Context
	remote     string
	publishers *publish.S
	Admins     [][]byte
	*database.D
	
	// Challenge storage for HTTP UI authentication
	challengeMutex sync.RWMutex
	challenges     map[string][]byte
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// log.T.C(
	// 	func() string {
	// 		return fmt.Sprintf("path %v header %v", r.URL, r.Header)
	// 	},
	// )
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

// UserInterface sets up a basic Nostr NDK interface that allows users to log into the relay user interface
func (s *Server) UserInterface() {
	if s.mux == nil {
		s.mux = http.NewServeMux()
	}
	
	// Initialize challenge storage if not already done
	if s.challenges == nil {
		s.challengeMutex.Lock()
		s.challenges = make(map[string][]byte)
		s.challengeMutex.Unlock()
	}
	
	// Serve the main login interface
	s.mux.HandleFunc("/", s.handleLoginInterface)
	
	// API endpoints for authentication
	s.mux.HandleFunc("/api/auth/challenge", s.handleAuthChallenge)
	s.mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	s.mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	s.mux.HandleFunc("/api/permissions/", s.handlePermissions)
}

// handleLoginInterface serves the main user interface for login
func (s *Server) handleLoginInterface(w http.ResponseWriter, r *http.Request) {
	// Create a file server handler for the embedded React app
	fileServer := http.FileServer(GetReactAppFS())
	
	// Serve the React app files
	fileServer.ServeHTTP(w, r)
}

// handleAuthChallenge generates and returns an authentication challenge
func (s *Server) handleAuthChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Generate a proper challenge using the auth package
	challenge := auth.GenerateChallenge()
	challengeHex := hex.Enc(challenge)
	
	// Store the challenge using the hex value as the key for easy lookup
	s.challengeMutex.Lock()
	s.challenges[challengeHex] = challenge
	s.challengeMutex.Unlock()
	
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"challenge": "` + challengeHex + `"}`))
}

// handleAuthLogin processes authentication requests
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if chk.E(err) {
		w.Write([]byte(`{"success": false, "error": "Failed to read request body"}`))
		return
	}
	
	// Parse the signed event
	var evt event.E
	if err = json.Unmarshal(body, &evt); chk.E(err) {
		w.Write([]byte(`{"success": false, "error": "Invalid event format"}`))
		return
	}
	
	// Extract the challenge from the event to look up the stored challenge
	challengeTag := evt.Tags.GetFirst([]byte("challenge"))
	if challengeTag == nil {
		w.Write([]byte(`{"success": false, "error": "Challenge tag missing from event"}`))
		return
	}
	
	challengeHex := string(challengeTag.Value())
	
	// Retrieve the stored challenge
	s.challengeMutex.RLock()
	_, exists := s.challenges[challengeHex]
	s.challengeMutex.RUnlock()
	
	if !exists {
		w.Write([]byte(`{"success": false, "error": "Invalid or expired challenge"}`))
		return
	}
	
	// Clean up the used challenge
	s.challengeMutex.Lock()
	delete(s.challenges, challengeHex)
	s.challengeMutex.Unlock()
	
	relayURL := s.ServiceURL(r)
	
	// Validate the authentication event with the correct challenge
	// The challenge in the event tag is hex-encoded, so we need to pass the hex string as bytes
	ok, err := auth.Validate(&evt, []byte(challengeHex), relayURL)
	if chk.E(err) || !ok {
		errorMsg := "Authentication validation failed"
		if err != nil {
			errorMsg = err.Error()
		}
		w.Write([]byte(`{"success": false, "error": "` + errorMsg + `"}`))
		return
	}
	
	// Authentication successful
	w.Write([]byte(`{"success": true, "pubkey": "` + hex.Enc(evt.Pubkey) + `", "message": "Authentication successful"}`))
}

// handleAuthStatus returns the current authentication status
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"authenticated": false}`))
}

// handlePermissions returns the permission level for a given pubkey
func (s *Server) handlePermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Extract pubkey from URL path
	pubkeyHex := strings.TrimPrefix(r.URL.Path, "/api/permissions/")
	if pubkeyHex == "" || pubkeyHex == "/" {
		http.Error(w, "Invalid pubkey", http.StatusBadRequest)
		return
	}
	
	// Convert hex to binary pubkey
	pubkey, err := hex.Dec(pubkeyHex)
	if chk.E(err) {
		http.Error(w, "Invalid pubkey format", http.StatusBadRequest)
		return
	}
	
	// Get access level using acl registry
	permission := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	
	// Set content type and write JSON response
	w.Header().Set("Content-Type", "application/json")
	
	// Format response as proper JSON
	response := struct {
		Permission string `json:"permission"`
	}{
		Permission: permission,
	}
	
	// Marshal and write the response
	jsonData, err := json.Marshal(response)
	if chk.E(err) {
		http.Error(w, "Error generating response", http.StatusInternalServerError)
		return
	}
	
	w.Write(jsonData)
}
