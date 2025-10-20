package policy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/hex"
)

// Kinds defines the filter for events by kind; the whitelist overrides the blacklist if it has any fields, and the blacklist is ignored (implicitly all not-whitelisted are blacklisted)
type Kinds struct {
	// Whitelist is a list of event kinds that are allowed to be written to the relay. If any are present, implicitly all others are denied.
	Whitelist []int `json:"whitelist,omitempty"`
	// Blacklist is a list of event kinds that are not allowed to be written to the relay. If any are present, implicitly all others are allowed. Only takes effect in the absence of a Whitelist.
	Blacklist []int `json:"blacklist,omitempty"`
}

// Rule is a rule for an event kind.
//
// If Script is present, it overrides all other criteria.
//
// The criteria have mutual exclude semantics on pubkey white/blacklists, if whitelist has any fields, blacklist is ignored (implicitly all not-whitelisted are blacklisted).
//
// The other criteria are evaluated as AND operations, everything specified must match for the event to be allowed to be written to the relay.
type Rule struct {
	// Description is a human-readable description of the rule.
	Description string `json:"description"`
	// Script is a path to a script that will be used to determine if the event should be allowed to be written to the relay. The script should be a standard bash script or whatever is native to the platform. The script will return its opinion to be one of the criteria that must be met for the event to be allowed to be written to the relay (AND).
	Script string `json:"script,omitempty"`
	// WriteAllow is a list of pubkeys that are allowed to write this event kind to the relay. If any are present, implicitly all others are denied.
	WriteAllow []string `json:"write_allow,omitempty"`
	// WriteDeny is a list of pubkeys that are not allowed to write this event kind to the relay. If any are present, implicitly all others are allowed. Only takes effect in the absence of a WriteAllow.
	WriteDeny []string `json:"write_deny,omitempty"`
	// ReadAllow is a list of pubkeys that are allowed to read this event kind from the relay. If any are present, implicitly all others are denied.
	ReadAllow []string `json:"read_allow,omitempty"`
	// ReadDeny is a list of pubkeys that are not allowed to read this event kind from the relay. If any are present, implicitly all others are allowed. Only takes effect in the absence of a ReadAllow.
	ReadDeny []string `json:"read_deny,omitempty"`
	// MaxExpiry is the maximum expiry time in seconds for events written to the relay. If 0, there is no maximum expiry. Events must have an expiry time if this is set, and it must be no more than this value in the future compared to the event's created_at time.
	MaxExpiry *int64 `json:"max_expiry,omitempty"`
	// MustHaveTags is a list of tag key letters that must be present on the event for it to be allowed to be written to the relay.
	MustHaveTags []string `json:"must_have_tags,omitempty"`
	// SizeLimit is the maximum size in bytes for the event's total serialized size.
	SizeLimit *int64 `json:"size_limit,omitempty"`
	// ContentLimit is the maximum size in bytes for the event's content field.
	ContentLimit *int64 `json:"content_limit,omitempty"`
	// Privileged means that this event is either authored by the authenticated pubkey, or has a p tag that contains the authenticated pubkey. This type of event is only sent to users who are authenticated and are party to the event.
	Privileged bool `json:"privileged,omitempty"`
	// RateLimit is the amount of data can be written to the relay per second by the authenticated pubkey. If 0, there is no rate limit. This is applied via the use of an EWMA of the event publication history on the authenticated connection
	RateLimit *int64 `json:"rate_limit,omitempty"`
	// MaxAgeOfEvent is the offset in seconds that is the oldest timestamp allowed for an event's created_at time. If 0, there is no maximum age. Events must have a created_at time if this is set, and it must be no more than this value in the past compared to the current time.
	MaxAgeOfEvent *int64 `json:"max_age_of_event,omitempty"`
	// MaxAgeEventInFuture is the offset in seconds that is the newest timestamp allowed for an event's created_at time ahead of the current time.
	MaxAgeEventInFuture *int64 `json:"max_age_event_in_future,omitempty"`
}

// PolicyEvent represents an event with additional context for policy scripts
type PolicyEvent struct {
	*event.E
	LoggedInPubkey string `json:"logged_in_pubkey,omitempty"`
	IPAddress      string `json:"ip_address,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for PolicyEvent
func (pe *PolicyEvent) MarshalJSON() ([]byte, error) {
	if pe.E == nil {
		return json.Marshal(map[string]interface{}{
			"logged_in_pubkey": pe.LoggedInPubkey,
			"ip_address":       pe.IPAddress,
		})
	}

	// Create a safe copy of the event for JSON marshaling
	safeEvent := map[string]interface{}{
		"id":         hex.Enc(pe.E.ID),
		"pubkey":     hex.Enc(pe.E.Pubkey),
		"created_at": pe.E.CreatedAt,
		"kind":       pe.E.Kind,
		"content":    string(pe.E.Content),
		"tags":       pe.E.Tags,
		"sig":        hex.Enc(pe.E.Sig),
	}

	// Add policy-specific fields
	if pe.LoggedInPubkey != "" {
		safeEvent["logged_in_pubkey"] = pe.LoggedInPubkey
	}
	if pe.IPAddress != "" {
		safeEvent["ip_address"] = pe.IPAddress
	}

	return json.Marshal(safeEvent)
}

// PolicyResponse represents a response from the policy script
type PolicyResponse struct {
	ID     string `json:"id"`
	Action string `json:"action"` // accept, reject, or shadowReject
	Msg    string `json:"msg"`    // NIP-20 response message (only used for reject)
}

// PolicyManager handles policy script execution and management
type PolicyManager struct {
	ctx           context.Context
	cancel        context.CancelFunc
	configDir     string
	scriptPath    string
	currentCmd    *exec.Cmd
	currentCancel context.CancelFunc
	mutex         sync.RWMutex
	isRunning     bool
	enabled       bool
	disabled      bool // true when policy is disabled due to failure
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	responseChan  chan PolicyResponse
}

// P is a policy for a relay's ACL.
type P struct {
	// Kind is policies for accepting or rejecting events by kind number.
	Kind Kinds `json:"kind"`
	// Rules is a map of rules for criteria that must be met for the event to be allowed to be written to the relay.
	Rules map[int]Rule `json:"rules"`
	// Global is a rule set that applies to all events.
	Global Rule `json:"global"`
	// Manager handles policy script execution
	Manager *PolicyManager `json:"-"`
}

// New creates a new policy from JSON configuration
func New(policyJSON []byte) (p *P, err error) {
	p = &P{}
	if len(policyJSON) > 0 {
		if err = json.Unmarshal(policyJSON, p); chk.E(err) {
			return nil, fmt.Errorf("failed to unmarshal policy JSON: %v", err)
		}
	}
	return
}

// NewWithManager creates a new policy with a policy manager for script execution
func NewWithManager(ctx context.Context, appName string, enabled bool) *P {
	configDir := filepath.Join(xdg.ConfigHome, appName)
	scriptPath := filepath.Join(configDir, "policy.sh")
	configPath := filepath.Join(configDir, "policy.json")

	ctx, cancel := context.WithCancel(ctx)

	manager := &PolicyManager{
		ctx:          ctx,
		cancel:       cancel,
		configDir:    configDir,
		scriptPath:   scriptPath,
		enabled:      enabled,
		disabled:     false,
		responseChan: make(chan PolicyResponse, 100), // Buffered channel for responses
	}

	// Load policy configuration from JSON file
	policy := &P{
		Manager: manager,
	}

	if enabled {
		if err := policy.LoadFromFile(configPath); err != nil {
			log.W.F("failed to load policy configuration from %s: %v", configPath, err)
			log.I.F("using default policy configuration")
		} else {
			log.I.F("loaded policy configuration from %s", configPath)
		}

		// Start the policy script if it exists and is enabled
		go manager.startPolicyIfExists()
		// Start periodic check for policy script availability
		go manager.periodicCheck()
	}

	return policy
}

// LoadFromFile loads policy configuration from a JSON file
func (p *P) LoadFromFile(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("policy configuration file does not exist: %s", configPath)
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read policy configuration file: %v", err)
	}

	if len(configData) == 0 {
		return fmt.Errorf("policy configuration file is empty")
	}

	if err := json.Unmarshal(configData, p); err != nil {
		return fmt.Errorf("failed to parse policy configuration JSON: %v", err)
	}

	return nil
}

// CheckPolicy checks if an event is allowed to be written to the relay based on the policy. The access parameter is either "write" or "read", write is for accepting events and read is for filtering events to send back to the client.
func (p *P) CheckPolicy(access string, ev *event.E, loggedInPubkey []byte, ipAddress string) (allowed bool, err error) {
	// Handle nil event
	if ev == nil {
		return false, fmt.Errorf("event cannot be nil")
	}

	// First check global rule filter (applies to all events)
	if !p.checkGlobalRulePolicy(access, ev, loggedInPubkey) {
		return false, nil
	}

	// Then check kinds white/blacklist
	if !p.checkKindsPolicy(ev.Kind) {
		return false, nil
	}

	// Get rule for this kind
	rule, hasRule := p.Rules[int(ev.Kind)]
	if !hasRule {
		// No specific rule for this kind, allow if global and kinds policy passed
		return true, nil
	}

	// Check if script is present and enabled
	if rule.Script != "" && p.Manager != nil && p.Manager.IsEnabled() {
		return p.checkScriptPolicy(access, ev, rule.Script, loggedInPubkey, ipAddress)
	}

	// Apply rule-based filtering
	return p.checkRulePolicy(access, ev, rule, loggedInPubkey)
}

// checkKindsPolicy checks if the event kind is allowed by the kinds white/blacklist
func (p *P) checkKindsPolicy(kind uint16) bool {
	// If whitelist is present, only allow whitelisted kinds
	if len(p.Kind.Whitelist) > 0 {
		for _, allowedKind := range p.Kind.Whitelist {
			if kind == uint16(allowedKind) {
				return true
			}
		}
		return false
	}

	// If blacklist is present, deny blacklisted kinds
	if len(p.Kind.Blacklist) > 0 {
		for _, deniedKind := range p.Kind.Blacklist {
			if kind == uint16(deniedKind) {
				return false
			}
		}
	}

	return true
}

// checkGlobalRulePolicy checks if the event passes the global rule filter
func (p *P) checkGlobalRulePolicy(access string, ev *event.E, loggedInPubkey []byte) bool {
	// Apply global rule filtering
	allowed, err := p.checkRulePolicy(access, ev, p.Global, loggedInPubkey)
	if err != nil {
		log.E.F("global rule policy check failed: %v", err)
		return false
	}
	return allowed
}

// checkRulePolicy applies rule-based filtering (pubkey lists, size limits, etc.)
func (p *P) checkRulePolicy(access string, ev *event.E, rule Rule, loggedInPubkey []byte) (allowed bool, err error) {
	pubkeyHex := hex.Enc(ev.Pubkey)

	// Check pubkey-based access control
	if access == "write" {
		// Check write allow/deny lists
		if len(rule.WriteAllow) > 0 {
			allowed = false
			for _, allowedPubkey := range rule.WriteAllow {
				if pubkeyHex == allowedPubkey {
					allowed = true
					break
				}
			}
			if !allowed {
				return false, nil
			}
		} else if len(rule.WriteDeny) > 0 {
			for _, deniedPubkey := range rule.WriteDeny {
				if pubkeyHex == deniedPubkey {
					return false, nil
				}
			}
		}
	} else if access == "read" {
		// Check read allow/deny lists
		if len(rule.ReadAllow) > 0 {
			allowed = false
			for _, allowedPubkey := range rule.ReadAllow {
				if pubkeyHex == allowedPubkey {
					allowed = true
					break
				}
			}
			if !allowed {
				return false, nil
			}
		} else if len(rule.ReadDeny) > 0 {
			for _, deniedPubkey := range rule.ReadDeny {
				if pubkeyHex == deniedPubkey {
					return false, nil
				}
			}
		}
	}

	// Check size limits
	if rule.SizeLimit != nil {
		eventSize := int64(len(ev.Serialize()))
		if eventSize > *rule.SizeLimit {
			return false, nil
		}
	}

	if rule.ContentLimit != nil {
		contentSize := int64(len(ev.Content))
		if contentSize > *rule.ContentLimit {
			return false, nil
		}
	}

	// Check required tags
	if len(rule.MustHaveTags) > 0 {
		for _, requiredTag := range rule.MustHaveTags {
			if ev.Tags.GetFirst([]byte(requiredTag)) == nil {
				return false, nil
			}
		}
	}

	// Check expiry time
	if rule.MaxExpiry != nil {
		expiryTag := ev.Tags.GetFirst([]byte("expiration"))
		if expiryTag == nil {
			return false, nil // Must have expiry if MaxExpiry is set
		}
		// TODO: Parse and validate expiry time
	}

	// Check MaxAgeOfEvent (maximum age of event in seconds)
	if rule.MaxAgeOfEvent != nil && *rule.MaxAgeOfEvent > 0 {
		currentTime := time.Now().Unix()
		maxAllowedTime := currentTime - *rule.MaxAgeOfEvent
		if ev.CreatedAt < maxAllowedTime {
			return false, nil // Event is too old
		}
	}

	// Check MaxAgeEventInFuture (maximum time event can be in the future in seconds)
	if rule.MaxAgeEventInFuture != nil && *rule.MaxAgeEventInFuture > 0 {
		currentTime := time.Now().Unix()
		maxFutureTime := currentTime + *rule.MaxAgeEventInFuture
		if ev.CreatedAt > maxFutureTime {
			return false, nil // Event is too far in the future
		}
	}

	// Check privileged events
	if rule.Privileged {
		if len(loggedInPubkey) == 0 {
			return false, nil // Must be authenticated
		}
		// Check if event is authored by logged in user or contains logged in user in p tags
		if !bytes.Equal(ev.Pubkey, loggedInPubkey) {
			// Check p tags
			pTags := ev.Tags.GetAll([]byte("p"))
			found := false
			for _, pTag := range pTags {
				if bytes.Equal(pTag.Value(), loggedInPubkey) {
					found = true
					break
				}
			}
			if !found {
				return false, nil
			}
		}
	}

	return true, nil
}

// checkScriptPolicy runs the policy script to determine if event should be allowed
func (p *P) checkScriptPolicy(access string, ev *event.E, scriptPath string, loggedInPubkey []byte, ipAddress string) (allowed bool, err error) {
	if p.Manager == nil || !p.Manager.IsRunning() {
		// If script is not running, default to allow
		return true, nil
	}

	// Create policy event with additional context
	policyEvent := &PolicyEvent{
		E:              ev,
		LoggedInPubkey: hex.Enc(loggedInPubkey),
		IPAddress:      ipAddress,
	}

	// Process event through policy script
	response, scriptErr := p.Manager.ProcessEvent(policyEvent)
	if chk.E(scriptErr) {
		log.E.F("policy script processing failed: %v", scriptErr)
		// Default to allow on script failure
		return true, nil
	}

	// Handle script response
	switch response.Action {
	case "accept":
		return true, nil
	case "reject":
		return false, nil
	case "shadowReject":
		return false, nil // Treat as reject for policy purposes
	default:
		log.W.F("unknown policy script action: %s", response.Action)
		// Default to allow for unknown actions
		return true, nil
	}
}

// PolicyManager methods (similar to SprocketManager)

// disablePolicy disables policy due to failure
func (pm *PolicyManager) disablePolicy() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.disabled {
		pm.disabled = true
		log.W.F("policy disabled due to failure - all events will be rejected (script location: %s)", pm.scriptPath)
	}
}

// enablePolicy re-enables policy and attempts to start it
func (pm *PolicyManager) enablePolicy() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.disabled {
		pm.disabled = false
		log.I.F("policy re-enabled, attempting to start")

		// Attempt to start policy in background
		go func() {
			if _, err := os.Stat(pm.scriptPath); err == nil {
				if err := pm.StartPolicy(); err != nil {
					log.E.F("failed to restart policy: %v", err)
					pm.disablePolicy()
				} else {
					log.I.F("policy restarted successfully")
				}
			} else {
				log.W.F("policy script still not found, keeping disabled")
				pm.disablePolicy()
			}
		}()
	}
}

// periodicCheck periodically checks if policy script becomes available
func (pm *PolicyManager) periodicCheck() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.mutex.RLock()
			disabled := pm.disabled
			running := pm.isRunning
			pm.mutex.RUnlock()

			// Only check if policy is disabled or not running
			if disabled || !running {
				if _, err := os.Stat(pm.scriptPath); err == nil {
					// Script is available, try to enable/restart
					if disabled {
						pm.enablePolicy()
					} else if !running {
						// Script exists but policy isn't running, try to start
						go func() {
							if err := pm.StartPolicy(); err != nil {
								log.E.F("failed to restart policy: %v", err)
								pm.disablePolicy()
							} else {
								log.I.F("policy restarted successfully")
							}
						}()
					}
				}
			}
		}
	}
}

// startPolicyIfExists starts the policy script if the file exists
func (pm *PolicyManager) startPolicyIfExists() {
	if _, err := os.Stat(pm.scriptPath); err == nil {
		if err := pm.StartPolicy(); err != nil {
			log.E.F("failed to start policy: %v", err)
			pm.disablePolicy()
		}
	} else {
		log.W.F("policy script not found at %s, disabling policy", pm.scriptPath)
		pm.disablePolicy()
	}
}

// StartPolicy starts the policy script
func (pm *PolicyManager) StartPolicy() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.isRunning {
		return fmt.Errorf("policy is already running")
	}

	if _, err := os.Stat(pm.scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("policy script does not exist")
	}

	// Create a new context for this command
	cmdCtx, cmdCancel := context.WithCancel(pm.ctx)

	// Make the script executable
	if err := os.Chmod(pm.scriptPath, 0755); chk.E(err) {
		cmdCancel()
		return fmt.Errorf("failed to make script executable: %v", err)
	}

	// Start the script
	cmd := exec.CommandContext(cmdCtx, pm.scriptPath)
	cmd.Dir = pm.configDir

	// Set up stdio pipes for communication
	stdin, err := cmd.StdinPipe()
	if chk.E(err) {
		cmdCancel()
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if chk.E(err) {
		cmdCancel()
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if chk.E(err) {
		cmdCancel()
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); chk.E(err) {
		cmdCancel()
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to start policy: %v", err)
	}

	pm.currentCmd = cmd
	pm.currentCancel = cmdCancel
	pm.stdin = stdin
	pm.stdout = stdout
	pm.stderr = stderr
	pm.isRunning = true

	// Start response reader in background
	go pm.readResponses()

	// Log stderr output in background
	go pm.logOutput(stdout, stderr)

	// Monitor the process
	go pm.monitorProcess()

	log.I.F("policy started (pid=%d)", cmd.Process.Pid)
	return nil
}

// StopPolicy stops the policy script gracefully, with SIGKILL fallback
func (pm *PolicyManager) StopPolicy() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.isRunning || pm.currentCmd == nil {
		return fmt.Errorf("policy is not running")
	}

	// Close stdin first to signal the script to exit
	if pm.stdin != nil {
		pm.stdin.Close()
	}

	// Cancel the context
	if pm.currentCancel != nil {
		pm.currentCancel()
	}

	// Wait for graceful shutdown with timeout
	done := make(chan error, 1)
	go func() {
		done <- pm.currentCmd.Wait()
	}()

	select {
	case <-done:
		// Process exited gracefully
		log.I.F("policy stopped gracefully")
	case <-time.After(5 * time.Second):
		// Force kill after 5 seconds
		log.W.F("policy did not stop gracefully, sending SIGKILL")
		if err := pm.currentCmd.Process.Kill(); chk.E(err) {
			log.E.F("failed to kill policy process: %v", err)
		}
		<-done // Wait for the kill to complete
	}

	// Clean up pipes
	if pm.stdin != nil {
		pm.stdin.Close()
		pm.stdin = nil
	}
	if pm.stdout != nil {
		pm.stdout.Close()
		pm.stdout = nil
	}
	if pm.stderr != nil {
		pm.stderr.Close()
		pm.stderr = nil
	}

	pm.isRunning = false
	pm.currentCmd = nil
	pm.currentCancel = nil

	return nil
}

// ProcessEvent sends an event to the policy script and waits for a response
func (pm *PolicyManager) ProcessEvent(evt *PolicyEvent) (*PolicyResponse, error) {
	pm.mutex.RLock()
	if !pm.isRunning || pm.stdin == nil {
		pm.mutex.RUnlock()
		return nil, fmt.Errorf("policy is not running")
	}
	stdin := pm.stdin
	pm.mutex.RUnlock()

	// Serialize the event to JSON
	eventJSON, err := json.Marshal(evt)
	if chk.E(err) {
		return nil, fmt.Errorf("failed to serialize event: %v", err)
	}

	// Send the event JSON to the policy script (newline-terminated for shell-readers)
	if _, err := stdin.Write(append(eventJSON, '\n')); chk.E(err) {
		return nil, fmt.Errorf("failed to write event to policy: %v", err)
	}

	// Wait for response with timeout
	select {
	case response := <-pm.responseChan:
		return &response, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("policy response timeout")
	case <-pm.ctx.Done():
		return nil, fmt.Errorf("policy context cancelled")
	}
}

// readResponses reads JSONL responses from the policy script
func (pm *PolicyManager) readResponses() {
	if pm.stdout == nil {
		return
	}

	scanner := bufio.NewScanner(pm.stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var response PolicyResponse
		if err := json.Unmarshal([]byte(line), &response); chk.E(err) {
			log.E.F("failed to parse policy response: %v", err)
			continue
		}

		// Send response to channel (non-blocking)
		select {
		case pm.responseChan <- response:
		default:
			log.W.F("policy response channel full, dropping response")
		}
	}

	if err := scanner.Err(); chk.E(err) {
		log.E.F("error reading policy responses: %v", err)
	}
}

// logOutput logs the output from stdout and stderr
func (pm *PolicyManager) logOutput(stdout, stderr io.ReadCloser) {
	defer stderr.Close()

	// Only log stderr, stdout is used by readResponses
	go func() {
		io.Copy(os.Stderr, stderr)
	}()
}

// monitorProcess monitors the policy process and cleans up when it exits
func (pm *PolicyManager) monitorProcess() {
	if pm.currentCmd == nil {
		return
	}

	err := pm.currentCmd.Wait()

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Clean up pipes
	if pm.stdin != nil {
		pm.stdin.Close()
		pm.stdin = nil
	}
	if pm.stdout != nil {
		pm.stdout.Close()
		pm.stdout = nil
	}
	if pm.stderr != nil {
		pm.stderr.Close()
		pm.stderr = nil
	}

	pm.isRunning = false
	pm.currentCmd = nil
	pm.currentCancel = nil

	if err != nil {
		log.E.F("policy process exited with error: %v", err)
		// Auto-disable policy on failure
		pm.disabled = true
		log.W.F("policy disabled due to process failure - all events will be rejected (script location: %s)", pm.scriptPath)
	} else {
		log.I.F("policy process exited normally")
	}
}

// IsEnabled returns whether policy is enabled
func (pm *PolicyManager) IsEnabled() bool {
	return pm.enabled
}

// IsRunning returns whether policy is currently running
func (pm *PolicyManager) IsRunning() bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.isRunning
}

// IsDisabled returns whether policy is disabled due to failure
func (pm *PolicyManager) IsDisabled() bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.disabled
}

// Shutdown gracefully shuts down the policy manager
func (pm *PolicyManager) Shutdown() {
	pm.cancel()
	if pm.isRunning {
		pm.StopPolicy()
	}
}
