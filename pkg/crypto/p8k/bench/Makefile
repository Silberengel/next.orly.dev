.PHONY: help bench bench-all bench-pubkey bench-sign bench-verify bench-ecdh clean install

# Default target
help:
	@echo "Secp256k1 Implementation Benchmark Suite"
	@echo ""
	@echo "Available targets:"
	@echo "  bench            - Run all comparative benchmarks (10s each)"
	@echo "  bench-all        - Run all benchmarks with statistical analysis"
	@echo "  bench-pubkey     - Benchmark public key derivation"
	@echo "  bench-sign       - Benchmark Schnorr signing"
	@echo "  bench-verify     - Benchmark Schnorr verification"
	@echo "  bench-ecdh       - Benchmark ECDH key exchange"
	@echo "  bench-quick      - Quick benchmark run (1s each)"
	@echo "  install          - Install benchmark dependencies"
	@echo "  clean            - Clean benchmark results"
	@echo ""
	@echo "Environment variables:"
	@echo "  BENCHTIME        - Duration for each benchmark (default: 10s)"
	@echo "  COUNT            - Number of iterations (default: 5)"

# Run all comparative benchmarks
bench:
	go test -bench=BenchmarkAll -benchmem -benchtime=10s

# Quick benchmark (1 second each)
bench-quick:
	go test -bench=BenchmarkComparative -benchmem -benchtime=1s

# Run all benchmarks with detailed output
bench-all:
	./run_benchmarks.sh

# Individual operation benchmarks
bench-pubkey:
	go test -bench=BenchmarkComparative_PubkeyDerivation -benchmem -benchtime=10s

bench-sign:
	go test -bench=BenchmarkComparative_SchnorrSign -benchmem -benchtime=10s

bench-verify:
	go test -bench=BenchmarkComparative_SchnorrVerify -benchmem -benchtime=10s

bench-ecdh:
	go test -bench=BenchmarkComparative_ECDH -benchmem -benchtime=10s

# Run BTCEC-only benchmarks
bench-btcec:
	go test -bench=BenchmarkBTCEC -benchmem -benchtime=5s

# Run P256K1-only benchmarks
bench-p256k1:
	go test -bench=BenchmarkP256K1 -benchmem -benchtime=5s

# Run P8K-only benchmarks
bench-p8k:
	go test -bench=BenchmarkP8K -benchmem -benchtime=5s

# Install dependencies
install:
	go get -u ./...
	go mod tidy
	@echo "Installing benchstat for statistical analysis..."
	@go install golang.org/x/perf/cmd/benchstat@latest || echo "Note: benchstat install failed, but benchmarks will still work"

# Clean results
clean:
	rm -rf results/
	go clean -testcache

# Show module info
info:
	@echo "Benchmark module information:"
	@go list -m all

