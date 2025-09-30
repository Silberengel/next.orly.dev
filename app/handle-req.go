package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/encoders/bech32encoding"
	"next.orly.dev/pkg/encoders/envelopes/authenvelope"
	"next.orly.dev/pkg/encoders/envelopes/closedenvelope"
	"next.orly.dev/pkg/encoders/envelopes/eoseenvelope"
	"next.orly.dev/pkg/encoders/envelopes/eventenvelope"
	"next.orly.dev/pkg/encoders/envelopes/okenvelope"
	"next.orly.dev/pkg/encoders/envelopes/reqenvelope"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/reason"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/utils"
	"next.orly.dev/pkg/utils/normalize"
	"next.orly.dev/pkg/utils/pointers"
)

func (l *Listener) HandleReq(msg []byte) (err error) {
	// log.T.F("HandleReq: START processing from %s\n%s\n", l.remote, msg)
	// var rem []byte
	env := reqenvelope.New()
	if _, err = env.Unmarshal(msg); chk.E(err) {
		return normalize.Error.Errorf(err.Error())
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
	}
	var events event.S
	for _, f := range *env.Filters {
		if f != nil && f.Authors != nil && f.Authors.Len() > 0 {
			var authors []string
			for _, a := range f.Authors.T {
				authors = append(authors, hex.Enc(a))
			}
		}
		if f != nil && pointers.Present(f.Limit) {
			if *f.Limit == 0 {
				continue
			}
		}
		// Use a separate context for QueryEvents to prevent cancellation issues
		queryCtx, cancel := context.WithTimeout(
			context.Background(), 30*time.Second,
		)
		defer cancel()
		if events, err = l.QueryEvents(queryCtx, f); chk.E(err) {
			if errors.Is(err, badger.ErrDBClosed) {
				return
			}
			err = nil
		}
		defer func() {
			for _, ev := range events {
				ev.Free()
			}
		}()
	}
	var tmp event.S
privCheck:
	for _, ev := range events {
		// Check for private tag first
		privateTags := ev.Tags.GetAll([]byte("private"))
		if len(privateTags) > 0 && accessLevel != "admin" {
			pk := l.authedPubkey.Load()
			if pk == nil {
				continue // no auth, can't access private events
			}

			// Convert authenticated pubkey to npub for comparison
			authedNpub, err := bech32encoding.BinToNpub(pk)
			if err != nil {
				continue // couldn't convert pubkey, skip
			}

			// Check if authenticated npub is in any private tag
			authorized := false
			for _, privateTag := range privateTags {
				authorizedNpubs := strings.Split(
					string(privateTag.Value()), ",",
				)
				for _, npub := range authorizedNpubs {
					if strings.TrimSpace(npub) == string(authedNpub) {
						authorized = true
						break
					}
				}
				if authorized {
					break
				}
			}

			if !authorized {
				continue // not authorized to see this private event
			}

			tmp = append(tmp, ev)
			continue
		}

		if kind.IsPrivileged(ev.Kind) &&
			accessLevel != "admin" { // admins can see all events
			// log.T.C(
			// 	func() string {
			// 		return fmt.Sprintf(
			// 			"checking privileged event %0x", ev.ID,
			// 		)
			// 	},
			// )
			pk := l.authedPubkey.Load()
			if pk == nil {
				continue
			}
			if utils.FastEqual(ev.Pubkey, pk) {
				log.T.C(
					func() string {
						return fmt.Sprintf(
							"privileged event %s is for logged in pubkey %0x",
							ev.ID, pk,
						)
					},
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
					// log.T.C(
					// 	func() string {
					// 		return fmt.Sprintf(
					// 			"privileged event %s is for logged in pubkey %0x",
					// 			ev.ID, pk,
					// 		)
					// 	},
					// )
					tmp = append(tmp, ev)
					continue privCheck
				}
			}
			// log.T.C(
			// 	func() string {
			// 		return fmt.Sprintf(
			// 			"privileged event %s does not contain the logged in pubkey %0x",
			// 			ev.ID, pk,
			// 		)
			// 	},
			// )
		} else {
			tmp = append(tmp, ev)
		}
	}
	events = tmp
	seen := make(map[string]struct{})
	for _, ev := range events {
		// log.D.C(
		// 	func() string {
		// 		return fmt.Sprintf(
		// 			"REQ %s: sending EVENT id=%s kind=%d", env.Subscription,
		// 			hex.Enc(ev.ID), ev.Kind,
		// 		)
		// 	},
		// )
		// log.T.C(
		// 	func() string {
		// 		return fmt.Sprintf("event:\n%s\n", ev.Serialize())
		// 	},
		// )
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
	// log.T.F("sending EOSE to %s", l.remote)
	if err = eoseenvelope.NewFrom(env.Subscription).
		Write(l); chk.E(err) {
		return
	}
	// if the query was for just Ids, we know there can't be any more results,
	// so cancel the subscription.
	cancel := true
	// log.T.F(
	// 	"REQ %s: computing cancel/subscription; events_sent=%d",
	// 	env.Subscription, len(events),
	// )
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
			// log.T.F(
			// 	"REQ %s: ids outstanding=%d of %d", env.Subscription,
			// 	len(notFounds), f.Ids.Len(),
			// )
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
	// log.T.F("HandleReq: COMPLETED processing from %s", l.remote)
	return
}
