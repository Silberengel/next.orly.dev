package types

import (
	"io"

	"github.com/minio/sha256-simd"
)

const TagKeyHashLen = 8

// TagKeyHash represents a hashed multi-letter tag key (e.g., "type", "book", "chapter", "verse", "version")
// It uses the same 8-byte hash approach as Ident for consistency
type TagKeyHash struct{ val [TagKeyHashLen]byte }

// FromTagKey hashes a multi-letter tag key (e.g., "type", "book") to 8 bytes
func (t *TagKeyHash) FromTagKey(key []byte) {
	keyh := sha256.Sum256(key)
	copy(t.val[:], keyh[:TagKeyHashLen])
	return
}

// Bytes returns the hash bytes
func (t *TagKeyHash) Bytes() (b []byte) { return t.val[:] }

// MarshalWrite writes the hash to the writer
func (t *TagKeyHash) MarshalWrite(w io.Writer) (err error) {
	_, err = w.Write(t.val[:])
	return
}

// UnmarshalRead reads the hash from the reader
func (t *TagKeyHash) UnmarshalRead(r io.Reader) (err error) {
	_, err = r.Read(t.val[:])
	return
}
