// Package p8k provides a signer.I implementation using p8k.mleku.dev
package p8k

import (
	"crypto/rand"

	"lol.mleku.dev/errorf"
	"next.orly.dev/pkg/crypto/ec/schnorr"
	"next.orly.dev/pkg/crypto/ec/secp256k1"
	secp "next.orly.dev/pkg/crypto/p8k"
	"next.orly.dev/pkg/interfaces/signer"
)

// Signer implements the signer.I interface using p8k.mleku.dev or pure Go fallback
type Signer struct {
	// libsecp256k1 implementation
	ctx     *secp.Context
	secKey  []byte
	pubKey  []byte
	keypair secp.Keypair

	// Pure Go fallback implementation
	fallback *FallbackSigner
}

// FallbackSigner implements the signer.I interface using pure Go btcec/secp256k1
type FallbackSigner struct {
	privKey  *secp256k1.SecretKey
	pubKey   *secp256k1.PublicKey
	xonlyPub []byte
}

// Ensure Signer implements signer.I
var _ signer.I = (*Signer)(nil)

// New creates a new P8K signer, falling back to pure Go implementation if libsecp256k1 is unavailable
func New() (s *Signer, err error) {
	var ctx *secp.Context
	if ctx, err = secp.NewContext(secp.ContextSign | secp.ContextVerify); err != nil {
		// Fallback to pure Go implementation
		fallback, fallbackErr := newFallbackSigner()
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		s = &Signer{fallback: fallback}
		return s, nil
	}
	s = &Signer{ctx: ctx}
	return s, nil
}

// MustNew creates a new P8K signer and panics on error
func MustNew() *Signer {
	s, err := New()
	if err != nil {
		panic(err)
	}
	return s
}

// newFallbackSigner creates a new fallback signer using pure Go implementation
func newFallbackSigner() (*FallbackSigner, error) {
	return &FallbackSigner{}, nil
}

// Generate creates a fresh new key pair from system entropy, and ensures it is even (so
// ECDH works).
func (s *Signer) Generate() (err error) {
	if s.fallback != nil {
		return s.fallback.Generate()
	}

	s.secKey = make([]byte, 32)
	if _, err = rand.Read(s.secKey); err != nil {
		return
	}

	// Create keypair
	if s.keypair, err = s.ctx.CreateKeypair(s.secKey); err != nil {
		return
	}

	// Extract x-only public key (internal 64-byte format)
	var xonly secp.XOnlyPublicKey
	var parity int32
	if xonly, parity, err = s.ctx.KeypairXOnlyPub(s.keypair); err != nil {
		return
	}
	_ = parity

	// Serialize the x-only public key to 32 bytes
	if s.pubKey, err = s.ctx.SerializeXOnlyPublicKey(xonly[:]); err != nil {
		return
	}
	return
}

// InitSec initialises the secret (signing) key from the raw bytes, and also
// derives the public key because it can.
func (s *Signer) InitSec(sec []byte) (err error) {
	if s.fallback != nil {
		return s.fallback.InitSec(sec)
	}

	if len(sec) != 32 {
		return errorf.E("secret key must be 32 bytes")
	}

	s.secKey = make([]byte, 32)
	copy(s.secKey, sec)

	// Create keypair
	if s.keypair, err = s.ctx.CreateKeypair(s.secKey); err != nil {
		return
	}

	// Extract x-only public key (internal 64-byte format)
	var xonly secp.XOnlyPublicKey
	var parity int32
	if xonly, parity, err = s.ctx.KeypairXOnlyPub(s.keypair); err != nil {
		return
	}
	_ = parity

	// Serialize the x-only public key to 32 bytes
	if s.pubKey, err = s.ctx.SerializeXOnlyPublicKey(xonly[:]); err != nil {
		return
	}
	return
}

// InitPub initializes the public (verification) key from raw bytes, this is
// expected to be an x-only 32 byte pubkey.
func (s *Signer) InitPub(pub []byte) (err error) {
	if s.fallback != nil {
		return s.fallback.InitPub(pub)
	}

	if len(pub) != 32 {
		return errorf.E("public key must be 32 bytes")
	}

	s.pubKey = make([]byte, 32)
	copy(s.pubKey, pub)
	return
}

// Sec returns the secret key bytes.
func (s *Signer) Sec() []byte {
	if s.fallback != nil {
		return s.fallback.Sec()
	}
	return s.secKey
}

// Pub returns the public key bytes (x-only schnorr pubkey).
func (s *Signer) Pub() []byte {
	if s.fallback != nil {
		return s.fallback.Pub()
	}
	return s.pubKey
}

// PubCompressed returns the compressed public key (33 bytes with 0x02/0x03 prefix).
// This is needed for ECDH operations like NIP-44.
func (s *Signer) PubCompressed() (compressed []byte, err error) {
	if s.fallback != nil {
		// For fallback, we need to derive the compressed key from the x-only key
		if s.fallback.pubKey == nil {
			return nil, errorf.E("public key not initialized")
		}
		return s.fallback.pubKey.SerializeCompressed(), nil
	}

	if len(s.keypair) == 0 {
		return nil, errorf.E("keypair not initialized")
	}

	// Get the internal public key from keypair
	var pubkeyInternal []byte
	if pubkeyInternal, err = s.ctx.KeypairPub(s.keypair); err != nil {
		return
	}

	// Serialize as compressed (33 bytes)
	if compressed, err = s.ctx.SerializePublicKeyCompressed(pubkeyInternal); err != nil {
		return
	}

	return
}

// Sign creates a signature using the stored secret key.
func (s *Signer) Sign(msg []byte) (sig []byte, err error) {
	if s.fallback != nil {
		return s.fallback.Sign(msg)
	}

	if len(s.keypair) == 0 {
		return nil, errorf.E("keypair not initialized")
	}

	// Generate auxiliary randomness
	auxRand := make([]byte, 32)
	if _, err = rand.Read(auxRand); err != nil {
		return
	}

	// Sign with Schnorr
	if sig, err = s.ctx.SchnorrSign(msg, s.keypair, auxRand); err != nil {
		return
	}

	return
}

// Verify checks a message hash and signature match the stored public key.
func (s *Signer) Verify(msg, sig []byte) (valid bool, err error) {
	if s.fallback != nil {
		return s.fallback.Verify(msg, sig)
	}

	if s.pubKey == nil {
		return false, errorf.E("public key not initialized")
	}

	if valid, err = s.ctx.SchnorrVerify(sig, msg, s.pubKey); err != nil {
		return
	}

	return
}

// Zero wipes the secret key to prevent memory leaks.
func (s *Signer) Zero() {
	if s.fallback != nil {
		s.fallback.Zero()
		return
	}

	if s.secKey != nil {
		for i := range s.secKey {
			s.secKey[i] = 0
		}
	}
	if len(s.keypair) > 0 {
		for i := range s.keypair {
			s.keypair[i] = 0
		}
	}
}

// ECDH returns a shared secret derived using Elliptic Curve Diffie-Hellman on
// the signer's secret and provided pubkey.
func (s *Signer) ECDH(pub []byte) (secret []byte, err error) {
	return s.ECDHRaw(pub)
}

// ECDHRaw returns the raw shared secret point (x-coordinate only, 32 bytes) without hashing.
// This is needed for protocols like NIP-44 that do their own key derivation.
// The pub parameter can be either:
// - 32 bytes (x-only): will be converted to compressed format by trying 0x02 then 0x03
// - 33 bytes (compressed): will be used as-is
func (s *Signer) ECDHRaw(pub []byte) (sharedX []byte, err error) {
	if s.fallback != nil {
		return s.fallback.ECDHRaw(pub)
	}

	if s.secKey == nil {
		return nil, errorf.E("secret key not initialized")
	}

	var pubKeyFull []byte

	if len(pub) == 33 {
		// Already compressed format (0x02 or 0x03 prefix)
		pubKeyFull = pub
	} else if len(pub) == 32 {
		// X-only format: try with 0x02 (even y), then try 0x03 (odd y) if that fails
		pubKeyFull = make([]byte, 33)
		pubKeyFull[0] = 0x02 // compressed even y
		copy(pubKeyFull[1:], pub)
	} else {
		return nil, errorf.E("public key must be 32 bytes (x-only) or 33 bytes (compressed), got %d bytes", len(pub))
	}

	// Parse the public key
	var pubKeyInternal []byte
	if pubKeyInternal, err = s.ctx.ParsePublicKey(pubKeyFull); err != nil {
		// If 32-byte x-only and even y failed, try odd y
		if len(pub) == 32 {
			pubKeyFull[0] = 0x03
			if pubKeyInternal, err = s.ctx.ParsePublicKey(pubKeyFull); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Compute ECDH - this returns the 32-byte x-coordinate of the shared point
	if sharedX, err = s.ctx.ECDH(pubKeyInternal, s.secKey); err != nil {
		return
	}

	return
}

// FallbackSigner method implementations

// Generate creates a fresh new key pair from system entropy
func (s *FallbackSigner) Generate() (err error) {
	// Generate a new private key
	if s.privKey, err = secp256k1.GenerateSecretKey(); err != nil {
		return errorf.E("failed to generate private key: %w", err)
	}

	// Derive public key
	if s.pubKey = s.privKey.PubKey(); s.pubKey == nil {
		return errorf.E("failed to derive public key")
	}

	// Get x-only public key (32 bytes) - compressed without the 0x02/0x03 prefix
	compressed := s.pubKey.SerializeCompressed()
	s.xonlyPub = make([]byte, 32)
	copy(s.xonlyPub, compressed[1:])

	return nil
}

// InitSec initializes the secret key from raw bytes
func (s *FallbackSigner) InitSec(sec []byte) (err error) {
	if len(sec) != 32 {
		return errorf.E("secret key must be 32 bytes")
	}

	// Create private key from bytes
	s.privKey = secp256k1.SecKeyFromBytes(sec)
	if s.privKey.Key.IsZero() {
		return errorf.E("invalid secret key")
	}

	// Derive public key
	if s.pubKey = s.privKey.PubKey(); s.pubKey == nil {
		return errorf.E("failed to derive public key")
	}

	// Get x-only public key (32 bytes) - compressed without the 0x02/0x03 prefix
	compressed := s.pubKey.SerializeCompressed()
	s.xonlyPub = make([]byte, 32)
	copy(s.xonlyPub, compressed[1:])

	return nil
}

// InitPub initializes the public key from raw bytes (x-only 32 bytes)
func (s *FallbackSigner) InitPub(pub []byte) (err error) {
	if len(pub) != 32 {
		return errorf.E("public key must be 32 bytes")
	}

	s.xonlyPub = make([]byte, 32)
	copy(s.xonlyPub, pub)

	// Parse the x-only public key into a full public key for verification
	if s.pubKey, err = schnorr.ParsePubKey(pub); err != nil {
		return errorf.E("failed to parse public key: %w", err)
	}

	return nil
}

// Sec returns the secret key bytes
func (s *FallbackSigner) Sec() []byte {
	if s.privKey == nil {
		return nil
	}
	return s.privKey.Serialize()
}

// Pub returns the public key bytes (x-only schnorr pubkey)
func (s *FallbackSigner) Pub() []byte {
	return s.xonlyPub
}

// Sign creates a signature using the stored secret key
func (s *FallbackSigner) Sign(msg []byte) (sig []byte, err error) {
	if s.privKey == nil {
		return nil, errorf.E("private key not initialized")
	}

	// Generate auxiliary randomness for BIP-340
	var auxRand [32]byte
	if _, err = rand.Read(auxRand[:]); err != nil {
		return nil, errorf.E("failed to generate aux randomness: %w", err)
	}

	// Sign using Schnorr
	var schnorrSig *schnorr.Signature
	if schnorrSig, err = schnorr.Sign(s.privKey, msg, schnorr.CustomNonce(auxRand)); err != nil {
		return nil, errorf.E("failed to sign: %w", err)
	}

	return schnorrSig.Serialize(), nil
}

// Verify checks a message hash and signature match the stored public key
func (s *FallbackSigner) Verify(msg, sig []byte) (valid bool, err error) {
	if s.pubKey == nil {
		return false, errorf.E("public key not initialized")
	}

	// Parse signature
	var schnorrSig *schnorr.Signature
	if schnorrSig, err = schnorr.ParseSignature(sig); err != nil {
		return false, errorf.E("failed to parse signature: %w", err)
	}

	// Verify signature
	valid = schnorrSig.Verify(msg, s.pubKey)
	return valid, nil
}

// Zero wipes the secret key
func (s *FallbackSigner) Zero() {
	if s.privKey != nil {
		privKeyBytes := s.privKey.Serialize()
		for i := range privKeyBytes {
			privKeyBytes[i] = 0
		}
		s.privKey = nil
	}
	if s.xonlyPub != nil {
		for i := range s.xonlyPub {
			s.xonlyPub[i] = 0
		}
	}
}

// ECDH returns a shared secret
func (s *FallbackSigner) ECDH(pub []byte) (secret []byte, err error) {
	return s.ECDHRaw(pub)
}

// ECDHRaw returns the raw shared secret (x-coordinate only)
func (s *FallbackSigner) ECDHRaw(pub []byte) (sharedX []byte, err error) {
	if s.privKey == nil {
		return nil, errorf.E("private key not initialized")
	}

	var pubKeyFull []byte

	if len(pub) == 33 {
		// Already compressed format
		pubKeyFull = pub
	} else if len(pub) == 32 {
		// X-only format: try with 0x02 (even y), then 0x03 (odd y)
		pubKeyFull = make([]byte, 33)
		pubKeyFull[0] = 0x02 // compressed even y
		copy(pubKeyFull[1:], pub)
	} else {
		return nil, errorf.E("public key must be 32 bytes (x-only) or 33 bytes (compressed), got %d bytes", len(pub))
	}

	// Parse the public key
	var parsedPub *secp256k1.PublicKey
	if parsedPub, err = secp256k1.ParsePubKey(pubKeyFull); err != nil {
		// If 32-byte x-only and even y failed, try odd y
		if len(pub) == 32 {
			pubKeyFull[0] = 0x03
			if parsedPub, err = secp256k1.ParsePubKey(pubKeyFull); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Compute ECDH
	sharedX = secp256k1.GenerateSharedSecret(s.privKey, parsedPub)
	return sharedX, nil
}
