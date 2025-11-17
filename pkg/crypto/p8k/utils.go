package secp

import (
	"crypto/rand"
	"fmt"
)

// GeneratePrivateKey generates a random 32-byte private key
func GeneratePrivateKey() (privKey []byte, err error) {
	privKey = make([]byte, PrivateKeySize)
	if _, err = rand.Read(privKey); err != nil {
		err = fmt.Errorf("failed to generate random key: %w", err)
		return
	}
	return
}

// PublicKeyFromPrivate generates a public key from a private key
// Returns the serialized public key in compressed format
func PublicKeyFromPrivate(privKey []byte, compressed bool) (pubKey []byte, err error) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	internalPubKey, err := ctx.CreatePublicKey(privKey)
	if err != nil {
		return
	}

	pubKey, err = ctx.SerializePublicKey(internalPubKey, compressed)
	return
}

// SignMessage signs a 32-byte message hash with a private key
// Returns the signature in compact format (64 bytes)
func SignMessage(msgHash []byte, privKey []byte) (sig []byte, err error) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	internalSig, err := ctx.Sign(msgHash, privKey)
	if err != nil {
		return
	}

	sig, err = ctx.SerializeSignatureCompact(internalSig)
	return
}

// VerifyMessage verifies a compact signature against a message hash and serialized public key
func VerifyMessage(msgHash []byte, compactSig []byte, serializedPubKey []byte) (valid bool, err error) {
	ctx, err := NewContext(ContextVerify)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	pubKey, err := ctx.ParsePublicKey(serializedPubKey)
	if err != nil {
		return
	}

	sig, err := ctx.ParseSignatureCompact(compactSig)
	if err != nil {
		return
	}

	valid, err = ctx.Verify(msgHash, sig, pubKey)
	return
}

// SignMessageDER signs a message and returns DER-encoded signature
func SignMessageDER(msgHash []byte, privKey []byte) (derSig []byte, err error) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	internalSig, err := ctx.Sign(msgHash, privKey)
	if err != nil {
		return
	}

	derSig, err = ctx.SerializeSignatureDER(internalSig)
	return
}

// VerifyMessageDER verifies a DER-encoded signature
func VerifyMessageDER(msgHash []byte, derSig []byte, serializedPubKey []byte) (valid bool, err error) {
	ctx, err := NewContext(ContextVerify)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	pubKey, err := ctx.ParsePublicKey(serializedPubKey)
	if err != nil {
		return
	}

	sig, err := ctx.ParseSignatureDER(derSig)
	if err != nil {
		return
	}

	valid, err = ctx.Verify(msgHash, sig, pubKey)
	return
}

// SchnorrSign signs a message with Schnorr signature (BIP-340)
// Returns 64-byte Schnorr signature
func SchnorrSign(msgHash []byte, privKey []byte, auxRand []byte) (sig []byte, err error) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	keypair, err := ctx.CreateKeypair(privKey)
	if err != nil {
		return
	}

	sig, err = ctx.SchnorrSign(msgHash, keypair, auxRand)
	return
}

// SchnorrVerifyWithPubKey verifies a Schnorr signature (BIP-340)
// xonlyPubKey should be 32 bytes
func SchnorrVerifyWithPubKey(msgHash []byte, sig []byte, xonlyPubKey []byte) (valid bool, err error) {
	ctx, err := NewContext(ContextVerify)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	valid, err = ctx.SchnorrVerify(sig, msgHash, xonlyPubKey)
	return
}

// XOnlyPubKeyFromPrivate generates an x-only public key from a private key
func XOnlyPubKeyFromPrivate(privKey []byte) (xonly []byte, pkParity int32, err error) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	keypair, err := ctx.CreateKeypair(privKey)
	if err != nil {
		return
	}

	var xonlyInternal XOnlyPublicKey
	xonlyInternal, pkParity, err = ctx.KeypairXOnlyPub(keypair)
	if err != nil {
		return
	}

	xonly = xonlyInternal[:]
	return
}

// ComputeECDH computes an ECDH shared secret
func ComputeECDH(serializedPubKey []byte, privKey []byte) (secret []byte, err error) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	pubKey, err := ctx.ParsePublicKey(serializedPubKey)
	if err != nil {
		return
	}

	secret, err = ctx.ECDH(pubKey, privKey)
	return
}

// SignRecoverableCompact signs a message with a recoverable signature
// Returns compact signature (64 bytes) and recovery ID
func SignRecoverableCompact(msgHash []byte, privKey []byte) (sig []byte, recID int32, err error) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	recSig, err := ctx.SignRecoverable(msgHash, privKey)
	if err != nil {
		return
	}

	sig, recID, err = ctx.SerializeRecoverableSignatureCompact(recSig)
	return
}

// RecoverPubKey recovers a public key from a recoverable signature
// Returns serialized public key in compressed format
func RecoverPubKey(msgHash []byte, compactSig []byte, recID int32, compressed bool) (pubKey []byte, err error) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	recSig, err := ctx.ParseRecoverableSignatureCompact(compactSig, recID)
	if err != nil {
		return
	}

	recoveredPubKey, err := ctx.Recover(recSig, msgHash)
	if err != nil {
		return
	}

	pubKey, err = ctx.SerializePublicKey(recoveredPubKey, compressed)
	return
}

// ValidatePrivateKey checks if a private key is valid
func ValidatePrivateKey(privKey []byte) (valid bool, err error) {
	if len(privKey) != PrivateKeySize {
		err = fmt.Errorf("private key must be %d bytes", PrivateKeySize)
		return
	}

	ctx, err := NewContext(ContextSign)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	_, err = ctx.CreatePublicKey(privKey)
	if err != nil {
		valid = false
		err = nil
		return
	}

	valid = true
	return
}

// IsPublicKeyValid checks if a serialized public key is valid
func IsPublicKeyValid(serializedPubKey []byte) (valid bool, err error) {
	ctx, err := NewContext(ContextVerify)
	if err != nil {
		return
	}
	defer ctx.Destroy()

	_, err = ctx.ParsePublicKey(serializedPubKey)
	if err != nil {
		valid = false
		err = nil
		return
	}

	valid = true
	return
}
