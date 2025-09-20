package database

import (
	"bytes"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
)

// FetchEventsBySerials fetches multiple events by their serials in a single database transaction.
// Returns a map of serial uint64 value to event, only including successfully fetched events.
func (d *D) FetchEventsBySerials(serials []*types.Uint40) (events map[uint64]*event.E, err error) {
	events = make(map[uint64]*event.E)
	
	if len(serials) == 0 {
		return events, nil
	}
	
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			for _, ser := range serials {
				buf := new(bytes.Buffer)
				if err = indexes.EventEnc(ser).MarshalWrite(buf); chk.E(err) {
					// Skip this serial on error but continue with others
					continue
				}
				
				var item *badger.Item
				if item, err = txn.Get(buf.Bytes()); err != nil {
					// Skip this serial if not found but continue with others
					err = nil
					continue
				}
				
				var v []byte
				if v, err = item.ValueCopy(nil); chk.E(err) {
					// Skip this serial on error but continue with others
					err = nil
					continue
				}
				
				// Check if we have valid data before attempting to unmarshal
				if len(v) < 32+32+1+2+1+1+64 { // ID + Pubkey + min varint fields + Sig
					// Skip this serial - incomplete data
					continue
				}
				
				ev := new(event.E)
				if err = ev.UnmarshalBinary(bytes.NewBuffer(v)); err != nil {
					// Skip this serial on unmarshal error but continue with others
					err = nil
					continue
				}
				
				// Successfully unmarshaled event, add to results
				events[ser.Get()] = ev
			}
			return nil
		},
	); err != nil {
		return
	}
	
	return events, nil
}