// Package tag provides an implementation of a nostr tag list, an array of
// strings with a usually single letter first "key" field, including methods to
// compare, marshal/unmarshal and access elements with their proper semantics.
package tag

import (
	"bytes"

	"lol.mleku.dev/errorf"
	"next.orly.dev/pkg/encoders/text"
	utils "next.orly.dev/pkg/utils"
)

// The tag position meanings, so they are clear when reading.
const (
	Key = iota
	Value
	Relay
)

type T struct {
	T [][]byte
}

func New() *T { return &T{} }

func NewFromBytesSlice(t ...[]byte) (tt *T) {
	tt = &T{T: t}
	return
}

func NewFromAny(t ...any) (tt *T) {
	tt = &T{}
	for _, v := range t {
		switch vv := v.(type) {
		case []byte:
			tt.T = append(tt.T, vv)
		case string:
			tt.T = append(tt.T, []byte(vv))
		default:
			panic("invalid type for tag fields, must be []byte or string")
		}
	}
	return
}

func NewWithCap(c int) *T {
	return &T{T: make([][]byte, 0, c)}
}

func (t *T) Free() {
	t.T = nil
}

func (t *T) Len() int {
	if t == nil {
		return 0
	}
	return len(t.T)
}

func (t *T) Less(i, j int) bool {
	return bytes.Compare(t.T[i], t.T[j]) < 0
}

func (t *T) Swap(i, j int) { t.T[i], t.T[j] = t.T[j], t.T[i] }

// Contains returns true if the provided element is found in the tag slice.
func (t *T) Contains(s []byte) (b bool) {
	for i := range t.T {
		if utils.FastEqual(t.T[i], s) {
			return true
		}
	}
	return false
}

// Marshal encodes a tag.T as standard minified JSON array of strings.
func (t *T) Marshal(dst []byte) (b []byte) {
	b = dst
	b = append(b, '[')
	for i, s := range t.T {
		b = text.AppendQuote(b, s, text.NostrEscape)
		if i < len(t.T)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return
}

// MarshalJSON encodes a tag.T as standard minified JSON array of strings.
//
// Warning: this will mangle the output if the tag fields contain <, > or &
// characters. do not use json.Marshal in the hopes of rendering tags verbatim
// in an event as you will have a bad time. Use the json.Marshal function in the
// pkg/encoders/json package instead, this has a fork of the json library that
// disables html escaping for json.Marshal.
func (t *T) MarshalJSON() (b []byte, err error) {
	b = t.Marshal(nil)
	return
}

// Unmarshal decodes a standard minified JSON array of strings to a tags.T.
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
			// Copy the quoted substring before unescaping so we don't mutate the
			// original JSON buffer in-place (which would corrupt subsequent parsing).
			copyBuf := make([]byte, i-quoteStart)
			copy(copyBuf, b[quoteStart:i])
			t.T = append(t.T, text.NostrUnescape(copyBuf))
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
