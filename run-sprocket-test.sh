#!/bin/bash

# Sprocket Integration Test Runner
# This script sets up and runs the sprocket integration test

set -e

echo "🧪 Running Sprocket Integration Test"
echo "===================================="

# Check if Python 3 is available
if ! command -v python3 &> /dev/null; then
    echo "❌ Python 3 is required but not installed"
    exit 1
fi

# Check if jq is available (for the bash sprocket script)
if ! command -v jq &> /dev/null; then
    echo "❌ jq is required but not installed"
    exit 1
fi

# Check if gorilla/websocket is available
echo "📦 Installing test dependencies..."
go mod tidy
go get github.com/gorilla/websocket

# Create test configuration directory
TEST_CONFIG_DIR="$HOME/.config/ORLY_TEST"
mkdir -p "$TEST_CONFIG_DIR"

# Copy the Python sprocket script to the test directory
cp test-sprocket.py "$TEST_CONFIG_DIR/sprocket.py"

# Create a simple bash wrapper for the Python script
cat > "$TEST_CONFIG_DIR/sprocket.sh" << 'EOF'
#!/bin/bash
python3 "$(dirname "$0")/sprocket.py"
EOF

chmod +x "$TEST_CONFIG_DIR/sprocket.sh"

echo "🔧 Test setup complete"
echo "📁 Sprocket script location: $TEST_CONFIG_DIR/sprocket.sh"

# Run the integration test
echo "🚀 Starting integration test..."
go test -v -run TestSprocketIntegration ./test-sprocket-integration.go

echo "✅ Sprocket integration test completed successfully!"
