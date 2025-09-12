package event

import (
	"fmt"
	"io"

	"github.com/templexxx/xhex"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/crypto/ec/schnorr"
	"next.orly.dev/pkg/crypto/sha256"
	"next.orly.dev/pkg/encoders/ints"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/text"
	"next.orly.dev/pkg/utils"
	"next.orly.dev/pkg/utils/bufpool"
)

// E is the primary datatype of nostr. This is the form of the structure that
// defines its JSON string-based format. Always use New() and Free() to create
// and free event.E to take advantage of the bufpool which greatly improves
// memory allocation behaviour when encoding and decoding nostr events.
//
// WARNING: DO NOT use json.Marshal with this type because it will not properly
// encode <, >, and & characters due to legacy bullcrap in the encoding/json
// library. Either call MarshalJSON directly or use a json.Encoder with html
// escaping disabled.
//
// Or import "next.orly.dev/pkg/encoders/json" and use json.Marshal which is the
// same as go 1.25 json v1 except with this one stupidity removed.
type E struct {

	// ID is the SHA256 hash of the canonical encoding of the event in binary
	// format
	ID []byte

	// Pubkey is the public key of the event creator in binary format
	Pubkey []byte

	// CreatedAt is the UNIX timestamp of the event according to the event
	// creator (never trust a timestamp!)
	CreatedAt int64

	// Kind is the nostr protocol code for the type of event. See kind.T
	Kind uint16

	// Tags are a list of tags, which are a list of strings usually structured
	// as a 3-layer scheme indicating specific features of an event.
	Tags *tag.S

	// Content is an arbitrary string that can contain anything, but usually
	// conforming to a specification relating to the Kind and the Tags.
	Content []byte

	// Sig is the signature on the ID hash that validates as coming from the
	// Pubkey in binary format.
	Sig []byte

	// b is the decode buffer for the event.E. this is where the UnmarshalJSON
	// will source the memory to store all of the fields except for the tags.
	b bufpool.B
}

var (
	jId        = []byte("id")
	jPubkey    = []byte("pubkey")
	jCreatedAt = []byte("created_at")
	jKind      = []byte("kind")
	jTags      = []byte("tags")
	jContent   = []byte("content")
	jSig       = []byte("sig")
)

// New returns a new event.E. The returned event.E should be freed with Free()
// to return the unmarshalling buffer to the bufpool.
func New() *E {
	return &E{
		b: bufpool.Get(),
	}
}

// Free returns the event.E to the pool, as well as nilling all of the fields.
// This should hint to the GC that the event.E can be freed, and the memory
// reused. The decode buffer will be returned to the pool for reuse.
func (ev *E) Free() {
	bufpool.Put(ev.b)
	ev.ID = nil
	ev.Pubkey = nil
	ev.Tags = nil
	ev.Content = nil
	ev.Sig = nil
	ev.b = nil
}

// EstimateSize returns a size for the event that allows for worst case scenario
// expansion of the escaped content and tags.
func (ev *E) EstimateSize() (size int) {
	size = len(ev.ID)*2 + len(ev.Pubkey)*2 + len(ev.Sig)*2 + len(ev.Content)*2
	for _, v := range *ev.Tags {
		for _, w := range (*v).T {
			size += len(w) * 2
		}
	}
	return
}

func (ev *E) Marshal(dst []byte) (b []byte) {
	b = dst
	b = append(b, '{')
	b = append(b, '"')
	b = append(b, jId...)
	b = append(b, `":"`...)
	// it can happen the slice has insufficient capacity to hold the content AND
	// the signature at this point, because the signature encoder must have
	// sufficient capacity pre-allocated as it does not append to the buffer.
	// unlike every other encoding function up to this point. This also ensures
	// that since the bufpool defaults to 1kb, most events won't have a
	// re-allocation required, but if they do, it will be this next one, and it
	// integrates properly with the buffer pool, reducing GC pressure and
	// avoiding new heap allocations.
	if cap(b) < len(b)+len(ev.Content)+7+256+2 {
		b2 := make([]byte, len(b)+len(ev.Content)*2+1024)
		copy(b2, b)
		b2 = b2[:len(b)]
		// return the old buffer to the pool for reuse.
		bufpool.PutBytes(b)
		b = b2
	}
	b = b[:len(b)+2*sha256.Size]
	xhex.Encode(b[len(b)-2*sha256.Size:], ev.ID)
	b = append(b, `","`...)
	b = append(b, jPubkey...)
	b = append(b, `":"`...)
	b = b[:len(b)+2*schnorr.PubKeyBytesLen]
	xhex.Encode(b[len(b)-2*schnorr.PubKeyBytesLen:], ev.Pubkey)
	b = append(b, `","`...)
	b = append(b, jCreatedAt...)
	b = append(b, `":`...)
	b = ints.New(ev.CreatedAt).Marshal(b)
	b = append(b, `,"`...)
	b = append(b, jKind...)
	b = append(b, `":`...)
	b = ints.New(ev.Kind).Marshal(b)
	b = append(b, `,"`...)
	b = append(b, jTags...)
	b = append(b, `":`...)
	if ev.Tags != nil {
		b = ev.Tags.Marshal(b)
	}
	b = append(b, `,"`...)
	b = append(b, jContent...)
	b = append(b, `":"`...)
	b = text.NostrEscape(b, ev.Content)
	b = append(b, `","`...)
	b = append(b, jSig...)
	b = append(b, `":"`...)
	b = b[:len(b)+2*schnorr.SignatureSize]
	xhex.Encode(b[len(b)-2*schnorr.SignatureSize:], ev.Sig)
	b = append(b, `"}`...)
	return
}

// MarshalJSON marshals an event.E into a JSON byte string.
//
// Call bufpool.PutBytes(b) to return the buffer to the bufpool after use.
//
// WARNING: if json.Marshal is called in the hopes of invoking this function on
// an event, if it has <, > or * in the content or tags they are escaped into
// unicode escapes and break the event ID. Call this function directly in order
// to bypass this issue.
func (ev *E) MarshalJSON() (b []byte, err error) {
	b = bufpool.Get()
	b = ev.Marshal(b[:0])
	return
}

func (ev *E) Serialize() (b []byte) {
	b = bufpool.Get()
	b = ev.Marshal(b[:0])
	return
}

// Unmarshal unmarshalls a JSON string into an event.E.
//
// Call ev.Free() to return the provided buffer to the bufpool afterwards.
func (ev *E) Unmarshal(b []byte) (rem []byte, err error) {
	key := make([]byte, 0, 9)
	for ; len(b) > 0; b = b[1:] {
		// Skip whitespace
		if isWhitespace(b[0]) {
			continue
		}
		if b[0] == '{' {
			b = b[1:]
			goto BetweenKeys
		}
	}
	log.I.F("start")
	goto eof
BetweenKeys:
	for ; len(b) > 0; b = b[1:] {
		// Skip whitespace
		if isWhitespace(b[0]) {
			continue
		}
		if b[0] == '"' {
			b = b[1:]
			goto InKey
		}
	}
	log.I.F("BetweenKeys")
	goto eof
InKey:
	for ; len(b) > 0; b = b[1:] {
		if b[0] == '"' {
			b = b[1:]
			goto InKV
		}
		key = append(key, b[0])
	}
	log.I.F("InKey")
	goto eof
InKV:
	for ; len(b) > 0; b = b[1:] {
		// Skip whitespace
		if isWhitespace(b[0]) {
			continue
		}
		if b[0] == ':' {
			b = b[1:]
			goto InVal
		}
	}
	log.I.F("InKV")
	goto eof
InVal:
	// Skip whitespace before value
	for len(b) > 0 && isWhitespace(b[0]) {
		b = b[1:]
	}
	switch key[0] {
	case jId[0]:
		if !utils.FastEqual(jId, key) {
			goto invalid
		}
		var id []byte
		if id, b, err = text.UnmarshalHex(b); chk.E(err) {
			return
		}
		if len(id) != sha256.Size {
			err = errorf.E(
				"invalid Subscription, require %d got %d", sha256.Size,
				len(id),
			)
			return
		}
		ev.ID = id
		goto BetweenKV
	case jPubkey[0]:
		if !utils.FastEqual(jPubkey, key) {
			goto invalid
		}
		var pk []byte
		if pk, b, err = text.UnmarshalHex(b); chk.E(err) {
			return
		}
		if len(pk) != schnorr.PubKeyBytesLen {
			err = errorf.E(
				"invalid pubkey, require %d got %d",
				schnorr.PubKeyBytesLen, len(pk),
			)
			return
		}
		ev.Pubkey = pk
		goto BetweenKV
	case jKind[0]:
		if !utils.FastEqual(jKind, key) {
			goto invalid
		}
		k := kind.New(0)
		if b, err = k.Unmarshal(b); chk.E(err) {
			return
		}
		ev.Kind = k.ToU16()
		goto BetweenKV
	case jTags[0]:
		if !utils.FastEqual(jTags, key) {
			goto invalid
		}
		ev.Tags = new(tag.S)
		if b, err = ev.Tags.Unmarshal(b); chk.E(err) {
			return
		}
		goto BetweenKV
	case jSig[0]:
		if !utils.FastEqual(jSig, key) {
			goto invalid
		}
		var sig []byte
		if sig, b, err = text.UnmarshalHex(b); chk.E(err) {
			return
		}
		if len(sig) != schnorr.SignatureSize {
			err = errorf.E(
				"invalid sig length, require %d got %d '%s'\n%s",
				schnorr.SignatureSize, len(sig), b, b,
			)
			return
		}
		ev.Sig = sig
		goto BetweenKV
	case jContent[0]:
		if key[1] == jContent[1] {
			if !utils.FastEqual(jContent, key) {
				goto invalid
			}
			if ev.Content, b, err = text.UnmarshalQuoted(b); chk.T(err) {
				return
			}
			goto BetweenKV
		} else if key[1] == jCreatedAt[1] {
			if !utils.FastEqual(jCreatedAt, key) {
				goto invalid
			}
			i := ints.New(0)
			if b, err = i.Unmarshal(b); chk.T(err) {
				return
			}
			ev.CreatedAt = i.Int64()
			goto BetweenKV
		} else {
			goto invalid
		}
	default:
		goto invalid
	}
BetweenKV:
	key = key[:0]
	for ; len(b) > 0; b = b[1:] {
		// Skip whitespace
		if isWhitespace(b[0]) {
			continue
		}
		switch {
		case len(b) == 0:
			return
		case b[0] == '}':
			b = b[1:]
			goto AfterClose
		case b[0] == ',':
			b = b[1:]
			goto BetweenKeys
		case b[0] == '"':
			b = b[1:]
			goto InKey
		}
	}
	log.I.F("between kv")
	goto eof
AfterClose:
	rem = b
	return
invalid:
	err = fmt.Errorf(
		"invalid key,\n'%s'\n'%s'\n'%s'", string(b), string(b[:len(b)]),
		string(b),
	)
	return
eof:
	err = io.EOF
	return
}

// UnmarshalJSON unmarshalls a JSON string into an event.E.
//
// Call ev.Free() to return the provided buffer to the bufpool afterwards.
func (ev *E) UnmarshalJSON(b []byte) (err error) {
	_, err = ev.Unmarshal(b)
	return
}

// isWhitespace returns true if the byte is a whitespace character (space, tab, newline, carriage return).
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// S is an array of event.E that sorts in reverse chronological order.
type S []*E

// Len returns the length of the event.Es.
func (ev S) Len() int { return len(ev) }

// Less returns whether the first is newer than the second (larger unix
// timestamp).
func (ev S) Less(i, j int) bool { return ev[i].CreatedAt > ev[j].CreatedAt }

// Swap two indexes of the event.Es with each other.
func (ev S) Swap(i, j int) { ev[i], ev[j] = ev[j], ev[i] }

// C is a channel that carries event.E.
type C chan *E
