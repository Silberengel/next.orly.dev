package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/crypto/p256k"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/protocol/ws"
)

func main() {
	var err error
	url := flag.String("url", "ws://127.0.0.1:3334", "relay websocket URL")
	timeout := flag.Duration("timeout", 20*time.Second, "publish timeout")
	flag.Parse()

	// Minimal client that publishes a single kind 4678 event and reports OK/err
	var rl *ws.Client
	if rl, err = ws.RelayConnect(context.Background(), *url); chk.E(err) {
		log.E.F("connect error: %v", err)
		return
	}
	defer rl.Close()

	signer := &p256k.Signer{}
	if err = signer.Generate(); chk.E(err) {
		log.E.F("signer generate error: %v", err)
		return
	}

	ev := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      kind.K{K: 4678}.K, // arbitrary custom kind
		Tags:      tag.NewS(),
		Content:   []byte("policy test: expect rejection"),
	}
	if err = ev.Sign(signer); chk.E(err) {
		log.E.F("sign error: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err = rl.Publish(ctx, ev); err != nil {
		// Expected path if policy rejects: client returns error with reason (from OK false)
		fmt.Println("policy reject:", err)
		return
	}

	log.I.Ln("publish result: accepted")
	fmt.Println("ACCEPT")
}
