package tag

import (
	"bytes"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/utils"
	"next.orly.dev/pkg/utils/bufpool"
)

// S is a list of tag.T - which are lists of string elements with ordering and
// no uniqueness constraint (not a set).
type S []*T

func NewS(t ...*T) (s *S) {
	s = new(S)
	*s = append(*s, t...)
	return
}

func NewSWithCap(c int) (s *S) {
	ss := make([]*T, 0, c)
	return (*S)(&ss)
}

func (s *S) Len() int {
	return len(*s)
}

func (s *S) Less(i, j int) bool {
	// only the first element is compared, this is only used for normalizing
	// filters and the individual tags must be separately sorted.
	return bytes.Compare((*s)[i].T[0], (*s)[j].T[0]) < 0
}

func (s *S) Swap(i, j int) {
	// TODO implement me
	panic("implement me")
}

func (s *S) Append(t ...*T) {
	*s = append(*s, t...)
}

// MarshalJSON encodes a tags.T appended to a provided byte slice in JSON form.
//
// Call bufpool.PutBytes(b) to return the buffer to the bufpool after use.
func (s *S) MarshalJSON() (b []byte, err error) {
	b = bufpool.Get()
	b = append(b, '[')
	for i, ss := range *s {
		b = ss.Marshal(b)
		if i < len(*s)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return
}

func (s *S) Marshal(dst []byte) (b []byte) {
	b = append(dst, '[')
	for i, ss := range *s {
		b = ss.Marshal(b)
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

// GetFirst returns the first tag.T that has the same Key as t.
func (s *S) GetFirst(t []byte) (first *T) {
	for _, tt := range *s {
		if utils.FastEqual(tt.T[0], t) {
			return tt
		}
	}
	return
}
