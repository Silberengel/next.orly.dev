package database

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
)

var (
	// ErrOlderThanExisting is returned when a candidate event is older than an existing replaceable/addressable event.
	ErrOlderThanExisting = errors.New("older than existing event")
	// ErrMissingDTag is returned when a parameterized replaceable event lacks the required 'd' tag.
	ErrMissingDTag = errors.New("event is missing a d tag identifier")
)

func (d *D) GetSerialsFromFilter(f *filter.F) (
	sers types.Uint40s, err error,
) {
	var idxs []Range
	if idxs, err = GetIndexesFromFilter(f); chk.E(err) {
		return
	}
	// Pre-allocate slice with estimated capacity to reduce reallocations
	sers = make(types.Uint40s, 0, len(idxs)*100) // Estimate 100 serials per index
	for _, idx := range idxs {
		var s types.Uint40s
		if s, err = d.GetSerialsByRange(idx); chk.E(err) {
			continue
		}
		sers = append(sers, s...)
	}
	return
}

// WouldReplaceEvent checks if the provided event would replace existing events
// based on Nostr's replaceable or parameterized replaceable semantics. It
// returns true if the candidate is newer-or-equal than existing events.
// If an existing event is newer, it returns (false, nil, ErrOlderThanExisting).
// If no conflicts exist, it returns (false, nil, nil).
func (d *D) WouldReplaceEvent(ev *event.E) (bool, types.Uint40s, error) {
	// Only relevant for replaceable or parameterized replaceable kinds
	if !(kind.IsReplaceable(ev.Kind) || kind.IsParameterizedReplaceable(ev.Kind)) {
		return false, nil, nil
	}

	var f *filter.F
	if kind.IsReplaceable(ev.Kind) {
		f = &filter.F{
			Authors: tag.NewFromBytesSlice(ev.Pubkey),
			Kinds:   kind.NewS(kind.New(ev.Kind)),
		}
	} else {
		// parameterized replaceable requires 'd' tag
		dTag := ev.Tags.GetFirst([]byte("d"))
		if dTag == nil {
			return false, nil, ErrMissingDTag
		}
		f = &filter.F{
			Authors: tag.NewFromBytesSlice(ev.Pubkey),
			Kinds:   kind.NewS(kind.New(ev.Kind)),
			Tags: tag.NewS(
				tag.NewFromAny("d", dTag.Value()),
			),
		}
	}

	sers, err := d.GetSerialsFromFilter(f)
	if chk.E(err) {
		return false, nil, err
	}
	if len(sers) == 0 {
		return false, nil, nil
	}

	// Determine if any existing event is newer than the candidate
	shouldReplace := true
	for _, s := range sers {
		oldEv, ferr := d.FetchEventBySerial(s)
		if chk.E(ferr) {
			continue
		}
		if ev.CreatedAt < oldEv.CreatedAt {
			shouldReplace = false
			break
		}
	}
	if shouldReplace {
		return true, nil, nil
	}
	return false, nil, ErrOlderThanExisting
}

// SaveEvent saves an event to the database, generating all the necessary indexes.
func (d *D) SaveEvent(c context.Context, ev *event.E) (
	replaced bool, err error,
) {
	if ev == nil {
		err = errors.New("nil event")
		return
	}
	
	// Reject ephemeral events (kinds 20000-29999) - they should never be stored
	if ev.Kind >= 20000 && ev.Kind <= 29999 {
		err = errors.New("blocked: ephemeral events should not be stored")
		return
	}
	
	// check if the event already exists
	var ser *types.Uint40
	if ser, err = d.GetSerialById(ev.ID); err == nil && ser != nil {
		err = errors.New("blocked: event already exists: " + hex.Enc(ev.ID[:]))
		return
	}

	// If the error is "id not found", we can proceed with saving the event
	if err != nil && strings.Contains(err.Error(), "id not found in database") {
		// Reset error since this is expected for new events
		err = nil
	} else if err != nil {
		// For any other error, return it
		// log.E.F("error checking if event exists: %s", err)
		return
	}

	// Check if the event has been deleted before allowing resubmission
	if err = d.CheckForDeleted(ev, nil); err != nil {
		// log.I.F(
		// 	"SaveEvent: rejecting resubmission of deleted event ID=%s: %v",
		// 	hex.Enc(ev.ID), err,
		// )
		err = fmt.Errorf("blocked: %s", err.Error())
		return
	}
	// check for replacement - only validate, don't delete old events
	if kind.IsReplaceable(ev.Kind) || kind.IsParameterizedReplaceable(ev.Kind) {
		var werr error
		if replaced, _, werr = d.WouldReplaceEvent(ev); werr != nil {
			if errors.Is(werr, ErrOlderThanExisting) {
				if kind.IsReplaceable(ev.Kind) {
					err = errors.New("blocked: event is older than existing replaceable event")
				} else {
					err = errors.New("blocked: event is older than existing addressable event")
				}
				return
			}
			if errors.Is(werr, ErrMissingDTag) {
				// keep behavior consistent with previous implementation
				err = ErrMissingDTag
				return
			}
			// any other error
			return
		}
		// Note: replaced flag is kept for compatibility but old events are no longer deleted
	}
	// Get the next sequence number for the event
	var serial uint64
	if serial, err = d.seq.Next(); chk.E(err) {
		return
	}
	// Generate all indexes for the event
	var idxs [][]byte
	if idxs, err = GetIndexesForEvent(ev, serial); chk.E(err) {
		return
	}
	log.T.F("SaveEvent: generated %d indexes for event %x (kind %d)", len(idxs), ev.ID, ev.Kind)
	// Start a transaction to save the event and all its indexes
	err = d.Update(
		func(txn *badger.Txn) (err error) {
			// Pre-allocate key buffer to avoid allocations in loop
			ser := new(types.Uint40)
			if err = ser.Set(serial); chk.E(err) {
				return
			}
			keyBuf := new(bytes.Buffer)
			if err = indexes.EventEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
				return
			}
			kb := keyBuf.Bytes()
			
			// Pre-allocate value buffer
			valueBuf := new(bytes.Buffer)
			ev.MarshalBinary(valueBuf)
			vb := valueBuf.Bytes()
			
			// Save each index
			for _, key := range idxs {
				if err = txn.Set(key, nil); chk.E(err) {
					return
				}
			}
			// write the event
			if err = txn.Set(kb, vb); chk.E(err) {
				return
			}
			return
		},
	)
	if err != nil {
		return
	}
	
	// Process deletion events to actually delete the referenced events
	if ev.Kind == kind.Deletion.K {
		if err = d.ProcessDelete(ev, nil); chk.E(err) {
			log.W.F("failed to process deletion for event %x: %v", ev.ID, err)
			// Don't return error - the deletion event was saved successfully
			err = nil
		}
	}
	return
}
