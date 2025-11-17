#!/bin/bash

# Manual Sprocket Test Script
# This script demonstrates sprocket functionality by sending test events

set -e

echo "🧪 Manual Sprocket Test"
echo "======================"

# Configuration
RELAY_HOST="127.0.0.1"
RELAY_PORT="3334"
RELAY_URL="ws://$RELAY_HOST:$RELAY_PORT"

# Check if websocat is available
if ! command -v websocat &> /dev/null; then
    echo "❌ websocat is required for this test"
    echo "Install it with: cargo install websocat"
    exit 1
fi

# Function to send an event and get response
send_event() {
    local event_json="$1"
    local description="$2"
    
    echo "📤 Testing: $description"
    echo "Event: $event_json"
    
    # Send EVENT message
    local message="[\"EVENT\",$event_json]"
    echo "Sending: $message"
    
    # Send and receive response
    local response=$(echo "$message" | websocat "$RELAY_URL" --text)
    echo "Response: $response"
    echo "---"
}

# Test events
echo "🚀 Starting manual sprocket test..."
echo "Make sure the relay is running with sprocket enabled!"
echo ""

# Test 1: Normal event (should be accepted)
send_event '{
    "id": "test_normal_123",
    "pubkey": "1234567890abcdef1234567890abcdef12345678",
    "created_at": '$(date +%s)',
    "kind": 1,
    "content": "Hello, world! This is a normal message.",
    "sig": "test_sig_normal"
}' "Normal event (should be accepted)"

# Test 2: Spam content (should be rejected)
send_event '{
    "id": "test_spam_456",
    "pubkey": "1234567890abcdef1234567890abcdef12345678",
    "created_at": '$(date +%s)',
    "kind": 1,
    "content": "This message contains spam content",
    "sig": "test_sig_spam"
}' "Spam content (should be rejected)"

# Test 3: Test kind 9999 (should be shadow rejected)
send_event '{
    "id": "test_kind_789",
    "pubkey": "1234567890abcdef1234567890abcdef12345678",
    "created_at": '$(date +%s)',
    "kind": 9999,
    "content": "Test message with special kind",
    "sig": "test_sig_kind"
}' "Test kind 9999 (should be shadow rejected)"

# Test 4: Blocked hashtag (should be rejected)
send_event '{
    "id": "test_hashtag_101",
    "pubkey": "1234567890abcdef1234567890abcdef12345678",
    "created_at": '$(date +%s)',
    "kind": 1,
    "content": "Message with blocked hashtag",
    "tags": [["t", "blocked"]],
    "sig": "test_sig_hashtag"
}' "Blocked hashtag (should be rejected)"

# Test 5: Too long content (should be rejected)
send_event '{
    "id": "test_long_202",
    "pubkey": "1234567890abcdef1234567890abcdef12345678",
    "created_at": '$(date +%s)',
    "kind": 1,
    "content": "'$(printf 'a%.0s' {1..1001})'",
    "sig": "test_sig_long"
}' "Too long content (should be rejected)"

# Test 6: Old timestamp (should be rejected)
send_event '{
    "id": "test_old_303",
    "pubkey": "1234567890abcdef1234567890abcdef12345678",
    "created_at": '$(($(date +%s) - 7200))',
    "kind": 1,
    "content": "Message with old timestamp",
    "sig": "test_sig_old"
}' "Old timestamp (should be rejected)"

echo "✅ Manual sprocket test completed!"
echo ""
echo "Expected results:"
echo "- Normal event: OK, true"
echo "- Spam content: OK, false, 'Content contains spam'"
echo "- Test kind 9999: OK, true (but shadow rejected)"
echo "- Blocked hashtag: OK, false, 'Hashtag blocked is not allowed'"
echo "- Too long content: OK, false, 'Content too long'"
echo "- Old timestamp: OK, false, 'Event timestamp too old'"
