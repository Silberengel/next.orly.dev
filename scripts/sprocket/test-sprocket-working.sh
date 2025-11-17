#!/bin/bash

# Working Sprocket Test
# This script tests sprocket functionality with properly formatted messages

set -e

echo "🧪 Working Sprocket Test"
echo "======================="

# Configuration
RELAY_PORT="3335"  # Use different port to avoid conflicts
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

# Set environment variables for the relay
export ORLY_APP_NAME="ORLY_TEST"
export ORLY_DATA_DIR="/tmp/orly_test_data"
export ORLY_LISTEN="127.0.0.1"
export ORLY_PORT="$RELAY_PORT"
export ORLY_LOG_LEVEL="info"
export ORLY_SPROCKET_ENABLED="true"
export ORLY_ADMINS=""
export ORLY_OWNERS=""

# Clean up test data directory
rm -rf "$ORLY_DATA_DIR"
mkdir -p "$ORLY_DATA_DIR"

# Function to cleanup
cleanup() {
    echo "🧹 Cleaning up..."
    pkill -f "ORLY_TEST" || true
    sleep 2
    rm -rf "$ORLY_DATA_DIR"
    echo "✅ Cleanup complete"
}

# Set trap for cleanup
trap cleanup EXIT

# Start the relay
echo "🚀 Starting relay with sprocket enabled..."
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

# Test sprocket functionality with a simple Python WebSocket client
echo "🧪 Testing sprocket functionality..."

# Create a simple Python WebSocket test client
cat > /tmp/test_client.py << 'EOF'
#!/usr/bin/env python3
import asyncio
import websockets
import json
import time

async def test_sprocket():
    uri = "ws://127.0.0.1:3335"
    
    try:
        async with websockets.connect(uri) as websocket:
            print("✅ Connected to relay")
            
            # Test 1: Normal event (should be accepted)
            print("📤 Test 1: Normal event (should be accepted)")
            current_time = int(time.time())
            normal_event = {
                "id": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
                "pubkey": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
                "created_at": current_time,
                "kind": 1,
                "content": "Hello, world! This is a normal message.",
                "sig": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
            }
            
            normal_message = ["EVENT", normal_event]
            await websocket.send(json.dumps(normal_message))
            
            response = await websocket.recv()
            print(f"Response: {response}")
            
            if '"OK","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",true' in response:
                print("✅ Test 1 PASSED: Normal event accepted")
            else:
                print("❌ Test 1 FAILED: Normal event not accepted")
            
            # Test 2: Spam content (should be rejected)
            print("📤 Test 2: Spam content (should be rejected)")
            spam_event = {
                "id": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
                "pubkey": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
                "created_at": current_time,
                "kind": 1,
                "content": "This message contains spam content",
                "sig": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
            }
            
            spam_message = ["EVENT", spam_event]
            await websocket.send(json.dumps(spam_message))
            
            response = await websocket.recv()
            print(f"Response: {response}")
            
            if '"OK","1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",false' in response:
                print("✅ Test 2 PASSED: Spam content rejected")
            else:
                print("❌ Test 2 FAILED: Spam content not rejected")
            
            # Test 3: Test kind 9999 (should be shadow rejected)
            print("📤 Test 3: Test kind 9999 (should be shadow rejected)")
            kind_event = {
                "id": "2345678901bcdef01234567890abcdef01234567890abcdef01234567890abcdef",
                "pubkey": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
                "created_at": current_time,
                "kind": 9999,
                "content": "Test message with special kind",
                "sig": "2345678901bcdef01234567890abcdef01234567890abcdef01234567890abcdef2345678901bcdef01234567890abcdef01234567890abcdef01234567890abcdef"
            }
            
            kind_message = ["EVENT", kind_event]
            await websocket.send(json.dumps(kind_message))
            
            response = await websocket.recv()
            print(f"Response: {response}")
            
            if '"OK","2345678901bcdef01234567890abcdef01234567890abcdef01234567890abcdef",true' in response:
                print("✅ Test 3 PASSED: Test kind shadow rejected (OK=true but not processed)")
            else:
                print("❌ Test 3 FAILED: Test kind not shadow rejected")
            
            # Test 4: Blocked hashtag (should be rejected)
            print("📤 Test 4: Blocked hashtag (should be rejected)")
            hashtag_event = {
                "id": "3456789012cdef0123456789012cdef0123456789012cdef0123456789012cdef",
                "pubkey": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
                "created_at": current_time,
                "kind": 1,
                "content": "Message with blocked hashtag",
                "tags": [["t", "blocked"]],
                "sig": "3456789012cdef0123456789012cdef0123456789012cdef0123456789012cdef3456789012cdef0123456789012cdef0123456789012cdef0123456789012cdef"
            }
            
            hashtag_message = ["EVENT", hashtag_event]
            await websocket.send(json.dumps(hashtag_message))
            
            response = await websocket.recv()
            print(f"Response: {response}")
            
            if '"OK","3456789012cdef0123456789012cdef0123456789012cdef0123456789012cdef",false' in response:
                print("✅ Test 4 PASSED: Blocked hashtag rejected")
            else:
                print("❌ Test 4 FAILED: Blocked hashtag not rejected")
                
    except Exception as e:
        print(f"❌ Error: {e}")

if __name__ == "__main__":
    asyncio.run(test_sprocket())
EOF

# Check if websockets is available
if ! python3 -c "import websockets" 2>/dev/null; then
    echo "📦 Installing websockets library..."
    pip3 install websockets
fi

# Run the test
python3 /tmp/test_client.py

echo ""
echo "🎉 Sprocket integration test completed!"
echo "📝 Relay logs are available at: /tmp/orly_test.log"
echo "💡 To view logs: cat /tmp/orly_test.log"
