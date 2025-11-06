# Critical Publisher Bug Fix

## Issue Discovered

Events were being published successfully but **never delivered to subscribers**. The test showed:
- Publisher logs: "saved event"
- Subscriber logs: No events received
- No delivery timeouts or errors

## Root Cause

The `Subscription` struct in `app/publisher.go` was missing the `Receiver` field:

```go
// BEFORE - Missing Receiver field
type Subscription struct {
	remote       string
	AuthedPubkey []byte
	*filter.S
}
```

This meant:
1. Subscriptions were registered with receiver channels in `handle-req.go`
2. Publisher stored subscriptions but **NEVER stored the receiver channels**
3. Consumer goroutines waited on receiver channels
4. Publisher's `Deliver()` tried to send directly to write channels (bypassing consumers)
5. Events never reached the consumer goroutines → never delivered to clients

## The Architecture (How it Should Work)

```
Event Published
    ↓
Publisher.Deliver() matches filters
    ↓
Sends event to Subscription.Receiver channel ← THIS WAS MISSING
    ↓
Consumer goroutine reads from Receiver
    ↓
Formats as EVENT envelope
    ↓
Sends to write channel
    ↓
Write worker sends to client
```

## The Fix

### 1. Add Receiver Field to Subscription Struct

**File**: `app/publisher.go:29-34`

```go
// AFTER - With Receiver field
type Subscription struct {
	remote       string
	AuthedPubkey []byte
	Receiver     event.C // Channel for delivering events to this subscription
	*filter.S
}
```

### 2. Store Receiver When Registering Subscription

**File**: `app/publisher.go:125,130`

```go
// BEFORE
subs[m.Id] = Subscription{
	S: m.Filters, remote: m.remote, AuthedPubkey: m.AuthedPubkey,
}

// AFTER
subs[m.Id] = Subscription{
	S: m.Filters, remote: m.remote, AuthedPubkey: m.AuthedPubkey, Receiver: m.Receiver,
}
```

### 3. Send Events to Receiver Channel (Not Write Channel)

**File**: `app/publisher.go:242-266`

```go
// BEFORE - Tried to format and send directly to write channel
var res *eventenvelope.Result
if res, err = eventenvelope.NewResultWith(d.id, ev); chk.E(err) {
	// ...
}
msgData := res.Marshal(nil)
writeChan <- publish.WriteRequest{Data: msgData, MsgType: websocket.TextMessage}

// AFTER - Send raw event to receiver channel
if d.sub.Receiver == nil {
	log.E.F("subscription %s has nil receiver channel", d.id)
	continue
}

select {
case d.sub.Receiver <- ev:
	log.D.F("subscription delivery QUEUED: event=%s to=%s sub=%s",
		hex.Enc(ev.ID), d.sub.remote, d.id)
case <-time.After(DefaultWriteTimeout):
	log.E.F("subscription delivery TIMEOUT: event=%s to=%s sub=%s",
		hex.Enc(ev.ID), d.sub.remote, d.id)
}
```

## Why This Pattern Matters (khatru Architecture)

The khatru pattern uses **per-subscription consumer goroutines** for good reasons:

1. **Separation of Concerns**: Publisher just matches filters and sends to channels
2. **Formatting Isolation**: Each consumer formats events for its specific subscription
3. **Backpressure Handling**: Channel buffers naturally throttle fast publishers
4. **Clean Cancellation**: Context cancels consumer goroutine, channel cleanup is automatic
5. **No Lock Contention**: Publisher doesn't hold locks during I/O operations

## Files Modified

| File | Lines | Change |
|------|-------|--------|
| `app/publisher.go` | 32 | Add `Receiver event.C` field to Subscription |
| `app/publisher.go` | 125, 130 | Store Receiver when registering |
| `app/publisher.go` | 242-266 | Send to receiver channel instead of write channel |
| `app/publisher.go` | 3-19 | Remove unused imports (chk, eventenvelope) |

## Testing

```bash
# Terminal 1: Start relay
./orly

# Terminal 2: Subscribe
websocat ws://localhost:3334 <<< '["REQ","test",{"kinds":[1]}]'

# Terminal 3: Publish event
websocat ws://localhost:3334 <<< '["EVENT",{"kind":1,"content":"test",...}]'
```

**Expected**: Terminal 2 receives the event immediately

## Impact

**Before:**
- ❌ No events delivered to subscribers
- ❌ Publisher tried to bypass consumer goroutines
- ❌ Consumer goroutines blocked forever waiting on receiver channels
- ❌ Architecture didn't follow khatru pattern

**After:**
- ✅ Events delivered via receiver channels
- ✅ Consumer goroutines receive and format events
- ✅ Full khatru pattern implementation
- ✅ Proper separation of concerns

## Summary

The subscription stability fixes in the previous work correctly implemented:
- Per-subscription consumer goroutines ✅
- Independent contexts ✅
- Concurrent message processing ✅

But the publisher was never connected to the consumer goroutines! This fix completes the implementation by:
- Storing receiver channels in subscriptions ✅
- Sending events to receiver channels ✅
- Letting consumers handle formatting and delivery ✅

**Result**: Events now flow correctly from publisher → receiver channel → consumer → client
