# Strfry WebSocket Implementation - Quick Reference

## Key Architecture Points

### 1. WebSocket Library
- **Library:** uWebSockets fork (custom from hoytech)
- **Event Multiplexing:** epoll (Linux), IOCP (Windows)
- **Threading Model:** Single-threaded event loop for I/O
- **File:** `/tmp/strfry/src/WSConnection.h` (client wrapper)
- **File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (server implementation)

### 2. Message Flow Architecture

```
Client → WebSocket Thread → Ingester Threads → Writer/ReqWorker/ReqMonitor → DB
Client ← WebSocket Thread ← Message Queue     ← All Worker Threads
```

### 3. Compression Configuration

**Enabled Compression:**
- `PERMESSAGE_DEFLATE` - RFC 7692 permessage compression
- `SLIDING_DEFLATE_WINDOW` - Sliding window (better compression, more memory)
- Custom ZSTD dictionaries for event decompression

**Config:** `/tmp/strfry/strfry.conf` lines 101-107

```conf
compression {
    enabled = true
    slidingWindow = true
}
```

### 4. Critical Data Structures

| Structure | File | Purpose |
|-----------|------|---------|
| `Connection` | RelayWebsocket.cpp:23-39 | Per-connection metadata + stats |
| `Subscription` | Subscription.h | Client REQ with filters + state |
| `SubId` | Subscription.h:8-37 | Compact subscription ID (71 bytes max) |
| `MsgWebsocket` | RelayServer.h:25-47 | Outgoing message variants |
| `MsgIngester` | RelayServer.h:49-63 | Incoming message variants |

### 5. Thread Pool Architecture

**ThreadPool<M> Template** (ThreadPool.h:7-61)

```cpp
// Deterministic dispatch based on connection ID hash
void dispatch(uint64_t connId, M &&msg) {
    uint64_t threadId = connId % numThreads;
    pool[threadId].inbox.push_move(std::move(msg));
}
```

**Thread Counts:**
- Ingester: 3 threads (default)
- ReqWorker: 3 threads (historical queries)
- ReqMonitor: 3 threads (live filtering)
- Negentropy: 2 threads (sync protocol)
- Writer: 1 thread (LMDB writes)
- WebSocket: 1 thread (I/O multiplexing)

### 6. Event Batching Optimization

**Location:** RelayWebsocket.cpp:286-299

When broadcasting event to multiple subscribers:
- Serialize event JSON once
- Reuse buffer with variable offset for subscription IDs
- Single memcpy per subscriber (not per message)
- Reduces CPU and memory overhead significantly

```cpp
SendEventToBatch {
    RecipientList list;  // Vector of (connId, subId) pairs
    std::string evJson;  // One copy, broadcast to all
}
```

### 7. Connection Lifecycle

1. **Connection** (RelayWebsocket.cpp:193-227)
   - onConnection() called
   - Connection metadata allocated
   - IP address extracted (with reverse proxy support)
   - TCP keepalive enabled (optional)

2. **Message Reception** (RelayWebsocket.cpp:256-263)
   - onMessage2() callback
   - Stats updated (compressed/uncompressed sizes)
   - Dispatched to ingester thread

3. **Message Ingestion** (RelayIngester.cpp:4-86)
   - JSON parsing
   - Command routing (EVENT/REQ/CLOSE/NEG-*)
   - Event validation (secp256k1 signature check)
   - Duplicate detection

4. **Disconnection** (RelayWebsocket.cpp:229-254)
   - onDisconnection() called
   - Stats logged
   - CloseConn message sent to all workers
   - Connection deallocated

### 8. Performance Optimizations

| Technique | Location | Benefit |
|-----------|----------|---------|
| Move semantics | ThreadPool.h:42-45 | Zero-copy message passing |
| std::string_view | Throughout | Avoid string copies |
| std::variant | RelayServer.h:25+ | Type-safe dispatch, no vtables |
| Pre-allocated buffers | RelayWebsocket.cpp:47-48 | Avoid allocations in hot path |
| Batch queue operations | RelayIngester.cpp:9 | Single lock per batch |
| Lazy initialization | RelayWebsocket.cpp:64+ | Cache HTTP responses |
| ZSTD dictionary caching | Decompressor.h:34-68 | Fast decompression |
| Sliding window compression | WSConnection.h:57 | Better compression ratio |

### 9. Key Configuration Parameters

```conf
relay {
    maxWebsocketPayloadSize = 131072      # 128 KB frame limit
    autoPingSeconds = 55                  # PING keepalive frequency
    enableTcpKeepalive = false            # TCP_KEEPALIVE socket option
    
    compression {
        enabled = true
        slidingWindow = true
    }
    
    numThreads {
        ingester = 3
        reqWorker = 3
        reqMonitor = 3
        negentropy = 2
    }
}
```

### 10. Bandwidth Tracking

Per-connection statistics:
```cpp
struct Stats {
    uint64_t bytesUp = 0;              // Sent (uncompressed)
    uint64_t bytesUpCompressed = 0;    // Sent (compressed)
    uint64_t bytesDown = 0;            // Received (uncompressed)
    uint64_t bytesDownCompressed = 0;  // Received (compressed)
}
```

Logged on disconnection with compression ratios.

### 11. Nostr Protocol Message Types

**Incoming (Client → Server):**
- `["EVENT", {...}]` - Submit event
- `["REQ", "sub_id", {...filters...}]` - Subscribe to events
- `["CLOSE", "sub_id"]` - Unsubscribe
- `["NEG-*", ...]` - Negentropy sync

**Outgoing (Server → Client):**
- `["EVENT", "sub_id", {...}]` - Event matching subscription
- `["EOSE", "sub_id"]` - End of stored events
- `["OK", event_id, success, message]` - Event submission result
- `["NOTICE", message]` - Server notices
- `["NEG-*", ...]` - Negentropy sync responses

### 12. Filter Processing Pipeline

```
Client REQ → Ingester → ReqWorker → ReqMonitor → Active Monitors (indexed)
                           ↓              ↓
                       DB Query       New Events
                           ↓              ↓
                        EOSE ----→ Matched Subscribers
                                       ↓
                                   WebSocket Send
```

**Indexes in ActiveMonitors:**
- `allIds` - B-tree by event ID
- `allAuthors` - B-tree by pubkey
- `allKinds` - B-tree by event kind
- `allTags` - B-tree by tag values
- `allOthers` - Hash map for unrestricted subscriptions

### 13. File Sizes & Complexity

| File | Lines | Role |
|------|-------|------|
| RelayWebsocket.cpp | 327 | Main WebSocket handler + event loop |
| RelayIngester.cpp | 170 | Message parsing & validation |
| ActiveMonitors.h | 235 | Subscription indexing |
| WriterPipeline.h | 209 | Batched DB writes |
| RelayServer.h | 231 | Message type definitions |
| Decompressor.h | 68 | ZSTD decompression |
| ThreadPool.h | 61 | Generic thread pool |

### 14. Error Handling

- JSON parsing errors → NOTICE message
- Invalid events → OK response with reason
- REQ validation → NOTICE message
- Bad subscription → Error response
- Signature verification failures → Detailed logging

### 15. Scalability Features

1. **Epoll-based I/O** - Handle thousands of connections on single thread
2. **Lock-free queues** - No contention for message passing
3. **Batch processing** - Amortize locks and allocations
4. **Load distribution** - Hash-based thread assignment
5. **Memory efficiency** - Move semantics, string_view, pre-allocation
6. **Compression** - Permessage-deflate + sliding window
7. **Graceful shutdown** - Finish pending subscriptions before exit

---

## Related Files in Strfry Repository

```
/tmp/strfry/
├── src/
│   ├── WSConnection.h                    # Client WebSocket wrapper
│   ├── Subscription.h                    # Subscription data structure
│   ├── Decompressor.h                    # ZSTD decompression
│   ├── ThreadPool.h                      # Generic thread pool
│   ├── WriterPipeline.h                  # Batched writes
│   ├── ActiveMonitors.h                  # Subscription indexing
│   ├── events.h                          # Event validation
│   ├── filters.h                         # Filter matching
│   ├── apps/relay/
│   │   ├── RelayWebsocket.cpp            # Main WebSocket server
│   │   ├── RelayIngester.cpp             # Message parsing
│   │   ├── RelayReqWorker.cpp            # Initial query processing
│   │   ├── RelayReqMonitor.cpp           # Live event filtering
│   │   ├── RelayWriter.cpp               # Database writes
│   │   ├── RelayNegentropy.cpp           # Sync protocol
│   │   └── RelayServer.h                 # Message definitions
├── strfry.conf                           # Configuration
└── README.md                             # Architecture documentation
```

---

## Key Insights

1. **Single WebSocket thread** with epoll handles all I/O - no thread contention for connections

2. **Message variants with std::variant** avoid virtual function calls for type dispatch

3. **Event batching** serializes event once, reuses for all subscribers - huge bandwidth/CPU savings

4. **Thread-deterministic dispatch** using modulo hash ensures related messages go to same thread

5. **Pre-allocated buffers** and move semantics minimize allocations in hot path

6. **Lazy response caching** means NIP-11 info is pre-generated and cached

7. **Compression on by default** with sliding window for better ratios

8. **TCP keepalive** detects dropped connections through reverse proxies

9. **Per-connection statistics** track compression effectiveness for observability

10. **Graceful shutdown** ensures EOSE is sent before closing subscriptions

