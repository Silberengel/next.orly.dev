package acl

import (
	"context"
	"reflect"
	"sync"

	database "database.orly"
	"database.orly/indexes/types"
	"encoders.orly/bech32encoding"
	"encoders.orly/event"
	"encoders.orly/filter"
	"encoders.orly/hex"
	"encoders.orly/kind"
	"encoders.orly/tag"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	utils "utils.orly"
)

type Follows struct {
	Ctx context.Context
	cfg *config.C
	*database.D
	followsMx sync.RWMutex
	admins    [][]byte
	follows   [][]byte
	updated   chan struct{}
}

func (f *Follows) Configure(cfg ...any) (err error) {
	log.I.F("configuring follows ACL")
	for _, ca := range cfg {
		switch c := ca.(type) {
		case *config.C:
			log.D.F("setting ACL config: %v", c)
			f.cfg = c
		case *database.D:
			log.D.F("setting ACL database: %s", c.Path())
			f.D = c
		case context.Context:
			log.D.F("setting ACL context: %s", c.Value("id"))
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
	log.I.F("finding admins")
	f.follows, f.admins = nil, nil
	for _, admin := range f.cfg.Admins {
		log.I.F("%s", admin)
		var adm []byte
		if adm, err = bech32encoding.NpubOrHexToPublicKeyBinary(admin); chk.E(err) {
			continue
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
					if a, err = hex.Dec(string(v.Value())); chk.E(err) {
						continue
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

func (f *Follows) GetAccessLevel(pub []byte) (level string) {
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

func (f *Follows) Syncer() {
	log.I.F("starting follows syncer")
	go func() {
		for {
			select {
			case <-f.Ctx.Done():
				return
			case <-f.updated:
				// close and reopen subscriptions to users on the follow list and
				// admins
				log.I.F("reopening subscriptions")
			}
		}
	}()
}

func init() {
	log.T.F("registering follows ACL")
	Registry.Register(new(Follows))
}
