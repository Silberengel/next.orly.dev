# ORLY Policy System

The ORLY relay includes a comprehensive policy system that allows fine-grained control over event storage and retrieval based on various criteria including event kinds, pubkeys, content, and custom script logic.

## Configuration

Enable the policy system by setting the environment variable:
```bash
export ORLY_POLICY_ENABLED=true
```

## Policy Configuration File

The policy configuration is loaded from `$HOME/.config/ORLY/policy.json`. See `docs/example-policy.json` for a complete example with global rules and age validation.

### Structure

```json
{
  "default_policy": "allow",
  "kind": {
    "whitelist": [1, 3, 5, 7, 9735],
    "blacklist": []
  },
  "global": {
    "description": "Global rules applied to all events",
    "write_allow": [],
    "write_deny": [],
    "read_allow": [],
    "read_deny": [],
    "size_limit": 100000,
    "content_limit": 50000,
    "max_age_of_event": 86400,
    "max_age_event_in_future": 300
  },
  "rules": {
    "1": {
      "description": "Text notes - allow all authenticated users",
      "write_allow": [],
      "write_deny": [],
      "read_allow": [],
      "read_deny": [],
      "size_limit": 32000,
      "content_limit": 10000,
      "max_age_of_event": 3600,
      "max_age_event_in_future": 60
    }
  }
}
```

### Default Policy

The `default_policy` field determines the default behavior when no specific rules deny an event:

- `"allow"` (default): Events are allowed unless explicitly denied by rules
- `"deny"`: Events are denied unless explicitly allowed by rules

This applies to:
- Events of whitelisted kinds that have no specific rules
- Events that pass all other policy checks but have no explicit allow/deny decision

### Policy Evaluation Order

The policy system evaluates events in the following order:

1. **Global Rules** - Applied to all events first
2. **Kinds Filtering** - Whitelist/blacklist by event kind
3. **Kind-specific Rules** - Rules for specific event kinds
4. **Script Rules** - Custom script logic (if enabled)
5. **Default Policy** - Applied when no rules make a decision

### Global Rules

The `global` section defines rules that apply to **all events** regardless of their kind. These rules are evaluated **first** and take precedence over kind-specific rules.

Global rules support all the same fields as kind-specific rules, allowing you to:
- Set site-wide size limits
- Block specific pubkeys globally
- Enforce age restrictions on all events
- Apply content limits across all event types

### Kinds Filtering

- `whitelist`: If present, only these event kinds are allowed. All others are denied.
- `blacklist`: If present, these event kinds are denied. All others are allowed.
- If both are empty, all kinds are allowed.

### Rule Fields

- `description`: Human-readable description of the rule
- `script`: Path to a script for custom logic (overrides other criteria)
- `write_allow`: List of pubkeys allowed to write this kind
- `write_deny`: List of pubkeys denied from writing this kind
- `read_allow`: List of pubkeys allowed to read this kind
- `read_deny`: List of pubkeys denied from reading this kind
- `max_expiry`: Maximum expiry time in seconds for events
- `must_have_tags`: List of tag keys that must be present
- `size_limit`: Maximum total event size in bytes
- `content_limit`: Maximum content field size in bytes
- `privileged`: If true, event must be authored by authenticated user or contain authenticated user in p tags
- `rate_limit`: Rate limit in bytes per second (not yet implemented)
- `max_age_of_event`: Maximum age of event in seconds (prevents replay attacks)
- `max_age_event_in_future`: Maximum time event can be in the future in seconds (prevents clock skew attacks)

### Age Validation

The policy system includes built-in timestamp validation to prevent common attacks:

#### MaxAgeOfEvent
- **Purpose**: Prevents replay attacks by rejecting events that are too old
- **Behavior**: Events with `created_at` older than `current_time - max_age_of_event` are rejected
- **Example**: Setting `max_age_of_event: 3600` rejects events older than 1 hour
- **Use Cases**: 
  - Prevent replay of old events
  - Ensure events are recent and relevant
  - Reduce storage of stale data

#### MaxAgeEventInFuture
- **Purpose**: Prevents clock skew attacks by rejecting events too far in the future
- **Behavior**: Events with `created_at` newer than `current_time + max_age_event_in_future` are rejected
- **Example**: Setting `max_age_event_in_future: 300` rejects events more than 5 minutes in the future
- **Use Cases**:
  - Prevent clock manipulation attacks
  - Ensure reasonable timestamp accuracy
  - Block events with impossible future timestamps

#### Age Validation Examples

```json
{
  "global": {
    "max_age_of_event": 86400,        // Reject events older than 24 hours
    "max_age_event_in_future": 300   // Reject events more than 5 minutes in future
  },
  "rules": {
    "1": {
      "max_age_of_event": 3600,      // Text notes: reject older than 1 hour
      "max_age_event_in_future": 60  // Text notes: reject more than 1 minute in future
    },
    "4": {
      "max_age_of_event": 604800     // Direct messages: reject older than 7 days
    }
  }
}
```

## Policy Scripts

For advanced policy logic, you can use custom scripts. The script should be placed at `$HOME/.config/ORLY/policy.sh` and made executable.

### Script Interface

The script receives JSON events via stdin and outputs JSON responses via stdout. Each event includes:

- All original event fields
- `logged_in_pubkey`: Hex-encoded authenticated user's pubkey (if any)
- `ip_address`: Client's IP address

### Response Format

```json
{"id": "event_id", "action": "accept|reject|shadowReject", "msg": "optional message"}
```

### Example Script

See `docs/example-policy.sh` for a complete example showing:
- IP address blocking
- Content filtering
- Authentication requirements
- User-specific permissions

## Integration Points

### EVENT Processing

When policy is enabled, every EVENT envelope is checked using `CheckPolicy("write", event, loggedInPubkey, ipAddress)` before being stored. The policy evaluation follows this order:

1. **Global Rules** - Applied first to all events
2. **Kinds Filtering** - Whitelist/blacklist check
3. **Kind-specific Rules** - Rules for the event's kind
4. **Script Rules** - Custom script logic (if enabled)

### REQ Processing

When policy is enabled, every event returned in REQ responses is filtered using `CheckPolicy("read", event, loggedInPubkey, ipAddress)` before being sent to the client. The same evaluation order applies for read access.

## Script Resilience

The policy system is designed to be resilient to script failures:

### Automatic Recovery
- Policy scripts are automatically restarted if they crash or fail to load
- The system continuously monitors script health and attempts recovery every 60 seconds (1 minute)
- Script failures don't disable the entire policy system

### Fallback Behavior
When a policy script fails or is not running:
- Events that would have been processed by the script fall back to the `default_policy`
- The system logs which policy rule is inactive and the fallback behavior
- Other policy rules (global, kinds, non-script rules) continue to function normally

### Error Handling

- If policy script fails or times out, events fall back to `default_policy` setting
- If policy configuration is invalid, default policy (allow all) is used  
- Policy script failures are logged with specific rule information but don't block relay operation

## Monitoring

Policy decisions and script health are logged:

### Policy Decisions
- `policy allowed event <id>`
- `policy rejected event <id>`

### Script Health
- `policy rule for kind <N> is inactive (script not running), falling back to default policy (<policy>)`
- `policy rule for kind <N> failed (script processing error: <error>), falling back to default policy (<policy>)`
- `policy rule for kind <N> returned unknown action '<action>', falling back to default policy (<policy>)`
- `policy script not found at <path>, will retry periodically`
- `policy script crashed - events will fall back to default policy until restart`
- `policy filtered out event <id> for read access`

## Best Practices

### Global Rules
- Use global rules for site-wide security policies (size limits, age restrictions)
- Keep global rules simple and broad to avoid unintended side effects
- Test global rules thoroughly as they affect all events

### Age Validation
- Set reasonable age limits based on your use case:
  - **Text notes (kind 1)**: 1-24 hours max age, 1-5 minutes future tolerance
  - **Direct messages (kind 4)**: 7-30 days max age, 1-5 minutes future tolerance
  - **Replaceable events (kind 0, 3)**: Longer max age, shorter future tolerance
- Consider network latency when setting future tolerance
- Monitor rejected events to tune age limits appropriately

### Policy Hierarchy
- Global rules should be broader than kind-specific rules
- Use global rules for security, kind-specific rules for functionality
- Avoid conflicting rules between global and kind-specific policies

## Security Considerations

- Policy scripts run with the same privileges as the relay process
- Scripts should be carefully reviewed and tested
- Consider using read-only filesystems for policy scripts in production
- Monitor script execution time to prevent DoS attacks
- Age validation helps prevent replay and clock skew attacks
- Global rules provide defense-in-depth security
