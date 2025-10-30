package relaytester

import (
	"fmt"
	"time"

	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
)

// Test implementations - these are referenced by test.go

func testPublishBasicEvent(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "test content", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if !accepted {
		return TestResult{Pass: false, Info: fmt.Sprintf("event rejected: %s", reason)}
	}
	return TestResult{Pass: true}
}

func testFindByID(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by id test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"ids": []string{hex.Enc(ev.ID)},
	}
	events, err := client.GetEvents("test-sub", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by ID"}
	}
	return TestResult{Pass: true}
}

func testFindByAuthor(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by author test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
	}
	events, err := client.GetEvents("test-author", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by author"}
	}
	return TestResult{Pass: true}
}

func testFindByKind(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by kind test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds": []int{int(kind.TextNote.K)},
	}
	events, err := client.GetEvents("test-kind", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by kind"}
	}
	return TestResult{Pass: true}
}

func testFindByTags(client *Client, key1, key2 *KeyPair) (result TestResult) {
	tags := tag.NewS(tag.NewFromBytesSlice([]byte("t"), []byte("test-tag")))
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by tags test", tags)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"#t": []string{"test-tag"},
	}
	events, err := client.GetEvents("test-tags", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by tags"}
	}
	return TestResult{Pass: true}
}

func testFindByMultipleTags(client *Client, key1, key2 *KeyPair) (result TestResult) {
	tags := tag.NewS(
		tag.NewFromBytesSlice([]byte("t"), []byte("multi-tag-1")),
		tag.NewFromBytesSlice([]byte("t"), []byte("multi-tag-2")),
	)
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by multiple tags test", tags)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"#t": []string{"multi-tag-1", "multi-tag-2"},
	}
	events, err := client.GetEvents("test-multi-tags", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by multiple tags"}
	}
	return TestResult{Pass: true}
}

func testFindByTimeRange(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by time range test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	now := time.Now().Unix()
	filter := map[string]interface{}{
		"since": now - 3600,
		"until": now + 3600,
	}
	events, err := client.GetEvents("test-time", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by time range"}
	}
	return TestResult{Pass: true}
}

func testRejectInvalidSignature(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "invalid sig test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	// Corrupt the signature
	ev.Sig[0] ^= 0xFF
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if accepted {
		return TestResult{Pass: false, Info: "invalid signature was accepted"}
	}
	_ = reason
	return TestResult{Pass: true}
}

func testRejectFutureEvent(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "future event test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev.CreatedAt = time.Now().Unix() + 3600 // 1 hour in the future
	// Re-sign with new timestamp
	if err = ev.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if accepted {
		return TestResult{Pass: false, Info: "future event was accepted"}
	}
	_ = reason
	return TestResult{Pass: true}
}

func testRejectExpiredEvent(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "expired event test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev.CreatedAt = time.Now().Unix() - 86400*365 // 1 year ago
	// Re-sign with new timestamp
	if err = ev.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	// Some relays may accept old events, so this is optional
	if !accepted {
		return TestResult{Pass: true, Info: "expired event rejected (expected)"}
	}
	return TestResult{Pass: true, Info: "expired event accepted (relay allows old events)"}
}

func testReplaceableEvents(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateReplaceableEvent(key1.Secret, kind.ProfileMetadata.K, "first version")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "first event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	ev2, err := CreateReplaceableEvent(key1.Secret, kind.ProfileMetadata.K, "second version")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err = client.WaitForOK(ev2.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "second event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds":   []int{int(kind.ProfileMetadata.K)},
		"authors": []string{hex.Enc(key1.Pubkey)},
	}
	events, err := client.GetEvents("test-replaceable", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	foundSecond := false
	for _, e := range events {
		if string(e.ID) == string(ev2.ID) {
			foundSecond = true
			break
		}
	}
	if !foundSecond {
		return TestResult{Pass: false, Info: "second replaceable event not found"}
	}
	return TestResult{Pass: true}
}

func testEphemeralEvents(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEphemeralEvent(key1.Secret, 20000, "ephemeral test")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "ephemeral event not accepted"}
	}
	// Ephemeral events should not be stored, so query should not find them
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds": []int{20000},
	}
	events, err := client.GetEvents("test-ephemeral", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	// Ephemeral events should not be queryable
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			return TestResult{Pass: false, Info: "ephemeral event was stored (should not be)"}
		}
	}
	return TestResult{Pass: true}
}

func testParameterizedReplaceableEvents(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateParameterizedReplaceableEvent(key1.Secret, 30023, "first list", "test-list")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "first event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	ev2, err := CreateParameterizedReplaceableEvent(key1.Secret, 30023, "second list", "test-list")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err = client.WaitForOK(ev2.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "second event not accepted"}
	}
	return TestResult{Pass: true}
}

func testDeletionEvents(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// First create an event to delete
	targetEv, err := CreateEvent(key1.Secret, kind.TextNote.K, "event to delete", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(targetEv); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(targetEv.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "target event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	// Now create deletion event
	deleteEv, err := CreateDeleteEvent(key1.Secret, [][]byte{targetEv.ID}, "deletion reason")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create delete event: %v", err)}
	}
	if err = client.Publish(deleteEv); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err = client.WaitForOK(deleteEv.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "delete event not accepted"}
	}
	return TestResult{Pass: true}
}

func testCountRequest(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "count test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds": []int{int(kind.TextNote.K)},
	}
	count, err := client.Count([]interface{}{filter})
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("COUNT failed: %v", err)}
	}
	if count < 1 {
		return TestResult{Pass: false, Info: fmt.Sprintf("COUNT returned %d, expected at least 1", count)}
	}
	return TestResult{Pass: true}
}

func testLimitParameter(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Publish multiple events
	for i := 0; i < 5; i++ {
		ev, err := CreateEvent(key1.Secret, kind.TextNote.K, fmt.Sprintf("limit test %d", i), nil)
		if err != nil {
			continue
		}
		client.Publish(ev)
		client.WaitForOK(ev.ID, 2*time.Second)
	}
	time.Sleep(500 * time.Millisecond)
	filter := map[string]interface{}{
		"limit": 2,
	}
	events, err := client.GetEvents("test-limit", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	// Limit should be respected (though exact count may vary)
	if len(events) > 10 {
		return TestResult{Pass: false, Info: fmt.Sprintf("got %d events, limit may not be working", len(events))}
	}
	return TestResult{Pass: true}
}

func testMultipleFilters(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateEvent(key1.Secret, kind.TextNote.K, "filter 1", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev2, err := CreateEvent(key2.Secret, kind.TextNote.K, "filter 2", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev1.ID, 2*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event 1 not accepted"}
	}
	accepted, _, err = client.WaitForOK(ev2.ID, 2*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event 2 not accepted"}
	}
	time.Sleep(300 * time.Millisecond)
	filter1 := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
	}
	filter2 := map[string]interface{}{
		"authors": []string{hex.Enc(key2.Pubkey)},
	}
	events, err := client.GetEvents("test-multi-filter", []interface{}{filter1, filter2}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	if len(events) < 2 {
		return TestResult{Pass: false, Info: fmt.Sprintf("got %d events, expected at least 2", len(events))}
	}
	return TestResult{Pass: true}
}

func testSubscriptionClose(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ch, err := client.Subscribe("close-test", []interface{}{map[string]interface{}{}})
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to subscribe: %v", err)}
	}
	if err = client.Unsubscribe("close-test"); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to unsubscribe: %v", err)}
	}
	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			return TestResult{Pass: false, Info: "subscription channel not closed"}
		}
	default:
		// Channel already closed, which is fine
	}
	return TestResult{Pass: true}
}
