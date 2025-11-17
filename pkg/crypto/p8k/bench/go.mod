module bench

go 1.25.3

require (
	github.com/btcsuite/btcd/btcec/v2 v2.3.6
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0
	p256k1.mleku.dev v1.0.2
	p8k.mleku.dev v0.0.0
	p8k.mleku.dev/p8k v0.0.0-00010101000000-000000000000
)

require (
	github.com/btcsuite/btcd/chaincfg/chainhash v1.1.0 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.1.0 // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	golang.org/x/sys v0.37.0 // indirect
)

replace (
	p8k.mleku.dev => ../
	p8k.mleku.dev/p8k => ../p8k
)
