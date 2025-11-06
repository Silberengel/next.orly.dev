# Strfry WebSocket - Detailed Code Flow Examples

## 1. Connection Establishment Flow

### Code Path: Connection → IP Resolution → Dispatch

**File: `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 193-227)**

```cpp
// Step 1: New WebSocket connection arrives
hubGroup->onConnection([&](uWS::WebSocket<uWS::SERVER> *ws, uWS::HttpRequest req) {
    // Step 2: Allocate connection ID and metadata
    uint64_t connId = nextConnectionId++;
    Connection *c = new Connection(ws, connId);
    
    // Step 3: Resolve real IP address
    if (cfg().relay__realIpHeader.size()) {
        // Check for X-Real-IP header (reverse proxy)
        auto header = req.getHeader(cfg().relay__realIpHeader.c_str()).toString();
        
        // Fix IPv6 parsing: uWebSockets strips leading ':'
        if (header == "1" || header.starts_with("ffff:")) 
            header = std::string("::") + header;
        
        c->ipAddr = parseIP(header);
    }
    
    // Step 4: Fallback to direct connection IP if header not present
    if (c->ipAddr.size() == 0) 
        c->ipAddr = ws->getAddressBytes();
    
    // Step 5: Store connection metadata for later retrieval
    ws->setUserData((void*)c);
    connIdToConnection.emplace(connId, c);
    
    // Step 6: Log connection with compression state
    bool compEnabled, compSlidingWindow;
    ws->getCompressionState(compEnabled, compSlidingWindow);
    LI << "[" << connId << "] Connect from " << renderIP(c->ipAddr)
       << " compression=" << (compEnabled ? 'Y' : 'N')
       << " sliding=" << (compSlidingWindow ? 'Y' : 'N');
    
    // Step 7: Enable TCP keepalive for early detection
    if (cfg().relay__enableTcpKeepalive) {
        int optval = 1;
        if (setsockopt(ws->getFd(), SOL_SOCKET, SO_KEEPALIVE, &optval, sizeof(optval))) {
            LW << "Failed to enable TCP keepalive: " << strerror(errno);
        }
    }
});

// Step 8: Event loop continues (hub.run() at line 326)
```

---

## 2. Incoming Message Processing Flow

### Code Path: Reception → Ingestion → Validation → Distribution

**File 1: `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 256-263)**

```cpp
// STEP 1: WebSocket receives message from client
hubGroup->onMessage2([&](uWS::WebSocket<uWS::SERVER> *ws, 
                         char *message, 
                         size_t length, 
                         uWS::OpCode opCode, 
                         size_t compressedSize) {
    auto &c = *(Connection*)ws->getUserData();
    
    // STEP 2: Update bandwidth statistics
    c.stats.bytesDown += length;                    // Uncompressed size
    c.stats.bytesDownCompressed += compressedSize; // Compressed size (or 0 if not compressed)
    
    // STEP 3: Dispatch message to ingester thread
    // Note: Uses move semantics to avoid copying message data again
    tpIngester.dispatch(c.connId, 
        MsgIngester{MsgIngester::ClientMessage{
            c.connId,           // Which connection sent it
            c.ipAddr,           // Sender's IP address
            std::string(message, length)  // Message payload
        }});
    // Message is now in ingester's inbox queue
});
```

**File 2: `/tmp/strfry/src/apps/relay/RelayIngester.cpp` (lines 4-86)**

```cpp
// STEP 4: Ingester thread processes batched messages
void RelayServer::runIngester(ThreadPool<MsgIngester>::Thread &thr) {
    secp256k1_context *secpCtx = secp256k1_context_create(SECP256K1_CONTEXT_VERIFY);
    Decompressor decomp;
    
    while(1) {
        // STEP 5: Get all pending messages (batched for efficiency)
        auto newMsgs = thr.inbox.pop_all();
        
        // STEP 6: Open read-only transaction for this batch
        auto txn = env.txn_ro();
        
        std::vector<MsgWriter> writerMsgs;
        
        for (auto &newMsg : newMsgs) {
            if (auto msg = std::get_if<MsgIngester::ClientMessage>(&newMsg.msg)) {
                try {
                    // STEP 7: Check if message is JSON array
                    if (msg->payload.starts_with('[')) {
                        auto payload = tao::json::from_string(msg->payload);
                        
                        auto &arr = jsonGetArray(payload, "message is not an array");
                        if (arr.size() < 2) throw herr("too few array elements");
                        
                        // STEP 8: Extract command from first array element
                        auto &cmd = jsonGetString(arr[0], "first element not a command");
                        
                        // STEP 9: Route based on command type
                        if (cmd == "EVENT") {
                            // EVENT command: ["EVENT", {event_object}]
                            // File: RelayIngester.cpp:88-123
                            try {
                                ingesterProcessEvent(txn, msg->connId, msg->ipAddr, 
                                                   secpCtx, arr[1], writerMsgs);
                            } catch (std::exception &e) {
                                sendOKResponse(msg->connId, 
                                    arr[1].is_object() && arr[1].at("id").is_string() 
                                        ? arr[1].at("id").get_string() : "?",
                                    false, 
                                    std::string("invalid: ") + e.what());
                            }
                        } 
                        else if (cmd == "REQ") {
                            // REQ command: ["REQ", "sub_id", {filter1}, {filter2}...]
                            // File: RelayIngester.cpp:125-132
                            try {
                                ingesterProcessReq(txn, msg->connId, arr);
                            } catch (std::exception &e) {
                                sendNoticeError(msg->connId, 
                                    std::string("bad req: ") + e.what());
                            }
                        } 
                        else if (cmd == "CLOSE") {
                            // CLOSE command: ["CLOSE", "sub_id"]
                            // File: RelayIngester.cpp:134-138
                            try {
                                ingesterProcessClose(txn, msg->connId, arr);
                            } catch (std::exception &e) {
                                sendNoticeError(msg->connId, 
                                    std::string("bad close: ") + e.what());
                            }
                        }
                        else if (cmd.starts_with("NEG-")) {
                            // Negentropy sync command
                            try {
                                ingesterProcessNegentropy(txn, decomp, msg->connId, arr);
                            } catch (std::exception &e) {
                                sendNoticeError(msg->connId, 
                                    std::string("negentropy error: ") + e.what());
                            }
                        }
                    }
                } catch (std::exception &e) {
                    sendNoticeError(msg->connId, std::string("bad msg: ") + e.what());
                }
            }
        }
        
        // STEP 10: Batch dispatch all validated events to writer thread
        if (writerMsgs.size()) {
            tpWriter.dispatchMulti(0, writerMsgs);
        }
    }
}
```

---

## 3. Event Submission Flow

### Code Path: EVENT Command → Validation → Database Storage → Acknowledgment

**File: `/tmp/strfry/src/apps/relay/RelayIngester.cpp` (lines 88-123)**

```cpp
void RelayServer::ingesterProcessEvent(
    lmdb::txn &txn, 
    uint64_t connId, 
    std::string ipAddr, 
    secp256k1_context *secpCtx, 
    const tao::json::value &origJson, 
    std::vector<MsgWriter> &output) {
    
    std::string packedStr, jsonStr;
    
    // STEP 1: Parse and verify event
    // - Extracts all fields (id, pubkey, created_at, kind, tags, content, sig)
    // - Verifies Schnorr signature using secp256k1
    // - Normalizes JSON to canonical form
    parseAndVerifyEvent(origJson, secpCtx, true, true, packedStr, jsonStr);
    
    PackedEventView packed(packedStr);
    
    // STEP 2: Check for protected events (marked with '-' tag)
    {
        bool foundProtected = false;
        packed.foreachTag([&](char tagName, std::string_view tagVal){
            if (tagName == '-') {
                foundProtected = true;
                return false;
            }
            return true;
        });
        
        if (foundProtected) {
            LI << "Protected event, skipping";
            // Send negative acknowledgment
            sendOKResponse(connId, to_hex(packed.id()), false, 
                         "blocked: event marked as protected");
            return;
        }
    }
    
    // STEP 3: Check for duplicate events
    {
        auto existing = lookupEventById(txn, packed.id());
        if (existing) {
            LI << "Duplicate event, skipping";
            // Send positive acknowledgment (duplicate)
            sendOKResponse(connId, to_hex(packed.id()), true, 
                         "duplicate: have this event");
            return;
        }
    }
    
    // STEP 4: Queue for writing to database
    output.emplace_back(MsgWriter{MsgWriter::AddEvent{
        connId,                    // Track which connection submitted
        std::move(ipAddr),         // Store source IP
        std::move(packedStr),      // Binary packed format (for DB storage)
        std::move(jsonStr)         // Normalized JSON (for relaying)
    }});
    
    // Note: OK response is sent later, AFTER database write is confirmed
}
```

---

## 4. Subscription Request (REQ) Flow

### Code Path: REQ Command → Filter Creation → Initial Query → Live Monitoring

**File 1: `/tmp/strfry/src/apps/relay/RelayIngester.cpp` (lines 125-132)**

```cpp
void RelayServer::ingesterProcessReq(lmdb::txn &txn, uint64_t connId, 
                                     const tao::json::value &arr) {
    // STEP 1: Validate REQ array structure
    // Array format: ["REQ", "subscription_id", {filter1}, {filter2}, ...]
    if (arr.get_array().size() < 2 + 1) 
        throw herr("arr too small");
    if (arr.get_array().size() > 2 + cfg().relay__maxReqFilterSize) 
        throw herr("arr too big");
    
    // STEP 2: Parse subscription ID and filter objects
    Subscription sub(
        connId, 
        jsonGetString(arr[1], "REQ subscription id was not a string"), 
        NostrFilterGroup(arr)  // Parses {filter1}, {filter2}, ... from arr[2..]
    );
    
    // STEP 3: Dispatch to ReqWorker thread for historical query
    tpReqWorker.dispatch(connId, MsgReqWorker{MsgReqWorker::NewSub{std::move(sub)}});
}
```

**File 2: `/tmp/strfry/src/apps/relay/RelayReqWorker.cpp` (lines 5-45)**

```cpp
void RelayServer::runReqWorker(ThreadPool<MsgReqWorker>::Thread &thr) {
    Decompressor decomp;
    QueryScheduler queries;
    
    // STEP 4: Define callback for matching events
    queries.onEvent = [&](lmdb::txn &txn, const auto &sub, uint64_t levId, 
                          std::string_view eventPayload){
        // Decompress event if needed, format JSON
        auto eventJson = decodeEventPayload(txn, decomp, eventPayload, nullptr, nullptr);
        
        // Send ["EVENT", "sub_id", event_json] to client
        sendEvent(sub.connId, sub.subId, eventJson);
    };
    
    // STEP 5: Define callback for query completion
    queries.onComplete = [&](lmdb::txn &, Subscription &sub){
        // Send ["EOSE", "sub_id"] - End Of Stored Events
        sendToConn(sub.connId, 
            tao::json::to_string(tao::json::value::array({ "EOSE", sub.subId.str() })));
        
        // STEP 6: Move subscription to ReqMonitor for live event delivery
        tpReqMonitor.dispatch(sub.connId, MsgReqMonitor{MsgReqMonitor::NewSub{std::move(sub)}});
    };
    
    while(1) {
        // STEP 7: Retrieve pending subscription requests
        auto newMsgs = queries.running.empty() 
            ? thr.inbox.pop_all()           // Block if idle
            : thr.inbox.pop_all_no_wait();  // Non-blocking if busy (queries running)
        
        auto txn = env.txn_ro();
        
        for (auto &newMsg : newMsgs) {
            if (auto msg = std::get_if<MsgReqWorker::NewSub>(&newMsg.msg)) {
                // STEP 8: Add subscription to query scheduler
                if (!queries.addSub(txn, std::move(msg->sub))) {
                    sendNoticeError(msg->connId, std::string("too many concurrent REQs"));
                }
                
                // STEP 9: Start processing the subscription
                // This will scan database and call onEvent for matches
                queries.process(txn);
            }
        }
        
        // STEP 10: Continue processing active subscriptions
        queries.process(txn);
        
        txn.abort();
    }
}
```

---

## 5. Event Broadcasting Flow

### Code Path: New Event → Multiple Subscribers → Batch Sending

**File: `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 286-299)**

```cpp
// This is the hot path for broadcasting events to subscribers

// STEP 1: Receive batch of event deliveries
else if (auto msg = std::get_if<MsgWebsocket::SendEventToBatch>(&newMsg.msg)) {
    // msg->list = vector of (connId, subId) pairs
    // msg->evJson = event JSON string (shared by all recipients)
    
    // STEP 2: Pre-allocate buffer for worst case
    tempBuf.reserve(13 + MAX_SUBID_SIZE + msg->evJson.size());
    
    // STEP 3: Construct frame template:
    // ["EVENT","<subId_placeholder>","event_json"]
    tempBuf.resize(10 + MAX_SUBID_SIZE);  // Reserve space for subId
    tempBuf += "\",";                      // Closing quote + comma
    tempBuf += msg->evJson;                // Event JSON
    tempBuf += "]";                        // Closing bracket
    
    // STEP 4: For each subscriber, write subId at correct offset
    for (auto &item : msg->list) {
        auto subIdSv = item.subId.sv();
        
        // STEP 5: Calculate write position for subId
        // MAX_SUBID_SIZE bytes allocated, so:
        // offset = MAX_SUBID_SIZE - actual_subId_length
        auto *p = tempBuf.data() + MAX_SUBID_SIZE - subIdSv.size();
        
        // STEP 6: Write frame header with variable-length subId
        memcpy(p, "[\"EVENT\",\"", 10);              // Frame prefix
        memcpy(p + 10, subIdSv.data(), subIdSv.size()); // SubId
        
        // STEP 7: Send to connection (compression handled by uWebSockets)
        doSend(item.connId, 
               std::string_view(p, 13 + subIdSv.size() + msg->evJson.size()), 
               uWS::OpCode::TEXT);
    }
}

// Key Optimization:
// - Event JSON serialized once (not per subscriber)
// - Buffer reused (not allocated per send)
// - Variable-length subId handled via pointer arithmetic
// - Result: O(n) sends with O(1) allocations and single JSON serialization
```

**Performance Impact:**
```
Without batching:
  - Serialize event JSON per subscriber: O(evJson.size() * numSubs)
  - Allocate frame buffer per subscriber: O(numSubs) allocations

With batching:
  - Serialize event JSON once: O(evJson.size())
  - Reuse single buffer: 1 allocation
  - Pointer arithmetic for variable subId: O(numSubs) cheap pointer ops
```

---

## 6. Connection Disconnection Flow

### Code Path: Disconnect Event → Statistics → Cleanup → Thread Notification

**File: `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 229-254)**

```cpp
hubGroup->onDisconnection([&](uWS::WebSocket<uWS::SERVER> *ws, 
                              int code, 
                              char *message, 
                              size_t length) {
    auto *c = (Connection*)ws->getUserData();
    uint64_t connId = c->connId;
    
    // STEP 1: Calculate compression effectiveness ratios
    // (shows if compression actually helped)
    auto upComp = renderPercent(1.0 - (double)c->stats.bytesUpCompressed / c->stats.bytesUp);
    auto downComp = renderPercent(1.0 - (double)c->stats.bytesDownCompressed / c->stats.bytesDown);
    
    // STEP 2: Log disconnection with detailed statistics
    LI << "[" << connId << "] Disconnect from " << renderIP(c->ipAddr)
       << " (" << code << "/" << (message ? std::string_view(message, length) : "-") << ")"
       << " UP: " << renderSize(c->stats.bytesUp) << " (" << upComp << " compressed)"
       << " DN: " << renderSize(c->stats.bytesDown) << " (" << downComp << " compressed)";
    
    // STEP 3: Notify ingester thread of disconnection
    // This message will be propagated to all worker threads
    tpIngester.dispatch(connId, MsgIngester{MsgIngester::CloseConn{connId}});
    
    // STEP 4: Remove from active connections map
    connIdToConnection.erase(connId);
    
    // STEP 5: Deallocate connection metadata
    delete c;
    
    // STEP 6: Handle graceful shutdown scenario
    if (gracefulShutdown) {
        LI << "Graceful shutdown in progress: " << connIdToConnection.size() 
           << " connections remaining";
        // Once all connections close, exit gracefully
        if (connIdToConnection.size() == 0) {
            LW << "All connections closed, shutting down";
            ::exit(0);
        }
    }
});

// From RelayIngester.cpp, the CloseConn message is then distributed:
// STEP 7: In ingester thread:
else if (auto msg = std::get_if<MsgIngester::CloseConn>(&newMsg.msg)) {
    auto connId = msg->connId;
    // STEP 8: Notify all worker threads
    tpWriter.dispatch(connId, MsgWriter{MsgWriter::CloseConn{connId}});
    tpReqWorker.dispatch(connId, MsgReqWorker{MsgReqWorker::CloseConn{connId}});
    tpNegentropy.dispatch(connId, MsgNegentropy{MsgNegentropy::CloseConn{connId}});
}
```

---

## 7. Thread Pool Message Dispatch

### Code Pattern: Deterministic Thread Assignment

**File: `/tmp/strfry/src/ThreadPool.h` (lines 42-50)**

```cpp
template <typename M>
struct ThreadPool {
    std::deque<Thread> pool;  // Multiple worker threads
    
    // Deterministic dispatch: same connId always goes to same thread
    void dispatch(uint64_t key, M &&msg) {
        // STEP 1: Compute thread ID from key
        uint64_t who = key % numThreads;  // Hash modulo
        
        // STEP 2: Push to that thread's inbox (lock-free or low-contention)
        pool[who].inbox.push_move(std::move(msg));
        
        // Benefit: Reduces lock contention and improves cache locality
    }
    
    // Batch dispatch multiple messages to same thread
    void dispatchMulti(uint64_t key, std::vector<M> &msgs) {
        uint64_t who = key % numThreads;
        
        // STEP 1: Atomic operation to push all messages
        pool[who].inbox.push_move_all(msgs);
        
        // Benefit: Single lock acquisition for multiple messages
    }
};

// Usage example:
tpIngester.dispatch(connId, MsgIngester{MsgIngester::ClientMessage{...}});
// If connId=42 and numThreads=3:
// thread_id = 42 % 3 = 0
// Message goes to ingester thread 0
```

---

## 8. Message Type Dispatch Pattern

### Code Pattern: std::variant for Type-Safe Routing

**File: `/tmp/strfry/src/apps/relay/RelayWebsocket.cpp` (lines 281-305)**

```cpp
// STEP 1: Retrieve all pending messages from inbox
auto newMsgs = thr.inbox.pop_all_no_wait();

// STEP 2: For each message, determine its type and handle accordingly
for (auto &newMsg : newMsgs) {
    // std::variant is like a type-safe union
    // std::get_if checks if it's that type and returns pointer if yes
    
    if (auto msg = std::get_if<MsgWebsocket::Send>(&newMsg.msg)) {
        // It's a Send message: text message to single connection
        doSend(msg->connId, msg->payload, uWS::OpCode::TEXT);
    } 
    else if (auto msg = std::get_if<MsgWebsocket::SendBinary>(&newMsg.msg)) {
        // It's a SendBinary message: binary frame to single connection
        doSend(msg->connId, msg->payload, uWS::OpCode::BINARY);
    } 
    else if (auto msg = std::get_if<MsgWebsocket::SendEventToBatch>(&newMsg.msg)) {
        // It's a SendEventToBatch message: same event to multiple subscribers
        // (See Section 5 for detailed implementation)
        // ... batch sending code ...
    } 
    else if (std::get_if<MsgWebsocket::GracefulShutdown>(&newMsg.msg)) {
        // It's a GracefulShutdown message: begin shutdown
        gracefulShutdown = true;
        hubGroup->stopListening();
    }
}

// Key Benefit: Type dispatch without virtual functions
// - Compiler generates optimal branching code
// - All data inline in variant, no heap allocation
// - Zero runtime polymorphism overhead
```

---

## 9. Subscription Lifecycle Summary

```
                    Client sends REQ
                           |
                           v
                    Ingester thread
                           |
                           v
                      REQ parsing ----> ["REQ", "subid", {filter1}, {filter2}]
                           |
                           v
                      ReqWorker thread
                           |
                    +------+------+
                    |             |
                    v             v
              DB Query       Historical events
                    |             |
                    |      ["EVENT", "subid", event1]
                    |      ["EVENT", "subid", event2]
                    |             |
                    +------+------+
                           |
                           v
                    Send ["EOSE", "subid"]
                           |
                           v
                    ReqMonitor thread
                           |
                    +------+------+
                    |             |
                    v             v
              New events       Live matching
              from DB          subscriptions
                    |             |
              ["EVENT",      ActiveMonitors
              "subid",       Indexed by:
              event]          - id
                    |          - author
                    |          - kind
                    |          - tags
                    |          - (unrestricted)
                    |             |
                    +------+------+
                           |
                    Match against filters
                           |
                           v
                    WebSocket thread
                           |
                    +------+------+
                    |             |
                    v             v
              SendEventToBatch
              (batch broadcasts)
                    |
                    v
              Client receives events
```

---

## 10. Error Handling Flow

### Code Pattern: Exception Propagation

**File: `/tmp/strfry/src/apps/relay/RelayIngester.cpp` (lines 16-73)**

```cpp
for (auto &newMsg : newMsgs) {
    if (auto msg = std::get_if<MsgIngester::ClientMessage>(&newMsg.msg)) {
        try {
            // STEP 1: Attempt to parse JSON
            if (msg->payload.starts_with('[')) {
                auto payload = tao::json::from_string(msg->payload);
                
                auto &arr = jsonGetArray(payload, "message is not an array");
                
                if (arr.size() < 2) 
                    throw herr("too few array elements");
                
                auto &cmd = jsonGetString(arr[0], "first element not a command");
                
                if (cmd == "EVENT") {
                    // STEP 2: Process event (may throw)
                    try {
                        ingesterProcessEvent(txn, msg->connId, msg->ipAddr, 
                                           secpCtx, arr[1], writerMsgs);
                    } catch (std::exception &e) {
                        // STEP 3a: Event-specific error handling
                        // Send OK response with false flag and error message
                        sendOKResponse(msg->connId, 
                            arr[1].is_object() && arr[1].at("id").is_string() 
                                ? arr[1].at("id").get_string() : "?",
                            false, 
                            std::string("invalid: ") + e.what());
                        if (cfg().relay__logging__invalidEvents) 
                            LI << "Rejected invalid event: " << e.what();
                    }
                } 
                else if (cmd == "REQ") {
                    // STEP 2: Process REQ (may throw)
                    try {
                        ingesterProcessReq(txn, msg->connId, arr);
                    } catch (std::exception &e) {
                        // STEP 3b: REQ-specific error handling
                        // Send NOTICE message with error
                        sendNoticeError(msg->connId, 
                            std::string("bad req: ") + e.what());
                    }
                }
            }
        } catch (std::exception &e) {
            // STEP 4: Catch-all for JSON parsing errors
            sendNoticeError(msg->connId, std::string("bad msg: ") + e.what());
        }
    }
}
```

**Error Handling Strategy:**
1. **Try-catch at command level** - EVENT, REQ, CLOSE each have their own
2. **Specific error responses** - OK (false) for EVENT, NOTICE for others
3. **Logging** - Configurable debug logging per message type
4. **Graceful degradation** - One bad message doesn't affect others

---

## Summary: Complete Message Lifecycle

```
1. RECEPTION (WebSocket Thread)
   Client sends ["EVENT", {...}]
   ↓
   onMessage2() callback triggers
   ↓
   Stats recorded (bytes down/compressed)
   ↓
   Dispatched to Ingester thread (via connId hash)

2. PARSING (Ingester Thread)
   JSON parsed from UTF-8 bytes
   ↓
   Command extracted (first array element)
   ↓
   Routed to command handler (EVENT/REQ/CLOSE/NEG-*)

3. VALIDATION (Ingester Thread for EVENT)
   Event structure validated
   ↓
   Schnorr signature verified (secp256k1)
   ↓
   Protected events rejected
   ↓
   Duplicates detected and skipped

4. QUEUING (Ingester Thread)
   Validated events batched
   ↓
   Sent to Writer thread (via dispatchMulti)

5. DATABASE (Writer Thread)
   Event written to LMDB
   ↓
   New subscribers notified via ReqMonitor
   ↓
   OK response sent back to client

6. DISTRIBUTION (ReqMonitor & WebSocket Threads)
   ActiveMonitors checked for matching subscriptions
   ↓
   Matching subscriptions collected into RecipientList
   ↓
   Sent to WebSocket thread as SendEventToBatch
   ↓
   Buffer reused, frame constructed with variable subId offset
   ↓
   Sent to each subscriber (compressed if supported)

7. ACKNOWLEDGMENT (WebSocket Thread)
   ["OK", event_id, true/false, message]
   ↓
   Sent back to originating connection
```

