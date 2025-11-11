package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/interfaces/signer/p8k"
	"next.orly.dev/pkg/protocol/ws"
)

func main() {
	var err error
	url := flag.String("url", "ws://127.0.0.1:3334", "relay websocket URL")
	timeout := flag.Duration("timeout", 20*time.Second, "operation timeout")
	testType := flag.String("type", "event", "test type: 'event' for write control, 'req' for read control, 'both' for both")
	eventKind := flag.Int("kind", 4678, "event kind to test")
	flag.Parse()

	// Connect to relay
	var rl *ws.Client
	if rl, err = ws.RelayConnect(context.Background(), *url); chk.E(err) {
		log.E.F("connect error: %v", err)
		return
	}
	defer rl.Close()

	// Create signer
	var signer *p8k.Signer
	if signer, err = p8k.New(); chk.E(err) {
		log.E.F("signer create error: %v", err)
		return
	}
	if err = signer.Generate(); chk.E(err) {
		log.E.F("signer generate error: %v", err)
		return
	}

	// Perform tests based on type
	switch *testType {
	case "event":
		testEventWrite(rl, signer, *eventKind, *timeout)
	case "req":
		testReqRead(rl, signer, *eventKind, *timeout)
	case "both":
		log.I.Ln("Testing EVENT (write control)...")
		testEventWrite(rl, signer, *eventKind, *timeout)
		log.I.Ln("\nTesting REQ (read control)...")
		testReqRead(rl, signer, *eventKind, *timeout)
	default:
		log.E.F("invalid test type: %s (must be 'event', 'req', or 'both')", *testType)
	}
}

func testEventWrite(rl *ws.Client, signer *p8k.Signer, eventKind int, timeout time.Duration) {
	ev := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      uint16(eventKind),
		Tags:      tag.NewS(),
		Content:   []byte("policy test: expect rejection for write"),
	}
	if err := ev.Sign(signer); chk.E(err) {
		log.E.F("sign error: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := rl.Publish(ctx, ev); err != nil {
		// Expected path if policy rejects: client returns error with reason (from OK false)
		fmt.Println("EVENT policy reject:", err)
		return
	}

	log.I.Ln("EVENT publish result: accepted")
	fmt.Println("EVENT ACCEPT")
}

func testReqRead(rl *ws.Client, signer *p8k.Signer, eventKind int, timeout time.Duration) {
	// First, publish a test event to the relay that we'll try to query
	testEvent := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      uint16(eventKind),
		Tags:      tag.NewS(),
		Content:   []byte("policy test: event for read control test"),
	}
	if err := testEvent.Sign(signer); chk.E(err) {
		log.E.F("sign error: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Try to publish the test event first (ignore errors if policy rejects)
	_ = rl.Publish(ctx, testEvent)
	log.I.F("published test event kind %d for read testing", eventKind)

	// Now try to query for events of this kind
	limit := uint(10)
	f := &filter.F{
		Kinds: kind.FromIntSlice([]int{eventKind}),
		Limit: &limit,
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
	defer cancel2()

	events, err := rl.QuerySync(ctx2, f)
	if chk.E(err) {
		log.E.F("query error: %v", err)
		fmt.Println("REQ query error:", err)
		return
	}

	// Check if we got the expected events
	if len(events) == 0 {
		// Could mean policy filtered it out, or it wasn't stored
		fmt.Println("REQ policy reject: no events returned (filtered by read policy)")
		log.I.F("REQ result: no events of kind %d returned (policy filtered or not stored)", eventKind)
		return
	}

	// Events were returned - read access allowed
	fmt.Printf("REQ ACCEPT: %d events returned\n", len(events))
	log.I.F("REQ result: %d events of kind %d returned", len(events), eventKind)
}
