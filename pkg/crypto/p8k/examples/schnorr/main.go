package main

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"

	secp "next.orly.dev/pkg/crypto/p8k"
)

func main() {
	ctx, err := secp.NewContext(secp.ContextSign | secp.ContextVerify)
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Destroy()

	// Generate private key
	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		log.Fatal(err)
	}

	// Create keypair for Schnorr
	keypair, err := ctx.CreateKeypair(privKey)
	if err != nil {
		log.Fatal(err)
	}

	// Extract x-only public key
	xonly, pkParity, err := ctx.KeypairXOnlyPub(keypair)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("X-only public key: %x\n", xonly)
	fmt.Printf("Public key parity: %d\n", pkParity)

	// Sign with Schnorr
	message := []byte("Hello, Schnorr!")
	msgHash := sha256.Sum256(message)

	auxRand := make([]byte, 32)
	if _, err := rand.Read(auxRand); err != nil {
		log.Fatal(err)
	}

	sig, err := ctx.SchnorrSign(msgHash[:], keypair, auxRand)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Schnorr signature: %x\n", sig)

	// Verify Schnorr signature
	valid, err := ctx.SchnorrVerify(sig, msgHash[:], xonly[:])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Schnorr signature valid: %v\n", valid)

	// Test with wrong message
	wrongMsg := []byte("Wrong message!")
	wrongHash := sha256.Sum256(wrongMsg)
	valid2, err := ctx.SchnorrVerify(sig, wrongHash[:], xonly[:])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Schnorr signature valid (wrong msg): %v\n", valid2)
}
