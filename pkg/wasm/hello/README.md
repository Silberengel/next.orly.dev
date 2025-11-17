# WebAssembly Test Server

Simple Go web server for serving WebAssembly files with correct MIME types.

## Quick Start

```bash
# Build and run the server
go run server.go

# Or with custom port
go run server.go -port 3000

# Or serve from a different directory
go run server.go -dir /path/to/wasm/files
```

## Build and Install

```bash
# Build binary
go build -o wasm-server server.go

# Run
./wasm-server

# Install to PATH
go install
```

## Usage

Once the server is running, open your browser to:
- http://localhost:8080/

The server will serve:
- `index.html` - Main HTML page
- `hello.js` - JavaScript loader for WASM
- `hello.wasm` - WebAssembly binary module
- `hello.wat` - WebAssembly text format (for reference)

## Files

- **server.go** - Go web server with WASM MIME type support
- **index.html** - HTML page that loads the WASM module
- **hello.js** - JavaScript glue code to instantiate and run WASM
- **hello.wasm** - Compiled WebAssembly binary
- **hello.wat** - WebAssembly text format source

## Building WASM Files

### From WAT (WebAssembly Text Format)

```bash
# Install wabt tools
sudo apt install wabt

# Compile WAT to WASM
wat2wasm hello.wat -o hello.wasm

# Disassemble WASM back to WAT
wasm2wat hello.wasm -o hello.wat
```

### From Go (using TinyGo)

```bash
# Install TinyGo
wget https://github.com/tinygo-org/tinygo/releases/download/v0.31.0/tinygo_0.31.0_amd64.deb
sudo dpkg -i tinygo_0.31.0_amd64.deb

# Create Go program
cat > main.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello from Go WASM!")
}
EOF

# Compile to WASM
tinygo build -o main.wasm -target=wasm main.go

# Get the WASM runtime helper
cp $(tinygo env TINYGOROOT)/targets/wasm_exec.js .
```

## Browser Console

Open your browser's developer console (F12) to see the output from the WASM module.

The `hello.wasm` module should print "Hello, World!" to the console.

## CORS Headers

The server includes CORS headers to allow:
- Cross-origin requests during development
- Loading WASM modules from different origins

This is useful when developing and testing WASM modules.
