.PHONY: test build clean examples install-deps check fmt vet lint

# Test the package
test:
	go test -v ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Build examples
build: examples

examples:
	@echo "Building examples..."
	@mkdir -p bin
	@go build -o bin/ecdsa-example ./examples/ecdsa
	@go build -o bin/schnorr-example ./examples/schnorr
	@go build -o bin/ecdh-example ./examples/ecdh
	@go build -o bin/recovery-example ./examples/recovery
	@echo "Examples built in bin/"

# Run all examples
run-examples: examples
	@echo "\n=== ECDSA Example ==="
	@./bin/ecdsa-example
	@echo "\n=== Schnorr Example ==="
	@./bin/schnorr-example || echo "Schnorr module not available"
	@echo "\n=== ECDH Example ==="
	@./bin/ecdh-example || echo "ECDH module not available"
	@echo "\n=== Recovery Example ==="
	@./bin/recovery-example || echo "Recovery module not available"

# Clean build artifacts
clean:
	@rm -rf bin/
	@go clean

# Install dependencies
install-deps:
	go get -u ./...
	go mod tidy

# Check code
check: fmt vet

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run linter (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/usage/install/"; exit 1)
	golangci-lint run

# Show module information
info:
	@echo "Module: p8k.mleku.dev"
	@echo "Go version: $(shell go version)"
	@echo "Dependencies:"
	@go list -m all

# Download and build libsecp256k1 from source (Linux/macOS)
install-secp256k1:
	@echo "Downloading and building libsecp256k1..."
	@rm -rf /tmp/secp256k1
	@git clone https://github.com/bitcoin-core/secp256k1 /tmp/secp256k1
	@cd /tmp/secp256k1 && ./autogen.sh
	@cd /tmp/secp256k1 && ./configure --enable-module-recovery --enable-module-schnorrsig --enable-module-ecdh --enable-module-extrakeys
	@cd /tmp/secp256k1 && make
	@cd /tmp/secp256k1 && sudo make install
	@sudo ldconfig || true
	@echo "libsecp256k1 installed successfully"

# Help
help:
	@echo "Available targets:"
	@echo "  test              - Run tests"
	@echo "  bench             - Run benchmarks"
	@echo "  build             - Build examples"
	@echo "  examples          - Build examples (alias for build)"
	@echo "  run-examples      - Build and run all examples"
	@echo "  clean             - Clean build artifacts"
	@echo "  install-deps      - Install Go dependencies"
	@echo "  check             - Run fmt and vet"
	@echo "  fmt               - Format code"
	@echo "  vet               - Run go vet"
	@echo "  lint              - Run golangci-lint"
	@echo "  info              - Show module information"
	@echo "  install-secp256k1 - Download and build libsecp256k1 from source"
	@echo "  help              - Show this help message"

