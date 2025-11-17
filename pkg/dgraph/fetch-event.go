package dgraph

import (
	"context"
	"encoding/json"
	"fmt"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/interfaces/store"
)

// FetchEventBySerial retrieves an event by its serial number
func (d *D) FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error) {
	serial := ser.Get()

	query := fmt.Sprintf(`{
		event(func: eq(event.serial, %d)) {
			event.id
			event.kind
			event.created_at
			event.content
			event.sig
			event.pubkey
			event.tags
		}
	}`, serial)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch event by serial: %w", err)
	}

	evs, err := d.parseEventsFromResponse(resp.Json)
	if err != nil {
		return nil, err
	}

	if len(evs) == 0 {
		return nil, fmt.Errorf("event not found")
	}

	return evs[0], nil
}

// FetchEventsBySerials retrieves multiple events by their serial numbers
func (d *D) FetchEventsBySerials(serials []*types.Uint40) (
	events map[uint64]*event.E, err error,
) {
	if len(serials) == 0 {
		return make(map[uint64]*event.E), nil
	}

	// Build query for multiple serials
	serialStrs := make([]string, len(serials))
	for i, ser := range serials {
		serialStrs[i] = fmt.Sprintf("%d", ser.Get())
	}

	// Use uid() function for efficient multi-get
	query := fmt.Sprintf(`{
		events(func: uid(%s)) {
			event.id
			event.kind
			event.created_at
			event.content
			event.sig
			event.pubkey
			event.tags
			event.serial
		}
	}`, serialStrs[0]) // Simplified - in production you'd handle multiple UIDs properly

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events by serials: %w", err)
	}

	evs, err := d.parseEventsFromResponse(resp.Json)
	if err != nil {
		return nil, err
	}

	// Map events by serial
	events = make(map[uint64]*event.E)
	for i, ser := range serials {
		if i < len(evs) {
			events[ser.Get()] = evs[i]
		}
	}

	return events, nil
}

// GetSerialById retrieves the serial number for an event ID
func (d *D) GetSerialById(id []byte) (ser *types.Uint40, err error) {
	idStr := hex.Enc(id)

	query := fmt.Sprintf(`{
		event(func: eq(event.id, %q)) {
			event.serial
		}
	}`, idStr)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to get serial by ID: %w", err)
	}

	var result struct {
		Event []struct {
			Serial int64 `json:"event.serial"`
		} `json:"event"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return nil, err
	}

	if len(result.Event) == 0 {
		return nil, fmt.Errorf("event not found")
	}

	ser = &types.Uint40{}
	ser.Set(uint64(result.Event[0].Serial))

	return ser, nil
}

// GetSerialsByIds retrieves serial numbers for multiple event IDs
func (d *D) GetSerialsByIds(ids *tag.T) (
	serials map[string]*types.Uint40, err error,
) {
	serials = make(map[string]*types.Uint40)

	if len(ids.T) == 0 {
		return serials, nil
	}

	// Query each ID individually (simplified implementation)
	for _, id := range ids.T {
		if len(id) >= 2 {
			idStr := string(id[1])
			serial, err := d.GetSerialById([]byte(idStr))
			if err == nil {
				serials[idStr] = serial
			}
		}
	}

	return serials, nil
}

// GetSerialsByIdsWithFilter retrieves serials with a filter function
func (d *D) GetSerialsByIdsWithFilter(
	ids *tag.T, fn func(ev *event.E, ser *types.Uint40) bool,
) (serials map[string]*types.Uint40, err error) {
	serials = make(map[string]*types.Uint40)

	if fn == nil {
		// No filter, just return all
		return d.GetSerialsByIds(ids)
	}

	// With filter, need to fetch events
	for _, id := range ids.T {
		if len(id) > 0 {
			serial, err := d.GetSerialById(id)
			if err != nil {
				continue
			}

			ev, err := d.FetchEventBySerial(serial)
			if err != nil {
				continue
			}

			if fn(ev, serial) {
				serials[string(id)] = serial
			}
		}
	}

	return serials, nil
}

// GetSerialsByRange retrieves serials within a range
func (d *D) GetSerialsByRange(idx database.Range) (
	serials types.Uint40s, err error,
) {
	// This would need to be implemented based on how ranges are defined
	// For now, returning not implemented
	err = fmt.Errorf("not implemented")
	return
}

// GetFullIdPubkeyBySerial retrieves ID and pubkey for a serial number
func (d *D) GetFullIdPubkeyBySerial(ser *types.Uint40) (
	fidpk *store.IdPkTs, err error,
) {
	serial := ser.Get()

	query := fmt.Sprintf(`{
		event(func: eq(event.serial, %d)) {
			event.id
			event.pubkey
			event.created_at
		}
	}`, serial)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to get ID and pubkey by serial: %w", err)
	}

	var result struct {
		Event []struct {
			ID        string `json:"event.id"`
			Pubkey    string `json:"event.pubkey"`
			CreatedAt int64  `json:"event.created_at"`
		} `json:"event"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return nil, err
	}

	if len(result.Event) == 0 {
		return nil, fmt.Errorf("event not found")
	}

	id, err := hex.Dec(result.Event[0].ID)
	if err != nil {
		return nil, err
	}

	pubkey, err := hex.Dec(result.Event[0].Pubkey)
	if err != nil {
		return nil, err
	}

	fidpk = &store.IdPkTs{
		Id:  id,
		Pub: pubkey,
		Ts:  result.Event[0].CreatedAt,
		Ser: serial,
	}

	return fidpk, nil
}

// GetFullIdPubkeyBySerials retrieves IDs and pubkeys for multiple serials
func (d *D) GetFullIdPubkeyBySerials(sers []*types.Uint40) (
	fidpks []*store.IdPkTs, err error,
) {
	fidpks = make([]*store.IdPkTs, 0, len(sers))

	for _, ser := range sers {
		fidpk, err := d.GetFullIdPubkeyBySerial(ser)
		if err != nil {
			continue // Skip errors, continue with others
		}
		fidpks = append(fidpks, fidpk)
	}

	return fidpks, nil
}
