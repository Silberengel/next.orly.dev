//go:build cgo

package p256k

import (
	"lol.mleku.dev/log"
	p256k1signer "p256k1.mleku.dev/signer"
)

func init() {
	log.T.Ln("using p256k1.mleku.dev/signer (CGO)")
}

// Signer is an alias for the P256K1Signer type from p256k1.mleku.dev/signer (cgo version).
type Signer = p256k1signer.P256K1Signer

// Keygen is an alias for the P256K1Gen type from p256k1.mleku.dev/signer (cgo version).
type Keygen = p256k1signer.P256K1Gen

var NewKeygen = p256k1signer.NewP256K1Gen