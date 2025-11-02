// Package p256k provides a signer interface that uses p256k1.mleku.dev library for
// fast signature creation and verification of BIP-340 nostr X-only signatures and
// public keys, and ECDH.
//
// The package provides type aliases to p256k1.mleku.dev/signer:
//   - cgo: Uses the CGO-optimized version from p256k1.mleku.dev
//   - btcec: Uses the btcec version from p256k1.mleku.dev
//   - default: Uses the pure Go version from p256k1.mleku.dev
package p256k
