# WASM Shell Runner

Run WebAssembly programs directly in your shell with stdin/stdout support using WASI (WebAssembly System Interface).

## Quick Start

```bash
# Build all WAT files to WASM
./build.sh

# Run the hello example
./run.sh hello.wasm

# Run the echo example (with stdin)
echo "Hello World" | ./run.sh echo.wasm

# Or interactive
./run.sh echo.wasm
```

## Prerequisites

### Install wabt (WebAssembly Binary Toolkit)

```bash
# Ubuntu/Debian
sudo apt install wabt

# Provides: wat2wasm, wasm2wat, wasm-objdump, etc.
```

### Install wasmtime (WASM Runtime)

```bash
# Install via official installer
curl https://wasmtime.dev/install.sh -sSf | bash

# Add to PATH (add to ~/.bashrc for persistence)
export PATH="$HOME/.wasmtime/bin:$PATH"
```

## Examples

### 1. Hello World (`hello.wat`)

Simple example that prints to stdout:

```bash
./build.sh
./run.sh hello.wasm
```

**Output:**
```
Hello from WASM shell!
```

### 2. Echo Program (`echo.wat`)

Reads from stdin and echoes back:

```bash
./build.sh

# Interactive mode
./run.sh echo.wasm
# Type something and press Enter

# Piped input
echo "Test message" | ./run.sh echo.wasm

# From file
cat somefile.txt | ./run.sh echo.wasm
```

**Output:**
```
Enter text: Test message
You entered: Test message
```

## How It Works

### WASI (WebAssembly System Interface)

WASI provides a standard interface for WASM programs to interact with the host system:

- **stdin** (fd 0) - Standard input
- **stdout** (fd 1) - Standard output
- **stderr** (fd 2) - Standard error

### Key WASI Functions Used

#### `fd_write` - Write to file descriptor
```wat
(import "wasi_snapshot_preview1" "fd_write"
  (func $fd_write (param i32 i32 i32 i32) (result i32)))

;; Usage: fd_write(fd, iovs_ptr, iovs_len, nwritten_ptr) -> errno
;; fd: File descriptor (1 = stdout)
;; iovs_ptr: Pointer to iovec array
;; iovs_len: Number of iovecs
;; nwritten_ptr: Where to store bytes written
```

#### `fd_read` - Read from file descriptor
```wat
(import "wasi_snapshot_preview1" "fd_read"
  (func $fd_read (param i32 i32 i32 i32) (result i32)))

;; Usage: fd_read(fd, iovs_ptr, iovs_len, nread_ptr) -> errno
;; fd: File descriptor (0 = stdin)
;; iovs_ptr: Pointer to iovec array
;; iovs_len: Number of iovecs
;; nread_ptr: Where to store bytes read
```

### iovec Structure

Both functions use an iovec (I/O vector) structure:

```
struct iovec {
    u32 buf;      // Pointer to buffer in WASM memory
    u32 buf_len;  // Length of buffer
}
```

## Writing Your Own WASM Programs

### Basic Template

```wat
(module
  ;; Import WASI functions you need
  (import "wasi_snapshot_preview1" "fd_write"
    (func $fd_write (param i32 i32 i32 i32) (result i32)))

  ;; Allocate memory
  (memory 1)
  (export "memory" (memory 0))

  ;; Store your strings in memory
  (data (i32.const 0) "Your message here\n")

  ;; Main function (entry point)
  (func $main (export "_start")
    ;; Setup iovec at some offset (e.g., 100)
    (i32.store (i32.const 100) (i32.const 0))   ;; buf pointer
    (i32.store (i32.const 104) (i32.const 18))  ;; buf length

    ;; Write to stdout
    (call $fd_write
      (i32.const 1)   ;; stdout
      (i32.const 100) ;; iovec pointer
      (i32.const 1)   ;; number of iovecs
      (i32.const 200) ;; nwritten pointer
    )
    drop ;; drop return value
  )
)
```

### Build and Run

```bash
# Compile WAT to WASM
wat2wasm yourprogram.wat -o yourprogram.wasm

# Run it
./run.sh yourprogram.wasm

# Or directly with wasmtime
wasmtime yourprogram.wasm
```

## Advanced Usage

### Pass Arguments

```bash
# WASM programs can receive command-line arguments
./run.sh program.wasm arg1 arg2 arg3
```

### Environment Variables

```bash
# Set environment variables (wasmtime flag)
wasmtime --env KEY=value program.wasm
```

### Mount Directories

```bash
# Give WASM access to directories (wasmtime flag)
wasmtime --dir=/tmp program.wasm
```

### Call Specific Functions

```bash
# Instead of _start, call a specific exported function
wasmtime --invoke my_function program.wasm
```

## Compiling from High-Level Languages

### From Go (using TinyGo)

```bash
# Install TinyGo
wget https://github.com/tinygo-org/tinygo/releases/download/v0.31.0/tinygo_0.31.0_amd64.deb
sudo dpkg -i tinygo_0.31.0_amd64.deb

# Write Go program
cat > main.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello from Go WASM!")
}
EOF

# Compile to WASM with WASI
tinygo build -o program.wasm -target=wasi main.go

# Run
./run.sh program.wasm
```

### From Rust

```bash
# Add WASI target
rustup target add wasm32-wasi

# Create project
cargo new --bin myprogram
cd myprogram

# Build for WASI
cargo build --target wasm32-wasi --release

# Run
wasmtime target/wasm32-wasi/release/myprogram.wasm
```

### From C/C++ (using wasi-sdk)

```bash
# Download wasi-sdk
wget https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-21/wasi-sdk-21.0-linux.tar.gz
tar xf wasi-sdk-21.0-linux.tar.gz

# Compile C program
cat > hello.c << 'EOF'
#include <stdio.h>

int main() {
    printf("Hello from C WASM!\n");
    return 0;
}
EOF

# Compile to WASM
./wasi-sdk-21.0/bin/clang hello.c -o hello.wasm

# Run
wasmtime hello.wasm
```

## Debugging

### Inspect WASM Module

```bash
# Disassemble WASM to WAT
wasm2wat program.wasm -o program.wat

# Show module structure
wasm-objdump -x program.wasm

# Show imports
wasm-objdump -x program.wasm | grep -A 10 "Import"

# Show exports
wasm-objdump -x program.wasm | grep -A 10 "Export"
```

### Verbose Execution

```bash
# Run with logging
WASMTIME_LOG=wasmtime=trace wasmtime program.wasm

# Enable debug info
wasmtime -g program.wasm
```

## Use Cases for ORLY

WASM with WASI is perfect for ORLY's policy system:

### Sandboxed Policy Scripts

```bash
# Write policy in any language that compiles to WASM
# Run it safely in a sandbox with controlled stdin/stdout
./run.sh policy.wasm < event.json
```

**Benefits:**
- **Security**: Sandboxed execution, no system access unless granted
- **Portability**: Same WASM runs on any platform
- **Performance**: Near-native speed with wasmtime's JIT
- **Language Choice**: Write policies in Go, Rust, C, JavaScript, etc.
- **Deterministic**: Same input always produces same output

### Example Policy Flow

```bash
# Event comes in via stdin (JSON)
echo '{"kind":1,"content":"hello"}' | ./run.sh filter-policy.wasm

# Policy outputs "accept" or "reject" to stdout
# ORLY reads the decision and acts accordingly
```

## Scripts Reference

### `build.sh`
Compiles all `.wat` files to `.wasm` using `wat2wasm`

### `run.sh [wasm-file] [args...]`
Runs a WASM file with `wasmtime`, defaults to `hello.wasm`

## Files

- **hello.wat** - Simple stdout example
- **echo.wat** - stdin/stdout interactive example
- **build.sh** - Build all WAT files
- **run.sh** - Run WASM files with wasmtime

## Resources

- [WASI Specification](https://github.com/WebAssembly/WASI)
- [Wasmtime Documentation](https://docs.wasmtime.dev/)
- [WebAssembly Reference](https://webassembly.github.io/spec/)
- [WAT Language Guide](https://developer.mozilla.org/en-US/docs/WebAssembly/Understanding_the_text_format)
- [TinyGo WASI Support](https://tinygo.org/docs/guides/webassembly/wasi/)
