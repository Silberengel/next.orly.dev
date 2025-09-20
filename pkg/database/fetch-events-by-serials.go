package database

import (
	"bytes"
	"sort"
	"strconv"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
)

// FetchEventsBySerials processes multiple serials in ascending order and retrieves
// the corresponding events from the database. It optimizes database access by
// sorting the serials and seeking to each one sequentially.
func (d *D) FetchEventsBySerials(serials []*types.Uint40) (
	evMap map[string]*event.E, err error,
) {
	log.T.F("FetchEventsBySerials: processing %d serials", len(serials))

	// Initialize the result map
	evMap = make(map[string]*event.E)

	// Return early if no serials are provided
	if len(serials) == 0 {
		return
	}

	// Sort serials in ascending order for more efficient database access
	sortedSerials := make([]*types.Uint40, len(serials))
	copy(sortedSerials, serials)
	sort.Slice(
		sortedSerials, func(i, j int) bool {
			return sortedSerials[i].Get() < sortedSerials[j].Get()
		},
	)

	// Process all serials in a single transaction
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			// Create an iterator with default options
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			// Process each serial sequentially
			for _, ser := range sortedSerials {
				// Create the key for this serial
				buf := new(bytes.Buffer)
				if err = indexes.EventEnc(ser).MarshalWrite(buf); chk.E(err) {
					continue
				}
				key := buf.Bytes()

				// Seek to this key in the database
				it.Seek(key)
				if it.Valid() {
					item := it.Item()

					// Verify the key matches exactly (should always be true after a Seek)
					if !bytes.Equal(item.Key(), key) {
						continue
					}

					ev := new(event.E)
					if err = item.Value(
						func(val []byte) (err error) {
							// Unmarshal the event
							if err = ev.UnmarshalBinary(bytes.NewBuffer(val)); chk.E(err) {
								return
							}
							// Store the event in the result map using the serial value as string key
							return
						},
					); chk.E(err) {
						continue
					}
					evMap[strconv.FormatUint(ser.Get(), 10)] = ev
					// // Get the item value
					// var v []byte
					// if v, err = item.ValueCopy(nil); chk.E(err) {
					// 	continue
					// }
					//
					// // Unmarshal the event
					// ev := new(event.E)
					// if err = ev.UnmarshalBinary(bytes.NewBuffer(v)); chk.E(err) {
					// 	continue
					// }

				}
			}
			return
		},
	); chk.E(err) {
		return
	}

	log.T.F(
		"FetchEventsBySerials: found %d events out of %d requested serials",
		len(evMap), len(serials),
	)
	return
}
