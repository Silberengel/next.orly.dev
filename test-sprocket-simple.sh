#!/bin/bash

# Simple Sprocket Test
# This script demonstrates sprocket functionality

set -e

echo "🧪 Simple Sprocket Test"
echo "======================"

# Configuration
RELAY_PORT="3334"
TEST_CONFIG_DIR="$HOME/.config/ORLY_TEST"

# Clean up any existing test processes
echo "🧹 Cleaning up existing processes..."
pkill -f "ORLY_TEST" || true
sleep 2

# Create test configuration directory
echo "📁 Setting up test environment..."
mkdir -p "$TEST_CONFIG_DIR"

# Copy the Python sprocket script
cp test-sprocket.py "$TEST_CONFIG_DIR/sprocket.py"

# Create bash wrapper for the Python script
cat > "$TEST_CONFIG_DIR/sprocket.sh" << 'EOF'
#!/bin/bash
python3 "$(dirname "$0")/sprocket.py"
EOF

chmod +x "$TEST_CONFIG_DIR/sprocket.sh"

echo "✅ Sprocket script created at: $TEST_CONFIG_DIR/sprocket.sh"

# Test the sprocket script directly first
echo "🧪 Testing sprocket script directly..."

# Test 1: Normal event
echo "📤 Test 1: Normal event"
current_time=$(date +%s)
normal_event="{\"id\":\"test_normal_123\",\"pubkey\":\"1234567890abcdef1234567890abcdef12345678\",\"created_at\":$current_time,\"kind\":1,\"content\":\"Hello, world!\",\"sig\":\"test_sig\"}"
echo "$normal_event" | python3 "$TEST_CONFIG_DIR/sprocket.py"

# Test 2: Spam content
echo "📤 Test 2: Spam content"
spam_event="{\"id\":\"test_spam_456\",\"pubkey\":\"1234567890abcdef1234567890abcdef12345678\",\"created_at\":$current_time,\"kind\":1,\"content\":\"This is spam content\",\"sig\":\"test_sig\"}"
echo "$spam_event" | python3 "$TEST_CONFIG_DIR/sprocket.py"

# Test 3: Test kind 9999
echo "📤 Test 3: Test kind 9999"
kind_event="{\"id\":\"test_kind_789\",\"pubkey\":\"1234567890abcdef1234567890abcdef12345678\",\"created_at\":$current_time,\"kind\":9999,\"content\":\"Test message\",\"sig\":\"test_sig\"}"
echo "$kind_event" | python3 "$TEST_CONFIG_DIR/sprocket.py"

# Test 4: Blocked hashtag
echo "📤 Test 4: Blocked hashtag"
hashtag_event="{\"id\":\"test_hashtag_101\",\"pubkey\":\"1234567890abcdef1234567890abcdef12345678\",\"created_at\":$current_time,\"kind\":1,\"content\":\"Message with hashtag\",\"tags\":[[\"t\",\"blocked\"]],\"sig\":\"test_sig\"}"
echo "$hashtag_event" | python3 "$TEST_CONFIG_DIR/sprocket.py"

echo ""
echo "✅ Direct sprocket script tests completed!"
echo ""
echo "Expected results:"
echo "1. Normal event: {\"id\":\"test_normal_123\",\"action\":\"accept\",\"msg\":\"\"}"
echo "2. Spam content: {\"id\":\"test_spam_456\",\"action\":\"reject\",\"msg\":\"Content contains spam\"}"
echo "3. Test kind 9999: {\"id\":\"test_kind_789\",\"action\":\"shadowReject\",\"msg\":\"\"}"
echo "4. Blocked hashtag: {\"id\":\"test_hashtag_101\",\"action\":\"reject\",\"msg\":\"Hashtag \\\"blocked\\\" is not allowed\"}"
echo ""
echo "💡 To test with the full relay, run:"
echo "   export ORLY_SPROCKET_ENABLED=true"
echo "   export ORLY_APP_NAME=ORLY_TEST"
echo "   go run . test"
echo ""
echo "   Then in another terminal:"
echo "   ./test-sprocket-manual.sh"
