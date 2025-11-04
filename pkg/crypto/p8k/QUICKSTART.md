# Quick Reference Guide for p8k.mleku.dev

## Installation

```bash
go get p8k.mleku.dev
```

## Library Requirements

Install libsecp256k1 on your system:

**Ubuntu/Debian:**
```bash
sudo apt-get install libsecp256k1-dev
```

**macOS:**
```bash
brew install libsecp256k1
```

**From source:**
```bash
make install-secp256k1
```

## Quick Start

### Basic ECDSA

```go
import "next.orly.dev/pkg/crypto/p8k"

// Generate key pair
privKey, _ := secp.GeneratePrivateKey()
pubKey, _ := secp.PublicKeyFromPrivate(privKey, true) // compressed

// Sign message
msgHash := sha256.Sum256([]byte("Hello"))
sig, _ := secp.SignMessage(msgHash[:], privKey)

// Verify signature
valid, _ := secp.VerifyMessage(msgHash[:], sig, pubKey)
```

### Schnorr Signatures (BIP-340)

```go
// Generate x-only public key
xonly, _, _ := secp.XOnlyPubKeyFromPrivate(privKey)

// Sign with Schnorr
auxRand, _ := secp.GeneratePrivateKey() // 32 random bytes
sig, _ := secp.SchnorrSign(msgHash[:], privKey, auxRand)

// Verify
valid, _ := secp.SchnorrVerifyWithPubKey(msgHash[:], sig, xonly)
```

### ECDH Key Exchange

```go
// Compute shared secret
sharedSecret, _ := secp.ComputeECDH(theirPubKey, myPrivKey)
```

### Public Key Recovery

```go
// Sign with recovery
sig, recID, _ := secp.SignRecoverableCompact(msgHash[:], privKey)

// Recover public key
recoveredPubKey, _ := secp.RecoverPubKey(msgHash[:], sig, recID, true)
```

## Context-Based API (Advanced)

For more control, use the context-based API:

```go
ctx, _ := secp.NewContext(secp.ContextSign | secp.ContextVerify)
defer ctx.Destroy()

// Use ctx methods directly
pubKey, _ := ctx.CreatePublicKey(privKey)
sig, _ := ctx.Sign(msgHash[:], privKey)
valid, _ := ctx.Verify(msgHash[:], sig, pubKey)
```

## Constants

```go
secp.PrivateKeySize              // 32 bytes
secp.PublicKeySize               // 64 bytes (internal format)
secp.CompressedPublicKeySize     // 33 bytes (serialized)
secp.UncompressedPublicKeySize   // 65 bytes (serialized)
secp.SignatureSize               // 64 bytes (internal format)
secp.CompactSignatureSize        // 64 bytes (serialized)
secp.SchnorrSignatureSize        // 64 bytes
secp.SharedSecretSize            // 32 bytes
secp.RecoverableSignatureSize    // 65 bytes
```

## Context Flags

```go
secp.ContextNone       // No flags
secp.ContextVerify     // For verification operations
secp.ContextSign       // For signing operations
secp.ContextDeclassify // For declassification
```

## Testing

```bash
# Run tests
make test

# Run benchmarks
make bench

# Run examples
make run-examples
```

## Performance Tips

1. **Reuse contexts**: Creating contexts is expensive. Reuse them when possible.
2. **Use utility functions**: For one-off operations, utility functions manage contexts for you.
3. **Batch operations**: If doing many operations, create one context and use it for all.

## Module Availability

Not all modules may be available in your libsecp256k1 build:

- **ECDSA**: Always available
- **Schnorr**: Requires `--enable-module-schnorrsig`
- **ECDH**: Requires `--enable-module-ecdh`
- **Recovery**: Requires `--enable-module-recovery`

Functions will return an error if the required module is not available.

## Error Handling

All functions return errors. Always check them:

```go
sig, err := secp.SignMessage(msgHash[:], privKey)
if err != nil {
    log.Fatal(err)
}
```

## Thread Safety

Context objects are NOT thread-safe. Each goroutine should have its own context.

```go
// BAD: Sharing context across goroutines
ctx, _ := secp.NewContext(secp.ContextSign)
go func() { ctx.Sign(...) }()
go func() { ctx.Sign(...) }() // Race condition!

// GOOD: Each goroutine gets its own context
go func() {
    ctx, _ := secp.NewContext(secp.ContextSign)
    defer ctx.Destroy()
    ctx.Sign(...)
}()
```

## License

MIT License

## Links

- Repository: https://github.com/bitcoin-core/secp256k1 (upstream)
- BIP-340 (Schnorr): https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki
- BIP-327 (MuSig2): https://github.com/bitcoin/bips/blob/master/bip-0327.mediawiki

