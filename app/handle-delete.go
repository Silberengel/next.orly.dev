package app

import (
	"fmt"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/envelopes/eventenvelope"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/ints"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/tag/atag"
	utils "next.orly.dev/pkg/utils"
)

func (l *Listener) GetSerialsFromFilter(f *filter.F) (
	sers types.Uint40s, err error,
) {
	return l.D.GetSerialsFromFilter(f)
}

func (l *Listener) HandleDelete(env *eventenvelope.Submission) (err error) {
	log.I.F("HandleDelete: processing delete event %0x from pubkey %0x", env.E.ID, env.E.Pubkey)
	log.I.F("HandleDelete: delete event tags: %d tags", len(*env.E.Tags))
	for i, t := range *env.E.Tags {
		log.I.F("HandleDelete: tag %d: %s = %s", i, string(t.Key()), string(t.Value()))
	}

	// Debug: log admin and owner lists
	log.I.F("HandleDelete: checking against %d admins and %d owners", len(l.Admins), len(l.Owners))
	for i, pk := range l.Admins {
		log.I.F("HandleDelete: admin[%d] = %0x", i, pk)
	}
	for i, pk := range l.Owners {
		log.I.F("HandleDelete: owner[%d] = %0x", i, pk)
	}

	var ownerDelete bool
	for _, pk := range l.Admins {
		if utils.FastEqual(pk, env.E.Pubkey) {
			ownerDelete = true
			log.I.F("HandleDelete: delete event from admin/owner %0x", env.E.Pubkey)
			break
		}
	}
	if !ownerDelete {
		for _, pk := range l.Owners {
			if utils.FastEqual(pk, env.E.Pubkey) {
				ownerDelete = true
				log.I.F("HandleDelete: delete event from owner %0x", env.E.Pubkey)
				break
			}
		}
	}
	if !ownerDelete {
		log.I.F("HandleDelete: delete event from regular user %0x", env.E.Pubkey)
	}
	// process the tags in the delete event
	var deleteErr error
	var validDeletionFound bool
	var deletionCount int
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
								log.I.F(
									"HandleDelete: skipping parameterized replaceable event %s - no DTag in a-tag",
									hex.Enc(ev.ID),
								)
								continue
							}
						} else if !kind.IsReplaceable(ev.Kind) {
							// For non-replaceable events, a-tags don't apply
							log.I.F(
								"HandleDelete: skipping non-replaceable event %s - a-tags only apply to replaceable events",
								hex.Enc(ev.ID),
							)
							continue
						}

						// Only delete events that are older than or equal to the delete event timestamp
						if ev.CreatedAt > env.E.CreatedAt {
							log.I.F(
								"HandleDelete: skipping newer event %s (created_at=%d) - delete event timestamp is %d",
								hex.Enc(ev.ID), ev.CreatedAt, env.E.CreatedAt,
							)
							continue
						}

						log.I.F(
							"HandleDelete: deleting event %s via a-tag %d:%s:%s (event_time=%d, delete_time=%d)",
							hex.Enc(ev.ID), at.Kind.K, hex.Enc(at.Pubkey),
							string(at.DTag), ev.CreatedAt, env.E.CreatedAt,
						)
						if err = l.DeleteEventBySerial(
							l.Ctx(), s, ev,
						); chk.E(err) {
							log.E.F("HandleDelete: failed to delete event %s: %v", hex.Enc(ev.ID), err)
							continue
						}
						deletionCount++
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
				log.W.F("HandleDelete: empty e-tag value")
				continue
			}
			log.I.F("HandleDelete: processing e-tag with value: %s", string(val))
			var dst []byte
			if b, e := hex.Dec(string(val)); chk.E(e) {
				log.E.F("HandleDelete: failed to decode hex event ID %s: %v", string(val), e)
				continue
			} else {
				dst = b
				log.I.F("HandleDelete: decoded event ID: %0x", dst)
			}
			f := &filter.F{
				Ids: tag.NewFromBytesSlice(dst),
			}
			var sers types.Uint40s
			if sers, err = l.GetSerialsFromFilter(f); chk.E(err) {
				log.E.F("HandleDelete: failed to get serials from filter: %v", err)
				continue
			}
			log.I.F("HandleDelete: found %d serials for event ID %s", len(sers), string(val))
			// if found, delete them
			if len(sers) > 0 {
				// there should be only one event per serial, so we can just
				// delete them all
				for _, s := range sers {
					var ev *event.E
					if ev, err = l.FetchEventBySerial(s); chk.E(err) {
						continue
					}
					// allow deletion if the signer is the author OR an admin/owner
					if !(ownerDelete || utils.FastEqual(env.E.Pubkey, ev.Pubkey)) {
						log.W.F(
							"HandleDelete: attempted deletion of event %s by unauthorized user - delete pubkey=%s, event pubkey=%s",
							hex.Enc(ev.ID), hex.Enc(env.E.Pubkey),
							hex.Enc(ev.Pubkey),
						)
						continue
					}
					validDeletionFound = true
					// exclude delete events
					if ev.Kind == kind.EventDeletion.K {
						continue
					}
					log.I.F(
						"HandleDelete: deleting event %s by authorized user %s",
						hex.Enc(ev.ID), hex.Enc(env.E.Pubkey),
					)
					if err = l.DeleteEventBySerial(l.Ctx(), s, ev); chk.E(err) {
						log.E.F("HandleDelete: failed to delete event %s: %v", hex.Enc(ev.ID), err)
						continue
					}
					deletionCount++
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
					validDeletionFound = true
					log.I.F(
						"HandleDelete: deleting event %s via k-tag by authorized user %s",
						hex.Enc(ev.ID), hex.Enc(env.E.Pubkey),
					)
					if err = l.DeleteEventBySerial(l.Ctx(), s, ev); chk.E(err) {
						log.E.F("HandleDelete: failed to delete event %s: %v", hex.Enc(ev.ID), err)
						continue
					}
					deletionCount++
				}
			}
		}
	}

	// If no valid deletions were found, return an error
	if !validDeletionFound {
		log.W.F("HandleDelete: no valid deletions found for event %0x", env.E.ID)
		return fmt.Errorf("blocked: cannot delete events that belong to other users")
	}

	log.I.F("HandleDelete: successfully processed %d deletions for event %0x", deletionCount, env.E.ID)
	return
}
