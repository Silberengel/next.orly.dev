# p8k - Unified Secp256k1 Signer with Automatic Fallback

This package provides a unified interface for secp256k1 cryptographic operations with automatic fallback from C bindings to pure Go.

## Features

- **Granular Fallback**: Uses libsecp256k1 via purego when available, falls back to pure Go p256k1 on a per-function basis
- **Module Detection**: Automatically detects which libsecp256k1 modules (Schnorr, ECDH, Recovery) are available
- **No Manual Configuration**: Fallback happens automatically at initialization
- **Thread-Safe**: All operations are protected with RWMutex
- **Complete API**: Schnorr (BIP-340), ECDSA, ECDH, and public key operations
- **Transparent Performance**: Get C-level performance when possible, pure Go reliability always

## How It Works

The signer detects which optional modules are compiled into libsecp256k1:

- **Core functions** (ECDSA, pubkey): Always use C if library loads
- **Schnorr functions**: Use C if Schnorr module available, otherwise pure Go
- **ECDH functions**: Use C if ECDH module available, otherwise pure Go
- **Recovery functions**: Use C if Recovery module available, otherwise pure Go

This means you can have libsecp256k1 without Schnorr support, and the signer will use C for ECDSA while transparently falling back to pure Go for Schnorr operations.

## Usage

```go
import "next.orly.dev/pkg/crypto/p8k/p8k"

func main() {
    // Create signer (automatically detects and falls back)
    signer, err := p8k.NewSigner()
    if err != nil {
        log.Fatal(err)
    }
    defer signer.Close()
    
    // Check which modules are available
    status := signer.GetModuleStatus()
    log.Printf("Library: %v, Schnorr: %v, ECDH: %v", 
        status["library"], status["schnorr"], status["ecdh"])
    
    // Use normally - interface is the same regardless
    privkey := make([]byte, 32)
    rand.Read(privkey)
    
    pubkey, _ := signer.GeneratePublicKey(privkey)
    sig, _ := signer.SchnorrSign(msg, privkey, auxrand)
    valid, _ := signer.SchnorrVerify(sig, msg, xonly)
}
```

## API

- `NewSigner()` - Create new signer with auto-fallback
- `Close()` - Clean up resources
- `IsUsingFallback()` - Check if using pure Go for everything
- `GetModuleStatus()` - Check which modules are available
- `GeneratePublicKey(privkey)` - Derive public key
- `SchnorrSign(msg, privkey, auxrand)` - BIP-340 Schnorr signature
- `SchnorrVerify(sig, msg, xonly)` - Verify Schnorr signature
- `Sign(msg, privkey)` - ECDSA signature
- `Verify(msg, sig, pubkey)` - Verify ECDSA signature
- `ECDHSharedSecret(pubkey, privkey)` - Compute shared secret
- `GetXOnlyPubkey(privkey)` - Extract x-only pubkey

## Performance

When libsecp256k1 is available with all modules, you get full C-level performance. When specific modules are missing, only those functions fall back to pure Go while the rest stay at C performance.

## Module Status Examples

**Full C bindings (all modules available):**
```
Library: true, Schnorr: true, ECDH: true, Recovery: true
→ All operations use C bindings (maximum performance)
```

**Partial C bindings (Schnorr module missing):**
```
Library: true, Schnorr: false, ECDH: true, Recovery: true
→ ECDSA and ECDH use C, Schnorr uses pure Go
```

**Full pure Go fallback (library not available):**
```
Library: false, Schnorr: false, ECDH: false, Recovery: false
→ All operations use pure Go (guaranteed compatibility)
```

## License

MIT License


