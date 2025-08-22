package tag

import (
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/utils/bufpool"
)

// S is a list of tag.T - which are lists of string elements with ordering and
// no uniqueness constraint (not a set).
type S []*T

// MarshalJSON encodes a tags.T appended to a provided byte slice in JSON form.
//
// Call bufpool.PutBytes(b) to return the buffer to the bufpool after use.
func (s *S) MarshalJSON() (b []byte, err error) {
	b = bufpool.Get()
	b = append(b, '[')
	for i, ss := range *s {
		b = append(b, ss.Marshal()...)
		if i < len(*s)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return
}

// UnmarshalJSON a tags.T from a provided byte slice and return what remains
// after the end of the array.
//
// Call bufpool.PutBytes(b) to return the buffer to the bufpool after use.
func (s *S) UnmarshalJSON(b []byte) (err error) {
	_, err = s.Unmarshal(b)
	return
}

// Unmarshal a tags.T from a provided byte slice and return what remains after
// the end of the array.
func (s *S) Unmarshal(b []byte) (r []byte, err error) {
	r = b[:]
	for len(r) > 0 {
		switch r[0] {
		case '[':
			r = r[1:]
			goto inTags
		case ',':
			r = r[1:]
			// next
		case ']':
			r = r[1:]
			// the end
			return
		default:
			r = r[1:]
		}
	inTags:
		for len(r) > 0 {
			switch r[0] {
			case '[':
				tt := New()
				if r, err = tt.Unmarshal(r); chk.E(err) {
					return
				}
				*s = append(*s, tt)
			case ',':
				r = r[1:]
				// next
			case ']':
				r = r[1:]
				// the end
				return
			default:
				r = r[1:]
			}
		}
	}
	return
}
