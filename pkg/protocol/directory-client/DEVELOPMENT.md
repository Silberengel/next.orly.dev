# Directory Client Library

TypeScript client library for the Nostr Distributed Directory Consensus Protocol (NIP-XX).

## Project Structure

```
src/
├── index.ts              # Main entry point
├── types.ts              # Type definitions
├── validation.ts         # Validation functions
├── parsers.ts            # Event parsers for all kinds
├── identity-resolver.ts  # Identity & delegation management
└── helpers.ts            # Utility functions
```

## Installation

```bash
cd pkg/protocol/directory-client
npm install
```

## Building

```bash
npm run build
```

## Development

```bash
npm run dev  # Watch mode
```

## Testing

```bash
npm test
```

## Features Implemented

- ✅ Type definitions for all directory event kinds (39100-39105)
- ✅ Event parsers with validation
- ✅ Identity tag handling and resolution
- ✅ Key delegation management
- ✅ Trust calculation utilities
- ✅ Replication filtering
- ✅ Comprehensive validation matching Go implementation

## Usage Example

See the main [README.md](./README.md) for detailed usage examples.

