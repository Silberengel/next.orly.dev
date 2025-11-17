# Dgraph Database Implementation for ORLY

This package provides a Dgraph-based implementation of the ORLY database interface, enabling graph-based storage for Nostr events with powerful relationship querying capabilities.

## Status: Step 1 Complete ✅

**Current State:** Dgraph server integration is complete and functional
**Next Step:** DQL query/mutation implementation in save-event.go and query-events.go

## Architecture

### Client-Server Model

The implementation uses a **client-server architecture**:

```
┌─────────────────────────────────────────────┐
│            ORLY Relay Process               │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │    Dgraph Client (pkg/dgraph)      │    │
│  │  - dgo library (gRPC)              │    │
│  │  - Schema management               │────┼───► Dgraph Server
│  │  - Query/Mutate methods            │    │    (localhost:9080)
│  └────────────────────────────────────┘    │    - Event graph
│                                             │    - Authors, tags
│  ┌────────────────────────────────────┐    │    - Relationships
│  │   Badger Metadata Store            │    │
│  │  - Markers (key-value)             │    │
│  │  - Serial counters                 │    │
│  │  - Relay identity                  │    │
│  └────────────────────────────────────┘    │
└─────────────────────────────────────────────┘
```

### Dual Storage Strategy

1. **Dgraph** (Graph Database)
   - Nostr events and their content
   - Author relationships
   - Tag relationships
   - Event references and mentions
   - Optimized for graph traversals and complex queries

2. **Badger** (Key-Value Store)
   - Metadata markers
   - Serial number counters
   - Relay identity keys
   - Fast key-value operations

## Setup

### 1. Start Dgraph Server

Using Docker (recommended):

```bash
docker run -d \
  --name dgraph \
  -p 8080:8080 \
  -p 9080:9080 \
  -p 8000:8000 \
  -v ~/dgraph:/dgraph \
  dgraph/standalone:latest
```

### 2. Configure ORLY

```bash
export ORLY_DB_TYPE=dgraph
export ORLY_DGRAPH_URL=localhost:9080  # Optional, this is the default
```

### 3. Run ORLY

```bash
./orly
```

On startup, ORLY will:
1. Connect to dgraph server via gRPC
2. Apply the Nostr schema automatically
3. Initialize badger metadata store
4. Initialize serial number counter
5. Start accepting events

## Schema

The Nostr schema defines the following types:

### Event Nodes
```dql
type Event {
  event.id          # Event ID (string, indexed)
  event.serial      # Sequential number (int, indexed)
  event.kind        # Event kind (int, indexed)
  event.created_at  # Timestamp (int, indexed)
  event.content     # Event content (string)
  event.sig         # Signature (string, indexed)
  event.pubkey      # Author pubkey (string, indexed)
  event.authored_by # -> Author (uid)
  event.references  # -> Events (uid list)
  event.mentions    # -> Events (uid list)
  event.tagged_with # -> Tags (uid list)
}
```

### Author Nodes
```dql
type Author {
  author.pubkey     # Pubkey (string, indexed, unique)
  author.events     # -> Events (uid list, reverse)
}
```

### Tag Nodes
```dql
type Tag {
  tag.type          # Tag type (string, indexed)
  tag.value         # Tag value (string, indexed + fulltext)
  tag.events        # -> Events (uid list, reverse)
}
```

### Marker Nodes (Metadata)
```dql
type Marker {
  marker.key        # Key (string, indexed, unique)
  marker.value      # Value (string)
}
```

## Configuration

### Environment Variables

- `ORLY_DB_TYPE=dgraph` - Enable dgraph database (default: badger)
- `ORLY_DGRAPH_URL=host:port` - Dgraph gRPC endpoint (default: localhost:9080)
- `ORLY_DATA_DIR=/path` - Data directory for metadata storage

### Connection Details

The dgraph client uses **insecure gRPC** by default for local development. For production deployments:

1. Set up TLS certificates for dgraph
2. Modify `pkg/dgraph/dgraph.go` to use `grpc.WithTransportCredentials()` with your certs

## Implementation Details

### Files

- `dgraph.go` - Main implementation, initialization, lifecycle
- `schema.go` - Schema definition and application
- `save-event.go` - Event storage (TODO: update to use Mutate)
- `query-events.go` - Event queries (TODO: update to parse DQL responses)
- `fetch-event.go` - Event retrieval methods
- `delete.go` - Event deletion
- `markers.go` - Key-value metadata storage (uses badger)
- `serial.go` - Serial number generation (uses badger)
- `subscriptions.go` - Subscription/payment tracking (uses markers)
- `nip43.go` - NIP-43 invite system (uses markers)
- `import-export.go` - Import/export operations
- `logger.go` - Logging adapter

### Key Methods

#### Initialization
```go
d, err := dgraph.New(ctx, cancel, dataDir, logLevel)
```

#### Querying (DQL)
```go
resp, err := d.Query(ctx, dqlQuery)
```

#### Mutations (RDF N-Quads)
```go
mutation := &api.Mutation{SetNquads: []byte(nquads)}
resp, err := d.Mutate(ctx, mutation)
```

## Development Status

### ✅ Step 1: Dgraph Server Integration (COMPLETE)

- [x] dgo client library integration
- [x] gRPC connection to external dgraph
- [x] Schema definition and auto-application
- [x] Query() and Mutate() method stubs
- [x] ORLY_DGRAPH_URL configuration
- [x] Dual-storage architecture
- [x] Proper lifecycle management

### 📝 Step 2: DQL Implementation (NEXT)

Priority tasks:

1. **save-event.go** - Replace RDF string building with actual Mutate() calls
2. **query-events.go** - Parse actual JSON responses from Query()
3. **fetch-event.go** - Implement DQL queries for event retrieval
4. **delete.go** - Implement deletion mutations

### 📝 Step 3: Testing (FUTURE)

- Integration testing with relay-tester
- Performance benchmarks vs badger
- Memory profiling
- Production deployment testing

## Troubleshooting

### Connection Refused

```
failed to connect to dgraph at localhost:9080: connection refused
```

**Solution:** Ensure dgraph server is running:
```bash
docker ps | grep dgraph
docker logs dgraph
```

### Schema Application Failed

```
failed to apply schema: ...
```

**Solution:** Check dgraph server logs and ensure no schema conflicts:
```bash
docker logs dgraph
```

### Binary Not Finding libsecp256k1.so

This is unrelated to dgraph. Ensure:
```bash
export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$(pwd)/pkg/crypto/p8k"
```

## Performance Considerations

### When to Use Dgraph

**Good fit:**
- Complex graph queries (follows-of-follows, social graphs)
- Full-text search requirements
- Advanced filtering and aggregations
- Multi-hop relationship traversals

**Not ideal for:**
- Simple key-value lookups (badger is faster)
- Very high write throughput (badger has lower latency)
- Single-node deployments with simple queries

### Optimization Tips

1. **Indexing**: Ensure frequently queried fields have appropriate indexes
2. **Pagination**: Use offset/limit in DQL queries for large result sets
3. **Caching**: Consider adding an LRU cache for hot events
4. **Schema Design**: Use reverse edges for efficient relationship traversal

## Resources

- [Dgraph Documentation](https://dgraph.io/docs/)
- [DQL Query Language](https://dgraph.io/docs/query-language/)
- [dgo Client Library](https://github.com/dgraph-io/dgo)
- [ORLY Implementation Status](../../DGRAPH_IMPLEMENTATION_STATUS.md)

## Contributing

When working on dgraph implementation:

1. Test changes against a local dgraph instance
2. Update schema.go if adding new node types or predicates
3. Ensure dual-storage strategy is maintained (dgraph for events, badger for metadata)
4. Add integration tests for new features
5. Update DGRAPH_IMPLEMENTATION_STATUS.md with progress
