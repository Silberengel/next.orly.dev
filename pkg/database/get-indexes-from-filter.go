package database

import (
	"bytes"
	"math"
	"sort"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	types2 "next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/tag"
)

type Range struct {
	Start, End []byte
}

// IsHexString checks if the byte slice contains only hex characters
func IsHexString(data []byte) (isHex bool) {
	if len(data)%2 != 0 {
		return false
	}
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
			return false
		}
	}
	return true
}

// CreateIdHashFromData creates an IdHash from data that could be hex or binary
func CreateIdHashFromData(data []byte) (i *types2.IdHash, err error) {
	i = new(types2.IdHash)

	// If data looks like hex string and has the right length for hex-encoded
	// sha256
	if len(data) == 64 {
		if err = i.FromIdHex(string(data)); chk.E(err) {
			err = nil
		} else {
			return
		}
	}
	// Assume it's binary data
	if err = i.FromId(data); chk.E(err) {
		return
	}
	return
}

// CreatePubHashFromData creates a PubHash from data that could be hex or binary
func CreatePubHashFromData(data []byte) (p *types2.PubHash, err error) {
	p = new(types2.PubHash)

	// If data looks like hex string and has the right length for hex-encoded
	// pubkey
	if len(data) == 64 {
		if err = p.FromPubkeyHex(string(data)); chk.E(err) {
			err = nil
		} else {
			return
		}
	} else {
		// Assume it's binary data
		if err = p.FromPubkey(data); chk.E(err) {
			return
		}
	}
	return
}

// GetIndexesFromFilter returns encoded indexes based on the given filter.
//
// An error is returned if any input values are invalid during encoding.
//
// The indexes are designed so that only one table needs to be iterated, being a
// complete set of combinations of all fields in the event, thus there is no
// need to decode events until they are to be delivered.
func GetIndexesFromFilter(f *filter.F) (idxs []Range, err error) {
	// ID eid
	//
	// If there is any Ids in the filter, none of the other fields matter. It
	// should be an error, but convention just ignores it.
	if f.Ids.Len() > 0 {
		for _, id := range f.Ids.T {
			if err = func() (err error) {
				var i *types2.IdHash
				if i, err = CreateIdHashFromData(id); chk.E(err) {
					return
				}
				buf := new(bytes.Buffer)
				// Create an index prefix without the serial number
				idx := indexes.IdEnc(i, nil)
				if err = idx.MarshalWrite(buf); chk.E(err) {
					return
				}
				b := buf.Bytes()
				// For ID filters, both start and end indexes are the same (exact match)
				r := Range{b, b}
				idxs = append(idxs, r)
				return
			}(); chk.E(err) {
				return
			}
		}
		return
	}

	// Word search: if Search field is present, generate word index ranges
	if len(f.Search) > 0 {
		for _, h := range TokenHashes(f.Search) {
			w := new(types2.Word)
			w.FromWord(h)
			buf := new(bytes.Buffer)
			idx := indexes.WordEnc(w, nil)
			if err = idx.MarshalWrite(buf); chk.E(err) {
				return
			}
			b := buf.Bytes()
			end := make([]byte, len(b))
			copy(end, b)
			for i := 0; i < 5; i++ { // match any serial
				end = append(end, 0xff)
			}
			idxs = append(idxs, Range{b, end})
		}
		return
	}

	caStart := new(types2.Uint64)
	caEnd := new(types2.Uint64)

	// Set the start of range (Since or default to zero)
	if f.Since != nil && f.Since.V != 0 {
		caStart.Set(uint64(f.Since.V))
	} else {
		caStart.Set(uint64(0))
	}

	// Set the end of range (Until or default to math.MaxInt64)
	if f.Until != nil && f.Until.V != 0 {
		caEnd.Set(uint64(f.Until.V))
	} else {
		caEnd.Set(uint64(math.MaxInt64))
	}

	// Filter out special tags that shouldn't affect index selection
	var filteredTags *tag.S
	bookstrTags := make(map[string][][]byte) // Track bookstr tags for composite index
	if f.Tags != nil && f.Tags.Len() > 0 {
		filteredTags = tag.NewSWithCap(f.Tags.Len())
		for _, t := range *f.Tags {
			// Skip the special "show_all_versions" tag
			if bytes.Equal(t.Key(), []byte("show_all_versions")) {
				continue
			}
			// Track bookstr tags for potential composite index use
			keyBytes := t.Key()
			keyStr := string(keyBytes)
			if keyStr == "type" || keyStr == "book" || keyStr == "chapter" || keyStr == "verse" || keyStr == "version" {
				if t.Len() >= 2 {
					bookstrTags[keyStr] = t.T[1:] // Store all values for this key
				}
			}
			filteredTags.Append(t)
		}
		// sort the filtered tags so they are in iteration order (reverse)
		if filteredTags.Len() > 0 {
			sort.Sort(filteredTags)
		}
	}

	// Check if we should use composite bookstr index
	// Use it if we have multiple bookstr tags and a kind filter
	useBookstrComposite := false
	if f.Kinds != nil && f.Kinds.Len() > 0 {
		bookstrTagCount := 0
		if _, ok := bookstrTags["type"]; ok {
			bookstrTagCount++
		}
		if _, ok := bookstrTags["book"]; ok {
			bookstrTagCount++
		}
		if _, ok := bookstrTags["chapter"]; ok {
			bookstrTagCount++
		}
		if _, ok := bookstrTags["verse"]; ok {
			bookstrTagCount++
		}
		if _, ok := bookstrTags["version"]; ok {
			bookstrTagCount++
		}
		// Use composite index if we have 2+ bookstr tags (more efficient than separate queries)
		useBookstrComposite = bookstrTagCount >= 2
	}

	// TagBookstr composite index (most efficient for multi-tag bookstr queries)
	if useBookstrComposite {
		for _, k := range f.Kinds.ToUint16() {
			kind := new(types2.Uint16)
			kind.Set(k)
			// Get bookstr tag values (use first value if multiple)
			var typeHash, bookHash, chapterHash, verseHash, versionHash *types2.Ident
			zeroHash := new(types2.Ident)
			zeroHash.FromIdent([]byte{}) // Zero hash for missing tags
			if values, ok := bookstrTags["type"]; ok && len(values) > 0 {
				typeHash = new(types2.Ident)
				typeHash.FromIdent(values[0])
			} else {
				typeHash = zeroHash
			}
			if values, ok := bookstrTags["book"]; ok && len(values) > 0 {
				bookHash = new(types2.Ident)
				bookHash.FromIdent(values[0])
			} else {
				bookHash = zeroHash
			}
			if values, ok := bookstrTags["chapter"]; ok && len(values) > 0 {
				chapterHash = new(types2.Ident)
				chapterHash.FromIdent(values[0])
			} else {
				chapterHash = zeroHash
			}
			if values, ok := bookstrTags["verse"]; ok && len(values) > 0 {
				verseHash = new(types2.Ident)
				verseHash.FromIdent(values[0])
			} else {
				verseHash = zeroHash
			}
			if values, ok := bookstrTags["version"]; ok && len(values) > 0 {
				versionHash = new(types2.Ident)
				versionHash.FromIdent(values[0])
			} else {
				versionHash = zeroHash
			}
			// Create range for composite index
			start, end := new(bytes.Buffer), new(bytes.Buffer)
			idxS := indexes.TagBookstrEnc(
				kind, typeHash, bookHash, chapterHash, verseHash, versionHash, caStart, nil,
			)
			if err = idxS.MarshalWrite(start); chk.E(err) {
				return
			}
			idxE := indexes.TagBookstrEnc(
				kind, typeHash, bookHash, chapterHash, verseHash, versionHash, caEnd, nil,
			)
			if err = idxE.MarshalWrite(end); chk.E(err) {
				return
			}
			idxs = append(
				idxs, Range{
					start.Bytes(), end.Bytes(),
				},
			)
		}
		// If we used composite index, we're done (don't fall through to individual tag handling)
		if useBookstrComposite {
			return
		}
	}

	// TagKindPubkey tkp (and TagLongKindPubkey for multi-letter keys)
	if f.Kinds != nil && f.Kinds.Len() > 0 && f.Authors != nil && f.Authors.Len() > 0 && filteredTags != nil && filteredTags.Len() > 0 {
		for _, k := range f.Kinds.ToUint16() {
			for _, author := range f.Authors.T {
				for _, t := range *filteredTags {
					if t.Len() >= 2 {
						keyBytes := t.Key()
						kind := new(types2.Uint16)
						kind.Set(k)
						var p *types2.PubHash
						if p, err = CreatePubHashFromData(author); chk.E(err) {
							return
						}
						// Handle multi-letter keys (e.g., "type", "book", "chapter", "verse", "version")
						// Multi-letter keys are those with length > 1 and not starting with '#'
						if len(keyBytes) > 1 && (len(keyBytes) > 2 || keyBytes[0] != '#') {
							keyHash := new(types2.TagKeyHash)
							keyHash.FromTagKey(keyBytes)
							for _, valueBytes := range t.T[1:] {
								valueHash := new(types2.Ident)
								valueHash.FromIdent(valueBytes)
								start, end := new(bytes.Buffer), new(bytes.Buffer)
								idxS := indexes.TagLongKindPubkeyEnc(
									keyHash, valueHash, kind, p, caStart, nil,
								)
								if err = idxS.MarshalWrite(start); chk.E(err) {
									return
								}
								idxE := indexes.TagLongKindPubkeyEnc(
									keyHash, valueHash, kind, p, caEnd, nil,
								)
								if err = idxE.MarshalWrite(end); chk.E(err) {
									return
								}
								idxs = append(
									idxs, Range{
										start.Bytes(), end.Bytes(),
									},
								)
							}
							continue
						}
						// Handle single-letter keys like "e" or filter-style keys like "#e"
						if len(keyBytes) == 1 || (len(keyBytes) == 2 && keyBytes[0] == '#') {
							key := new(types2.Letter)
							// If the tag key starts with '#', use the second character as the key
							if len(keyBytes) == 2 && keyBytes[0] == '#' {
								key.Set(keyBytes[1])
							} else {
								key.Set(keyBytes[0])
							}
							for _, valueBytes := range t.T[1:] {
								valueHash := new(types2.Ident)
								valueHash.FromIdent(valueBytes)
								start, end := new(bytes.Buffer), new(bytes.Buffer)
								idxS := indexes.TagKindPubkeyEnc(
									key, valueHash, kind, p, caStart, nil,
								)
								if err = idxS.MarshalWrite(start); chk.E(err) {
									return
								}
								idxE := indexes.TagKindPubkeyEnc(
									key, valueHash, kind, p, caEnd, nil,
								)
								if err = idxE.MarshalWrite(end); chk.E(err) {
									return
								}
								idxs = append(
									idxs, Range{
										start.Bytes(), end.Bytes(),
									},
								)
							}
						}
					}
				}
			}
		}
		return
	}

	// TagKind tkc (and TagLongKind for multi-letter keys)
	if f.Kinds != nil && f.Kinds.Len() > 0 && filteredTags != nil && filteredTags.Len() > 0 {
		for _, k := range f.Kinds.ToUint16() {
			for _, t := range *filteredTags {
				if t.Len() >= 2 {
					keyBytes := t.Key()
					kind := new(types2.Uint16)
					kind.Set(k)
					// Handle multi-letter keys
					if len(keyBytes) > 1 && (len(keyBytes) > 2 || keyBytes[0] != '#') {
						keyHash := new(types2.TagKeyHash)
						keyHash.FromTagKey(keyBytes)
						for _, valueBytes := range t.T[1:] {
							valueHash := new(types2.Ident)
							valueHash.FromIdent(valueBytes)
							start, end := new(bytes.Buffer), new(bytes.Buffer)
							idxS := indexes.TagLongKindEnc(
								keyHash, valueHash, kind, caStart, nil,
							)
							if err = idxS.MarshalWrite(start); chk.E(err) {
								return
							}
							idxE := indexes.TagLongKindEnc(
								keyHash, valueHash, kind, caEnd, nil,
							)
							if err = idxE.MarshalWrite(end); chk.E(err) {
								return
							}
							idxs = append(
								idxs, Range{
									start.Bytes(), end.Bytes(),
								},
							)
						}
						continue
					}
					// Handle single-letter keys
					if len(keyBytes) == 1 || (len(keyBytes) == 2 && keyBytes[0] == '#') {
						key := new(types2.Letter)
						if len(keyBytes) == 2 && keyBytes[0] == '#' {
							key.Set(keyBytes[1])
						} else {
							key.Set(keyBytes[0])
						}
						for _, valueBytes := range t.T[1:] {
							valueHash := new(types2.Ident)
							valueHash.FromIdent(valueBytes)
							start, end := new(bytes.Buffer), new(bytes.Buffer)
							idxS := indexes.TagKindEnc(
								key, valueHash, kind, caStart, nil,
							)
							if err = idxS.MarshalWrite(start); chk.E(err) {
								return
							}
							idxE := indexes.TagKindEnc(
								key, valueHash, kind, caEnd, nil,
							)
							if err = idxE.MarshalWrite(end); chk.E(err) {
								return
							}
							idxs = append(
								idxs, Range{
									start.Bytes(), end.Bytes(),
								},
							)
						}
					}
				}
			}
		}
		return
	}

	// TagPubkey tpc (and TagLongPubkey for multi-letter keys)
	if f.Authors != nil && f.Authors.Len() > 0 && filteredTags != nil && filteredTags.Len() > 0 {
		for _, author := range f.Authors.T {
			for _, t := range *filteredTags {
				if t.Len() >= 2 {
					keyBytes := t.Key()
					var p *types2.PubHash
					log.I.S(author)
					if p, err = CreatePubHashFromData(author); chk.E(err) {
						return
					}
					// Handle multi-letter keys
					if len(keyBytes) > 1 && (len(keyBytes) > 2 || keyBytes[0] != '#') {
						keyHash := new(types2.TagKeyHash)
						keyHash.FromTagKey(keyBytes)
						for _, valueBytes := range t.T[1:] {
							valueHash := new(types2.Ident)
							valueHash.FromIdent(valueBytes)
							start, end := new(bytes.Buffer), new(bytes.Buffer)
							idxS := indexes.TagLongPubkeyEnc(
								keyHash, valueHash, p, caStart, nil,
							)
							if err = idxS.MarshalWrite(start); chk.E(err) {
								return
							}
							idxE := indexes.TagLongPubkeyEnc(
								keyHash, valueHash, p, caEnd, nil,
							)
							if err = idxE.MarshalWrite(end); chk.E(err) {
								return
							}
							idxs = append(
								idxs, Range{start.Bytes(), end.Bytes()},
							)
						}
						continue
					}
					// Handle single-letter keys
					if len(keyBytes) == 1 || (len(keyBytes) == 2 && keyBytes[0] == '#') {
						key := new(types2.Letter)
						if len(keyBytes) == 2 && keyBytes[0] == '#' {
							key.Set(keyBytes[1])
						} else {
							key.Set(keyBytes[0])
						}
						for _, valueBytes := range t.T[1:] {
							valueHash := new(types2.Ident)
							valueHash.FromIdent(valueBytes)
							start, end := new(bytes.Buffer), new(bytes.Buffer)
							idxS := indexes.TagPubkeyEnc(
								key, valueHash, p, caStart, nil,
							)
							if err = idxS.MarshalWrite(start); chk.E(err) {
								return
							}
							idxE := indexes.TagPubkeyEnc(
								key, valueHash, p, caEnd, nil,
							)
							if err = idxE.MarshalWrite(end); chk.E(err) {
								return
							}
							idxs = append(
								idxs, Range{start.Bytes(), end.Bytes()},
							)
						}
					}
				}
			}
		}
		return
	}

	// Tag tc- (and TagLong for multi-letter keys)
	if filteredTags != nil && filteredTags.Len() > 0 && (f.Authors == nil || f.Authors.Len() == 0) && (f.Kinds == nil || f.Kinds.Len() == 0) {
		for _, t := range *filteredTags {
			if t.Len() >= 2 {
				keyBytes := t.Key()
				// Handle multi-letter keys
				if len(keyBytes) > 1 && (len(keyBytes) > 2 || keyBytes[0] != '#') {
					keyHash := new(types2.TagKeyHash)
					keyHash.FromTagKey(keyBytes)
					for _, valueBytes := range t.T[1:] {
						valueHash := new(types2.Ident)
						valueHash.FromIdent(valueBytes)
						start, end := new(bytes.Buffer), new(bytes.Buffer)
						idxS := indexes.TagLongEnc(keyHash, valueHash, caStart, nil)
						if err = idxS.MarshalWrite(start); chk.E(err) {
							return
						}
						idxE := indexes.TagLongEnc(keyHash, valueHash, caEnd, nil)
						if err = idxE.MarshalWrite(end); chk.E(err) {
							return
						}
						idxs = append(
							idxs, Range{start.Bytes(), end.Bytes()},
						)
					}
					continue
				}
				// Handle single-letter keys
				if len(keyBytes) == 1 || (len(keyBytes) == 2 && keyBytes[0] == '#') {
					key := new(types2.Letter)
					if len(keyBytes) == 2 && keyBytes[0] == '#' {
						key.Set(keyBytes[1])
					} else {
						key.Set(keyBytes[0])
					}
					for _, valueBytes := range t.T[1:] {
						valueHash := new(types2.Ident)
						valueHash.FromIdent(valueBytes)
						start, end := new(bytes.Buffer), new(bytes.Buffer)
						idxS := indexes.TagEnc(key, valueHash, caStart, nil)
						if err = idxS.MarshalWrite(start); chk.E(err) {
							return
						}
						idxE := indexes.TagEnc(key, valueHash, caEnd, nil)
						if err = idxE.MarshalWrite(end); chk.E(err) {
							return
						}
						idxs = append(
							idxs, Range{start.Bytes(), end.Bytes()},
						)
					}
				}
			}
		}
		return
	}

	// KindPubkey kpc
	if f.Kinds != nil && f.Kinds.Len() > 0 && f.Authors != nil && f.Authors.Len() > 0 {
		for _, k := range f.Kinds.ToUint16() {
			for _, author := range f.Authors.T {
				kind := new(types2.Uint16)
				kind.Set(k)
				var p *types2.PubHash
				if p, err = CreatePubHashFromData(author); chk.E(err) {
					return
				}
				start, end := new(bytes.Buffer), new(bytes.Buffer)
				idxS := indexes.KindPubkeyEnc(kind, p, caStart, nil)
				if err = idxS.MarshalWrite(start); chk.E(err) {
					return
				}
				idxE := indexes.KindPubkeyEnc(kind, p, caEnd, nil)
				if err = idxE.MarshalWrite(end); chk.E(err) {
					return
				}
				idxs = append(
					idxs, Range{start.Bytes(), end.Bytes()},
				)
			}
		}
		return
	}

	// Kind kc-
	if f.Kinds != nil && f.Kinds.Len() > 0 && (f.Authors == nil || f.Authors.Len() == 0) && (filteredTags == nil || filteredTags.Len() == 0) {
		for _, k := range f.Kinds.ToUint16() {
			kind := new(types2.Uint16)
			kind.Set(k)
			start, end := new(bytes.Buffer), new(bytes.Buffer)
			idxS := indexes.KindEnc(kind, caStart, nil)
			if err = idxS.MarshalWrite(start); chk.E(err) {
				return
			}
			idxE := indexes.KindEnc(kind, caEnd, nil)
			if err = idxE.MarshalWrite(end); chk.E(err) {
				return
			}
			idxs = append(
				idxs, Range{start.Bytes(), end.Bytes()},
			)
		}
		return
	}

	// Pubkey pc-
	if f.Authors != nil && f.Authors.Len() > 0 {
		for _, author := range f.Authors.T {
			var p *types2.PubHash
			if p, err = CreatePubHashFromData(author); chk.E(err) {
				return
			}
			start, end := new(bytes.Buffer), new(bytes.Buffer)
			idxS := indexes.PubkeyEnc(p, caStart, nil)
			if err = idxS.MarshalWrite(start); chk.E(err) {
				return
			}
			idxE := indexes.PubkeyEnc(p, caEnd, nil)
			if err = idxE.MarshalWrite(end); chk.E(err) {
				return
			}
			idxs = append(
				idxs, Range{start.Bytes(), end.Bytes()},
			)
		}
		return
	}

	// CreatedAt c--
	start, end := new(bytes.Buffer), new(bytes.Buffer)
	idxS := indexes.CreatedAtEnc(caStart, nil)
	if err = idxS.MarshalWrite(start); chk.E(err) {
		return
	}
	idxE := indexes.CreatedAtEnc(caEnd, nil)
	if err = idxE.MarshalWrite(end); chk.E(err) {
		return
	}
	idxs = append(
		idxs, Range{start.Bytes(), end.Bytes()},
	)
	return
}
