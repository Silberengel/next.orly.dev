package event

// E is the primary datatype of nostr. This is the form of the structure that
// defines its JSON string-based format.
type E struct {

	// ID is the SHA256 hash of the canonical encoding of the event in binary format
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
	Tags [][]byte

	// Content is an arbitrary string that can contain anything, but usually
	// conforming to a specification relating to the Kind and the Tags.
	Content []byte

	// Sig is the signature on the ID hash that validates as coming from the
	// Pubkey in binary format.
	Sig []byte
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
