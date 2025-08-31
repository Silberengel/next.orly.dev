package database

import (
	"bytes"

	"database.orly/indexes"
	"database.orly/indexes/types"
	"encoders.orly/event"
	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
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
			ev = new(event.E)
			if err = ev.UnmarshalBinary(bytes.NewBuffer(v)); chk.E(err) {
				return
			}
			return
		},
	); err != nil {
		return
	}
	return
}
