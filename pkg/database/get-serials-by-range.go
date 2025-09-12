package database

import (
	"bytes"
	"sort"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes/types"
)

func (d *D) GetSerialsByRange(idx Range) (
	sers types.Uint40s, err error,
) {
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			it := txn.NewIterator(
				badger.IteratorOptions{
					Reverse: true,
				},
			)
			defer it.Close()
			// Start from a position that includes the end boundary (until timestamp)
			// We create an end boundary that's slightly beyond the actual end to ensure inclusivity
			endBoundary := make([]byte, len(idx.End))
			copy(endBoundary, idx.End)
			// Add 0xff bytes to ensure we capture all events at the exact until timestamp
			for i := 0; i < 5; i++ {
				endBoundary = append(endBoundary, 0xff)
			}
			for it.Seek(endBoundary); it.Valid(); it.Next() {
				item := it.Item()
				var key []byte
				key = item.Key()
				if bytes.Compare(
					key[:len(key)-5], idx.Start,
				) < 0 {
					// didn't find it within the timestamp range
					return
				}
				ser := new(types.Uint40)
				buf := bytes.NewBuffer(key[len(key)-5:])
				if err = ser.UnmarshalRead(buf); chk.E(err) {
					return
				}
				sers = append(sers, ser)
			}
			return
		},
	); chk.E(err) {
		return
	}
	sort.Slice(
		sers, func(i, j int) bool {
			return sers[i].Get() < sers[j].Get()
		},
	)
	return
}
