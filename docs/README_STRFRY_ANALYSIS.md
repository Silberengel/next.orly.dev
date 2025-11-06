# Strfry WebSocket Implementation - Complete Analysis

This directory contains a comprehensive analysis of how strfry implements WebSocket handling for Nostr relays in C++.

## Documents Included

### 1. `strfry_websocket_analysis.md` (1138 lines)
**Complete reference guide covering:**
- WebSocket library selection and connection setup (uWebSockets fork)
- Message parsing and serialization (JSON → binary packed format)
- Event handling and subscription management (filters, indexing)
- Connection management and cleanup (lifecycle, graceful shutdown)
- Performance optimizations specific to C++ (move semantics, batching, etc.)
- Architecture summary with diagrams
- Code complexity analysis
- References and related files

**Key Sections:**
1. WebSocket Library & Connection Setup
2. Message Parsing and Serialization
3. Event Handling and Subscription Management
4. Connection Management and Cleanup
5. Performance Optimizations Specific to C++
6. Architecture Summary Diagram
7. Key Statistics and Tuning
8. Code Complexity Summary

### 2. `strfry_websocket_quick_reference.md`
**Quick lookup guide for:**
- Architecture points and thread pools
- Critical data structures
- Event batching optimization
- Connection lifecycle
- Performance techniques with specific file:line references
- Configuration parameters
- Nostr protocol message types
- Filter processing pipeline
- Bandwidth tracking
- Scalability features
- Key insights (10 actionable takeaways)

### 3. `strfry_websocket_code_flow.md`
**Detailed code flow examples:**
1. Connection Establishment Flow
2. Incoming Message Processing Flow
3. Event Submission Flow (validation → database → acknowledgment)
4. Subscription Request (REQ) Flow
5. Event Broadcasting Flow (critical batching optimization)
6. Connection Disconnection Flow
7. Thread Pool Message Dispatch (deterministic routing)
8. Message Type Dispatch Pattern (std::variant routing)
9. Subscription Lifecycle Summary
10. Error Handling Flow

**Each section includes:**
- Exact file paths and line numbers
- Full code examples with inline comments
- Step-by-step execution trace
- Performance impact analysis

## Repository Information

**Source:** https://github.com/hoytech/strfry  
**Local Clone:** `/tmp/strfry/`

## Key Findings Summary

### Architecture
- **Single WebSocket thread** uses epoll for connection multiplexing (thousands of concurrent connections)
- **Multiple worker threads** (Ingester, Writer, ReqWorker, ReqMonitor, Negentropy) communicate via message queues
- **"Shared nothing" design** eliminates lock contention for connection state

### WebSocket Library
- **uWebSockets fork** (custom from hoytech)
- Event-driven architecture (epoll on Linux, IOCP on Windows)
- Built-in permessage-deflate compression with sliding window
- Callbacks for connection, disconnection, message reception

### Message Flow
```
WebSocket Thread (I/O) → Ingester Threads (validation) 
→ Writer Thread (DB) → ReqMonitor Threads (filtering) 
→ WebSocket Thread (sending)
```

### Critical Optimizations

1. **Event Batching for Broadcast**
   - Single event JSON serialization
   - Reusable buffer with variable subscription ID offset
   - One memcpy per subscriber, not per message
   - Huge CPU and memory savings at scale

2. **Move Semantics**
   - Messages moved between threads without copying
   - Zero-copy thread communication via std::move
   - RAII ensures cleanup

3. **std::variant Type Dispatch**
   - Type-safe message routing without virtual functions
   - Compiler-optimized branching
   - All data inline in variant (no heap allocation)

4. **Thread Pool Hash Distribution**
   - `connId % numThreads` for deterministic assignment
   - Improves cache locality
   - Reduces lock contention

5. **Lazy Response Caching**
   - NIP-11 HTTP responses pre-generated and cached
   - Only regenerated when config changes
   - Template system for HTML generation

6. **Compression with Dictionaries**
   - ZSTD dictionaries trained on Nostr event format
   - Dictionary caching avoids repeated lookups
   - Sliding window for better compression ratios

7. **Batched Queue Operations**
   - Single lock acquisition per message batch
   - Amortizes synchronization overhead
   - Improves throughput

8. **Pre-allocated Buffers**
   - Avoid allocations in hot path
   - Single buffer reused across messages
   - Reserve with maximum event size

## File Structure

```
strfry/src/
├── WSConnection.h                   (175 lines) - Client WebSocket wrapper
├── Subscription.h                   (69 lines) - Subscription data structure
├── ThreadPool.h                     (61 lines) - Generic thread pool template
├── Decompressor.h                   (68 lines) - ZSTD decompression with cache
├── WriterPipeline.h                 (209 lines) - Batched database writes
├── ActiveMonitors.h                 (235 lines) - Subscription indexing
├── apps/relay/
│   ├── RelayWebsocket.cpp           (327 lines) - Main WebSocket server + event loop
│   ├── RelayIngester.cpp            (170 lines) - Message parsing + validation
│   ├── RelayReqWorker.cpp           (45 lines) - Initial DB query processor
│   ├── RelayReqMonitor.cpp          (62 lines) - Live event filtering
│   ├── RelayWriter.cpp              (113 lines) - Database write handler
│   ├── RelayNegentropy.cpp          (264 lines) - Sync protocol handler
│   └── RelayServer.h                (231 lines) - Message type definitions
```

## Configuration

**File:** `/tmp/strfry/strfry.conf`

Key tuning parameters:
```conf
relay {
    maxWebsocketPayloadSize = 131072      # 128 KB frame limit
    autoPingSeconds = 55                  # PING keepalive
    enableTcpKeepalive = false            # TCP_KEEPALIVE option
    
    compression {
        enabled = true                    # Permessage-deflate
        slidingWindow = true              # Sliding window
    }
    
    numThreads {
        ingester = 3                      # JSON parsing
        reqWorker = 3                     # Historical queries
        reqMonitor = 3                    # Live filtering
        negentropy = 2                    # Sync protocol
    }
}
```

## Performance Metrics

From code analysis:

| Metric | Value |
|--------|-------|
| Max concurrent connections | Thousands (epoll-limited) |
| Max message size | 131,072 bytes |
| Max subscriptions per connection | 20 |
| Query time slice budget | 10,000 microseconds |
| Auto-ping frequency | 55 seconds |
| Compression overhead | Varies (measured per connection) |

## Nostr Protocol Support

**NIP-01** (Core)
- EVENT: event submission
- REQ: subscription requests
- CLOSE: subscription cancellation
- OK: submission acknowledgment
- EOSE: end of stored events

**NIP-11** (Server Information)
- Provides relay metadata and capabilities

**Additional NIPs:** 2, 4, 9, 22, 28, 40, 70, 77
**Set Reconciliation:** Negentropy protocol for efficient syncing

## Key Insights

1. **Single-threaded I/O** with epoll achieves better throughput than multi-threaded approaches for WebSocket servers

2. **Message variants** (std::variant) avoid virtual function overhead while providing type-safe dispatch

3. **Event batching** is critical for scaling to thousands of subscribers - reuse serialization, not message

4. **Deterministic thread assignment** (hash-based) eliminates need for locks on connection state

5. **Pre-allocation strategies** prevent allocation/deallocation churn in hot paths

6. **Lazy initialization** of responses means zero work for unconfigured relay info

7. **Compression always enabled** with sliding window balances CPU vs bandwidth

8. **TCP keepalive** essential for production with reverse proxies (detects dropped connections)

9. **Per-connection statistics** provide observability for compression effectiveness and troubleshooting

10. **Graceful shutdown** ensures EOSE is sent before disconnecting subscribers

## Building and Testing

**From README.md:**
```bash
# Debian/Ubuntu
sudo apt install -y git g++ make libssl-dev zlib1g-dev liblmdb-dev libflatbuffers-dev libsecp256k1-dev libzstd-dev
git clone https://github.com/hoytech/strfry && cd strfry/
git submodule update --init
make setup-golpe
make -j4

# Run relay
./strfry relay

# Stream events from another relay
./strfry stream wss://relay.example.com
```

## Related Resources

- **Repository:** https://github.com/hoytech/strfry
- **Nostr Protocol:** https://github.com/nostr-protocol/nostr
- **LMDB:** Lightning Memory-Mapped Database (embedded KV store)
- **Negentropy:** Set reconciliation protocol for efficient syncing
- **secp256k1:** Schnorr signature verification library
- **FlatBuffers:** Zero-copy serialization library
- **ZSTD:** Zstandard compression

## Analysis Methodology

This analysis was performed by:
1. Cloning the official strfry repository
2. Examining all WebSocket-related source files
3. Tracing message flow through the entire system
4. Identifying performance optimization patterns
5. Documenting code examples with exact file:line references
6. Creating flow diagrams for complex operations

## Author Notes

Strfry demonstrates several best practices for high-performance C++ networking:
- Separation of concerns with thread-based actors
- Deterministic routing to improve cache locality
- Lazy evaluation and caching for computation reduction
- Memory efficiency through move semantics and pre-allocation
- Type safety with std::variant and no virtual dispatch overhead

This is production code battle-tested in the Nostr ecosystem, handling real-world relay operations at scale.

---

**Last Updated:** 2025-11-06  
**Source Repository Version:** Latest from GitHub  
**Analysis Completeness:** Comprehensive coverage of all WebSocket and connection handling code
