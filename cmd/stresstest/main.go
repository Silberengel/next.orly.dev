package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lol.mleku.dev/log"
	"next.orly.dev/pkg/crypto/p256k"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/event/examples"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/timestamp"
	"next.orly.dev/pkg/protocol/ws"
)

// randomHex returns a hex-encoded string of n random bytes (2n hex chars)
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.Enc(b)
}

func makeEvent(rng *rand.Rand, signer *p256k.Signer) (*event.E, error) {
	ev := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      kind.TextNote.K,
		Tags:      tag.NewS(),
		Content:   []byte(fmt.Sprintf("stresstest %d", rng.Int63())),
	}

	// Random number of p-tags up to 100
	nPTags := rng.Intn(101) // 0..100 inclusive
	for i := 0; i < nPTags; i++ {
		// random 32-byte pubkey in hex (64 chars)
		phex := randomHex(32)
		ev.Tags.Append(tag.NewFromAny("p", phex))
	}

	// Sign and verify to ensure pubkey, id and signature are coherent
	if err := ev.Sign(signer); err != nil {
		return nil, err
	}
	if ok, err := ev.Verify(); err != nil || !ok {
		return nil, fmt.Errorf("event signature verification failed: %v", err)
	}
	return ev, nil
}

type RelayConn struct {
	mu     sync.RWMutex
	client *ws.Client
	url    string
}

type CacheIndex struct {
	events  []*event.E
	ids     [][]byte
	authors [][]byte
	times   []int64
	tags    map[byte][][]byte // single-letter tag -> list of values
}

func (rc *RelayConn) Get() *ws.Client {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.client
}

func (rc *RelayConn) Reconnect(ctx context.Context) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.client != nil {
		_ = rc.client.Close()
	}
	c, err := ws.RelayConnect(ctx, rc.url)
	if err != nil {
		return err
	}
	rc.client = c
	return nil
}

// loadCacheAndIndex parses examples.Cache (JSONL of events) and builds an index
func loadCacheAndIndex() (*CacheIndex, error) {
	scanner := bufio.NewScanner(bytes.NewReader(examples.Cache))
	idx := &CacheIndex{tags: make(map[byte][][]byte)}
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		ev := event.New()
		rem, err := ev.Unmarshal(line)
		_ = rem
		if err != nil {
			// skip malformed lines
			continue
		}
		idx.events = append(idx.events, ev)
		// collect fields
		if len(ev.ID) > 0 {
			idx.ids = append(idx.ids, append([]byte(nil), ev.ID...))
		}
		if len(ev.Pubkey) > 0 {
			idx.authors = append(idx.authors, append([]byte(nil), ev.Pubkey...))
		}
		idx.times = append(idx.times, ev.CreatedAt)
		if ev.Tags != nil {
			for _, tg := range *ev.Tags {
				if tg == nil || tg.Len() < 2 {
					continue
				}
				k := tg.Key()
				if len(k) != 1 {
					continue // only single-letter keys per requirement
				}
				key := k[0]
				for _, v := range tg.T[1:] {
					idx.tags[key] = append(
						idx.tags[key], append([]byte(nil), v...),
					)
				}
			}
		}
	}
	return idx, nil
}

// publishCacheEvents uploads all cache events to the relay, waiting for OKs
func publishCacheEvents(
	ctx context.Context, rc *RelayConn, idx *CacheIndex,
	publishTimeout time.Duration,
) (okCount int) {
	// Use an index-based loop so we can truly retry the same event on transient errors
	for i := 0; i < len(idx.events); i++ {
		// allow cancellation
		select {
		case <-ctx.Done():
			return okCount
		default:
		}
		ev := idx.events[i]
		client := rc.Get()
		if client == nil {
			_ = rc.Reconnect(ctx)
			// retry same index
			i--
			continue
		}
		// Create per-publish timeout context and add diagnostics to understand cancellations
		pubCtx, cancel := context.WithTimeout(ctx, publishTimeout)
		if dl, ok := pubCtx.Deadline(); ok {
			log.T.F(
				"cache publish %d/%d: deadline=%s now=%s remain=%s",
				i+1, len(idx.events), dl.Format(time.RFC3339Nano),
				time.Now().Format(time.RFC3339Nano),
				time.Until(dl).Truncate(time.Millisecond),
			)
		}
		err := client.Publish(pubCtx, ev)
		log.I.F("event: %s", ev.Serialize())
		// it's safe to cancel our per-publish context after Publish returns
		cancel()
		if err != nil {
			log.E.F("cache publish error: %v (ctxErr=%v)", err, pubCtx.Err())
			errStr := err.Error()
			if strings.Contains(errStr, "connection closed") ||
				strings.Contains(errStr, "context deadline exceeded") ||
				strings.Contains(errStr, "given up waiting for an OK") {
				_ = rc.Reconnect(ctx)
				// retry this event by decrementing i so the for-loop will attempt it again
				i--
				continue
			}
			// small backoff and move on to next event on non-transient errors
			time.Sleep(50 * time.Millisecond)
			continue
		}
		okCount++
	}
	return okCount
}

// buildRandomFilter builds a filter combining random subsets of id, author, timestamp, and a single-letter tag value.
func buildRandomFilter(idx *CacheIndex, rng *rand.Rand, mask int) *filter.F {
	// pick a random base event as anchor for fields
	i := rng.Intn(len(idx.events))
	ev := idx.events[i]
	f := filter.New()
	// clear defaults we don't set
	f.Kinds = kind.NewS() // we don't constrain kinds
	// include fields based on mask bits: 1=id, 2=author, 4=timestamp, 8=tag
	if mask&1 != 0 {
		f.Ids.T = append(f.Ids.T, append([]byte(nil), ev.ID...))
	}
	if mask&2 != 0 {
		f.Authors.T = append(f.Authors.T, append([]byte(nil), ev.Pubkey...))
	}
	if mask&4 != 0 {
		// use a tight window around the event timestamp (exact match)
		f.Since = timestamp.FromUnix(ev.CreatedAt)
		f.Until = timestamp.FromUnix(ev.CreatedAt)
	}
	if mask&8 != 0 {
		// choose a random single-letter tag from this event if present; fallback to global index
		var key byte
		var val []byte
		chosen := false
		if ev.Tags != nil {
			for _, tg := range *ev.Tags {
				if tg == nil || tg.Len() < 2 {
					continue
				}
				k := tg.Key()
				if len(k) == 1 {
					key = k[0]
					vv := tg.T[1:]
					val = vv[rng.Intn(len(vv))]
					chosen = true
					break
				}
			}
		}
		if !chosen && len(idx.tags) > 0 {
			// pick a random entry from global tags map
			keys := make([]byte, 0, len(idx.tags))
			for k := range idx.tags {
				keys = append(keys, k)
			}
			key = keys[rng.Intn(len(keys))]
			vals := idx.tags[key]
			val = vals[rng.Intn(len(vals))]
		}
		if key != 0 && len(val) > 0 {
			f.Tags.Append(tag.NewFromBytesSlice([]byte{key}, val))
		}
	}
	return f
}

func publisherWorker(
	ctx context.Context, rc *RelayConn, id int, stats *uint64,
	publishTimeout time.Duration,
) {
	// Unique RNG per worker
	src := rand.NewSource(time.Now().UnixNano() ^ int64(id<<16))
	rng := rand.New(src)
	// Generate and reuse signing key per worker
	signer := &p256k.Signer{}
	if err := signer.Generate(); err != nil {
		log.E.F("worker %d: signer generate error: %v", id, err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ev, err := makeEvent(rng, signer)
		if err != nil {
			log.E.F("worker %d: makeEvent error: %v", id, err)
			return
		}

		// Publish waits for OK (ws.Client.Publish does the waiting)
		client := rc.Get()
		if client == nil {
			_ = rc.Reconnect(ctx)
			continue
		}
		// per-publish timeout context
		pubCtx, cancel := context.WithTimeout(ctx, publishTimeout)
		if err := client.Publish(pubCtx, ev); err != nil {
			cancel()
			log.E.F("worker %d: publish error: %v", id, err)
			errStr := err.Error()
			if strings.Contains(
				errStr, "connection closed",
			) || strings.Contains(
				errStr, "context deadline exceeded",
			) || strings.Contains(errStr, "given up waiting for an OK") {
				for attempt := 0; attempt < 5; attempt++ {
					if ctx.Err() != nil {
						return
					}
					if err := rc.Reconnect(ctx); err == nil {
						log.I.F("worker %d: reconnected to %s", id, rc.url)
						break
					}
					select {
					case <-time.After(200 * time.Millisecond):
					case <-ctx.Done():
						return
					}
				}
			}
			// back off briefly on error to avoid tight loop if relay misbehaves
			select {
			case <-time.After(100 * time.Millisecond):
			case <-ctx.Done():
				return
			}
			continue
		}
		cancel()

		atomic.AddUint64(stats, 1)

		// Randomly fluctuate pacing: small random sleep 0..50ms plus occasional longer jitter
		sleep := time.Duration(rng.Intn(50)) * time.Millisecond
		if rng.Intn(10) == 0 { // 10% chance add extra 100..400ms
			sleep += time.Duration(100+rng.Intn(300)) * time.Millisecond
		}
		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return
		}
	}
}

func queryWorker(
	ctx context.Context, rc *RelayConn, idx *CacheIndex, id int,
	queries, results *uint64, subTimeout time.Duration,
	minInterval, maxInterval time.Duration,
) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano() ^ int64(id<<24)))
	mask := 1
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if len(idx.events) == 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		f := buildRandomFilter(idx, rng, mask)
		mask++
		if mask > 15 { // all combinations of 4 criteria (excluding 0)
			mask = 1
		}
		client := rc.Get()
		if client == nil {
			_ = rc.Reconnect(ctx)
			continue
		}
		ff := filter.S{f}
		sCtx, cancel := context.WithTimeout(ctx, subTimeout)
		sub, err := client.Subscribe(
			sCtx, &ff, ws.WithLabel("stresstest-query"),
		)
		if err != nil {
			cancel()
			// reconnect on connection issues
			errStr := err.Error()
			if strings.Contains(errStr, "connection closed") {
				_ = rc.Reconnect(ctx)
			}
			continue
		}
		atomic.AddUint64(queries, 1)
		// read until EOSE or timeout
		innerDone := false
		for !innerDone {
			select {
			case <-sCtx.Done():
				innerDone = true
			case <-sub.EndOfStoredEvents:
				innerDone = true
			case ev, ok := <-sub.Events:
				if !ok {
					innerDone = true
					break
				}
				if ev != nil {
					atomic.AddUint64(results, 1)
				}
			}
		}
		sub.Unsub()
		cancel()
		// wait a random interval between queries
		interval := minInterval
		if maxInterval > minInterval {
			delta := rng.Int63n(int64(maxInterval - minInterval))
			interval += time.Duration(delta)
		}
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return
		}
	}
}

func startReader(ctx context.Context, rl *ws.Client, received *uint64) error {
	// Broad filter: subscribe to text notes since now-5m to catch our own writes
	f := filter.New()
	f.Kinds = kind.NewS(kind.TextNote)
	// We don't set authors to ensure we read all text notes coming in
	ff := filter.S{f}
	sub, err := rl.Subscribe(ctx, &ff, ws.WithLabel("stresstest-reader"))
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-sub.Events:
				if !ok {
					return
				}
				if ev != nil {
					atomic.AddUint64(received, 1)
				}
			}
		}
	}()

	return nil
}

func main() {
	var (
		address        string
		port           int
		workers        int
		duration       time.Duration
		publishTimeout time.Duration
		queryWorkers   int
		queryTimeout   time.Duration
		queryMinInt    time.Duration
		queryMaxInt    time.Duration
		skipCache      bool
	)

	flag.StringVar(
		&address, "address", "localhost", "relay address (host or IP)",
	)
	flag.IntVar(&port, "port", 3334, "relay port")
	flag.IntVar(
		&workers, "workers", 8, "number of concurrent publisher workers",
	)
	flag.DurationVar(
		&duration, "duration", 60*time.Second,
		"how long to run the stress test",
	)
	flag.DurationVar(
		&publishTimeout, "publish-timeout", 15*time.Second,
		"timeout waiting for OK per publish",
	)
	flag.IntVar(
		&queryWorkers, "query-workers", 4, "number of concurrent query workers",
	)
	flag.DurationVar(
		&queryTimeout, "query-timeout", 3*time.Second,
		"subscription timeout for queries",
	)
	flag.DurationVar(
		&queryMinInt, "query-min-interval", 50*time.Millisecond,
		"minimum interval between queries per worker",
	)
	flag.DurationVar(
		&queryMaxInt, "query-max-interval", 300*time.Millisecond,
		"maximum interval between queries per worker",
	)
	flag.BoolVar(
		&skipCache, "skip-cache", false,
		"skip uploading examples.Cache before running",
	)
	flag.Parse()

	relayURL := fmt.Sprintf("ws://%s:%d", address, port)
	log.I.F("stresstest: connecting to %s", relayURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		select {
		case <-sigc:
			log.I.Ln("interrupt received, shutting down...")
			cancel()
		case <-ctx.Done():
		}
	}()

	rl, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		log.E.F("failed to connect to relay %s: %v", relayURL, err)
		os.Exit(1)
	}
	defer rl.Close()

	rc := &RelayConn{client: rl, url: relayURL}

	// Load and publish cache events first (unless skipped)
	idx, err := loadCacheAndIndex()
	if err != nil {
		log.E.F("failed to load examples.Cache: %v", err)
	}
	cachePublished := 0
	if !skipCache && idx != nil && len(idx.events) > 0 {
		log.I.F("uploading %d events from examples.Cache...", len(idx.events))
		cachePublished = publishCacheEvents(ctx, rc, idx, publishTimeout)
		log.I.F("uploaded %d/%d cache events", cachePublished, len(idx.events))
	}

	var pubOK uint64
	var recvCount uint64
	var qCount uint64
	var qResults uint64

	if err := startReader(ctx, rl, &recvCount); err != nil {
		log.E.F("reader subscribe error: %v", err)
		// continue anyway, we can still write
	}

	wg := sync.WaitGroup{}
	// Start publisher workers
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			publisherWorker(ctx, rc, i, &pubOK, publishTimeout)
		}()
	}
	// Start query workers
	if idx != nil && len(idx.events) > 0 && queryWorkers > 0 {
		wg.Add(queryWorkers)
		for i := 0; i < queryWorkers; i++ {
			i := i
			go func() {
				defer wg.Done()
				queryWorker(
					ctx, rc, idx, i, &qCount, &qResults, queryTimeout,
					queryMinInt, queryMaxInt,
				)
			}()
		}
	}

	// Timer for duration and periodic stats
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	end := time.NewTimer(duration)
	start := time.Now()

loop:
	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(start).Seconds()
			p := atomic.LoadUint64(&pubOK)
			r := atomic.LoadUint64(&recvCount)
			qc := atomic.LoadUint64(&qCount)
			qr := atomic.LoadUint64(&qResults)
			log.I.F(
				"elapsed=%.1fs published_ok=%d (%.0f/s) received=%d cache_published=%d queries=%d results=%d",
				elapsed, p, float64(p)/elapsed, r, cachePublished, qc, qr,
			)
		case <-end.C:
			break loop
		case <-ctx.Done():
			break loop
		}
	}

	cancel()
	wg.Wait()
	p := atomic.LoadUint64(&pubOK)
	r := atomic.LoadUint64(&recvCount)
	qc := atomic.LoadUint64(&qCount)
	qr := atomic.LoadUint64(&qResults)
	log.I.F(
		"stresstest complete: cache_published=%d published_ok=%d received=%d queries=%d results=%d duration=%s",
		cachePublished, p, r, qc, qr,
		time.Since(start).Truncate(time.Millisecond),
	)
}
