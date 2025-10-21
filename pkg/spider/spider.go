package spider

import (
	"context"
	"strconv"
	"strings"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/timestamp"
	"next.orly.dev/pkg/protocol/ws"
	"next.orly.dev/pkg/utils/normalize"
)

const (
	OneTimeSpiderSyncMarker = "spider_one_time_sync_completed"
	SpiderLastScanMarker    = "spider_last_scan_time"
	// MaxWebSocketMessageSize is the maximum size for WebSocket messages
	MaxWebSocketMessageSize = 100 * 1024 * 1024 // 100MB
	// PubkeyHexSize is the size of a hex-encoded pubkey (32 bytes = 64 hex chars)
	PubkeyHexSize = 64
)

type Spider struct {
	db     *database.D
	cfg    *config.C
	ctx    context.Context
	cancel context.CancelFunc
	// Configured relay addresses for self-detection
	relayAddresses []string
}

func New(
	db *database.D, cfg *config.C, ctx context.Context,
	cancel context.CancelFunc,
) *Spider {
	return &Spider{
		db:             db,
		cfg:            cfg,
		ctx:            ctx,
		cancel:         cancel,
		relayAddresses: cfg.RelayAddresses,
	}
}

// Start initializes the spider functionality based on configuration
func (s *Spider) Start() {
	if s.cfg.SpiderMode != "follows" {
		log.D.Ln("Spider mode is not set to 'follows', skipping spider functionality")
		return
	}

	log.I.Ln("Starting spider in follow mode")

	// Check if one-time sync has been completed
	if !s.db.HasMarker(OneTimeSpiderSyncMarker) {
		log.I.Ln("Performing one-time spider sync back one month")
		go s.performOneTimeSync()
	} else {
		log.D.Ln("One-time spider sync already completed, skipping")
	}

	// Start periodic scanning
	go s.startPeriodicScanning()
}

// performOneTimeSync performs the initial sync going back one month
func (s *Spider) performOneTimeSync() {
	defer func() {
		// Mark the one-time sync as completed
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		if err := s.db.SetMarker(
			OneTimeSpiderSyncMarker, []byte(timestamp),
		); err != nil {
			log.E.F("Failed to set one-time sync marker: %v", err)
		} else {
			log.I.Ln("One-time spider sync completed and marked")
		}
	}()

	// Calculate the time one month ago
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	log.I.F("Starting one-time spider sync from %v", oneMonthAgo)

	// Perform the sync (placeholder - would need actual implementation based on follows)
	if err := s.performSync(oneMonthAgo, time.Now()); err != nil {
		log.E.F("One-time spider sync failed: %v", err)
		return
	}

	log.I.Ln("One-time spider sync completed successfully")
}

// startPeriodicScanning starts the regular scanning process
func (s *Spider) startPeriodicScanning() {
	ticker := time.NewTicker(s.cfg.SpiderFrequency)
	defer ticker.Stop()

	log.I.F("Starting periodic spider scanning every %v", s.cfg.SpiderFrequency)

	for {
		select {
		case <-s.ctx.Done():
			log.D.Ln("Spider periodic scanning stopped due to context cancellation")
			return
		case <-ticker.C:
			s.performPeriodicScan()
		}
	}
}

// performPeriodicScan performs the regular scan of the last two hours (double the frequency window)
func (s *Spider) performPeriodicScan() {
	// Calculate the scanning window (double the frequency period)
	scanWindow := s.cfg.SpiderFrequency * 2
	scanStart := time.Now().Add(-scanWindow)
	scanEnd := time.Now()

	log.D.F(
		"Performing periodic spider scan from %v to %v (window: %v)", scanStart,
		scanEnd, scanWindow,
	)

	if err := s.performSync(scanStart, scanEnd); err != nil {
		log.E.F("Periodic spider scan failed: %v", err)
		return
	}

	// Update the last scan marker
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	if err := s.db.SetMarker(
		SpiderLastScanMarker, []byte(timestamp),
	); err != nil {
		log.E.F("Failed to update last scan marker: %v", err)
	}

	log.D.F("Periodic spider scan completed successfully")
}

// performSync performs the actual sync operation for the given time range
func (s *Spider) performSync(startTime, endTime time.Time) error {
	log.D.F(
		"Spider sync from %v to %v - starting implementation", startTime,
		endTime,
	)

	// 1. Check ACL mode is set to "follows"
	if s.cfg.ACLMode != "follows" {
		log.D.F(
			"Spider sync skipped - ACL mode is not 'follows' (current: %s)",
			s.cfg.ACLMode,
		)
		return nil
	}

	// 2. Get the list of followed users from the ACL system
	followedPubkeys, err := s.getFollowedPubkeys()
	if err != nil {
		return err
	}

	if len(followedPubkeys) == 0 {
		log.D.Ln("Spider sync: no followed pubkeys found")
		return nil
	}

	log.D.F("Spider sync: found %d followed pubkeys", len(followedPubkeys))

	// 3. Discover relay lists from followed users
	relayURLs, err := s.discoverRelays(followedPubkeys)
	if err != nil {
		return err
	}

	if len(relayURLs) == 0 {
		log.W.Ln("Spider sync: no relays discovered from followed users")
		return nil
	}

	log.I.F("Spider sync: discovered %d relay URLs", len(relayURLs))

	// 4. Query each relay for events from followed pubkeys in the time range
	eventsFound := 0
	for _, relayURL := range relayURLs {
		log.I.F("Spider sync: fetching follow lists from relay %s", relayURL)
		count, err := s.queryRelayForEvents(
			relayURL, followedPubkeys, startTime, endTime,
		)
		if err != nil {
			log.E.F("Spider sync: error querying relay %s: %v", relayURL, err)
			continue
		}
		log.I.F("Spider sync: completed fetching from relay %s, found %d events", relayURL, count)
		eventsFound += count
	}

	log.I.F(
		"Spider sync completed: found %d new events from %d relays",
		eventsFound, len(relayURLs),
	)

	return nil
}

// getFollowedPubkeys retrieves the list of followed pubkeys from the ACL system
func (s *Spider) getFollowedPubkeys() ([][]byte, error) {
	// Access the ACL registry to get the current ACL instance
	var followedPubkeys [][]byte

	// Get all ACL instances and find the active one
	for _, aclInstance := range acl.Registry.ACL {
		if aclInstance.Type() == acl.Registry.Active.Load() {
			// Cast to *Follows to access the follows field
			if followsACL, ok := aclInstance.(*acl.Follows); ok {
				followedPubkeys = followsACL.GetFollowedPubkeys()
				break
			}
		}
	}

	return followedPubkeys, nil
}

// discoverRelays discovers relay URLs from kind 10002 events of followed users
func (s *Spider) discoverRelays(followedPubkeys [][]byte) ([]string, error) {
	seen := make(map[string]struct{})
	var urls []string

	for _, pubkey := range followedPubkeys {
		// Query for kind 10002 (RelayListMetadata) events from this pubkey
		fl := &filter.F{
			Authors: tag.NewFromAny(pubkey),
			Kinds:   kind.NewS(kind.New(kind.RelayListMetadata.K)),
		}

		idxs, err := database.GetIndexesFromFilter(fl)
		if chk.E(err) {
			continue
		}

		var sers types.Uint40s
		for _, idx := range idxs {
			s, err := s.db.GetSerialsByRange(idx)
			if chk.E(err) {
				continue
			}
			sers = append(sers, s...)
		}

		for _, ser := range sers {
			ev, err := s.db.FetchEventBySerial(ser)
			if chk.E(err) || ev == nil {
				continue
			}

			// Extract relay URLs from 'r' tags
			for _, v := range ev.Tags.GetAll([]byte("r")) {
				u := string(v.Value())
				n := string(normalize.URL(u))
				if n == "" {
					continue
				}
				// Skip if this relay is one of the configured relay addresses
				skipRelay := false
				for _, relayAddr := range s.relayAddresses {
					if n == relayAddr {
						log.D.F("spider: skipping configured relay address: %s", n)
						skipRelay = true
						break
					}
				}
				if skipRelay {
					continue
				}
				if _, ok := seen[n]; ok {
					continue
				}
				seen[n] = struct{}{}
				urls = append(urls, n)
			}
		}
	}

	return urls, nil
}

// calculateOptimalChunkSize calculates the optimal chunk size for pubkeys to stay under message size limit
func (s *Spider) calculateOptimalChunkSize() int {
	// Estimate the size of a filter with timestamps and other fields
	// Base filter overhead: ~200 bytes for timestamps, limits, etc.
	baseFilterSize := 200

	// Calculate how many pubkeys we can fit in the remaining space
	availableSpace := MaxWebSocketMessageSize - baseFilterSize
	maxPubkeys := availableSpace / PubkeyHexSize

	// Use a conservative chunk size (80% of max to be safe)
	chunkSize := int(float64(maxPubkeys) * 0.8)

	// Ensure minimum chunk size of 10
	if chunkSize < 10 {
		chunkSize = 10
	}

	log.D.F(
		"Spider: calculated optimal chunk size: %d pubkeys (max would be %d)",
		chunkSize, maxPubkeys,
	)
	return chunkSize
}

// queryRelayForEvents connects to a relay and queries for events from followed pubkeys
func (s *Spider) queryRelayForEvents(
	relayURL string, followedPubkeys [][]byte, startTime, endTime time.Time,
) (int, error) {
	log.T.F(
		"Spider sync: querying relay %s with %d pubkeys", relayURL,
		len(followedPubkeys),
	)

	// Connect to the relay with a timeout context
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	client, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	// Break pubkeys into chunks to avoid 32KB message limit
	chunkSize := s.calculateOptimalChunkSize()
	totalEventsSaved := 0

	for i := 0; i < len(followedPubkeys); i += chunkSize {
		end := i + chunkSize
		if end > len(followedPubkeys) {
			end = len(followedPubkeys)
		}

		chunk := followedPubkeys[i:end]
		log.T.F(
			"Spider sync: processing chunk %d-%d (%d pubkeys) for relay %s",
			i, end-1, len(chunk), relayURL,
		)

		// Create filter for this chunk of pubkeys
		f := &filter.F{
			Authors: tag.NewFromBytesSlice(chunk...),
			Since:   timestamp.FromUnix(startTime.Unix()),
			Until:   timestamp.FromUnix(endTime.Unix()),
			Limit:   func() *uint { l := uint(500); return &l }(), // Limit to avoid overwhelming
		}

		// Subscribe to get events for this chunk
		sub, err := client.Subscribe(ctx, filter.NewS(f))
		if err != nil {
			log.E.F(
				"Spider sync: failed to subscribe to chunk %d-%d for relay %s: %v",
				i, end-1, relayURL, err,
			)
			continue
		}

		chunkEventsSaved := 0
		chunkEventsCount := 0
		timeout := time.After(10 * time.Second) // Timeout for receiving events

		chunkDone := false
		for !chunkDone {
			select {
			case <-ctx.Done():
				log.T.F(
					"Spider sync: context done for relay %s chunk %d-%d, saved %d/%d events",
					relayURL, i, end-1, chunkEventsSaved, chunkEventsCount,
				)
				chunkDone = true
			case <-timeout:
				log.T.F(
					"Spider sync: timeout for relay %s chunk %d-%d, saved %d/%d events",
					relayURL, i, end-1, chunkEventsSaved, chunkEventsCount,
				)
				chunkDone = true
			case <-sub.EndOfStoredEvents:
				log.T.F(
					"Spider sync: end of stored events for relay %s chunk %d-%d, saved %d/%d events",
					relayURL, i, end-1, chunkEventsSaved, chunkEventsCount,
				)
				chunkDone = true
			case ev := <-sub.Events:
				if ev == nil {
					continue
				}
				chunkEventsCount++

				// Verify the event signature
				if ok, err := ev.Verify(); !ok || err != nil {
					log.T.F(
						"Spider sync: invalid event signature from relay %s",
						relayURL,
					)
					ev.Free()
					continue
				}

				// Save the event to the database
				if _, err := s.db.SaveEvent(s.ctx, ev); err != nil {
					if !strings.HasPrefix(err.Error(), "blocked:") {
						log.T.F(
							"Spider sync: error saving event from relay %s: %v",
							relayURL, err,
						)
					}
					// Event might already exist, which is fine for deduplication
				} else {
					chunkEventsSaved++
					if chunkEventsSaved%10 == 0 {
						log.T.F(
							"Spider sync: saved %d events from relay %s chunk %d-%d",
							chunkEventsSaved, relayURL, i, end-1,
						)
					}
				}
				ev.Free()
			}
		}

		// Clean up subscription
		sub.Unsub()
		totalEventsSaved += chunkEventsSaved

		log.T.F(
			"Spider sync: completed chunk %d-%d for relay %s, saved %d events",
			i, end-1, relayURL, chunkEventsSaved,
		)
	}

	log.T.F(
		"Spider sync: completed all chunks for relay %s, total saved %d events",
		relayURL, totalEventsSaved,
	)
	return totalEventsSaved, nil
}

// Stop stops the spider functionality
func (s *Spider) Stop() {
	log.D.Ln("Stopping spider")
	s.cancel()
}
