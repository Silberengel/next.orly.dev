package acl

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/bech32encoding"
	"next.orly.dev/pkg/encoders/envelopes"
	"next.orly.dev/pkg/encoders/envelopes/eoseenvelope"
	"next.orly.dev/pkg/encoders/envelopes/eventenvelope"
	"next.orly.dev/pkg/encoders/envelopes/reqenvelope"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/timestamp"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/utils"
	"next.orly.dev/pkg/utils/normalize"
	"next.orly.dev/pkg/utils/values"
)

type Follows struct {
	Ctx context.Context
	cfg *config.C
	*database.D
	pubs       *publish.S
	followsMx  sync.RWMutex
	admins     [][]byte
	owners     [][]byte
	follows    [][]byte
	updated    chan struct{}
	subsCancel context.CancelFunc
	// Track last follow list fetch time
	lastFollowListFetch time.Time
}

func (f *Follows) Configure(cfg ...any) (err error) {
	log.I.F("configuring follows ACL")
	for _, ca := range cfg {
		switch c := ca.(type) {
		case *config.C:
			// log.D.F("setting ACL config: %v", c)
			f.cfg = c
		case *database.D:
			// log.D.F("setting ACL database: %s", c.Path())
			f.D = c
		case context.Context:
			// log.D.F("setting ACL context: %s", c.Value("id"))
			f.Ctx = c
		case *publish.S:
			// set publisher for dispatching new events
			f.pubs = c
		default:
			err = errorf.E("invalid type: %T", reflect.TypeOf(ca))
		}
	}
	if f.cfg == nil || f.D == nil {
		err = errorf.E("both config and database must be set")
		return
	}
	// add owners list
	for _, owner := range f.cfg.Owners {
		var own []byte
		if o, e := bech32encoding.NpubOrHexToPublicKeyBinary(owner); chk.E(e) {
			continue
		} else {
			own = o
		}
		f.owners = append(f.owners, own)
	}
	// find admin follow lists
	f.followsMx.Lock()
	defer f.followsMx.Unlock()
	// log.I.F("finding admins")
	f.follows, f.admins = nil, nil
	for _, admin := range f.cfg.Admins {
		// log.I.F("%s", admin)
		var adm []byte
		if a, e := bech32encoding.NpubOrHexToPublicKeyBinary(admin); chk.E(e) {
			continue
		} else {
			adm = a
		}
		// log.I.F("admin: %0x", adm)
		f.admins = append(f.admins, adm)
		fl := &filter.F{
			Authors: tag.NewFromAny(adm),
			Kinds:   kind.NewS(kind.New(kind.FollowList.K)),
		}
		var idxs []database.Range
		if idxs, err = database.GetIndexesFromFilter(fl); chk.E(err) {
			return
		}
		var sers types.Uint40s
		for _, idx := range idxs {
			var s types.Uint40s
			if s, err = f.D.GetSerialsByRange(idx); chk.E(err) {
				continue
			}
			sers = append(sers, s...)
		}
		if len(sers) > 0 {
			for _, s := range sers {
				var ev *event.E
				if ev, err = f.D.FetchEventBySerial(s); chk.E(err) {
					continue
				}
				// log.I.F("admin follow list:\n%s", ev.Serialize())
				for _, v := range ev.Tags.GetAll([]byte("p")) {
					// log.I.F("adding follow: %s", v.Value())
					var a []byte
					if b, e := hex.DecodeString(string(v.Value())); chk.E(e) {
						continue
					} else {
						a = b
					}
					f.follows = append(f.follows, a)
				}
			}
		}
	}
	if f.updated == nil {
		f.updated = make(chan struct{})
	} else {
		f.updated <- struct{}{}
	}
	return
}

func (f *Follows) GetAccessLevel(pub []byte, address string) (level string) {
	f.followsMx.RLock()
	defer f.followsMx.RUnlock()
	for _, v := range f.owners {
		if utils.FastEqual(v, pub) {
			return "owner"
		}
	}
	for _, v := range f.admins {
		if utils.FastEqual(v, pub) {
			return "admin"
		}
	}
	for _, v := range f.follows {
		if utils.FastEqual(v, pub) {
			return "write"
		}
	}
	if f.cfg == nil {
		return "write"
	}
	return "read"
}

func (f *Follows) GetACLInfo() (name, description, documentation string) {
	return "follows", "whitelist follows of admins",
		`This ACL mode searches for follow lists of admins and grants all followers write access`
}

func (f *Follows) Type() string { return "follows" }

func (f *Follows) adminRelays() (urls []string) {
	f.followsMx.RLock()
	admins := make([][]byte, len(f.admins))
	copy(admins, f.admins)
	f.followsMx.RUnlock()
	seen := make(map[string]struct{})

	// First, try to get relay URLs from admin kind 10002 events
	for _, adm := range admins {
		fl := &filter.F{
			Authors: tag.NewFromAny(adm),
			Kinds:   kind.NewS(kind.New(kind.RelayListMetadata.K)),
		}
		idxs, err := database.GetIndexesFromFilter(fl)
		if chk.E(err) {
			continue
		}
		var sers types.Uint40s
		for _, idx := range idxs {
			s, err := f.D.GetSerialsByRange(idx)
			if chk.E(err) {
				continue
			}
			sers = append(sers, s...)
		}
		for _, s := range sers {
			ev, err := f.D.FetchEventBySerial(s)
			if chk.E(err) || ev == nil {
				continue
			}
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

	// If no admin relays found, use bootstrap relays as fallback
	if len(urls) == 0 {
		log.I.F("no admin relays found in DB, checking bootstrap relays")
		if len(f.cfg.BootstrapRelays) > 0 {
			log.I.F("using bootstrap relays: %v", f.cfg.BootstrapRelays)
			for _, relay := range f.cfg.BootstrapRelays {
				n := string(normalize.URL(relay))
				if n == "" {
					log.W.F("invalid bootstrap relay URL: %s", relay)
					continue
				}
				if _, ok := seen[n]; ok {
					continue
				}
				seen[n] = struct{}{}
				urls = append(urls, n)
			}
		} else {
			log.W.F("no bootstrap relays configured")
		}
	}

	return
}

func (f *Follows) startEventSubscriptions(ctx context.Context) {
	// build authors list: admins + follows
	f.followsMx.RLock()
	authors := make([][]byte, 0, len(f.admins)+len(f.follows))
	authors = append(authors, f.admins...)
	authors = append(authors, f.follows...)
	f.followsMx.RUnlock()
	if len(authors) == 0 {
		log.W.F("follows syncer: no authors (admins+follows) to subscribe to")
		return
	}
	urls := f.adminRelays()
	// log.I.S(urls)
	if len(urls) == 0 {
		log.W.F("follows syncer: no admin relays found in DB (kind 10002) and no bootstrap relays configured")
		return
	}
	log.I.F(
		"follows syncer: subscribing to %d relays for %d authors", len(urls),
		len(authors),
	)
	log.I.F("follows syncer: starting follow list fetching from relays: %v", urls)
	for _, u := range urls {
		u := u
		go func() {
			backoff := time.Second
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				// Create a timeout context for the connection
				connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

				// Create proper headers for the WebSocket connection
				headers := http.Header{}
				headers.Set("User-Agent", "ORLY-Relay/0.9.2")
				headers.Set("Origin", "https://orly.dev")

				// Use proper WebSocket dial options
				dialOptions := &websocket.DialOptions{
					HTTPHeader: headers,
				}

				c, _, err := websocket.Dial(connCtx, u, dialOptions)
				cancel()
				if err != nil {
					log.W.F("follows syncer: dial %s failed: %v", u, err)

					// Handle different types of errors
					if strings.Contains(
						err.Error(), "response status code 101 but got 403",
					) {
						// 403 means the relay is not accepting connections from us
						// Forbidden is the meaning, usually used to indicate either the IP or user is blocked
						// But we should still retry after a longer delay
						log.W.F(
							"follows syncer: relay %s returned 403, will retry after longer delay",
							u,
						)
						timer := time.NewTimer(5 * time.Minute) // Wait 5 minutes before retrying 403 errors
						select {
						case <-ctx.Done():
							return
						case <-timer.C:
						}
						continue
					} else if strings.Contains(
						err.Error(), "timeout",
					) || strings.Contains(err.Error(), "connection refused") {
						// Network issues, retry with normal backoff
						log.W.F(
							"follows syncer: network issue with %s, retrying in %v",
							u, backoff,
						)
					} else {
						// Other errors, retry with normal backoff
						log.W.F(
							"follows syncer: connection error with %s, retrying in %v",
							u, backoff,
						)
					}

					timer := time.NewTimer(backoff)
					select {
					case <-ctx.Done():
						return
					case <-timer.C:
					}
					if backoff < 30*time.Second {
						backoff *= 2
					}
					continue
				}
				backoff = time.Second
				log.T.F("follows syncer: successfully connected to %s", u)
				log.I.F("follows syncer: subscribing to events from relay %s", u)

				// send REQ for admin follow lists, relay lists, and all events from follows
				ff := &filter.S{}
				// Add filter for admin follow lists (kind 3) - for immediate updates
				f1 := &filter.F{
					Authors: tag.NewFromBytesSlice(f.admins...),
					Kinds:   kind.NewS(kind.New(kind.FollowList.K)),
					Limit:   values.ToUintPointer(100),
				}
				f2 := &filter.F{
					Authors: tag.NewFromBytesSlice(authors...),
					Kinds:   kind.NewS(kind.New(kind.RelayListMetadata.K)),
					Limit:   values.ToUintPointer(100),
				}
				// Add filter for all events from follows (last 30 days)
				oneMonthAgo := timestamp.FromUnix(time.Now().Add(-30 * 24 * time.Hour).Unix())
				f3 := &filter.F{
					Authors: tag.NewFromBytesSlice(authors...),
					Since:   oneMonthAgo,
					Limit:   values.ToUintPointer(1000),
				}
				*ff = append(*ff, f1, f2, f3)
				// Use a subscription ID for event sync (no follow lists)
				subID := "event-sync"
				req := reqenvelope.NewFrom([]byte(subID), ff)
				if err = c.Write(
					ctx, websocket.MessageText, req.Marshal(nil),
				); chk.E(err) {
					log.W.F(
						"follows syncer: failed to send event REQ to %s: %v", u, err,
					)
					_ = c.Close(websocket.StatusInternalError, "write failed")
					continue
				}
				log.T.F(
					"follows syncer: sent event REQ to %s for admin follow lists, kind 10002, and all events (last 30 days) from followed users",
					u,
				)
				// read loop with keepalive
				keepaliveTicker := time.NewTicker(30 * time.Second)
				defer keepaliveTicker.Stop()

				for {
					select {
					case <-ctx.Done():
						_ = c.Close(websocket.StatusNormalClosure, "ctx done")
						return
					case <-keepaliveTicker.C:
						// Send ping to keep connection alive
						if err := c.Ping(ctx); err != nil {
							log.T.F("follows syncer: ping failed for %s: %v", u, err)
							break
						}
						log.T.F("follows syncer: sent ping to %s", u)
						continue
					default:
						// Set a read timeout to avoid hanging
						readCtx, readCancel := context.WithTimeout(ctx, 60*time.Second)
						_, data, err := c.Read(readCtx)
						readCancel()
						if err != nil {
							_ = c.Close(websocket.StatusNormalClosure, "read err")
							break
						}
						label, rem, err := envelopes.Identify(data)
						if chk.E(err) {
							continue
						}
						switch label {
						case eventenvelope.L:
							res, _, err := eventenvelope.ParseResult(rem)
							if chk.E(err) || res == nil || res.Event == nil {
								continue
							}
							// verify signature before saving
							if ok, err := res.Event.Verify(); chk.T(err) || !ok {
								continue
							}

							// Process events based on kind
							switch res.Event.Kind {
							case kind.FollowList.K:
								// Check if this is from an admin and process immediately
								if f.isAdminPubkey(res.Event.Pubkey) {
									log.I.F(
										"follows syncer: received admin follow list from %s on relay %s - processing immediately",
										hex.EncodeToString(res.Event.Pubkey), u,
									)
									f.extractFollowedPubkeys(res.Event)
								} else {
									log.T.F(
										"follows syncer: received follow list from non-admin %s on relay %s - ignoring",
										hex.EncodeToString(res.Event.Pubkey), u,
									)
								}
							case kind.RelayListMetadata.K:
								log.T.F(
									"follows syncer: received kind 10002 (relay list) event from %s on relay %s",
									hex.EncodeToString(res.Event.Pubkey), u,
								)
							default:
								// Log all other events from followed users
								log.T.F(
									"follows syncer: received kind %d event from %s on relay %s",
									res.Event.Kind,
									hex.EncodeToString(res.Event.Pubkey), u,
								)
							}

							if _, err = f.D.SaveEvent(
								ctx, res.Event,
							); err != nil {
								if !strings.HasPrefix(
									err.Error(), "blocked:",
								) {
									log.W.F(
										"follows syncer: save event failed: %v",
										err,
									)
								}
								// ignore duplicates and continue
							} else {
								// Only dispatch if the event was newly saved (no error)
								if f.pubs != nil {
									go f.pubs.Deliver(res.Event)
								}
								// log.I.F(
								// 	"saved new event from follows syncer: %0x",
								// 	res.Event.ID,
								// )
							}
						case eoseenvelope.L:
							log.T.F("follows syncer: received EOSE from %s, continuing persistent subscription", u)
							// Continue the subscription for new events
						default:
							// ignore other labels
						}
					}
				}
				// Connection dropped, reconnect after delay
				log.W.F("follows syncer: connection to %s dropped, will reconnect in 30 seconds", u)

				// Wait before reconnecting to avoid tight reconnection loops
				timer := time.NewTimer(30 * time.Second)
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					// Continue to reconnect
				}
			}
		}()
	}
}

func (f *Follows) Syncer() {
	log.I.F("starting follows syncer")

	// Start periodic follow list fetching
	go f.startPeriodicFollowListFetching()

	// Start event subscriptions
	go func() {
		// start immediately if Configure already ran
		for {
			var innerCancel context.CancelFunc
			select {
			case <-f.Ctx.Done():
				if f.subsCancel != nil {
					f.subsCancel()
				}
				return
			case <-f.updated:
				// close and reopen subscriptions to users on the follow list and admins
				if f.subsCancel != nil {
					log.I.F("follows syncer: cancelling existing subscriptions")
					f.subsCancel()
				}
				ctx, cancel := context.WithCancel(f.Ctx)
				f.subsCancel = cancel
				innerCancel = cancel
				log.I.F("follows syncer: (re)opening subscriptions")
				f.startEventSubscriptions(ctx)
			}
			// small sleep to avoid tight loop if updated fires rapidly
			if innerCancel == nil {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()
	f.updated <- struct{}{}
}

// startPeriodicFollowListFetching starts periodic fetching of admin follow lists
func (f *Follows) startPeriodicFollowListFetching() {
	frequency := f.cfg.FollowListFrequency
	if frequency == 0 {
		frequency = time.Hour // Default to 1 hour
	}

	log.I.F("starting periodic follow list fetching every %v", frequency)

	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	// Fetch immediately on startup
	f.fetchAdminFollowLists()

	for {
		select {
		case <-f.Ctx.Done():
			log.D.F("periodic follow list fetching stopped due to context cancellation")
			return
		case <-ticker.C:
			f.fetchAdminFollowLists()
		}
	}
}

// fetchAdminFollowLists fetches follow lists from admin relays
func (f *Follows) fetchAdminFollowLists() {
	log.I.F("follows syncer: fetching admin follow lists")

	urls := f.adminRelays()
	if len(urls) == 0 {
		log.W.F("follows syncer: no admin relays found for follow list fetching")
		return
	}

	// build authors list: admins only (not follows)
	f.followsMx.RLock()
	authors := make([][]byte, len(f.admins))
	copy(authors, f.admins)
	f.followsMx.RUnlock()

	if len(authors) == 0 {
		log.W.F("follows syncer: no admins to fetch follow lists for")
		return
	}

	log.I.F("follows syncer: fetching follow lists from %d relays for %d admins", len(urls), len(authors))

	for _, u := range urls {
		go f.fetchFollowListsFromRelay(u, authors)
	}
}

// fetchFollowListsFromRelay fetches follow lists from a specific relay
func (f *Follows) fetchFollowListsFromRelay(relayURL string, authors [][]byte) {
	ctx, cancel := context.WithTimeout(f.Ctx, 30*time.Second)
	defer cancel()

	// Create proper headers for the WebSocket connection
	headers := http.Header{}
	headers.Set("User-Agent", "ORLY-Relay/0.9.2")
	headers.Set("Origin", "https://orly.dev")

	// Use proper WebSocket dial options
	dialOptions := &websocket.DialOptions{
		HTTPHeader: headers,
	}

	c, _, err := websocket.Dial(ctx, relayURL, dialOptions)
	if err != nil {
		log.W.F("follows syncer: failed to connect to %s for follow list fetch: %v", relayURL, err)
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "follow list fetch complete")

	log.I.F("follows syncer: fetching follow lists from relay %s", relayURL)

	// Create filter for follow lists only (kind 3)
	ff := &filter.S{}
	f1 := &filter.F{
		Authors: tag.NewFromBytesSlice(authors...),
		Kinds:   kind.NewS(kind.New(kind.FollowList.K)),
		Limit:   values.ToUintPointer(100),
	}
	*ff = append(*ff, f1)

	// Use a specific subscription ID for follow list fetching
	subID := "follow-lists-fetch"
	req := reqenvelope.NewFrom([]byte(subID), ff)
	if err = c.Write(ctx, websocket.MessageText, req.Marshal(nil)); chk.E(err) {
		log.W.F("follows syncer: failed to send follow list REQ to %s: %v", relayURL, err)
		return
	}

	log.T.F("follows syncer: sent follow list REQ to %s", relayURL)

	// Read follow list events with timeout
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			log.T.F("follows syncer: timeout reading follow lists from %s", relayURL)
			return
		default:
		}

		_, data, err := c.Read(ctx)
		if err != nil {
			log.T.F("follows syncer: error reading follow lists from %s: %v", relayURL, err)
			return
		}

		label, rem, err := envelopes.Identify(data)
		if chk.E(err) {
			continue
		}

		switch label {
		case eventenvelope.L:
			res, _, err := eventenvelope.ParseResult(rem)
			if chk.E(err) || res == nil || res.Event == nil {
				continue
			}

			// Process follow list events
			if res.Event.Kind == kind.FollowList.K {
				log.I.F("follows syncer: received follow list from %s on relay %s",
					hex.EncodeToString(res.Event.Pubkey), relayURL)
				f.extractFollowedPubkeys(res.Event)
			}
		case eoseenvelope.L:
			log.T.F("follows syncer: end of follow list events from %s", relayURL)
			return
		default:
			// ignore other labels
		}
	}
}

// GetFollowedPubkeys returns a copy of the followed pubkeys list
func (f *Follows) GetFollowedPubkeys() [][]byte {
	f.followsMx.RLock()
	defer f.followsMx.RUnlock()

	followedPubkeys := make([][]byte, len(f.follows))
	copy(followedPubkeys, f.follows)
	return followedPubkeys
}

// isAdminPubkey checks if a pubkey belongs to an admin
func (f *Follows) isAdminPubkey(pubkey []byte) bool {
	f.followsMx.RLock()
	defer f.followsMx.RUnlock()

	for _, admin := range f.admins {
		if utils.FastEqual(admin, pubkey) {
			return true
		}
	}
	return false
}

// extractFollowedPubkeys extracts followed pubkeys from 'p' tags in kind 3 events
func (f *Follows) extractFollowedPubkeys(event *event.E) {
	if event.Kind != kind.FollowList.K {
		return
	}

	// Extract all 'p' tags (followed pubkeys) from the kind 3 event
	for _, tag := range event.Tags.GetAll([]byte("p")) {
		if len(tag.Value()) == 32 { // Valid pubkey length
			f.AddFollow(tag.Value())
		}
	}
}

// AddFollow appends a pubkey to the in-memory follows list if not already present
// and signals the syncer to refresh subscriptions.
func (f *Follows) AddFollow(pub []byte) {
	if len(pub) == 0 {
		return
	}
	f.followsMx.Lock()
	defer f.followsMx.Unlock()
	for _, p := range f.follows {
		if bytes.Equal(p, pub) {
			return
		}
	}
	b := make([]byte, len(pub))
	copy(b, pub)
	f.follows = append(f.follows, b)
	log.I.F(
		"follows syncer: added new followed pubkey: %s",
		hex.EncodeToString(pub),
	)
	// notify syncer if initialized
	if f.updated != nil {
		select {
		case f.updated <- struct{}{}:
		default:
			// if channel is full or not yet listened to, ignore
		}
	}
}

func init() {
	log.T.F("registering follows ACL")
	Registry.Register(new(Follows))
}
