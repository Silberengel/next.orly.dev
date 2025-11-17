/**
 * Helper utilities for the Directory Consensus Protocol
 */

import type { NostrEvent } from 'applesauce-core/helpers';
import type { EventStore } from 'applesauce-core';
import type {
  RelayIdentity,
  TrustAct,
  GroupTagAct,
  TrustLevel,
} from './types.js';
import { EventKinds } from './types.js';
import {
  parseRelayIdentity,
  parseTrustAct,
  parseGroupTagAct,
} from './parsers.js';
import { Observable, combineLatest, map } from 'rxjs';

/**
 * Trust calculator for computing aggregate trust scores
 */
export class TrustCalculator {
  private acts: Map<string, TrustAct[]> = new Map();

  /**
   * Add a trust act to the calculator
   */
  public addAct(act: TrustAct): void {
    const key = act.targetPubkey;
    if (!this.acts.has(key)) {
      this.acts.set(key, []);
    }
    this.acts.get(key)!.push(act);
  }

  /**
   * Calculate aggregate trust score for a pubkey
   * 
   * @param pubkey - The public key to calculate trust for
   * @returns Numeric trust score (0-100)
   */
  public calculateTrust(pubkey: string): number {
    const acts = this.acts.get(pubkey) || [];
    if (acts.length === 0) return 0;

    // Simple weighted average: high=100, medium=50, low=25
    const weights: Record<TrustLevel, number> = {
      [TrustLevel.High]: 100,
      [TrustLevel.Medium]: 50,
      [TrustLevel.Low]: 25,
    };

    let total = 0;
    let count = 0;

    for (const act of acts) {
      // Skip expired acts
      if (act.expiry && act.expiry < new Date()) {
        continue;
      }

      total += weights[act.trustLevel];
      count++;
    }

    return count > 0 ? total / count : 0;
  }

  /**
   * Get all acts for a pubkey
   */
  public getActs(pubkey: string): TrustAct[] {
    return this.acts.get(pubkey) || [];
  }

  /**
   * Clear all acts
   */
  public clear(): void {
    this.acts.clear();
  }
}

/**
 * Replication filter for managing which events to replicate
 */
export class ReplicationFilter {
  private trustedRelays: Set<string> = new Set();
  private trustCalculator: TrustCalculator;
  private minTrustScore: number;

  constructor(minTrustScore = 50) {
    this.trustCalculator = new TrustCalculator();
    this.minTrustScore = minTrustScore;
  }

  /**
   * Add a trust act to influence replication decisions
   */
  public addTrustAct(act: TrustAct): void {
    this.trustCalculator.addAct(act);
    
    // Update trusted relays based on trust score
    const score = this.trustCalculator.calculateTrust(act.targetPubkey);
    if (score >= this.minTrustScore) {
      this.trustedRelays.add(act.targetPubkey);
    } else {
      this.trustedRelays.delete(act.targetPubkey);
    }
  }

  /**
   * Check if a relay is trusted enough for replication
   */
  public shouldReplicate(pubkey: string): boolean {
    return this.trustedRelays.has(pubkey);
  }

  /**
   * Get all trusted relay pubkeys
   */
  public getTrustedRelays(): string[] {
    return Array.from(this.trustedRelays);
  }

  /**
   * Get trust score for a relay
   */
  public getTrustScore(pubkey: string): number {
    return this.trustCalculator.calculateTrust(pubkey);
  }
}

/**
 * Helper to find all relay identities in an event store
 */
export function findRelayIdentities(eventStore: EventStore): Observable<RelayIdentity[]> {
  return eventStore.stream({ kinds: [EventKinds.RelayIdentityAnnouncement] }).pipe(
    map(events => {
      const identities: RelayIdentity[] = [];
      for (const event of events as any) {
        try {
          identities.push(parseRelayIdentity(event));
        } catch (err) {
          // Skip invalid events
          console.warn('Invalid relay identity:', err);
        }
      }
      return identities;
    })
  );
}

/**
 * Helper to find all trust acts for a specific relay
 */
export function findTrustActsForRelay(
  eventStore: EventStore,
  targetPubkey: string
): Observable<TrustAct[]> {
  return eventStore.stream({ kinds: [EventKinds.TrustAct] }).pipe(
    map(events => {
      const acts: TrustAct[] = [];
      for (const event of events as any) {
        try {
          const act = parseTrustAct(event);
          if (act.targetPubkey === targetPubkey) {
            acts.push(act);
          }
        } catch (err) {
          // Skip invalid events
          console.warn('Invalid trust act:', err);
        }
      }
      return acts;
    })
  );
}

/**
 * Helper to find all group tag acts for a specific relay
 */
export function findGroupTagActsForRelay(
  eventStore: EventStore,
  targetPubkey: string
): Observable<GroupTagAct[]> {
  return eventStore.stream({ kinds: [EventKinds.GroupTagAct] }).pipe(
    map(events => {
      const acts: GroupTagAct[] = [];
      for (const event of events as any) {
        try {
          const act = parseGroupTagAct(event);
          if (act.targetPubkey === targetPubkey) {
            acts.push(act);
          }
        } catch (err) {
          // Skip invalid events
          console.warn('Invalid group tag act:', err);
        }
      }
      return acts;
    })
  );
}

/**
 * Helper to build a trust graph from an event store
 */
export function buildTrustGraph(eventStore: EventStore): Observable<Map<string, TrustAct[]>> {
  return eventStore.stream({ kinds: [EventKinds.TrustAct] }).pipe(
    map(events => {
      const graph = new Map<string, TrustAct[]>();
      for (const event of events as any) {
        try {
          const act = parseTrustAct(event);
          const source = event.pubkey;
          if (!graph.has(source)) {
            graph.set(source, []);
          }
          graph.get(source)!.push(act);
        } catch (err) {
          // Skip invalid events
          console.warn('Invalid trust act:', err);
        }
      }
      return graph;
    })
  );
}

/**
 * Helper to check if an event is a directory event
 */
export function isDirectoryEvent(event: NostrEvent): boolean {
  return Object.values(EventKinds).includes(event.kind as any);
}

/**
 * Helper to filter directory events from a stream
 */
export function filterDirectoryEvents(eventStore: EventStore): Observable<NostrEvent> {
  return eventStore.stream({ kinds: Object.values(EventKinds) });
}

/**
 * Format a relay URL to canonical format (with trailing slash)
 */
export function normalizeRelayURL(url: string): string {
  const trimmed = url.trim();
  return trimmed.endsWith('/') ? trimmed : `${trimmed}/`;
}

/**
 * Extract relay URL from a NIP-11 URL
 */
export function extractRelayURL(nip11URL: string): string {
  try {
    const url = new URL(nip11URL);
    // Convert http(s) to ws(s)
    const protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    return normalizeRelayURL(`${protocol}//${url.host}${url.pathname}`);
  } catch (err) {
    throw new Error(`Invalid NIP-11 URL: ${nip11URL}`);
  }
}

/**
 * Create a NIP-11 URL from a relay WebSocket URL
 */
export function createNIP11URL(relayURL: string): string {
  try {
    const url = new URL(relayURL);
    // Convert ws(s) to http(s)
    const protocol = url.protocol === 'wss:' ? 'https:' : 'http:';
    return `${protocol}//${url.host}${url.pathname}`;
  } catch (err) {
    throw new Error(`Invalid relay URL: ${relayURL}`);
  }
}

