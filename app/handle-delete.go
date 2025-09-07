package app

import (
	"fmt"

	database "database.orly"
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
	var idxs []database.Range
	if idxs, err = database.GetIndexesFromFilter(f); chk.E(err) {
		return
	}
	for _, idx := range idxs {
		var s types.Uint40s
		if s, err = l.GetSerialsByRange(idx); chk.E(err) {
			continue
		}
		sers = append(sers, s...)
	}
	return
}

func (l *Listener) HandleDelete(env *eventenvelope.Submission) {
	log.T.C(
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
	var err error
	for _, t := range *env.E.Tags {
		// first search for a tags, as these are the simplest to process
		if utils.FastEqual(t.Key(), []byte("a")) {
			at := new(atag.T)
			if _, err = at.Unmarshal(t.Value()); chk.E(err) {
				continue
			}
			if ownerDelete || utils.FastEqual(env.E.Pubkey, at.Pubkey) {
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
						if !(kind.IsReplaceable(ev.Kind) && len(at.DTag) == 0) {
							// skip a tags with no dtag if the kind is not
							// replaceable.
							continue
						}
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
			var dst []byte
			if _, err = hex.DecBytes(dst, t.Value()); chk.E(err) {
				continue
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
					// delete, for the k tag case the author is the signer of
					// the event.
					if !utils.FastEqual(env.E.Pubkey, ev.Pubkey) {
						continue
					}
					// exclude delete events
					if ev.Kind == kind.EventDeletion.K {
						continue
					}
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
	return
}
