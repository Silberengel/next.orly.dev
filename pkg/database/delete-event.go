package database

import (
	"bytes"
	"context"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
)

// DeleteEvent removes an event from the database identified by `eid`. If
// noTombstone is false or not provided, a tombstone is created for the event.
func (d *D) DeleteEvent(c context.Context, eid []byte) (err error) {
	d.Logger.Warningf("deleting event %0x", eid)

	// Get the serial number for the event ID
	var ser *types.Uint40
	ser, err = d.GetSerialById(eid)
	if chk.E(err) {
		return
	}
	if ser == nil {
		// Event wasn't found, nothing to delete
		return
	}
	// Fetch the event to get its data
	var ev *event.E
	ev, err = d.FetchEventBySerial(ser)
	if chk.E(err) {
		return
	}
	if ev == nil {
		// Event wasn't found, nothing to delete. this shouldn't happen.
		return
	}
	if err = d.DeleteEventBySerial(c, ser, ev); chk.E(err) {
		return
	}
	return
}

func (d *D) DeleteEventBySerial(
	c context.Context, ser *types.Uint40, ev *event.E,
) (err error) {
	// Get all indexes for the event
	var idxs [][]byte
	idxs, err = GetIndexesForEvent(ev, ser.Get())
	if chk.E(err) {
		return
	}
	// Get the event key
	eventKey := new(bytes.Buffer)
	if err = indexes.EventEnc(ser).MarshalWrite(eventKey); chk.E(err) {
		return
	}
	// Delete the event and all its indexes in a transaction
	err = d.Update(
		func(txn *badger.Txn) (err error) {
			// Delete the event
			if err = txn.Delete(eventKey.Bytes()); chk.E(err) {
				return
			}
			// Delete all indexes
			for _, key := range idxs {
				if err = txn.Delete(key); chk.E(err) {
					return
				}
			}
			return
		},
	)
	return
}
