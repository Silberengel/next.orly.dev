/**
 * Event parsers for the Distributed Directory Consensus Protocol
 * 
 * This module provides parsers for all directory event kinds (39100-39105)
 * matching the Go implementation in pkg/protocol/directory/
 */

import type { NostrEvent } from 'applesauce-core/helpers';
import type {
  IdentityTag,
  RelayIdentity,
  TrustAct,
  GroupTagAct,
  PublicKeyAdvertisement,
  ReplicationRequest,
  ReplicationResponse,
} from './types.js';
import {
  EventKinds,
  TrustLevel,
  TrustReason,
  KeyPurpose,
  ReplicationStatus,
} from './types.js';
import {
  ValidationError,
  validateHexKey,
  validateWebSocketURL,
  validateTrustLevel,
  validateKeyPurpose,
  validateReplicationStatus,
  validateIdentityTagStructure,
} from './validation.js';

/**
 * Helper to get a tag value by name
 */
function getTagValue(event: NostrEvent, tagName: string): string | undefined {
  const tag = event.tags.find(t => t[0] === tagName);
  return tag?.[1];
}

/**
 * Helper to get all tag values by name
 */
function getTagValues(event: NostrEvent, tagName: string): string[] {
  return event.tags.filter(t => t[0] === tagName).map(t => t[1]);
}

/**
 * Helper to parse a timestamp tag
 */
function parseTimestamp(value: string | undefined): Date | undefined {
  if (!value) return undefined;
  const timestamp = parseInt(value, 10);
  if (isNaN(timestamp)) return undefined;
  return new Date(timestamp * 1000);
}

/**
 * Helper to parse a number tag
 */
function parseNumber(value: string | undefined): number | undefined {
  if (!value) return undefined;
  const num = parseFloat(value);
  return isNaN(num) ? undefined : num;
}

/**
 * Parse an Identity Tag (I tag) from an event
 * 
 * Format: ["I", <identity>, <delegate>, <signature>, <relay_hint>]
 */
export function parseIdentityTag(event: NostrEvent): IdentityTag | undefined {
  const iTag = event.tags.find(t => t[0] === 'I');
  if (!iTag) return undefined;
  
  const [, identity, delegate, signature, relayHint] = iTag;
  
  if (!identity || !delegate || !signature) {
    throw new ValidationError('invalid I tag format: missing required fields');
  }
  
  const tag: IdentityTag = {
    identity,
    delegate,
    signature,
    relayHint: relayHint || undefined,
  };
  
  validateIdentityTagStructure(tag);
  
  return tag;
}

/**
 * Parse a Relay Identity Declaration (Kind 39100)
 */
export function parseRelayIdentity(event: NostrEvent): RelayIdentity {
  if (event.kind !== EventKinds.RelayIdentityAnnouncement) {
    throw new ValidationError(`invalid event kind: expected ${EventKinds.RelayIdentityAnnouncement}, got ${event.kind}`);
  }
  
  const relayURL = getTagValue(event, 'relay');
  if (!relayURL) {
    throw new ValidationError('relay tag is required');
  }
  validateWebSocketURL(relayURL);
  
  const signingKey = getTagValue(event, 'signing_key');
  if (!signingKey) {
    throw new ValidationError('signing_key tag is required');
  }
  validateHexKey(signingKey);
  
  const encryptionKey = getTagValue(event, 'encryption_key');
  if (!encryptionKey) {
    throw new ValidationError('encryption_key tag is required');
  }
  validateHexKey(encryptionKey);
  
  const version = getTagValue(event, 'version');
  if (!version) {
    throw new ValidationError('version tag is required');
  }
  
  const nip11URL = getTagValue(event, 'nip11_url');
  const identityTag = parseIdentityTag(event);
  
  return {
    event,
    relayURL,
    signingKey,
    encryptionKey,
    version,
    nip11URL,
    identityTag,
  };
}

/**
 * Parse a Trust Act (Kind 39101)
 */
export function parseTrustAct(event: NostrEvent): TrustAct {
  if (event.kind !== EventKinds.TrustAct) {
    throw new ValidationError(`invalid event kind: expected ${EventKinds.TrustAct}, got ${event.kind}`);
  }
  
  const targetPubkey = getTagValue(event, 'p');
  if (!targetPubkey) {
    throw new ValidationError('p tag (target pubkey) is required');
  }
  validateHexKey(targetPubkey);
  
  const trustLevelStr = getTagValue(event, 'trust_level');
  if (!trustLevelStr) {
    throw new ValidationError('trust_level tag is required');
  }
  validateTrustLevel(trustLevelStr);
  const trustLevel = trustLevelStr as TrustLevel;
  
  const expiry = parseTimestamp(getTagValue(event, 'expiry'));
  
  const reasonStr = getTagValue(event, 'reason');
  const reason = reasonStr ? (reasonStr as TrustReason) : undefined;
  
  const notes = event.content || undefined;
  const identityTag = parseIdentityTag(event);
  
  return {
    event,
    targetPubkey,
    trustLevel,
    expiry,
    reason,
    notes,
    identityTag,
  };
}

/**
 * Parse a Group Tag Act (Kind 39102)
 */
export function parseGroupTagAct(event: NostrEvent): GroupTagAct {
  if (event.kind !== EventKinds.GroupTagAct) {
    throw new ValidationError(`invalid event kind: expected ${EventKinds.GroupTagAct}, got ${event.kind}`);
  }
  
  const targetPubkey = getTagValue(event, 'p');
  if (!targetPubkey) {
    throw new ValidationError('p tag (target pubkey) is required');
  }
  validateHexKey(targetPubkey);
  
  const groupTag = getTagValue(event, 'group_tag');
  if (!groupTag) {
    throw new ValidationError('group_tag tag is required');
  }
  
  const actor = getTagValue(event, 'actor');
  if (!actor) {
    throw new ValidationError('actor tag is required');
  }
  validateHexKey(actor);
  
  const confidence = parseNumber(getTagValue(event, 'confidence'));
  const expiry = parseTimestamp(getTagValue(event, 'expiry'));
  const notes = event.content || undefined;
  const identityTag = parseIdentityTag(event);
  
  return {
    event,
    targetPubkey,
    groupTag,
    actor,
    confidence,
    expiry,
    notes,
    identityTag,
  };
}

/**
 * Parse a Public Key Advertisement (Kind 39103)
 */
export function parsePublicKeyAdvertisement(event: NostrEvent): PublicKeyAdvertisement {
  if (event.kind !== EventKinds.PublicKeyAdvertisement) {
    throw new ValidationError(`invalid event kind: expected ${EventKinds.PublicKeyAdvertisement}, got ${event.kind}`);
  }
  
  const keyID = getTagValue(event, 'd');
  if (!keyID) {
    throw new ValidationError('d tag (key ID) is required');
  }
  
  const publicKey = getTagValue(event, 'p');
  if (!publicKey) {
    throw new ValidationError('p tag (public key) is required');
  }
  validateHexKey(publicKey);
  
  const purposeStr = getTagValue(event, 'purpose');
  if (!purposeStr) {
    throw new ValidationError('purpose tag is required');
  }
  validateKeyPurpose(purposeStr);
  const purpose = purposeStr as KeyPurpose;
  
  const expiry = parseTimestamp(getTagValue(event, 'expiration'));
  
  const algorithm = getTagValue(event, 'algorithm');
  if (!algorithm) {
    throw new ValidationError('algorithm tag is required');
  }
  
  const derivationPath = getTagValue(event, 'derivation_path');
  if (!derivationPath) {
    throw new ValidationError('derivation_path tag is required');
  }
  
  const keyIndexStr = getTagValue(event, 'key_index');
  if (!keyIndexStr) {
    throw new ValidationError('key_index tag is required');
  }
  const keyIndex = parseInt(keyIndexStr, 10);
  if (isNaN(keyIndex)) {
    throw new ValidationError('key_index must be a valid integer');
  }
  
  const identityTag = parseIdentityTag(event);
  
  return {
    event,
    keyID,
    publicKey,
    purpose,
    expiry,
    algorithm,
    derivationPath,
    keyIndex,
    identityTag,
  };
}

/**
 * Parse a Replication Request (Kind 39104)
 */
export function parseReplicationRequest(event: NostrEvent): ReplicationRequest {
  if (event.kind !== EventKinds.DirectoryEventReplicationRequest) {
    throw new ValidationError(`invalid event kind: expected ${EventKinds.DirectoryEventReplicationRequest}, got ${event.kind}`);
  }
  
  const requestID = getTagValue(event, 'request_id');
  if (!requestID) {
    throw new ValidationError('request_id tag is required');
  }
  
  const requestorRelay = getTagValue(event, 'relay');
  if (!requestorRelay) {
    throw new ValidationError('relay tag (requestor) is required');
  }
  validateWebSocketURL(requestorRelay);
  
  // Parse content as JSON for filter parameters
  let content: any = {};
  if (event.content) {
    try {
      content = JSON.parse(event.content);
    } catch (err) {
      throw new ValidationError('invalid JSON content in replication request');
    }
  }
  
  const targetRelay = content.target_relay || getTagValue(event, 'target_relay');
  if (!targetRelay) {
    throw new ValidationError('target_relay is required');
  }
  validateWebSocketURL(targetRelay);
  
  const kinds = content.kinds || [];
  if (!Array.isArray(kinds) || kinds.length === 0) {
    throw new ValidationError('kinds array is required and must not be empty');
  }
  
  const authors = content.authors;
  const since = content.since ? new Date(content.since * 1000) : undefined;
  const until = content.until ? new Date(content.until * 1000) : undefined;
  const limit = content.limit;
  
  const identityTag = parseIdentityTag(event);
  
  return {
    event,
    requestID,
    requestorRelay,
    targetRelay,
    kinds,
    authors,
    since,
    until,
    limit,
    identityTag,
  };
}

/**
 * Parse a Replication Response (Kind 39105)
 */
export function parseReplicationResponse(event: NostrEvent): ReplicationResponse {
  if (event.kind !== EventKinds.DirectoryEventReplicationResponse) {
    throw new ValidationError(`invalid event kind: expected ${EventKinds.DirectoryEventReplicationResponse}, got ${event.kind}`);
  }
  
  const requestID = getTagValue(event, 'request_id');
  if (!requestID) {
    throw new ValidationError('request_id tag is required');
  }
  
  const statusStr = getTagValue(event, 'status');
  if (!statusStr) {
    throw new ValidationError('status tag is required');
  }
  validateReplicationStatus(statusStr);
  const status = statusStr as ReplicationStatus;
  
  const eventIDs = getTagValues(event, 'event_id');
  const error = getTagValue(event, 'error');
  const identityTag = parseIdentityTag(event);
  
  return {
    event,
    requestID,
    status,
    eventIDs,
    error,
    identityTag,
  };
}

/**
 * Parse any directory event based on its kind
 */
export function parseDirectoryEvent(event: NostrEvent): 
  | RelayIdentity 
  | TrustAct 
  | GroupTagAct 
  | PublicKeyAdvertisement 
  | ReplicationRequest 
  | ReplicationResponse {
  switch (event.kind) {
    case EventKinds.RelayIdentityAnnouncement:
      return parseRelayIdentity(event);
    case EventKinds.TrustAct:
      return parseTrustAct(event);
    case EventKinds.GroupTagAct:
      return parseGroupTagAct(event);
    case EventKinds.PublicKeyAdvertisement:
      return parsePublicKeyAdvertisement(event);
    case EventKinds.DirectoryEventReplicationRequest:
      return parseReplicationRequest(event);
    case EventKinds.DirectoryEventReplicationResponse:
      return parseReplicationResponse(event);
    default:
      throw new ValidationError(`unknown directory event kind: ${event.kind}`);
  }
}

