package acl

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"time"

	database "database.orly"
	"database.orly/indexes/types"
	"encoders.orly/bech32encoding"
	"encoders.orly/envelopes"
	"encoders.orly/envelopes/eoseenvelope"
	"encoders.orly/envelopes/eventenvelope"
	"encoders.orly/envelopes/reqenvelope"
	"encoders.orly/event"
	"encoders.orly/filter"
	"encoders.orly/hex"
	"encoders.orly/kind"
	"encoders.orly/tag"
	"github.com/coder/websocket"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	utils "utils.orly"
	"utils.orly/normalize"
	"utils.orly/values"
)

type Follows struct {
	Ctx context.Context
	cfg *config.C
	*database.D
	followsMx  sync.RWMutex
	admins     [][]byte
	follows    [][]byte
	updated    chan struct{}
	subsCancel context.CancelFunc
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
		default:
			err = errorf.E("invalid type: %T", reflect.TypeOf(ca))
		}
	}
	if f.cfg == nil || f.D == nil {
		err = errorf.E("both config and database must be set")
		return
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
		log.I.F("admin: %0x", adm)
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
				log.I.F("admin follow list:\n%s", ev.Serialize())
				for _, v := range ev.Tags.GetAll([]byte("p")) {
					log.I.F("adding follow: %s", v.Value())
					var a []byte
					if b, e := hex.Dec(string(v.Value())); chk.E(e) {
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
	if f.cfg == nil {
		return "write"
	}
	f.followsMx.RLock()
	defer f.followsMx.RUnlock()
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
	return
}

func (f *Follows) startSubscriptions(ctx context.Context) {
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
	if len(urls) == 0 {
		log.W.F("follows syncer: no admin relays found in DB (kind 10002)")
		return
	}
	log.T.F(
		"follows syncer: subscribing to %d relays for %d authors", len(urls),
		len(authors),
	)
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
				c, _, err := websocket.Dial(ctx, u, nil)
				if err != nil {
					log.W.F("follows syncer: dial %s failed: %v", u, err)
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
				// send REQ
				ff := &filter.S{}
				f1 := &filter.F{
					Authors: tag.NewFromBytesSlice(authors...),
					Limit:   values.ToUintPointer(0),
				}
				*ff = append(*ff, f1)
				req := reqenvelope.NewFrom([]byte("follows-sync"), ff)
				if err = c.Write(
					ctx, websocket.MessageText, req.Marshal(nil),
				); chk.E(err) {
					_ = c.Close(websocket.StatusInternalError, "write failed")
					continue
				}
				log.T.F("sent REQ to %s for follows subscription", u)
				// read loop
				for {
					select {
					case <-ctx.Done():
						_ = c.Close(websocket.StatusNormalClosure, "ctx done")
						return
					default:
					}
					_, data, err := c.Read(ctx)
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
						if _, _, err := f.D.SaveEvent(
							ctx, res.Event,
						); err != nil {
							if !strings.HasPrefix(
								err.Error(), "event already exists",
							) {
								log.W.F(
									"follows syncer: save event failed: %v",
									err,
								)
							}
							// ignore duplicates and continue
						}
						log.I.F(
							"saved new event from follows syncer: %0x",
							res.Event.ID,
						)
					case eoseenvelope.L:
						// ignore, continue subscription
					default:
						// ignore other labels
					}
				}
				// loop reconnect
			}
		}()
	}
}

func (f *Follows) Syncer() {
	log.I.F("starting follows syncer")
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
				f.startSubscriptions(ctx)
			}
			// small sleep to avoid tight loop if updated fires rapidly
			if innerCancel == nil {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()
	f.updated <- struct{}{}
}

func init() {
	log.T.F("registering follows ACL")
	Registry.Register(new(Follows))
}
