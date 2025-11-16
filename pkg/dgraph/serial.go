package dgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dgraph-io/dgo/v230/protos/api"
)

// Serial number management
// We use a special counter node to track the next available serial number

const serialCounterKey = "serial_counter"

var (
	serialMutex sync.Mutex
)

// getNextSerial atomically increments and returns the next serial number
func (d *D) getNextSerial() (uint64, error) {
	serialMutex.Lock()
	defer serialMutex.Unlock()

	// Query current serial value
	query := fmt.Sprintf(`{
		counter(func: eq(marker.key, %q)) {
			uid
			marker.value
		}
	}`, serialCounterKey)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return 0, fmt.Errorf("failed to query serial counter: %w", err)
	}

	var result struct {
		Counter []struct {
			UID   string `json:"uid"`
			Value string `json:"marker.value"`
		} `json:"counter"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return 0, fmt.Errorf("failed to parse serial counter: %w", err)
	}

	var currentSerial uint64 = 1
	var uid string

	if len(result.Counter) > 0 {
		// Parse current serial
		uid = result.Counter[0].UID
		if result.Counter[0].Value != "" {
			fmt.Sscanf(result.Counter[0].Value, "%d", &currentSerial)
		}
	}

	// Increment serial
	nextSerial := currentSerial + 1

	// Update or create counter
	var nquads string
	if uid != "" {
		// Update existing counter
		nquads = fmt.Sprintf(`<%s> <marker.value> "%d" .`, uid, nextSerial)
	} else {
		// Create new counter
		nquads = fmt.Sprintf(`
_:counter <dgraph.type> "Marker" .
_:counter <marker.key> %q .
_:counter <marker.value> "%d" .
`, serialCounterKey, nextSerial)
	}

	mutation := &api.Mutation{
		SetNquads: []byte(nquads),
		CommitNow: true,
	}

	if _, err = d.Mutate(context.Background(), mutation); err != nil {
		return 0, fmt.Errorf("failed to update serial counter: %w", err)
	}

	return currentSerial, nil
}

// initSerialCounter initializes the serial counter if it doesn't exist
func (d *D) initSerialCounter() error {
	query := fmt.Sprintf(`{
		counter(func: eq(marker.key, %q)) {
			uid
		}
	}`, serialCounterKey)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to check serial counter: %w", err)
	}

	var result struct {
		Counter []struct {
			UID string `json:"uid"`
		} `json:"counter"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return fmt.Errorf("failed to parse counter check: %w", err)
	}

	// Counter already exists
	if len(result.Counter) > 0 {
		return nil
	}

	// Initialize counter at 1
	nquads := fmt.Sprintf(`
_:counter <dgraph.type> "Marker" .
_:counter <marker.key> %q .
_:counter <marker.value> "1" .
`, serialCounterKey)

	mutation := &api.Mutation{
		SetNquads: []byte(nquads),
		CommitNow: true,
	}

	if _, err = d.Mutate(context.Background(), mutation); err != nil {
		return fmt.Errorf("failed to initialize serial counter: %w", err)
	}

	d.Logger.Infof("initialized serial counter")
	return nil
}
