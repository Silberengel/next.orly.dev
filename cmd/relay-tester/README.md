# relay-tester

A command-line tool for testing Nostr relay implementations against the NIP-01 specification and related NIPs.

## Usage

```bash
relay-tester -url <relay-url> [options]
```

## Options

- `-url` (required): Relay websocket URL (e.g., `ws://127.0.0.1:3334` or `wss://relay.example.com`)
- `-test <name>`: Run a specific test by name (default: run all tests)
- `-json`: Output results in JSON format
- `-v`: Verbose output (shows additional info for each test)
- `-list`: List all available tests and exit

## Examples

### Run all tests against a local relay:
```bash
relay-tester -url ws://127.0.0.1:3334
```

### Run all tests with verbose output:
```bash
relay-tester -url ws://127.0.0.1:3334 -v
```

### Run a specific test:
```bash
relay-tester -url ws://127.0.0.1:3334 -test "Publishes basic event"
```

### Output results as JSON:
```bash
relay-tester -url ws://127.0.0.1:3334 -json
```

### List all available tests:
```bash
relay-tester -list
```

## Exit Codes

- `0`: All required tests passed
- `1`: One or more required tests failed, or an error occurred

## Test Categories

The relay-tester runs tests covering:

- **Basic Event Operations**: Publishing, finding by ID/author/kind/tags
- **Filtering**: Time ranges, limits, multiple filters, scrape queries
- **Replaceable Events**: Metadata and contact list replacement
- **Parameterized Replaceable Events**: Addressable events with `d` tags
- **Event Deletion**: Deletion events (NIP-09)
- **Ephemeral Events**: Event handling for ephemeral kinds
- **EOSE Handling**: End of stored events signaling
- **Event Validation**: Signature verification, ID hash verification
- **JSON Compliance**: NIP-01 JSON escape sequences

## Notes

- Tests are run in dependency order (some tests depend on others)
- Required tests must pass for the relay to be considered compliant
- Optional tests may fail without affecting overall compliance
- The tool connects to the relay using WebSocket and runs tests sequentially

