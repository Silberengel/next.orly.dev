package database

import (
	"bytes"
	"context"

	"database.orly/indexes"
	"database.orly/indexes/types"
	"encoders.orly/event"
	"encoders.orly/filter"
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
		err = errorf.E("event already exists: %0x", ev.ID)
		return
	}
	// check for replacement
	if kind.IsReplaceable(ev.Kind) {
		// find the events and delete them
		f := &filter.F{
			Authors: tag.NewFromBytesSlice(ev.Pubkey),
			Kinds:   kind.NewS(kind.New(ev.Kind)),
		}
		var sers types.Uint40s
		if sers, err = d.GetSerialsFromFilter(f); chk.E(err) {
			return
		}
		// if found, delete them
		if len(sers) > 0 {
			for _, s := range sers {
				var oldEv *event.E
				if oldEv, err = d.FetchEventBySerial(s); chk.E(err) {
					continue
				}
				if err = d.DeleteEventBySerial(
					c, s, oldEv,
				); chk.E(err) {
					continue
				}
			}
		}
	} else if kind.IsParameterizedReplaceable(ev.Kind) {
		// find the events and delete them
		f := &filter.F{
			Authors: tag.NewFromBytesSlice(ev.Pubkey),
			Kinds:   kind.NewS(kind.New(ev.Kind)),
			Tags: tag.NewS(
				tag.NewFromAny(
					"d", ev.Tags.GetFirst([]byte("d")),
				),
			),
		}
		var sers types.Uint40s
		if sers, err = d.GetSerialsFromFilter(f); chk.E(err) {
			return
		}
		// if found, delete them
		if len(sers) > 0 {
			for _, s := range sers {
				var oldEv *event.E
				if oldEv, err = d.FetchEventBySerial(s); chk.E(err) {
					continue
				}
				if err = d.DeleteEventBySerial(
					c, s, oldEv,
				); chk.E(err) {
					continue
				}
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
	log.T.F("total data written: %d bytes keys %d bytes values", kc, vc)
	return
}
