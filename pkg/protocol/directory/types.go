// Package directory provides data structures and validation for the distributed
// directory consensus protocol as defined in NIP-XX.
//
// This package implements message encoding and validation for the following
// event kinds:
//   - 39100: Relay Identity Announcement
//   - 39101: Trust Act
//   - 39102: Group Tag Act
//   - 39103: Public Key Advertisement
//   - 39104: Directory Event Replication Request
//   - 39105: Directory Event Replication Response
//
// # Legal Concept of Acts
//
// The term "act" in this protocol draws from legal terminology, where an act
// represents a formal declaration or testimony that has legal significance.
// Similar to legal instruments such as:
//
//   - Deed Poll: A legal document binding one party to a particular course of action
//   - Witness Testimony: A formal statement given under oath as evidence
//   - Affidavit: A written statement confirmed by oath for use as evidence
//
// In the context of this protocol, acts serve as cryptographically signed
// declarations that establish trust relationships, group memberships, or other
// formal statements within the relay consortium. Like their legal counterparts,
// these acts:
//
//   - Are formally structured with specific required elements
//   - Carry the authority and responsibility of the signing party
//   - Create binding relationships or obligations within the consortium
//   - Can be verified for authenticity through cryptographic signatures
//   - May have expiration dates or other temporal constraints
//
// This legal framework provides a conceptual foundation for understanding the
// formal nature and binding character of consortium declarations.
package directory

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
)

// Event kinds for the distributed directory consensus protocol
var (
	RelayIdentityAnnouncementKind         = kind.New(39100)
	TrustActKind                          = kind.New(39101)
	GroupTagActKind                       = kind.New(39102)
	PublicKeyAdvertisementKind            = kind.New(39103)
	DirectoryEventReplicationRequestKind  = kind.New(39104)
	DirectoryEventReplicationResponseKind = kind.New(39105)
)

// Common tag names used across directory protocol messages
var (
	DTag              = []byte("d")
	RelayTag          = []byte("relay")
	SigningKeyTag     = []byte("signing_key")
	EncryptionKeyTag  = []byte("encryption_key")
	VersionTag        = []byte("version")
	NIP11URLTag       = []byte("nip11_url")
	PubkeyTag         = []byte("p")
	TrustLevelTag     = []byte("trust_level")
	ExpiryTag         = []byte("expiry")
	ReasonTag         = []byte("reason")
	KTag              = []byte("K")
	ITag              = []byte("I")
	GroupTagTag       = []byte("group_tag")
	ActorTag          = []byte("actor")
	ConfidenceTag     = []byte("confidence")
	PurposeTag        = []byte("purpose")
	AlgorithmTag      = []byte("algorithm")
	DerivationPathTag = []byte("derivation_path")
	KeyIndexTag       = []byte("key_index")
	RequestIDTag      = []byte("request_id")
	EventIDTag        = []byte("event_id")
	StatusTag         = []byte("status")
	ErrorTag          = []byte("error")
)

// Trust levels for trust acts
type TrustLevel string

const (
	TrustLevelHigh   TrustLevel = "high"
	TrustLevelMedium TrustLevel = "medium"
	TrustLevelLow    TrustLevel = "low"
)

// Reason types for trust establishment
type TrustReason string

const (
	TrustReasonManual    TrustReason = "manual"
	TrustReasonAutomatic TrustReason = "automatic"
	TrustReasonInherited TrustReason = "inherited"
)

// Key purposes for public key advertisements
type KeyPurpose string

const (
	KeyPurposeSigning    KeyPurpose = "signing"
	KeyPurposeEncryption KeyPurpose = "encryption"
	KeyPurposeDelegation KeyPurpose = "delegation"
)

// Replication status codes
type ReplicationStatus string

const (
	ReplicationStatusSuccess ReplicationStatus = "success"
	ReplicationStatusError   ReplicationStatus = "error"
	ReplicationStatusPending ReplicationStatus = "pending"
)

// GenerateNonce creates a cryptographically secure random nonce for use in
// identity tags and other protocol messages.
func GenerateNonce(size int) (nonce []byte, err error) {
	if size <= 0 {
		size = 16 // Default to 16 bytes
	}
	nonce = make([]byte, size)
	if _, err = rand.Read(nonce); chk.E(err) {
		return
	}
	return
}

// GenerateNonceHex creates a hex-encoded nonce of the specified byte size.
func GenerateNonceHex(size int) (nonceHex string, err error) {
	var nonce []byte
	if nonce, err = GenerateNonce(size); chk.E(err) {
		return
	}
	nonceHex = hex.EncodeToString(nonce)
	return
}

// IsDirectoryEventKind returns true if the given kind is a directory event
// that should always be replicated among consortium members.
//
// Directory events include:
//   - Kind 0: User Metadata
//   - Kind 3: Follow Lists
//   - Kind 5: Event Deletion Requests
//   - Kind 1984: Reporting
//   - Kind 10002: Relay List Metadata
//   - Kind 10000: Mute Lists
//   - Kind 10050: DM Relay Lists
func IsDirectoryEventKind(k uint16) (isDirectory bool) {
	switch k {
	case 0, 3, 5, 1984, 10002, 10000, 10050:
		return true
	default:
		return false
	}
}

// ValidateTrustLevel checks if the provided trust level is valid.
func ValidateTrustLevel(level string) (err error) {
	switch TrustLevel(level) {
	case TrustLevelHigh, TrustLevelMedium, TrustLevelLow:
		return nil
	default:
		return errorf.E("invalid trust level: %s", level)
	}
}

// ValidateKeyPurpose checks if the provided key purpose is valid.
func ValidateKeyPurpose(purpose string) (err error) {
	switch KeyPurpose(purpose) {
	case KeyPurposeSigning, KeyPurposeEncryption, KeyPurposeDelegation:
		return nil
	default:
		return errorf.E("invalid key purpose: %s", purpose)
	}
}

// ValidateReplicationStatus checks if the provided replication status is valid.
func ValidateReplicationStatus(status string) (err error) {
	switch ReplicationStatus(status) {
	case ReplicationStatusSuccess, ReplicationStatusError, ReplicationStatusPending:
		return nil
	default:
		return errorf.E("invalid replication status: %s", status)
	}
}

// CreateBaseEvent creates a basic event structure with common fields set.
func CreateBaseEvent(pubkey []byte, k *kind.K) (ev *event.E) {
	return &event.E{
		Pubkey:    pubkey,
		CreatedAt: time.Now().Unix(),
		Kind:      k.K,
		Tags:      tag.NewS(),
		Content:   []byte(""),
	}
}
