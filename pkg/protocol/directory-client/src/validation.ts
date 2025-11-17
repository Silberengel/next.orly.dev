/**
 * Validation functions for the Distributed Directory Consensus Protocol
 * 
 * This module provides validation matching the Go implementation in
 * pkg/protocol/directory/validation.go
 */

import type { IdentityTag } from './types.js';
import { TrustLevel, KeyPurpose, ReplicationStatus } from './types.js';

/**
 * Validation error class
 */
export class ValidationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ValidationError';
  }
}

// Regular expressions for validation
const HEX_KEY_REGEX = /^[0-9a-fA-F]{64}$/;
const NPUB_REGEX = /^npub1[0-9a-z]+$/;
const WS_URL_REGEX = /^wss?:\/\/[a-zA-Z0-9.-]+(?::[0-9]+)?(?:\/.*)?$/;

/**
 * Validates that a string is a valid 64-character hex key
 */
export function validateHexKey(key: string): void {
  if (!HEX_KEY_REGEX.test(key)) {
    throw new ValidationError('invalid hex key format: must be 64 hex characters');
  }
}

/**
 * Validates that a string is a valid npub-encoded public key
 */
export function validateNPub(npub: string): void {
  if (!NPUB_REGEX.test(npub)) {
    throw new ValidationError('invalid npub format');
  }
  
  // Additional validation would require bech32 decoding
  // which should be handled by applesauce-core utilities
}

/**
 * Validates that a string is a valid WebSocket URL
 */
export function validateWebSocketURL(url: string): void {
  if (!WS_URL_REGEX.test(url)) {
    throw new ValidationError('invalid WebSocket URL format');
  }
  
  try {
    const parsed = new URL(url);
    
    if (parsed.protocol !== 'ws:' && parsed.protocol !== 'wss:') {
      throw new ValidationError('URL must use ws:// or wss:// scheme');
    }
    
    if (!parsed.host) {
      throw new ValidationError('URL must have a host');
    }
    
    // Ensure trailing slash for canonical format
    if (!url.endsWith('/')) {
      throw new ValidationError('Canonical WebSocket URL must end with /');
    }
  } catch (err) {
    if (err instanceof ValidationError) {
      throw err;
    }
    throw new ValidationError(`invalid URL: ${err instanceof Error ? err.message : String(err)}`);
  }
}

/**
 * Validates a nonce meets minimum security requirements
 */
export function validateNonce(nonce: string): void {
  const MIN_NONCE_SIZE = 16; // bytes
  
  if (nonce.length < MIN_NONCE_SIZE * 2) { // hex encoding doubles length
    throw new ValidationError(`nonce must be at least ${MIN_NONCE_SIZE} bytes (${MIN_NONCE_SIZE * 2} hex characters)`);
  }
  
  if (!/^[0-9a-fA-F]+$/.test(nonce)) {
    throw new ValidationError('nonce must be valid hex');
  }
}

/**
 * Validates trust level value
 */
export function validateTrustLevel(level: string): void {
  if (!Object.values(TrustLevel).includes(level as TrustLevel)) {
    throw new ValidationError(`invalid trust level: must be one of ${Object.values(TrustLevel).join(', ')}`);
  }
}

/**
 * Validates key purpose value
 */
export function validateKeyPurpose(purpose: string): void {
  if (!Object.values(KeyPurpose).includes(purpose as KeyPurpose)) {
    throw new ValidationError(`invalid key purpose: must be one of ${Object.values(KeyPurpose).join(', ')}`);
  }
}

/**
 * Validates replication status value
 */
export function validateReplicationStatus(status: string): void {
  if (!Object.values(ReplicationStatus).includes(status as ReplicationStatus)) {
    throw new ValidationError(`invalid replication status: must be one of ${Object.values(ReplicationStatus).join(', ')}`);
  }
}

/**
 * Validates confidence value (must be between 0.0 and 1.0)
 */
export function validateConfidence(confidence: number): void {
  if (confidence < 0.0 || confidence > 1.0) {
    throw new ValidationError('confidence must be between 0.0 and 1.0');
  }
}

/**
 * Validates an identity tag structure
 * 
 * Note: This performs structural validation only. Signature verification
 * requires cryptographic operations and should be done separately.
 */
export function validateIdentityTagStructure(tag: IdentityTag): void {
  if (!tag.identity) {
    throw new ValidationError('identity tag must have an identity field');
  }
  
  validateHexKey(tag.identity);
  
  if (!tag.delegate) {
    throw new ValidationError('identity tag must have a delegate field');
  }
  
  validateHexKey(tag.delegate);
  
  if (!tag.signature) {
    throw new ValidationError('identity tag must have a signature field');
  }
  
  validateHexKey(tag.signature);
  
  if (tag.relayHint) {
    validateWebSocketURL(tag.relayHint);
  }
}

/**
 * Validates event content is valid JSON
 */
export function validateJSONContent(content: string): void {
  if (!content || content.trim() === '') {
    return; // Empty content is valid
  }
  
  try {
    JSON.parse(content);
  } catch (err) {
    throw new ValidationError(`invalid JSON content: ${err instanceof Error ? err.message : String(err)}`);
  }
}

/**
 * Validates a timestamp is in the past
 */
export function validatePastTimestamp(timestamp: Date | number): void {
  const now = Date.now();
  const ts = timestamp instanceof Date ? timestamp.getTime() : timestamp * 1000;
  
  if (ts > now) {
    throw new ValidationError('timestamp must be in the past');
  }
}

/**
 * Validates a timestamp is in the future
 */
export function validateFutureTimestamp(timestamp: Date | number): void {
  const now = Date.now();
  const ts = timestamp instanceof Date ? timestamp.getTime() : timestamp * 1000;
  
  if (ts <= now) {
    throw new ValidationError('timestamp must be in the future');
  }
}

/**
 * Validates an expiry timestamp (must be in the future if provided)
 */
export function validateExpiry(expiry?: Date | number): void {
  if (expiry === undefined || expiry === null) {
    return; // No expiry is valid
  }
  
  validateFutureTimestamp(expiry);
}

/**
 * Validates a BIP32 derivation path
 */
export function validateDerivationPath(path: string): void {
  // Basic validation - should start with m/ and contain numbers/apostrophes
  if (!/^m(\/\d+'?)*$/.test(path)) {
    throw new ValidationError('invalid BIP32 derivation path format');
  }
}

/**
 * Validates a key index is non-negative
 */
export function validateKeyIndex(index: number): void {
  if (!Number.isInteger(index) || index < 0) {
    throw new ValidationError('key index must be a non-negative integer');
  }
}

/**
 * Validates event kinds array is not empty
 */
export function validateEventKinds(kinds: number[]): void {
  if (!Array.isArray(kinds) || kinds.length === 0) {
    throw new ValidationError('event kinds array must not be empty');
  }
  
  for (const kind of kinds) {
    if (!Number.isInteger(kind) || kind < 0) {
      throw new ValidationError(`invalid event kind: ${kind}`);
    }
  }
}

/**
 * Validates authors array contains valid pubkeys
 */
export function validateAuthors(authors: string[]): void {
  if (!Array.isArray(authors)) {
    throw new ValidationError('authors must be an array');
  }
  
  for (const author of authors) {
    validateHexKey(author);
  }
}

/**
 * Validates limit is positive
 */
export function validateLimit(limit: number): void {
  if (!Number.isInteger(limit) || limit <= 0) {
    throw new ValidationError('limit must be a positive integer');
  }
}

