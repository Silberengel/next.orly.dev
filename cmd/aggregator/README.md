# Nostr Event Aggregator

A comprehensive program that searches for all events related to a specific npub across multiple Nostr relays and outputs them in JSONL format to stdout. The program finds both events authored by the user and events that mention the user in "p" tags. It features dynamic relay discovery from relay list events and progressive backward time-based fetching for complete historical data collection.

## Usage

```bash
go run main.go -npub <npub> [-since <timestamp>] [-until <timestamp>]
```

Where:
- `<npub>` is a bech32-encoded Nostr public key (starting with "npub1")
- `<timestamp>` is a Unix timestamp (seconds since epoch) - optional

## Examples

```bash
# Get all events related to a user (authored by and mentioning)
go run main.go -npub npub1234567890abcdef...

# Get events related to a user since January 1, 2022
go run main.go -npub npub1234567890abcdef... -since 1640995200

# Get events related to a user between two dates
go run main.go -npub npub1234567890abcdef... -since 1640995200 -until 1672531200

# Get events related to a user until December 31, 2022
go run main.go -npub npub1234567890abcdef... -until 1672531200
```

## Features

- **Comprehensive event discovery**: Finds both events authored by the user and events that mention the user
- **Dynamic relay discovery**: Automatically discovers and connects to new relays from relay list events (kind 10002)
- **Progressive backward fetching**: Systematically collects historical data in time-based batches
- **Triple filter approach**: Uses separate filters for authored events, p-tag mentions, and relay list events
- **Intelligent time management**: Works backwards from current time (or until timestamp) to since timestamp
- **Memory-efficient deduplication**: Uses bloom filter with ~0.1% false positive rate instead of unbounded maps
- **Fixed memory footprint**: Bloom filter uses only ~1.75MB for 1M events with controlled memory growth
- **Memory monitoring**: Real-time memory usage tracking and automatic garbage collection
- Connects to multiple relays simultaneously with dynamic expansion
- Outputs events in JSONL format (one JSON object per line)
- Handles connection failures gracefully
- Continues running until all relay connections are closed
- Time-based filtering with Unix timestamps (since/until parameters)
- Input validation for timestamp ranges

## Event Discovery

The aggregator searches for three types of events:

1. **Authored Events**: Events where the specified npub is the author (pubkey field matches)
2. **Mentioned Events**: Events that contain "p" tags referencing the specified npub (replies, mentions, etc.)
3. **Relay List Events**: Kind 10002 events that contain relay URLs for dynamic relay discovery

This comprehensive approach ensures you capture all events related to a user, including:
- Posts authored by the user
- Replies to the user's posts
- Posts that mention or tag the user
- Any other events that reference the user in p-tags
- Relay list metadata for discovering additional relays

## Progressive Fetching

The aggregator uses an intelligent progressive backward fetching strategy:

1. **Time-based batches**: Fetches data in weekly batches working backwards from the end time
2. **Dynamic relay expansion**: As relay list events are discovered, new relays are automatically added to the search
3. **Complete coverage**: Ensures all events between since and until timestamps are collected
4. **Efficient processing**: Processes each time batch completely before moving to the next
5. **Boundary respect**: Stops when reaching the since timestamp or beginning of available data

## Memory Management

The aggregator uses advanced memory management techniques to handle large-scale data collection:

### Bloom Filter Deduplication
- **Fixed Size**: Uses exactly 1.75MB for the bloom filter regardless of event count
- **Low False Positive Rate**: Configured for ~0.1% false positive rate with 1M events
- **Hash Functions**: Uses 10 independent hash functions based on SHA256 for optimal distribution
- **Thread-Safe**: Concurrent access protected with read-write mutexes

### Memory Monitoring
- **Real-time Tracking**: Monitors total memory usage every 30 seconds
- **Automatic GC**: Triggers garbage collection when approaching memory limits
- **Statistics Logging**: Reports bloom filter usage, estimated event count, and memory consumption
- **Controlled Growth**: Prevents unbounded memory growth through fixed-size data structures

### Performance Characteristics
- **Memory Usage**: ~1.75MB bloom filter + ~256MB total memory limit
- **False Positives**: ~0.1% chance of incorrectly identifying a duplicate (very low impact)
- **Scalability**: Can handle millions of events without memory issues
- **Efficiency**: O(k) time complexity for both add and lookup operations (k = hash functions)

## Relays

The program starts with the following initial relays:

- wss://nostr.wine/
- wss://nostr.land/
- wss://orly-relay.imwald.eu
- wss://relay.orly.dev/
- wss://relay.damus.io/
- wss://nos.lol/
- wss://theforest.nostr1.com/

**Dynamic Relay Discovery**: Additional relays are automatically discovered and added during execution when the program finds relay list events (kind 10002) authored by the target user. This ensures comprehensive coverage across the user's preferred relay network.

## Output Format

Each line of output is a JSON object representing a Nostr event with the following fields:

- `id`: Event ID (hex)
- `pubkey`: Author's public key (hex)
- `created_at`: Unix timestamp
- `kind`: Event kind number
- `tags`: Array of tag arrays
- `content`: Event content string
- `sig`: Event signature (hex)
