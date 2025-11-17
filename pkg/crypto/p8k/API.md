# API Documentation - p8k.mleku.dev

Complete API reference for the libsecp256k1 Go bindings.

## Table of Contents

1. [Context Management](#context-management)
2. [Public Key Operations](#public-key-operations)
3. [ECDSA Signatures](#ecdsa-signatures)
4. [Schnorr Signatures](#schnorr-signatures)
5. [ECDH](#ecdh)
6. [Recovery](#recovery)
7. [Utility Functions](#utility-functions)
8. [Constants](#constants)
9. [Types](#types)

---

## Context Management

### NewContext

Creates a new secp256k1 context.

```go
func NewContext(flags uint32) (c *Context, err error)
```

**Parameters:**
- `flags`: Context flags (ContextSign, ContextVerify, or combined with `|`)

**Returns:**
- `c`: Context pointer
- `err`: Error if context creation failed

**Example:**
```go
ctx, err := secp.NewContext(secp.ContextSign | secp.ContextVerify)
if err != nil {
    log.Fatal(err)
}
defer ctx.Destroy()
```

### Context.Destroy

Destroys the context and frees resources.

```go
func (c *Context) Destroy()
```

**Note:** Contexts are automatically destroyed via finalizer, but explicit cleanup is recommended.

### Context.Randomize

Randomizes the context with entropy for additional security.

```go
func (c *Context) Randomize(seed32 []byte) (err error)
```

**Parameters:**
- `seed32`: 32 bytes of random data

**Returns:**
- `err`: Error if randomization failed

---

## Public Key Operations

### Context.CreatePublicKey

Creates a public key from a private key.

```go
func (c *Context) CreatePublicKey(seckey []byte) (pubkey []byte, err error)
```

**Parameters:**
- `seckey`: 32-byte private key

**Returns:**
- `pubkey`: 64-byte internal public key representation
- `err`: Error if key creation failed

### Context.SerializePublicKey

Serializes a public key to compressed or uncompressed format.

```go
func (c *Context) SerializePublicKey(pubkey []byte, compressed bool) (output []byte, err error)
```

**Parameters:**
- `pubkey`: 64-byte internal public key
- `compressed`: true for compressed (33 bytes), false for uncompressed (65 bytes)

**Returns:**
- `output`: Serialized public key
- `err`: Error if serialization failed

### Context.ParsePublicKey

Parses a serialized public key.

```go
func (c *Context) ParsePublicKey(input []byte) (pubkey []byte, err error)
```

**Parameters:**
- `input`: Serialized public key (33 or 65 bytes)

**Returns:**
- `pubkey`: 64-byte internal public key representation
- `err`: Error if parsing failed

---

## ECDSA Signatures

### Context.Sign

Creates an ECDSA signature.

```go
func (c *Context) Sign(msg32 []byte, seckey []byte) (sig []byte, err error)
```

**Parameters:**
- `msg32`: 32-byte message hash
- `seckey`: 32-byte private key

**Returns:**
- `sig`: 64-byte internal signature representation
- `err`: Error if signing failed

### Context.Verify

Verifies an ECDSA signature.

```go
func (c *Context) Verify(msg32 []byte, sig []byte, pubkey []byte) (valid bool, err error)
```

**Parameters:**
- `msg32`: 32-byte message hash
- `sig`: 64-byte internal signature
- `pubkey`: 64-byte internal public key

**Returns:**
- `valid`: true if signature is valid
- `err`: Error if verification failed

### Context.SerializeSignatureDER

Serializes a signature to DER format.

```go
func (c *Context) SerializeSignatureDER(sig []byte) (output []byte, err error)
```

**Parameters:**
- `sig`: 64-byte internal signature

**Returns:**
- `output`: DER-encoded signature (variable length, max 72 bytes)
- `err`: Error if serialization failed

### Context.ParseSignatureDER

Parses a DER-encoded signature.

```go
func (c *Context) ParseSignatureDER(input []byte) (sig []byte, err error)
```

**Parameters:**
- `input`: DER-encoded signature

**Returns:**
- `sig`: 64-byte internal signature representation
- `err`: Error if parsing failed

### Context.SerializeSignatureCompact

Serializes a signature to compact format (64 bytes).

```go
func (c *Context) SerializeSignatureCompact(sig []byte) (output []byte, err error)
```

**Parameters:**
- `sig`: 64-byte internal signature

**Returns:**
- `output`: 64-byte compact signature
- `err`: Error if serialization failed

### Context.ParseSignatureCompact

Parses a compact (64-byte) signature.

```go
func (c *Context) ParseSignatureCompact(input64 []byte) (sig []byte, err error)
```

**Parameters:**
- `input64`: 64-byte compact signature

**Returns:**
- `sig`: 64-byte internal signature representation
- `err`: Error if parsing failed

### Context.NormalizeSignature

Normalizes a signature to lower-S form.

```go
func (c *Context) NormalizeSignature(sig []byte) (normalized []byte, wasNormalized bool, err error)
```

**Parameters:**
- `sig`: 64-byte internal signature

**Returns:**
- `normalized`: Normalized signature
- `wasNormalized`: true if signature was modified
- `err`: Error if normalization failed

---

## Schnorr Signatures

### Context.CreateKeypair

Creates a keypair for Schnorr signatures.

```go
func (c *Context) CreateKeypair(seckey []byte) (keypair Keypair, err error)
```

**Parameters:**
- `seckey`: 32-byte private key

**Returns:**
- `keypair`: 96-byte keypair structure
- `err`: Error if creation failed

### Context.KeypairXOnlyPub

Extracts the x-only public key from a keypair.

```go
func (c *Context) KeypairXOnlyPub(keypair Keypair) (xonly XOnlyPublicKey, pkParity int32, err error)
```

**Parameters:**
- `keypair`: 96-byte keypair

**Returns:**
- `xonly`: 32-byte x-only public key
- `pkParity`: Public key parity (0 or 1)
- `err`: Error if extraction failed

### Context.SchnorrSign

Creates a Schnorr signature (BIP-340).

```go
func (c *Context) SchnorrSign(msg32 []byte, keypair Keypair, auxRand32 []byte) (sig []byte, err error)
```

**Parameters:**
- `msg32`: 32-byte message hash
- `keypair`: 96-byte keypair
- `auxRand32`: 32 bytes of auxiliary random data (can be nil)

**Returns:**
- `sig`: 64-byte Schnorr signature
- `err`: Error if signing failed

### Context.SchnorrVerify

Verifies a Schnorr signature (BIP-340).

```go
func (c *Context) SchnorrVerify(sig64 []byte, msg []byte, xonlyPubkey []byte) (valid bool, err error)
```

**Parameters:**
- `sig64`: 64-byte Schnorr signature
- `msg`: Message (any length)
- `xonlyPubkey`: 32-byte x-only public key

**Returns:**
- `valid`: true if signature is valid
- `err`: Error if verification failed

### Context.ParseXOnlyPublicKey

Parses a 32-byte x-only public key.

```go
func (c *Context) ParseXOnlyPublicKey(input32 []byte) (xonly []byte, err error)
```

**Parameters:**
- `input32`: 32-byte x-only public key

**Returns:**
- `xonly`: 64-byte internal representation
- `err`: Error if parsing failed

### Context.SerializeXOnlyPublicKey

Serializes an x-only public key to 32 bytes.

```go
func (c *Context) SerializeXOnlyPublicKey(xonly []byte) (output32 []byte, err error)
```

**Parameters:**
- `xonly`: 64-byte internal x-only public key

**Returns:**
- `output32`: 32-byte serialized x-only public key
- `err`: Error if serialization failed

### Context.XOnlyPublicKeyFromPublicKey

Converts a regular public key to an x-only public key.

```go
func (c *Context) XOnlyPublicKeyFromPublicKey(pubkey []byte) (xonly []byte, pkParity int32, err error)
```

**Parameters:**
- `pubkey`: 64-byte internal public key

**Returns:**
- `xonly`: 64-byte internal x-only public key
- `pkParity`: Public key parity
- `err`: Error if conversion failed

---

## ECDH

### Context.ECDH

Computes an EC Diffie-Hellman shared secret.

```go
func (c *Context) ECDH(pubkey []byte, seckey []byte) (output []byte, err error)
```

**Parameters:**
- `pubkey`: 64-byte internal public key
- `seckey`: 32-byte private key

**Returns:**
- `output`: 32-byte shared secret
- `err`: Error if computation failed

---

## Recovery

### Context.SignRecoverable

Creates a recoverable ECDSA signature.

```go
func (c *Context) SignRecoverable(msg32 []byte, seckey []byte) (sig []byte, err error)
```

**Parameters:**
- `msg32`: 32-byte message hash
- `seckey`: 32-byte private key

**Returns:**
- `sig`: 65-byte recoverable signature
- `err`: Error if signing failed

### Context.SerializeRecoverableSignatureCompact

Serializes a recoverable signature.

```go
func (c *Context) SerializeRecoverableSignatureCompact(sig []byte) (output64 []byte, recid int32, err error)
```

**Parameters:**
- `sig`: 65-byte recoverable signature

**Returns:**
- `output64`: 64-byte compact signature
- `recid`: Recovery ID (0-3)
- `err`: Error if serialization failed

### Context.ParseRecoverableSignatureCompact

Parses a compact recoverable signature.

```go
func (c *Context) ParseRecoverableSignatureCompact(input64 []byte, recid int32) (sig []byte, err error)
```

**Parameters:**
- `input64`: 64-byte compact signature
- `recid`: Recovery ID (0-3)

**Returns:**
- `sig`: 65-byte recoverable signature
- `err`: Error if parsing failed

### Context.Recover

Recovers a public key from a recoverable signature.

```go
func (c *Context) Recover(sig []byte, msg32 []byte) (pubkey []byte, err error)
```

**Parameters:**
- `sig`: 65-byte recoverable signature
- `msg32`: 32-byte message hash

**Returns:**
- `pubkey`: 64-byte internal public key
- `err`: Error if recovery failed

---

## Utility Functions

Convenience functions that manage contexts automatically.

### GeneratePrivateKey

```go
func GeneratePrivateKey() (privKey []byte, err error)
```

Generates a random 32-byte private key.

### PublicKeyFromPrivate

```go
func PublicKeyFromPrivate(privKey []byte, compressed bool) (pubKey []byte, err error)
```

Generates a serialized public key from a private key.

### SignMessage

```go
func SignMessage(msgHash []byte, privKey []byte) (sig []byte, err error)
```

Signs a message and returns compact signature (64 bytes).

### VerifyMessage

```go
func VerifyMessage(msgHash []byte, compactSig []byte, serializedPubKey []byte) (valid bool, err error)
```

Verifies a compact signature.

### SignMessageDER

```go
func SignMessageDER(msgHash []byte, privKey []byte) (derSig []byte, err error)
```

Signs a message and returns DER-encoded signature.

### VerifyMessageDER

```go
func VerifyMessageDER(msgHash []byte, derSig []byte, serializedPubKey []byte) (valid bool, err error)
```

Verifies a DER-encoded signature.

### SchnorrSign

```go
func SchnorrSign(msgHash []byte, privKey []byte, auxRand []byte) (sig []byte, err error)
```

Creates a Schnorr signature (64 bytes).

### SchnorrVerifyWithPubKey

```go
func SchnorrVerifyWithPubKey(msgHash []byte, sig []byte, xonlyPubKey []byte) (valid bool, err error)
```

Verifies a Schnorr signature.

### XOnlyPubKeyFromPrivate

```go
func XOnlyPubKeyFromPrivate(privKey []byte) (xonly []byte, pkParity int32, err error)
```

Generates x-only public key from private key.

### ComputeECDH

```go
func ComputeECDH(serializedPubKey []byte, privKey []byte) (secret []byte, err error)
```

Computes ECDH shared secret.

### SignRecoverableCompact

```go
func SignRecoverableCompact(msgHash []byte, privKey []byte) (sig []byte, recID int32, err error)
```

Signs with recovery information.

### RecoverPubKey

```go
func RecoverPubKey(msgHash []byte, compactSig []byte, recID int32, compressed bool) (pubKey []byte, err error)
```

Recovers public key from signature.

### ValidatePrivateKey

```go
func ValidatePrivateKey(privKey []byte) (valid bool, err error)
```

Checks if a private key is valid.

### IsPublicKeyValid

```go
func IsPublicKeyValid(serializedPubKey []byte) (valid bool, err error)
```

Checks if a serialized public key is valid.

---

## Constants

### Context Flags

```go
const (
    ContextNone       = 1
    ContextVerify     = 257
    ContextSign       = 513
    ContextDeclassify = 1025
)
```

### EC Flags

```go
const (
    ECCompressed   = 258
    ECUncompressed = 2
)
```

### Size Constants

```go
const (
    PublicKeySize              = 64
    CompressedPublicKeySize    = 33
    UncompressedPublicKeySize  = 65
    SignatureSize              = 64
    CompactSignatureSize       = 64
    PrivateKeySize             = 32
    SharedSecretSize           = 32
    SchnorrSignatureSize       = 64
    RecoverableSignatureSize   = 65
)
```

---

## Types

### Context

```go
type Context struct {
    ctx uintptr
}
```

Opaque context handle.

### Keypair

```go
type Keypair [96]byte
```

Schnorr keypair structure.

### XOnlyPublicKey

```go
type XOnlyPublicKey [64]byte
```

64-byte x-only public key (internal representation).

---

## Error Handling

All functions return errors. Common error conditions:

- Library not loaded or not found
- Invalid parameter sizes
- Invalid keys or signatures
- Module not available (Schnorr, ECDH, Recovery)

Always check returned errors:

```go
result, err := secp.SomeFunction(...)
if err != nil {
    // Handle error
    return err
}
```

---

## Thread Safety

Context objects are **NOT** thread-safe. Each goroutine should create its own context.

Utility functions are safe to use concurrently as they create temporary contexts.

---

## Memory Management

Contexts are automatically cleaned up via finalizers, but explicit cleanup with `Destroy()` is recommended:

```go
ctx, _ := secp.NewContext(secp.ContextSign)
defer ctx.Destroy()
```

All byte slices returned by the library are copies and safe to use/modify.

