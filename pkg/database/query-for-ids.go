package database

import (
	"context"
	"errors"
	"sort"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/interfaces/store"
)

// QueryForIds retrieves a list of IdPkTs based on the provided filter.
// It supports filtering by ranges and tags but disallows filtering by Ids.
// Results are sorted by timestamp in reverse chronological order by default.
// When a search query is present, results are ranked by a 50/50 blend of
// match count (how many distinct search terms matched) and recency.
// Returns an error if the filter contains Ids or if any operation fails.
func (d *D) QueryForIds(c context.Context, f *filter.F) (
	idPkTs []*store.IdPkTs, err error,
) {
	if f.Ids != nil && f.Ids.Len() > 0 {
		// if there is Ids in the query, this is an error for this query
		err = errors.New("query for Ids is invalid for a filter with Ids")
		return
	}
	var idxs []Range
	if idxs, err = GetIndexesFromFilter(f); chk.E(err) {
		return
	}
	var results []*store.IdPkTs
	var founds []*types.Uint40
	// When searching, we want to count how many index ranges (search terms)
	// matched each note. We'll track counts by serial.
	counts := make(map[uint64]int)
	for _, idx := range idxs {
		if founds, err = d.GetSerialsByRange(idx); chk.E(err) {
			return
		}
		var tmp []*store.IdPkTs
		if tmp, err = d.GetFullIdPubkeyBySerials(founds); chk.E(err) {
			return
		}
		// If this query is driven by Search terms, increment count per serial
		if len(f.Search) > 0 {
			for _, v := range tmp {
				counts[v.Ser]++
			}
		}
		results = append(results, tmp...)
	}
	// deduplicate in case this somehow happened (such as two or more
	// from one tag matched, only need it once)
	seen := make(map[uint64]struct{})
	for _, idpk := range results {
		if _, ok := seen[idpk.Ser]; !ok {
			seen[idpk.Ser] = struct{}{}
			idPkTs = append(idPkTs, idpk)
		}
	}

	if len(f.Search) == 0 {
		// No search query: sort by timestamp in reverse chronological order
		sort.Slice(
			idPkTs, func(i, j int) bool {
				return idPkTs[i].Ts > idPkTs[j].Ts
			},
		)
	} else {
		// Search query present: blend match count relevance with recency (50/50)
		// Normalize both match count and timestamp to [0,1] and compute score.
		var maxCount int
		var minTs, maxTs int64
		if len(idPkTs) > 0 {
			minTs, maxTs = idPkTs[0].Ts, idPkTs[0].Ts
		}
		for _, v := range idPkTs {
			if c := counts[v.Ser]; c > maxCount {
				maxCount = c
			}
			if v.Ts < minTs {
				minTs = v.Ts
			}
			if v.Ts > maxTs {
				maxTs = v.Ts
			}
		}
		// Precompute denominator to avoid div-by-zero
		tsSpan := maxTs - minTs
		if tsSpan <= 0 {
			tsSpan = 1
		}
		if maxCount <= 0 {
			maxCount = 1
		}
		sort.Slice(
			idPkTs, func(i, j int) bool {
				ci := float64(counts[idPkTs[i].Ser]) / float64(maxCount)
				cj := float64(counts[idPkTs[j].Ser]) / float64(maxCount)
				ai := float64(idPkTs[i].Ts-minTs) / float64(tsSpan)
				aj := float64(idPkTs[j].Ts-minTs) / float64(tsSpan)
				si := 0.5*ci + 0.5*ai
				sj := 0.5*cj + 0.5*aj
				if si == sj {
					// tie-break by recency
					return idPkTs[i].Ts > idPkTs[j].Ts
				}
				return si > sj
			},
		)
	}

	if f.Limit != nil && len(idPkTs) > int(*f.Limit) {
		idPkTs = idPkTs[:*f.Limit]
	}
	return
}
