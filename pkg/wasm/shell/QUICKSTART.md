# Quick Start Guide

## TL;DR

```bash
# Build all examples
./build.sh

# Run hello example (stdout only)
./run.sh hello.wasm

# Run echo example (stdin/stdout)
echo "test" | ./run.sh echo.wasm

# Run all tests
./test.sh
```

## What's Included

### Scripts
- **`build.sh`** - Compile all `.wat` files to `.wasm` using `wat2wasm`
- **`run.sh`** - Execute WASM files with `wasmtime` WASI runtime
- **`test.sh`** - Run complete test suite

### Examples
- **`hello.wat/wasm`** - Simple "Hello World" to stdout
- **`echo.wat/wasm`** - Read from stdin, echo to stdout

### Documentation
- **`README.md`** - Complete documentation with examples
- **`QUICKSTART.md`** - This file

## Running WASM in Shell - The Basics

### Console Output (stdout)
```bash
./run.sh hello.wasm
# Output: Hello from WASM shell!
```

### Console Input (stdin)
```bash
# Piped input
echo "your text" | ./run.sh echo.wasm

# Interactive input
./run.sh echo.wasm
# (type your input and press Enter)

# From file
cat file.txt | ./run.sh echo.wasm
```

## Use Case: ORLY Policy Scripts

This WASM shell runner is perfect for ORLY's policy system:

```bash
# Event JSON comes via stdin
echo '{"kind":1,"content":"hello","pubkey":"..."}' | ./run.sh policy.wasm

# Policy script:
# - Reads JSON from stdin
# - Applies rules
# - Outputs decision to stdout: "accept" or "reject"

# ORLY reads the decision and acts accordingly
```

### Benefits
- **Sandboxed** - Cannot access system unless explicitly granted
- **Fast** - Near-native performance with wasmtime's JIT
- **Portable** - Same WASM binary runs everywhere
- **Multi-language** - Write policies in Go, Rust, C, JavaScript, etc.
- **Deterministic** - Same input = same output, always

## Next Steps

1. **Read the full README** - `cat README.md`
2. **Try the examples** - `./test.sh`
3. **Write your own** - Start with the template in README.md
4. **Compile from Go** - Use TinyGo to compile Go to WASM
5. **Integrate with ORLY** - Use as policy execution engine

## File Structure

```
pkg/wasm/shell/
├── build.sh           # Build script (wat -> wasm)
├── run.sh             # Run script (execute wasm)
├── test.sh            # Test all examples
├── hello.wat          # Source: Hello World
├── hello.wasm         # Binary: Hello World
├── echo.wat           # Source: Echo stdin/stdout
├── echo.wasm          # Binary: Echo stdin/stdout
├── README.md          # Full documentation
└── QUICKSTART.md      # This file
```

## Troubleshooting

### "wasmtime not found"
```bash
curl https://wasmtime.dev/install.sh -sSf | bash
export PATH="$HOME/.wasmtime/bin:$PATH"
```

### "wat2wasm not found"
```bash
sudo apt install wabt
```

### WASM fails to run
```bash
# Rebuild from source
./build.sh

# Check the WASM module
wasm-objdump -x your.wasm
```

---

**Happy WASM hacking!** 🎉
