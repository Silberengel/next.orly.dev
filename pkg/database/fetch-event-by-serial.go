package database

import (
	"bytes"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
)

func (d *D) FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error) {
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			buf := new(bytes.Buffer)
			if err = indexes.EventEnc(ser).MarshalWrite(buf); chk.E(err) {
				return
			}
			var item *badger.Item
			if item, err = txn.Get(buf.Bytes()); err != nil {
				return
			}
			var v []byte
			if v, err = item.ValueCopy(nil); chk.E(err) {
				return
			}
			// Check if we have valid data before attempting to unmarshal
			if len(v) < 32+32+1+2+1+1+64 { // ID + Pubkey + min varint fields + Sig
				err = fmt.Errorf(
					"incomplete event data: got %d bytes, expected at least %d",
					len(v), 32+32+1+2+1+1+64,
				)
				return
			}
			ev = new(event.E)
			if err = ev.UnmarshalBinary(bytes.NewBuffer(v)); err != nil {
				// Add more context to EOF errors for debugging
				if err.Error() == "EOF" {
					err = fmt.Errorf(
						"EOF while unmarshaling event (serial=%v, data_len=%d): %w",
						ser, len(v), err,
					)
				}
				return
			}
			return
		},
	); err != nil {
		return
	}
	return
}
