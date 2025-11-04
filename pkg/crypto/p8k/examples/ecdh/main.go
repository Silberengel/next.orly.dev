package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"log"

	secp "next.orly.dev/pkg/crypto/p8k"
)

func main() {
	ctx, err := secp.NewContext(secp.ContextSign)
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Destroy()

	// Alice's keys
	alicePriv := make([]byte, 32)
	if _, err := rand.Read(alicePriv); err != nil {
		log.Fatal(err)
	}
	alicePub, err := ctx.CreatePublicKey(alicePriv)
	if err != nil {
		log.Fatal(err)
	}

	// Bob's keys
	bobPriv := make([]byte, 32)
	if _, err := rand.Read(bobPriv); err != nil {
		log.Fatal(err)
	}
	bobPub, err := ctx.CreatePublicKey(bobPriv)
	if err != nil {
		log.Fatal(err)
	}

	// Alice computes shared secret with Bob's public key
	aliceShared, err := ctx.ECDH(bobPub, alicePriv)
	if err != nil {
		log.Fatal(err)
	}

	// Bob computes shared secret with Alice's public key
	bobShared, err := ctx.ECDH(alicePub, bobPriv)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Alice's shared secret: %x\n", aliceShared)
	fmt.Printf("Bob's shared secret:   %x\n", bobShared)
	fmt.Printf("Secrets match: %v\n", bytes.Equal(aliceShared, bobShared))
}
