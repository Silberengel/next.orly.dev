//go:build !cgo

package p256k

import (
	"lol.mleku.dev/log"
	p256k1signer "p256k1.mleku.dev/signer"
)

func init() {
	log.T.Ln("using p256k1.mleku.dev/signer (pure Go/Btcec)")
}

// Signer is an alias for the BtcecSigner type from p256k1.mleku.dev/signer (btcec version).
// This is used when CGO is not available.
type Signer = p256k1signer.BtcecSigner

// Keygen is an alias for the P256K1Gen type from p256k1.mleku.dev/signer (btcec version).
type Keygen = p256k1signer.P256K1Gen

var NewKeygen = p256k1signer.NewP256K1Gen