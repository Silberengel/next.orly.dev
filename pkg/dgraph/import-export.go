package dgraph

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"next.orly.dev/pkg/encoders/event"
)

// Import imports events from a reader (JSONL format)
func (d *D) Import(rr io.Reader) {
	d.ImportEventsFromReader(context.Background(), rr)
}

// Export exports events to a writer (JSONL format)
func (d *D) Export(c context.Context, w io.Writer, pubkeys ...[]byte) {
	// Query all events or events for specific pubkeys
	// Write as JSONL

	// Stub implementation
	fmt.Fprintf(w, "# Export not yet implemented for dgraph\n")
}

// ImportEventsFromReader imports events from a reader
func (d *D) ImportEventsFromReader(ctx context.Context, rr io.Reader) error {
	scanner := bufio.NewScanner(rr)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max line size

	count := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Skip comments
		if line[0] == '#' {
			continue
		}

		// Parse event
		ev := &event.E{}
		if err := json.Unmarshal(line, ev); err != nil {
			d.Logger.Warningf("failed to parse event: %v", err)
			continue
		}

		// Save event
		if _, err := d.SaveEvent(ctx, ev); err != nil {
			d.Logger.Warningf("failed to import event: %v", err)
			continue
		}

		count++
		if count%1000 == 0 {
			d.Logger.Infof("imported %d events", count)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	d.Logger.Infof("import complete: %d events", count)
	return nil
}

// ImportEventsFromStrings imports events from JSON strings
func (d *D) ImportEventsFromStrings(
	ctx context.Context,
	eventJSONs []string,
	policyManager interface{ CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error) },
) error {
	for _, eventJSON := range eventJSONs {
		ev := &event.E{}
		if err := json.Unmarshal([]byte(eventJSON), ev); err != nil {
			continue
		}

		// Check policy if manager is provided
		if policyManager != nil {
			if allowed, err := policyManager.CheckPolicy("write", ev, ev.Pubkey[:], "import"); err != nil || !allowed {
				continue
			}
		}

		// Save event
		if _, err := d.SaveEvent(ctx, ev); err != nil {
			d.Logger.Warningf("failed to import event: %v", err)
		}
	}

	return nil
}
