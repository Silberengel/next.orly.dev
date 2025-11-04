package bench

import (
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"

	p256k1 "p256k1.mleku.dev"

	secp "next.orly.dev/pkg/crypto/p8k"
	p8k "next.orly.dev/pkg/interfaces/signer/p8k"
)

// Shared test data
var (
	benchPrivKey [32]byte
	benchMsg     []byte
	benchMsgHash [32]byte
)

func init() {
	// Generate deterministic test data
	rand.Read(benchPrivKey[:])
	benchMsg = make([]byte, 32)
	rand.Read(benchMsg)
	benchMsgHash = sha256.Sum256(benchMsg)
}

// =============================================================================
// BTCEC Benchmarks
// =============================================================================

func BenchmarkBTCEC_PubkeyDerivation(b *testing.B) {
	privKey, _ := btcec.PrivKeyFromBytes(benchPrivKey[:])
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = privKey.PubKey()
	}
}

func BenchmarkBTCEC_SchnorrSign(b *testing.B) {
	privKey, _ := btcec.PrivKeyFromBytes(benchPrivKey[:])

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := schnorr.Sign(privKey, benchMsgHash[:])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBTCEC_SchnorrVerify(b *testing.B) {
	privKey, _ := btcec.PrivKeyFromBytes(benchPrivKey[:])
	pubKey := privKey.PubKey()
	sig, _ := schnorr.Sign(privKey, benchMsgHash[:])

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid := sig.Verify(benchMsgHash[:], pubKey)
		if !valid {
			b.Fatal("signature verification failed")
		}
	}
}

func BenchmarkBTCEC_ECDH(b *testing.B) {
	privKey1, _ := btcec.PrivKeyFromBytes(benchPrivKey[:])

	var privKey2Bytes [32]byte
	rand.Read(privKey2Bytes[:])
	privKey2, _ := btcec.PrivKeyFromBytes(privKey2Bytes[:])
	pubKey2 := privKey2.PubKey()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = secp256k1.GenerateSharedSecret(privKey1, pubKey2)
	}
}

// =============================================================================
// P256K1 (Pure Go) Benchmarks
// =============================================================================

func BenchmarkP256K1_PubkeyDerivation(b *testing.B) {
	ctx := p256k1.ContextCreate(p256k1.ContextSign)
	defer p256k1.ContextDestroy(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var pubkey p256k1.PublicKey
		err := p256k1.ECPubkeyCreate(&pubkey, benchPrivKey[:])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkP256K1_SchnorrSign(b *testing.B) {
	keypair, err := p256k1.KeyPairCreate(benchPrivKey[:])
	if err != nil {
		b.Fatal(err)
	}

	auxRand := make([]byte, 32)
	rand.Read(auxRand)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var sig [64]byte
		err := p256k1.SchnorrSign(sig[:], benchMsgHash[:], keypair, auxRand)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkP256K1_SchnorrVerify(b *testing.B) {
	keypair, err := p256k1.KeyPairCreate(benchPrivKey[:])
	if err != nil {
		b.Fatal(err)
	}

	xonlyPubkey, err := keypair.XOnlyPubkey()
	if err != nil {
		b.Fatal(err)
	}

	auxRand := make([]byte, 32)
	rand.Read(auxRand)

	var sig [64]byte
	err = p256k1.SchnorrSign(sig[:], benchMsgHash[:], keypair, auxRand)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !p256k1.SchnorrVerify(sig[:], benchMsgHash[:], xonlyPubkey) {
			b.Fatal("verification failed")
		}
	}
}

func BenchmarkP256K1_ECDH(b *testing.B) {
	var privKey2Bytes [32]byte
	rand.Read(privKey2Bytes[:])

	var pubkey2 p256k1.PublicKey
	err := p256k1.ECPubkeyCreate(&pubkey2, privKey2Bytes[:])
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output [32]byte
		err := p256k1.ECDHXOnly(output[:], &pubkey2, benchPrivKey[:])
		if err != nil {
			b.Fatal(err)
		}
	}
}

// =============================================================================
// P8K (Purego) Benchmarks
// =============================================================================

func BenchmarkP8K_PubkeyDerivation(b *testing.B) {
	ctx, err := secp.NewContext(secp.ContextSign)
	if err != nil {
		b.Skip("libsecp256k1 not available:", err)
	}
	defer ctx.Destroy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ctx.CreatePublicKey(benchPrivKey[:])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkP8K_SchnorrSign(b *testing.B) {
	ctx, err := secp.NewContext(secp.ContextSign)
	if err != nil {
		b.Skip("libsecp256k1 not available:", err)
	}
	defer ctx.Destroy()

	keypair, err := ctx.CreateKeypair(benchPrivKey[:])
	if err != nil {
		b.Skip("schnorr module not available:", err)
	}

	auxRand := make([]byte, 32)
	rand.Read(auxRand)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ctx.SchnorrSign(benchMsgHash[:], keypair, auxRand)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkP8K_SchnorrVerify(b *testing.B) {
	ctx, err := secp.NewContext(secp.ContextSign | secp.ContextVerify)
	if err != nil {
		b.Skip("libsecp256k1 not available:", err)
	}
	defer ctx.Destroy()

	keypair, err := ctx.CreateKeypair(benchPrivKey[:])
	if err != nil {
		b.Skip("schnorr module not available:", err)
	}

	xonly, _, err := ctx.KeypairXOnlyPub(keypair)
	if err != nil {
		b.Fatal(err)
	}

	auxRand := make([]byte, 32)
	rand.Read(auxRand)

	sig, err := ctx.SchnorrSign(benchMsgHash[:], keypair, auxRand)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid, err := ctx.SchnorrVerify(sig, benchMsgHash[:], xonly[:])
		if err != nil {
			b.Fatal(err)
		}
		if !valid {
			b.Fatal("verification failed")
		}
	}
}

func BenchmarkP8K_ECDH(b *testing.B) {
	ctx, err := secp.NewContext(secp.ContextSign)
	if err != nil {
		b.Skip("libsecp256k1 not available:", err)
	}
	defer ctx.Destroy()

	var privKey2Bytes [32]byte
	rand.Read(privKey2Bytes[:])

	pubkey2, err := ctx.CreatePublicKey(privKey2Bytes[:])
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ctx.ECDH(pubkey2, benchPrivKey[:])
		if err != nil {
			b.Fatal(err)
		}
	}
}

// =============================================================================
// P8K Signer Interface Benchmarks (with automatic fallback)
// =============================================================================

func BenchmarkSigner_Generate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig, err := p8k.New()
		if err != nil {
			b.Fatal(err)
		}
		if err := sig.Generate(); err != nil {
			b.Fatal(err)
		}
		sig.Zero()
	}
}

func BenchmarkSigner_SchnorrSign(b *testing.B) {
	sig, err := p8k.New()
	if err != nil {
		b.Fatal(err)
	}
	defer sig.Zero()

	if err := sig.InitSec(benchPrivKey[:]); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sig.Sign(benchMsgHash[:])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSigner_SchnorrVerify(b *testing.B) {
	sig, err := p8k.New()
	if err != nil {
		b.Fatal(err)
	}
	defer sig.Zero()

	if err := sig.InitSec(benchPrivKey[:]); err != nil {
		b.Fatal(err)
	}

	signature, err := sig.Sign(benchMsgHash[:])
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid, err := sig.Verify(benchMsgHash[:], signature)
		if err != nil {
			b.Fatal(err)
		}
		if !valid {
			b.Fatal("verification failed")
		}
	}
}

func BenchmarkSigner_ECDH(b *testing.B) {
	sig, err := p8k.New()
	if err != nil {
		b.Fatal(err)
	}
	defer sig.Zero()

	if err := sig.InitSec(benchPrivKey[:]); err != nil {
		b.Fatal(err)
	}

	var privKey2Bytes [32]byte
	rand.Read(privKey2Bytes[:])

	sig2, err := p8k.New()
	if err != nil {
		b.Fatal(err)
	}
	defer sig2.Zero()

	if err := sig2.InitSec(privKey2Bytes[:]); err != nil {
		b.Fatal(err)
	}

	pubkey2 := sig2.Pub()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sig.ECDH(pubkey2)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// =============================================================================
// Comparative Benchmarks (All Implementations)
// =============================================================================

func BenchmarkComparative_SchnorrSign(b *testing.B) {
	b.Run("BTCEC", BenchmarkBTCEC_SchnorrSign)
	b.Run("P256K1", BenchmarkP256K1_SchnorrSign)
	b.Run("P8K", BenchmarkP8K_SchnorrSign)
	b.Run("Signer", BenchmarkSigner_SchnorrSign)
}

func BenchmarkComparative_SchnorrVerify(b *testing.B) {
	b.Run("BTCEC", BenchmarkBTCEC_SchnorrVerify)
	b.Run("P256K1", BenchmarkP256K1_SchnorrVerify)
	b.Run("P8K", BenchmarkP8K_SchnorrVerify)
	b.Run("Signer", BenchmarkSigner_SchnorrVerify)
}

func BenchmarkComparative_ECDH(b *testing.B) {
	b.Run("BTCEC", BenchmarkBTCEC_ECDH)
	b.Run("P256K1", BenchmarkP256K1_ECDH)
	b.Run("P8K", BenchmarkP8K_ECDH)
	b.Run("Signer", BenchmarkSigner_ECDH)
}

// Run all comparative benchmarks
func BenchmarkAll(b *testing.B) {
	b.Run("SchnorrSign", BenchmarkComparative_SchnorrSign)
	b.Run("SchnorrVerify", BenchmarkComparative_SchnorrVerify)
	b.Run("ECDH", BenchmarkComparative_ECDH)
}

// Benchmark to show signer initialization overhead
func BenchmarkSigner_Initialization(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig, err := p8k.New()
		if err != nil {
			b.Fatal(err)
		}
		sig.Zero()
	}
}

// Benchmark to show status check overhead
func BenchmarkSigner_GetModuleStatus(b *testing.B) {
	sig, err := p8k.New()
	if err != nil {
		b.Fatal(err)
	}
	defer sig.Zero()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sig.GetModuleStatus()
	}
}
