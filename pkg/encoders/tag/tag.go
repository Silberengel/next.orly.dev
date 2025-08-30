// Package tag provides an implementation of a nostr tag list, an array of
// strings with a usually single letter first "key" field, including methods to
// compare, marshal/unmarshal and access elements with their proper semantics.
package tag

import (
	"bytes"

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

func New(t ...any) *T {
	var bs [][]byte
	for _, v := range t {
		if vb, ok := v.([]byte); ok {
			bs = append(bs, vb)
		} else if vs, ok := v.(string); ok {
			bs = append(bs, []byte(vs))
		} else {
			panic("programmer error: type of tag element is not []byte or string")
		}
	}
	return &T{T: bs, b: bufpool.Get()}
}

func NewWithCap(c int) *T {
	return &T{T: make([][]byte, 0, c), b: bufpool.Get()}
}

func (t *T) Free() {
	bufpool.Put(t.b)
	t.T = nil
}

func (t *T) Len() int { return len(t.T) }

func (t *T) Less(i, j int) bool {
	return bytes.Compare(t.T[i], t.T[j]) < 0
}

func (t *T) Swap(i, j int) { t.T[i], t.T[j] = t.T[j], t.T[i] }

// Marshal encodes a tag.T as standard minified JSON array of strings.
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

// MarshalJSON encodes a tag.T as standard minified JSON array of strings.
//
// Warning: this will mangle the output if the tag fields contain <, > or &
// characters. do not use json.Marshal in the hopes of rendering tags verbatim
// in an event as you will have a bad time. Use the json.Marshal function in the
// pkg/encoders/json package instead, this has a fork of the json library that
// disables html escaping for json.Marshal.
//
// Call bufpool.PutBytes(b) to return the buffer to the bufpool after use.
func (t *T) MarshalJSON() (b []byte, err error) {
	b = bufpool.Get()
	b = t.Marshal(b)
	return
}

// Unmarshal decodes a standard minified JSON array of strings to a tags.T.
//
// Call bufpool.PutBytes(b) to return the buffer to the bufpool after use if it
// was originally created using bufpool.Get().
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

func (t *T) UnmarshalJSON(b []byte) (err error) {
	_, err = t.Unmarshal(b)
	return
}

func (t *T) Key() (key []byte) {
	if len(t.T) > Key {
		return t.T[Key]
	}
	return
}

func (t *T) Value() (key []byte) {
	if len(t.T) > Value {
		return t.T[Value]
	}
	return
}

func (t *T) Relay() (key []byte) {
	if len(t.T) > Relay {
		return t.T[Relay]
	}
	return
}
