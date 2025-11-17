# Dgraph Integration Testing

This directory contains scripts and configuration for testing the ORLY dgraph integration.

## Quick Start

### 1. Start Dgraph Server

```bash
# Using the convenience script
./scripts/dgraph-start.sh

# Or manually with docker-compose
cd scripts
docker-compose -f dgraph-docker-compose.yml up -d

# Or directly with docker
docker run -d \
  -p 8080:8080 \
  -p 9080:9080 \
  -p 8000:8000 \
  --name dgraph-orly \
  dgraph/standalone:latest
```

### 2. Run Dgraph Tests

```bash
# Run all dgraph package tests
./scripts/test-dgraph.sh

# Run tests with relay-tester
./scripts/test-dgraph.sh --relay-tester
```

### 3. Manual Testing

```bash
# Start ORLY with dgraph backend
export ORLY_DB_TYPE=dgraph
export ORLY_DGRAPH_URL=localhost:9080
./orly

# In another terminal, run relay-tester
go run cmd/relay-tester/main.go -url ws://localhost:3334
```

## Test Files

The dgraph package includes comprehensive tests:

- **testmain_test.go** - Test configuration and logging setup
- **helpers_test.go** - Helper functions for test setup/teardown
- **save-event_test.go** - Event storage tests
- **query-events_test.go** - Event query tests

All tests mirror the existing badger tests to ensure feature parity.

## Test Coverage

The dgraph tests cover:

✅ **Event Storage**
- Saving events from examples.Cache
- Duplicate event rejection
- Deletion event validation

✅ **Event Queries**
- Query by ID
- Query by kind
- Query by author
- Query by time range
- Query by tags
- Event counting

✅ **Advanced Features**
- Replaceable events (kind 0)
- Parameterized replaceable events (kind 30000+)
- Event deletion (kind 5)
- Event replacement logic

## Requirements

### Dgraph Server

The tests require a running dgraph server. Tests will be skipped if dgraph is not available.

**Endpoints:**
- gRPC: `localhost:9080` (required for ORLY)
- HTTP: `localhost:8080` (for health checks)
- Ratel UI: `localhost:8000` (optional, for debugging)

**Custom Endpoint:**
```bash
export ORLY_DGRAPH_URL=remote.server.com:9080
./scripts/test-dgraph.sh
```

### Docker

The docker-compose setup requires:
- Docker Engine 20.10+
- Docker Compose 1.29+ (or docker-compose plugin)

## Test Workflow

### Running Tests Locally

```bash
# 1. Start dgraph
./scripts/dgraph-start.sh

# 2. Run tests
./scripts/test-dgraph.sh

# 3. Clean up when done
cd scripts && docker-compose -f dgraph-docker-compose.yml down
```

### CI/CD Integration

For CI pipelines, use the docker-compose file:

```yaml
# Example GitHub Actions workflow
services:
  dgraph:
    image: dgraph/standalone:latest
    ports:
      - 8080:8080
      - 9080:9080

steps:
  - name: Run dgraph tests
    run: |
      export ORLY_DGRAPH_URL=localhost:9080
      CGO_ENABLED=0 go test -v ./pkg/dgraph/...
```

## Debugging

### View Dgraph Logs

```bash
docker logs dgraph-orly-test -f
```

### Access Ratel UI

Open http://localhost:8000 in your browser to:
- View schema
- Run DQL queries
- Inspect data

### Enable Test Logging

```bash
export TEST_LOG=1
./scripts/test-dgraph.sh
```

### Manual DQL Queries

```bash
# Using curl
curl -X POST localhost:8080/query -d '{
  q(func: type(Event)) {
    uid
    event.id
    event.kind
    event.created_at
  }
}'

# Using grpcurl (if installed)
grpcurl -plaintext -d '{
  "query": "{ q(func: type(Event)) { uid event.id } }"
}' localhost:9080 api.Dgraph/Query
```

## Troubleshooting

### Tests Skip with "Dgraph server not available"

**Solution:** Ensure dgraph is running:
```bash
docker ps | grep dgraph
./scripts/dgraph-start.sh
```

### Connection Refused Errors

**Symptoms:**
```
failed to connect to dgraph at localhost:9080: connection refused
```

**Solutions:**
1. Check dgraph is running: `docker ps`
2. Check port mapping: `docker port dgraph-orly-test`
3. Check firewall rules
4. Verify ORLY_DGRAPH_URL is correct

### Schema Application Failed

**Symptoms:**
```
failed to apply schema: ...
```

**Solutions:**
1. Check dgraph logs: `docker logs dgraph-orly-test`
2. Drop all data and retry: Use `dropAll` in test setup
3. Verify dgraph version compatibility

### Tests Timeout

**Symptoms:**
```
panic: test timed out after 10m
```

**Solutions:**
1. Increase timeout: `go test -timeout 20m ./pkg/dgraph/...`
2. Check dgraph performance: May need more resources
3. Reduce test dataset size

## Performance Benchmarks

Compare dgraph vs badger performance:

```bash
# Run badger benchmarks
go test -bench=. ./pkg/database/...

# Run dgraph benchmarks
go test -bench=. ./pkg/dgraph/...
```

## Test Data

Tests use `pkg/encoders/event/examples.Cache` which contains:
- ~100 real Nostr events
- Various kinds (text notes, metadata, etc.)
- Different authors and timestamps
- Events with tags and relationships

## Cleanup

### Remove Test Data

```bash
# Stop and remove containers
cd scripts
docker-compose -f dgraph-docker-compose.yml down

# Remove volumes
docker volume rm scripts_dgraph-data
```

### Reset Dgraph

```bash
# Drop all data (via test helper)
# The dropAll() function is called in test setup

# Or manually via HTTP
curl -X POST localhost:8080/alter -d '{"drop_all": true}'
```

## Related Documentation

- [Dgraph Implementation Status](../DGRAPH_IMPLEMENTATION_STATUS.md)
- [Package README](../pkg/dgraph/README.md)
- [Dgraph Documentation](https://dgraph.io/docs/)
- [DQL Query Language](https://dgraph.io/docs/query-language/)
