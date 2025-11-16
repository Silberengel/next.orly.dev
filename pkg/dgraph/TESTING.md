# Dgraph Test Suite

This directory contains a comprehensive test suite for the dgraph database implementation, mirroring all tests from the badger implementation to ensure feature parity.

## Test Files

- **testmain_test.go** - Test configuration (logging, setup)
- **helpers_test.go** - Helper functions for test database setup/teardown
- **save-event_test.go** - Event storage tests
- **query-events_test.go** - Event query tests

## Quick Start

### 1. Start Dgraph Server

```bash
# From project root
./scripts/dgraph-start.sh

# Verify it's running
curl http://localhost:8080/health
```

### 2. Run Tests

```bash
# Run all dgraph tests
./scripts/test-dgraph.sh

# Or run manually
export ORLY_DGRAPH_URL=localhost:9080
CGO_ENABLED=0 go test -v ./pkg/dgraph/...

# Run specific test
CGO_ENABLED=0 go test -v -run TestSaveEvents ./pkg/dgraph
```

## Test Coverage

### Event Storage Tests (`save-event_test.go`)

✅ **TestSaveEvents**
- Loads ~100 events from examples.Cache
- Saves all events chronologically
- Verifies no errors during save
- Reports performance metrics

✅ **TestDeletionEventWithETagRejection**
- Creates a regular event
- Attempts to save deletion event with e-tag
- Verifies deletion events with e-tags are rejected

✅ **TestSaveExistingEvent**
- Saves an event
- Attempts to save same event again
- Verifies duplicate events are rejected

### Event Query Tests (`query-events_test.go`)

✅ **TestQueryEventsByID**
- Queries event by exact ID match
- Verifies single result returned
- Verifies correct event retrieved

✅ **TestQueryEventsByKind**
- Queries events by kind (e.g., kind 1)
- Verifies all results have correct kind
- Tests filtering logic

✅ **TestQueryEventsByAuthor**
- Queries events by author pubkey
- Verifies all results from correct author
- Tests author filtering

✅ **TestReplaceableEventsAndDeletion**
- Creates replaceable event (kind 0)
- Creates newer version
- Verifies only newer version returned in general queries
- Creates deletion event
- Verifies deleted event not returned
- Tests replaceable event logic and deletion

✅ **TestParameterizedReplaceableEventsAndDeletion**
- Creates parameterized replaceable event (kind 30000+)
- Adds d-tag
- Creates deletion event with e-tag
- Verifies deleted event not returned
- Tests parameterized replaceable logic

✅ **TestQueryEventsByTimeRange**
- Queries events by since/until timestamps
- Verifies all results within time range
- Tests temporal filtering

✅ **TestQueryEventsByTag**
- Finds event with tags
- Queries by tag key/value
- Verifies all results have the tag
- Tests tag filtering logic

✅ **TestCountEvents**
- Counts all events
- Counts events by kind filter
- Verifies correct counts returned
- Tests counting functionality

## Test Helpers

### setupTestDB(t *testing.T)

Creates a test dgraph database:

1. **Checks dgraph availability** - Skips test if server not running
2. **Creates temp directory** - For metadata storage
3. **Initializes dgraph client** - Connects to server
4. **Drops all data** - Starts with clean slate
5. **Loads test events** - From examples.Cache (~100 events)
6. **Sorts chronologically** - Ensures addressable events processed in order
7. **Saves all events** - Populates test database

**Returns:** `(*D, []*event.E, context.Context, context.CancelFunc, string)`

### cleanupTestDB(t, db, cancel, tempDir)

Cleans up after tests:
- Closes database connection
- Cancels context
- Removes temp directory

### skipIfDgraphNotAvailable(t *testing.T)

Checks if dgraph is running and skips test if not available.

## Running Tests

### Prerequisites

1. **Dgraph Server** - Must be running before tests
2. **Go 1.21+** - For running tests
3. **CGO_ENABLED=0** - For pure Go build

### Test Execution

#### All Tests

```bash
./scripts/test-dgraph.sh
```

#### Specific Test File

```bash
CGO_ENABLED=0 go test -v ./pkg/dgraph -run TestSaveEvents
```

#### With Logging

```bash
export TEST_LOG=1
CGO_ENABLED=0 go test -v ./pkg/dgraph/...
```

#### With Timeout

```bash
CGO_ENABLED=0 go test -v -timeout 10m ./pkg/dgraph/...
```

### Integration Testing

Run tests + relay-tester:

```bash
./scripts/test-dgraph.sh --relay-tester
```

This will:
1. Run all dgraph package tests
2. Start ORLY with dgraph backend
3. Run relay-tester against ORLY
4. Report results

## Test Data

Tests use `pkg/encoders/event/examples.Cache` which contains:
- ~100 real Nostr events
- Text notes (kind 1)
- Profile metadata (kind 0)
- Various other kinds
- Events with tags, references, mentions
- Multiple authors and timestamps

This ensures tests cover realistic scenarios.

## Debugging Tests

### View Test Output

```bash
CGO_ENABLED=0 go test -v ./pkg/dgraph/... 2>&1 | tee test-output.log
```

### Check Dgraph State

```bash
# View data via Ratel UI
open http://localhost:8000

# Query via HTTP
curl -X POST localhost:8080/query -d '{
  events(func: type(Event), first: 10) {
    uid
    event.id
    event.kind
    event.created_at
  }
}'
```

### Enable Dgraph Logging

```bash
docker logs dgraph-orly-test -f
```

## Test Failures

### "Dgraph server not available"

**Cause:** Dgraph is not running

**Fix:**
```bash
./scripts/dgraph-start.sh
```

### Connection Timeouts

**Cause:** Dgraph server overloaded or network issues

**Fix:**
- Increase test timeout: `go test -timeout 20m`
- Check dgraph resources: `docker stats dgraph-orly-test`
- Restart dgraph: `docker restart dgraph-orly-test`

### Schema Errors

**Cause:** Schema conflicts or version mismatch

**Fix:**
- Drop all data: Tests call `dropAll()` automatically
- Check dgraph version: `docker exec dgraph-orly-test dgraph version`

### Test Hangs

**Cause:** Deadlock or infinite loop

**Fix:**
- Send SIGQUIT: `kill -QUIT <test-pid>`
- View goroutine dump
- Check dgraph logs

## Continuous Integration

### GitHub Actions Example

```yaml
name: Dgraph Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      dgraph:
        image: dgraph/standalone:latest
        ports:
          - 8080:8080
          - 9080:9080
        options: >-
          --health-cmd "curl -f http://localhost:8080/health"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run dgraph tests
        env:
          ORLY_DGRAPH_URL: localhost:9080
        run: |
          CGO_ENABLED=0 go test -v -timeout 10m ./pkg/dgraph/...
```

## Performance Benchmarks

Compare with badger:

```bash
# Badger benchmarks
go test -bench=. -benchmem ./pkg/database/...

# Dgraph benchmarks
go test -bench=. -benchmem ./pkg/dgraph/...
```

## Related Documentation

- [Main Testing Guide](../../scripts/DGRAPH_TESTING.md)
- [Implementation Status](../../DGRAPH_IMPLEMENTATION_STATUS.md)
- [Package README](README.md)

## Contributing

When adding new tests:

1. **Mirror badger tests** - Ensure feature parity
2. **Use test helpers** - setupTestDB() and cleanupTestDB()
3. **Skip if unavailable** - Call skipIfDgraphNotAvailable(t)
4. **Clean up resources** - Always defer cleanupTestDB()
5. **Test chronologically** - Sort events by timestamp for addressable events
6. **Verify behavior** - Don't just check for no errors, verify correctness
