package secp

import (
	"crypto/rand"
	"crypto/sha256"
	"testing"
)

func TestContextCreation(t *testing.T) {
	ctx, err := NewContext(ContextSign | ContextVerify)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	if ctx.ctx == 0 {
		t.Fatal("Context handle is null")
	}
}

func TestPublicKeyGeneration(t *testing.T) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	pubKey, err := ctx.CreatePublicKey(privKey)
	if err != nil {
		t.Fatalf("Failed to create public key: %v", err)
	}

	if len(pubKey) != PublicKeySize {
		t.Fatalf("Public key size incorrect: got %d, want %d", len(pubKey), PublicKeySize)
	}
}

func TestPublicKeySerialization(t *testing.T) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	pubKey, err := ctx.CreatePublicKey(privKey)
	if err != nil {
		t.Fatalf("Failed to create public key: %v", err)
	}

	// Test compressed
	compressed, err := ctx.SerializePublicKey(pubKey, true)
	if err != nil {
		t.Fatalf("Failed to serialize compressed: %v", err)
	}
	if len(compressed) != CompressedPublicKeySize {
		t.Fatalf("Compressed size incorrect: got %d, want %d", len(compressed), CompressedPublicKeySize)
	}

	// Test uncompressed
	uncompressed, err := ctx.SerializePublicKey(pubKey, false)
	if err != nil {
		t.Fatalf("Failed to serialize uncompressed: %v", err)
	}
	if len(uncompressed) != UncompressedPublicKeySize {
		t.Fatalf("Uncompressed size incorrect: got %d, want %d", len(uncompressed), UncompressedPublicKeySize)
	}

	// Parse back compressed
	parsed, err := ctx.ParsePublicKey(compressed)
	if err != nil {
		t.Fatalf("Failed to parse compressed: %v", err)
	}
	if len(parsed) != PublicKeySize {
		t.Fatalf("Parsed size incorrect: got %d, want %d", len(parsed), PublicKeySize)
	}
}

func TestECDSASignAndVerify(t *testing.T) {
	ctx, err := NewContext(ContextSign | ContextVerify)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	pubKey, err := ctx.CreatePublicKey(privKey)
	if err != nil {
		t.Fatalf("Failed to create public key: %v", err)
	}

	message := []byte("Test message")
	msgHash := sha256.Sum256(message)

	sig, err := ctx.Sign(msgHash[:], privKey)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	valid, err := ctx.Verify(msgHash[:], sig, pubKey)
	if err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}

	if !valid {
		t.Fatal("Signature should be valid")
	}

	// Test with wrong message
	wrongMsg := []byte("Wrong message")
	wrongHash := sha256.Sum256(wrongMsg)
	valid2, err := ctx.Verify(wrongHash[:], sig, pubKey)
	if err != nil {
		t.Fatalf("Failed to verify wrong message: %v", err)
	}

	if valid2 {
		t.Fatal("Signature should be invalid for wrong message")
	}
}

func TestDERSignatureSerialization(t *testing.T) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	message := []byte("Test message")
	msgHash := sha256.Sum256(message)

	sig, err := ctx.Sign(msgHash[:], privKey)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	derSig, err := ctx.SerializeSignatureDER(sig)
	if err != nil {
		t.Fatalf("Failed to serialize DER: %v", err)
	}

	parsed, err := ctx.ParseSignatureDER(derSig)
	if err != nil {
		t.Fatalf("Failed to parse DER: %v", err)
	}

	if len(parsed) != SignatureSize {
		t.Fatalf("Parsed signature size incorrect: got %d, want %d", len(parsed), SignatureSize)
	}
}

func TestCompactSignatureSerialization(t *testing.T) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	message := []byte("Test message")
	msgHash := sha256.Sum256(message)

	sig, err := ctx.Sign(msgHash[:], privKey)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	compact, err := ctx.SerializeSignatureCompact(sig)
	if err != nil {
		t.Fatalf("Failed to serialize compact: %v", err)
	}

	if len(compact) != CompactSignatureSize {
		t.Fatalf("Compact size incorrect: got %d, want %d", len(compact), CompactSignatureSize)
	}

	parsed, err := ctx.ParseSignatureCompact(compact)
	if err != nil {
		t.Fatalf("Failed to parse compact: %v", err)
	}

	if len(parsed) != SignatureSize {
		t.Fatalf("Parsed signature size incorrect: got %d, want %d", len(parsed), SignatureSize)
	}
}

func TestSignatureNormalization(t *testing.T) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	message := []byte("Test message")
	msgHash := sha256.Sum256(message)

	sig, err := ctx.Sign(msgHash[:], privKey)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	normalized, wasNormalized, err := ctx.NormalizeSignature(sig)
	if err != nil {
		t.Fatalf("Failed to normalize: %v", err)
	}

	if len(normalized) != SignatureSize {
		t.Fatalf("Normalized signature size incorrect: got %d, want %d", len(normalized), SignatureSize)
	}

	_ = wasNormalized // May or may not be normalized
}

func TestSchnorrSignAndVerify(t *testing.T) {
	if schnorrsigSign32 == nil {
		t.Skip("Schnorr module not available")
	}

	ctx, err := NewContext(ContextSign | ContextVerify)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	keypair, err := ctx.CreateKeypair(privKey)
	if err != nil {
		t.Fatalf("Failed to create keypair: %v", err)
	}

	xonly, _, err := ctx.KeypairXOnlyPub(keypair)
	if err != nil {
		t.Fatalf("Failed to extract xonly pubkey: %v", err)
	}

	message := []byte("Test message")
	msgHash := sha256.Sum256(message)

	auxRand := make([]byte, 32)
	if _, err := rand.Read(auxRand); err != nil {
		t.Fatalf("Failed to generate aux_rand: %v", err)
	}

	sig, err := ctx.SchnorrSign(msgHash[:], keypair, auxRand)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	if len(sig) != SchnorrSignatureSize {
		t.Fatalf("Signature size incorrect: got %d, want %d", len(sig), SchnorrSignatureSize)
	}

	valid, err := ctx.SchnorrVerify(sig, msgHash[:], xonly[:])
	if err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}

	if !valid {
		t.Fatal("Schnorr signature should be valid")
	}

	// Test with wrong message
	wrongMsg := []byte("Wrong message")
	wrongHash := sha256.Sum256(wrongMsg)
	valid2, err := ctx.SchnorrVerify(sig, wrongHash[:], xonly[:])
	if err != nil {
		t.Fatalf("Failed to verify wrong message: %v", err)
	}

	if valid2 {
		t.Fatal("Schnorr signature should be invalid for wrong message")
	}
}

func TestECDH(t *testing.T) {
	if ecdh == nil {
		t.Skip("ECDH module not available")
	}

	ctx, err := NewContext(ContextSign)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	// Alice's keys
	alicePriv := make([]byte, 32)
	if _, err := rand.Read(alicePriv); err != nil {
		t.Fatalf("Failed to generate Alice's key: %v", err)
	}
	alicePub, err := ctx.CreatePublicKey(alicePriv)
	if err != nil {
		t.Fatalf("Failed to create Alice's public key: %v", err)
	}

	// Bob's keys
	bobPriv := make([]byte, 32)
	if _, err := rand.Read(bobPriv); err != nil {
		t.Fatalf("Failed to generate Bob's key: %v", err)
	}
	bobPub, err := ctx.CreatePublicKey(bobPriv)
	if err != nil {
		t.Fatalf("Failed to create Bob's public key: %v", err)
	}

	// Compute shared secrets
	aliceShared, err := ctx.ECDH(bobPub, alicePriv)
	if err != nil {
		t.Fatalf("Failed to compute Alice's shared secret: %v", err)
	}

	bobShared, err := ctx.ECDH(alicePub, bobPriv)
	if err != nil {
		t.Fatalf("Failed to compute Bob's shared secret: %v", err)
	}

	if len(aliceShared) != SharedSecretSize {
		t.Fatalf("Shared secret size incorrect: got %d, want %d", len(aliceShared), SharedSecretSize)
	}

	// Secrets should match
	if string(aliceShared) != string(bobShared) {
		t.Fatal("Shared secrets should match")
	}
}

func TestRecovery(t *testing.T) {
	if ecdsaSignRecoverable == nil {
		t.Skip("Recovery module not available")
	}

	ctx, err := NewContext(ContextSign | ContextVerify)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	originalPubKey, err := ctx.CreatePublicKey(privKey)
	if err != nil {
		t.Fatalf("Failed to create public key: %v", err)
	}

	message := []byte("Test message")
	msgHash := sha256.Sum256(message)

	recSig, err := ctx.SignRecoverable(msgHash[:], privKey)
	if err != nil {
		t.Fatalf("Failed to sign recoverable: %v", err)
	}

	sigBytes, recID, err := ctx.SerializeRecoverableSignatureCompact(recSig)
	if err != nil {
		t.Fatalf("Failed to serialize recoverable: %v", err)
	}

	if len(sigBytes) != 64 {
		t.Fatalf("Signature size incorrect: got %d, want 64", len(sigBytes))
	}

	if recID < 0 || recID > 3 {
		t.Fatalf("Recovery ID out of range: %d", recID)
	}

	parsedSig, err := ctx.ParseRecoverableSignatureCompact(sigBytes, recID)
	if err != nil {
		t.Fatalf("Failed to parse recoverable: %v", err)
	}

	recoveredPubKey, err := ctx.Recover(parsedSig, msgHash[:])
	if err != nil {
		t.Fatalf("Failed to recover public key: %v", err)
	}

	// Serialize both for comparison
	origSer, err := ctx.SerializePublicKey(originalPubKey, true)
	if err != nil {
		t.Fatalf("Failed to serialize original: %v", err)
	}

	recSer, err := ctx.SerializePublicKey(recoveredPubKey, true)
	if err != nil {
		t.Fatalf("Failed to serialize recovered: %v", err)
	}

	if string(origSer) != string(recSer) {
		t.Fatal("Recovered public key should match original")
	}
}

func BenchmarkSign(b *testing.B) {
	ctx, err := NewContext(ContextSign)
	if err != nil {
		b.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	privKey := make([]byte, 32)
	rand.Read(privKey)

	message := []byte("Benchmark message")
	msgHash := sha256.Sum256(message)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ctx.Sign(msgHash[:], privKey)
		if err != nil {
			b.Fatalf("Failed to sign: %v", err)
		}
	}
}

func BenchmarkVerify(b *testing.B) {
	ctx, err := NewContext(ContextSign | ContextVerify)
	if err != nil {
		b.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Destroy()

	privKey := make([]byte, 32)
	rand.Read(privKey)

	pubKey, err := ctx.CreatePublicKey(privKey)
	if err != nil {
		b.Fatalf("Failed to create public key: %v", err)
	}

	message := []byte("Benchmark message")
	msgHash := sha256.Sum256(message)

	sig, err := ctx.Sign(msgHash[:], privKey)
	if err != nil {
		b.Fatalf("Failed to sign: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ctx.Verify(msgHash[:], sig, pubKey)
		if err != nil {
			b.Fatalf("Failed to verify: %v", err)
		}
	}
}
