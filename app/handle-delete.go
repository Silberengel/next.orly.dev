package app

import (
	"fmt"

	"database.orly/indexes/types"
	"encoders.orly/envelopes/eventenvelope"
	"encoders.orly/event"
	"encoders.orly/filter"
	"encoders.orly/hex"
	"encoders.orly/ints"
	"encoders.orly/kind"
	"encoders.orly/tag"
	"encoders.orly/tag/atag"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	utils "utils.orly"
)

func (l *Listener) GetSerialsFromFilter(f *filter.F) (
	sers types.Uint40s, err error,
) {
	return l.D.GetSerialsFromFilter(f)
}

func (l *Listener) HandleDelete(env *eventenvelope.Submission) (err error) {
	log.I.C(
		func() string {
			return fmt.Sprintf(
				"delete event\n%s", env.E.Serialize(),
			)
		},
	)
	var ownerDelete bool
	for _, pk := range l.Admins {
		if utils.FastEqual(pk, env.E.Pubkey) {
			ownerDelete = true
			break
		}
	}
	// process the tags in the delete event
	var deleteErr error
	var validDeletionFound bool
	for _, t := range *env.E.Tags {
		// first search for a tags, as these are the simplest to process
		if utils.FastEqual(t.Key(), []byte("a")) {
			at := new(atag.T)
			if _, deleteErr = at.Unmarshal(t.Value()); chk.E(deleteErr) {
				continue
			}
			if ownerDelete || utils.FastEqual(env.E.Pubkey, at.Pubkey) {
				validDeletionFound = true
				// find the event and delete it
				f := &filter.F{
					Authors: tag.NewFromBytesSlice(at.Pubkey),
					Kinds:   kind.NewS(at.Kind),
				}
				if len(at.DTag) > 0 {
					f.Tags = tag.NewS(
						tag.NewFromAny("d", at.DTag),
					)
				}
				var sers types.Uint40s
				if sers, err = l.GetSerialsFromFilter(f); chk.E(err) {
					continue
				}
				// if found, delete them
				if len(sers) > 0 {
					for _, s := range sers {
						var ev *event.E
						if ev, err = l.FetchEventBySerial(s); chk.E(err) {
							continue
						}
						// Only delete events that match the a-tag criteria:
						// - For parameterized replaceable events: must have matching d-tag
						// - For regular replaceable events: should not have d-tag constraint
						if kind.IsParameterizedReplaceable(ev.Kind) {
							// For parameterized replaceable, we need a DTag to match
							if len(at.DTag) == 0 {
								log.I.F("HandleDelete: skipping parameterized replaceable event %s - no DTag in a-tag", hex.Enc(ev.ID))
								continue
							}
						} else if !kind.IsReplaceable(ev.Kind) {
							// For non-replaceable events, a-tags don't apply
							log.I.F("HandleDelete: skipping non-replaceable event %s - a-tags only apply to replaceable events", hex.Enc(ev.ID))
							continue
						}
						log.I.F("HandleDelete: deleting event %s via a-tag %d:%s:%s", 
							hex.Enc(ev.ID), at.Kind.K, hex.Enc(at.Pubkey), string(at.DTag))
						if err = l.DeleteEventBySerial(
							l.Ctx, s, ev,
						); chk.E(err) {
							continue
						}
					}
				}
			}
			continue
		}
		// if e tags are found, delete them if the author is signer, or one of
		// the owners is signer
		if utils.FastEqual(t.Key(), []byte("e")) {
			val := t.Value()
			if len(val) == 0 {
				continue
			}
			var dst []byte
			if b, e := hex.Dec(string(val)); chk.E(e) {
				continue
			} else {
				dst = b
			}
			f := &filter.F{
				Ids: tag.NewFromBytesSlice(dst),
			}
			var sers types.Uint40s
			if sers, err = l.GetSerialsFromFilter(f); chk.E(err) {
				continue
			}
			// if found, delete them
			if len(sers) > 0 {
				// there should be only one event per serial, so we can just
				// delete them all
				for _, s := range sers {
					var ev *event.E
					if ev, err = l.FetchEventBySerial(s); chk.E(err) {
						continue
					}
					// check that the author is the same as the signer of the
					// delete, for the e tag case the author is the signer of
					// the event.
					if !utils.FastEqual(env.E.Pubkey, ev.Pubkey) {
						log.W.F("HandleDelete: attempted deletion of event %s by different user - delete pubkey=%s, event pubkey=%s", 
							hex.Enc(ev.ID), hex.Enc(env.E.Pubkey), hex.Enc(ev.Pubkey))
						continue
					}
					validDeletionFound = true
					// exclude delete events
					if ev.Kind == kind.EventDeletion.K {
						continue
					}
					log.I.F("HandleDelete: deleting event %s by authorized user %s", 
						hex.Enc(ev.ID), hex.Enc(env.E.Pubkey))
					if err = l.DeleteEventBySerial(l.Ctx, s, ev); chk.E(err) {
						continue
					}
				}
				continue
			}
		}
		// if k tags are found, check they are replaceable
		if utils.FastEqual(t.Key(), []byte("k")) {
			ki := ints.New(0)
			if _, err = ki.Unmarshal(t.Value()); chk.E(err) {
				continue
			}
			kn := ki.Uint16()
			// skip events that are delete events or that are not replaceable
			if !kind.IsReplaceable(kn) || kn != kind.EventDeletion.K {
				continue
			}
			f := &filter.F{
				Authors: tag.NewFromBytesSlice(env.E.Pubkey),
				Kinds:   kind.NewS(kind.New(kn)),
			}
			var sers types.Uint40s
			if sers, err = l.GetSerialsFromFilter(f); chk.E(err) {
				continue
			}
			// if found, delete them
			if len(sers) > 0 {
				// there should be only one event per serial because replaces
				// delete old ones, so we can just delete them all
				for _, s := range sers {
					var ev *event.E
					if ev, err = l.FetchEventBySerial(s); chk.E(err) {
						continue
					}
					// check that the author is the same as the signer of the
					// delete, for the k tag case the author is the signer of
					// the event.
					if !utils.FastEqual(env.E.Pubkey, ev.Pubkey) {
						continue
					}
				}
				continue
			}
		}
		continue
	}
	
	// If no valid deletions were found, return an error
	if !validDeletionFound {
		return fmt.Errorf("blocked: cannot delete events that belong to other users")
	}
	
	return
}
