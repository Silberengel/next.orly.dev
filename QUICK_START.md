# Quick Start - Subscription Stability Testing

## TL;DR

Subscriptions were dropping. Now they're fixed. Here's how to verify:

## 1. Build Everything

```bash
go build -o orly
go build -o subscription-test ./cmd/subscription-test
```

## 2. Test It

```bash
# Terminal 1: Start relay
./orly

# Terminal 2: Run test
./subscription-test -url ws://localhost:3334 -duration 60 -v
```

## 3. Expected Output

```
✓ Connected
✓ Received EOSE - subscription is active

Waiting for real-time events...

[EVENT #1] id=abc123... kind=1 created=1234567890
[EVENT #2] id=def456... kind=1 created=1234567891
...

[STATUS] Elapsed: 30s/60s | Events: 15 | Last event: 2s ago
[STATUS] Elapsed: 60s/60s | Events: 30 | Last event: 1s ago

✓ TEST PASSED - Subscription remained stable
```

## What Changed?

**Before:** Subscriptions dropped after ~30-60 seconds
**After:** Subscriptions stay active indefinitely

## Key Files Modified

- `app/listener.go` - Added subscription tracking
- `app/handle-req.go` - Consumer goroutines per subscription
- `app/handle-close.go` - Proper cleanup
- `app/handle-websocket.go` - Cancel all subs on disconnect

## Why Did It Break?

Receiver channels were created but never consumed → filled up → publisher timeout → subscription removed

## How Is It Fixed?

Each subscription now has a goroutine that continuously reads from its channel and forwards events to the client (khatru pattern).

## More Info

- **Technical details:** [SUBSCRIPTION_STABILITY_FIXES.md](SUBSCRIPTION_STABILITY_FIXES.md)
- **Full testing guide:** [TESTING_GUIDE.md](TESTING_GUIDE.md)
- **Complete summary:** [SUMMARY.md](SUMMARY.md)

## Questions?

```bash
./subscription-test -h     # Test tool help
export ORLY_LOG_LEVEL=debug  # Enable debug logs
```

That's it! 🎉
