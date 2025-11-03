package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database"
)

// Manager handles distributed synchronization between relay peers using serial numbers as clocks
type Manager struct {
	ctx        context.Context
	cancel     context.CancelFunc
	db         *database.D
	nodeID     string
	relayURL   string
	peers      []string
	currentSerial uint64
	peerSerials   map[string]uint64 // peer URL -> latest serial seen
	mutex      sync.RWMutex
}

// CurrentRequest represents a request for the current serial number
type CurrentRequest struct {
	NodeID   string `json:"node_id"`
	RelayURL string `json:"relay_url"`
}

// CurrentResponse returns the current serial number
type CurrentResponse struct {
	NodeID      string `json:"node_id"`
	RelayURL    string `json:"relay_url"`
	Serial      uint64 `json:"serial"`
}

// FetchRequest represents a request for events in a serial range
type FetchRequest struct {
	NodeID   string `json:"node_id"`
	RelayURL string `json:"relay_url"`
	From     uint64 `json:"from"`
	To       uint64 `json:"to"`
}

// FetchResponse contains the requested events as JSONL
type FetchResponse struct {
	Events []string `json:"events"` // JSONL formatted events
}

// NewManager creates a new sync manager
func NewManager(ctx context.Context, db *database.D, nodeID, relayURL string, peers []string) *Manager {
	ctx, cancel := context.WithCancel(ctx)

	m := &Manager{
		ctx:          ctx,
		cancel:       cancel,
		db:           db,
		nodeID:       nodeID,
		relayURL:     relayURL,
		peers:        peers,
		currentSerial: 0,
		peerSerials:  make(map[string]uint64),
	}

	// Start sync routine
	go m.syncRoutine()

	return m
}

// Stop stops the sync manager
func (m *Manager) Stop() {
	m.cancel()
}

// GetCurrentSerial returns the current serial number
func (m *Manager) GetCurrentSerial() uint64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentSerial
}

// UpdateSerial updates the current serial number when a new event is stored
func (m *Manager) UpdateSerial() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get the latest serial from database
	if latest, err := m.getLatestSerial(); err == nil {
		m.currentSerial = latest
	}
}

// getLatestSerial gets the latest serial number from the database
func (m *Manager) getLatestSerial() (uint64, error) {
	// This is a simplified implementation
	// In practice, you'd want to track the highest serial number
	// For now, return the current serial
	return m.currentSerial, nil
}

// syncRoutine periodically syncs with peers
func (m *Manager) syncRoutine() {
	ticker := time.NewTicker(5 * time.Second) // Sync every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.syncWithPeers()
		}
	}
}

// syncWithPeers syncs with all configured peers
func (m *Manager) syncWithPeers() {
	for _, peerURL := range m.peers {
		go m.syncWithPeer(peerURL)
	}
}

// syncWithPeer syncs with a specific peer
func (m *Manager) syncWithPeer(peerURL string) {
	// Get the peer's current serial
	currentReq := CurrentRequest{
		NodeID:   m.nodeID,
		RelayURL: m.relayURL,
	}

	jsonData, err := json.Marshal(currentReq)
	if err != nil {
		log.E.F("failed to marshal current request: %v", err)
		return
	}

	resp, err := http.Post(peerURL+"/api/sync/current", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.D.F("failed to get current serial from %s: %v", peerURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.D.F("current request failed with %s: status %d", peerURL, resp.StatusCode)
		return
	}

	var currentResp CurrentResponse
	if err := json.NewDecoder(resp.Body).Decode(&currentResp); err != nil {
		log.E.F("failed to decode current response from %s: %v", peerURL, err)
		return
	}

	// Check if we need to sync
	peerSerial := currentResp.Serial
	ourLastSeen := m.peerSerials[peerURL]

	if peerSerial > ourLastSeen {
		// Request missing events
		m.requestEvents(peerURL, ourLastSeen+1, peerSerial)
		// Update our knowledge of peer's serial
		m.mutex.Lock()
		m.peerSerials[peerURL] = peerSerial
		m.mutex.Unlock()
	}
}

// requestEvents requests a range of events from a peer
func (m *Manager) requestEvents(peerURL string, from, to uint64) {
	req := FetchRequest{
		NodeID:   m.nodeID,
		RelayURL: m.relayURL,
		From:     from,
		To:       to,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		log.E.F("failed to marshal fetch request: %v", err)
		return
	}

	resp, err := http.Post(peerURL+"/api/sync/fetch", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.E.F("failed to request events from %s: %v", peerURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.E.F("fetch request failed with %s: status %d", peerURL, resp.StatusCode)
		return
	}

	var fetchResp FetchResponse
	if err := json.NewDecoder(resp.Body).Decode(&fetchResp); err != nil {
		log.E.F("failed to decode fetch response from %s: %v", peerURL, err)
		return
	}

	// Import the received events
	if len(fetchResp.Events) > 0 {
		if err := m.db.ImportEventsFromStrings(context.Background(), fetchResp.Events); err != nil {
			log.E.F("failed to import events from %s: %v", peerURL, err)
			return
		}
		log.I.F("imported %d events from peer %s", len(fetchResp.Events), peerURL)
	}
}

// getEventsBySerialRange retrieves events by serial range from the database as JSONL
func (m *Manager) getEventsBySerialRange(from, to uint64) ([]string, error) {
	var events []string

	// Get event serials by serial range
	serials, err := m.db.EventIdsBySerial(from, int(to-from+1))
	if err != nil {
		return nil, err
	}

	// TODO: For each serial, retrieve the actual event and marshal to JSONL
	// For now, return serial numbers as placeholder JSON strings
	for _, serial := range serials {
		// This should be replaced with actual event JSON marshalling
		events = append(events, `{"serial":`+strconv.FormatUint(serial, 10)+`}`)
	}

	return events, nil
}

// HandleCurrentRequest handles requests for current serial number
func (m *Manager) HandleCurrentRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CurrentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	resp := CurrentResponse{
		NodeID:   m.nodeID,
		RelayURL: m.relayURL,
		Serial:   m.GetCurrentSerial(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleFetchRequest handles requests for events in a serial range
func (m *Manager) HandleFetchRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get events in the requested range
	events, err := m.getEventsBySerialRange(req.From, req.To)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get events: %v", err), http.StatusInternalServerError)
		return
	}

	resp := FetchResponse{
		Events: events,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
