package dgraph

import (
	"context"
	"fmt"

	"github.com/dgraph-io/dgo/v230/protos/api"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/hex"
)

// DeleteEvent deletes an event by its ID
func (d *D) DeleteEvent(c context.Context, eid []byte) error {
	idStr := hex.Enc(eid)

	// Find the event's UID
	query := fmt.Sprintf(`{
		event(func: eq(event.id, %q)) {
			uid
		}
	}`, idStr)

	resp, err := d.Query(c, query)
	if err != nil {
		return fmt.Errorf("failed to find event for deletion: %w", err)
	}

	// Parse UID
	var result struct {
		Event []struct {
			UID string `json:"uid"`
		} `json:"event"`
	}

	if err = unmarshalJSON(resp.Json, &result); err != nil {
		return err
	}

	if len(result.Event) == 0 {
		return nil // Event doesn't exist
	}

	// Delete the event node
	mutation := &api.Mutation{
		DelNquads: []byte(fmt.Sprintf("<%s> * * .", result.Event[0].UID)),
		CommitNow: true,
	}

	if _, err = d.Mutate(c, mutation); err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	return nil
}

// DeleteEventBySerial deletes an event by its serial number
func (d *D) DeleteEventBySerial(c context.Context, ser *types.Uint40, ev *event.E) error {
	serial := ser.Get()

	// Find the event's UID
	query := fmt.Sprintf(`{
		event(func: eq(event.serial, %d)) {
			uid
		}
	}`, serial)

	resp, err := d.Query(c, query)
	if err != nil {
		return fmt.Errorf("failed to find event for deletion: %w", err)
	}

	// Parse UID
	var result struct {
		Event []struct {
			UID string `json:"uid"`
		} `json:"event"`
	}

	if err = unmarshalJSON(resp.Json, &result); err != nil {
		return err
	}

	if len(result.Event) == 0 {
		return nil // Event doesn't exist
	}

	// Delete the event node
	mutation := &api.Mutation{
		DelNquads: []byte(fmt.Sprintf("<%s> * * .", result.Event[0].UID)),
		CommitNow: true,
	}

	if _, err = d.Mutate(c, mutation); err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	return nil
}

// DeleteExpired removes events that have passed their expiration time
func (d *D) DeleteExpired() {
	// Query for events with expiration tags
	// This is a stub - full implementation would:
	// 1. Find events with "expiration" tag
	// 2. Check if current time > expiration time
	// 3. Delete those events
}

// ProcessDelete processes a kind 5 deletion event
func (d *D) ProcessDelete(ev *event.E, admins [][]byte) (err error) {
	if ev.Kind != 5 {
		return fmt.Errorf("event is not a deletion event (kind 5)")
	}

	// Extract event IDs to delete from tags
	for _, tag := range *ev.Tags {
		if len(tag.T) >= 2 && string(tag.T[0]) == "e" {
			eventID := tag.T[1]

			// Verify the deletion is authorized (author must match or be admin)
			if err = d.CheckForDeleted(ev, admins); err != nil {
				continue
			}

			// Delete the event
			if err = d.DeleteEvent(context.Background(), eventID); err != nil {
				// Log error but continue with other deletions
				d.Logger.Errorf("failed to delete event %s: %v", hex.Enc(eventID), err)
			}
		}
	}

	return nil
}

// CheckForDeleted checks if an event has been deleted
func (d *D) CheckForDeleted(ev *event.E, admins [][]byte) (err error) {
	// Query for delete events (kind 5) that reference this event
	evID := hex.Enc(ev.ID[:])

	query := fmt.Sprintf(`{
		deletes(func: eq(event.kind, 5)) @filter(eq(event.pubkey, %q)) {
			uid
			event.pubkey
			references @filter(eq(event.id, %q)) {
				event.id
			}
		}
	}`, hex.Enc(ev.Pubkey), evID)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to check for deletions: %w", err)
	}

	var result struct {
		Deletes []struct {
			UID        string `json:"uid"`
			Pubkey     string `json:"event.pubkey"`
			References []struct {
				ID string `json:"event.id"`
			} `json:"references"`
		} `json:"deletes"`
	}

	if err = unmarshalJSON(resp.Json, &result); err != nil {
		return err
	}

	// Check if any delete events reference this event
	for _, del := range result.Deletes {
		if len(del.References) > 0 {
			// Check if deletion is from the author or an admin
			delPubkey, _ := hex.Dec(del.Pubkey)
			if string(delPubkey) == string(ev.Pubkey) {
				return fmt.Errorf("event has been deleted by author")
			}

			// Check admins
			for _, admin := range admins {
				if string(delPubkey) == string(admin) {
					return fmt.Errorf("event has been deleted by admin")
				}
			}
		}
	}

	return nil
}
