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
	writeChan        chan publish.WriteRequest // Channel for write requests (back to queued approach)
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


func (l *Listener) Write(p []byte) (n int, err error) {
	// Send write request to channel - non-blocking with timeout
	select {
	case <-l.ctx.Done():
		return 0, l.ctx.Err()
	case l.writeChan <- publish.WriteRequest{Data: p, MsgType: websocket.TextMessage, IsControl: false}:
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
	case l.writeChan <- publish.WriteRequest{Data: data, MsgType: messageType, IsControl: true, Deadline: deadline}:
		return nil
	case <-time.After(DefaultWriteTimeout):
		log.E.F("ws->%s writeControl channel timeout", l.remote)
		return errorf.E("writeControl channel timeout")
	}
}

// writeWorker is the single goroutine that handles all writes to the websocket connection.
// This serializes all writes to prevent concurrent write panics and allows pings to interrupt writes.
func (l *Listener) writeWorker() {
	defer func() {
		// Only unregister write channel if connection is actually dead/closing
		// Unregister if:
		// 1. Context is cancelled (connection closing)
		// 2. Channel was closed (connection closing)
		// 3. Connection error occurred (already handled inline)
		if l.ctx.Err() != nil {
			// Connection is closing - safe to unregister
			if socketPub := l.publishers.GetSocketPublisher(); socketPub != nil {
				log.D.F("ws->%s write worker: unregistering write channel (connection closing)", l.remote)
				socketPub.SetWriteChan(l.conn, nil)
			}
		} else {
			// Exiting for other reasons (timeout, etc.) but connection may still be valid
			log.D.F("ws->%s write worker exiting unexpectedly", l.remote)
		}
		close(l.writeDone)
	}()

	for {
		select {
		case <-l.ctx.Done():
			log.D.F("ws->%s write worker context cancelled", l.remote)
			return
		case req, ok := <-l.writeChan:
			if !ok {
				log.D.F("ws->%s write channel closed", l.remote)
				return
			}

			// Handle the write request
			var err error
			if req.IsPing {
				// Special handling for ping messages
				log.D.F("sending PING #%d", req.MsgType)
				deadline := time.Now().Add(DefaultWriteTimeout)
				err = l.conn.WriteControl(websocket.PingMessage, nil, deadline)
				if err != nil {
					if !strings.HasSuffix(err.Error(), "use of closed network connection") {
						log.E.F("error writing ping: %v; closing websocket", err)
					}
					return
				}
			} else if req.IsControl {
				// Control message
				err = l.conn.WriteControl(req.MsgType, req.Data, req.Deadline)
				if err != nil {
					log.E.F("ws->%s control write failed: %v", l.remote, err)
					return
				}
			} else {
				// Regular message
				l.conn.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout))
				err = l.conn.WriteMessage(req.MsgType, req.Data)
				if err != nil {
					log.E.F("ws->%s write failed: %v", l.remote, err)
					return
				}
			}
		}
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
