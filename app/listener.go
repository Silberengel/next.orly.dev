package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/utils"
	"next.orly.dev/pkg/utils/atomic"
)

// WriteRequest represents a write operation to be performed by the write worker
type WriteRequest = publish.WriteRequest

type Listener struct {
	*Server
	conn             *websocket.Conn
	ctx              context.Context
	remote           string
	req              *http.Request
	challenge        atomic.Bytes
	authedPubkey     atomic.Bytes
	startTime        time.Time
	isBlacklisted    bool      // Marker to identify blacklisted IPs
	blacklistTimeout time.Time // When to timeout blacklisted connections
	writeChan        chan WriteRequest // Channel for write requests
	writeDone        chan struct{}     // Closed when write worker exits
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

// writeWorker is the single goroutine that handles all writes to the websocket connection.
// This serializes all writes to prevent concurrent write panics.
func (l *Listener) writeWorker() {
	defer close(l.writeDone)
	for {
		select {
		case <-l.ctx.Done():
			return
		case req, ok := <-l.writeChan:
			if !ok {
				return
			}
			deadline := req.Deadline
			if deadline.IsZero() {
				deadline = time.Now().Add(DefaultWriteTimeout)
			}
			l.conn.SetWriteDeadline(deadline)
			writeStart := time.Now()
			var err error
			if req.IsControl {
				err = l.conn.WriteControl(req.MsgType, req.Data, deadline)
			} else {
				err = l.conn.WriteMessage(req.MsgType, req.Data)
			}
			if err != nil {
				writeDuration := time.Since(writeStart)
				log.E.F("ws->%s write worker FAILED: len=%d duration=%v error=%v",
					l.remote, len(req.Data), writeDuration, err)
				// Check for connection errors - if so, stop the worker
				isConnectionError := strings.Contains(err.Error(), "use of closed network connection") ||
					strings.Contains(err.Error(), "broken pipe") ||
					strings.Contains(err.Error(), "connection reset") ||
					websocket.IsCloseError(err, websocket.CloseAbnormalClosure,
						websocket.CloseGoingAway,
						websocket.CloseNoStatusReceived)
				if isConnectionError {
					return
				}
				// Continue for other errors (timeouts, etc.)
			} else {
				writeDuration := time.Since(writeStart)
				if writeDuration > time.Millisecond*100 {
					log.D.F("ws->%s write worker SLOW: len=%d duration=%v",
						l.remote, len(req.Data), writeDuration)
				}
			}
		}
	}
}

func (l *Listener) Write(p []byte) (n int, err error) {
	// Send write request to channel - non-blocking with timeout
	select {
	case <-l.ctx.Done():
		return 0, l.ctx.Err()
	case l.writeChan <- WriteRequest{Data: p, MsgType: websocket.TextMessage, IsControl: false}:
		return len(p), nil
	case <-time.After(DefaultWriteTimeout):
		log.E.F("ws->%s write channel timeout", l.remote)
		return 0, errorf.E("write channel timeout")
	}
}

// WriteControl sends a control message through the write channel
func (l *Listener) WriteControl(messageType int, data []byte, deadline time.Time) (err error) {
	select {
	case <-l.ctx.Done():
		return l.ctx.Err()
	case l.writeChan <- WriteRequest{Data: data, MsgType: messageType, IsControl: true, Deadline: deadline}:
		return nil
	case <-time.After(DefaultWriteTimeout):
		log.E.F("ws->%s writeControl channel timeout", l.remote)
		return errorf.E("writeControl channel timeout")
	}
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

// canSeePrivateEvent checks if the authenticated user can see an event with a private tag
func (l *Listener) canSeePrivateEvent(authedPubkey, privatePubkey []byte) (canSee bool) {
	// If no authenticated user, deny access
	if len(authedPubkey) == 0 {
		return false
	}

	// If the authenticated user matches the private tag pubkey, allow access
	if len(privatePubkey) > 0 && utils.FastEqual(authedPubkey, privatePubkey) {
		return true
	}

	// Check if user is an admin or owner (they can see all private events)
	accessLevel := acl.Registry.GetAccessLevel(authedPubkey, l.remote)
	if accessLevel == "admin" || accessLevel == "owner" {
		return true
	}

	// Default deny
	return false
}
