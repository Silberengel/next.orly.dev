package database

import (
	"bytes"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes"
	. "next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
)

// appendIndexBytes marshals an index to a byte slice and appends it to the idxs slice
func appendIndexBytes(idxs *[][]byte, idx *indexes.T) (err error) {
	buf := new(bytes.Buffer)
	// Marshal the index to the buffer
	if err = idx.MarshalWrite(buf); chk.E(err) {
		return
	}
	// Copy the buffer's bytes to a new byte slice
	// Append the byte slice to the idxs slice
	*idxs = append(*idxs, buf.Bytes())
	return
}

// GetIndexesForEvent creates all the indexes for an event.E instance as defined
// in keys.go. It returns a slice of byte slices that can be used to store the
// event in the database.
func GetIndexesForEvent(ev *event.E, serial uint64) (
	idxs [][]byte, err error,
) {
	defer func() {
		if chk.E(err) {
			idxs = nil
		}
	}()
	// Convert serial to Uint40
	ser := new(Uint40)
	if err = ser.Set(serial); chk.E(err) {
		return
	}
	// ID index
	idHash := new(IdHash)
	if err = idHash.FromId(ev.ID); chk.E(err) {
		return
	}
	idIndex := indexes.IdEnc(idHash, ser)
	if err = appendIndexBytes(&idxs, idIndex); chk.E(err) {
		return
	}
	// FullIdPubkey index
	fullID := new(Id)
	if err = fullID.FromId(ev.ID); chk.E(err) {
		return
	}
	pubHash := new(PubHash)
	if err = pubHash.FromPubkey(ev.Pubkey); chk.E(err) {
		return
	}
	createdAt := new(Uint64)
	createdAt.Set(uint64(ev.CreatedAt))
	idPubkeyIndex := indexes.FullIdPubkeyEnc(
		ser, fullID, pubHash, createdAt,
	)
	if err = appendIndexBytes(&idxs, idPubkeyIndex); chk.E(err) {
		return
	}
	// CreatedAt index
	createdAtIndex := indexes.CreatedAtEnc(createdAt, ser)
	if err = appendIndexBytes(&idxs, createdAtIndex); chk.E(err) {
		return
	}
	// PubkeyCreatedAt index
	pubkeyIndex := indexes.PubkeyEnc(pubHash, createdAt, ser)
	if err = appendIndexBytes(&idxs, pubkeyIndex); chk.E(err) {
		return
	}
	// Process tags for tag-related indexes
	if ev.Tags != nil && ev.Tags.Len() > 0 {
		for _, t := range *ev.Tags {
			// only index tags with a value field
			if t.Len() >= 2 {
				// Get the key and value from the tag
				keyBytes := t.Key()
				valueBytes := t.Value()
				valueHash := new(Ident)
				valueHash.FromIdent(valueBytes)
				kind := new(Uint16)
				kind.Set(ev.Kind)

				// Handle single-letter keys (existing behavior)
				if len(keyBytes) == 1 {
					// if the key is not a-zA-Z skip
					if (keyBytes[0] < 'a' || keyBytes[0] > 'z') &&
						(keyBytes[0] < 'A' || keyBytes[0] > 'Z') {
						continue
					}
					// Create tag key and value
					key := new(Letter)
					key.Set(keyBytes[0])
					// TagPubkey index
					pubkeyTagIndex := indexes.TagPubkeyEnc(
						key, valueHash, pubHash, createdAt, ser,
					)
					if err = appendIndexBytes(
						&idxs, pubkeyTagIndex,
					); chk.E(err) {
						return
					}
					// Tag index
					tagIndex := indexes.TagEnc(
						key, valueHash, createdAt, ser,
					)
					if err = appendIndexBytes(
						&idxs, tagIndex,
					); chk.E(err) {
						return
					}
					// TagKind index
					kindTagIndex := indexes.TagKindEnc(
						key, valueHash, kind, createdAt, ser,
					)
					if err = appendIndexBytes(
						&idxs, kindTagIndex,
					); chk.E(err) {
						return
					}
					// TagKindPubkey index
					kindPubkeyTagIndex := indexes.TagKindPubkeyEnc(
						key, valueHash, kind, pubHash, createdAt, ser,
					)
					if err = appendIndexBytes(
						&idxs, kindPubkeyTagIndex,
					); chk.E(err) {
						return
					}
				} else {
					// Handle multi-letter keys (e.g., "type", "book", "chapter", "verse", "version")
					// Only index specific multi-letter keys to avoid indexing everything
					allowedKeys := map[string]bool{
						"type":    true,
						"book":    true,
						"chapter": true,
						"verse":   true,
						"version": true,
					}
					keyStr := string(keyBytes)
					if !allowedKeys[keyStr] {
						continue
					}
					// Create tag key hash
					keyHash := new(TagKeyHash)
					keyHash.FromTagKey(keyBytes)
					// TagLongPubkey index
					pubkeyTagLongIndex := indexes.TagLongPubkeyEnc(
						keyHash, valueHash, pubHash, createdAt, ser,
					)
					if err = appendIndexBytes(
						&idxs, pubkeyTagLongIndex,
					); chk.E(err) {
						return
					}
					// TagLong index
					tagLongIndex := indexes.TagLongEnc(
						keyHash, valueHash, createdAt, ser,
					)
					if err = appendIndexBytes(
						&idxs, tagLongIndex,
					); chk.E(err) {
						return
					}
					// TagLongKind index
					kindTagLongIndex := indexes.TagLongKindEnc(
						keyHash, valueHash, kind, createdAt, ser,
					)
					if err = appendIndexBytes(
						&idxs, kindTagLongIndex,
					); chk.E(err) {
						return
					}
					// TagLongKindPubkey index
					kindPubkeyTagLongIndex := indexes.TagLongKindPubkeyEnc(
						keyHash, valueHash, kind, pubHash, createdAt, ser,
					)
					if err = appendIndexBytes(
						&idxs, kindPubkeyTagLongIndex,
					); chk.E(err) {
						return
					}
				}
			}
		}
		// Create composite bookstr index if event has bookstr tags
		// This allows efficient queries filtering by multiple bookstr tags simultaneously
		if ev.Tags != nil && ev.Tags.Len() > 0 {
			var typeHash, bookHash, chapterHash, verseHash, versionHash *Ident
			// Collect bookstr tag values
			for _, t := range *ev.Tags {
				if t.Len() >= 2 {
					keyBytes := t.Key()
					keyStr := string(keyBytes)
					valueBytes := t.Value()
					switch keyStr {
					case "type":
						typeHash = new(Ident)
						typeHash.FromIdent(valueBytes)
					case "book":
						bookHash = new(Ident)
						bookHash.FromIdent(valueBytes)
					case "chapter":
						chapterHash = new(Ident)
						chapterHash.FromIdent(valueBytes)
					case "verse":
						verseHash = new(Ident)
						verseHash.FromIdent(valueBytes)
					case "version":
						versionHash = new(Ident)
						versionHash.FromIdent(valueBytes)
					}
				}
			}
			// Create composite index if we have at least type and book (minimum for bookstr)
			if typeHash != nil && bookHash != nil {
				// Use zero hashes for missing tags
				zeroHash := new(Ident)
				zeroHash.FromIdent([]byte{}) // Empty byte slice creates zero hash
				if chapterHash == nil {
					chapterHash = zeroHash
				}
				if verseHash == nil {
					verseHash = zeroHash
				}
				if versionHash == nil {
					versionHash = zeroHash
				}
				// Create composite bookstr index
				evKind := new(Uint16)
				evKind.Set(ev.Kind)
				bookstrIndex := indexes.TagBookstrEnc(
					evKind, typeHash, bookHash, chapterHash, verseHash, versionHash, createdAt, ser,
				)
				if err = appendIndexBytes(&idxs, bookstrIndex); chk.E(err) {
					return
				}
			}
		}
	}
	kind := new(Uint16)
	kind.Set(uint16(ev.Kind))
	// Kind index
	kindIndex := indexes.KindEnc(kind, createdAt, ser)
	if err = appendIndexBytes(&idxs, kindIndex); chk.E(err) {
		return
	}
	// KindPubkey index
	// Using the correct parameters based on the function signature
	kindPubkeyIndex := indexes.KindPubkeyEnc(
		kind, pubHash, createdAt, ser,
	)
	if err = appendIndexBytes(&idxs, kindPubkeyIndex); chk.E(err) {
		return
	}

	// Word token indexes (from content)
	if len(ev.Content) > 0 {
		for _, h := range TokenHashes(ev.Content) {
			w := new(Word)
			w.FromWord(h) // 8-byte truncated hash
			wIdx := indexes.WordEnc(w, ser)
			if err = appendIndexBytes(&idxs, wIdx); chk.E(err) {
				return
			}
		}
	}
	// Extend full-text search to include all fields of all tags
	if ev.Tags != nil && ev.Tags.Len() > 0 {
		for _, t := range *ev.Tags {
			for _, field := range t.T { // include key and all values
				if len(field) == 0 {
					continue
				}
				for _, h := range TokenHashes(field) {
					w := new(Word)
					w.FromWord(h)
					wIdx := indexes.WordEnc(w, ser)
					if err = appendIndexBytes(&idxs, wIdx); chk.E(err) {
						return
					}
				}
			}
		}
	}
	return
}
