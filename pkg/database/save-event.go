package database

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"database.orly/indexes"
	"database.orly/indexes/types"
	"encoders.orly/event"
	"encoders.orly/filter"
	"encoders.orly/hex"
	"encoders.orly/kind"
	"encoders.orly/tag"
	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
)

func (d *D) GetSerialsFromFilter(f *filter.F) (
	sers types.Uint40s, err error,
) {
	var idxs []Range
	if idxs, err = GetIndexesFromFilter(f); chk.E(err) {
		return
	}
	for _, idx := range idxs {
		var s types.Uint40s
		if s, err = d.GetSerialsByRange(idx); chk.E(err) {
			continue
		}
		sers = append(sers, s...)
	}
	return
}

// SaveEvent saves an event to the database, generating all the necessary indexes.
func (d *D) SaveEvent(c context.Context, ev *event.E) (kc, vc int, err error) {
	if ev == nil {
		err = errorf.E("nil event")
		return
	}
	// check if the event already exists
	var ser *types.Uint40
	if ser, err = d.GetSerialById(ev.ID); err == nil && ser != nil {
		err = errorf.E("blocked: event already exists: %0x", ev.ID)
		return
	}

	// If the error is "id not found", we can proceed with saving the event
	if err != nil && strings.Contains(err.Error(), "id not found in database") {
		// Reset error since this is expected for new events
		err = nil
	} else if err != nil {
		// For any other error, return it
		log.E.F("error checking if event exists: %s", err)
		return
	}

	// Check if the event has been deleted before allowing resubmission
	if err = d.CheckForDeleted(ev, nil); err != nil {
		// log.I.F(
		// 	"SaveEvent: rejecting resubmission of deleted event ID=%s: %v",
		// 	hex.Enc(ev.ID), err,
		// )
		err = errorf.E("blocked: %s", err.Error())
		return
	}
	// check for replacement
	if kind.IsReplaceable(ev.Kind) {
		// find the events and check timestamps before deleting
		f := &filter.F{
			Authors: tag.NewFromBytesSlice(ev.Pubkey),
			Kinds:   kind.NewS(kind.New(ev.Kind)),
		}
		var sers types.Uint40s
		if sers, err = d.GetSerialsFromFilter(f); chk.E(err) {
			return
		}
		// if found, check timestamps before deleting
		if len(sers) > 0 {
			var shouldReplace bool = true
			for _, s := range sers {
				var oldEv *event.E
				if oldEv, err = d.FetchEventBySerial(s); chk.E(err) {
					continue
				}
				// Only replace if the new event is newer or same timestamp
				if ev.CreatedAt < oldEv.CreatedAt {
					log.I.F(
						"SaveEvent: rejecting older replaceable event ID=%s (created_at=%d) - existing event ID=%s (created_at=%d)",
						hex.Enc(ev.ID), ev.CreatedAt, hex.Enc(oldEv.ID),
						oldEv.CreatedAt,
					)
					shouldReplace = false
					break
				}
			}
			if shouldReplace {
				for _, s := range sers {
					var oldEv *event.E
					if oldEv, err = d.FetchEventBySerial(s); chk.E(err) {
						continue
					}
					log.I.F(
						"SaveEvent: replacing older replaceable event ID=%s (created_at=%d) with newer event ID=%s (created_at=%d)",
						hex.Enc(oldEv.ID), oldEv.CreatedAt, hex.Enc(ev.ID),
						ev.CreatedAt,
					)
					if err = d.DeleteEventBySerial(
						c, s, oldEv,
					); chk.E(err) {
						continue
					}
				}
			} else {
				// Don't save the older event - return an error
				err = errorf.E("blocked: event is older than existing replaceable event")
				return
			}
		}
	} else if kind.IsParameterizedReplaceable(ev.Kind) {
		// find the events and check timestamps before deleting
		dTag := ev.Tags.GetFirst([]byte("d"))
		if dTag == nil {
			err = errorf.E("event is missing a d tag identifier")
			return
		}
		f := &filter.F{
			Authors: tag.NewFromBytesSlice(ev.Pubkey),
			Kinds:   kind.NewS(kind.New(ev.Kind)),
			Tags: tag.NewS(
				tag.NewFromAny("d", dTag.Value()),
			),
		}
		var sers types.Uint40s
		if sers, err = d.GetSerialsFromFilter(f); chk.E(err) {
			return
		}
		// if found, check timestamps before deleting
		if len(sers) > 0 {
			var shouldReplace bool = true
			for _, s := range sers {
				var oldEv *event.E
				if oldEv, err = d.FetchEventBySerial(s); chk.E(err) {
					continue
				}
				// Only replace if the new event is newer or same timestamp
				if ev.CreatedAt < oldEv.CreatedAt {
					log.I.F(
						"SaveEvent: rejecting older addressable event ID=%s (created_at=%d) - existing event ID=%s (created_at=%d)",
						hex.Enc(ev.ID), ev.CreatedAt, hex.Enc(oldEv.ID),
						oldEv.CreatedAt,
					)
					shouldReplace = false
					break
				}
			}
			if shouldReplace {
				for _, s := range sers {
					var oldEv *event.E
					if oldEv, err = d.FetchEventBySerial(s); chk.E(err) {
						continue
					}
					log.I.F(
						"SaveEvent: replacing older addressable event ID=%s (created_at=%d) with newer event ID=%s (created_at=%d)",
						hex.Enc(oldEv.ID), oldEv.CreatedAt, hex.Enc(ev.ID),
						ev.CreatedAt,
					)
					if err = d.DeleteEventBySerial(
						c, s, oldEv,
					); chk.E(err) {
						continue
					}
				}
			} else {
				// Don't save the older event - return an error
				err = errorf.E("blocked: event is older than existing addressable event")
				return
			}
		}
	}
	// Get the next sequence number for the event
	var serial uint64
	if serial, err = d.seq.Next(); chk.E(err) {
		return
	}
	// Generate all indexes for the event
	var idxs [][]byte
	if idxs, err = GetIndexesForEvent(ev, serial); chk.E(err) {
		return
	}
	// log.I.S(idxs)
	for _, k := range idxs {
		kc += len(k)
	}
	// Start a transaction to save the event and all its indexes
	err = d.Update(
		func(txn *badger.Txn) (err error) {
			// Save each index
			for _, key := range idxs {
				if err = func() (err error) {
					// Save the index to the database
					if err = txn.Set(key, nil); chk.E(err) {
						return err
					}
					return
				}(); chk.E(err) {
					return
				}
			}
			// write the event
			k := new(bytes.Buffer)
			ser := new(types.Uint40)
			if err = ser.Set(serial); chk.E(err) {
				return
			}
			if err = indexes.EventEnc(ser).MarshalWrite(k); chk.E(err) {
				return
			}
			v := new(bytes.Buffer)
			ev.MarshalBinary(v)
			kb, vb := k.Bytes(), v.Bytes()
			kc += len(kb)
			vc += len(vb)
			// log.I.S(kb, vb)
			if err = txn.Set(kb, vb); chk.E(err) {
				return
			}
			return
		},
	)
	log.T.F(
		"total data written: %d bytes keys %d bytes values for event ID %s", kc,
		vc, hex.Enc(ev.ID),
	)
	log.T.C(
		func() string {
			return fmt.Sprintf("event:\n%s\n", ev.Serialize())
		},
	)
	return
}
