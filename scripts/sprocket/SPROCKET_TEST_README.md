# Sprocket Test Suite

This directory contains a comprehensive test suite for the ORLY relay's sprocket event processing system.

## Overview

The sprocket system allows external scripts to process Nostr events before they are stored in the relay. Events are sent to the sprocket script via stdin, and the script responds with JSONL messages indicating whether to accept, reject, or shadow reject the event.

## Test Files

### Core Test Files

- **`test-sprocket.py`** - Python sprocket script that implements various filtering criteria
- **`test-sprocket-integration.go`** - Go integration tests using the testing framework
- **`test-sprocket-complete.sh`** - Complete test suite that starts relay and runs tests
- **`test-sprocket-manual.sh`** - Manual test script for interactive testing
- **`run-sprocket-test.sh`** - Automated test runner

### Example Scripts

- **`test-sprocket-example.sh`** - Simple bash example sprocket script

## Test Criteria

The Python sprocket script (`test-sprocket.py`) implements the following test criteria:

1. **Spam Content**: Rejects events containing "spam" in the content
2. **Test Kind**: Shadow rejects events with kind 9999
3. **Blocked Hashtags**: Rejects events with hashtags "blocked", "rejected", or "test-block"
4. **Blocked Pubkeys**: Shadow rejects events from pubkeys starting with "00000000", "11111111", or "22222222"
5. **Content Length**: Rejects events with content longer than 1000 characters
6. **Timestamp Validation**: Rejects events that are too old (>1 hour) or too far in the future (>5 minutes)

## Running Tests

### Quick Test (Recommended)

```bash
./test-sprocket-complete.sh
```

This script will:
1. Set up the test environment
2. Start the relay with sprocket enabled
3. Run all test cases
4. Clean up automatically

### Manual Testing

```bash
# Start relay manually with sprocket enabled
export ORLY_SPROCKET_ENABLED=true
go run . test

# In another terminal, run manual tests
./test-sprocket-manual.sh
```

### Integration Tests

```bash
# Run Go integration tests
go test -v -run TestSprocketIntegration ./test-sprocket-integration.go
```

## Prerequisites

- **Python 3**: Required for the Python sprocket script
- **jq**: Required for JSON processing in bash scripts
- **websocat**: Required for WebSocket testing
  ```bash
  cargo install websocat
  ```
- **Go dependencies**: gorilla/websocket for integration tests
  ```bash
  go get github.com/gorilla/websocket
  ```

## Test Cases

### 1. Normal Event (Accept)
```json
{
  "id": "test_normal_123",
  "pubkey": "1234567890abcdef1234567890abcdef12345678",
  "created_at": 1640995200,
  "kind": 1,
  "content": "Hello, world!",
  "sig": "test_sig"
}
```
**Expected**: `["OK","test_normal_123",true]`

### 2. Spam Content (Reject)
```json
{
  "id": "test_spam_456",
  "pubkey": "1234567890abcdef1234567890abcdef12345678",
  "created_at": 1640995200,
  "kind": 1,
  "content": "This is spam content",
  "sig": "test_sig"
}
```
**Expected**: `["OK","test_spam_456",false,"error: Content contains spam"]`

### 3. Test Kind (Shadow Reject)
```json
{
  "id": "test_kind_789",
  "pubkey": "1234567890abcdef1234567890abcdef12345678",
  "created_at": 1640995200,
  "kind": 9999,
  "content": "Test message",
  "sig": "test_sig"
}
```
**Expected**: `["OK","test_kind_789",true]` (but event not processed)

### 4. Blocked Hashtag (Reject)
```json
{
  "id": "test_hashtag_101",
  "pubkey": "1234567890abcdef1234567890abcdef12345678",
  "created_at": 1640995200,
  "kind": 1,
  "content": "Message with hashtag",
  "tags": [["t", "blocked"]],
  "sig": "test_sig"
}
```
**Expected**: `["OK","test_hashtag_101",false,"error: Hashtag \"blocked\" is not allowed"]`

### 5. Too Long Content (Reject)
```json
{
  "id": "test_long_202",
  "pubkey": "1234567890abcdef1234567890abcdef12345678",
  "created_at": 1640995200,
  "kind": 1,
  "content": "a... (1001 characters)",
  "sig": "test_sig"
}
```
**Expected**: `["OK","test_long_202",false,"error: Content too long (max 1000 characters)"]`

## Sprocket Script Protocol

### Input Format
Events are sent to the sprocket script as JSON objects via stdin, one per line.

### Output Format
The sprocket script must respond with JSONL (JSON Lines) format:

```json
{"id": "event_id", "action": "accept", "msg": ""}
{"id": "event_id", "action": "reject", "msg": "reason for rejection"}
{"id": "event_id", "action": "shadowReject", "msg": ""}
```

### Actions
- **`accept`**: Continue with normal event processing
- **`reject`**: Return OK false to client with message
- **`shadowReject`**: Return OK true to client but abort processing

## Configuration

To enable sprocket in the relay:

```bash
export ORLY_SPROCKET_ENABLED=true
export ORLY_APP_NAME="ORLY"
```

The sprocket script should be placed at:
`~/.config/{ORLY_APP_NAME}/sprocket.sh`

## Troubleshooting

### Common Issues

1. **Sprocket script not found**
   - Ensure the script exists at the correct path
   - Check file permissions (must be executable)

2. **Python script errors**
   - Verify Python 3 is installed
   - Check script syntax with `python3 -m py_compile test-sprocket.py`

3. **WebSocket connection failed**
   - Ensure relay is running on the correct port
   - Check firewall settings

4. **Test failures**
   - Check relay logs for sprocket errors
   - Verify sprocket script is responding correctly

### Debug Mode

Enable debug logging:
```bash
export ORLY_LOG_LEVEL=debug
```

### Manual Sprocket Testing

Test the sprocket script directly:
```bash
echo '{"id":"test","kind":1,"content":"spam test"}' | python3 test-sprocket.py
```

Expected output:
```json
{"id": "test", "action": "reject", "msg": "Content contains spam"}
```

## Contributing

When adding new test cases:

1. Add the test case to `test-sprocket.py`
2. Add corresponding test in `test-sprocket-complete.sh`
3. Update this README with the new test case
4. Ensure all tests pass before submitting

## License

This test suite is part of the ORLY relay project and follows the same license.
