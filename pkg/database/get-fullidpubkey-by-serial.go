package database

import (
	"bytes"

	"database.orly/indexes"
	"database.orly/indexes/types"
	"github.com/dgraph-io/badger/v4"
	"interfaces.orly/store"
	"lol.mleku.dev/chk"
)

func (d *D) GetFullIdPubkeyBySerial(ser *types.Uint40) (
	fidpk *store.IdPkTs, err error,
) {
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			buf := new(bytes.Buffer)
			if err = indexes.FullIdPubkeyEnc(
				ser, nil, nil, nil,
			).MarshalWrite(buf); chk.E(err) {
				return
			}
			prf := buf.Bytes()
			it := txn.NewIterator(
				badger.IteratorOptions{
					Prefix: prf,
				},
			)
			defer it.Close()
			it.Seek(prf)
			if it.Valid() {
				item := it.Item()
				key := item.Key()
				ser, fid, p, ca := indexes.FullIdPubkeyVars()
				buf2 := bytes.NewBuffer(key)
				if err = indexes.FullIdPubkeyDec(
					ser, fid, p, ca,
				).UnmarshalRead(buf2); chk.E(err) {
					return
				}
				idpkts := store.IdPkTs{
					Id:  fid.Bytes(),
					Pub: p.Bytes(),
					Ts:  int64(ca.Get()),
					Ser: ser.Get(),
				}
				fidpk = &idpkts
			}
			return
		},
	); chk.E(err) {
		return
	}
	return
}
