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
)

type Spider struct {
	db     *database.D
	cfg    *config.C
	ctx    context.Context
	cancel context.CancelFunc
}

func New(
	db *database.D, cfg *config.C, ctx context.Context,
	cancel context.CancelFunc,
) *Spider {
	return &Spider{
		db:     db,
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
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
		count, err := s.queryRelayForEvents(
			relayURL, followedPubkeys, startTime, endTime,
		)
		if err != nil {
			log.E.F("Spider sync: error querying relay %s: %v", relayURL, err)
			continue
		}
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

// queryRelayForEvents connects to a relay and queries for events from followed pubkeys
func (s *Spider) queryRelayForEvents(
	relayURL string, followedPubkeys [][]byte, startTime, endTime time.Time,
) (int, error) {
	log.T.F("Spider sync: querying relay %s", relayURL)

	// Connect to the relay with a timeout context
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	client, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	// Create filter for the time range and followed pubkeys
	f := &filter.F{
		Authors: tag.NewFromBytesSlice(followedPubkeys...),
		Since:   timestamp.FromUnix(startTime.Unix()),
		Until:   timestamp.FromUnix(endTime.Unix()),
		Limit:   func() *uint { l := uint(1000); return &l }(), // Limit to avoid overwhelming
	}

	// Subscribe to get events
	sub, err := client.Subscribe(ctx, filter.NewS(f))
	if err != nil {
		return 0, err
	}
	defer sub.Unsub()

	eventsCount := 0
	eventsSaved := 0
	timeout := time.After(10 * time.Second) // Timeout for receiving events

	for {
		select {
		case <-ctx.Done():
			log.T.F(
				"Spider sync: context done for relay %s, saved %d/%d events",
				relayURL, eventsSaved, eventsCount,
			)
			return eventsSaved, nil
		case <-timeout:
			log.T.F(
				"Spider sync: timeout for relay %s, saved %d/%d events",
				relayURL, eventsSaved, eventsCount,
			)
			return eventsSaved, nil
		case <-sub.EndOfStoredEvents:
			log.T.F(
				"Spider sync: end of stored events for relay %s, saved %d/%d events",
				relayURL, eventsSaved, eventsCount,
			)
			return eventsSaved, nil
		case ev := <-sub.Events:
			if ev == nil {
				continue
			}
			eventsCount++

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
			if _, _, err := s.db.SaveEvent(s.ctx, ev); err != nil {
				if !strings.HasPrefix(err.Error(), "blocked:") {
					log.T.F(
						"Spider sync: error saving event from relay %s: %v",
						relayURL, err,
					)
				}
				// Event might already exist, which is fine for deduplication
			} else {
				eventsSaved++
				if eventsSaved%10 == 0 {
					log.T.F(
						"Spider sync: saved %d events from relay %s",
						eventsSaved, relayURL,
					)
				}
			}
			ev.Free()
		}
	}
}

// Stop stops the spider functionality
func (s *Spider) Stop() {
	log.D.Ln("Stopping spider")
	s.cancel()
}
