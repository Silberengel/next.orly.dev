# WebSocket REQ Handling Comparison: Khatru vs Next.orly.dev

## Overview

This document compares how two Nostr relay implementations handle WebSocket connections and REQ (subscription) messages:

1. **Khatru** - A popular Go-based Nostr relay library by fiatjaf
2. **Next.orly.dev** - A custom relay implementation with advanced features

## Architecture Comparison

### Khatru Architecture
- **Monolithic approach**: Single large `HandleWebsocket` method (~380 lines) processes all message types
- **Inline processing**: REQ handling is embedded within the main websocket handler
- **Hook-based extensibility**: Uses function slices for customizable behavior
- **Simple structure**: WebSocket struct with basic fields and mutex for thread safety

### Next.orly.dev Architecture  
- **Modular approach**: Separate methods for each message type (`HandleReq`, `HandleEvent`, etc.)
- **Layered processing**: Message identification → envelope parsing → type-specific handling
- **Publisher-subscriber system**: Dedicated infrastructure for subscription management
- **Rich context**: Listener struct with detailed state tracking and metrics

## Connection Establishment

### Khatru
```go
// Simple websocket upgrade
conn, err := rl.upgrader.Upgrade(w, r, nil)
ws := &WebSocket{
    conn:               conn,
    Request:            r,
    Challenge:          hex.EncodeToString(challenge),
    negentropySessions: xsync.NewMapOf[string, *NegentropySession](),
}
```

### Next.orly.dev
```go
// More sophisticated setup with IP whitelisting
conn, err = websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
listener := &Listener{
    ctx:    ctx,
    Server: s,
    conn:   conn,
    remote: remote,
    req:    r,
}
// Immediate AUTH challenge if ACLs are configured
```

**Key Differences:**
- Next.orly.dev includes IP whitelisting and immediate authentication challenges
- Khatru uses fasthttp/websocket library vs next.orly.dev using coder/websocket
- Next.orly.dev has more detailed connection state tracking

## Message Processing

### Khatru
- Uses `nostr.MessageParser` for sequential parsing
- Switch statement on envelope type within goroutine
- Direct processing without intermediate validation layers

### Next.orly.dev
- Custom envelope identification system (`envelopes.Identify`)
- Separate validation and processing phases
- Extensive logging and error handling at each step

## REQ Message Handling

### Khatru REQ Processing
```go
case *nostr.ReqEnvelope:
    eose := sync.WaitGroup{}
    eose.Add(len(env.Filters))
    
    // Handle each filter separately
    for _, filter := range env.Filters {
        err := srl.handleRequest(reqCtx, env.SubscriptionID, &eose, ws, filter)
        if err != nil {
            // Fail everything if any filter is rejected
            ws.WriteJSON(nostr.ClosedEnvelope{SubscriptionID: env.SubscriptionID, Reason: reason})
            return
        } else {
            rl.addListener(ws, env.SubscriptionID, srl, filter, cancelReqCtx)
        }
    }
    
    go func() {
        eose.Wait()
        ws.WriteJSON(nostr.EOSEEnvelope(env.SubscriptionID))
    }()
```

### Next.orly.dev REQ Processing
```go
// Comprehensive ACL and authentication checks first
accessLevel := acl.Registry.GetAccessLevel(l.authedPubkey.Load(), l.remote)
switch accessLevel {
case "none":
    return // Send auth-required response
}

// Process all filters and collect events
for _, f := range *env.Filters {
    filterEvents, err = l.QueryEvents(queryCtx, f)
    allEvents = append(allEvents, filterEvents...)
}

// Apply privacy and privilege checks
// Send all historical events
// Set up ongoing subscription only if needed
```

## Key Architectural Differences

### 1. **Filter Processing Strategy**

**Khatru:**
- Processes each filter independently and concurrently
- Uses WaitGroup to coordinate EOSE across all filters
- Immediately sets up listeners for ongoing subscriptions
- Fails entire subscription if any filter is rejected

**Next.orly.dev:**
- Processes all filters sequentially in a single context
- Collects all events before applying access control
- Only sets up subscriptions for filters that need ongoing updates
- Gracefully handles individual filter failures

### 2. **Access Control Integration**

**Khatru:**
- Basic NIP-42 authentication support
- Hook-based authorization via `RejectFilter` functions
- Limited built-in access control features

**Next.orly.dev:**
- Comprehensive ACL system with multiple access levels
- Built-in support for private events with npub authorization
- Privileged event filtering based on pubkey and p-tags
- Granular permission checking at multiple stages

### 3. **Subscription Management**

**Khatru:**
```go
// Simple listener registration
type listenerSpec struct {
    filter     nostr.Filter
    cancel     context.CancelCauseFunc
    subRelay   *Relay
}
rl.addListener(ws, subscriptionID, relay, filter, cancel)
```

**Next.orly.dev:**
```go
// Publisher-subscriber system with rich metadata
type W struct {
    Conn         *websocket.Conn
    remote       string
    Id           string
    Receiver     event.C
    Filters      *filter.S
    AuthedPubkey []byte
}
l.publishers.Receive(&W{...})
```

### 4. **Performance Optimizations**

**Khatru:**
- Concurrent filter processing
- Immediate streaming of events as they're found
- Memory-efficient with direct event streaming

**Next.orly.dev:**
- Batch processing with deduplication
- Memory management with explicit `ev.Free()` calls
- Smart subscription cancellation for ID-only queries
- Event result caching and seen-tracking

### 5. **Error Handling & Observability**

**Khatru:**
- Basic error logging
- Simple connection state management
- Limited metrics and observability

**Next.orly.dev:**
- Comprehensive error handling with context preservation
- Detailed logging at each processing stage
- Built-in metrics (message count, REQ count, event count)
- Graceful degradation on individual component failures

## Memory Management

### Khatru
- Relies on Go's garbage collector
- Simple WebSocket struct with minimal state
- Uses sync.Map for thread-safe operations

### Next.orly.dev
- Explicit memory management with `ev.Free()` calls
- Resource pooling and reuse patterns
- Detailed tracking of connection resources

## Concurrency Models

### Khatru
- Per-connection goroutine for message reading
- Additional goroutines for each message processing
- WaitGroup coordination for multi-filter EOSE

### Next.orly.dev
- Per-connection goroutine with single-threaded message processing
- Publisher-subscriber system handles concurrent event distribution
- Context-based cancellation throughout

## Trade-offs Analysis

### Khatru Advantages
- **Simplicity**: Easier to understand and modify
- **Performance**: Lower latency due to concurrent processing
- **Flexibility**: Hook-based architecture allows extensive customization
- **Streaming**: Events sent as soon as they're found

### Khatru Disadvantages
- **Monolithic**: Large methods harder to maintain
- **Limited ACL**: Basic authentication and authorization
- **Error handling**: Less graceful failure recovery
- **Resource usage**: No explicit memory management

### Next.orly.dev Advantages
- **Security**: Comprehensive ACL and privacy features
- **Observability**: Extensive logging and metrics
- **Resource management**: Explicit memory and connection lifecycle management
- **Modularity**: Easier to test and extend individual components
- **Robustness**: Graceful handling of edge cases and failures

### Next.orly.dev Disadvantages
- **Complexity**: Higher cognitive overhead and learning curve
- **Latency**: Sequential processing may be slower for some use cases
- **Resource overhead**: More memory usage due to batching and state tracking
- **Coupling**: Tighter integration between components

## Conclusion

Both implementations represent different philosophies:

- **Khatru** prioritizes simplicity, performance, and extensibility through a hook-based architecture
- **Next.orly.dev** prioritizes security, observability, and robustness through comprehensive built-in features

The choice between them depends on specific requirements:
- Choose **Khatru** for high-performance relays with custom business logic
- Choose **Next.orly.dev** for production relays requiring comprehensive access control and monitoring

Both approaches demonstrate mature understanding of Nostr protocol requirements while making different trade-offs in complexity vs. features.