// Package database provides shared import utilities for events
package database

import (
	"bufio"
	"context"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/encoders/event"
)

const maxLen = 500000000

// ImportEventsFromReader imports events from an io.Reader containing JSONL data
func (d *D) ImportEventsFromReader(ctx context.Context, rr io.Reader) error {
	// store to disk so we can return fast
	tmpPath := os.TempDir() + string(os.PathSeparator) + "orly"
	os.MkdirAll(tmpPath, 0700)
	tmp, err := os.CreateTemp(tmpPath, "")
	if chk.E(err) {
		return err
	}
	defer os.Remove(tmp.Name()) // Clean up temp file when done

	log.I.F("buffering upload to %s", tmp.Name())
	if _, err = io.Copy(tmp, rr); chk.E(err) {
		return err
	}
	if _, err = tmp.Seek(0, 0); chk.E(err) {
		return err
	}

	return d.processJSONLEvents(ctx, tmp)
}

// ImportEventsFromStrings imports events from a slice of JSON strings
func (d *D) ImportEventsFromStrings(ctx context.Context, eventJSONs []string) error {
	// Create a reader from the string slice
	reader := strings.NewReader(strings.Join(eventJSONs, "\n"))
	return d.processJSONLEvents(ctx, reader)
}

// processJSONLEvents processes JSONL events from a reader
func (d *D) processJSONLEvents(ctx context.Context, rr io.Reader) error {
	// Create a scanner to read the buffer line by line
	scan := bufio.NewScanner(rr)
	scanBuf := make([]byte, maxLen)
	scan.Buffer(scanBuf, maxLen)

	var count, total int
	for scan.Scan() {
		select {
		case <-ctx.Done():
			log.I.F("context closed")
			return ctx.Err()
		default:
		}

		b := scan.Bytes()
		total += len(b) + 1
		if len(b) < 1 {
			continue
		}

		ev := event.New()
		if _, err := ev.Unmarshal(b); err != nil {
			// return the pooled buffer on error
			ev.Free()
			log.W.F("failed to unmarshal event: %v", err)
			continue
		}

		if _, err := d.SaveEvent(ctx, ev); err != nil {
			// return the pooled buffer on error paths too
			ev.Free()
			log.W.F("failed to save event: %v", err)
			continue
		}

		// return the pooled buffer after successful save
		ev.Free()
		b = nil
		count++
		if count%100 == 0 {
			log.I.F("processed %d events", count)
			debug.FreeOSMemory()
		}
	}

	log.I.F("read %d bytes and saved %d events", total, count)
	if err := scan.Err(); err != nil {
		return err
	}

	return nil
}
