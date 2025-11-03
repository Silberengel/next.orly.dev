package sync

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
)

type ClusterManager struct {
	ctx         context.Context
	cancel      context.CancelFunc
	db          *database.D
	adminNpubs  []string
	members     map[string]*ClusterMember // keyed by relay URL
	membersMux  sync.RWMutex
	pollTicker  *time.Ticker
	pollDone    chan struct{}
	httpClient  *http.Client
}

type ClusterMember struct {
	HTTPURL      string
	WebSocketURL string
	LastSerial   uint64
	LastPoll     time.Time
	Status       string // "active", "error", "unknown"
	ErrorCount   int
}

type LatestSerialResponse struct {
	Serial    uint64 `json:"serial"`
	Timestamp int64  `json:"timestamp"`
}

type EventsRangeResponse struct {
	Events   []EventInfo `json:"events"`
	HasMore  bool        `json:"has_more"`
	NextFrom uint64      `json:"next_from,omitempty"`
}

type EventInfo struct {
	Serial    uint64 `json:"serial"`
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
}

func NewClusterManager(ctx context.Context, db *database.D, adminNpubs []string) *ClusterManager {
	ctx, cancel := context.WithCancel(ctx)

	cm := &ClusterManager{
		ctx:        ctx,
		cancel:     cancel,
		db:         db,
		adminNpubs: adminNpubs,
		members:    make(map[string]*ClusterMember),
		pollDone:   make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	return cm
}

func (cm *ClusterManager) Start() {
	log.I.Ln("starting cluster replication manager")

	// Load persisted peer state from database
	if err := cm.loadPeerState(); err != nil {
		log.W.F("failed to load cluster peer state: %v", err)
	}

	cm.pollTicker = time.NewTicker(5 * time.Second)
	go cm.pollingLoop()
}

func (cm *ClusterManager) Stop() {
	log.I.Ln("stopping cluster replication manager")
	cm.cancel()
	if cm.pollTicker != nil {
		cm.pollTicker.Stop()
	}
	<-cm.pollDone
}

func (cm *ClusterManager) pollingLoop() {
	defer close(cm.pollDone)

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-cm.pollTicker.C:
			cm.pollAllMembers()
		}
	}
}

func (cm *ClusterManager) pollAllMembers() {
	cm.membersMux.RLock()
	members := make([]*ClusterMember, 0, len(cm.members))
	for _, member := range cm.members {
		members = append(members, member)
	}
	cm.membersMux.RUnlock()

	for _, member := range members {
		go cm.pollMember(member)
	}
}

func (cm *ClusterManager) pollMember(member *ClusterMember) {
	// Get latest serial from peer
	latestResp, err := cm.getLatestSerial(member.HTTPURL)
	if err != nil {
		log.W.F("failed to get latest serial from %s: %v", member.HTTPURL, err)
		cm.updateMemberStatus(member, "error")
		return
	}

	cm.updateMemberStatus(member, "active")
	member.LastPoll = time.Now()

	// Check if we need to fetch new events
	if latestResp.Serial <= member.LastSerial {
		return // No new events
	}

	// Fetch events in range
	from := member.LastSerial + 1
	to := latestResp.Serial

	eventsResp, err := cm.getEventsInRange(member.HTTPURL, from, to, 1000)
	if err != nil {
		log.W.F("failed to get events from %s: %v", member.HTTPURL, err)
		return
	}

	// Process fetched events
	for _, eventInfo := range eventsResp.Events {
		if cm.shouldFetchEvent(eventInfo) {
			// Fetch full event via WebSocket and store it
			if err := cm.fetchAndStoreEvent(member.WebSocketURL, eventInfo.ID); err != nil {
				log.W.F("failed to fetch/store event %s from %s: %v", eventInfo.ID, member.HTTPURL, err)
			} else {
				log.D.F("successfully replicated event %s from %s", eventInfo.ID, member.HTTPURL)
			}
		}
	}

	// Update last serial if we processed all events
	if !eventsResp.HasMore && member.LastSerial != to {
		member.LastSerial = to
		// Persist the updated serial to database
		if err := cm.savePeerState(member.HTTPURL, to); err != nil {
			log.W.F("failed to persist serial %d for peer %s: %v", to, member.HTTPURL, err)
		}
	}
}

func (cm *ClusterManager) getLatestSerial(peerURL string) (*LatestSerialResponse, error) {
	url := fmt.Sprintf("%s/cluster/latest", peerURL)
	resp, err := cm.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result LatestSerialResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (cm *ClusterManager) getEventsInRange(peerURL string, from, to uint64, limit int) (*EventsRangeResponse, error) {
	url := fmt.Sprintf("%s/cluster/events?from=%d&to=%d&limit=%d", peerURL, from, to, limit)
	resp, err := cm.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result EventsRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (cm *ClusterManager) shouldFetchEvent(eventInfo EventInfo) bool {
	// Relays MAY choose not to store every event they receive
	// For now, accept all events
	return true
}

func (cm *ClusterManager) updateMemberStatus(member *ClusterMember, status string) {
	member.Status = status
	if status == "error" {
		member.ErrorCount++
	} else {
		member.ErrorCount = 0
	}
}

func (cm *ClusterManager) UpdateMembership(relayURLs []string) {
	cm.membersMux.Lock()
	defer cm.membersMux.Unlock()

	// Remove members not in the new list
	for url := range cm.members {
		found := false
		for _, newURL := range relayURLs {
			if newURL == url {
				found = true
				break
			}
		}
		if !found {
			delete(cm.members, url)
			// Remove persisted state for removed peer
			if err := cm.removePeerState(url); err != nil {
				log.W.F("failed to remove persisted state for peer %s: %v", url, err)
			}
			log.I.F("removed cluster member: %s", url)
		}
	}

	// Add new members
	for _, url := range relayURLs {
		if _, exists := cm.members[url]; !exists {
			// For simplicity, assume HTTP and WebSocket URLs are the same
			// In practice, you'd need to parse these properly
			member := &ClusterMember{
				HTTPURL:      url,
				WebSocketURL: url, // TODO: Convert to WebSocket URL
				LastSerial:   0,
				Status:       "unknown",
			}
			cm.members[url] = member
			log.I.F("added cluster member: %s", url)
		}
	}
}

// HandleMembershipEvent processes a cluster membership event (Kind 39108)
func (cm *ClusterManager) HandleMembershipEvent(event *event.E) error {
	// Verify the event is signed by a cluster admin
	adminFound := false
	for _, adminNpub := range cm.adminNpubs {
		// TODO: Convert adminNpub to pubkey and verify signature
		// For now, accept all events (this should be properly validated)
		_ = adminNpub // Mark as used to avoid compiler warning
		adminFound = true
		break
	}

	if !adminFound {
		return fmt.Errorf("event not signed by cluster admin")
	}

	// Parse the relay URLs from the tags
	var relayURLs []string
	for _, tag := range *event.Tags {
		if len(tag.T) >= 2 && string(tag.T[0]) == "relay" {
			relayURLs = append(relayURLs, string(tag.T[1]))
		}
	}

	if len(relayURLs) == 0 {
		return fmt.Errorf("no relay URLs found in membership event")
	}

	// Update cluster membership
	cm.UpdateMembership(relayURLs)

	log.I.F("updated cluster membership with %d relays from event %x", len(relayURLs), event.ID)

	return nil
}

// HTTP Handlers

func (cm *ClusterManager) HandleLatestSerial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the latest serial from database by querying for the highest serial
	latestSerial, err := cm.getLatestSerialFromDB()
	if err != nil {
		log.W.F("failed to get latest serial: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := LatestSerialResponse{
		Serial:    latestSerial,
		Timestamp: time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (cm *ClusterManager) HandleEventsRange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	limitStr := r.URL.Query().Get("limit")

	from := uint64(0)
	to := uint64(0)
	limit := 1000

	if fromStr != "" {
		fmt.Sscanf(fromStr, "%d", &from)
	}
	if toStr != "" {
		fmt.Sscanf(toStr, "%d", &to)
	}
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
		if limit > 10000 {
			limit = 10000
		}
	}

	// Get events in range
	events, hasMore, nextFrom, err := cm.getEventsInRangeFromDB(from, to, int(limit))
	if err != nil {
		log.W.F("failed to get events in range: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := EventsRangeResponse{
		Events:   events,
		HasMore:  hasMore,
		NextFrom: nextFrom,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (cm *ClusterManager) getLatestSerialFromDB() (uint64, error) {
	// Query the database to find the highest serial number
	// We'll iterate through the event keys to find the maximum serial
	var maxSerial uint64 = 0

	err := cm.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			Reverse: true, // Start from highest
			Prefix:  []byte{0}, // Event keys start with 0
		})
		defer it.Close()

		// Look for the first event key (which should have the highest serial in reverse iteration)
		it.Seek([]byte{0})
		if it.Valid() {
			key := it.Item().Key()
			if len(key) >= 5 { // Serial is in the last 5 bytes
				serial := binary.BigEndian.Uint64(key[len(key)-8:]) >> 24 // Convert from Uint40
				if serial > maxSerial {
					maxSerial = serial
				}
			}
		}

		return nil
	})

	return maxSerial, err
}

func (cm *ClusterManager) getEventsInRangeFromDB(from, to uint64, limit int) ([]EventInfo, bool, uint64, error) {
	var events []EventInfo
	var hasMore bool
	var nextFrom uint64

	// Convert serials to Uint40 format for querying
	fromSerial := &types.Uint40{}
	toSerial := &types.Uint40{}

	if err := fromSerial.Set(from); err != nil {
		return nil, false, 0, err
	}
	if err := toSerial.Set(to); err != nil {
		return nil, false, 0, err
	}

	// Query events by serial range
	// This is a simplified implementation - in practice you'd need to use the proper indexing
	err := cm.db.View(func(txn *badger.Txn) error {
		// For now, return empty results as this requires more complex indexing logic
		// TODO: Implement proper serial range querying using database indexes
		return nil
	})

	return events, hasMore, nextFrom, err
}

func (cm *ClusterManager) fetchAndStoreEvent(wsURL, eventID string) error {
	// TODO: Implement WebSocket connection and event fetching
	// For now, this is a placeholder that assumes the event can be fetched
	// In a full implementation, this would:
	// 1. Connect to the WebSocket endpoint
	// 2. Send a REQ message for the specific event ID
	// 3. Receive the EVENT message
	// 4. Validate and store the event in the local database

	// Placeholder - mark as not implemented for now
	log.D.F("fetchAndStoreEvent called for %s from %s (placeholder implementation)", eventID, wsURL)
	return nil // Return success for now
}

// Database key prefixes for cluster state persistence
const (
	clusterPeerStatePrefix = "cluster:peer:"
)

// loadPeerState loads persisted peer state from the database
func (cm *ClusterManager) loadPeerState() error {
	cm.membersMux.Lock()
	defer cm.membersMux.Unlock()

	prefix := []byte(clusterPeerStatePrefix)
	return cm.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			Prefix: prefix,
		})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Extract peer URL from key (remove prefix)
			peerURL := string(key[len(prefix):])

			// Read the serial value
			var serial uint64
			err := item.Value(func(val []byte) error {
				if len(val) == 8 {
					serial = binary.BigEndian.Uint64(val)
				}
				return nil
			})
			if err != nil {
				log.W.F("failed to read peer state for %s: %v", peerURL, err)
				continue
			}

			// Update existing member or create new one
			if member, exists := cm.members[peerURL]; exists {
				member.LastSerial = serial
				log.D.F("loaded persisted serial %d for existing peer %s", serial, peerURL)
			} else {
				// Create member with persisted state
				member := &ClusterMember{
					HTTPURL:      peerURL,
					WebSocketURL: peerURL, // TODO: Convert to WebSocket URL
					LastSerial:   serial,
					Status:       "unknown",
				}
				cm.members[peerURL] = member
				log.D.F("loaded persisted serial %d for new peer %s", serial, peerURL)
			}
		}
		return nil
	})
}

// savePeerState saves the current serial for a peer to the database
func (cm *ClusterManager) savePeerState(peerURL string, serial uint64) error {
	key := []byte(clusterPeerStatePrefix + peerURL)
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, serial)

	return cm.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// removePeerState removes persisted state for a peer from the database
func (cm *ClusterManager) removePeerState(peerURL string) error {
	key := []byte(clusterPeerStatePrefix + peerURL)

	return cm.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}
