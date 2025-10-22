package spider

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/timestamp"
	"next.orly.dev/pkg/interfaces/publisher"
	"next.orly.dev/pkg/protocol/ws"
)

const (
	// BatchSize is the number of pubkeys per subscription batch
	BatchSize = 20
	// CatchupWindow is the extra time added to disconnection periods for catch-up
	CatchupWindow = 30 * time.Minute
	// ReconnectDelay is the delay between reconnection attempts
	ReconnectDelay = 5 * time.Second
	// MaxReconnectDelay is the maximum delay between reconnection attempts
	MaxReconnectDelay = 5 * time.Minute
)

// Spider manages connections to admin relays and syncs events for followed pubkeys
type Spider struct {
	ctx    context.Context
	cancel context.CancelFunc
	db     *database.D
	pub    publisher.I
	mode   string

	// Configuration
	adminRelays []string
	followList  [][]byte

	// State management
	mu          sync.RWMutex
	connections map[string]*RelayConnection
	running     bool

	// Callbacks for getting updated data
	getAdminRelays func() []string
	getFollowList  func() [][]byte
}

// RelayConnection manages a single relay connection and its subscriptions
type RelayConnection struct {
	url    string
	client *ws.Client
	ctx    context.Context
	cancel context.CancelFunc
	spider *Spider

	// Subscription management
	mu            sync.RWMutex
	subscriptions map[string]*BatchSubscription

	// Disconnection tracking
	lastDisconnect time.Time
	reconnectDelay time.Duration
}

// BatchSubscription represents a subscription for a batch of pubkeys
type BatchSubscription struct {
	id        string
	pubkeys   [][]byte
	startTime time.Time
	sub       *ws.Subscription
	relay     *RelayConnection

	// Track disconnection periods for catch-up
	disconnectedAt *time.Time
}

// DisconnectionPeriod tracks when a subscription was disconnected
type DisconnectionPeriod struct {
	Start time.Time
	End   time.Time
}

// New creates a new Spider instance
func New(ctx context.Context, db *database.D, pub publisher.I, mode string) (s *Spider, err error) {
	if db == nil {
		err = errorf.E("database cannot be nil")
		return
	}

	// Validate mode
	switch mode {
	case "follows", "none":
		// Valid modes
	default:
		err = errorf.E("invalid spider mode: %s (valid modes: none, follows)", mode)
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	s = &Spider{
		ctx:         ctx,
		cancel:      cancel,
		db:          db,
		pub:         pub,
		mode:        mode,
		connections: make(map[string]*RelayConnection),
	}

	return
}

// SetCallbacks sets the callback functions for getting updated admin relays and follow lists
func (s *Spider) SetCallbacks(getAdminRelays func() []string, getFollowList func() [][]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getAdminRelays = getAdminRelays
	s.getFollowList = getFollowList
}

// Start begins the spider operation
func (s *Spider) Start() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		err = errorf.E("spider already running")
		return
	}

	// Handle 'none' mode - no-op
	if s.mode == "none" {
		log.I.F("spider: mode is 'none', not starting")
		return
	}

	if s.getAdminRelays == nil || s.getFollowList == nil {
		err = errorf.E("callbacks must be set before starting")
		return
	}

	s.running = true

	// Start the main loop
	go s.mainLoop()

	log.I.F("spider: started in '%s' mode", s.mode)
	return
}

// Stop stops the spider operation
func (s *Spider) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	s.cancel()

	// Close all connections
	for _, conn := range s.connections {
		conn.close()
	}
	s.connections = make(map[string]*RelayConnection)

	log.I.F("spider: stopped")
}

// mainLoop is the main spider loop that manages connections and subscriptions
func (s *Spider) mainLoop() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.updateConnections()
		}
	}
}

// updateConnections updates relay connections based on current admin relays and follow lists
func (s *Spider) updateConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	// Get current admin relays and follow list
	adminRelays := s.getAdminRelays()
	followList := s.getFollowList()

	if len(adminRelays) == 0 || len(followList) == 0 {
		log.D.F("spider: no admin relays (%d) or follow list (%d) available",
			len(adminRelays), len(followList))
		return
	}

	// Update connections for current admin relays
	currentRelays := make(map[string]bool)
	for _, url := range adminRelays {
		currentRelays[url] = true

		if conn, exists := s.connections[url]; exists {
			// Update existing connection
			conn.updateSubscriptions(followList)
		} else {
			// Create new connection
			s.createConnection(url, followList)
		}
	}

	// Remove connections for relays no longer in admin list
	for url, conn := range s.connections {
		if !currentRelays[url] {
			log.I.F("spider: removing connection to %s (no longer in admin relays)", url)
			conn.close()
			delete(s.connections, url)
		}
	}
}

// createConnection creates a new relay connection
func (s *Spider) createConnection(url string, followList [][]byte) {
	log.I.F("spider: creating connection to %s", url)

	ctx, cancel := context.WithCancel(s.ctx)
	conn := &RelayConnection{
		url:            url,
		ctx:            ctx,
		cancel:         cancel,
		spider:         s,
		subscriptions:  make(map[string]*BatchSubscription),
		reconnectDelay: ReconnectDelay,
	}

	s.connections[url] = conn

	// Start connection in goroutine
	go conn.manage(followList)
}

// manage handles the lifecycle of a relay connection
func (rc *RelayConnection) manage(followList [][]byte) {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		// Attempt to connect
		if err := rc.connect(); chk.E(err) {
			log.W.F("spider: failed to connect to %s: %v", rc.url, err)
			rc.waitBeforeReconnect()
			continue
		}

		log.I.F("spider: connected to %s", rc.url)
		rc.reconnectDelay = ReconnectDelay // Reset delay on successful connection

		// Create subscriptions for follow list
		rc.createSubscriptions(followList)

		// Wait for disconnection
		<-rc.client.Context().Done()

		log.W.F("spider: disconnected from %s: %v", rc.url, rc.client.ConnectionCause())
		rc.handleDisconnection()

		// Clean up
		rc.client = nil
		rc.clearSubscriptions()
	}
}

// connect establishes a websocket connection to the relay
func (rc *RelayConnection) connect() (err error) {
	connectCtx, cancel := context.WithTimeout(rc.ctx, 10*time.Second)
	defer cancel()

	if rc.client, err = ws.RelayConnect(connectCtx, rc.url); chk.E(err) {
		return
	}

	return
}

// waitBeforeReconnect waits before attempting to reconnect with exponential backoff
func (rc *RelayConnection) waitBeforeReconnect() {
	select {
	case <-rc.ctx.Done():
		return
	case <-time.After(rc.reconnectDelay):
	}

	// Exponential backoff
	rc.reconnectDelay *= 2
	if rc.reconnectDelay > MaxReconnectDelay {
		rc.reconnectDelay = MaxReconnectDelay
	}
}

// handleDisconnection records disconnection time for catch-up logic
func (rc *RelayConnection) handleDisconnection() {
	now := time.Now()
	rc.lastDisconnect = now

	// Mark all subscriptions as disconnected
	rc.mu.Lock()
	defer rc.mu.Unlock()

	for _, sub := range rc.subscriptions {
		if sub.disconnectedAt == nil {
			sub.disconnectedAt = &now
		}
	}
}

// createSubscriptions creates batch subscriptions for the follow list
func (rc *RelayConnection) createSubscriptions(followList [][]byte) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Clear existing subscriptions
	rc.clearSubscriptionsLocked()

	// Create batches of pubkeys
	batches := rc.createBatches(followList)

	log.I.F("spider: creating %d subscription batches for %d pubkeys on %s",
		len(batches), len(followList), rc.url)

	for i, batch := range batches {
		batchID := fmt.Sprintf("batch-%d", i) // Simple batch ID
		rc.createBatchSubscription(batchID, batch)
	}
}

// createBatches splits the follow list into batches of BatchSize
func (rc *RelayConnection) createBatches(followList [][]byte) (batches [][][]byte) {
	for i := 0; i < len(followList); i += BatchSize {
		end := i + BatchSize
		if end > len(followList) {
			end = len(followList)
		}

		batch := make([][]byte, end-i)
		copy(batch, followList[i:end])
		batches = append(batches, batch)
	}
	return
}

// createBatchSubscription creates a subscription for a batch of pubkeys
func (rc *RelayConnection) createBatchSubscription(batchID string, pubkeys [][]byte) {
	if rc.client == nil {
		return
	}

	// Create filters: one for authors, one for p tags
	var pTags tag.S
	for _, pk := range pubkeys {
		pTags = append(pTags, tag.NewFromAny("p", pk))
	}

	filters := filter.NewS(
		&filter.F{
			Authors: tag.NewFromBytesSlice(pubkeys...),
		},
		&filter.F{
			Tags: tag.NewS(pTags...),
		},
	)

	// Subscribe
	sub, err := rc.client.Subscribe(rc.ctx, filters)
	if chk.E(err) {
		log.E.F("spider: failed to create subscription %s on %s: %v", batchID, rc.url, err)
		return
	}

	batchSub := &BatchSubscription{
		id:        batchID,
		pubkeys:   pubkeys,
		startTime: time.Now(),
		sub:       sub,
		relay:     rc,
	}

	rc.subscriptions[batchID] = batchSub

	// Start event handler
	go batchSub.handleEvents()

	log.D.F("spider: created subscription %s for %d pubkeys on %s",
		batchID, len(pubkeys), rc.url)
}

// handleEvents processes events from the subscription
func (bs *BatchSubscription) handleEvents() {
	for {
		select {
		case <-bs.relay.ctx.Done():
			return
		case ev := <-bs.sub.Events:
			if ev == nil {
				return // Subscription closed
			}

			// Save event to database
			if _, err := bs.relay.spider.db.SaveEvent(bs.relay.ctx, ev); err != nil {
				if !chk.E(err) {
					log.T.F("spider: saved event %s from %s",
						hex.EncodeToString(ev.ID[:]), bs.relay.url)
				}
			} else {
				// Publish event if it was newly saved
				if bs.relay.spider.pub != nil {
					go bs.relay.spider.pub.Deliver(ev)
				}
			}
		}
	}
}

// updateSubscriptions updates subscriptions for a connection with new follow list
func (rc *RelayConnection) updateSubscriptions(followList [][]byte) {
	if rc.client == nil || !rc.client.IsConnected() {
		return // Will be handled on reconnection
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Check if we need to perform catch-up for disconnected subscriptions
	now := time.Now()
	needsCatchup := false

	for _, sub := range rc.subscriptions {
		if sub.disconnectedAt != nil {
			needsCatchup = true
			rc.performCatchup(sub, *sub.disconnectedAt, now, followList)
			sub.disconnectedAt = nil // Clear disconnection marker
		}
	}

	if needsCatchup {
		log.I.F("spider: performed catch-up for disconnected subscriptions on %s", rc.url)
	}

	// Recreate subscriptions with updated follow list
	rc.clearSubscriptionsLocked()

	batches := rc.createBatches(followList)
	for i, batch := range batches {
		batchID := fmt.Sprintf("batch-%d", i)
		rc.createBatchSubscription(batchID, batch)
	}
}

// performCatchup queries for events missed during disconnection
func (rc *RelayConnection) performCatchup(sub *BatchSubscription, disconnectTime, reconnectTime time.Time, followList [][]byte) {
	// Expand time window by CatchupWindow on both sides
	since := disconnectTime.Add(-CatchupWindow)
	until := reconnectTime.Add(CatchupWindow)

	log.I.F("spider: performing catch-up for %s from %v to %v (expanded window)",
		rc.url, since, until)

	// Create catch-up filters with time constraints
	sinceTs := timestamp.T{V: since.Unix()}
	untilTs := timestamp.T{V: until.Unix()}

	var pTags tag.S
	for _, pk := range sub.pubkeys {
		pTags = append(pTags, tag.NewFromAny("p", pk))
	}

	filters := filter.NewS(
		&filter.F{
			Authors: tag.NewFromBytesSlice(sub.pubkeys...),
			Since:   &sinceTs,
			Until:   &untilTs,
		},
		&filter.F{
			Tags:  tag.NewS(pTags...),
			Since: &sinceTs,
			Until: &untilTs,
		},
	)

	// Create temporary subscription for catch-up
	catchupCtx, cancel := context.WithTimeout(rc.ctx, 30*time.Second)
	defer cancel()

	catchupSub, err := rc.client.Subscribe(catchupCtx, filters)
	if chk.E(err) {
		log.E.F("spider: failed to create catch-up subscription on %s: %v", rc.url, err)
		return
	}
	defer catchupSub.Unsub()

	// Process catch-up events
	eventCount := 0
	timeout := time.After(30 * time.Second)

	for {
		select {
		case <-catchupCtx.Done():
			log.D.F("spider: catch-up completed on %s, processed %d events", rc.url, eventCount)
			return
		case <-timeout:
			log.D.F("spider: catch-up timeout on %s, processed %d events", rc.url, eventCount)
			return
		case <-catchupSub.EndOfStoredEvents:
			log.D.F("spider: catch-up EOSE on %s, processed %d events", rc.url, eventCount)
			return
		case ev := <-catchupSub.Events:
			if ev == nil {
				return
			}

			eventCount++

			// Save event to database
			if _, err := rc.spider.db.SaveEvent(rc.ctx, ev); err != nil {
				if !chk.E(err) {
					log.T.F("spider: catch-up saved event %s from %s",
						hex.EncodeToString(ev.ID[:]), rc.url)
				}
			} else {
				// Publish event if it was newly saved
				if rc.spider.pub != nil {
					go rc.spider.pub.Deliver(ev)
				}
			}
		}
	}
}

// clearSubscriptions clears all subscriptions (with lock)
func (rc *RelayConnection) clearSubscriptions() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.clearSubscriptionsLocked()
}

// clearSubscriptionsLocked clears all subscriptions (without lock)
func (rc *RelayConnection) clearSubscriptionsLocked() {
	for _, sub := range rc.subscriptions {
		if sub.sub != nil {
			sub.sub.Unsub()
		}
	}
	rc.subscriptions = make(map[string]*BatchSubscription)
}

// close closes the relay connection
func (rc *RelayConnection) close() {
	rc.clearSubscriptions()

	if rc.client != nil {
		rc.client.Close()
		rc.client = nil
	}

	rc.cancel()
}
