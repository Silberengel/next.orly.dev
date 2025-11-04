// Package p8k provides a signer.I implementation using p8k.mleku.dev
package p8k

import (
	"crypto/rand"
	
	"lol.mleku.dev/errorf"
	secp "next.orly.dev/pkg/crypto/p8k"
	"next.orly.dev/pkg/interfaces/signer"
)

// Signer implements the signer.I interface using p8k.mleku.dev
type Signer struct {
	ctx      *secp.Context
	secKey   []byte
	pubKey   []byte
	keypair  secp.Keypair
}

// Ensure Signer implements signer.I
var _ signer.I = (*Signer)(nil)

// New creates a new P8K signer
func New() (s *Signer, err error) {
	var ctx *secp.Context
	if ctx, err = secp.NewContext(secp.ContextSign | secp.ContextVerify); err != nil {
		return
	}
	s = &Signer{ctx: ctx}
	return
}

// MustNew creates a new P8K signer and panics on error
func MustNew() (s *Signer) {
	var err error
	if s, err = New(); err != nil {
		panic(err)
	}
	return
}

// Generate creates a fresh new key pair from system entropy, and ensures it is even (so
// ECDH works).
func (s *Signer) Generate() (err error) {
	s.secKey = make([]byte, 32)
	if _, err = rand.Read(s.secKey); err != nil {
		return
	}
	
	// Create keypair
	if s.keypair, err = s.ctx.CreateKeypair(s.secKey); err != nil {
		return
	}
	
	// Extract x-only public key
	var xonly secp.XOnlyPublicKey
	var parity int32
	if xonly, parity, err = s.ctx.KeypairXOnlyPub(s.keypair); err != nil {
		return
	}
	_ = parity
	// XOnlyPublicKey is [64]byte, but we only need the first 32 bytes (the x coordinate)
	s.pubKey = xonly[:32]
	return
}

// InitSec initialises the secret (signing) key from the raw bytes, and also
// derives the public key because it can.
func (s *Signer) InitSec(sec []byte) (err error) {
	if len(sec) != 32 {
		return errorf.E("secret key must be 32 bytes")
	}
	
	s.secKey = make([]byte, 32)
	copy(s.secKey, sec)
	
	// Create keypair
	if s.keypair, err = s.ctx.CreateKeypair(s.secKey); err != nil {
		return
	}
	
	// Extract x-only public key
	var xonly secp.XOnlyPublicKey
	var parity int32
	if xonly, parity, err = s.ctx.KeypairXOnlyPub(s.keypair); err != nil {
		return
	}
	_ = parity
	// XOnlyPublicKey is [64]byte, but we only need the first 32 bytes (the x coordinate)
	s.pubKey = xonly[:32]
	return
}

// InitPub initializes the public (verification) key from raw bytes, this is
// expected to be an x-only 32 byte pubkey.
func (s *Signer) InitPub(pub []byte) (err error) {
	if len(pub) != 32 {
		return errorf.E("public key must be 32 bytes")
	}
	
	s.pubKey = make([]byte, 32)
	copy(s.pubKey, pub)
	return
}

// Sec returns the secret key bytes.
func (s *Signer) Sec() []byte {
	return s.secKey
}

// Pub returns the public key bytes (x-only schnorr pubkey).
func (s *Signer) Pub() []byte {
	return s.pubKey
}

// Sign creates a signature using the stored secret key.
func (s *Signer) Sign(msg []byte) (sig []byte, err error) {
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
func (s *Signer) ECDHRaw(pub []byte) (sharedX []byte, err error) {
	if s.secKey == nil {
		return nil, errorf.E("secret key not initialized")
	}
	
	if len(pub) != 32 {
		return nil, errorf.E("public key must be 32 bytes")
	}
	
	// Convert x-only pubkey to full pubkey
	// For ECDH, we need the full public key, not just x-only
	// Try with 0x02 (even y), then try 0x03 (odd y) if that fails
	pubKeyFull := make([]byte, 33)
	pubKeyFull[0] = 0x02 // compressed even y
	copy(pubKeyFull[1:], pub)
	
	// Parse the public key
	var pubKeyInternal []byte
	if pubKeyInternal, err = s.ctx.ParsePublicKey(pubKeyFull); err != nil {
		// Try odd y coordinate
		pubKeyFull[0] = 0x03
		if pubKeyInternal, err = s.ctx.ParsePublicKey(pubKeyFull); err != nil {
			return nil, err
		}
	}
	
	// Compute ECDH - this returns the 32-byte x-coordinate of the shared point
	if sharedX, err = s.ctx.ECDH(pubKeyInternal, s.secKey); err != nil {
		return
	}
	
	return
}

