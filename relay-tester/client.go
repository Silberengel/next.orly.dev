package relaytester

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"lol.mleku.dev/errorf"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/hex"
)

// Client wraps a WebSocket connection to a relay for testing.
type Client struct {
	conn   *websocket.Conn
	url    string
	mu     sync.Mutex
	subs   map[string]chan []byte
	okCh   chan []byte // Channel for OK messages
	countCh chan []byte // Channel for COUNT messages
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new test client connected to the relay.
func NewClient(url string) (c *Client, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	var conn *websocket.Conn
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	if conn, _, err = dialer.Dial(url, nil); err != nil {
		cancel()
		return
	}
	c = &Client{
		conn:    conn,
		url:     url,
		subs:    make(map[string]chan []byte),
		okCh:    make(chan []byte, 100),
		countCh: make(chan []byte, 100),
		ctx:     ctx,
		cancel:  cancel,
	}
	go c.readLoop()
	return
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.cancel()
	return c.conn.Close()
}

// Send sends a JSON message to the relay.
func (c *Client) Send(msg interface{}) (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var data []byte
	if data, err = json.Marshal(msg); err != nil {
		return errorf.E("failed to marshal message: %w", err)
	}
	if err = c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return errorf.E("failed to write message: %w", err)
	}
	return
}

// readLoop reads messages from the relay and routes them to subscriptions.
func (c *Client) readLoop() {
	defer c.conn.Close()
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var raw []interface{}
		if err = json.Unmarshal(msg, &raw); err != nil {
			continue
		}
		if len(raw) < 2 {
			continue
		}
		typ, ok := raw[0].(string)
		if !ok {
			continue
		}
		c.mu.Lock()
		switch typ {
		case "EVENT":
			if len(raw) >= 2 {
				if subID, ok := raw[1].(string); ok {
					if ch, exists := c.subs[subID]; exists {
						select {
						case ch <- msg:
						default:
						}
					}
				}
			}
		case "EOSE":
			if len(raw) >= 2 {
				if subID, ok := raw[1].(string); ok {
					if ch, exists := c.subs[subID]; exists {
						close(ch)
						delete(c.subs, subID)
					}
				}
			}
		case "OK":
			// Route OK messages to okCh for WaitForOK
			select {
			case c.okCh <- msg:
			default:
			}
		case "COUNT":
			// Route COUNT messages to countCh for Count
			select {
			case c.countCh <- msg:
			default:
			}
		case "NOTICE":
			// Notice messages are logged
		case "CLOSED":
			// Closed messages indicate subscription ended
		case "AUTH":
			// Auth challenge messages
		}
		c.mu.Unlock()
	}
}

// Subscribe creates a subscription and returns a channel for events.
func (c *Client) Subscribe(subID string, filters []interface{}) (ch chan []byte, err error) {
	req := []interface{}{"REQ", subID}
	req = append(req, filters...)
	if err = c.Send(req); err != nil {
		return
	}
	c.mu.Lock()
	ch = make(chan []byte, 100)
	c.subs[subID] = ch
	c.mu.Unlock()
	return
}

// Unsubscribe closes a subscription.
func (c *Client) Unsubscribe(subID string) error {
	c.mu.Lock()
	if ch, exists := c.subs[subID]; exists {
		// Channel might already be closed by EOSE, so use recover to handle gracefully
		func() {
			defer func() {
				if recover() != nil {
					// Channel was already closed, ignore
				}
			}()
			close(ch)
		}()
		delete(c.subs, subID)
	}
	c.mu.Unlock()
	return c.Send([]interface{}{"CLOSE", subID})
}

// Publish sends an EVENT message to the relay.
func (c *Client) Publish(ev *event.E) (err error) {
	evJSON := ev.Serialize()
	var evMap map[string]interface{}
	if err = json.Unmarshal(evJSON, &evMap); err != nil {
		return errorf.E("failed to unmarshal event: %w", err)
	}
	return c.Send([]interface{}{"EVENT", evMap})
}

// WaitForOK waits for an OK response for the given event ID.
func (c *Client) WaitForOK(eventID []byte, timeout time.Duration) (accepted bool, reason string, err error) {
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	idStr := hex.Enc(eventID)
	for {
		select {
		case <-ctx.Done():
			return false, "", errorf.E("timeout waiting for OK response")
		case msg := <-c.okCh:
			var raw []interface{}
			if err = json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			if len(raw) < 3 {
				continue
			}
			if id, ok := raw[1].(string); ok && id == idStr {
				accepted, _ = raw[2].(bool)
				if len(raw) > 3 {
					reason, _ = raw[3].(string)
				}
				return
			}
		}
	}
}

// Count sends a COUNT request and returns the count.
func (c *Client) Count(filters []interface{}) (count int64, err error) {
	req := []interface{}{"COUNT", "count-sub"}
	req = append(req, filters...)
	if err = c.Send(req); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return 0, errorf.E("timeout waiting for COUNT response")
		case msg := <-c.countCh:
			var raw []interface{}
			if err = json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			if len(raw) >= 3 {
				if subID, ok := raw[1].(string); ok && subID == "count-sub" {
					// COUNT response format: ["COUNT", "subscription-id", count, approximate?]
					if cnt, ok := raw[2].(float64); ok {
						return int64(cnt), nil
					}
				}
			}
		}
	}
}

// Auth sends an AUTH message with the signed event.
func (c *Client) Auth(ev *event.E) error {
	evJSON := ev.Serialize()
	var evMap map[string]interface{}
	if err := json.Unmarshal(evJSON, &evMap); err != nil {
		return errorf.E("failed to unmarshal event: %w", err)
	}
	return c.Send([]interface{}{"AUTH", evMap})
}

// GetEvents collects all events from a subscription until EOSE.
func (c *Client) GetEvents(subID string, filters []interface{}, timeout time.Duration) (events []*event.E, err error) {
	ch, err := c.Subscribe(subID, filters)
	if err != nil {
		return
	}
	defer c.Unsubscribe(subID)
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return events, nil
		case msg, ok := <-ch:
			if !ok {
				return events, nil
			}
			var raw []interface{}
			if err = json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			if len(raw) >= 3 && raw[0] == "EVENT" {
				if evData, ok := raw[2].(map[string]interface{}); ok {
					evJSON, _ := json.Marshal(evData)
					ev := event.New()
					if _, err = ev.Unmarshal(evJSON); err == nil {
						events = append(events, ev)
					}
				}
			}
		}
	}
}
