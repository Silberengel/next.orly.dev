/**
 * Core types for the Distributed Directory Consensus Protocol (NIP-XX)
 * 
 * This module defines TypeScript types that match the Go implementation
 * in pkg/protocol/directory/types.go
 */

import type { NostrEvent } from 'applesauce-core/helpers';

// Event kinds for the distributed directory consensus protocol
export const EventKinds = {
  RelayIdentityAnnouncement: 39100,
  TrustAct: 39101,
  GroupTagAct: 39102,
  PublicKeyAdvertisement: 39103,
  DirectoryEventReplicationRequest: 39104,
  DirectoryEventReplicationResponse: 39105,
} as const;

export type DirectoryEventKind = typeof EventKinds[keyof typeof EventKinds];

// Trust levels for trust acts
export enum TrustLevel {
  High = 'high',
  Medium = 'medium',
  Low = 'low',
}

// Reason types for trust establishment
export enum TrustReason {
  Manual = 'manual',
  Reciprocal = 'reciprocal',
  Transitive = 'transitive',
  Vouched = 'vouched',
}

// Key purposes for public key advertisements
export enum KeyPurpose {
  Signing = 'signing',
  Encryption = 'encryption',
  Authentication = 'authentication',
}

// Replication statuses
export enum ReplicationStatus {
  Pending = 'pending',
  InProgress = 'in_progress',
  Completed = 'completed',
  Failed = 'failed',
  PartialSuccess = 'partial_success',
}

/**
 * Identity Tag (I tag) structure
 * 
 * Binds an identity to a delegate public key with proof-of-control signature.
 * Format: ["I", <identity_pubkey>, <delegate_pubkey>, <signature>, <relay_hint>]
 */
export interface IdentityTag {
  /** The primary identity public key (hex) */
  identity: string;
  
  /** The delegate public key used for signing (hex) */
  delegate: string;
  
  /** Schnorr signature proving control of the identity key */
  signature: string;
  
  /** Optional relay hint for finding the identity's events */
  relayHint?: string;
}

/**
 * Relay Identity Declaration (Kind 39100)
 * 
 * Announces a relay's identity and associated keys.
 */
export interface RelayIdentity {
  /** The underlying Nostr event */
  event: NostrEvent;
  
  /** Canonical WebSocket URL of the relay (must end with /) */
  relayURL: string;
  
  /** Public key for event signing (hex) */
  signingKey: string;
  
  /** Public key for NIP-04/NIP-44 encryption (hex) */
  encryptionKey: string;
  
  /** Protocol version */
  version: string;
  
  /** NIP-11 relay information document URL */
  nip11URL?: string;
  
  /** Identity tag binding this key to a primary identity */
  identityTag?: IdentityTag;
}

/**
 * Trust Act (Kind 39101)
 * 
 * Establishes trust relationship between relays.
 */
export interface TrustAct {
  /** The underlying Nostr event */
  event: NostrEvent;
  
  /** Public key of the relay being trusted (hex) */
  targetPubkey: string;
  
  /** Level of trust being granted */
  trustLevel: TrustLevel;
  
  /** When this trust expires */
  expiry?: Date;
  
  /** Reason for establishing trust */
  reason?: TrustReason;
  
  /** Additional context or notes */
  notes?: string;
  
  /** Identity tag if signed by a delegate */
  identityTag?: IdentityTag;
}

/**
 * Group Tag Act (Kind 39102)
 * 
 * Attests to a relay's membership in a named group.
 */
export interface GroupTagAct {
  /** The underlying Nostr event */
  event: NostrEvent;
  
  /** Public key of the relay being attested (hex) */
  targetPubkey: string;
  
  /** Name of the group */
  groupTag: string;
  
  /** Public key of the actor making the attestation (hex) */
  actor: string;
  
  /** Confidence level (0.0 to 1.0) */
  confidence?: number;
  
  /** When this attestation expires */
  expiry?: Date;
  
  /** Additional context or notes */
  notes?: string;
  
  /** Identity tag if signed by a delegate */
  identityTag?: IdentityTag;
}

/**
 * Public Key Advertisement (Kind 39103)
 * 
 * Advertises HD-derived public keys for specific purposes.
 */
export interface PublicKeyAdvertisement {
  /** The underlying Nostr event */
  event: NostrEvent;
  
  /** Unique identifier for this key */
  keyID: string;
  
  /** The public key being advertised (hex) */
  publicKey: string;
  
  /** Purpose of this key */
  purpose: KeyPurpose;
  
  /** When this key expires */
  expiry?: Date;
  
  /** Cryptographic algorithm (e.g., 'secp256k1') */
  algorithm: string;
  
  /** BIP32 derivation path */
  derivationPath: string;
  
  /** Index in the derivation path */
  keyIndex: number;
  
  /** Identity tag if signed by a delegate */
  identityTag?: IdentityTag;
}

/**
 * Replication Request (Kind 39104)
 * 
 * Requests replication of directory events.
 */
export interface ReplicationRequest {
  /** The underlying Nostr event */
  event: NostrEvent;
  
  /** Unique identifier for this request */
  requestID: string;
  
  /** WebSocket URL of the requesting relay */
  requestorRelay: string;
  
  /** WebSocket URL of the target relay */
  targetRelay: string;
  
  /** Event kinds to replicate */
  kinds: number[];
  
  /** Author pubkeys to filter by */
  authors?: string[];
  
  /** Timestamp to replicate from */
  since?: Date;
  
  /** Timestamp to replicate until */
  until?: Date;
  
  /** Maximum number of events to return */
  limit?: number;
  
  /** Identity tag if signed by a delegate */
  identityTag?: IdentityTag;
}

/**
 * Replication Response (Kind 39105)
 * 
 * Response to a replication request.
 */
export interface ReplicationResponse {
  /** The underlying Nostr event */
  event: NostrEvent;
  
  /** Request ID this response corresponds to */
  requestID: string;
  
  /** Status of the replication */
  status: ReplicationStatus;
  
  /** IDs of events being replicated */
  eventIDs: string[];
  
  /** Error message if status is Failed */
  error?: string;
  
  /** Identity tag if signed by a delegate */
  identityTag?: IdentityTag;
}

/**
 * Parsed content structure for directory events
 */
export interface DirectoryEventContent {
  /** Original JSON string */
  raw: string;
  
  /** Parsed JSON object */
  data: Record<string, any>;
}

/**
 * Helper type for event validation errors
 */
export interface ValidationError {
  field: string;
  message: string;
  value?: any;
}

/**
 * Check if a Nostr event kind is a directory event kind
 */
export function isDirectoryEventKind(kind: number): boolean {
  return Object.values(EventKinds).includes(kind as DirectoryEventKind);
}

/**
 * Check if a trust level is valid
 */
export function isValidTrustLevel(level: string): level is TrustLevel {
  return Object.values(TrustLevel).includes(level as TrustLevel);
}

/**
 * Check if a key purpose is valid
 */
export function isValidKeyPurpose(purpose: string): purpose is KeyPurpose {
  return Object.values(KeyPurpose).includes(purpose as KeyPurpose);
}

/**
 * Check if a replication status is valid
 */
export function isValidReplicationStatus(status: string): status is ReplicationStatus {
  return Object.values(ReplicationStatus).includes(status as ReplicationStatus);
}

