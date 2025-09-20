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
}

// handleLoginInterface serves the main user interface for login
func (s *Server) handleLoginInterface(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Nostr Relay Login</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        .container { background: #f9f9f9; padding: 30px; border-radius: 8px; }
        .form-group { margin-bottom: 20px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input, textarea { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 4px; }
        button { background: #007cba; color: white; padding: 12px 20px; border: none; border-radius: 4px; cursor: pointer; }
        button:hover { background: #005a87; }
        .status { margin-top: 20px; padding: 10px; border-radius: 4px; }
        .success { background: #d4edda; color: #155724; }
        .error { background: #f8d7da; color: #721c24; }
        .info { background: #d1ecf1; color: #0c5460; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Nostr Relay Authentication</h1>
        <p>Connect to this Nostr relay using your private key or browser extension.</p>
        
        <div id="status" class="status info">
            Ready to authenticate
        </div>
        
        <div class="form-group">
            <button onclick="loginWithExtension()">Login with Browser Extension (NIP-07)</button>
        </div>
        
        <div class="form-group">
            <label for="nsec">Or login with private key (nsec):</label>
            <input type="password" id="nsec" placeholder="nsec1...">
            <button onclick="loginWithPrivateKey()" style="margin-top: 10px;">Login with Private Key</button>
        </div>
        
        <div class="form-group">
            <button onclick="logout()" style="background: #dc3545;">Logout</button>
        </div>
    </div>

    <script>
        let currentUser = null;
        
        function updateStatus(message, type = 'info') {
            const status = document.getElementById('status');
            status.textContent = message;
            status.className = 'status ' + type;
        }
        
        async function getChallenge() {
            try {
                const response = await fetch('/api/auth/challenge');
                const data = await response.json();
                return data.challenge;
            } catch (error) {
                updateStatus('Failed to get authentication challenge: ' + error.message, 'error');
                throw error;
            }
        }
        
        async function loginWithExtension() {
            if (!window.nostr) {
                updateStatus('No Nostr extension found. Please install a NIP-07 compatible extension like nos2x or Alby.', 'error');
                return;
            }
            
            try {
                updateStatus('Connecting to extension...', 'info');
                
                // Get public key from extension
                const pubkey = await window.nostr.getPublicKey();
                
                // Get challenge from server
                const challenge = await getChallenge();
                
                // Create authentication event
                const authEvent = {
                    kind: 22242,
                    created_at: Math.floor(Date.now() / 1000),
                    tags: [
                        ['relay', window.location.protocol.replace('http', 'ws') + '//' + window.location.host],
                        ['challenge', challenge]
                    ],
                    content: ''
                };
                
                // Sign the event with extension
                const signedEvent = await window.nostr.signEvent(authEvent);
                
                // Send to server
                await authenticate(signedEvent);
                
            } catch (error) {
                updateStatus('Extension login failed: ' + error.message, 'error');
            }
        }
        
        async function loginWithPrivateKey() {
            const nsec = document.getElementById('nsec').value;
            if (!nsec) {
                updateStatus('Please enter your private key', 'error');
                return;
            }
            
            updateStatus('Private key login not implemented in this basic interface. Please use a proper Nostr client or extension.', 'error');
        }
        
        async function authenticate(signedEvent) {
            try {
                const response = await fetch('/api/auth/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(signedEvent)
                });
                
                const result = await response.json();
                
                if (result.success) {
                    currentUser = result.pubkey;
                    updateStatus('Successfully authenticated as: ' + result.pubkey.slice(0, 16) + '...', 'success');
                } else {
                    updateStatus('Authentication failed: ' + result.error, 'error');
                }
            } catch (error) {
                updateStatus('Authentication request failed: ' + error.message, 'error');
            }
        }
        
        async function logout() {
            currentUser = null;
            updateStatus('Logged out', 'info');
        }
        
        // Check authentication status on page load
        async function checkStatus() {
            try {
                const response = await fetch('/api/auth/status');
                const data = await response.json();
                if (data.authenticated) {
                    currentUser = data.pubkey;
                    updateStatus('Already authenticated as: ' + data.pubkey.slice(0, 16) + '...', 'success');
                }
            } catch (error) {
                // Ignore errors for status check
            }
        }
        
        checkStatus();
    </script>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
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
