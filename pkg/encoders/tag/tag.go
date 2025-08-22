// Package tag provides an implementation of a nostr tag list, an array of
// strings with a usually single letter first "key" field, including methods to
// compare, marshal/unmarshal and access elements with their proper semantics.
package tag

import (
	"lol.mleku.dev/errorf"
	"next.orly.dev/pkg/encoders/text"
	"next.orly.dev/pkg/utils/bufpool"
)

// The tag position meanings, so they are clear when reading.
const (
	Key = iota
	Value
	Relay
)

type T struct {
	T [][]byte
	b bufpool.B
}

func New(t ...[]byte) *T {
	return &T{T: t, b: bufpool.Get()}
}

func (t *T) Free() {
	bufpool.Put(t.b)
	t.T = nil
}

// Marshal encodes a tag.T as standard minified JSON array of strings.
//
// Call bufpool.PutBytes(b) to return the buffer to the bufpool after use.
func (t *T) Marshal(dst []byte) (b []byte) {
	dst = append(dst, '[')
	for i, s := range t.T {
		dst = text.AppendQuote(dst, s, text.NostrEscape)
		if i < len(t.T)-1 {
			dst = append(dst, ',')
		}
	}
	dst = append(dst, ']')
	return dst
}

// Unmarshal decodes a standard minified JSON array of strings to a tags.T.
//
// Call bufpool.PutBytes(b) to return the buffer to the bufpool after use.
func (t *T) Unmarshal(b []byte) (r []byte, err error) {
	var inQuotes, openedBracket bool
	var quoteStart int
	for i := 0; i < len(b); i++ {
		if !openedBracket && b[i] == '[' {
			openedBracket = true
		} else if !inQuotes {
			if b[i] == '"' {
				inQuotes, quoteStart = true, i+1
			} else if b[i] == ']' {
				return b[i+1:], err
			}
		} else if b[i] == '\\' && i < len(b)-1 {
			i++
		} else if b[i] == '"' {
			inQuotes = false
			t.T = append(t.T, text.NostrUnescape(b[quoteStart:i]))
		}
	}
	if !openedBracket || inQuotes {
		return nil, errorf.E("tag: failed to parse tag")
	}
	return
}
