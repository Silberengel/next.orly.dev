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
	// Add p tag with hex-encoded pubkey
	addTag(testEvent, "p", hex.Enc([]byte("test-pubkey-2")))
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
		responseChan: make(chan PolicyResponse, 100),
	}

	// Test manager state
	if !manager.IsEnabled() {
		t.Error("Expected policy manager to be enabled")
	}

	if manager.IsRunning() {
		t.Error("Expected policy manager to not be running initially")
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

func TestCheckGlobalRulePolicy(t *testing.T) {
	tests := []struct {
		name           string
		globalRule     Rule
		event          *event.E
		loggedInPubkey []byte
		expected       bool
	}{
		{
			name: "global rule with write allow - event allowed",
			globalRule: Rule{
				WriteAllow: []string{"746573742d7075626b6579"},
			},
			event:          createTestEvent("test-id", "test-pubkey", "test content", 1),
			loggedInPubkey: []byte("test-logged-in-pubkey"),
			expected:       true,
		},
		{
			name: "global rule with write deny - event denied",
			globalRule: Rule{
				WriteDeny: []string{"746573742d7075626b6579"},
			},
			event:          createTestEvent("test-id", "test-pubkey", "test content", 1),
			loggedInPubkey: []byte("test-logged-in-pubkey"),
			expected:       false,
		},
		{
			name: "global rule with size limit - event too large",
			globalRule: Rule{
				SizeLimit: func() *int64 { v := int64(10); return &v }(),
			},
			event:          createTestEvent("test-id", "test-pubkey", "this is a very long content that exceeds the size limit", 1),
			loggedInPubkey: []byte("test-logged-in-pubkey"),
			expected:       false,
		},
		{
			name: "global rule with max age of event - event too old",
			globalRule: Rule{
				MaxAgeOfEvent: func() *int64 { v := int64(3600); return &v }(), // 1 hour
			},
			event: func() *event.E {
				ev := createTestEvent("test-id", "test-pubkey", "test content", 1)
				ev.CreatedAt = time.Now().Unix() - 7200 // 2 hours ago
				return ev
			}(),
			loggedInPubkey: []byte("test-logged-in-pubkey"),
			expected:       false,
		},
		{
			name: "global rule with max age event in future - event too far in future",
			globalRule: Rule{
				MaxAgeEventInFuture: func() *int64 { v := int64(3600); return &v }(), // 1 hour
			},
			event: func() *event.E {
				ev := createTestEvent("test-id", "test-pubkey", "test content", 1)
				ev.CreatedAt = time.Now().Unix() + 7200 // 2 hours in future
				return ev
			}(),
			loggedInPubkey: []byte("test-logged-in-pubkey"),
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &P{
				Global: tt.globalRule,
			}

			result := policy.checkGlobalRulePolicy("write", tt.event, tt.loggedInPubkey)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCheckPolicyWithGlobalRule(t *testing.T) {
	// Test that global rule is applied first
	policy := &P{
		Global: Rule{
			WriteDeny: []string{"746573742d7075626b6579"}, // Deny test-pubkey globally
		},
		Kind: Kinds{
			Whitelist: []int{1}, // Allow kind 1
		},
		Rules: map[int]Rule{
			1: {
				WriteAllow: []string{"746573742d7075626b6579"}, // Allow test-pubkey for kind 1
			},
		},
	}

	event := createTestEvent("test-id", "test-pubkey", "test content", 1)
	loggedInPubkey := []byte("test-logged-in-pubkey")

	// Global rule should deny this event even though kind-specific rule would allow it
	allowed, err := policy.CheckPolicy("write", event, loggedInPubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy failed: %v", err)
	}

	if allowed {
		t.Error("Expected event to be denied by global rule, but it was allowed")
	}
}

func TestMaxAgeChecks(t *testing.T) {
	tests := []struct {
		name           string
		rule           Rule
		event          *event.E
		loggedInPubkey []byte
		expected       bool
	}{
		{
			name: "max age of event - event within allowed age",
			rule: Rule{
				MaxAgeOfEvent: func() *int64 { v := int64(3600); return &v }(), // 1 hour
			},
			event: func() *event.E {
				ev := createTestEvent("test-id", "test-pubkey", "test content", 1)
				ev.CreatedAt = time.Now().Unix() - 1800 // 30 minutes ago
				return ev
			}(),
			loggedInPubkey: []byte("test-logged-in-pubkey"),
			expected:       true,
		},
		{
			name: "max age of event - event too old",
			rule: Rule{
				MaxAgeOfEvent: func() *int64 { v := int64(3600); return &v }(), // 1 hour
			},
			event: func() *event.E {
				ev := createTestEvent("test-id", "test-pubkey", "test content", 1)
				ev.CreatedAt = time.Now().Unix() - 7200 // 2 hours ago
				return ev
			}(),
			loggedInPubkey: []byte("test-logged-in-pubkey"),
			expected:       false,
		},
		{
			name: "max age event in future - event within allowed future time",
			rule: Rule{
				MaxAgeEventInFuture: func() *int64 { v := int64(3600); return &v }(), // 1 hour
			},
			event: func() *event.E {
				ev := createTestEvent("test-id", "test-pubkey", "test content", 1)
				ev.CreatedAt = time.Now().Unix() + 1800 // 30 minutes in future
				return ev
			}(),
			loggedInPubkey: []byte("test-logged-in-pubkey"),
			expected:       true,
		},
		{
			name: "max age event in future - event too far in future",
			rule: Rule{
				MaxAgeEventInFuture: func() *int64 { v := int64(3600); return &v }(), // 1 hour
			},
			event: func() *event.E {
				ev := createTestEvent("test-id", "test-pubkey", "test content", 1)
				ev.CreatedAt = time.Now().Unix() + 7200 // 2 hours in future
				return ev
			}(),
			loggedInPubkey: []byte("test-logged-in-pubkey"),
			expected:       false,
		},
		{
			name: "both age checks - event within both limits",
			rule: Rule{
				MaxAgeOfEvent:       func() *int64 { v := int64(3600); return &v }(), // 1 hour
				MaxAgeEventInFuture: func() *int64 { v := int64(1800); return &v }(), // 30 minutes
			},
			event: func() *event.E {
				ev := createTestEvent("test-id", "test-pubkey", "test content", 1)
				ev.CreatedAt = time.Now().Unix() + 900 // 15 minutes in future
				return ev
			}(),
			loggedInPubkey: []byte("test-logged-in-pubkey"),
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &P{}

			allowed, err := policy.checkRulePolicy("write", tt.event, tt.rule, tt.loggedInPubkey)
			if err != nil {
				t.Fatalf("checkRulePolicy failed: %v", err)
			}

			if allowed != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, allowed)
			}
		})
	}
}

func TestScriptPolicyNotRunningFallsBackToDefault(t *testing.T) {
	// Create a policy with a script rule but no running manager, default policy is "allow"
	policy := &P{
		DefaultPolicy: "allow",
		Rules: map[int]Rule{
			1: {
				Description: "script rule",
				Script:      "policy.sh",
			},
		},
		Manager: &PolicyManager{
			enabled:   true,
			isRunning: false, // Script is not running
		},
	}

	// Create test event
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	// Should allow the event when script is configured but not running (falls back to default "allow")
	allowed, err := policy.CheckPolicy("write", testEvent, []byte("test-pubkey"), "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed when script is not running (should fall back to default policy 'allow')")
	}

	// Test with default policy "deny"
	policy.DefaultPolicy = "deny"
	allowed2, err2 := policy.CheckPolicy("write", testEvent, []byte("test-pubkey"), "127.0.0.1")
	if err2 != nil {
		t.Errorf("Unexpected error: %v", err2)
	}
	if allowed2 {
		t.Error("Expected event to be denied when script is not running and default policy is 'deny'")
	}
}

func TestDefaultPolicyAllow(t *testing.T) {
	// Test default policy "allow" behavior
	policy := &P{
		DefaultPolicy: "allow",
		Kind:          Kinds{},
		Rules:         map[int]Rule{}, // No specific rules
	}

	// Create test event for kind 1 (no specific rule exists)
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	// Should allow the event with default policy "allow"
	allowed, err := policy.CheckPolicy("write", testEvent, []byte("test-pubkey"), "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed with default_policy 'allow'")
	}
}

func TestDefaultPolicyDeny(t *testing.T) {
	// Test default policy "deny" behavior
	policy := &P{
		DefaultPolicy: "deny",
		Kind:          Kinds{},
		Rules:         map[int]Rule{}, // No specific rules
	}

	// Create test event for kind 1 (no specific rule exists)
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	// Should deny the event with default policy "deny"
	allowed, err := policy.CheckPolicy("write", testEvent, []byte("test-pubkey"), "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if allowed {
		t.Error("Expected event to be denied with default_policy 'deny'")
	}
}

func TestDefaultPolicyEmpty(t *testing.T) {
	// Test empty default policy (should default to "allow")
	policy := &P{
		DefaultPolicy: "",
		Kind:          Kinds{},
		Rules:         map[int]Rule{}, // No specific rules
	}

	// Create test event for kind 1 (no specific rule exists)
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	// Should allow the event with empty default policy (defaults to "allow")
	allowed, err := policy.CheckPolicy("write", testEvent, []byte("test-pubkey"), "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed with empty default_policy (should default to 'allow')")
	}
}

func TestDefaultPolicyInvalid(t *testing.T) {
	// Test invalid default policy (should default to "allow")
	policy := &P{
		DefaultPolicy: "invalid",
		Kind:          Kinds{},
		Rules:         map[int]Rule{}, // No specific rules
	}

	// Create test event for kind 1 (no specific rule exists)
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	// Should allow the event with invalid default policy (defaults to "allow")
	allowed, err := policy.CheckPolicy("write", testEvent, []byte("test-pubkey"), "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed with invalid default_policy (should default to 'allow')")
	}
}

func TestDefaultPolicyWithSpecificRule(t *testing.T) {
	// Test that specific rules override default policy
	policy := &P{
		DefaultPolicy: "deny", // Default is deny
		Kind:          Kinds{},
		Rules: map[int]Rule{
			1: {
				Description: "allow kind 1",
				WriteAllow:  []string{}, // Allow all for kind 1
			},
		},
	}

	// Create test event for kind 1 (has specific rule)
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	// Should allow the event because specific rule allows it, despite default policy being "deny"
	allowed, err := policy.CheckPolicy("write", testEvent, []byte("test-pubkey"), "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed by specific rule, despite default_policy 'deny'")
	}

	// Create test event for kind 2 (no specific rule exists)
	testEvent2 := createTestEvent("test-event-id-2", "test-pubkey", "test content", 2)

	// Should deny the event because no specific rule and default policy is "deny"
	allowed2, err2 := policy.CheckPolicy("write", testEvent2, []byte("test-pubkey"), "127.0.0.1")
	if err2 != nil {
		t.Errorf("Unexpected error: %v", err2)
	}
	if allowed2 {
		t.Error("Expected event to be denied with default_policy 'deny' for kind without specific rule")
	}
}

func TestNewPolicyDefaultsToAllow(t *testing.T) {
	// Test that New() function sets default policy to "allow"
	policy, err := New([]byte(`{}`))
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if policy.DefaultPolicy != "allow" {
		t.Errorf("Expected default policy to be 'allow', got '%s'", policy.DefaultPolicy)
	}
}

func TestNewPolicyWithDefaultPolicyJSON(t *testing.T) {
	// Test loading default policy from JSON
	jsonConfig := `{"default_policy": "deny"}`
	policy, err := New([]byte(jsonConfig))
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if policy.DefaultPolicy != "deny" {
		t.Errorf("Expected default policy to be 'deny', got '%s'", policy.DefaultPolicy)
	}
}

func TestScriptProcessingFailureFallsBackToDefault(t *testing.T) {
	// Test that script processing failures fall back to default policy
	// We'll test this by using a manager that's not running (simulating failure)
	policy := &P{
		DefaultPolicy: "allow",
		Rules: map[int]Rule{
			1: {
				Description: "script rule",
				Script:      "policy.sh",
			},
		},
		Manager: &PolicyManager{
			enabled:   true,
			isRunning: false, // Script is not running (simulating failure)
		},
	}

	// Create test event
	testEvent := createTestEvent("test-event-id", "test-pubkey", "test content", 1)

	// Should allow the event when script is not running (falls back to default "allow")
	allowed, err := policy.checkScriptPolicy("write", testEvent, "policy.sh", []byte("test-pubkey"), "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed when script is not running (should fall back to default policy 'allow')")
	}

	// Test with default policy "deny"
	policy.DefaultPolicy = "deny"
	allowed2, err2 := policy.checkScriptPolicy("write", testEvent, "policy.sh", []byte("test-pubkey"), "127.0.0.1")
	if err2 != nil {
		t.Errorf("Unexpected error: %v", err2)
	}
	if allowed2 {
		t.Error("Expected event to be denied when script is not running and default policy is 'deny'")
	}
}

func TestDefaultPolicyLogicWithRules(t *testing.T) {
	// Test that default policy logic works correctly with rules

	// Test 1: default_policy "deny" - should only allow if rule explicitly allows
	policy1 := &P{
		DefaultPolicy: "deny",
		Kind: Kinds{
			Whitelist: []int{1, 2, 3}, // Allow kinds 1, 2, 3
		},
		Rules: map[int]Rule{
			1: {
				Description: "allow all for kind 1",
				WriteAllow:  []string{}, // Empty means allow all
			},
			2: {
				Description: "deny specific pubkey for kind 2",
				WriteDeny:   []string{"64656e6965642d7075626b6579"}, // hex of "denied-pubkey"
			},
			// No rule for kind 3
		},
	}

	// Kind 1: has rule that allows all - should be allowed
	event1 := createTestEvent("test-1", "test-pubkey", "content", 1)
	allowed1, err1 := policy1.CheckPolicy("write", event1, []byte("test-pubkey"), "127.0.0.1")
	if err1 != nil {
		t.Errorf("Unexpected error for kind 1: %v", err1)
	}
	if !allowed1 {
		t.Error("Expected kind 1 to be allowed (rule allows all)")
	}

	// Kind 2: has rule that denies specific pubkey - should be allowed for other pubkeys
	event2 := createTestEvent("test-2", "test-pubkey", "content", 2)
	allowed2, err2 := policy1.CheckPolicy("write", event2, []byte("test-pubkey"), "127.0.0.1")
	if err2 != nil {
		t.Errorf("Unexpected error for kind 2: %v", err2)
	}
	if !allowed2 {
		t.Error("Expected kind 2 to be allowed for non-denied pubkey")
	}

	// Kind 2: denied pubkey should be denied
	event2Denied := createTestEvent("test-2-denied", "denied-pubkey", "content", 2)
	allowed2Denied, err2Denied := policy1.CheckPolicy("write", event2Denied, []byte("test-pubkey"), "127.0.0.1")
	if err2Denied != nil {
		t.Errorf("Unexpected error for kind 2 denied: %v", err2Denied)
	}
	if allowed2Denied {
		t.Error("Expected kind 2 to be denied for denied pubkey")
	}

	// Kind 3: whitelisted but no rule - should follow default policy (deny)
	event3 := createTestEvent("test-3", "test-pubkey", "content", 3)
	allowed3, err3 := policy1.CheckPolicy("write", event3, []byte("test-pubkey"), "127.0.0.1")
	if err3 != nil {
		t.Errorf("Unexpected error for kind 3: %v", err3)
	}
	if allowed3 {
		t.Error("Expected kind 3 to be denied (no rule, default policy is deny)")
	}

	// Test 2: default_policy "allow" - should allow unless rule explicitly denies
	policy2 := &P{
		DefaultPolicy: "allow",
		Kind: Kinds{
			Whitelist: []int{1, 2, 3}, // Allow kinds 1, 2, 3
		},
		Rules: map[int]Rule{
			1: {
				Description: "deny specific pubkey for kind 1",
				WriteDeny:   []string{"64656e6965642d7075626b6579"}, // hex of "denied-pubkey"
			},
			// No rules for kind 2, 3
		},
	}

	// Kind 1: has rule that denies specific pubkey - should be allowed for other pubkeys
	event1Allow := createTestEvent("test-1-allow", "test-pubkey", "content", 1)
	allowed1Allow, err1Allow := policy2.CheckPolicy("write", event1Allow, []byte("test-pubkey"), "127.0.0.1")
	if err1Allow != nil {
		t.Errorf("Unexpected error for kind 1 allow: %v", err1Allow)
	}
	if !allowed1Allow {
		t.Error("Expected kind 1 to be allowed for non-denied pubkey")
	}

	// Kind 1: denied pubkey should be denied
	event1Deny := createTestEvent("test-1-deny", "denied-pubkey", "content", 1)
	allowed1Deny, err1Deny := policy2.CheckPolicy("write", event1Deny, []byte("test-pubkey"), "127.0.0.1")
	if err1Deny != nil {
		t.Errorf("Unexpected error for kind 1 deny: %v", err1Deny)
	}
	if allowed1Deny {
		t.Error("Expected kind 1 to be denied for denied pubkey")
	}

	// Kind 2: whitelisted but no rule - should follow default policy (allow)
	event2Allow := createTestEvent("test-2-allow", "test-pubkey", "content", 2)
	allowed2Allow, err2Allow := policy2.CheckPolicy("write", event2Allow, []byte("test-pubkey"), "127.0.0.1")
	if err2Allow != nil {
		t.Errorf("Unexpected error for kind 2 allow: %v", err2Allow)
	}
	if !allowed2Allow {
		t.Error("Expected kind 2 to be allowed (no rule, default policy is allow)")
	}
}
