package database

import (
	"context"
	"sort"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/ints"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/interfaces/store"
)

func (d *D) ProcessDelete(ev *event.E, admins [][]byte) (err error) {
	eTags := ev.Tags.GetAll([]byte("e"))
	aTags := ev.Tags.GetAll([]byte("a"))
	kTags := ev.Tags.GetAll([]byte("k"))
	// if there are no e or a tags, we assume the intent is to delete all
	// replaceable events of the kinds specified by the k tags for the pubkey of
	// the delete event.
	if len(eTags) == 0 && len(aTags) == 0 {
		// parse the kind tags
		var kinds []*kind.K
		for _, k := range kTags {
			kv := k.Value()
			iv := ints.New(0)
			if _, err = iv.Unmarshal(kv); chk.E(err) {
				continue
			}
			kinds = append(kinds, kind.New(iv.N))
		}
		var idxs []Range
		if idxs, err = GetIndexesFromFilter(
			&filter.F{
				Authors: tag.NewFromBytesSlice(ev.Pubkey),
				Kinds:   kind.NewS(kinds...),
			},
		); chk.E(err) {
			return
		}
		var sers types.Uint40s
		for _, idx := range idxs {
			var s types.Uint40s
			if s, err = d.GetSerialsByRange(idx); chk.E(err) {
				return
			}
			sers = append(sers, s...)
		}
		if len(sers) > 0 {
			var idPkTss []*store.IdPkTs
			var tmp []*store.IdPkTs
			if tmp, err = d.GetFullIdPubkeyBySerials(sers); chk.E(err) {
				return
			}
			idPkTss = append(idPkTss, tmp...)
			// sort by timestamp, so the first is the oldest, so we can collect
			// all of them until the delete event created_at.
			sort.Slice(
				idPkTss, func(i, j int) bool {
					return idPkTss[i].Ts > idPkTss[j].Ts
				},
			)
			for _, v := range idPkTss {
				if v.Ts < ev.CreatedAt {
					if err = d.DeleteEvent(
						context.Background(), v.Id,
					); chk.E(err) {
						continue
					}
				}
			}
		}
	}
	return
}
