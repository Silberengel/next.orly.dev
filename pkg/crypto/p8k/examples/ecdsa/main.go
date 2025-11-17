package main

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"

	secp "next.orly.dev/pkg/crypto/p8k"
)

func main() {
	// Create a context for signing and verification
	ctx, err := secp.NewContext(secp.ContextSign | secp.ContextVerify)
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Destroy()

	// Generate a private key (32 random bytes)
	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		log.Fatal(err)
	}

	// Create public key from private key
	pubKey, err := ctx.CreatePublicKey(privKey)
	if err != nil {
		log.Fatal(err)
	}

	// Serialize public key (compressed)
	pubKeyBytes, err := ctx.SerializePublicKey(pubKey, true)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Public key: %x\n", pubKeyBytes)

	// Sign a message
	message := []byte("Hello, libsecp256k1!")
	msgHash := sha256.Sum256(message)

	sig, err := ctx.Sign(msgHash[:], privKey)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Signature: %x\n", sig)

	// Verify the signature
	valid, err := ctx.Verify(msgHash[:], sig, pubKey)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Signature valid: %v\n", valid)

	// Test with serialized/parsed public key
	parsedPubKey, err := ctx.ParsePublicKey(pubKeyBytes)
	if err != nil {
		log.Fatal(err)
	}

	valid2, err := ctx.Verify(msgHash[:], sig, parsedPubKey)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Signature valid (parsed key): %v\n", valid2)

	// Test DER encoding
	derSig, err := ctx.SerializeSignatureDER(sig)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("DER signature: %x\n", derSig)

	// Parse DER signature
	parsedSig, err := ctx.ParseSignatureDER(derSig)
	if err != nil {
		log.Fatal(err)
	}

	valid3, err := ctx.Verify(msgHash[:], parsedSig, pubKey)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Signature valid (DER): %v\n", valid3)
}
