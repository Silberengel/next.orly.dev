package app

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/utils/atomic"
)

type Listener struct {
	*Server
	conn             *websocket.Conn
	ctx              context.Context
	remote           string
	req              *http.Request
	challenge        atomic.Bytes
	authedPubkey     atomic.Bytes
	startTime        time.Time
	isSelfConnection bool      // Marker to identify self-connections
	isBlacklisted    bool      // Marker to identify blacklisted IPs
	blacklistTimeout time.Time // When to timeout blacklisted connections
	// Diagnostics: per-connection counters
	msgCount   int
	reqCount   int
	eventCount int
}

// Ctx returns the listener's context, but creates a new context for each operation
// to prevent cancellation from affecting subsequent operations
func (l *Listener) Ctx() context.Context {
	return l.ctx
}

func (l *Listener) Write(p []byte) (n int, err error) {
	start := time.Now()
	msgLen := len(p)

	// Log message attempt with content preview (first 200 chars for diagnostics)
	preview := string(p)
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	log.T.F(
		"ws->%s attempting write: len=%d preview=%q", l.remote, msgLen, preview,
	)

	// Use a separate context with timeout for writes to prevent race conditions
	// where the main connection context gets cancelled while writing events
	writeCtx, cancel := context.WithTimeout(
		context.Background(), DefaultWriteTimeout,
	)
	defer cancel()

	// Attempt the write operation
	writeStart := time.Now()
	if err = l.conn.Write(writeCtx, websocket.MessageText, p); err != nil {
		writeDuration := time.Since(writeStart)
		totalDuration := time.Since(start)

		// Log detailed failure information
		log.E.F(
			"ws->%s WRITE FAILED: len=%d duration=%v write_duration=%v error=%v preview=%q",
			l.remote, msgLen, totalDuration, writeDuration, err, preview,
		)

		// Check if this is a context timeout
		if writeCtx.Err() != nil {
			log.E.F(
				"ws->%s write timeout after %v (limit=%v)", l.remote,
				writeDuration, DefaultWriteTimeout,
			)
		}

		// Check connection state
		if l.conn != nil {
			log.T.F(
				"ws->%s connection state during failure: remote_addr=%v",
				l.remote, l.req.RemoteAddr,
			)
		}

		chk.E(err) // Still call the original error handler
		return
	}

	// Log successful write with timing
	writeDuration := time.Since(writeStart)
	totalDuration := time.Since(start)
	n = msgLen

	log.T.F(
		"ws->%s WRITE SUCCESS: len=%d duration=%v write_duration=%v",
		l.remote, n, totalDuration, writeDuration,
	)

	// Log slow writes for performance diagnostics
	if writeDuration > time.Millisecond*100 {
		log.T.F(
			"ws->%s SLOW WRITE detected: %v (>100ms) len=%d", l.remote,
			writeDuration, n,
		)
	}

	return
}

// getManagedACL returns the managed ACL instance if available
func (l *Listener) getManagedACL() *database.ManagedACL {
	// Get the managed ACL instance from the ACL registry
	for _, aclInstance := range acl.Registry.ACL {
		if aclInstance.Type() == "managed" {
			if managed, ok := aclInstance.(*acl.Managed); ok {
				return managed.GetManagedACL()
			}
		}
	}
	return nil
}

// QueryEvents queries events using the database QueryEvents method
func (l *Listener) QueryEvents(ctx context.Context, f *filter.F) (event.S, error) {
	return l.D.QueryEvents(ctx, f)
}

// QueryAllVersions queries events using the database QueryAllVersions method
func (l *Listener) QueryAllVersions(ctx context.Context, f *filter.F) (event.S, error) {
	return l.D.QueryAllVersions(ctx, f)
}
