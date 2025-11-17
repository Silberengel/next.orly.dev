package secp

import (
	"fmt"
)

// SignRecoverable creates a recoverable ECDSA signature
func (c *Context) SignRecoverable(msg32 []byte, seckey []byte) (sig []byte, err error) {
	if ecdsaSignRecoverable == nil {
		err = fmt.Errorf("recovery module not available")
		return
	}

	if len(msg32) != 32 {
		err = fmt.Errorf("message must be 32 bytes")
		return
	}

	if len(seckey) != PrivateKeySize {
		err = fmt.Errorf("private key must be %d bytes", PrivateKeySize)
		return
	}

	sig = make([]byte, RecoverableSignatureSize)
	ret := ecdsaSignRecoverable(c.ctx, &sig[0], &msg32[0], &seckey[0], 0, 0)
	if ret != 1 {
		err = fmt.Errorf("failed to create recoverable signature")
		return
	}

	return
}

// SerializeRecoverableSignatureCompact serializes a recoverable signature
func (c *Context) SerializeRecoverableSignatureCompact(sig []byte) (output64 []byte, recid int32, err error) {
	if ecdsaRecoverableSignatureSerializeCompact == nil {
		err = fmt.Errorf("recovery module not available")
		return
	}

	if len(sig) != RecoverableSignatureSize {
		err = fmt.Errorf("recoverable signature must be %d bytes", RecoverableSignatureSize)
		return
	}

	output64 = make([]byte, 64)
	ret := ecdsaRecoverableSignatureSerializeCompact(c.ctx, &output64[0], &recid, &sig[0])
	if ret != 1 {
		err = fmt.Errorf("failed to serialize recoverable signature")
		return
	}

	return
}

// ParseRecoverableSignatureCompact parses a compact recoverable signature
func (c *Context) ParseRecoverableSignatureCompact(input64 []byte, recid int32) (sig []byte, err error) {
	if ecdsaRecoverableSignatureParseCompact == nil {
		err = fmt.Errorf("recovery module not available")
		return
	}

	if len(input64) != 64 {
		err = fmt.Errorf("compact signature must be 64 bytes")
		return
	}

	if recid < 0 || recid > 3 {
		err = fmt.Errorf("recovery id must be 0-3")
		return
	}

	sig = make([]byte, RecoverableSignatureSize)
	ret := ecdsaRecoverableSignatureParseCompact(c.ctx, &sig[0], &input64[0], recid)
	if ret != 1 {
		err = fmt.Errorf("failed to parse recoverable signature")
		return
	}

	return
}

// Recover recovers a public key from a recoverable signature
func (c *Context) Recover(sig []byte, msg32 []byte) (pubkey []byte, err error) {
	if ecdsaRecover == nil {
		err = fmt.Errorf("recovery module not available")
		return
	}

	if len(sig) != RecoverableSignatureSize {
		err = fmt.Errorf("recoverable signature must be %d bytes", RecoverableSignatureSize)
		return
	}

	if len(msg32) != 32 {
		err = fmt.Errorf("message must be 32 bytes")
		return
	}

	pubkey = make([]byte, PublicKeySize)
	ret := ecdsaRecover(c.ctx, &pubkey[0], &sig[0], &msg32[0])
	if ret != 1 {
		err = fmt.Errorf("failed to recover public key")
		return
	}

	return
}
