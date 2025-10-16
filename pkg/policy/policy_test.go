package policy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/tag"
)

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}

// Helper function to create test event
func createTestEvent(id, pubkey, content string, kind uint16) *event.E {
	return &event.E{
		ID:        []byte(id),
		Kind:      kind,
		Pubkey:    []byte(pubkey),
		Content:   []byte(content),
		Tags:      &tag.S{},
		CreatedAt: time.Now().Unix(),
	}
}

// Helper function to add tags to event
func addTag(ev *event.E, key, value string) {
	tagItem := tag.New()
	tagItem.T = append(tagItem.T, []byte(key), []byte(value))
	*ev.Tags = append(*ev.Tags, tagItem)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		policyJSON  []byte
		expectError bool
		expectRules int
	}{
		{
			name:        "empty JSON",
			policyJSON:  []byte("{}"),
			expectError: false,
			expectRules: 0,
		},
		{
			name:        "valid policy JSON",
			policyJSON:  []byte(`{"kind":{"whitelist":[1,3,5]},"rules":{"1":{"description":"test"}}}`),
			expectError: false,
			expectRules: 1,
		},
		{
			name:        "invalid JSON",
			policyJSON:  []byte(`{"invalid": json}`),
			expectError: true,
			expectRules: 0,
		},
		{
			name:        "nil JSON",
			policyJSON:  nil,
			expectError: false,
			expectRules: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := New(tt.policyJSON)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if policy == nil {
				t.Errorf("Expected policy but got nil")
				return
			}
			if len(policy.Rules) != tt.expectRules {
				t.Errorf("Expected %d rules, got %d", tt.expectRules, len(policy.Rules))
			}
		})
	}
}

func TestCheckKindsPolicy(t *testing.T) {
	tests := []struct {
		name     string
		policy   *P
		kind     uint16
		expected bool
	}{
		{
			name: "no whitelist or blacklist - allow all",
			policy: &P{
				Kind: Kinds{},
			},
			kind:     1,
			expected: true,
		},
		{
			name: "whitelist - kind allowed",
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{1, 3, 5},
				},
			},
			kind:     1,
			expected: true,
		},
		{
			name: "whitelist - kind not allowed",
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{1, 3, 5},
				},
			},
			kind:     2,
			expected: false,
		},
		{
			name: "blacklist - kind not blacklisted",
			policy: &P{
				Kind: Kinds{
					Blacklist: []int{2, 4, 6},
				},
			},
			kind:     1,
			expected: true,
		},
		{
			name: "blacklist - kind blacklisted",
			policy: &P{
				Kind: Kinds{
					Blacklist: []int{2, 4, 6},
				},
			},
			kind:     2,
			expected: false,
		},
		{
			name: "whitelist overrides blacklist",
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{1, 3, 5},
					Blacklist: []int{1, 2, 3},
				},
			},
			kind:     1,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy.checkKindsPolicy(tt.kind)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCheckRulePolicy(t *testing.T) {
	// Create test event
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)
	addTag(testEvent, "p", "test-pubkey-2")
	addTag(testEvent, "expiration", "1234567890")

	tests := []struct {
		name           string
		access         string
		event          *event.E
		rule           Rule
		loggedInPubkey []byte
		expected       bool
	}{
		{
			name:   "write access - no restrictions",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "no restrictions",
			},
			loggedInPubkey: []byte("test-pubkey"),
			expected:       true,
		},
		{
			name:   "write access - pubkey allowed",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "pubkey allowed",
				WriteAllow:  []string{hex.Enc(testEvent.Pubkey)},
			},
			loggedInPubkey: []byte("test-pubkey"),
			expected:       true,
		},
		{
			name:   "write access - pubkey not allowed",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "pubkey not allowed",
				WriteAllow:  []string{"other-pubkey"},
			},
			loggedInPubkey: []byte("test-pubkey"),
			expected:       false,
		},
		{
			name:   "size limit - within limit",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "size limit",
				SizeLimit:   int64Ptr(10000),
			},
			loggedInPubkey: []byte("test-pubkey"),
			expected:       true,
		},
		{
			name:   "size limit - exceeds limit",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "size limit exceeded",
				SizeLimit:   int64Ptr(10),
			},
			loggedInPubkey: []byte("test-pubkey"),
			expected:       false,
		},
		{
			name:   "content limit - within limit",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description:  "content limit",
				ContentLimit: int64Ptr(1000),
			},
			loggedInPubkey: []byte("test-pubkey"),
			expected:       true,
		},
		{
			name:   "content limit - exceeds limit",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description:  "content limit exceeded",
				ContentLimit: int64Ptr(5),
			},
			loggedInPubkey: []byte("test-pubkey"),
			expected:       false,
		},
		{
			name:   "required tags - has required tag",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description:  "required tags",
				MustHaveTags: []string{"p"},
			},
			loggedInPubkey: []byte("test-pubkey"),
			expected:       true,
		},
		{
			name:   "required tags - missing required tag",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description:  "required tags missing",
				MustHaveTags: []string{"e"},
			},
			loggedInPubkey: []byte("test-pubkey"),
			expected:       false,
		},
		{
			name:   "privileged - event authored by logged in user",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event",
				Privileged:  true,
			},
			loggedInPubkey: testEvent.Pubkey,
			expected:       true,
		},
		{
			name:   "privileged - event contains logged in user in p tag",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event with p tag",
				Privileged:  true,
			},
			loggedInPubkey: []byte("test-pubkey-2"),
			expected:       true,
		},
		{
			name:   "privileged - not authenticated",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event not authenticated",
				Privileged:  true,
			},
			loggedInPubkey: nil,
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &P{}
			result, err := policy.checkRulePolicy(tt.access, tt.event, tt.rule, tt.loggedInPubkey)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCheckPolicy(t *testing.T) {
	// Create test event
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	tests := []struct {
		name           string
		access         string
		event          *event.E
		policy         *P
		loggedInPubkey []byte
		ipAddress      string
		expected       bool
		expectError    bool
	}{
		{
			name:   "no policy rules - allow",
			access: "write",
			event:  testEvent,
			policy: &P{
				Kind:  Kinds{},
				Rules: map[int]Rule{},
			},
			loggedInPubkey: []byte("test-pubkey"),
			ipAddress:      "127.0.0.1",
			expected:       true,
			expectError:    false,
		},
		{
			name:   "kinds policy blocks - deny",
			access: "write",
			event:  testEvent,
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{3, 5},
				},
				Rules: map[int]Rule{},
			},
			loggedInPubkey: []byte("test-pubkey"),
			ipAddress:      "127.0.0.1",
			expected:       false,
			expectError:    false,
		},
		{
			name:   "rule blocks - deny",
			access: "write",
			event:  testEvent,
			policy: &P{
				Kind: Kinds{},
				Rules: map[int]Rule{
					1: {
						Description: "block test",
						WriteDeny:   []string{hex.Enc(testEvent.Pubkey)},
					},
				},
			},
			loggedInPubkey: []byte("test-pubkey"),
			ipAddress:      "127.0.0.1",
			expected:       false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.policy.CheckPolicy(tt.access, tt.event, tt.loggedInPubkey, tt.ipAddress)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "policy.json")

	tests := []struct {
		name        string
		configData  string
		expectError bool
		expectRules int
	}{
		{
			name:        "valid policy file",
			configData:  `{"kind":{"whitelist":[1,3,5]},"rules":{"1":{"description":"test"}}}`,
			expectError: false,
			expectRules: 1,
		},
		{
			name:        "empty policy file",
			configData:  `{}`,
			expectError: false,
			expectRules: 0,
		},
		{
			name:        "invalid JSON",
			configData:  `{"invalid": json}`,
			expectError: true,
			expectRules: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test config file
			if tt.configData != "" {
				err := os.WriteFile(configPath, []byte(tt.configData), 0644)
				if err != nil {
					t.Fatalf("Failed to write test config file: %v", err)
				}
			}

			policy := &P{}
			err := policy.LoadFromFile(configPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if len(policy.Rules) != tt.expectRules {
				t.Errorf("Expected %d rules, got %d", tt.expectRules, len(policy.Rules))
			}
		})
	}

	// Test file not found
	t.Run("file not found", func(t *testing.T) {
		policy := &P{}
		err := policy.LoadFromFile("/nonexistent/policy.json")
		if err == nil {
			t.Errorf("Expected error for nonexistent file but got none")
		}
	})
}

func TestPolicyEventSerialization(t *testing.T) {
	// Create test event
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	// Create policy event
	policyEvent := &PolicyEvent{
		E:              testEvent,
		LoggedInPubkey: "test-logged-in-pubkey",
		IPAddress:      "127.0.0.1",
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(policyEvent)
	if err != nil {
		t.Fatalf("Failed to marshal policy event: %v", err)
	}

	// Verify the JSON contains expected fields
	jsonStr := string(jsonData)
	t.Logf("Generated JSON: %s", jsonStr)

	if !strings.Contains(jsonStr, "test-logged-in-pubkey") {
		t.Error("JSON should contain logged_in_pubkey field")
	}
	if !strings.Contains(jsonStr, "127.0.0.1") {
		t.Error("JSON should contain ip_address field")
	}
	if !strings.Contains(jsonStr, "746573742d6576656e742d6964") { // hex encoded "test-event-id"
		t.Error("JSON should contain event id field (hex encoded)")
	}

	// Test with nil event
	nilPolicyEvent := &PolicyEvent{
		E:              nil,
		LoggedInPubkey: "test-logged-in-pubkey",
		IPAddress:      "127.0.0.1",
	}

	jsonData2, err := json.Marshal(nilPolicyEvent)
	if err != nil {
		t.Fatalf("Failed to marshal nil policy event: %v", err)
	}

	jsonStr2 := string(jsonData2)
	if !strings.Contains(jsonStr2, "test-logged-in-pubkey") {
		t.Error("JSON should contain logged_in_pubkey field even with nil event")
	}
	if !strings.Contains(jsonStr2, "127.0.0.1") {
		t.Error("JSON should contain ip_address field even with nil event")
	}
}

func TestPolicyResponseSerialization(t *testing.T) {
	// Test JSON serialization
	response := &PolicyResponse{
		ID:     "test-id",
		Action: "accept",
		Msg:    "test message",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal policy response: %v", err)
	}

	// Test JSON deserialization
	var deserializedResponse PolicyResponse
	err = json.Unmarshal(jsonData, &deserializedResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal policy response: %v", err)
	}

	// Verify fields
	if deserializedResponse.ID != response.ID {
		t.Errorf("Expected ID %s, got %s", response.ID, deserializedResponse.ID)
	}
	if deserializedResponse.Action != response.Action {
		t.Errorf("Expected Action %s, got %s", response.Action, deserializedResponse.Action)
	}
	if deserializedResponse.Msg != response.Msg {
		t.Errorf("Expected Msg %s, got %s", response.Msg, deserializedResponse.Msg)
	}
}

func TestNewWithManager(t *testing.T) {
	ctx := context.Background()
	appName := "test-app"
	enabled := true

	policy := NewWithManager(ctx, appName, enabled)

	if policy == nil {
		t.Fatal("Expected policy but got nil")
	}

	if policy.Manager == nil {
		t.Fatal("Expected policy manager but got nil")
	}

	if !policy.Manager.IsEnabled() {
		t.Error("Expected policy manager to be enabled")
	}

	if policy.Manager.IsRunning() {
		t.Error("Expected policy manager to not be running initially")
	}

	if policy.Manager.IsDisabled() {
		t.Error("Expected policy manager to not be disabled initially")
	}
}

func TestPolicyManagerLifecycle(t *testing.T) {
	// Test basic manager initialization without script execution
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &PolicyManager{
		ctx:          ctx,
		cancel:       cancel,
		configDir:    "/tmp",
		scriptPath:   "/tmp/policy.sh",
		enabled:      true,
		disabled:     false,
		responseChan: make(chan PolicyResponse, 100),
	}

	// Test manager state
	if !manager.IsEnabled() {
		t.Error("Expected policy manager to be enabled")
	}

	if manager.IsRunning() {
		t.Error("Expected policy manager to not be running initially")
	}

	if manager.IsDisabled() {
		t.Error("Expected policy manager to not be disabled initially")
	}

	// Test starting with non-existent script (should fail gracefully)
	err := manager.StartPolicy()
	if err == nil {
		t.Error("Expected error when starting policy with non-existent script")
	}

	// Test stopping when not running (should fail gracefully)
	err = manager.StopPolicy()
	if err == nil {
		t.Error("Expected error when stopping policy that's not running")
	}
}

func TestPolicyManagerProcessEvent(t *testing.T) {
	// Test processing event when manager is not running (should fail gracefully)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &PolicyManager{
		ctx:          ctx,
		cancel:       cancel,
		configDir:    "/tmp",
		scriptPath:   "/tmp/policy.sh",
		enabled:      true,
		disabled:     false,
		responseChan: make(chan PolicyResponse, 100),
	}

	// Create test event
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	// Create policy event
	policyEvent := &PolicyEvent{
		E:              testEvent,
		LoggedInPubkey: "test-logged-in-pubkey",
		IPAddress:      "127.0.0.1",
	}

	// Process event when not running (should fail gracefully)
	_, err := manager.ProcessEvent(policyEvent)
	if err == nil {
		t.Error("Expected error when processing event with non-running policy manager")
	}
}

func TestEdgeCasesEmptyPolicy(t *testing.T) {
	policy := &P{}

	// Create test event
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	// Should allow all events when policy is empty
	allowed, err := policy.CheckPolicy("write", testEvent, []byte("test-pubkey"), "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed with empty policy")
	}
}

func TestEdgeCasesNilEvent(t *testing.T) {
	policy := &P{}

	// Should handle nil event gracefully
	allowed, err := policy.CheckPolicy("write", nil, []byte("test-pubkey"), "127.0.0.1")
	if err == nil {
		t.Error("Expected error when event is nil")
	}
	if allowed {
		t.Error("Expected event to be blocked when nil")
	}

	// Verify the error message
	if err != nil && !strings.Contains(err.Error(), "event cannot be nil") {
		t.Errorf("Expected error message to contain 'event cannot be nil', got: %v", err)
	}
}

func TestEdgeCasesLargeEvent(t *testing.T) {
	// Create large content
	largeContent := strings.Repeat("a", 100000) // 100KB content

	policy := &P{
		Kind: Kinds{},
		Rules: map[int]Rule{
			1: {
				Description:  "size limit test",
				SizeLimit:    int64Ptr(50000), // 50KB limit
				ContentLimit: int64Ptr(10000), // 10KB content limit
			},
		},
	}

	// Create test event with large content
	testEvent := createTestEvent("test-event-id", "test-pubkey", largeContent, 1)

	// Should block large event
	allowed, err := policy.CheckPolicy("write", testEvent, []byte("test-pubkey"), "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if allowed {
		t.Error("Expected large event to be blocked")
	}
}

func TestEdgeCasesWhitelistBlacklistConflict(t *testing.T) {
	policy := &P{
		Kind: Kinds{
			Whitelist: []int{1, 3, 5},
			Blacklist: []int{1, 2, 3}, // Overlap with whitelist
		},
	}

	// Test kind in both whitelist and blacklist - whitelist should win
	allowed := policy.checkKindsPolicy(1)
	if !allowed {
		t.Error("Expected whitelist to override blacklist")
	}

	// Test kind in blacklist but not whitelist
	allowed = policy.checkKindsPolicy(2)
	if allowed {
		t.Error("Expected kind in blacklist but not whitelist to be blocked")
	}

	// Test kind in whitelist but not blacklist
	allowed = policy.checkKindsPolicy(5)
	if !allowed {
		t.Error("Expected kind in whitelist to be allowed")
	}
}

func TestEdgeCasesManagerWithInvalidScript(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "policy.sh")

	// Create invalid script (not executable, wrong shebang, etc.)
	scriptContent := `invalid script content`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644) // Not executable
	if err != nil {
		t.Fatalf("Failed to create invalid script: %v", err)
	}

	ctx := context.Background()
	manager := &PolicyManager{
		ctx:          ctx,
		configDir:    tempDir,
		scriptPath:   scriptPath,
		enabled:      true,
		disabled:     false,
		responseChan: make(chan PolicyResponse, 100),
	}

	// Should fail to start with invalid script
	err = manager.StartPolicy()
	if err == nil {
		t.Error("Expected error when starting policy with invalid script")
	}
}

func TestEdgeCasesManagerDoubleStart(t *testing.T) {
	// Test double start without actually starting (simpler test)
	ctx := context.Background()
	manager := &PolicyManager{
		ctx:          ctx,
		configDir:    "/tmp",
		scriptPath:   "/tmp/policy.sh",
		enabled:      true,
		disabled:     false,
		responseChan: make(chan PolicyResponse, 100),
	}

	// Try to start with non-existent script - should fail
	err := manager.StartPolicy()
	if err == nil {
		t.Error("Expected error when starting policy manager with non-existent script")
	}

	// Try to start again - should still fail
	err = manager.StartPolicy()
	if err == nil {
		t.Error("Expected error when starting policy manager twice")
	}
}

func TestEdgeCasesManagerDoubleStop(t *testing.T) {
	// Test double stop without actually starting (simpler test)
	ctx := context.Background()
	manager := &PolicyManager{
		ctx:          ctx,
		configDir:    "/tmp",
		scriptPath:   "/tmp/policy.sh",
		enabled:      true,
		disabled:     false,
		responseChan: make(chan PolicyResponse, 100),
	}

	// Try to stop when not running - should fail
	err := manager.StopPolicy()
	if err == nil {
		t.Error("Expected error when stopping policy manager that's not running")
	}

	// Try to stop again - should still fail
	err = manager.StopPolicy()
	if err == nil {
		t.Error("Expected error when stopping policy manager twice")
	}
}
