package event

import (
	"next.orly.dev/pkg/crypto/sha256"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/ints"
	"next.orly.dev/pkg/encoders/text"
)

// ToCanonical converts the event to the canonical encoding used to derive the
// event ID.
func (ev *E) ToCanonical(dst []byte) (b []byte) {
	b = dst
	b = append(b, "[0,\""...)
	b = hex.EncAppend(b, ev.Pubkey)
	b = append(b, "\","...)
	b = ints.New(ev.CreatedAt).Marshal(b)
	b = append(b, ',')
	b = ints.New(ev.Kind).Marshal(b)
	b = append(b, ',')
	b = ev.Tags.Marshal(b)
	b = append(b, ',')
	b = text.AppendQuote(b, ev.Content, text.NostrEscape)
	b = append(b, ']')
	// log.D.F("canonical: %s", b)
	return
}

// GetIDBytes returns the raw SHA256 hash of the canonical form of an event.E.
func (ev *E) GetIDBytes() []byte { return Hash(ev.ToCanonical(nil)) }

// Hash is a little helper generate a hash and return a slice instead of an
// array.
func Hash(in []byte) (out []byte) {
	h := sha256.Sum256(in)
	return h[:]
}
