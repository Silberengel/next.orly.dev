package secp

import (
	"fmt"
)

// ECDH computes an EC Diffie-Hellman shared secret
func (c *Context) ECDH(pubkey []byte, seckey []byte) (output []byte, err error) {
	if ecdh == nil {
		err = fmt.Errorf("ecdh module not available")
		return
	}

	if len(pubkey) != PublicKeySize {
		err = fmt.Errorf("public key must be %d bytes", PublicKeySize)
		return
	}

	if len(seckey) != PrivateKeySize {
		err = fmt.Errorf("private key must be %d bytes", PrivateKeySize)
		return
	}

	output = make([]byte, SharedSecretSize)
	ret := ecdh(c.ctx, &output[0], &pubkey[0], &seckey[0], 0, 0)
	if ret != 1 {
		err = fmt.Errorf("failed to compute ECDH")
		return
	}

	return
}
