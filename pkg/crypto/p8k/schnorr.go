package secp

import (
	"fmt"
)

// Keypair represents a secp256k1 keypair for Schnorr signatures
type Keypair [96]byte

// XOnlyPublicKey represents a 64-byte x-only public key (internal format)
type XOnlyPublicKey [64]byte

// CreateKeypair creates a keypair from a 32-byte secret key
func (c *Context) CreateKeypair(seckey []byte) (keypair Keypair, err error) {
	if keypairCreate == nil {
		err = fmt.Errorf("schnorrsig module not available")
		return
	}

	if len(seckey) != PrivateKeySize {
		err = fmt.Errorf("private key must be %d bytes", PrivateKeySize)
		return
	}

	ret := keypairCreate(c.ctx, &keypair[0], &seckey[0])
	if ret != 1 {
		err = fmt.Errorf("failed to create keypair")
		return
	}

	return
}

// KeypairXOnlyPub extracts the x-only public key from a keypair
func (c *Context) KeypairXOnlyPub(keypair Keypair) (xonly XOnlyPublicKey, pkParity int32, err error) {
	if keypairXonlyPub == nil {
		err = fmt.Errorf("schnorrsig module not available")
		return
	}

	ret := keypairXonlyPub(c.ctx, &xonly[0], &pkParity, &keypair[0])
	if ret != 1 {
		err = fmt.Errorf("failed to extract xonly pubkey")
		return
	}

	return
}

// KeypairPub extracts the full public key (64-byte internal format) from a keypair
func (c *Context) KeypairPub(keypair Keypair) (pubkey []byte, err error) {
	if keypairPub == nil {
		err = fmt.Errorf("keypair_pub function not available")
		return
	}

	pubkey = make([]byte, PublicKeySize)
	ret := keypairPub(c.ctx, &pubkey[0], &keypair[0])
	if ret != 1 {
		err = fmt.Errorf("failed to extract public key from keypair")
		return
	}

	return
}

// SchnorrSign creates a Schnorr signature (BIP-340)
func (c *Context) SchnorrSign(msg32 []byte, keypair Keypair, auxRand32 []byte) (sig []byte, err error) {
	if schnorrsigSign32 == nil {
		err = fmt.Errorf("schnorrsig module not available")
		return
	}

	if len(msg32) != 32 {
		err = fmt.Errorf("message must be 32 bytes")
		return
	}

	var auxPtr *byte
	if len(auxRand32) > 0 {
		if len(auxRand32) != 32 {
			err = fmt.Errorf("aux_rand must be 32 bytes")
			return
		}
		auxPtr = &auxRand32[0]
	}

	sig = make([]byte, SchnorrSignatureSize)
	ret := schnorrsigSign32(c.ctx, &sig[0], &msg32[0], &keypair[0], auxPtr)
	if ret != 1 {
		err = fmt.Errorf("failed to create Schnorr signature")
		return
	}

	return
}

// SchnorrVerify verifies a Schnorr signature (BIP-340)
func (c *Context) SchnorrVerify(sig64 []byte, msg []byte, xonlyPubkey []byte) (valid bool, err error) {
	if schnorrsigVerify == nil {
		err = fmt.Errorf("schnorrsig module not available")
		return
	}

	if len(sig64) != SchnorrSignatureSize {
		err = fmt.Errorf("signature must be %d bytes", SchnorrSignatureSize)
		return
	}

	// xonlyPubkey can be either 32 bytes (serialized) or 64 bytes (internal)
	var xonly [64]byte
	if len(xonlyPubkey) == 32 {
		// Parse the 32-byte serialized format
		ret := xonlyPubkeyParse(c.ctx, &xonly[0], &xonlyPubkey[0])
		if ret != 1 {
			err = fmt.Errorf("failed to parse xonly pubkey")
			return
		}
	} else if len(xonlyPubkey) == 64 {
		// Already in internal format
		copy(xonly[:], xonlyPubkey)
	} else {
		err = fmt.Errorf("xonly public key must be 32 or 64 bytes")
		return
	}

	ret := schnorrsigVerify(c.ctx, &sig64[0], &msg[0], uint64(len(msg)), &xonly[0])
	valid = ret == 1

	return
}

// ParseXOnlyPublicKey parses a 32-byte x-only public key
func (c *Context) ParseXOnlyPublicKey(input32 []byte) (xonly []byte, err error) {
	if xonlyPubkeyParse == nil {
		err = fmt.Errorf("schnorrsig module not available")
		return
	}

	if len(input32) != 32 {
		err = fmt.Errorf("xonly public key must be 32 bytes")
		return
	}

	xonly = make([]byte, 64) // Internal representation is 64 bytes
	ret := xonlyPubkeyParse(c.ctx, &xonly[0], &input32[0])
	if ret != 1 {
		err = fmt.Errorf("failed to parse xonly public key")
		return
	}

	return
}

// SerializeXOnlyPublicKey serializes an x-only public key to 32 bytes
func (c *Context) SerializeXOnlyPublicKey(xonly []byte) (output32 []byte, err error) {
	if xonlyPubkeySerialize == nil {
		err = fmt.Errorf("schnorrsig module not available")
		return
	}

	if len(xonly) != 64 {
		err = fmt.Errorf("xonly public key must be 64 bytes (internal format)")
		return
	}

	output32 = make([]byte, 32)
	ret := xonlyPubkeySerialize(c.ctx, &output32[0], &xonly[0])
	if ret != 1 {
		err = fmt.Errorf("failed to serialize xonly public key")
		return
	}

	return
}

// XOnlyPublicKeyFromPublicKey converts a regular public key to an x-only public key
func (c *Context) XOnlyPublicKeyFromPublicKey(pubkey []byte) (xonly []byte, pkParity int32, err error) {
	if xonlyPubkeyFromPubkey == nil {
		err = fmt.Errorf("schnorrsig module not available")
		return
	}

	if len(pubkey) != PublicKeySize {
		err = fmt.Errorf("public key must be %d bytes", PublicKeySize)
		return
	}

	xonly = make([]byte, 64) // Internal representation
	ret := xonlyPubkeyFromPubkey(c.ctx, &xonly[0], &pkParity, &pubkey[0])
	if ret != 1 {
		err = fmt.Errorf("failed to convert to xonly public key")
		return
	}

	return
}
