package database

import (
	"bytes"
	"sort"

	"database.orly/indexes"
	"database.orly/indexes/types"
	"encoders.orly/event"
	"encoders.orly/ints"
	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

const (
	currentVersion uint32 = 1
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
		// write the version tag now
		if err = d.Update(
			func(txn *badger.Txn) (err error) {
				buf := new(bytes.Buffer)
				vv := new(types.Uint32)
				vv.Set(currentVersion)
				log.I.S(vv)
				if err = indexes.VersionEnc(vv).MarshalWrite(buf); chk.E(err) {
					return
				}
				if err = txn.Set(buf.Bytes(), nil); chk.E(err) {
					return
				}
				return
			},
		); chk.E(err) {
			return
		}
	}
	if dbVersion < 1 {
		log.I.F("migrating to version 1...")
		// the first migration is expiration tags
		d.UpdateExpirationTags()
	}
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
