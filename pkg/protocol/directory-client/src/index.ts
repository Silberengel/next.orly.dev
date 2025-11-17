/**
 * Directory Consensus Protocol Client Library
 * 
 * Main entry point for the TypeScript client library.
 */

// Export types
export type {
  IdentityTag,
  RelayIdentity,
  TrustAct,
  GroupTagAct,
  PublicKeyAdvertisement,
  ReplicationRequest,
  ReplicationResponse,
  DirectoryEventContent,
  ValidationError as ValidationErrorType,
} from './types.js';

export {
  EventKinds,
  TrustLevel,
  TrustReason,
  KeyPurpose,
  ReplicationStatus,
  isDirectoryEventKind,
  isValidTrustLevel,
  isValidKeyPurpose,
  isValidReplicationStatus,
} from './types.js';

// Export validation
export {
  ValidationError,
  validateHexKey,
  validateNPub,
  validateWebSocketURL,
  validateNonce,
  validateTrustLevel,
  validateKeyPurpose,
  validateReplicationStatus,
  validateConfidence,
  validateIdentityTagStructure,
  validateJSONContent,
  validatePastTimestamp,
  validateFutureTimestamp,
  validateExpiry,
  validateDerivationPath,
  validateKeyIndex,
  validateEventKinds,
  validateAuthors,
  validateLimit,
} from './validation.js';

// Export parsers
export {
  parseIdentityTag,
  parseRelayIdentity,
  parseTrustAct,
  parseGroupTagAct,
  parsePublicKeyAdvertisement,
  parseReplicationRequest,
  parseReplicationResponse,
  parseDirectoryEvent,
} from './parsers.js';

// Export identity resolver
export {
  IdentityResolver,
  createIdentityResolver,
} from './identity-resolver.js';

// Export helpers
export * from './helpers.js';

