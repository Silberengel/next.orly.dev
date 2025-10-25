/**
 * Identity Resolution for Directory Consensus Protocol
 * 
 * This module provides functionality to resolve actual identities behind
 * delegate keys and manage key delegations.
 */

import type { EventStore } from 'applesauce-core';
import type { NostrEvent } from 'applesauce-core/helpers';
import type { IdentityTag, PublicKeyAdvertisement } from './types.js';
import { EventKinds } from './types.js';
import { parseIdentityTag, parsePublicKeyAdvertisement } from './parsers.js';
import { ValidationError } from './validation.js';
import { Observable, combineLatest, map, startWith } from 'rxjs';

/**
 * Manages identity resolution and key delegation tracking
 */
export class IdentityResolver {
  private eventStore: EventStore;
  private delegateToIdentity: Map<string, string> = new Map();
  private identityToDelegates: Map<string, Set<string>> = new Map();
  private identityTagCache: Map<string, IdentityTag> = new Map();
  private publicKeyAds: Map<string, PublicKeyAdvertisement> = new Map();

  constructor(eventStore: EventStore) {
    this.eventStore = eventStore;
    this.initializeTracking();
  }

  /**
   * Initialize tracking of identity tags and key delegations
   */
  private initializeTracking(): void {
    // Track all events with I tags
    this.eventStore.stream({ kinds: Object.values(EventKinds) }).subscribe(event => {
      this.processEvent(event);
    });

    // Track Public Key Advertisements (kind 39103)
    this.eventStore.stream({ kinds: [EventKinds.PublicKeyAdvertisement] }).subscribe(event => {
      try {
        const keyAd = parsePublicKeyAdvertisement(event);
        this.publicKeyAds.set(keyAd.keyID, keyAd);
      } catch (err) {
        // Ignore invalid events
        console.warn('Invalid public key advertisement:', err);
      }
    });
  }

  /**
   * Process an event to extract and cache identity information
   */
  private processEvent(event: NostrEvent): void {
    try {
      const identityTag = parseIdentityTag(event);
      if (identityTag) {
        this.cacheIdentityTag(identityTag);
      }
    } catch (err) {
      // Event doesn't have a valid I tag or parsing failed
    }
  }

  /**
   * Cache an identity tag mapping
   */
  private cacheIdentityTag(tag: IdentityTag): void {
    const { identity, delegate } = tag;
    
    // Store delegate -> identity mapping
    this.delegateToIdentity.set(delegate, identity);
    
    // Store identity -> delegates mapping
    if (!this.identityToDelegates.has(identity)) {
      this.identityToDelegates.set(identity, new Set());
    }
    this.identityToDelegates.get(identity)!.add(delegate);
    
    // Cache the full tag
    this.identityTagCache.set(delegate, tag);
  }

  /**
   * Resolve the actual identity behind a public key (which may be a delegate)
   * 
   * @param pubkey - The public key to resolve (may be delegate or identity)
   * @returns The actual identity public key, or the input if it's already an identity
   */
  public resolveIdentity(pubkey: string): string {
    return this.delegateToIdentity.get(pubkey) || pubkey;
  }

  /**
   * Resolve the actual identity behind an event's pubkey
   * 
   * @param event - The event to resolve
   * @returns The actual identity public key
   */
  public resolveEventIdentity(event: NostrEvent): string {
    return this.resolveIdentity(event.pubkey);
  }

  /**
   * Check if a public key is a known delegate
   * 
   * @param pubkey - The public key to check
   * @returns true if the key is a delegate, false otherwise
   */
  public isDelegateKey(pubkey: string): boolean {
    return this.delegateToIdentity.has(pubkey);
  }

  /**
   * Check if a public key is a known identity (has delegates)
   * 
   * @param pubkey - The public key to check
   * @returns true if the key is an identity with delegates, false otherwise
   */
  public isIdentityKey(pubkey: string): boolean {
    return this.identityToDelegates.has(pubkey);
  }

  /**
   * Get all delegate keys for a given identity
   * 
   * @param identity - The identity public key
   * @returns Set of delegate public keys
   */
  public getDelegatesForIdentity(identity: string): Set<string> {
    return this.identityToDelegates.get(identity) || new Set();
  }

  /**
   * Get the identity tag for a delegate key
   * 
   * @param delegate - The delegate public key
   * @returns The identity tag, or undefined if not found
   */
  public getIdentityTag(delegate: string): IdentityTag | undefined {
    return this.identityTagCache.get(delegate);
  }

  /**
   * Get all public key advertisements for an identity
   * 
   * @param identity - The identity public key
   * @returns Array of public key advertisements
   */
  public getPublicKeyAdvertisements(identity: string): PublicKeyAdvertisement[] {
    const delegates = this.getDelegatesForIdentity(identity);
    const ads: PublicKeyAdvertisement[] = [];
    
    for (const keyAd of this.publicKeyAds.values()) {
      const adIdentity = this.resolveIdentity(keyAd.event.pubkey);
      if (adIdentity === identity || delegates.has(keyAd.publicKey)) {
        ads.push(keyAd);
      }
    }
    
    return ads;
  }

  /**
   * Get a public key advertisement by key ID
   * 
   * @param keyID - The unique key identifier
   * @returns The public key advertisement, or undefined if not found
   */
  public getPublicKeyAdvertisementByID(keyID: string): PublicKeyAdvertisement | undefined {
    return this.publicKeyAds.get(keyID);
  }

  /**
   * Stream all events by their actual identity
   * 
   * @param identity - The identity public key
   * @param includeNewEvents - If true, include future events (default: false)
   * @returns Observable of events signed by this identity or its delegates
   */
  public streamEventsByIdentity(identity: string, includeNewEvents = false): Observable<NostrEvent> {
    const delegates = this.getDelegatesForIdentity(identity);
    const allKeys = [identity, ...Array.from(delegates)];
    
    return this.eventStore.stream(
      { authors: allKeys },
      includeNewEvents
    );
  }

  /**
   * Stream events by identity with real-time delegate updates
   * 
   * This will automatically include events from newly discovered delegates.
   * 
   * @param identity - The identity public key
   * @returns Observable of events signed by this identity or its delegates
   */
  public streamEventsByIdentityLive(identity: string): Observable<NostrEvent> {
    // Create an observable that emits whenever delegates change
    const delegateUpdates$ = new Observable<Set<string>>(observer => {
      // Emit initial delegates
      observer.next(this.getDelegatesForIdentity(identity));
      
      // Watch for new delegates
      const subscription = this.eventStore.stream({ kinds: Object.values(EventKinds) }, true)
        .subscribe(event => {
          try {
            const identityTag = parseIdentityTag(event);
            if (identityTag && identityTag.identity === identity) {
              this.cacheIdentityTag(identityTag);
              observer.next(this.getDelegatesForIdentity(identity));
            }
          } catch (err) {
            // Ignore invalid events
          }
        });
      
      return () => subscription.unsubscribe();
    });
    
    // Map delegate updates to event streams
    return delegateUpdates$.pipe(
      map(delegates => {
        const allKeys = [identity, ...Array.from(delegates)];
        return this.eventStore.stream({ authors: allKeys }, true);
      }),
      // Flatten the nested observable
      map(stream$ => stream$),
    ) as any; // Type assertion needed due to complex Observable nesting
  }

  /**
   * Verify that an identity tag signature is valid
   * 
   * Note: This requires schnorr signature verification which should be
   * implemented using appropriate cryptographic libraries.
   * 
   * @param tag - The identity tag to verify
   * @returns Promise that resolves to true if valid, false otherwise
   */
  public async verifyIdentityTag(tag: IdentityTag): Promise<boolean> {
    // TODO: Implement schnorr signature verification
    // The signature is over: sha256(identity + delegate + relayHint)
    // 
    // Example implementation would require:
    // 1. Concatenate: identity + delegate + (relayHint || '')
    // 2. Compute SHA256 hash
    // 3. Verify signature using identity key
    
    throw new Error('Identity tag verification not yet implemented');
  }

  /**
   * Clear all cached identity mappings
   */
  public clearCache(): void {
    this.delegateToIdentity.clear();
    this.identityToDelegates.clear();
    this.identityTagCache.clear();
    this.publicKeyAds.clear();
  }

  /**
   * Get statistics about tracked identities and delegates
   */
  public getStats(): {
    identities: number;
    delegates: number;
    publicKeyAds: number;
  } {
    return {
      identities: this.identityToDelegates.size,
      delegates: this.delegateToIdentity.size,
      publicKeyAds: this.publicKeyAds.size,
    };
  }
}

/**
 * Helper function to create an identity resolver instance
 */
export function createIdentityResolver(eventStore: EventStore): IdentityResolver {
  return new IdentityResolver(eventStore);
}

