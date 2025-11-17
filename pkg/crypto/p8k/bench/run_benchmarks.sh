#!/bin/bash

# Benchmark runner script for secp256k1 implementation comparison
# Runs benchmarks multiple times and generates statistical analysis

set -e

echo "=========================================="
echo "secp256k1 Implementation Benchmark Suite"
echo "=========================================="
echo ""

# Check for dependencies
echo "Checking dependencies..."

if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

if ! command -v benchstat &> /dev/null; then
    echo "Installing benchstat for statistical analysis..."
    go install golang.org/x/perf/cmd/benchstat@latest
fi

# Check if libsecp256k1 is available
if ! ldconfig -p | grep -q libsecp256k1; then
    echo "Warning: libsecp256k1 not found. P8K benchmarks may be skipped."
    echo "Install with: sudo apt-get install libsecp256k1-dev (Ubuntu/Debian)"
    echo "or: brew install libsecp256k1 (macOS)"
    echo ""
fi

# Configuration
BENCHTIME=${BENCHTIME:-3s}
COUNT=${COUNT:-1}
OUTPUT_DIR="results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo "Benchmark configuration:"
echo "  Duration: $BENCHTIME per benchmark"
echo "  Iterations: $COUNT runs"
echo "  Output directory: $OUTPUT_DIR"
echo ""

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Function to run benchmarks
run_benchmark() {
    local name=$1
    local bench_pattern=$2
    local output_file="$OUTPUT_DIR/${name}_${TIMESTAMP}.txt"
    
    echo "Running: $name"
    echo "  Output: $output_file"
    
    go test -bench="$bench_pattern" \
        -benchmem \
        -benchtime="$BENCHTIME" \
        -count="$COUNT" \
        2>&1 | tee "$output_file"
    
    echo "✓ Completed: $name"
    echo ""
}

# Run all benchmarks
echo "=========================================="
echo "Running Benchmarks"
echo "=========================================="
echo ""

run_benchmark "all_operations" "BenchmarkAll"
run_benchmark "pubkey_derivation" "BenchmarkComparative_PubkeyDerivation"
run_benchmark "schnorr_sign" "BenchmarkComparative_SchnorrSign"
run_benchmark "schnorr_verify" "BenchmarkComparative_SchnorrVerify"
run_benchmark "ecdh" "BenchmarkComparative_ECDH"

# Run individual implementation benchmarks
run_benchmark "btcec_only" "BenchmarkBTCEC"
run_benchmark "p256k1_only" "BenchmarkP256K1"
run_benchmark "p8k_only" "BenchmarkP8K"

# Generate statistical analysis
echo "=========================================="
echo "Generating Statistical Analysis"
echo "=========================================="
echo ""

for file in "$OUTPUT_DIR"/*_${TIMESTAMP}.txt; do
    if [ -f "$file" ]; then
        basename=$(basename "$file" .txt)
        echo "Analysis: $basename"
        benchstat "$file" | tee "$OUTPUT_DIR/${basename}_stats.txt"
        echo ""
    fi
done

# Generate comparison report
COMPARISON_FILE="$OUTPUT_DIR/comparison_${TIMESTAMP}.txt"
echo "=========================================="
echo "Implementation Comparison Summary"
echo "=========================================="
echo ""

echo "Comparison between implementations" > "$COMPARISON_FILE"
echo "Generated: $(date)" >> "$COMPARISON_FILE"
echo "" >> "$COMPARISON_FILE"

# Compare each operation
for op in pubkey_derivation schnorr_sign schnorr_verify ecdh; do
    file="$OUTPUT_DIR/${op}_${TIMESTAMP}.txt"
    if [ -f "$file" ]; then
        echo "=== $op ===" >> "$COMPARISON_FILE"
        benchstat "$file" >> "$COMPARISON_FILE"
        echo "" >> "$COMPARISON_FILE"
    fi
done

cat "$COMPARISON_FILE"

echo "=========================================="
echo "Benchmark Results Summary"
echo "=========================================="
echo ""
echo "Results saved to: $OUTPUT_DIR"
echo ""
echo "Files generated:"
ls -lh "$OUTPUT_DIR"/*_${TIMESTAMP}* | awk '{print "  " $9 " (" $5 ")"}'
echo ""

# Generate markdown report
MARKDOWN_FILE="$OUTPUT_DIR/REPORT_${TIMESTAMP}.md"
echo "Generating markdown report: $MARKDOWN_FILE"

cat > "$MARKDOWN_FILE" << 'EOF'
# secp256k1 Implementation Benchmark Results

## Test Environment

EOF

echo "- **Date**: $(date)" >> "$MARKDOWN_FILE"
echo "- **Go Version**: $(go version)" >> "$MARKDOWN_FILE"
echo "- **OS**: $(uname -s) $(uname -r)" >> "$MARKDOWN_FILE"
echo "- **CPU**: $(grep -m1 "model name" /proc/cpuinfo 2>/dev/null | cut -d: -f2 | xargs || echo "Unknown")" >> "$MARKDOWN_FILE"
echo "- **Benchmark Time**: $BENCHTIME per test" >> "$MARKDOWN_FILE"
echo "- **Iterations**: $COUNT runs" >> "$MARKDOWN_FILE"
echo "" >> "$MARKDOWN_FILE"

cat >> "$MARKDOWN_FILE" << 'EOF'
## Implementations Tested

1. **BTCEC** - btcsuite/btcd implementation (pure Go)
2. **P256K1** - mleku/p256k1 implementation (pure Go)
3. **P8K** - p8k.mleku.dev implementation (purego, C bindings)

## Results

EOF

# Add results from comparison file
cat "$COMPARISON_FILE" >> "$MARKDOWN_FILE"

echo "" >> "$MARKDOWN_FILE"
echo "## Raw Data" >> "$MARKDOWN_FILE"
echo "" >> "$MARKDOWN_FILE"
echo "Full benchmark results are available in:" >> "$MARKDOWN_FILE"
echo "" >> "$MARKDOWN_FILE"
for file in "$OUTPUT_DIR"/*_${TIMESTAMP}.txt; do
    if [ -f "$file" ]; then
        echo "- $(basename "$file")" >> "$MARKDOWN_FILE"
    fi
done

echo ""
echo "✓ Markdown report generated: $MARKDOWN_FILE"
echo ""
echo "=========================================="
echo "Benchmark suite completed!"
echo "=========================================="

