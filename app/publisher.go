package app

import (
	"context"
	"fmt"
	"sync"

	"encoders.orly/envelopes/eventenvelope"
	"encoders.orly/event"
	"encoders.orly/filter"
	"encoders.orly/hex"
	"encoders.orly/kind"
	"github.com/coder/websocket"
	"interfaces.orly/publisher"
	"interfaces.orly/typer"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	utils "utils.orly"
)

const Type = "socketapi"

type Subscription struct {
	remote       string
	AuthedPubkey []byte
	*filter.S
}

// Map is a map of filters associated with a collection of ws.Listener
// connections.
type Map map[*websocket.Conn]map[string]Subscription

type W struct {
	*websocket.Conn

	remote string

	// If Cancel is true, this is a close command.
	Cancel bool

	// Id is the subscription Id. If Cancel is true, cancel the named
	// subscription, otherwise, cancel the publisher for the socket.
	Id string

	// The Receiver holds the event channel for receiving notifications or data
	// relevant to this WebSocket connection.
	Receiver event.C

	// Filters holds a collection of filters used to match or process events
	// associated with this WebSocket connection. It is used to determine which
	// notifications or data should be received by the subscriber.
	Filters *filter.S

	// AuthedPubkey is the authenticated pubkey associated with the listener (if any).
	AuthedPubkey []byte
}

func (w *W) Type() (typeName string) { return Type }

// P is a structure that manages subscriptions and associated filters for
// websocket listeners. It uses a mutex to synchronize access to a map storing
// subscriber connections and their filter configurations.
type P struct {
	c context.Context
	// Mx is the mutex for the Map.
	Mx sync.RWMutex
	// Map is the map of subscribers and subscriptions from the websocket api.
	Map
}

var _ publisher.I = &P{}

func NewPublisher(c context.Context) (publisher *P) {
	return &P{
		c:   c,
		Map: make(Map),
	}
}

func (p *P) Type() (typeName string) { return Type }

// Receive handles incoming messages to manage websocket listener subscriptions
// and associated filters.
//
// # Parameters
//
// - msg (publisher.Message): The incoming message to process; expected to be of
// type *W to trigger subscription management actions.
//
// # Expected behaviour
//
// - Checks if the message is of type *W.
//
// - If Cancel is true, removes a subscriber by ID or the entire listener.
//
// - Otherwise, adds the subscription to the map under a mutex lock.
//
// - Logs actions related to subscription creation or removal.
func (p *P) Receive(msg typer.T) {
	if m, ok := msg.(*W); ok {
		if m.Cancel {
			if m.Id == "" {
				p.removeSubscriber(m.Conn)
				log.D.F("removed listener %s", m.remote)
			} else {
				p.removeSubscriberId(m.Conn, m.Id)
				log.D.C(
					func() string {
						return fmt.Sprintf(
							"removed subscription %s for %s", m.Id,
							m.remote,
						)
					},
				)
			}
			return
		}
		p.Mx.Lock()
		defer p.Mx.Unlock()
		if subs, ok := p.Map[m.Conn]; !ok {
			subs = make(map[string]Subscription)
			subs[m.Id] = Subscription{S: m.Filters, remote: m.remote, AuthedPubkey: m.AuthedPubkey}
			p.Map[m.Conn] = subs
			log.D.C(
				func() string {
					return fmt.Sprintf(
						"created new subscription for %s, %s",
						m.remote,
						m.Filters.Marshal(nil),
					)
				},
			)
		} else {
			subs[m.Id] = Subscription{S: m.Filters, remote: m.remote, AuthedPubkey: m.AuthedPubkey}
			log.D.C(
				func() string {
					return fmt.Sprintf(
						"added subscription %s for %s", m.Id,
						m.remote,
					)
				},
			)
		}
	}
}

// Deliver processes and distributes an event to all matching subscribers based on their filter configurations.
//
// # Parameters
//
// - ev (*event.E): The event to be delivered to subscribed clients.
//
// # Expected behaviour
//
// Delivers the event to all subscribers whose filters match the event. It
// applies authentication checks if required by the server and skips delivery
// for unauthenticated users when events are privileged.
func (p *P) Deliver(ev *event.E) {
	var err error
	// Snapshot the deliveries under read lock to avoid holding locks during I/O
	p.Mx.RLock()
	type delivery struct {
		w   *websocket.Conn
		id  string
		sub Subscription
	}
	var deliveries []delivery
	for w, subs := range p.Map {
		for id, subscriber := range subs {
			if subscriber.Match(ev) {
				deliveries = append(deliveries, delivery{w: w, id: id, sub: subscriber})
			}
		}
	}
	p.Mx.RUnlock()
	log.D.C(
		func() string {
			return fmt.Sprintf(
				"delivering event %0x to websocket subscribers %d", ev.ID,
				len(deliveries),
			)
		},
	)
	for _, d := range deliveries {
		// If the event is privileged, enforce that the subscriber's authed pubkey matches
		// either the event pubkey or appears in any 'p' tag of the event.
		if kind.IsPrivileged(ev.Kind) && len(d.sub.AuthedPubkey) > 0 {
			pk := d.sub.AuthedPubkey
			allowed := false
			// Direct author match
			if utils.FastEqual(ev.Pubkey, pk) {
				allowed = true
			} else if ev.Tags != nil {
				for _, pTag := range ev.Tags.GetAll([]byte("p")) {
					// pTag.Value() returns []byte hex string; decode to bytes
					dec, derr := hex.Dec(string(pTag.Value()))
					if derr != nil {
						continue
					}
					if utils.FastEqual(dec, pk) {
						allowed = true
						break
					}
				}
			}
			if !allowed {
				// Skip delivery for this subscriber
				continue
			}
		}
		var res *eventenvelope.Result
		if res, err = eventenvelope.NewResultWith(d.id, ev); chk.E(err) {
			continue
		}
		if err = d.w.Write(
			p.c, websocket.MessageText, res.Marshal(nil),
		); chk.E(err) {
			// On error, remove the subscriber connection safely
			p.removeSubscriber(d.w)
			_ = d.w.CloseNow()
			continue
		}
		log.D.C(
			func() string {
				return fmt.Sprintf(
					"dispatched event %0x to subscription %s, %s",
					ev.ID, d.id, d.sub.remote,
				)
			},
		)
	}
}

// removeSubscriberId removes a specific subscription from a subscriber
// websocket.
func (p *P) removeSubscriberId(ws *websocket.Conn, id string) {
	p.Mx.Lock()
	defer p.Mx.Unlock()
	var subs map[string]Subscription
	var ok bool
	if subs, ok = p.Map[ws]; ok {
		delete(p.Map[ws], id)
		_ = subs
		if len(subs) == 0 {
			delete(p.Map, ws)
		}
	}
}

// removeSubscriber removes a websocket from the P collection.
func (p *P) removeSubscriber(ws *websocket.Conn) {
	p.Mx.Lock()
	defer p.Mx.Unlock()
	clear(p.Map[ws])
	delete(p.Map, ws)
}
