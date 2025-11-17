package database

import (
	"bytes"
	"sort"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes/types"
)

func (d *D) GetSerialsByRange(idx Range) (
	sers types.Uint40s, err error,
) {
	// Pre-allocate slice with estimated capacity to reduce reallocations
	sers = make(types.Uint40s, 0, 100) // Estimate based on typical range sizes
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
			iterCount := 0
			it.Seek(endBoundary)
			log.T.F("GetSerialsByRange: iterator valid=%v, sought to endBoundary", it.Valid())
			for it.Valid() {
				iterCount++
				if iterCount > 100 {
					// Safety limit to prevent infinite loops in debugging
					log.T.F("GetSerialsByRange: hit safety limit of 100 iterations")
					break
				}
				item := it.Item()
				var key []byte
				key = item.Key()
				// Safety check: ensure key is long enough to have a serial
				if len(key) < 5 {
					log.T.F("GetSerialsByRange: key too short (%d bytes), skipping", len(key))
					it.Next()
					continue
				}
				// Safety check: ensure key is at least as long as the start range
				if len(key) < len(idx.Start)+5 {
					log.T.F("GetSerialsByRange: key length mismatch (key=%d, expected>=%d), stopping", len(key), len(idx.Start)+5)
					return
				}
				keyWithoutSerial := key[:len(key)-5]
				// Compare keyWithoutSerial with idx.Start
				// For proper index keys, they should have the same length
				// But handle length mismatches gracefully
				var cmp int
				if len(keyWithoutSerial) == len(idx.Start) {
					// Same length: direct comparison
					cmp = bytes.Compare(keyWithoutSerial, idx.Start)
				} else if len(keyWithoutSerial) > len(idx.Start) {
					// keyWithoutSerial is longer: compare the prefix
					cmp = bytes.Compare(keyWithoutSerial[:len(idx.Start)], idx.Start)
					// If prefix matches, the key is >= start (we want to include it)
					if cmp == 0 {
						cmp = 1
					}
				} else {
					// keyWithoutSerial is shorter: compare what we have
					cmp = bytes.Compare(keyWithoutSerial, idx.Start[:len(keyWithoutSerial)])
					// If it matches the prefix, it might still be in range, but it's unusual
					if cmp == 0 {
						// This shouldn't happen for properly structured keys, but handle it
						log.T.F("GetSerialsByRange: keyWithoutSerial shorter than idx.Start, treating as in range")
						cmp = 0 // Treat as equal to include it
					}
				}
				// Safe debug log
				prefixMatch := bytes.HasPrefix(key, idx.Start)
				log.T.F("GetSerialsByRange: iter %d, key prefix matches=%v, cmp=%d, keyWithoutSerial len=%d, idx.Start len=%d", iterCount, prefixMatch, cmp, len(keyWithoutSerial), len(idx.Start))
				if cmp < 0 {
					// didn't find it within the timestamp range
					log.T.F("GetSerialsByRange: key out of range (cmp=%d), stopping iteration", cmp)
					log.T.F("  keyWithoutSerial len=%d: %x", len(keyWithoutSerial), keyWithoutSerial)
					log.T.F("  idx.Start len=%d: %x", len(idx.Start), idx.Start)
					return
				}
				ser := new(types.Uint40)
				buf := bytes.NewBuffer(key[len(key)-5:])
				if err = ser.UnmarshalRead(buf); chk.E(err) {
					return
				}
				sers = append(sers, ser)
				it.Next()
			}
			log.T.F("GetSerialsByRange: iteration complete, found %d serials", len(sers))
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
