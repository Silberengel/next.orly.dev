# ORLY Scripts Directory

This directory contains automation scripts for building, testing, and deploying ORLY.

## Quick Reference

### Dgraph Integration Testing

```bash
# Local testing (requires dgraph server)
./dgraph-start.sh           # Start dgraph server
./test-dgraph.sh            # Run dgraph package tests
./test-dgraph.sh --relay-tester  # Run tests + relay-tester

# Docker testing (containers for everything)
./docker-build.sh           # Build ORLY docker image
./test-docker.sh            # Run integration tests in containers
./test-docker.sh --relay-tester --keep-running  # Full test, keep running
```

### Build & Deploy

```bash
./build-all-platforms.sh    # Build for multiple platforms
./deploy.sh                 # Deploy to systemd
./update-embedded-web.sh    # Build and embed web UI
```

## Script Descriptions

### Dgraph Testing Scripts

#### dgraph-start.sh
Starts dgraph server using docker-compose for local testing.

**Usage:**
```bash
./dgraph-start.sh
```

**What it does:**
- Checks if dgraph is already running
- Starts dgraph via docker-compose
- Waits for health check
- Shows endpoints and commands

#### dgraph-docker-compose.yml
Docker Compose configuration for standalone dgraph server.

**Ports:**
- 8080: HTTP API
- 9080: gRPC (ORLY connects here)
- 8000: Ratel UI

#### test-dgraph.sh
Runs dgraph package tests against a running dgraph server.

**Usage:**
```bash
./test-dgraph.sh                    # Just tests
./test-dgraph.sh --relay-tester     # Tests + relay-tester
```

**Requirements:**
- Dgraph server running at ORLY_DGRAPH_URL (default: localhost:9080)
- Go 1.21+

### Docker Integration Scripts

#### docker-build.sh
Builds Docker images for ORLY and optionally relay-tester.

**Usage:**
```bash
./docker-build.sh                   # ORLY only
./docker-build.sh --with-tester     # ORLY + relay-tester
```

**Output:**
- orly:latest
- orly-relay-tester:latest (if --with-tester)

#### docker-compose-test.yml
Full-stack docker-compose with dgraph, ORLY, and relay-tester.

**Services:**
- dgraph: Database backend
- orly: Relay with dgraph backend
- relay-tester: Protocol tests (optional, profile: test)

**Features:**
- Health checks for all services
- Dependency management (ORLY waits for dgraph)
- Custom network with DNS
- Persistent volumes

#### test-docker.sh
Comprehensive integration testing in Docker containers.

**Usage:**
```bash
./test-docker.sh                              # Basic test
./test-docker.sh --relay-tester               # Run relay-tester
./test-docker.sh --keep-running               # Keep containers running
./test-docker.sh --skip-build                 # Use existing images
./test-docker.sh --relay-tester --keep-running  # Full test + keep running
```

**What it does:**
1. Stops any existing containers
2. Optionally rebuilds images
3. Starts dgraph and waits for health
4. Starts ORLY and waits for health
5. Verifies connectivity
6. Optionally runs relay-tester
7. Shows status and endpoints
8. Cleanup (unless --keep-running)

### Build Scripts

#### build-all-platforms.sh
Cross-compiles ORLY for multiple platforms.

**Platforms:**
- linux/amd64
- linux/arm64
- darwin/amd64
- darwin/arm64

**Output:** `dist/` directory with platform-specific binaries

#### update-embedded-web.sh
Builds the Svelte web UI and embeds it in ORLY binary.

**Steps:**
1. Builds web UI with bun
2. Generates embedded assets
3. Rebuilds ORLY with embedded UI

### Deployment Scripts

#### deploy.sh
Automated deployment with systemd service.

**What it does:**
1. Installs Go if needed
2. Builds ORLY with embedded web UI
3. Installs to ~/.local/bin/orly
4. Creates systemd service
5. Enables and starts service
6. Sets up port binding capabilities

### Test Scripts

#### test.sh
Runs all Go tests in the project.

**Usage:**
```bash
./test.sh          # All tests
TEST_LOG=1 ./test.sh  # With logging
```

## Environment Variables

### Common Variables

```bash
# Dgraph
export ORLY_DGRAPH_URL=localhost:9080    # Dgraph endpoint
export ORLY_DB_TYPE=dgraph               # Use dgraph backend

# Logging
export ORLY_LOG_LEVEL=debug              # Log verbosity
export TEST_LOG=1                        # Enable test logging

# Server
export ORLY_PORT=3334                    # HTTP/WebSocket port
export ORLY_LISTEN=0.0.0.0               # Listen address

# Data
export ORLY_DATA_DIR=/path/to/data       # Data directory
```

### Script-Specific Variables

```bash
# Docker scripts
export SKIP_BUILD=true                   # Skip image rebuild
export KEEP_RUNNING=true                 # Don't cleanup containers

# Dgraph scripts
export DGRAPH_VERSION=latest             # Dgraph image tag
```

## File Organization

```
scripts/
├── README.md                    # This file
├── DGRAPH_TESTING.md           # Dgraph testing guide
├── DOCKER_TESTING.md           # Docker testing guide
│
├── dgraph-start.sh             # Start dgraph server
├── dgraph-docker-compose.yml   # Dgraph docker config
├── test-dgraph.sh              # Run dgraph tests
│
├── docker-build.sh             # Build docker images
├── docker-compose-test.yml     # Full stack docker config
├── test-docker.sh              # Run docker integration tests
│
├── build-all-platforms.sh      # Cross-compile
├── deploy.sh                   # Deploy to systemd
├── update-embedded-web.sh      # Build web UI
└── test.sh                     # Run Go tests
```

## Workflows

### Local Development with Dgraph

```bash
# 1. Start dgraph
./scripts/dgraph-start.sh

# 2. Run ORLY locally with dgraph
export ORLY_DB_TYPE=dgraph
export ORLY_DGRAPH_URL=localhost:9080
./orly

# 3. Test changes
go run cmd/relay-tester/main.go -url ws://localhost:3334

# 4. Run unit tests
./scripts/test-dgraph.sh
```

### Docker Development

```bash
# 1. Make changes
vim pkg/dgraph/save-event.go

# 2. Build and test in containers
./scripts/test-docker.sh --relay-tester --keep-running

# 3. Make more changes

# 4. Rebuild just ORLY
cd scripts
docker-compose -f docker-compose-test.yml up -d --build orly

# 5. View logs
docker logs orly-relay -f

# 6. Stop when done
docker-compose -f docker-compose-test.yml down
```

### CI/CD Testing

```bash
# Quick test (no containers)
./scripts/test.sh

# Full integration test
./scripts/test-docker.sh --relay-tester

# Build for deployment
./scripts/build-all-platforms.sh
```

### Production Deployment

```bash
# Deploy with systemd
./scripts/deploy.sh

# Check status
systemctl status orly

# View logs
journalctl -u orly -f

# Update
./scripts/deploy.sh  # Rebuilds and restarts
```

## Troubleshooting

### Dgraph Not Available

```bash
# Check if running
docker ps | grep dgraph

# Start it
./scripts/dgraph-start.sh

# Check logs
docker logs dgraph-orly-test -f
```

### Port Conflicts

```bash
# Find what's using port 3334
lsof -i :3334
netstat -tlnp | grep 3334

# Kill process
kill $(lsof -t -i :3334)

# Or use different port
export ORLY_PORT=3335
```

### Docker Build Failures

```bash
# Clear docker cache
docker builder prune

# Rebuild from scratch
docker build --no-cache -t orly:latest -f Dockerfile .

# Check Dockerfile syntax
docker build --dry-run -f Dockerfile .
```

### Permission Issues

```bash
# Fix script permissions
chmod +x scripts/*.sh

# Fix docker socket
sudo usermod -aG docker $USER
newgrp docker
```

## Best Practices

1. **Always use scripts from project root**
   ```bash
   ./scripts/test-docker.sh  # Good
   cd scripts && ./test-docker.sh  # May have path issues
   ```

2. **Check prerequisites before running**
   ```bash
   # Check docker
   docker --version
   docker-compose --version

   # Check dgraph
   curl http://localhost:9080/health
   ```

3. **Clean up after testing**
   ```bash
   # Stop containers
   cd scripts && docker-compose -f docker-compose-test.yml down

   # Remove volumes if needed
   docker-compose -f docker-compose-test.yml down -v
   ```

4. **Use --keep-running for debugging**
   ```bash
   ./scripts/test-docker.sh --keep-running
   # Inspect, debug, make changes
   docker-compose -f scripts/docker-compose-test.yml down
   ```

5. **Check logs on failures**
   ```bash
   # Container logs
   docker logs orly-relay --tail 100

   # Test output
   ./scripts/test-dgraph.sh 2>&1 | tee test.log
   ```

## Related Documentation

- [Dgraph Testing Guide](DGRAPH_TESTING.md)
- [Docker Testing Guide](DOCKER_TESTING.md)
- [Package Tests](../pkg/dgraph/TESTING.md)
- [Main Implementation Status](../DGRAPH_IMPLEMENTATION_STATUS.md)

## Contributing

When adding new scripts:

1. **Add executable permission**
   ```bash
   chmod +x scripts/new-script.sh
   ```

2. **Use bash strict mode**
   ```bash
   #!/bin/bash
   set -e  # Exit on error
   ```

3. **Add help text**
   ```bash
   if [ "$1" == "--help" ]; then
       echo "Usage: $0 [options]"
       exit 0
   fi
   ```

4. **Document in this README**
   - Add to appropriate section
   - Include usage examples
   - Note any requirements

5. **Test on fresh system**
   ```bash
   # Use Docker to test
   docker run --rm -v $(pwd):/app -w /app ubuntu:latest ./scripts/new-script.sh
   ```
