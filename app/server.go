package app

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"lol.mleku.dev/chk"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/tag"
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

	// optional reverse proxy for dev web server
	devProxy *httputil.ReverseProxy

	// Challenge storage for HTTP UI authentication
	challengeMutex sync.RWMutex
	challenges     map[string][]byte
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for all responses
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set(
		"Access-Control-Allow-Headers", "Content-Type, Authorization",
	)

	// Handle preflight OPTIONS requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// If this is a websocket request, only intercept the relay root path.
	// This allows other websocket paths (e.g., Vite HMR) to be handled by the dev proxy when enabled.
	if r.Header.Get("Upgrade") == "websocket" {
		if s.mux != nil && s.Config != nil && s.Config.WebDisableEmbedded && s.Config.WebDevProxyURL != "" && r.URL.Path != "/" {
			// forward to mux (which will proxy to dev server)
			s.mux.ServeHTTP(w, r)
			return
		}
		s.HandleWebsocket(w, r)
		return
	}

	if r.Header.Get("Accept") == "application/nostr+json" {
		s.HandleRelayInfo(w, r)
		return
	}

	if s.mux == nil {
		http.Error(w, "Upgrade required", http.StatusUpgradeRequired)
		return
	}
	s.mux.ServeHTTP(w, r)
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

	// If dev proxy is configured, initialize it
	if s.Config != nil && s.Config.WebDisableEmbedded && s.Config.WebDevProxyURL != "" {
		proxyURL := s.Config.WebDevProxyURL
		// Add default scheme if missing to avoid: proxy error: unsupported protocol scheme ""
		if !strings.Contains(proxyURL, "://") {
			proxyURL = "http://" + proxyURL
		}
		if target, err := url.Parse(proxyURL); !chk.E(err) {
			if target.Scheme == "" || target.Host == "" {
				// invalid URL, disable proxy
				log.Printf(
					"invalid ORLY_WEB_DEV_PROXY_URL: %q — disabling dev proxy\n",
					s.Config.WebDevProxyURL,
				)
			} else {
				s.devProxy = httputil.NewSingleHostReverseProxy(target)
				// Ensure Host header points to upstream for dev servers that care
				origDirector := s.devProxy.Director
				s.devProxy.Director = func(req *http.Request) {
					origDirector(req)
					req.Host = target.Host
				}
			}
		}
	}

	// Initialize challenge storage if not already done
	if s.challenges == nil {
		s.challengeMutex.Lock()
		s.challenges = make(map[string][]byte)
		s.challengeMutex.Unlock()
	}

	// Serve the main login interface (and static assets) or proxy in dev mode
	s.mux.HandleFunc("/", s.handleLoginInterface)

	// API endpoints for authentication
	s.mux.HandleFunc("/api/auth/challenge", s.handleAuthChallenge)
	s.mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	s.mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	s.mux.HandleFunc("/api/auth/logout", s.handleAuthLogout)
	s.mux.HandleFunc("/api/permissions/", s.handlePermissions)
	// Export endpoints
	s.mux.HandleFunc("/api/export", s.handleExport)
	s.mux.HandleFunc("/api/export/mine", s.handleExportMine)
	// Events endpoints
	s.mux.HandleFunc("/api/events/mine", s.handleEventsMine)
	// Import endpoint (admin only)
	s.mux.HandleFunc("/api/import", s.handleImport)
}

// handleLoginInterface serves the main user interface for login
func (s *Server) handleLoginInterface(w http.ResponseWriter, r *http.Request) {
	// In dev mode with proxy configured, forward to dev server
	if s.Config != nil && s.Config.WebDisableEmbedded && s.devProxy != nil {
		s.devProxy.ServeHTTP(w, r)
		return
	}
	// If embedded UI is disabled but no proxy configured, return a helpful message
	if s.Config != nil && s.Config.WebDisableEmbedded {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Web UI disabled (ORLY_WEB_DISABLE=true). Run the web app in standalone dev mode (e.g., npm run dev) or set ORLY_WEB_DEV_PROXY_URL to proxy through this server."))
		return
	}
	// Default: serve embedded React app
	fileServer := http.FileServer(GetReactAppFS())
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

	// Authentication successful: set a simple session cookie with the pubkey
	cookie := &http.Cookie{
		Name:     "orly_auth",
		Value:    hex.Enc(evt.Pubkey),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 30, // 30 days
	}
	http.SetCookie(w, cookie)
	w.Write([]byte(`{"success": true, "pubkey": "` + hex.Enc(evt.Pubkey) + `", "message": "Authentication successful"}`))
}

// handleAuthStatus returns the current authentication status
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// Check for auth cookie
	if c, err := r.Cookie("orly_auth"); err == nil && c.Value != "" {
		// Validate pubkey format (hex)
		if _, err := hex.Dec(c.Value); !chk.E(err) {
			w.Write([]byte(`{"authenticated": true, "pubkey": "` + c.Value + `"}`))
			return
		}
	}
	w.Write([]byte(`{"authenticated": false}`))
}

// handleAuthLogout clears the auth cookie
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Expire the cookie
	http.SetCookie(
		w, &http.Cookie{
			Name:     "orly_auth",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		},
	)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success": true}`))
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
		http.Error(
			w, "Error generating response", http.StatusInternalServerError,
		)
		return
	}

	w.Write(jsonData)
}

// handleExport streams all events as JSONL (NDJSON). Admins only.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require auth cookie
	c, err := r.Cookie("orly_auth")
	if err != nil || c.Value == "" {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	requesterPubHex := c.Value
	requesterPub, err := hex.Dec(requesterPubHex)
	if chk.E(err) {
		http.Error(w, "Invalid auth cookie", http.StatusUnauthorized)
		return
	}
	// Check permissions
	if acl.Registry.GetAccessLevel(requesterPub, r.RemoteAddr) != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Optional filtering by pubkey(s)
	var pks [][]byte
	q := r.URL.Query()
	for _, pkHex := range q["pubkey"] {
		if pkHex == "" {
			continue
		}
		if pk, err := hex.Dec(pkHex); !chk.E(err) {
			pks = append(pks, pk)
		}
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	filename := "events-" + time.Now().UTC().Format("20060102-150405Z") + ".jsonl"
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	// Stream export
	s.D.Export(s.Ctx, w, pks...)
}


// handleExportMine streams only the authenticated user's events as JSONL (NDJSON).
func (s *Server) handleExportMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require auth cookie
	c, err := r.Cookie("orly_auth")
	if err != nil || c.Value == "" {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	pubkey, err := hex.Dec(c.Value)
	if chk.E(err) {
		http.Error(w, "Invalid auth cookie", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	filename := "my-events-" + time.Now().UTC().Format("20060102-150405Z") + ".jsonl"
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	// Stream export for this user's pubkey only
	s.D.Export(s.Ctx, w, pubkey)
}

// handleImport receives a JSONL/NDJSON file or body and enqueues an async import. Admins only.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require auth cookie
	c, err := r.Cookie("orly_auth")
	if err != nil || c.Value == "" {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	requesterPub, err := hex.Dec(c.Value)
	if chk.E(err) {
		http.Error(w, "Invalid auth cookie", http.StatusUnauthorized)
		return
	}
	// Admins only
	if acl.Registry.GetAccessLevel(requesterPub, r.RemoteAddr) != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); chk.E(err) { // 32MB memory, rest to temp files
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("file")
		if chk.E(err) {
			http.Error(w, "Missing file", http.StatusBadRequest)
			return
		}
		defer file.Close()
		s.D.Import(file)
	} else {
		if r.Body == nil {
			http.Error(w, "Empty request body", http.StatusBadRequest)
			return
		}
 	s.D.Import(r.Body)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"success": true, "message": "Import started"}`))
}

// handleEventsMine returns the authenticated user's events in JSON format with pagination
func (s *Server) handleEventsMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require auth cookie
	c, err := r.Cookie("orly_auth")
	if err != nil || c.Value == "" {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	pubkey, err := hex.Dec(c.Value)
	if chk.E(err) {
		http.Error(w, "Invalid auth cookie", http.StatusUnauthorized)
		return
	}

	// Parse pagination parameters
	query := r.URL.Query()
	limit := 50 // default limit
	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Use QueryEvents with filter for this user's events
	f := &filter.F{
		Authors: tag.NewFromBytesSlice(pubkey),
	}

	log.Printf("DEBUG: Querying events for pubkey: %s", hex.Enc(pubkey))
	events, err := s.D.QueryEvents(s.Ctx, f)
	if chk.E(err) {
		log.Printf("DEBUG: QueryEvents failed: %v", err)
		http.Error(w, "Failed to query events", http.StatusInternalServerError)
		return
	}
	log.Printf("DEBUG: QueryEvents returned %d events", len(events))

	// If no events found, let's also check if there are any events at all in the database
	if len(events) == 0 {
		// Create a filter to get any events (no authors filter)
		allEventsFilter := &filter.F{}
		allEvents, err := s.D.QueryEvents(s.Ctx, allEventsFilter)
		if err == nil {
			log.Printf("DEBUG: Total events in database: %d", len(allEvents))
		} else {
			log.Printf("DEBUG: Failed to query all events: %v", err)
		}
	}

	// Events are already sorted by QueryEvents in reverse chronological order
	
	// Apply offset and limit manually since QueryEvents doesn't support offset
	totalEvents := len(events)
	start := offset
	if start > totalEvents {
		start = totalEvents
	}
	end := start + limit
	if end > totalEvents {
		end = totalEvents
	}

	paginatedEvents := events[start:end]

	// Convert events to JSON response format
	type EventResponse struct {
		ID        string    `json:"id"`
		Kind      int       `json:"kind"`
		CreatedAt int64     `json:"created_at"`
		Content   string    `json:"content"`
		RawJSON   string    `json:"raw_json"`
	}

	response := struct {
		Events []EventResponse `json:"events"`
		Total  int             `json:"total"`
		Offset int             `json:"offset"`
		Limit  int             `json:"limit"`
	}{
		Events: make([]EventResponse, len(paginatedEvents)),
		Total:  totalEvents,
		Offset: offset,
		Limit:  limit,
	}

	for i, ev := range paginatedEvents {
		response.Events[i] = EventResponse{
			ID:        hex.Enc(ev.ID),
			Kind:      int(ev.Kind),
			CreatedAt: int64(ev.CreatedAt),
			Content:   string(ev.Content),
			RawJSON:   string(ev.Serialize()),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
