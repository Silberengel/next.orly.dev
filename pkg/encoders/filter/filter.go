package filter

import (
	"bytes"
	"sort"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"next.orly.dev/pkg/crypto/ec/schnorr"
	"next.orly.dev/pkg/crypto/sha256"
	"next.orly.dev/pkg/encoders/ints"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/text"
	"next.orly.dev/pkg/encoders/timestamp"
	"next.orly.dev/pkg/utils/pointers"
)

// F is the primary query form for requesting events from a nostr relay.
//
// The ordering of fields of filters is not specified as in the protocol there
// is no requirement to generate a hash for fast recognition of identical
// filters. However, for internal use in a relay, by applying a consistent sort
// order, this library will produce an identical JSON from the same *set* of
// fields no matter what order they were provided.
//
// This is to facilitate the deduplication of filters so an effective identical
// match is not performed on an identical filter.
type F struct {
	Ids     *tag.T       `json:"ids,omitempty"`
	Kinds   *kind.S      `json:"kinds,omitempty"`
	Authors *tag.T       `json:"authors,omitempty"`
	Tags    *tag.S       `json:"-,omitempty"`
	Since   *timestamp.T `json:"since,omitempty"`
	Until   *timestamp.T `json:"until,omitempty"`
	Search  []byte       `json:"search,omitempty"`
	Limit   *uint        `json:"limit,omitempty"`
}

// New creates a new, reasonably initialized filter that will be ready for most uses without
// further allocations.
func New() (f *F) {
	return &F{
		Ids:     tag.NewWithCap(10),
		Kinds:   kind.NewWithCap(10),
		Authors: tag.NewWithCap(10),
		Tags:    tag.NewSWithCap(10),
		Since:   timestamp.New(),
		Until:   timestamp.New(),
	}
}

var (
	// IDs is the JSON object key for IDs.
	IDs = []byte("ids")
	// Kinds is the JSON object key for Kinds.
	Kinds = []byte("kinds")
	// Authors is the JSON object key for Authors.
	Authors = []byte("authors")
	// Since is the JSON object key for Since.
	Since = []byte("since")
	// Until is the JSON object key for Until.
	Until = []byte("until")
	// Limit is the JSON object key for Limit.
	Limit = []byte("limit")
	// Search is the JSON object key for Search.
	Search = []byte("search")
)

// Sort the fields of a filter so a fingerprint on a filter that has the same set of content
// produces the same fingerprint.
func (f *F) Sort() {
	if f.Ids != nil {
		sort.Sort(f.Ids)
	}
	if f.Kinds != nil {
		sort.Sort(f.Kinds)
	}
	if f.Authors != nil {
		sort.Sort(f.Authors)
	}
	if f.Tags != nil {
		for i, v := range *f.Tags {
			if len(v.T) > 2 {
				vv := (v.T)[1:]
				sort.Slice(
					vv, func(i, j int) bool {
						return bytes.Compare((v.T)[i+1], (v.T)[j+1]) < 0
					},
				)
				// keep the first as is, this is the #x prefix
				first := (v.T)[:1]
				// append the sorted values to the prefix
				v.T = append(first, vv...)
				// replace the old value with the sorted one
				(*f.Tags)[i] = v
			}
		}
		sort.Sort(f.Tags)
	}
}

// Marshal a filter into raw JSON bytes, minified. The field ordering and sort
// of fields is canonicalized so that a hash can identify the same filter.
func (f *F) Marshal(dst []byte) (b []byte) {
	var err error
	_ = err
	var first bool
	// sort the fields so they come out the same
	f.Sort()
	// open parentheses
	dst = append(dst, '{')
	if f.Ids != nil && f.Ids.Len() > 0 {
		first = true
		dst = text.JSONKey(dst, IDs)
		dst = text.MarshalHexArray(dst, f.Ids.T)
	}
	if f.Kinds.Len() > 0 {
		if first {
			dst = append(dst, ',')
		} else {
			first = true
		}
		dst = text.JSONKey(dst, Kinds)
		dst = f.Kinds.Marshal(dst)
	}
	if f.Authors.Len() > 0 {
		if first {
			dst = append(dst, ',')
		} else {
			first = true
		}
		dst = text.JSONKey(dst, Authors)
		dst = text.MarshalHexArray(dst, f.Authors.T)
	}
	if f.Tags.Len() > 0 {
		// tags are stored as tags with the initial element the "#a" and the rest the list in
		// each element of the tags list. eg:
		//
		//     [["#p","<pubkey1>","<pubkey3"],["#t","hashtag","stuff"]]
		//
		for _, tg := range *f.Tags {
			if tg == nil {
				// nothing here
				continue
			}
			if tg.Len() < 2 {
				// must have at least key and one value
				continue
			}
			tKey := tg.T[0]
			if len(tKey) != 1 ||
				((tKey[0] < 'a' || tKey[0] > 'z') && (tKey[0] < 'A' || tKey[0] > 'Z')) {
				// key must be single alpha character
				continue
			}
			values := tg.T[1:]
			if len(values) == 0 {
				continue
			}
			if first {
				dst = append(dst, ',')
			} else {
				first = true
			}
			// append the key with # prefix
			dst = append(dst, '"', '#', tKey[0], '"', ':')
			dst = append(dst, '[')
			for i, value := range values {
				dst = append(dst, '"')
				dst = append(dst, value...)
				dst = append(dst, '"')
				if i < len(values)-1 {
					dst = append(dst, ',')
				}
			}
			dst = append(dst, ']')
		}
	}
	if f.Since != nil && f.Since.U64() > 0 {
		if first {
			dst = append(dst, ',')
		} else {
			first = true
		}
		dst = text.JSONKey(dst, Since)
		dst = f.Since.Marshal(dst)
	}
	if f.Until != nil && f.Until.U64() > 0 {
		if first {
			dst = append(dst, ',')
		} else {
			first = true
		}
		dst = text.JSONKey(dst, Until)
		dst = f.Until.Marshal(dst)
	}
	if len(f.Search) > 0 {
		if first {
			dst = append(dst, ',')
		} else {
			first = true
		}
		dst = text.JSONKey(dst, Search)
		dst = text.AppendQuote(dst, f.Search, text.NostrEscape)
	}
	if pointers.Present(f.Limit) {
		if first {
			dst = append(dst, ',')
		} else {
			first = true
		}
		dst = text.JSONKey(dst, Limit)
		dst = ints.New(*f.Limit).Marshal(dst)
	}
	// close parentheses
	dst = append(dst, '}')
	b = dst
	return
}

// Serialize a filter.F into raw minified JSON bytes.
func (f *F) Serialize() (b []byte) { return f.Marshal(nil) }

// states of the unmarshaler
const (
	beforeOpen = iota
	openParen
	inKey
	inKV
	inVal
	betweenKV
	afterClose
)

// Unmarshal a filter from raw (minified) JSON bytes into the runtime format.
//
// todo: this may tolerate whitespace, not certain currently.
func (f *F) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	var key []byte
	var state int
	for ; len(r) > 0; r = r[1:] {
		// log.I.ToSliceOfBytes("%c", rem[0])
		switch state {
		case beforeOpen:
			if r[0] == '{' {
				state = openParen
				// log.I.Ln("openParen")
			}
		case openParen:
			if r[0] == '"' {
				state = inKey
				// log.I.Ln("inKey")
			}
		case inKey:
			if r[0] == '"' {
				state = inKV
				// log.I.Ln("inKV")
			} else {
				key = append(key, r[0])
			}
		case inKV:
			if r[0] == ':' {
				state = inVal
			}
		case inVal:
			if len(key) < 1 {
				err = errorf.E("filter key zero length: '%s'\n'%s", b, r)
				return
			}
			switch key[0] {
			case '#':
				// tags start with # and have 1 letter
				l := len(key)
				if l != 2 {
					err = errorf.E(
						"filter tag keys can only be # and one alpha character: '%s'\n%s",
						key, b,
					)
					return
				}
				k := make([]byte, len(key))
				copy(k, key)
				var ff [][]byte
				if ff, r, err = text.UnmarshalStringArray(r); chk.E(err) {
					return
				}
				ff = append([][]byte{k}, ff...)
				s := append(*f.Tags, tag.NewFromByteSlice(ff...))
				f.Tags = &s
				// f.Tags.F = append(f.Tags.F, tag.New(ff...))
				// }
				state = betweenKV
			case IDs[0]:
				if len(key) < len(IDs) {
					goto invalid
				}
				var ff [][]byte
				if ff, r, err = text.UnmarshalHexArray(
					r, sha256.Size,
				); chk.E(err) {
					return
				}
				f.Ids = tag.NewFromByteSlice(ff...)
				state = betweenKV
			case Kinds[0]:
				if len(key) < len(Kinds) {
					goto invalid
				}
				f.Kinds = kind.NewWithCap(0)
				if r, err = f.Kinds.Unmarshal(r); chk.E(err) {
					return
				}
				state = betweenKV
			case Authors[0]:
				if len(key) < len(Authors) {
					goto invalid
				}
				var ff [][]byte
				if ff, r, err = text.UnmarshalHexArray(
					r, schnorr.PubKeyBytesLen,
				); chk.E(err) {
					return
				}
				f.Authors = tag.NewFromByteSlice(ff...)
				state = betweenKV
			case Until[0]:
				if len(key) < len(Until) {
					goto invalid
				}
				u := ints.New(0)
				if r, err = u.Unmarshal(r); chk.E(err) {
					return
				}
				f.Until = timestamp.FromUnix(int64(u.N))
				state = betweenKV
			case Limit[0]:
				if len(key) < len(Limit) {
					goto invalid
				}
				l := ints.New(0)
				if r, err = l.Unmarshal(r); chk.E(err) {
					return
				}
				u := uint(l.N)
				f.Limit = &u
				state = betweenKV
			case Search[0]:
				if len(key) < len(Since) {
					goto invalid
				}
				switch key[1] {
				case Search[1]:
					if len(key) < len(Search) {
						goto invalid
					}
					var txt []byte
					if txt, r, err = text.UnmarshalQuoted(r); chk.E(err) {
						return
					}
					f.Search = txt
					// log.I.ToSliceOfBytes("\n%s\n%s", txt, rem)
					state = betweenKV
					// log.I.Ln("betweenKV")
				case Since[1]:
					if len(key) < len(Since) {
						goto invalid
					}
					s := ints.New(0)
					if r, err = s.Unmarshal(r); chk.E(err) {
						return
					}
					f.Since = timestamp.FromUnix(int64(s.N))
					state = betweenKV
					// log.I.Ln("betweenKV")
				}
			default:
				goto invalid
			}
			key = key[:0]
		case betweenKV:
			if len(r) == 0 {
				return
			}
			if r[0] == '}' {
				state = afterClose
				// log.I.Ln("afterClose")
				// rem = rem[1:]
			} else if r[0] == ',' {
				state = openParen
				// log.I.Ln("openParen")
			} else if r[0] == '"' {
				state = inKey
				// log.I.Ln("inKey")
			}
		}
		if len(r) == 0 {
			return
		}
		if r[0] == '}' {
			r = r[1:]
			return
		}
	}
invalid:
	err = errorf.E("invalid key,\n'%s'\n'%s'", string(b), string(r))
	return
}
