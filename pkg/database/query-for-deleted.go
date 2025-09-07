package database

import (
	"fmt"
	"sort"

	"database.orly/indexes/types"
	"encoders.orly/event"
	"encoders.orly/filter"
	"encoders.orly/hex"
	"encoders.orly/kind"
	"encoders.orly/tag"
	"encoders.orly/tag/atag"
	"interfaces.orly/store"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
)

// CheckForDeleted checks if the event is deleted, and returns an error with
// prefix "blocked:" if it is. This function also allows designating admin
// pubkeys that also may delete the event, normally only the author is allowed
// to delete an event.
func (d *D) CheckForDeleted(ev *event.E, admins [][]byte) (err error) {
	keys := append([][]byte{ev.Pubkey}, admins...)
	authors := tag.NewFromBytesSlice(keys...)
	// if the event is addressable, check for a deletion event with the same
	// kind/pubkey/dtag
	if kind.IsParameterizedReplaceable(ev.Kind) {
		var idxs []Range
		// construct a tag
		t := ev.Tags.GetFirst([]byte("d"))
		a := atag.T{
			Kind:   kind.New(ev.Kind),
			Pubkey: ev.Pubkey,
			DTag:   t.Value(),
		}
		at := a.Marshal(nil)
		if idxs, err = GetIndexesFromFilter(
			&filter.F{
				Authors: authors,
				Kinds:   kind.NewS(kind.Deletion),
				Tags:    tag.NewS(tag.NewFromAny("#a", at)),
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
			// there can be multiple of these because the author/kind/tag is a
			// stable value but refers to any event from the author, of the
			// kind, with the identifier. so we need to fetch the full ID index
			// to get the timestamp and ensure that the event post-dates it.
			// otherwise, it should be rejected.
			var idPkTss []*store.IdPkTs
			var tmp []*store.IdPkTs
			if tmp, err = d.GetFullIdPubkeyBySerials(sers); chk.E(err) {
				return
			}
			idPkTss = append(idPkTss, tmp...)
			// sort by timestamp, so the first is the newest, which the event
			// must be newer to not be deleted.
			sort.Slice(
				idPkTss, func(i, j int) bool {
					return idPkTss[i].Ts > idPkTss[j].Ts
				},
			)
			if ev.CreatedAt < idPkTss[0].Ts {
				err = errorf.E(
					"blocked: %0x was deleted by address %s because it is older than the delete: event: %d delete: %d",
					ev.ID, at, ev.CreatedAt, idPkTss[0].Ts,
				)
				return
			}
			return
		}
		return
	}
	// if the event is replaceable, check if there is a deletion event newer
	// than the event, it must specify the same kind/pubkey. this type of delete
	// only has the k tag to specify the kind, it can be what an author would
	// use, as the author is part of the replaceable event specification.
	if kind.IsReplaceable(ev.Kind) {
		var idxs []Range
		if idxs, err = GetIndexesFromFilter(
			&filter.F{
				Authors: tag.NewFromBytesSlice(ev.Pubkey),
				Kinds:   kind.NewS(kind.Deletion),
				Tags: tag.NewS(
					tag.NewFromAny("#k", fmt.Sprint(ev.Kind)),
				),
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
			// sort by timestamp, so the first is the newest, which the event
			// must be newer to not be deleted.
			sort.Slice(
				idPkTss, func(i, j int) bool {
					return idPkTss[i].Ts > idPkTss[j].Ts
				},
			)
			if ev.CreatedAt < idPkTss[0].Ts {
				err = errorf.E(
					"blocked: %0x was deleted: the event is older than the delete event %0x: event: %d delete: %d",
					ev.ID, idPkTss[0].Id, ev.CreatedAt, idPkTss[0].Ts,
				)
				return
			}
		}
		// this type of delete can also use an a tag to specify kind and
		// author, which would be required for admin deletes
		idxs = nil
		// construct a tag
		a := atag.T{
			Kind:   kind.New(ev.Kind),
			Pubkey: ev.Pubkey,
		}
		at := a.Marshal(nil)
		if idxs, err = GetIndexesFromFilter(
			&filter.F{
				Authors: authors,
				Kinds:   kind.NewS(kind.Deletion),
				Tags:    tag.NewS(tag.NewFromAny("#a", at)),
			},
		); chk.E(err) {
			return
		}
		sers = nil
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
			// sort by timestamp, so the first is the newest
			sort.Slice(
				idPkTss, func(i, j int) bool {
					return idPkTss[i].Ts > idPkTss[j].Ts
				},
			)
			if ev.CreatedAt < idPkTss[0].Ts {
				err = errorf.E(
					"blocked: %0x was deleted by address %s: event is older than the delete: event: %d delete: %d",
					ev.ID, at, idPkTss[0].Id, ev.CreatedAt, idPkTss[0].Ts,
				)
				return
			}
		}
	}
	// otherwise we check for a delete by event id
	var idxs []Range
	if idxs, err = GetIndexesFromFilter(
		&filter.F{
			Authors: authors,
			Kinds:   kind.NewS(kind.Deletion),
			Tags: tag.NewS(
				tag.NewFromAny("#e", hex.Enc(ev.ID)),
			),
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
		// sort by timestamp, so the first is the newest
		sort.Slice(
			idPkTss, func(i, j int) bool {
				return idPkTss[i].Ts > idPkTss[j].Ts
			},
		)
		if ev.CreatedAt < idPkTss[0].Ts {
			err = errorf.E(
				"blocked: %0x was deleted because it is older than the delete: event: %d delete: %d",
				ev.ID, ev.CreatedAt, idPkTss[0].Ts,
			)
			return
		}
		return
	}

	return
}
