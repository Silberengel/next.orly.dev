package app

import (
	"errors"
	"fmt"

	acl "acl.orly"
	"encoders.orly/envelopes/authenvelope"
	"encoders.orly/envelopes/closedenvelope"
	"encoders.orly/envelopes/eoseenvelope"
	"encoders.orly/envelopes/eventenvelope"
	"encoders.orly/envelopes/okenvelope"
	"encoders.orly/envelopes/reqenvelope"
	"encoders.orly/event"
	"encoders.orly/filter"
	"encoders.orly/hex"
	"encoders.orly/kind"
	"encoders.orly/reason"
	"encoders.orly/tag"
	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	utils "utils.orly"
	"utils.orly/normalize"
	"utils.orly/pointers"
)

func (l *Listener) HandleReq(msg []byte) (err error) {
	log.T.F("HandleReq: from %s\n%s\n", l.remote, msg)
	var rem []byte
	env := reqenvelope.New()
	if rem, err = env.Unmarshal(msg); chk.E(err) {
		return normalize.Error.Errorf(err.Error())
	}
	if len(rem) > 0 {
		log.I.F("REQ extra bytes: '%s'", rem)
	}
	// send a challenge to the client to auth if an ACL is active
	if acl.Registry.Active.Load() != "none" {
		if err = authenvelope.NewChallengeWith(l.challenge.Load()).
			Write(l); chk.E(err) {
			return
		}
	}
	// check permissions of user
	accessLevel := acl.Registry.GetAccessLevel(l.authedPubkey.Load(), l.remote)
	switch accessLevel {
	case "none":
		if err = okenvelope.NewFrom(
			env.Subscription, false,
			reason.AuthRequired.F("user not authed or has no read access"),
		).Write(l); chk.E(err) {
			return
		}
		return
	default:
		// user has read access or better, continue
		log.D.F("user has %s access", accessLevel)
	}
	var events event.S
	for _, f := range *env.Filters {
		idsLen := 0
		kindsLen := 0
		authorsLen := 0
		tagsLen := 0
		if f != nil {
			if f.Ids != nil {
				idsLen = f.Ids.Len()
			}
			if f.Kinds != nil {
				kindsLen = f.Kinds.Len()
			}
			if f.Authors != nil {
				authorsLen = f.Authors.Len()
			}
			if f.Tags != nil {
				tagsLen = f.Tags.Len()
			}
		}
		log.T.F(
			"REQ %s: filter summary ids=%d kinds=%d authors=%d tags=%d",
			env.Subscription, idsLen, kindsLen, authorsLen, tagsLen,
		)
		if f != nil && f.Authors != nil && f.Authors.Len() > 0 {
			var authors []string
			for _, a := range f.Authors.T {
				authors = append(authors, hex.Enc(a))
			}
			log.T.F("REQ %s: authors=%v", env.Subscription, authors)
		}
		if f != nil && f.Kinds != nil && f.Kinds.Len() > 0 {
			log.T.F("REQ %s: kinds=%v", env.Subscription, f.Kinds.ToUint16())
		}
		if f != nil && f.Ids != nil && f.Ids.Len() > 0 {
			var ids []string
			for _, id := range f.Ids.T {
				ids = append(ids, hex.Enc(id))
			}
			var lim any
			if pointers.Present(f.Limit) {
				lim = *f.Limit
			} else {
				lim = nil
			}
			log.T.F(
				"REQ %s: ids filter count=%d ids=%v limit=%v", env.Subscription,
				f.Ids.Len(), ids, lim,
			)
		}
		if pointers.Present(f.Limit) {
			if *f.Limit == 0 {
				continue
			}
		}
		if events, err = l.QueryEvents(l.Ctx, f); chk.E(err) {
			if errors.Is(err, badger.ErrDBClosed) {
				return
			}
			err = nil
		}
	}
	var tmp event.S
privCheck:
	for _, ev := range events {
		if kind.IsPrivileged(ev.Kind) &&
			accessLevel != "admin" { // admins can see all events
			log.I.F("checking privileged event %s", ev.ID)
			pk := l.authedPubkey.Load()
			if pk == nil {
				continue
			}
			if utils.FastEqual(ev.Pubkey, pk) {
				log.I.F(
					"privileged event %s is for logged in pubkey %0x", ev.ID,
					pk,
				)
				tmp = append(tmp, ev)
				continue
			}
			pTags := ev.Tags.GetAll([]byte("p"))
			for _, pTag := range pTags {
				var pt []byte
				if pt, err = hex.Dec(string(pTag.Value())); chk.E(err) {
					continue
				}
				if utils.FastEqual(pt, pk) {
					log.I.F(
						"privileged event %s is for logged in pubkey %0x",
						ev.ID, pk,
					)
					tmp = append(tmp, ev)
					continue privCheck
				}
			}
			log.W.F(
				"privileged event %s does not contain the logged in pubkey %0x",
				ev.ID, pk,
			)
		} else {
			tmp = append(tmp, ev)
		}
	}
	events = tmp
	seen := make(map[string]struct{})
	for _, ev := range events {
		log.T.F(
			"REQ %s: sending EVENT id=%s kind=%d", env.Subscription,
			hex.Enc(ev.ID), ev.Kind,
		)
		log.T.C(
			func() string {
				return fmt.Sprintf("event:\n%s\n", ev.Serialize())
			},
		)
		var res *eventenvelope.Result
		if res, err = eventenvelope.NewResultWith(
			env.Subscription, ev,
		); chk.E(err) {
			return
		}
		if err = res.Write(l); chk.E(err) {
			return
		}
		// track the IDs we've sent (use hex encoding for stable key)
		seen[hex.Enc(ev.ID)] = struct{}{}
	}
	// write the EOSE to signal to the client that all events found have been
	// sent.
	log.T.F("sending EOSE to %s", l.remote)
	if err = eoseenvelope.NewFrom(env.Subscription).
		Write(l); chk.E(err) {
		return
	}
	// if the query was for just Ids, we know there can't be any more results,
	// so cancel the subscription.
	cancel := true
	log.T.F(
		"REQ %s: computing cancel/subscription; events_sent=%d",
		env.Subscription, len(events),
	)
	var subbedFilters filter.S
	for _, f := range *env.Filters {
		if f.Ids.Len() < 1 {
			cancel = false
			subbedFilters = append(subbedFilters, f)
		} else {
			// remove the IDs that we already sent
			var notFounds [][]byte
			for _, id := range f.Ids.T {
				if _, ok := seen[hex.Enc(id)]; ok {
					continue
				}
				notFounds = append(notFounds, id)
			}
			log.T.F(
				"REQ %s: ids outstanding=%d of %d", env.Subscription,
				len(notFounds), f.Ids.Len(),
			)
			// if all were found, don't add to subbedFilters
			if len(notFounds) == 0 {
				continue
			}
			// rewrite the filter Ids to remove the ones we already sent
			f.Ids = tag.NewFromBytesSlice(notFounds...)
			// add the filter to the list of filters we're subscribing to
			subbedFilters = append(subbedFilters, f)
		}
		// also, if we received the limit number of events, subscription ded
		if pointers.Present(f.Limit) {
			if len(events) < int(*f.Limit) {
				cancel = false
			}
		}
	}
	receiver := make(event.C, 32)
	// if the subscription should be cancelled, do so
	if !cancel {
		l.publishers.Receive(
			&W{
				Conn:         l.conn,
				remote:       l.remote,
				Id:           string(env.Subscription),
				Receiver:     receiver,
				Filters:      env.Filters,
				AuthedPubkey: l.authedPubkey.Load(),
			},
		)
	} else {
		if err = closedenvelope.NewFrom(
			env.Subscription, nil,
		).Write(l); chk.E(err) {
			return
		}
	}
	return
}
