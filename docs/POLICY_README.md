# ORLY Policy System

The ORLY relay includes a comprehensive policy system that allows fine-grained control over event storage and retrieval based on various criteria including event kinds, pubkeys, content, and custom script logic.

## Configuration

Enable the policy system by setting the environment variable:
```bash
export ORLY_POLICY_ENABLED=true
```

## Policy Configuration File

The policy configuration is loaded from `$HOME/.config/ORLY/policy.json`. See `example-policy.json` for a complete example.

### Structure

```json
{
  "kind": {
    "whitelist": [1, 3, 5, 7, 9735],
    "blacklist": []
  },
  "rules": {
    "1": {
      "description": "Text notes - allow all authenticated users",
      "write_allow": [],
      "write_deny": [],
      "read_allow": [],
      "read_deny": [],
      "size_limit": 32000,
      "content_limit": 10000
    }
  }
}
```

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

See `example-policy.sh` for a complete example showing:
- IP address blocking
- Content filtering
- Authentication requirements
- User-specific permissions

## Integration Points

### EVENT Processing

When policy is enabled, every EVENT envelope is checked using `CheckPolicy("write", event, loggedInPubkey, ipAddress)` before being stored.

### REQ Processing

When policy is enabled, every event returned in REQ responses is filtered using `CheckPolicy("read", event, loggedInPubkey, ipAddress)` before being sent to the client.

## Error Handling

- If policy script fails or times out, events are allowed by default
- If policy configuration is invalid, default policy (allow all) is used
- Policy script failures are logged but don't block relay operation

## Monitoring

Policy decisions are logged at debug level:
- `policy allowed event <id>`
- `policy rejected event <id>`
- `policy filtered out event <id> for read access`

## Security Considerations

- Policy scripts run with the same privileges as the relay process
- Scripts should be carefully reviewed and tested
- Consider using read-only filesystems for policy scripts in production
- Monitor script execution time to prevent DoS attacks
