package database

import (
	"bytes"
	"sort"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/ints"
)

const (
	currentVersion uint32 = 2
)

func (d *D) RunMigrations() {
	var err error
	var dbVersion uint32
	// first find the current version tag if any
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			buf := new(bytes.Buffer)
			if err = indexes.VersionEnc(nil).MarshalWrite(buf); chk.E(err) {
				return
			}
			verPrf := new(bytes.Buffer)
			if _, err = indexes.VersionPrefix.Write(verPrf); chk.E(err) {
				return
			}
			it := txn.NewIterator(
				badger.IteratorOptions{
					Prefix: verPrf.Bytes(),
				},
			)
			defer it.Close()
			ver := indexes.VersionVars()
			for it.Rewind(); it.Valid(); it.Next() {
				// there should only be one
				item := it.Item()
				key := item.Key()
				if err = indexes.VersionDec(ver).UnmarshalRead(
					bytes.NewBuffer(key),
				); chk.E(err) {
					return
				}
				log.I.F("found version tag: %d", ver.Get())
				dbVersion = ver.Get()
			}
			return
		},
	); chk.E(err) {
	}
	if dbVersion == 0 {
		log.D.F("no version tag found, creating...")
		// write the version tag now (ensure any old tags are removed first)
		if err = d.writeVersionTag(currentVersion); chk.E(err) {
			return
		}
	}
	if dbVersion < 1 {
		log.I.F("migrating to version 1...")
		// the first migration is expiration tags
		d.UpdateExpirationTags()
		// bump to version 1
		_ = d.writeVersionTag(1)
	}
	if dbVersion < 2 {
		log.I.F("migrating to version 2...")
		// backfill word indexes
		d.UpdateWordIndexes()
		// bump to version 2
		_ = d.writeVersionTag(2)
	}
}

// writeVersionTag writes a new version tag key to the database (no value)
func (d *D) writeVersionTag(ver uint32) (err error) {
	return d.Update(
		func(txn *badger.Txn) (err error) {
			// delete any existing version keys first (there should only be one, but be safe)
			verPrf := new(bytes.Buffer)
			if _, err = indexes.VersionPrefix.Write(verPrf); chk.E(err) {
				return
			}
			it := txn.NewIterator(badger.IteratorOptions{Prefix: verPrf.Bytes()})
			defer it.Close()
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				key := item.KeyCopy(nil)
				if err = txn.Delete(key); chk.E(err) {
					return
				}
			}

			// now write the new version key
			buf := new(bytes.Buffer)
			vv := new(types.Uint32)
			vv.Set(ver)
			if err = indexes.VersionEnc(vv).MarshalWrite(buf); chk.E(err) {
				return
			}
			return txn.Set(buf.Bytes(), nil)
		},
	)
}

func (d *D) UpdateWordIndexes() {
	log.T.F("updating word indexes...")
	var err error
	var wordIndexes [][]byte
	// iterate all events and generate word index keys from content and tags
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			prf := new(bytes.Buffer)
			if err = indexes.EventEnc(nil).MarshalWrite(prf); chk.E(err) {
				return
			}
			it := txn.NewIterator(badger.IteratorOptions{Prefix: prf.Bytes()})
			defer it.Close()
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				var val []byte
				if val, err = item.ValueCopy(nil); chk.E(err) {
					continue
				}
				// decode the event
				ev := new(event.E)
				if err = ev.UnmarshalBinary(bytes.NewBuffer(val)); chk.E(err) {
					continue
				}
				// log.I.F("updating word indexes for event: %s", ev.Serialize())
				// read serial from key
				key := item.Key()
				ser := indexes.EventVars()
				if err = indexes.EventDec(ser).UnmarshalRead(bytes.NewBuffer(key)); chk.E(err) {
					continue
				}
				// collect unique word hashes for this event
				seen := make(map[string]struct{})
				// from content
				if len(ev.Content) > 0 {
					for _, h := range TokenHashes(ev.Content) {
						seen[string(h)] = struct{}{}
					}
				}
				// from all tag fields (key and values)
				if ev.Tags != nil && ev.Tags.Len() > 0 {
					for _, t := range *ev.Tags {
						for _, field := range t.T {
							if len(field) == 0 {
								continue
							}
							for _, h := range TokenHashes(field) {
								seen[string(h)] = struct{}{}
							}
						}
					}
				}
				// build keys
				for k := range seen {
					w := new(types.Word)
					w.FromWord([]byte(k))
					buf := new(bytes.Buffer)
					if err = indexes.WordEnc(
						w, ser,
					).MarshalWrite(buf); chk.E(err) {
						continue
					}
					wordIndexes = append(wordIndexes, buf.Bytes())
				}
			}
			return
		},
	); chk.E(err) {
		return
	}
	// sort the indexes for ordered writes
	sort.Slice(
		wordIndexes, func(i, j int) bool {
			return bytes.Compare(
				wordIndexes[i], wordIndexes[j],
			) < 0
		},
	)
	// write in a batch
	batch := d.NewWriteBatch()
	for _, v := range wordIndexes {
		if err = batch.Set(v, nil); chk.E(err) {
			continue
		}
	}
	_ = batch.Flush()
	log.T.F("finished updating word indexes...")
}

func (d *D) UpdateExpirationTags() {
	log.T.F("updating expiration tag indexes...")
	var err error
	var expIndexes [][]byte
	// iterate all event records and decode and look for version tags
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			prf := new(bytes.Buffer)
			if err = indexes.EventEnc(nil).MarshalWrite(prf); chk.E(err) {
				return
			}
			it := txn.NewIterator(badger.IteratorOptions{Prefix: prf.Bytes()})
			defer it.Close()
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				var val []byte
				if val, err = item.ValueCopy(nil); chk.E(err) {
					continue
				}
				// decode the event
				ev := new(event.E)
				if err = ev.UnmarshalBinary(bytes.NewBuffer(val)); chk.E(err) {
					continue
				}
				expTag := ev.Tags.GetFirst([]byte("expiration"))
				if expTag == nil {
					continue
				}
				expTS := ints.New(0)
				if _, err = expTS.Unmarshal(expTag.Value()); chk.E(err) {
					continue
				}
				key := item.Key()
				ser := indexes.EventVars()
				if err = indexes.EventDec(ser).UnmarshalRead(
					bytes.NewBuffer(key),
				); chk.E(err) {
					continue
				}
				// create the expiration tag
				exp, _ := indexes.ExpirationVars()
				exp.Set(expTS.N)
				expBuf := new(bytes.Buffer)
				if err = indexes.ExpirationEnc(
					exp, ser,
				).MarshalWrite(expBuf); chk.E(err) {
					continue
				}
				expIndexes = append(expIndexes, expBuf.Bytes())
			}
			return
		},
	); chk.E(err) {
		return
	}
	// sort the indexes first so they're written in order, improving compaction
	// and iteration.
	sort.Slice(
		expIndexes, func(i, j int) bool {
			return bytes.Compare(expIndexes[i], expIndexes[j]) < 0
		},
	)
	// write the collected indexes
	batch := d.NewWriteBatch()
	for _, v := range expIndexes {
		if err = batch.Set(v, nil); chk.E(err) {
			continue
		}
	}
	if err = batch.Flush(); chk.E(err) {
		return
	}
}
