# Dgraph Integration Guide for ORLY Relay

This document outlines how to integrate Dgraph as an embedded graph database within the ORLY Nostr relay, enabling advanced querying capabilities beyond standard Nostr REQ filters.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Embedding Dgraph as a Goroutine](#embedding-dgraph-as-a-goroutine)
4. [Internal Query Interface](#internal-query-interface)
5. [GraphQL Endpoint Setup](#graphql-endpoint-setup)
6. [Schema Design](#schema-design)
7. [Integration Points](#integration-points)
8. [Performance Considerations](#performance-considerations)

## Overview

### What Dgraph Provides

Dgraph is a distributed graph database that can be embedded into Go applications. For ORLY, it offers:

- **Graph Queries**: Traverse relationships between events, authors, and tags
- **GraphQL API**: External access to relay data with complex queries
- **DQL (Dgraph Query Language)**: Internal programmatic queries
- **Real-time Updates**: Live query subscriptions
- **Advanced Filtering**: Complex multi-hop queries impossible with Nostr REQ

### Why Integrate?

Nostr REQ filters are limited to:
- Single-author or tag-based queries
- Time range filters
- Kind filters
- Simple AND/OR combinations

Dgraph enables:
- "Find all events from users followed by my follows" (2-hop social graph)
- "Show threads where Alice replied to Bob who replied to Carol"
- "Find all events tagged with #bitcoin by authors in my Web of Trust"
- Complex graph analytics on social networks

## Architecture

### Dgraph Components

```
┌────────────────────────────────────────────────────────┐
│                    ORLY Relay                          │
│                                                        │
│  ┌──────────────┐         ┌─────────────────────────┐  │
│  │   HTTP API   │◄────────┤   GraphQL Endpoint      │  │
│  │  (existing)  │         │   (new - external)      │  │
│  └──────────────┘         └─────────────────────────┘  │
│         │                           │                  │
│         ▼                           ▼                  │
│  ┌──────────────────────────────────────────────────┐  │
│  │            Event Ingestion Layer                 │  │
│  │  - Save to Badger (existing)                     │  │
│  │  - Sync to Dgraph (new)                          │  │
│  └──────────────────────────────────────────────────┘  │
│         │                           │                  │
│         ▼                           ▼                  │
│  ┌────────────┐            ┌─────────────────┐         │
│  │   Badger   │            │  Dgraph Engine  │         │
│  │  (events)  │            │  (graph index)  │         │
│  └────────────┘            └─────────────────┘         │
│                                     │                  │
│                            ┌────────┴────────┐         │
│                            │                 │         │
│                            ▼                 ▼         │
│                     ┌──────────┐     ┌──────────┐      │
│                     │  Badger  │     │ RaftWAL  │      │
│                     │(postings)│     │  (WAL)   │      │
│                     └──────────┘     └──────────┘      │
└─────────────────────────────────────────────────────────┘
```

### Storage Strategy

**Dual Storage Approach:**

1. **Badger (Primary)**: Continue using existing Badger database for:
   - Fast event retrieval by ID
   - Time-based queries
   - Author-based queries
   - Tag-based queries
   - Kind-based queries

2. **Dgraph (Secondary)**: Use for:
   - Graph relationship queries
   - Complex multi-hop traversals
   - Social graph analytics
   - Web of Trust calculations

**Data Sync**: Events are written to both stores, but Dgraph contains:
- Event nodes (ID, kind, created_at, content)
- Author nodes (pubkey)
- Tag nodes (tag values)
- Relationships (authored_by, tagged_with, replies_to, mentions, etc.)

## Embedding Dgraph as a Goroutine

### Initialization Pattern

Based on dgraph's embedded mode (`worker/embedded.go` and `worker/server_state.go`):

```go
package dgraph

import (
    "context"
    "net"
    "net/http"

    "github.com/dgraph-io/badger/v4"
    "github.com/dgraph-io/dgraph/edgraph"
    "github.com/dgraph-io/dgraph/graphql/admin"
    "github.com/dgraph-io/dgraph/posting"
    "github.com/dgraph-io/dgraph/schema"
    "github.com/dgraph-io/dgraph/worker"
    "github.com/dgraph-io/dgraph/x"
    "github.com/dgraph-io/ristretto/z"
)

// Manager handles the embedded Dgraph instance
type Manager struct {
    ctx           context.Context
    cancel        context.CancelFunc

    // Dgraph components
    pstore        *badger.DB          // Postings store
    walstore      *worker.DiskStorage // Write-ahead log

    // GraphQL servers
    mainServer    admin.IServeGraphQL
    adminServer   admin.IServeGraphQL
    healthStore   *admin.GraphQLHealthStore

    // Lifecycle
    closer        *z.Closer
    serverCloser  *z.Closer
}

// Config holds Dgraph configuration
type Config struct {
    DataDir              string
    PostingDir           string
    WALDir               string

    // Performance tuning
    PostingCacheMB       int64
    MutationsMode        string

    // Network
    GraphQLPort          int
    AdminPort            int

    // Feature flags
    EnableGraphQL        bool
    EnableIntrospection  bool
}

// New creates a new embedded Dgraph manager
func New(ctx context.Context, cfg *Config) (*Manager, error) {
    ctx, cancel := context.WithCancel(ctx)

    m := &Manager{
        ctx:          ctx,
        cancel:       cancel,
        closer:       z.NewCloser(1),
        serverCloser: z.NewCloser(3),
    }

    // Initialize storage
    if err := m.initStorage(cfg); err != nil {
        return nil, err
    }

    // Initialize Dgraph components
    if err := m.initDgraph(cfg); err != nil {
        return nil, err
    }

    // Setup GraphQL endpoints
    if cfg.EnableGraphQL {
        if err := m.setupGraphQL(cfg); err != nil {
            return nil, err
        }
    }

    return m, nil
}

// initStorage opens Badger databases for postings and WAL
func (m *Manager) initStorage(cfg *Config) error {
    // Open postings store (Dgraph's main data)
    opts := badger.DefaultOptions(cfg.PostingDir).
        WithNumVersionsToKeep(math.MaxInt32).
        WithNamespaceOffset(x.NamespaceOffset)

    var err error
    m.pstore, err = badger.OpenManaged(opts)
    if err != nil {
        return fmt.Errorf("failed to open postings store: %w", err)
    }

    // Open WAL store
    m.walstore, err = worker.InitStorage(cfg.WALDir)
    if err != nil {
        m.pstore.Close()
        return fmt.Errorf("failed to open WAL: %w", err)
    }

    return nil
}

// initDgraph initializes Dgraph worker components
func (m *Manager) initDgraph(cfg *Config) error {
    // Initialize server state
    worker.State.Pstore = m.pstore
    worker.State.WALstore = m.walstore
    worker.State.FinishCh = make(chan struct{})

    // Initialize schema and posting layers
    schema.Init(m.pstore)
    posting.Init(m.pstore, cfg.PostingCacheMB, true)
    worker.Init(m.pstore)

    // For embedded/lite mode without Raft
    worker.InitForLite(m.pstore)

    return nil
}

// setupGraphQL initializes GraphQL servers
func (m *Manager) setupGraphQL(cfg *Config) error {
    globalEpoch := make(map[uint64]*uint64)

    // Create GraphQL servers
    m.mainServer, m.adminServer, m.healthStore = admin.NewServers(
        cfg.EnableIntrospection,
        globalEpoch,
        m.serverCloser,
    )

    return nil
}

// Start launches Dgraph in goroutines
func (m *Manager) Start() error {
    // Start worker server (internal gRPC)
    go worker.RunServer(false)

    return nil
}

// Stop gracefully shuts down Dgraph
func (m *Manager) Stop() error {
    m.cancel()

    // Signal shutdown
    m.closer.SignalAndWait()
    m.serverCloser.SignalAndWait()

    // Close databases
    if m.walstore != nil {
        m.walstore.Close()
    }
    if m.pstore != nil {
        m.pstore.Close()
    }

    return nil
}
```

### Integration with ORLY Main

In `app/main.go`:

```go
import (
    "next.orly.dev/pkg/dgraph"
)

type Listener struct {
    // ... existing fields ...

    dgraphManager *dgraph.Manager
}

func (l *Listener) init(ctx context.Context, cfg *config.C) (err error) {
    // ... existing initialization ...

    // Initialize Dgraph if enabled
    if cfg.DgraphEnabled {
        dgraphCfg := &dgraph.Config{
            DataDir:             cfg.DgraphDataDir,
            PostingDir:          filepath.Join(cfg.DgraphDataDir, "p"),
            WALDir:              filepath.Join(cfg.DgraphDataDir, "w"),
            PostingCacheMB:      cfg.DgraphCacheMB,
            EnableGraphQL:       cfg.DgraphGraphQL,
            EnableIntrospection: cfg.DgraphIntrospection,
            GraphQLPort:         cfg.DgraphGraphQLPort,
        }

        l.dgraphManager, err = dgraph.New(ctx, dgraphCfg)
        if err != nil {
            return fmt.Errorf("failed to initialize dgraph: %w", err)
        }

        if err = l.dgraphManager.Start(); err != nil {
            return fmt.Errorf("failed to start dgraph: %w", err)
        }

        log.I.F("dgraph manager started successfully")
    }

    // ... rest of initialization ...
}
```

## Internal Query Interface

### Direct Query Execution

Dgraph provides `edgraph.Server{}.QueryNoGrpc()` for internal queries:

```go
package dgraph

import (
    "context"

    "github.com/dgraph-io/dgo/v230/protos/api"
    "github.com/dgraph-io/dgraph/edgraph"
)

// Query executes a DQL query internally
func (m *Manager) Query(ctx context.Context, query string) (*api.Response, error) {
    server := &edgraph.Server{}

    req := &api.Request{
        Query: query,
    }

    return server.QueryNoGrpc(ctx, req)
}

// Mutate applies a mutation to the graph
func (m *Manager) Mutate(ctx context.Context, mutation *api.Mutation) (*api.Response, error) {
    server := &edgraph.Server{}

    req := &api.Request{
        Mutations: []*api.Mutation{mutation},
        CommitNow: true,
    }

    return server.QueryNoGrpc(ctx, req)
}
```

### Example: Adding Events to Graph

```go
// AddEvent indexes a Nostr event in the graph
func (m *Manager) AddEvent(ctx context.Context, event *event.E) error {
    // Build RDF triples for the event
    nquads := buildEventNQuads(event)

    mutation := &api.Mutation{
        SetNquads: []byte(nquads),
        CommitNow: true,
    }

    _, err := m.Mutate(ctx, mutation)
    return err
}

func buildEventNQuads(event *event.E) string {
    var nquads strings.Builder

    eventID := hex.EncodeToString(event.ID[:])
    authorPubkey := hex.EncodeToString(event.Pubkey)

    // Event node
    nquads.WriteString(fmt.Sprintf("_:%s <dgraph.type> \"Event\" .\n", eventID))
    nquads.WriteString(fmt.Sprintf("_:%s <event.id> %q .\n", eventID, eventID))
    nquads.WriteString(fmt.Sprintf("_:%s <event.kind> %q .\n", eventID, event.Kind))
    nquads.WriteString(fmt.Sprintf("_:%s <event.created_at> %q .\n", eventID, event.CreatedAt))
    nquads.WriteString(fmt.Sprintf("_:%s <event.content> %q .\n", eventID, event.Content))

    // Author relationship
    nquads.WriteString(fmt.Sprintf("_:%s <authored_by> _:%s .\n", eventID, authorPubkey))
    nquads.WriteString(fmt.Sprintf("_:%s <dgraph.type> \"Author\" .\n", authorPubkey))
    nquads.WriteString(fmt.Sprintf("_:%s <author.pubkey> %q .\n", authorPubkey, authorPubkey))

    // Tag relationships
    for _, tag := range event.Tags {
        if len(tag) >= 2 {
            tagType := string(tag[0])
            tagValue := string(tag[1])

            switch tagType {
            case "e": // Event reference
                nquads.WriteString(fmt.Sprintf("_:%s <references> _:%s .\n", eventID, tagValue))
            case "p": // Pubkey mention
                nquads.WriteString(fmt.Sprintf("_:%s <mentions> _:%s .\n", eventID, tagValue))
            case "t": // Hashtag
                tagID := "tag_" + tagValue
                nquads.WriteString(fmt.Sprintf("_:%s <tagged_with> _:%s .\n", eventID, tagID))
                nquads.WriteString(fmt.Sprintf("_:%s <dgraph.type> \"Tag\" .\n", tagID))
                nquads.WriteString(fmt.Sprintf("_:%s <tag.value> %q .\n", tagID, tagValue))
            }
        }
    }

    return nquads.String()
}
```

### Example: Query Social Graph

```go
// FindFollowsOfFollows returns events from 2-hop social network
func (m *Manager) FindFollowsOfFollows(ctx context.Context, pubkey []byte) ([]*event.E, error) {
    pubkeyHex := hex.EncodeToString(pubkey)

    query := fmt.Sprintf(`{
        follows_of_follows(func: eq(author.pubkey, %q)) {
            # My follows (kind 3)
            ~authored_by @filter(eq(event.kind, "3")) {
                # Their follows
                references {
                    # Events from their follows
                    ~authored_by {
                        event.id
                        event.kind
                        event.created_at
                        event.content
                        authored_by {
                            author.pubkey
                        }
                    }
                }
            }
        }
    }`, pubkeyHex)

    resp, err := m.Query(ctx, query)
    if err != nil {
        return nil, err
    }

    // Parse response and convert to Nostr events
    return parseEventsFromDgraphResponse(resp.Json)
}
```

## GraphQL Endpoint Setup

### Exposing GraphQL via HTTP

Add GraphQL handlers to the existing HTTP mux in `app/server.go`:

```go
// setupGraphQLEndpoints adds Dgraph GraphQL endpoints
func (s *Server) setupGraphQLEndpoints() {
    if s.dgraphManager == nil {
        return
    }

    // Main GraphQL endpoint for queries
    s.mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
        // Extract namespace (for multi-tenancy)
        namespace := x.ExtractNamespaceHTTP(r)

        // Lazy load schema
        admin.LazyLoadSchema(namespace)

        // Serve GraphQL
        s.dgraphManager.MainServer().HTTPHandler().ServeHTTP(w, r)
    })

    // Admin endpoint for schema updates
    s.mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
        namespace := x.ExtractNamespaceHTTP(r)
        admin.LazyLoadSchema(namespace)
        s.dgraphManager.AdminServer().HTTPHandler().ServeHTTP(w, r)
    })

    // Health check
    s.mux.HandleFunc("/graphql/health", func(w http.ResponseWriter, r *http.Request) {
        health := s.dgraphManager.HealthStore()
        if health.IsGraphQLReady() {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("GraphQL is ready"))
        } else {
            w.WriteHeader(http.StatusServiceUnavailable)
            w.Write([]byte("GraphQL is not ready"))
        }
    })
}
```

### GraphQL Resolver Integration

The manager needs to expose the GraphQL servers:

```go
// MainServer returns the main GraphQL server
func (m *Manager) MainServer() admin.IServeGraphQL {
    return m.mainServer
}

// AdminServer returns the admin GraphQL server
func (m *Manager) AdminServer() admin.IServeGraphQL {
    return m.adminServer
}

// HealthStore returns the health check store
func (m *Manager) HealthStore() *admin.GraphQLHealthStore {
    return m.healthStore
}
```

## Schema Design

### Dgraph Schema for Nostr Events

```graphql
# Types
type Event {
    id: String! @id @index(exact)
    kind: Int! @index(int)
    created_at: Int! @index(int)
    content: String @index(fulltext)
    sig: String

    # Relationships
    authored_by: Author! @reverse
    references: [Event] @reverse
    mentions: [Author] @reverse
    tagged_with: [Tag] @reverse
    replies_to: Event @reverse
}

type Author {
    pubkey: String! @id @index(exact)

    # Relationships
    events: [Event] @reverse
    follows: [Author] @reverse
    followed_by: [Author] @reverse

    # Computed/cached fields
    follower_count: Int
    following_count: Int
    event_count: Int
}

type Tag {
    value: String! @id @index(exact, term, fulltext)
    type: String @index(exact)

    # Relationships
    events: [Event] @reverse
    usage_count: Int
}

# Indexes for efficient queries
<event.kind>: int @index .
<event.created_at>: int @index .
<event.content>: string @index(fulltext) .
<author.pubkey>: string @index(exact) .
<tag.value>: string @index(exact, term, fulltext) .
```

### Setting the Schema

```go
func (m *Manager) SetSchema(ctx context.Context) error {
    schemaStr := `
type Event {
    event.id: string @index(exact) .
    event.kind: int @index(int) .
    event.created_at: int @index(int) .
    event.content: string @index(fulltext) .
    authored_by: uid @reverse .
    references: [uid] @reverse .
    mentions: [uid] @reverse .
    tagged_with: [uid] @reverse .
}

type Author {
    author.pubkey: string @index(exact) .
}

type Tag {
    tag.value: string @index(exact, term, fulltext) .
}
`

    mutation := &api.Mutation{
        SetNquads: []byte(schemaStr),
        CommitNow: true,
    }

    _, err := m.Mutate(ctx, mutation)
    return err
}
```

## Integration Points

### Event Ingestion Hook

Modify `pkg/database/save-event.go` to sync events to Dgraph:

```go
func (d *D) SaveEvent(ctx context.Context, ev *event.E) (exists bool, err error) {
    // ... existing Badger save logic ...

    // Sync to Dgraph if enabled
    if d.dgraphManager != nil {
        go func() {
            if err := d.dgraphManager.AddEvent(context.Background(), ev); err != nil {
                log.E.F("failed to sync event to dgraph: %v", err)
            }
        }()
    }

    return
}
```

### Query Interface Extension

Add GraphQL query support alongside Nostr REQ:

```go
// app/handle-graphql.go

func (s *Server) handleGraphQLQuery(w http.ResponseWriter, r *http.Request) {
    if s.dgraphManager == nil {
        http.Error(w, "GraphQL not enabled", http.StatusNotImplemented)
        return
    }

    // Read GraphQL query from request
    var req struct {
        Query     string                 `json:"query"`
        Variables map[string]interface{} `json:"variables"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Execute via Dgraph
    gqlReq := &schema.Request{
        Query:     req.Query,
        Variables: req.Variables,
    }

    namespace := x.ExtractNamespaceHTTP(r)
    resp := s.dgraphManager.MainServer().ResolveWithNs(r.Context(), namespace, gqlReq)

    // Return response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}
```

## Performance Considerations

### Memory Usage

- **Dgraph Overhead**: ~500MB-1GB baseline
- **Posting Cache**: Configurable (recommend 25% of available RAM)
- **WAL**: Disk-based, minimal memory impact

### Storage Requirements

- **Badger (Postings)**: ~2-3x event data size (compressed)
- **WAL**: ~1.5x mutation data (compacted periodically)
- **Total**: Estimate 4-5x your Nostr event storage

### Query Performance

- **Graph Traversals**: O(edges) typically sub-100ms for 2-3 hops
- **Full-text Search**: O(log n) with indexes
- **Time-range Queries**: O(log n) with int indexes
- **Complex Joins**: Can be expensive; use pagination

### Optimization Strategies

1. **Selective Indexing**: Only index events that need graph queries (e.g., kinds 1, 3, 6, 7)
2. **Async Writes**: Don't block event saves on Dgraph sync
3. **Read-through Cache**: Query Badger first for simple lookups
4. **Batch Mutations**: Accumulate mutations and apply in batches
5. **Schema Optimization**: Only index fields you'll query
6. **Pagination**: Always use `first:` and `after:` in GraphQL queries

### Monitoring

```go
// Add metrics
var (
    dgraphQueriesTotal = prometheus.NewCounter(...)
    dgraphQueryDuration = prometheus.NewHistogram(...)
    dgraphMutationsTotal = prometheus.NewCounter(...)
    dgraphErrors = prometheus.NewCounter(...)
)

// Wrap queries with instrumentation
func (m *Manager) Query(ctx context.Context, query string) (*api.Response, error) {
    start := time.Now()
    defer func() {
        dgraphQueriesTotal.Inc()
        dgraphQueryDuration.Observe(time.Since(start).Seconds())
    }()

    resp, err := m.query(ctx, query)
    if err != nil {
        dgraphErrors.Inc()
    }
    return resp, err
}
```

## Alternative: Lightweight Graph Library

Given Dgraph's complexity and resource requirements, consider these alternatives:

### cayley (Google's graph database)

```bash
go get github.com/cayleygraph/cayley
```

- Lighter weight (~50MB overhead)
- Multiple backend support (Badger, Memory, SQL)
- Simpler API
- Good for smaller graphs (<10M nodes)

### badger-graph (Custom Implementation)

Build a custom graph layer on top of existing Badger:

```go
// Simplified graph index using Badger directly
type GraphIndex struct {
    db *badger.DB
}

// Store edge: subject -> predicate -> object
func (g *GraphIndex) AddEdge(subject, predicate, object string) error {
    key := fmt.Sprintf("edge:%s:%s:%s", subject, predicate, object)
    return g.db.Update(func(txn *badger.Txn) error {
        return txn.Set([]byte(key), []byte{})
    })
}

// Query edges
func (g *GraphIndex) GetEdges(subject, predicate string) ([]string, error) {
    prefix := fmt.Sprintf("edge:%s:%s:", subject, predicate)
    // Iterate and collect
}
```

This avoids Dgraph's overhead while providing basic graph functionality.

## Conclusion

Embedding Dgraph in ORLY enables powerful graph queries that extend far beyond Nostr's REQ filters. However, it comes with significant complexity and resource requirements. Consider:

- **Full Dgraph**: For production relays with advanced query needs
- **Cayley**: For medium-sized relays with moderate graph needs
- **Custom Badger-Graph**: For lightweight graph indexing with minimal overhead

Choose based on your specific use case, expected load, and query complexity requirements.
