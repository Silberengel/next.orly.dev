# Strfry WebSocket Implementation for Nostr Relays - Comprehensive Analysis

## Overview

Strfry is a high-performance Nostr relay implementation written in C++ that implements sophisticated WebSocket handling for managing thousands of concurrent connections. It employs a "shared nothing" architecture with multiple specialized threads communicating through message queues.

---

## 1. WebSocket Library & Connection Setup

### 1.1 Library Choice: uWebSockets Fork

**File:** `/tmp/strfry/src/WSConnection.h` (line 4) and `/tmp/strfry/src/apps/relay/RelayServer.h` (line 10)

Strfry uses a custom fork of **uWebSockets** - a high-performance WebSocket library optimized for event-driven networking:

```cpp
#include <uWebSockets/src/uWS.h>

// From README.md:
// "The Websocket thread is a single thread that multiplexes IO to/from 
// multiple connections using the most scalable OS-level interface available 
// (for example, epoll on Linux). It uses [my fork of uWebSockets]"
```

**Key Benefits:**
- Uses OS-level event multiplexing (epoll on Linux, IOCP on Windows)
- Single-threaded WebSocket server handling thousands of connections
- Built-in compression support (permessage-deflate)
- Minimal latency and memory overhead

### 1.2 Server Connection Setup

**File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 161-227)

```cpp
// Initialize the WebSocket group with compression options
{
    int extensionOptions = 0;
    
    // Configure compression based on config settings
    if (cfg().relay__compression__enabled) 
        extensionOptions |= uWS::PERMESSAGE_DEFLATE;
    if (cfg().relay__compression__slidingWindow) 
        extensionOptions |= uWS::SLIDING_DEFLATE_WINDOW;
    
    // Create server group with max payload size limit
    hubGroup = hub.createGroup<uWS::SERVER>(
        extensionOptions, 
        cfg().relay__maxWebsocketPayloadSize  // 131,072 bytes default
    );
}

// Configure automatic PING frames (NIP-11 best practice)
if (cfg().relay__autoPingSeconds) 
    hubGroup->startAutoPing(cfg().relay__autoPingSeconds * 1'000);

// Listen on configured port with SO_REUSEPORT for load balancing
if (!hub.listen(bindHost.c_str(), port, nullptr, uS::REUSE_PORT, hubGroup))
    throw herr("unable to listen on port ", port);

LI << "Started websocket server on " << bindHost << ":" << port;
hub.run();  // Event loop runs here indefinitely
```

### 1.3 Individual Connection Management

**File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 193-227)

```cpp
hubGroup->onConnection([&](uWS::WebSocket<uWS::SERVER> *ws, uWS::HttpRequest req) {
    uint64_t connId = nextConnectionId++;
    
    // Allocate connection metadata structure
    Connection *c = new Connection(ws, connId);
    
    // Extract real IP from header (for reverse proxy setups)
    if (cfg().relay__realIpHeader.size()) {
        auto header = req.getHeader(cfg().relay__realIpHeader.c_str()).toString();
        // Fix IPv6 parsing issues where uWebSockets strips leading colons
        if (header == "1" || header.starts_with("ffff:")) 
            header = std::string("::") + header;
        c->ipAddr = parseIP(header);
    }
    
    // Fallback to WebSocket address bytes if header parsing fails
    if (c->ipAddr.size() == 0) 
        c->ipAddr = ws->getAddressBytes();
    
    // Store connection metadata in WebSocket user data
    ws->setUserData((void*)c);
    connIdToConnection.emplace(connId, c);
    
    // Get compression state
    bool compEnabled, compSlidingWindow;
    ws->getCompressionState(compEnabled, compSlidingWindow);
    LI << "[" << connId << "] Connect from " << renderIP(c->ipAddr)
       << " compression=" << (compEnabled ? 'Y' : 'N')
       << " sliding=" << (compSlidingWindow ? 'Y' : 'N');
    
    // Enable TCP keepalive for early detection of dropped connections
    if (cfg().relay__enableTcpKeepalive) {
        int optval = 1;
        if (setsockopt(ws->getFd(), SOL_SOCKET, SO_KEEPALIVE, &optval, sizeof(optval))) {
            LW << "Failed to enable TCP keepalive: " << strerror(errno);
        }
    }
});
```

### 1.4 Client Connection Wrapper (WSConnection.h)

**File:** `/tmp/strfry/src/WSConnection.h` (lines 56-154)

For outbound connections to other relays, strfry provides a generic WebSocket client wrapper:

```cpp
class WSConnection : NonCopyable {
    uWS::Hub hub;
    uWS::Group<uWS::CLIENT> *hubGroup = nullptr;
    uWS::WebSocket<uWS::CLIENT> *currWs = nullptr;
    
    // Connection callbacks
    std::function<void()> onConnect;
    std::function<void(std::string_view, uWS::OpCode, size_t)> onMessage;
    std::function<void()> onDisconnect;
    std::function<void()> onError;
    
    bool reconnect = true;
    uint64_t reconnectDelayMilliseconds = 5'000;
    
public:
    void run() {
        // Setup with compression for outbound connections
        hubGroup = hub.createGroup<uWS::CLIENT>(
            uWS::PERMESSAGE_DEFLATE | uWS::SLIDING_DEFLATE_WINDOW
        );
        
        // Connection handler with TCP keepalive
        hubGroup->onConnection([&](uWS::WebSocket<uWS::CLIENT> *ws, uWS::HttpRequest req) {
            if (shutdown) return;
            
            remoteAddr = ws->getAddress().address;
            LI << "Connected to " << url << " (" << remoteAddr << ")";
            
            // Enable TCP keepalive
            int optval = 1;
            if (setsockopt(ws->getFd(), SOL_SOCKET, SO_KEEPALIVE, &optval, sizeof(optval))) {
                LW << "Failed to enable TCP keepalive: " << strerror(errno);
            }
            
            currWs = ws;
            if (onConnect) onConnect();
        });
        
        // Automatic reconnection on disconnect
        hubGroup->onDisconnection([&](uWS::WebSocket<uWS::CLIENT> *ws, int code, char *message, size_t length) {
            LI << "Disconnected from " << url << " : " << code;
            
            if (shutdown) return;
            if (ws == currWs) {
                currWs = nullptr;
                if (onDisconnect) onDisconnect();
                if (reconnect) doConnect(reconnectDelayMilliseconds);
            }
        });
        
        // Message reception
        hubGroup->onMessage2([&](uWS::WebSocket<uWS::CLIENT> *ws, char *message, size_t length, uWS::OpCode opCode, size_t compressedSize) {
            if (!onMessage) return;
            try {
                onMessage(std::string_view(message, length), opCode, compressedSize);
            } catch (std::exception &e) {
                LW << "onMessage failure: " << e.what();
            }
        });
        
        hub.run();
    }
};
```

**Configuration:** `/tmp/strfry/strfry.conf` (lines 75-107)

```conf
relay {
    # Maximum accepted incoming websocket frame size (should be larger than max event)
    maxWebsocketPayloadSize = 131072
    
    # Websocket-level PING message frequency (should be less than any reverse proxy idle timeouts)
    autoPingSeconds = 55
    
    # If TCP keep-alive should be enabled (detect dropped connections)
    enableTcpKeepalive = false
    
    compression {
        # Use permessage-deflate compression if supported by client
        enabled = true
        
        # Maintain a sliding window buffer for each connection
        slidingWindow = true
    }
}
```

---

## 2. Message Parsing and Serialization

### 2.1 Incoming Message Reception

**File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 256-263)

When a client sends a message through the WebSocket, the bytes are received and dispatched to the ingester thread:

```cpp
hubGroup->onMessage2([&](uWS::WebSocket<uWS::SERVER> *ws, 
                         char *message, 
                         size_t length, 
                         uWS::OpCode opCode, 
                         size_t compressedSize) {
    auto &c = *(Connection*)ws->getUserData();
    
    // Track bandwidth statistics
    c.stats.bytesDown += length;              // Uncompressed size
    c.stats.bytesDownCompressed += compressedSize;  // Compressed size
    
    // Send to ingester thread for processing
    // Using copy constructor to move data across thread boundary
    tpIngester.dispatch(c.connId, 
        MsgIngester{MsgIngester::ClientMessage{
            c.connId, 
            c.ipAddr, 
            std::string(message, length)  // Copy message data
        }});
});
```

### 2.2 JSON Parsing and Command Routing

**File:** `/tmp/strfry/src/apps/relay/RelayIngester.cpp` (lines 4-86)

The ingester thread parses JSON and routes to appropriate handlers:

```cpp
void RelayServer::runIngester(ThreadPool<MsgIngester>::Thread &thr) {
    secp256k1_context *secpCtx = secp256k1_context_create(SECP256K1_CONTEXT_VERIFY);
    Decompressor decomp;
    
    while(1) {
        // Get all pending messages from ingester inbox (batched)
        auto newMsgs = thr.inbox.pop_all();
        
        // Open read-only transaction for this batch
        auto txn = env.txn_ro();
        
        std::vector<MsgWriter> writerMsgs;
        
        for (auto &newMsg : newMsgs) {
            if (auto msg = std::get_if<MsgIngester::ClientMessage>(&newMsg.msg)) {
                try {
                    // Check if message is valid JSON array
                    if (msg->payload.starts_with('[')) {
                        auto payload = tao::json::from_string(msg->payload);
                        
                        // Optional: dump all incoming messages for debugging
                        if (cfg().relay__logging__dumpInAll) 
                            LI << "[" << msg->connId << "] dumpInAll: " << msg->payload;
                        
                        auto &arr = jsonGetArray(payload, "message is not an array");
                        if (arr.size() < 2) throw herr("too few array elements");
                        
                        // Extract command (first element of array)
                        auto &cmd = jsonGetString(arr[0], "first element not a command");
                        
                        // Route based on command type
                        if (cmd == "EVENT") {
                            // Event submission: ["EVENT", {event}]
                            try {
                                ingesterProcessEvent(txn, msg->connId, msg->ipAddr, 
                                                   secpCtx, arr[1], writerMsgs);
                            } catch (std::exception &e) {
                                // Send negative acknowledgment
                                sendOKResponse(msg->connId, 
                                    arr[1].is_object() && arr[1].at("id").is_string() 
                                        ? arr[1].at("id").get_string() : "?",
                                    false, 
                                    std::string("invalid: ") + e.what());
                            }
                        } 
                        else if (cmd == "REQ") {
                            // Subscription request: ["REQ", "subid", {filter1}, {filter2}, ...]
                            try {
                                ingesterProcessReq(txn, msg->connId, arr);
                            } catch (std::exception &e) {
                                sendNoticeError(msg->connId, 
                                    std::string("bad req: ") + e.what());
                            }
                        } 
                        else if (cmd == "CLOSE") {
                            // Close subscription: ["CLOSE", "subid"]
                            try {
                                ingesterProcessClose(txn, msg->connId, arr);
                            } catch (std::exception &e) {
                                sendNoticeError(msg->connId, 
                                    std::string("bad close: ") + e.what());
                            }
                        } 
                        else if (cmd.starts_with("NEG-")) {
                            // Negentropy synchronization protocol
                            if (!cfg().relay__negentropy__enabled) 
                                throw herr("negentropy disabled");
                            
                            try {
                                ingesterProcessNegentropy(txn, decomp, msg->connId, arr);
                            } catch (std::exception &e) {
                                sendNoticeError(msg->connId, 
                                    std::string("negentropy error: ") + e.what());
                            }
                        } 
                        else {
                            throw herr("unknown cmd");
                        }
                    } 
                    else if (msg->payload == "\n") {
                        // Ignore newlines (for debugging with websocat)
                    } 
                    else {
                        throw herr("unparseable message");
                    }
                } catch (std::exception &e) {
                    sendNoticeError(msg->connId, std::string("bad msg: ") + e.what());
                }
            } 
            else if (auto msg = std::get_if<MsgIngester::CloseConn>(&newMsg.msg)) {
                // Connection closed: propagate to all worker threads
                auto connId = msg->connId;
                tpWriter.dispatch(connId, MsgWriter{MsgWriter::CloseConn{connId}});
                tpReqWorker.dispatch(connId, MsgReqWorker{MsgReqWorker::CloseConn{connId}});
                tpNegentropy.dispatch(connId, MsgNegentropy{MsgNegentropy::CloseConn{connId}});
            }
        }
        
        // Send all validated events to writer thread in one batch
        if (writerMsgs.size()) {
            tpWriter.dispatchMulti(0, writerMsgs);
        }
    }
}
```

### 2.3 Event Processing and Serialization

**File:** `/tmp/strfry/src/apps/relay/RelayIngester.cpp` (lines 88-123)

Events are parsed, validated, and converted to binary format:

```cpp
void RelayServer::ingesterProcessEvent(lmdb::txn &txn, uint64_t connId, 
                                       std::string ipAddr, 
                                       secp256k1_context *secpCtx, 
                                       const tao::json::value &origJson, 
                                       std::vector<MsgWriter> &output) {
    std::string packedStr, jsonStr;
    
    // Parse JSON and verify event structure, signature
    // Uses secp256k1 for Schnorr signature verification
    parseAndVerifyEvent(origJson, secpCtx, true, true, packedStr, jsonStr);
    
    PackedEventView packed(packedStr);
    
    // Check for protected events
    {
        bool foundProtected = false;
        packed.foreachTag([&](char tagName, std::string_view tagVal){
            if (tagName == '-') {  // Protected tag
                foundProtected = true;
                return false;
            }
            return true;
        });
        
        if (foundProtected) {
            LI << "Protected event, skipping";
            sendOKResponse(connId, to_hex(packed.id()), false, 
                         "blocked: event marked as protected");
            return;
        }
    }
    
    // Check for duplicate events
    {
        auto existing = lookupEventById(txn, packed.id());
        if (existing) {
            LI << "Duplicate event, skipping";
            sendOKResponse(connId, to_hex(packed.id()), true, 
                         "duplicate: have this event");
            return;
        }
    }
    
    // Add to output queue for writer thread
    output.emplace_back(MsgWriter{MsgWriter::AddEvent{
        connId, 
        std::move(ipAddr), 
        std::move(packedStr),  // Binary packed format
        std::move(jsonStr)     // Normalized JSON for storage
    }});
}
```

### 2.4 REQ (Subscription) Request Parsing

**File:** `/tmp/strfry/src/apps/relay/RelayIngester.cpp` (lines 125-132)

```cpp
void RelayServer::ingesterProcessReq(lmdb::txn &txn, uint64_t connId, 
                                     const tao::json::value &arr) {
    // Validate array: ["REQ", "subscription_id", {filter}, {filter}, ...]
    if (arr.get_array().size() < 2 + 1) throw herr("arr too small");
    if (arr.get_array().size() > 2 + cfg().relay__maxReqFilterSize) 
        throw herr("arr too big");
    
    // Create subscription object with filters
    Subscription sub(connId, 
                    jsonGetString(arr[1], "REQ subscription id was not a string"), 
                    NostrFilterGroup(arr));  // Parse all filter objects starting at arr[2]
    
    // Dispatch to ReqWorker thread for DB query
    tpReqWorker.dispatch(connId, MsgReqWorker{MsgReqWorker::NewSub{std::move(sub)}});
}
```

### 2.5 Nostr Protocol Message Structures

**File:** `/tmp/strfry/src/apps/relay/RelayServer.h` (lines 25-63)

Three main message types between threads:

```cpp
struct MsgWebsocket : NonCopyable {
    struct Send {
        uint64_t connId;
        std::string payload;  // JSON text to send
    };
    
    struct SendBinary {
        uint64_t connId;
        std::string payload;  // Binary data to send
    };
    
    struct SendEventToBatch {
        RecipientList list;   // Multiple subscribers to same event
        std::string evJson;   // Event JSON (once, reused for all)
    };
    
    struct GracefulShutdown {
    };
    
    using Var = std::variant<Send, SendBinary, SendEventToBatch, GracefulShutdown>;
    Var msg;
};

struct MsgIngester : NonCopyable {
    struct ClientMessage {
        uint64_t connId;
        std::string ipAddr;
        std::string payload;  // Raw client message
    };
    
    struct CloseConn {
        uint64_t connId;
    };
    
    using Var = std::variant<ClientMessage, CloseConn>;
    Var msg;
};
```

---

## 3. Event Handling and Subscription Management

### 3.1 Subscription Data Structure

**File:** `/tmp/strfry/src/Subscription.h`

```cpp
struct SubId {
    char buf[72];  // Max 71 bytes + 1 length byte
    
    SubId(std::string_view val) {
        if (val.size() > 71) throw herr("subscription id too long");
        if (val.size() == 0) throw herr("subscription id too short");
        
        // Validate characters (no control chars, backslash, quotes, UTF-8)
        auto badChar = [](char c){
            return c < 0x20 || c == '\\' || c == '"' || c >= 0x7F;
        };
        
        if (std::any_of(val.begin(), val.end(), badChar)) 
            throw herr("invalid character in subscription id");
        
        // Store length in first byte for O(1) size queries
        buf[0] = (char)val.size();
        memcpy(&buf[1], val.data(), val.size());
    }
    
    std::string_view sv() const {
        return std::string_view(&buf[1], (size_t)buf[0]);
    }
};

// Custom hash function for use in flat_hash_map
namespace std {
    template<> struct hash<SubId> {
        std::size_t operator()(SubId const &p) const {
            return phmap::HashState().combine(0, p.sv());
        }
    };
}

struct Subscription : NonCopyable {
    Subscription(uint64_t connId_, std::string subId_, NostrFilterGroup filterGroup_) 
        : connId(connId_), subId(subId_), filterGroup(filterGroup_) {}
    
    // Subscription parameters
    uint64_t connId;              // Which connection owns this subscription
    SubId subId;                  // Client-assigned subscription identifier
    NostrFilterGroup filterGroup; // Nostr filters to match against events
    
    // Subscription state
    uint64_t latestEventId = MAX_U64;  // Latest event ID seen by this subscription
};

// For batched event delivery to multiple subscribers
struct ConnIdSubId {
    uint64_t connId;
    SubId subId;
};

using RecipientList = std::vector<ConnIdSubId>;
```

### 3.2 ReqWorker: Initial Query Processing

**File:** `/tmp/strfry/src/apps/relay/RelayReqWorker.cpp`

Handles initial historical query from REQ messages:

```cpp
void RelayServer::runReqWorker(ThreadPool<MsgReqWorker>::Thread &thr) {
    Decompressor decomp;
    QueryScheduler queries;
    
    // Callback when an event matches a subscription
    queries.onEvent = [&](lmdb::txn &txn, const auto &sub, uint64_t levId, std::string_view eventPayload){
        // Decompress event if needed, then send to client
        sendEvent(sub.connId, sub.subId, 
                 decodeEventPayload(txn, decomp, eventPayload, nullptr, nullptr));
    };
    
    // Callback when all historical events have been sent
    queries.onComplete = [&](lmdb::txn &, Subscription &sub){
        // Send EOSE (End Of Stored Events) message
        sendToConn(sub.connId, 
            tao::json::to_string(tao::json::value::array({ "EOSE", sub.subId.str() })));
        
        // Move subscription to ReqMonitor for live event streaming
        tpReqMonitor.dispatch(sub.connId, MsgReqMonitor{MsgReqMonitor::NewSub{std::move(sub)}});
    };
    
    while(1) {
        // Process pending subscriptions (or idle if queries running)
        auto newMsgs = queries.running.empty() 
            ? thr.inbox.pop_all()           // Block if idle
            : thr.inbox.pop_all_no_wait();  // Non-blocking if busy
        
        auto txn = env.txn_ro();
        
        for (auto &newMsg : newMsgs) {
            if (auto msg = std::get_if<MsgReqWorker::NewSub>(&newMsg.msg)) {
                auto connId = msg->sub.connId;
                
                // Add subscription to query scheduler
                if (!queries.addSub(txn, std::move(msg->sub))) {
                    sendNoticeError(connId, std::string("too many concurrent REQs"));
                }
                
                // Start processing the subscription
                queries.process(txn);
            } 
            else if (auto msg = std::get_if<MsgReqWorker::RemoveSub>(&newMsg.msg)) {
                // Client sent CLOSE message
                queries.removeSub(msg->connId, msg->subId);
                tpReqMonitor.dispatch(msg->connId, 
                    MsgReqMonitor{MsgReqMonitor::RemoveSub{msg->connId, msg->subId}});
            } 
            else if (auto msg = std::get_if<MsgReqWorker::CloseConn>(&newMsg.msg)) {
                // Connection closed
                queries.closeConn(msg->connId);
                tpReqMonitor.dispatch(msg->connId, 
                    MsgReqMonitor{MsgReqMonitor::CloseConn{msg->connId}});
            }
        }
        
        // Continue processing active subscriptions
        queries.process(txn);
        
        txn.abort();
    }
}
```

### 3.3 ReqMonitor: Live Event Streaming

**File:** `/tmp/strfry/src/ActiveMonitors.h` (lines 13-67)

Handles filtering and delivery of new events to subscriptions:

```cpp
struct ActiveMonitors : NonCopyable {
private:
    struct Monitor : NonCopyable {
        Subscription sub;
        Monitor(Subscription &sub_) : sub(std::move(sub_)) {}
    };
    
    // Connection -> (SubId -> Monitor)
    using ConnMonitor = std::unordered_map<SubId, Monitor>;
    flat_hash_map<uint64_t, ConnMonitor> conns;
    
    // Indexed lookups by event properties for efficient filtering
    struct MonitorItem {
        Monitor *mon;
        uint64_t latestEventId;
    };
    
    using MonitorSet = flat_hash_map<NostrFilter*, MonitorItem>;
    
    btree_map<Bytes32, MonitorSet> allIds;      // By event ID
    btree_map<Bytes32, MonitorSet> allAuthors;  // By author pubkey
    btree_map<std::string, MonitorSet> allTags; // By tag values
    btree_map<uint64_t, MonitorSet> allKinds;   // By event kind
    MonitorSet allOthers;                        // Without filters
    
public:
    // Add a new subscription to live event monitoring
    bool addSub(lmdb::txn &txn, Subscription &&sub, uint64_t currEventId) {
        if (sub.latestEventId != currEventId) 
            throw herr("sub not up to date");
        
        // Check for duplicates
        {
            auto *existing = findMonitor(sub.connId, sub.subId);
            if (existing) removeSub(sub.connId, sub.subId);
        }
        
        // Limit subscriptions per connection
        auto res = conns.try_emplace(sub.connId);
        auto &connMonitors = res.first->second;
        
        if (connMonitors.size() >= cfg().relay__maxSubsPerConnection) {
            return false;
        }
        
        // Insert monitor and index it
        auto subId = sub.subId;
        auto *m = &connMonitors.try_emplace(subId, sub).first->second;
        
        installLookups(m, currEventId);
        return true;
    }
    
    // Remove a subscription
    void removeSub(uint64_t connId, const SubId &subId) {
        auto *monitor = findMonitor(connId, subId);
        if (!monitor) return;
        
        uninstallLookups(monitor);
        
        conns[connId].erase(subId);
        if (conns[connId].empty()) conns.erase(connId);
    }
    
    // Handle connection closure
    void closeConn(uint64_t connId) {
        auto f1 = conns.find(connId);
        // ... remove all subscriptions for this connection
    }
};
```

---

## 4. Connection Management and Cleanup

### 4.1 Graceful Connection Disconnection

**File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 229-254)

```cpp
hubGroup->onDisconnection([&](uWS::WebSocket<uWS::SERVER> *ws, 
                              int code, 
                              char *message, 
                              size_t length) {
    auto *c = (Connection*)ws->getUserData();
    uint64_t connId = c->connId;
    
    // Calculate compression ratios for statistics
    auto upComp = renderPercent(1.0 - (double)c->stats.bytesUpCompressed / c->stats.bytesUp);
    auto downComp = renderPercent(1.0 - (double)c->stats.bytesDownCompressed / c->stats.bytesDown);
    
    // Log disconnection with statistics
    LI << "[" << connId << "] Disconnect from " << renderIP(c->ipAddr)
       << " (" << code << "/" << (message ? std::string_view(message, length) : "-") << ")"
       << " UP: " << renderSize(c->stats.bytesUp) << " (" << upComp << " compressed)"
       << " DN: " << renderSize(c->stats.bytesDown) << " (" << downComp << " compressed)";
    
    // Notify ingester of disconnection (propagates to all workers)
    tpIngester.dispatch(connId, MsgIngester{MsgIngester::CloseConn{connId}});
    
    // Remove connection from map and deallocate
    connIdToConnection.erase(connId);
    delete c;
    
    // Handle graceful shutdown
    if (gracefulShutdown) {
        LI << "Graceful shutdown in progress: " << connIdToConnection.size() 
           << " connections remaining";
        if (connIdToConnection.size() == 0) {
            LW << "All connections closed, shutting down";
            ::exit(0);
        }
    }
});
```

### 4.2 Connection Structure with Statistics

**File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 23-39)

```cpp
struct Connection {
    uWS::WebSocket<uWS::SERVER> *websocket;
    uint64_t connId;
    uint64_t connectedTimestamp;
    std::string ipAddr;
    
    struct Stats {
        uint64_t bytesUp = 0;              // Total uncompressed bytes sent
        uint64_t bytesUpCompressed = 0;    // Total compressed bytes sent
        uint64_t bytesDown = 0;            // Total uncompressed bytes received
        uint64_t bytesDownCompressed = 0;  // Total compressed bytes received
    } stats;
    
    Connection(uWS::WebSocket<uWS::SERVER> *p, uint64_t connId_)
        : websocket(p), connId(connId_), 
          connectedTimestamp(hoytech::curr_time_us()) { }
    Connection(const Connection &) = delete;
    Connection(Connection &&) = delete;
};
```

### 4.3 Thread-Safe Connection Closure Flow

When a connection closes, the event propagates through the system:

1. **WebSocket Thread** detects disconnection, notifies ingester
2. **Ingester Thread** sends CloseConn to Writer, ReqWorker, Negentropy threads
3. **ReqMonitor Thread** cleans up active subscriptions
4. All threads deallocate their connection state

---

## 5. Performance Optimizations Specific to C++

### 5.1 Event Batching for Broadcast

**File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 286-299)

When an event is broadcast to multiple subscribers, memory-efficient batching is used:

```cpp
else if (auto msg = std::get_if<MsgWebsocket::SendEventToBatch>(&newMsg.msg)) {
    // Pre-allocate buffer with maximum needed size
    tempBuf.reserve(13 + MAX_SUBID_SIZE + msg->evJson.size());
    
    // Construct the frame once, with variable subscription ID offset
    tempBuf.resize(10 + MAX_SUBID_SIZE);
    tempBuf += "\",";
    tempBuf += msg->evJson;  // Event JSON
    tempBuf += "]";
    
    // For each recipient, write subscription ID at correct offset and send
    for (auto &item : msg->list) {
        auto subIdSv = item.subId.sv();
        
        // Calculate offset: MaxSubIdSize - actualSubIdSize
        auto *p = tempBuf.data() + MAX_SUBID_SIZE - subIdSv.size();
        
        // Write frame header with subscription ID
        memcpy(p, "[\"EVENT\",\"", 10);
        memcpy(p + 10, subIdSv.data(), subIdSv.size());
        
        // Send frame (compression handled by uWebSockets)
        doSend(item.connId, 
               std::string_view(p, 13 + subIdSv.size() + msg->evJson.size()), 
               uWS::OpCode::TEXT);
    }
}
```

**Optimization Details:**
- Event JSON is serialized once and reused for all recipients
- Buffer is pre-allocated to avoid allocations in hot path
- Memory layout allows variable-length subscription IDs without copying
- Frame is constructed by writing subscription ID at correct offset

### 5.2 String View Usage for Zero-Copy

Throughout the codebase, `std::string_view` is used to avoid unnecessary allocations:

```cpp
// From RelayIngester.cpp - message parsing
hubGroup->onMessage2([&](uWS::WebSocket<uWS::SERVER> *ws, 
                         char *message, 
                         size_t length, 
                         uWS::OpCode opCode, 
                         size_t compressedSize) {
    // Pass by string_view to avoid copy
    tpIngester.dispatch(c.connId, 
        MsgIngester{MsgIngester::ClientMessage{
            c.connId, 
            c.ipAddr, 
            std::string(message, length)  // Only copy what needed
        }});
});
```

### 5.3 Move Semantics for Message Queues

**File:** `/tmp/strfry/src/ThreadPool.h` (lines 42-50)

Thread-safe message dispatch using move semantics:

```cpp
template <typename M>
struct ThreadPool {
    struct Thread {
        uint64_t id;
        std::thread thread;
        hoytech::protected_queue<M> inbox;
    };
    
    // Dispatch message using move (zero-copy)
    void dispatch(uint64_t key, M &&m) {
        uint64_t who = key % numThreads;
        pool[who].inbox.push_move(std::move(m));
    }
    
    // Dispatch multiple messages in batch
    void dispatchMulti(uint64_t key, std::vector<M> &m) {
        uint64_t who = key % numThreads;
        pool[who].inbox.push_move_all(m);
    }
};
```

**Benefits:**
- Messages are moved between threads without copying
- RAII ensures cleanup if reception fails
- Lock-free (or low-contention) queue implementation

### 5.4 Variant-Based Polymorphism

**File:** `/tmp/strfry/src/apps/relay/RelayServer.h` (lines 25-47)

Uses `std::variant` for type-safe message routing without virtual dispatch overhead:

```cpp
struct MsgWebsocket : NonCopyable {
    struct Send { ... };
    struct SendBinary { ... };
    struct SendEventToBatch { ... };
    struct GracefulShutdown { ... };
    
    using Var = std::variant<Send, SendBinary, SendEventToBatch, GracefulShutdown>;
    Var msg;
};

// In handler:
for (auto &newMsg : newMsgs) {
    if (auto msg = std::get_if<MsgWebsocket::Send>(&newMsg.msg)) {
        // Handle Send variant
    } else if (auto msg = std::get_if<MsgWebsocket::SendBinary>(&newMsg.msg)) {
        // Handle SendBinary variant
    }
    // ... etc
}
```

**Advantages:**
- Zero virtual function call overhead
- Compiler generates optimized type-dispatch code
- All memory inline in variant
- Supports both move and copy semantics

### 5.5 Memory Pre-allocation and Buffer Reuse

**File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 47-48)

```cpp
std::string tempBuf;
// Pre-allocate for maximum message size
tempBuf.reserve(cfg().events__maxEventSize + MAX_SUBID_SIZE + 100);
```

This single buffer is reused across all messages in the event loop, avoiding allocation overhead.

### 5.6 Protected Queues with Batch Operations

**File:** `/tmp/strfry/src/apps/relay/RelayIngester.cpp` (line 9)

```cpp
// Batch retrieve all pending messages
auto newMsgs = thr.inbox.pop_all();

for (auto &newMsg : newMsgs) {
    // Process messages in batch
}
```

**Benefits:**
- Single lock acquisition per batch, not per message
- Better CPU cache locality
- Amortizes lock overhead

### 5.7 Lazy Initialization and Caching

**File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 64-105)

HTTP responses are pre-generated and cached:

```cpp
auto getServerInfoHttpResponse = [&supportedNips, ver = uint64_t(0), 
                                  rendered = std::string("")](){ mutable {
    // Only regenerate if config version changed
    if (ver != cfg().version()) {
        tao::json::value nip11 = tao::json::value({
            { "supported_nips", supportedNips() },
            { "software", "git+https://github.com/hoytech/strfry.git" },
            { "version", APP_GIT_VERSION },
            // ... build response
        });
        
        rendered = preGenerateHttpResponse("application/json", tao::json::to_string(nip11));
        ver = cfg().version();
    }
    
    return std::string_view(rendered);
};
```

### 5.8 Compression with Dictionary Support

**File:** `/tmp/strfry/src/Decompressor.h` (lines 34-68)

Efficient decompression with ZSTD dictionaries:

```cpp
struct Decompressor {
    ZSTD_DCtx *dctx;
    flat_hash_map<uint32_t, ZSTD_DDict*> dicts;
    std::string buffer;  // Reusable buffer
    
    Decompressor() {
        dctx = ZSTD_createDCtx();  // Context created once
    }
    
    // Decompress with cached dictionaries
    std::string_view decompress(lmdb::txn &txn, uint32_t dictId, std::string_view src) {
        auto it = dicts.find(dictId);
        ZSTD_DDict *dict;
        
        if (it == dicts.end()) {
            // Load from DB if not cached
            dict = dicts[dictId] = globalDictionaryBroker.getDict(txn, dictId);
        } else {
            dict = it->second;  // Use cached dictionary
        }
        
        auto ret = ZSTD_decompress_usingDDict(dctx, buffer.data(), buffer.size(), 
                                             src.data(), src.size(), dict);
        if (ZDICT_isError(ret)) 
            throw herr("zstd decompression failed: ", ZSTD_getErrorName(ret));
        
        return std::string_view(buffer.data(), ret);
    }
};
```

**Optimizations:**
- Single decompression context reused across messages
- Dictionary caching avoids repeated lookups
- Buffer reuse prevents allocations
- Uses custom dictionaries trained on Nostr event format

### 5.9 Single-Threaded Event Loop for WebSocket I/O

**File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (line 326)

```cpp
hub.run();  // Blocks here, running epoll event loop
```

**Benefits:**
- Single thread handles all I/O multiplexing
- No contention for connection structures
- Optimal CPU cache utilization
- O(1) event handling with epoll

### 5.10 Lock-Free Inter-Thread Communication

Thread pools use lock-free or low-contention queues:

```cpp
void dispatch(uint64_t key, M &&m) {
    uint64_t who = key % numThreads;  // Deterministic dispatch
    pool[who].inbox.push_move(std::move(m));  // Lock-free push
}
```

Connection ID is hash distributed across ingester threads to balance load without locks.

### 5.11 Template-Based HTTP Response Caching

**File:** `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 98-104)

Uses template system for pre-generating HTML responses:

```cpp
rendered = preGenerateHttpResponse("text/html", ::strfrytmpl::landing(ctx).str);
```

This is compiled at build time, avoiding runtime template processing.

### 5.12 Ring Buffer Implementation for Subscriptions

Active monitors use efficient data structures:

```cpp
btree_map<Bytes32, MonitorSet> allIds;      // B-tree for range queries
flat_hash_map<NostrFilter*, MonitorItem> allOthers;  // Hash map for others
```

These provide O(log n) or O(1) lookups depending on filter type.

---

## 6. Architecture Summary Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Single WebSocket Thread                   │
│              (uWebSockets Hub with epoll/IOCP)              │
│  - Connection multiplexing                                   │
│  - Message reception & serialization                         │
│  - Response sending with compression                         │
└──────────────────────┬──────────────────────────────────────┘
                       │ ThreadPool::dispatch()
         ┌─────────────┼─────────────┬──────────────┐
         │             │             │              │
    ┌────▼────┐  ┌─────▼──┐  ┌──────▼────┐  ┌─────▼────┐
    │ Ingester│  │ Writer │  │ReqWorker  │  │ReqMonitor│
    │ Threads │  │Thread  │  │ Threads   │  │ Threads  │
    │ (1-N)   │  │ (1)    │  │ (1-N)     │  │ (1-N)    │
    │         │  │        │  │           │  │          │
    │ - Parse │  │- Event │  │- DB Query │  │- Live    │
    │   JSON  │  │  Write │  │  Scan     │  │  Event   │
    │ - Route │  │- Sig   │  │- EOSE     │  │  Filter  │
    │   Cmds  │  │  Verify│  │  Signal   │  │- Dispatch│
    │ - Valid │  │- Batch │  │           │  │  Matches │
    │   Event │  │  Writes│  │           │  │          │
    └────┬────┘  └────┬───┘  └───────┬──┘  └────┬─────┘
         │             │              │           │
         └─────────────┴──────────────┴───────────┘
                       │
         ┌─────────────▼──────────────┐
         │    LMDB Key-Value Store    │
         │   (Local Data Storage)     │
         └────────────────────────────┘
```

---

## 7. Key Statistics and Tuning

From `/tmp/strfry/strfry.conf`:

| Parameter | Default | Purpose |
|-----------|---------|---------|
| `maxWebsocketPayloadSize` | 131,072 bytes | Max frame size |
| `autoPingSeconds` | 55 | PING frequency for keepalive |
| `maxReqFilterSize` | 200 | Max filters per REQ |
| `maxSubsPerConnection` | 20 | Concurrent subscriptions per client |
| `maxFilterLimit` | 500 | Max events returned per filter |
| `queryTimesliceBudgetMicroseconds` | 10,000 | CPU time per query slice |
| `ingester` threads | 3 | Message parsing threads |
| `reqWorker` threads | 3 | Initial query threads |
| `reqMonitor` threads | 3 | Live event filtering threads |
| `negentropy` threads | 2 | Sync protocol threads |

---

## 8. Code Complexity Summary

| Component | Lines | Complexity | Key Technique |
|-----------|-------|-----------|----------------|
| RelayWebsocket.cpp | 327 | High | Event loop + async dispatch |
| RelayIngester.cpp | 170 | High | JSON parsing + routing |
| ActiveMonitors.h | 235 | Very High | Indexed subscription tracking |
| WriterPipeline.h | 209 | High | Batched writes with debounce |
| RelayServer.h | 231 | Medium | Message type routing |
| WSConnection.h | 175 | Medium | Client WebSocket wrapper |

---

## 9. References

- **Repository:** https://github.com/hoytech/strfry
- **WebSocket Library:** https://github.com/hoytech/uWebSockets (fork)
- **LMDB:** Lightning Memory-Mapped Database
- **secp256k1:** Schnorr signature verification
- **ZSTD:** Zstandard compression with dictionaries
- **FlatBuffers:** Zero-copy serialization
- **Negentropy:** Set reconciliation protocol

