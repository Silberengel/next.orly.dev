package database

import (
	"bytes"

	"database.orly/indexes/types"
	"encoders.orly/filter"
	"encoders.orly/hex"
	"encoders.orly/tag"
	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
)

func (d *D) GetSerialById(id []byte) (ser *types.Uint40, err error) {
	log.T.F("GetSerialById: input id=%s", hex.Enc(id))
	var idxs []Range
	if idxs, err = GetIndexesFromFilter(&filter.F{Ids: tag.NewFromBytesSlice(id)}); chk.E(err) {
		return
	}

	for i, idx := range idxs {
		log.T.F(
			"GetSerialById: searching range %d: start=%x, end=%x", i, idx.Start,
			idx.End,
		)
	}
	if len(idxs) == 0 {
		err = errorf.E("no indexes found for id %0x", id)
		return
	}

	idFound := false
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			var key []byte
			defer it.Close()
			it.Seek(idxs[0].Start)
			if it.ValidForPrefix(idxs[0].Start) {
				item := it.Item()
				key = item.Key()
				ser = new(types.Uint40)
				buf := bytes.NewBuffer(key[len(key)-5:])
				if err = ser.UnmarshalRead(buf); chk.E(err) {
					return
				}
				idFound = true
			} else {
				// Item not found in database
				log.T.F(
					"GetSerialById: ID not found in database: %s", hex.Enc(id),
				)
			}
			return
		},
	); chk.E(err) {
		return
	}

	if !idFound {
		err = errorf.T("id not found in database: %s", hex.Enc(id))
		return
	}

	return
}

//
// func (d *D) GetSerialBytesById(id []byte) (ser []byte, err error) {
// 	var idxs []Range
// 	if idxs, err = GetIndexesFromFilter(&filter.F{Ids: tag.New(id)}); chk.E(err) {
// 		return
// 	}
// 	if len(idxs) == 0 {
// 		err = errorf.E("no indexes found for id %0x", id)
// 	}
// 	if err = d.View(
// 		func(txn *badger.Txn) (err error) {
// 			it := txn.NewIterator(badger.DefaultIteratorOptions)
// 			var key []byte
// 			defer it.Close()
// 			it.Seek(idxs[0].Start)
// 			if it.ValidForPrefix(idxs[0].Start) {
// 				item := it.Item()
// 				key = item.Key()
// 				ser = key[len(key)-5:]
// 			} else {
// 				// just don't return what we don't have? others may be
// 				// found tho.
// 			}
// 			return
// 		},
// 	); chk.E(err) {
// 		return
// 	}
// 	return
// }
