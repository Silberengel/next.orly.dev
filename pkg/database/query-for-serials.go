package database

import (
	"context"

	"database.orly/indexes/types"
	"encoders.orly/filter"
	"interfaces.orly/store"
	"lol.mleku.dev/chk"
)

// QueryForSerials takes a filter and returns the serials of events that match,
// sorted in reverse chronological order.
func (d *D) QueryForSerials(c context.Context, f *filter.F) (
	sers types.Uint40s, err error,
) {
	var founds []*types.Uint40
	var idPkTs []*store.IdPkTs
	if f.Ids != nil && f.Ids.Len() > 0 {
		for _, id := range f.Ids.T {
			var ser *types.Uint40
			if ser, err = d.GetSerialById(id); chk.E(err) {
				return
			}
			founds = append(founds, ser)
		}
		var tmp []*store.IdPkTs
		if tmp, err = d.GetFullIdPubkeyBySerials(founds); chk.E(err) {
			return
		}
		idPkTs = append(idPkTs, tmp...)
	} else {
		if idPkTs, err = d.QueryForIds(c, f); chk.E(err) {
			return
		}
	}
	// extract the serials
	for _, idpk := range idPkTs {
		ser := new(types.Uint40)
		if err = ser.Set(idpk.Ser); chk.E(err) {
			continue
		}
		sers = append(sers, ser)
	}
	return
}
