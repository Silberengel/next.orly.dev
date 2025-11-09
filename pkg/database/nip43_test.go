package database

import (
	"context"
	"os"
	"testing"
	"time"
)

func setupNIP43TestDB(t *testing.T) (*D, func()) {
	tempDir, err := os.MkdirTemp("", "nip43_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

// TestAddNIP43Member tests adding a member
func TestAddNIP43Member(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}
	inviteCode := "test-invite-123"

	err := db.AddNIP43Member(pubkey, inviteCode)
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	// Verify member was added
	isMember, err := db.IsNIP43Member(pubkey)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if !isMember {
		t.Error("member was not added")
	}
}

// TestAddNIP43Member_InvalidPubkey tests adding member with invalid pubkey
func TestAddNIP43Member_InvalidPubkey(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	// Test with wrong length
	invalidPubkey := make([]byte, 16)
	err := db.AddNIP43Member(invalidPubkey, "test-code")
	if err == nil {
		t.Error("expected error for invalid pubkey length")
	}
}

// TestRemoveNIP43Member tests removing a member
func TestRemoveNIP43Member(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}

	// Add member
	err := db.AddNIP43Member(pubkey, "test-code")
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	// Remove member
	err = db.RemoveNIP43Member(pubkey)
	if err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}

	// Verify member was removed
	isMember, err := db.IsNIP43Member(pubkey)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if isMember {
		t.Error("member was not removed")
	}
}

// TestIsNIP43Member tests membership checking
func TestIsNIP43Member(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}

	// Check non-existent member
	isMember, err := db.IsNIP43Member(pubkey)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if isMember {
		t.Error("non-existent member reported as member")
	}

	// Add member
	err = db.AddNIP43Member(pubkey, "test-code")
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	// Check existing member
	isMember, err = db.IsNIP43Member(pubkey)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if !isMember {
		t.Error("existing member not found")
	}
}

// TestGetNIP43Membership tests retrieving membership details
func TestGetNIP43Membership(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}
	inviteCode := "test-invite-abc123"

	// Add member
	beforeAdd := time.Now()
	err := db.AddNIP43Member(pubkey, inviteCode)
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}
	afterAdd := time.Now()

	// Get membership
	membership, err := db.GetNIP43Membership(pubkey)
	if err != nil {
		t.Fatalf("failed to get membership: %v", err)
	}

	// Verify details
	if len(membership.Pubkey) != 32 {
		t.Errorf("wrong pubkey length: got %d, want 32", len(membership.Pubkey))
	}
	for i := range pubkey {
		if membership.Pubkey[i] != pubkey[i] {
			t.Errorf("pubkey mismatch at index %d", i)
			break
		}
	}

	if membership.InviteCode != inviteCode {
		t.Errorf("invite code mismatch: got %s, want %s", membership.InviteCode, inviteCode)
	}

	// Allow some tolerance for timestamp (database operations may take time)
	if membership.AddedAt.Before(beforeAdd.Add(-5*time.Second)) || membership.AddedAt.After(afterAdd.Add(5*time.Second)) {
		t.Errorf("AddedAt timestamp out of expected range: got %v, expected between %v and %v",
			membership.AddedAt, beforeAdd, afterAdd)
	}
}

// TestGetAllNIP43Members tests retrieving all members
func TestGetAllNIP43Members(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	// Add multiple members
	memberCount := 5
	for i := 0; i < memberCount; i++ {
		pubkey := make([]byte, 32)
		for j := range pubkey {
			pubkey[j] = byte(i*10 + j)
		}
		err := db.AddNIP43Member(pubkey, "code-"+string(rune(i)))
		if err != nil {
			t.Fatalf("failed to add member %d: %v", i, err)
		}
	}

	// Get all members
	members, err := db.GetAllNIP43Members()
	if err != nil {
		t.Fatalf("failed to get all members: %v", err)
	}

	if len(members) != memberCount {
		t.Errorf("wrong member count: got %d, want %d", len(members), memberCount)
	}

	// Verify each member has valid pubkey
	for i, member := range members {
		if len(member) != 32 {
			t.Errorf("member %d has invalid pubkey length: %d", i, len(member))
		}
	}
}

// TestStoreInviteCode tests storing invite codes
func TestStoreInviteCode(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	code := "test-invite-xyz789"
	expiresAt := time.Now().Add(24 * time.Hour)

	err := db.StoreInviteCode(code, expiresAt)
	if err != nil {
		t.Fatalf("failed to store invite code: %v", err)
	}

	// Validate the code
	valid, err := db.ValidateInviteCode(code)
	if err != nil {
		t.Fatalf("failed to validate invite code: %v", err)
	}
	if !valid {
		t.Error("stored invite code is not valid")
	}
}

// TestValidateInviteCode_Expired tests expired invite code handling
func TestValidateInviteCode_Expired(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	code := "expired-code"
	expiresAt := time.Now().Add(-1 * time.Hour) // Already expired

	err := db.StoreInviteCode(code, expiresAt)
	if err != nil {
		t.Fatalf("failed to store invite code: %v", err)
	}

	// Validate the code - should be invalid because it's expired
	valid, err := db.ValidateInviteCode(code)
	if err != nil {
		t.Fatalf("failed to validate invite code: %v", err)
	}
	if valid {
		t.Error("expired invite code reported as valid")
	}
}

// TestValidateInviteCode_NonExistent tests non-existent code validation
func TestValidateInviteCode_NonExistent(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	valid, err := db.ValidateInviteCode("non-existent-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Error("non-existent code reported as valid")
	}
}

// TestDeleteInviteCode tests deleting invite codes
func TestDeleteInviteCode(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	code := "delete-me-code"
	expiresAt := time.Now().Add(24 * time.Hour)

	// Store code
	err := db.StoreInviteCode(code, expiresAt)
	if err != nil {
		t.Fatalf("failed to store invite code: %v", err)
	}

	// Verify it exists
	valid, err := db.ValidateInviteCode(code)
	if err != nil {
		t.Fatalf("failed to validate invite code: %v", err)
	}
	if !valid {
		t.Error("stored code is not valid")
	}

	// Delete code
	err = db.DeleteInviteCode(code)
	if err != nil {
		t.Fatalf("failed to delete invite code: %v", err)
	}

	// Verify it's gone
	valid, err = db.ValidateInviteCode(code)
	if err != nil {
		t.Fatalf("failed to validate after delete: %v", err)
	}
	if valid {
		t.Error("deleted code still valid")
	}
}

// TestNIP43Membership_Serialization tests membership serialization
func TestNIP43Membership_Serialization(t *testing.T) {
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}

	original := NIP43Membership{
		Pubkey:     pubkey,
		AddedAt:    time.Now(),
		InviteCode: "test-code-123",
	}

	// Serialize
	data := serializeNIP43Membership(original)

	// Deserialize
	deserialized := deserializeNIP43Membership(data)

	// Verify
	if deserialized == nil {
		t.Fatal("deserialization returned nil")
	}

	if len(deserialized.Pubkey) != 32 {
		t.Errorf("wrong pubkey length: got %d, want 32", len(deserialized.Pubkey))
	}

	for i := range pubkey {
		if deserialized.Pubkey[i] != pubkey[i] {
			t.Errorf("pubkey mismatch at index %d", i)
			break
		}
	}

	if deserialized.InviteCode != original.InviteCode {
		t.Errorf("invite code mismatch: got %s, want %s", deserialized.InviteCode, original.InviteCode)
	}

	// Allow 1 second tolerance for timestamp comparison (due to Unix conversion)
	timeDiff := deserialized.AddedAt.Sub(original.AddedAt)
	if timeDiff < -1*time.Second || timeDiff > 1*time.Second {
		t.Errorf("timestamp mismatch: got %v, want %v (diff: %v)", deserialized.AddedAt, original.AddedAt, timeDiff)
	}
}

// TestNIP43Membership_ConcurrentAccess tests concurrent access to membership
func TestNIP43Membership_ConcurrentAccess(t *testing.T) {
	db, cleanup := setupNIP43TestDB(t)
	defer cleanup()

	const goroutines = 10
	const membersPerGoroutine = 5

	done := make(chan bool, goroutines)

	// Add members concurrently
	for g := 0; g < goroutines; g++ {
		go func(offset int) {
			for i := 0; i < membersPerGoroutine; i++ {
				pubkey := make([]byte, 32)
				for j := range pubkey {
					pubkey[j] = byte((offset*membersPerGoroutine+i)*10 + j)
				}
				if err := db.AddNIP43Member(pubkey, "code"); err != nil {
					t.Errorf("failed to add member: %v", err)
				}
			}
			done <- true
		}(g)
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Verify all members were added
	members, err := db.GetAllNIP43Members()
	if err != nil {
		t.Fatalf("failed to get all members: %v", err)
	}

	expected := goroutines * membersPerGoroutine
	if len(members) != expected {
		t.Errorf("wrong member count: got %d, want %d", len(members), expected)
	}
}
