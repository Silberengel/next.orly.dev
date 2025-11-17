package main

import (
	"bytes"
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

	// Generate keys
	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		log.Fatal(err)
	}
	originalPubKey, err := ctx.CreatePublicKey(privKey)
	if err != nil {
		log.Fatal(err)
	}

	// Sign with recovery
	message := []byte("Recover me!")
	msgHash := sha256.Sum256(message)

	recSig, err := ctx.SignRecoverable(msgHash[:], privKey)
	if err != nil {
		log.Fatal(err)
	}

	// Serialize to get recovery ID
	sigBytes, recID, err := ctx.SerializeRecoverableSignatureCompact(recSig)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Signature: %x\n", sigBytes)
	fmt.Printf("Recovery ID: %d\n", recID)

	// Parse back
	parsedSig, err := ctx.ParseRecoverableSignatureCompact(sigBytes, recID)
	if err != nil {
		log.Fatal(err)
	}

	// Recover public key
	recoveredPubKey, err := ctx.Recover(parsedSig, msgHash[:])
	if err != nil {
		log.Fatal(err)
	}

	// Serialize both for comparison
	origSer, err := ctx.SerializePublicKey(originalPubKey, true)
	if err != nil {
		log.Fatal(err)
	}
	recSer, err := ctx.SerializePublicKey(recoveredPubKey, true)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Original public key:  %x\n", origSer)
	fmt.Printf("Recovered public key: %x\n", recSer)
	fmt.Printf("Keys match: %v\n", bytes.Equal(origSer, recSer))
}
