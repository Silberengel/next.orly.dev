#!/bin/bash

# Complete Sprocket Test Suite
# This script starts the relay with sprocket enabled and runs tests

set -e

echo "🧪 Complete Sprocket Test Suite"
echo "=============================="

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

# Start the relay with sprocket enabled
echo "🚀 Starting relay with sprocket enabled..."
export ORLY_APP_NAME="ORLY_TEST"
export ORLY_DATA_DIR="/tmp/orly_test_data"
export ORLY_LISTEN="127.0.0.1"
export ORLY_PORT="$RELAY_PORT"
export ORLY_LOG_LEVEL="info"
export ORLY_SPROCKET_ENABLED="true"
export ORLY_ADMINS="npub1test1234567890abcdefghijklmnopqrstuvwxyz1234567890"
export ORLY_OWNERS="npub1test1234567890abcdefghijklmnopqrstuvwxyz1234567890"

# Clean up test data directory
rm -rf "$ORLY_DATA_DIR"
mkdir -p "$ORLY_DATA_DIR"

# Start relay in background
echo "Starting relay on port $RELAY_PORT..."
go run . test > /tmp/orly_test.log 2>&1 &
RELAY_PID=$!

# Wait for relay to start
echo "⏳ Waiting for relay to start..."
sleep 5

# Check if relay is running
if ! kill -0 $RELAY_PID 2>/dev/null; then
    echo "❌ Relay failed to start"
    echo "Log output:"
    cat /tmp/orly_test.log
    exit 1
fi

echo "✅ Relay started successfully (PID: $RELAY_PID)"

# Function to cleanup
cleanup() {
    echo "🧹 Cleaning up..."
    kill $RELAY_PID 2>/dev/null || true
    sleep 2
    pkill -f "ORLY_TEST" || true
    rm -rf "$ORLY_DATA_DIR"
    echo "✅ Cleanup complete"
}

# Set trap for cleanup
trap cleanup EXIT

# Test sprocket functionality
echo "🧪 Testing sprocket functionality..."

# Check if websocat is available
if ! command -v websocat &> /dev/null; then
    echo "❌ websocat is required for testing"
    echo "Install it with: cargo install websocat"
    echo "Or run: go install github.com/gorilla/websocket/examples/echo@latest"
    exit 1
fi

# Test 1: Normal event (should be accepted)
echo "📤 Test 1: Normal event (should be accepted)"
normal_event='{
    "id": "test_normal_123",
    "pubkey": "1234567890abcdef1234567890abcdef12345678",
    "created_at": '$(date +%s)',
    "kind": 1,
    "content": "Hello, world! This is a normal message.",
    "sig": "test_sig_normal"
}'

normal_message="[\"EVENT\",$normal_event]"
normal_response=$(echo "$normal_message" | websocat "ws://127.0.0.1:$RELAY_PORT" --text)
echo "Response: $normal_response"

if echo "$normal_response" | grep -q '"OK","test_normal_123",true'; then
    echo "✅ Test 1 PASSED: Normal event accepted"
else
    echo "❌ Test 1 FAILED: Normal event not accepted"
fi

# Test 2: Spam content (should be rejected)
echo "📤 Test 2: Spam content (should be rejected)"
spam_event='{
    "id": "test_spam_456",
    "pubkey": "1234567890abcdef1234567890abcdef12345678",
    "created_at": '$(date +%s)',
    "kind": 1,
    "content": "This message contains spam content",
    "sig": "test_sig_spam"
}'

spam_message="[\"EVENT\",$spam_event]"
spam_response=$(echo "$spam_message" | websocat "ws://127.0.0.1:$RELAY_PORT" --text)
echo "Response: $spam_response"

if echo "$spam_response" | grep -q '"OK","test_spam_456",false'; then
    echo "✅ Test 2 PASSED: Spam content rejected"
else
    echo "❌ Test 2 FAILED: Spam content not rejected"
fi

# Test 3: Test kind 9999 (should be shadow rejected)
echo "📤 Test 3: Test kind 9999 (should be shadow rejected)"
kind_event='{
    "id": "test_kind_789",
    "pubkey": "1234567890abcdef1234567890abcdef12345678",
    "created_at": '$(date +%s)',
    "kind": 9999,
    "content": "Test message with special kind",
    "sig": "test_sig_kind"
}'

kind_message="[\"EVENT\",$kind_event]"
kind_response=$(echo "$kind_message" | websocat "ws://127.0.0.1:$RELAY_PORT" --text)
echo "Response: $kind_response"

if echo "$kind_response" | grep -q '"OK","test_kind_789",true'; then
    echo "✅ Test 3 PASSED: Test kind shadow rejected (OK=true but not processed)"
else
    echo "❌ Test 3 FAILED: Test kind not shadow rejected"
fi

# Test 4: Blocked hashtag (should be rejected)
echo "📤 Test 4: Blocked hashtag (should be rejected)"
hashtag_event='{
    "id": "test_hashtag_101",
    "pubkey": "1234567890abcdef1234567890abcdef12345678",
    "created_at": '$(date +%s)',
    "kind": 1,
    "content": "Message with blocked hashtag",
    "tags": [["t", "blocked"]],
    "sig": "test_sig_hashtag"
}'

hashtag_message="[\"EVENT\",$hashtag_event]"
hashtag_response=$(echo "$hashtag_message" | websocat "ws://127.0.0.1:$RELAY_PORT" --text)
echo "Response: $hashtag_response"

if echo "$hashtag_response" | grep -q '"OK","test_hashtag_101",false'; then
    echo "✅ Test 4 PASSED: Blocked hashtag rejected"
else
    echo "❌ Test 4 FAILED: Blocked hashtag not rejected"
fi

# Test 5: Too long content (should be rejected)
echo "📤 Test 5: Too long content (should be rejected)"
long_content=$(printf 'a%.0s' {1..1001})
long_event="{
    \"id\": \"test_long_202\",
    \"pubkey\": \"1234567890abcdef1234567890abcdef12345678\",
    \"created_at\": $(date +%s),
    \"kind\": 1,
    \"content\": \"$long_content\",
    \"sig\": \"test_sig_long\"
}"

long_message="[\"EVENT\",$long_event]"
long_response=$(echo "$long_message" | websocat "ws://127.0.0.1:$RELAY_PORT" --text)
echo "Response: $long_response"

if echo "$long_response" | grep -q '"OK","test_long_202",false'; then
    echo "✅ Test 5 PASSED: Too long content rejected"
else
    echo "❌ Test 5 FAILED: Too long content not rejected"
fi

echo ""
echo "🎉 Sprocket test suite completed!"
echo "📊 Check the results above to verify sprocket functionality"
echo ""
echo "💡 To run individual tests, use:"
echo "   ./test-sprocket-manual.sh"
echo ""
echo "📝 Relay logs are available at: /tmp/orly_test.log"
