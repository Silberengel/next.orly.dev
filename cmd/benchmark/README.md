# Nostr Relay Benchmark Suite

A comprehensive benchmarking system for testing and comparing the performance of multiple Nostr relay implementations, including:

- **next.orly.dev** (this repository) - BadgerDB-based relay
- **Khatru** - SQLite and Badger variants
- **Relayer** - Basic example implementation
- **Strfry** - C++ LMDB-based relay
- **nostr-rs-relay** - Rust-based relay with SQLite

## Features

### Benchmark Tests

1. **Peak Throughput Test**
   - Tests maximum event ingestion rate
   - Concurrent workers pushing events as fast as possible
   - Measures events/second, latency distribution, success rate

2. **Burst Pattern Test**
   - Simulates real-world traffic patterns
   - Alternating high-activity bursts and quiet periods
   - Tests relay behavior under varying loads

3. **Mixed Read/Write Test**
   - Concurrent read and write operations
   - Tests query performance while events are being ingested
   - Measures combined throughput and latency

### Performance Metrics

- **Throughput**: Events processed per second
- **Latency**: Average, P95, and P99 response times
- **Success Rate**: Percentage of successful operations
- **Memory Usage**: Peak memory consumption during tests
- **Error Analysis**: Detailed error reporting and categorization

### Reporting

- Individual relay reports with detailed metrics
- Aggregate comparison report across all relays
- Comparison tables for easy performance analysis
- Timestamped results for tracking improvements over time

## Quick Start

### 1. Setup External Relays

Run the setup script to download and configure all external relay repositories:

```bash
cd cmd/benchmark
./setup-external-relays.sh
```

This will:
- Clone all external relay repositories
- Create Docker configurations for each relay
- Set up configuration files
- Create data and report directories

### 2. Run Benchmarks

Start all relays and run the benchmark suite:

```bash
docker compose up --build
```

The system will:
- Build and start all relay containers
- Wait for all relays to become healthy
- Run benchmarks against each relay sequentially
- Generate individual and aggregate reports

### 3. View Results

Results are stored in the `reports/` directory with timestamps:

```bash
# View the aggregate report
cat reports/run_YYYYMMDD_HHMMSS/aggregate_report.txt

# View individual relay results
ls reports/run_YYYYMMDD_HHMMSS/
```

## Architecture

### Docker Compose Services

| Service | Port | Description |
|---------|------|-------------|
| next-orly | 8001 | This repository's BadgerDB relay |
| khatru-sqlite | 8002 | Khatru with SQLite backend |
| khatru-badger | 8003 | Khatru with Badger backend |
| relayer-basic | 8004 | Basic relayer example |
| strfry | 8005 | Strfry C++ LMDB relay |
| nostr-rs-relay | 8006 | Rust SQLite relay |
| benchmark-runner | - | Orchestrates tests and aggregates results |

### File Structure

```
cmd/benchmark/
├── main.go                      # Benchmark tool implementation
├── docker-compose.yml           # Service orchestration
├── setup-external-relays.sh     # Repository setup script
├── benchmark-runner.sh          # Test orchestration script
├── Dockerfile.next-orly         # This repo's relay container
├── Dockerfile.benchmark         # Benchmark runner container
├── Dockerfile.khatru-sqlite     # Khatru SQLite variant
├── Dockerfile.khatru-badger     # Khatru Badger variant
├── Dockerfile.relayer-basic     # Relayer basic example
├── Dockerfile.strfry            # Strfry relay
├── Dockerfile.nostr-rs-relay    # Rust relay
├── configs/
│   ├── strfry.conf             # Strfry configuration
│   └── config.toml             # nostr-rs-relay configuration
├── external/                   # External relay repositories
├── data/                       # Persistent data for each relay
└── reports/                    # Benchmark results
```

## Configuration

### Environment Variables

The benchmark can be configured via environment variables in `docker-compose.yml`:

```yaml
environment:
  - BENCHMARK_EVENTS=10000      # Number of events per test
  - BENCHMARK_WORKERS=8         # Concurrent workers
  - BENCHMARK_DURATION=60s      # Test duration
  - BENCHMARK_TARGETS=...       # Relay endpoints to test
```

### Custom Configuration

1. **Modify test parameters**: Edit environment variables in `docker-compose.yml`
2. **Add new relays**: 
   - Add service to `docker-compose.yml`
   - Create appropriate Dockerfile
   - Update `BENCHMARK_TARGETS` environment variable
3. **Adjust relay configs**: Edit files in `configs/` directory

## Manual Usage

### Run Individual Relay

```bash
# Build and run a specific relay
docker-compose up next-orly

# Run benchmark against specific endpoint
./benchmark -datadir=/tmp/test -events=1000 -workers=4
```

### Run Benchmark Tool Directly

```bash
# Build the benchmark tool
go build -o benchmark main.go

# Run with custom parameters
./benchmark \
  -datadir=/tmp/benchmark_db \
  -events=5000 \
  -workers=4 \
  -duration=30s
```

## Benchmark Results Interpretation

### Peak Throughput Test
- **High events/sec**: Good write performance
- **Low latency**: Efficient event processing
- **High success rate**: Stable under load

### Burst Pattern Test  
- **Consistent performance**: Good handling of variable loads
- **Low P95/P99 latency**: Predictable response times
- **No errors during bursts**: Robust queuing/buffering

### Mixed Read/Write Test
- **Balanced throughput**: Good concurrent operation handling
- **Low read latency**: Efficient query processing
- **Stable write performance**: Queries don't significantly impact writes

## Development

### Adding New Tests

1. Extend the `Benchmark` struct in `main.go`
2. Add new test method following existing patterns
3. Update `main()` function to call new test
4. Update result aggregation in `benchmark-runner.sh`

### Modifying Relay Configurations

Each relay's Dockerfile and configuration can be customized:
- **Resource limits**: Adjust memory/CPU limits in docker-compose.yml
- **Database settings**: Modify configuration files in `configs/`
- **Network settings**: Update port mappings and health checks

### Debugging

```bash
# View logs for specific relay
docker-compose logs next-orly

# Run benchmark with debug output
docker-compose up --build benchmark-runner

# Check individual container health
docker-compose ps
```

## Troubleshooting

### Common Issues

1. **Relay fails to start**: Check logs with `docker-compose logs <service>`
2. **Connection refused**: Ensure relay health checks are passing
3. **Build failures**: Verify external repositories were cloned correctly
4. **Permission errors**: Ensure setup script is executable

### Performance Issues

- **Low throughput**: Check resource limits and concurrent worker count
- **High memory usage**: Monitor container resource consumption
- **Network bottlenecks**: Test on different host configurations

### Reset Environment

```bash
# Clean up everything
docker-compose down -v
docker system prune -f
rm -rf external/ data/ reports/

# Start fresh
./setup-external-relays.sh
docker-compose up --build
```

## Contributing

To add support for new relay implementations:

1. Create appropriate Dockerfile following existing patterns
2. Add service definition to `docker-compose.yml`
3. Update `BENCHMARK_TARGETS` environment variable
4. Test the new relay integration
5. Update documentation

## License

This benchmark suite is part of the next.orly.dev project and follows the same licensing terms.