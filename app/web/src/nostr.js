import { SimplePool } from 'nostr-tools/pool';
import { EventStore } from 'applesauce-core';
import { PrivateKeySigner } from 'applesauce-signers';
import { DEFAULT_RELAYS } from "./constants.js";

// Nostr client wrapper using nostr-tools
class NostrClient {
  constructor() {
    this.pool = new SimplePool();
    this.eventStore = new EventStore();
    this.isConnected = false;
    this.signer = null;
    this.relays = [...DEFAULT_RELAYS];
  }

  async connect() {
    console.log("Starting connection to", this.relays.length, "relays...");
    
    try {
      // SimplePool doesn't require explicit connect
      this.isConnected = true;
      console.log("✓ Successfully initialized relay pool");
      
      // Wait a bit for connections to stabilize
      await new Promise((resolve) => setTimeout(resolve, 1000));
    } catch (error) {
      console.error("✗ Connection failed:", error);
      throw error;
    }
  }

  async connectToRelay(relayUrl) {
    console.log(`Adding relay: ${relayUrl}`);
    
    try {
      if (!this.relays.includes(relayUrl)) {
        this.relays.push(relayUrl);
      }
      console.log(`✓ Successfully added relay ${relayUrl}`);
      return true;
    } catch (error) {
      console.error(`✗ Failed to add relay ${relayUrl}:`, error);
      return false;
    }
  }

  subscribe(filters, callback) {
    console.log("Creating subscription with filters:", filters);
    
    const sub = this.pool.subscribeMany(
      this.relays,
      filters,
      {
        onevent(event) {
          console.log("Event received:", event);
          callback(event);
        },
        oneose() {
          console.log("EOSE received");
          window.dispatchEvent(new CustomEvent('nostr-eose', {
            detail: { subscriptionId: sub.id }
          }));
        }
      }
    );

    return sub;
  }

  unsubscribe(subscription) {
    console.log(`Closing subscription`);
    if (subscription && subscription.close) {
      subscription.close();
    }
  }

  disconnect() {
    console.log("Disconnecting relay pool");
    if (this.pool) {
      this.pool.close(this.relays);
    }
    this.isConnected = false;
  }

  // Publish an event
  async publish(event) {
    console.log("Publishing event:", event);
    
    try {
      const promises = this.pool.publish(this.relays, event);
      await Promise.allSettled(promises);
      console.log("✓ Event published successfully");
      return { success: true, okCount: 1, errorCount: 0 };
    } catch (error) {
      console.error("✗ Failed to publish event:", error);
      throw error;
    }
  }

  // Get pool for advanced usage
  getPool() {
    return this.pool;
  }

  // Get event store
  getEventStore() {
    return this.eventStore;
  }

  // Get signer
  getSigner() {
    return this.signer;
  }

  // Set signer
  setSigner(signer) {
    this.signer = signer;
  }
}

// Create a global client instance
export const nostrClient = new NostrClient();

// Export the class for creating new instances
export { NostrClient };

// Export signer classes
export { PrivateKeySigner };

// Export NIP-07 helper
export class Nip07Signer {
  async getPublicKey() {
    if (window.nostr) {
      return await window.nostr.getPublicKey();
    }
    throw new Error('NIP-07 extension not found');
  }

  async signEvent(event) {
    if (window.nostr) {
      return await window.nostr.signEvent(event);
    }
    throw new Error('NIP-07 extension not found');
  }

  async nip04Encrypt(pubkey, plaintext) {
    if (window.nostr && window.nostr.nip04) {
      return await window.nostr.nip04.encrypt(pubkey, plaintext);
    }
    throw new Error('NIP-07 extension does not support NIP-04');
  }

  async nip04Decrypt(pubkey, ciphertext) {
    if (window.nostr && window.nostr.nip04) {
      return await window.nostr.nip04.decrypt(pubkey, ciphertext);
    }
    throw new Error('NIP-07 extension does not support NIP-04');
  }

  async nip44Encrypt(pubkey, plaintext) {
    if (window.nostr && window.nostr.nip44) {
      return await window.nostr.nip44.encrypt(pubkey, plaintext);
    }
    throw new Error('NIP-07 extension does not support NIP-44');
  }

  async nip44Decrypt(pubkey, ciphertext) {
    if (window.nostr && window.nostr.nip44) {
      return await window.nostr.nip44.decrypt(pubkey, ciphertext);
    }
    throw new Error('NIP-07 extension does not support NIP-44');
  }
}

// IndexedDB helpers for caching events (kind 0 profiles)
const DB_NAME = "nostrCache";
const DB_VERSION = 1;
const STORE_EVENTS = "events";

function openDB() {
  return new Promise((resolve, reject) => {
    try {
      const req = indexedDB.open(DB_NAME, DB_VERSION);
      req.onupgradeneeded = () => {
        const db = req.result;
        if (!db.objectStoreNames.contains(STORE_EVENTS)) {
          const store = db.createObjectStore(STORE_EVENTS, { keyPath: "id" });
          store.createIndex("byKindAuthor", ["kind", "pubkey"], {
            unique: false,
          });
          store.createIndex(
            "byKindAuthorCreated",
            ["kind", "pubkey", "created_at"],
            { unique: false },
          );
        }
      };
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    } catch (e) {
      reject(e);
    }
  });
}

async function getLatestProfileEvent(pubkey) {
  try {
    const db = await openDB();
    return await new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_EVENTS, "readonly");
      const idx = tx.objectStore(STORE_EVENTS).index("byKindAuthorCreated");
      const range = IDBKeyRange.bound(
        [0, pubkey, -Infinity],
        [0, pubkey, Infinity],
      );
      const req = idx.openCursor(range, "prev"); // newest first
      req.onsuccess = () => {
        const cursor = req.result;
        resolve(cursor ? cursor.value : null);
      };
      req.onerror = () => reject(req.error);
    });
  } catch (e) {
    console.warn("IDB getLatestProfileEvent failed", e);
    return null;
  }
}

async function putEvent(event) {
  try {
    const db = await openDB();
    await new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_EVENTS, "readwrite");
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
      tx.objectStore(STORE_EVENTS).put(event);
    });
  } catch (e) {
    console.warn("IDB putEvent failed", e);
  }
}

function parseProfileFromEvent(event) {
  try {
    const profile = JSON.parse(event.content || "{}");
    return {
      name: profile.name || profile.display_name || "",
      picture: profile.picture || "",
      banner: profile.banner || "",
      about: profile.about || "",
      nip05: profile.nip05 || "",
      lud16: profile.lud16 || profile.lud06 || "",
    };
  } catch (e) {
    return {
      name: "",
      picture: "",
      banner: "",
      about: "",
      nip05: "",
      lud16: "",
    };
  }
}

// Fetch user profile metadata (kind 0)
export async function fetchUserProfile(pubkey) {
  console.log(`Starting profile fetch for pubkey: ${pubkey}`);

  // 1) Try cached profile first and resolve immediately if present
  try {
    const cachedEvent = await getLatestProfileEvent(pubkey);
    if (cachedEvent) {
      console.log("Using cached profile event");
      const profile = parseProfileFromEvent(cachedEvent);
      return profile;
    }
  } catch (e) {
    console.warn("Failed to load cached profile", e);
  }

  // 2) Fetch profile from relays
  try {
    const filters = [{
      kinds: [0],
      authors: [pubkey],
      limit: 1
    }];
    
    const events = await fetchEvents(filters, { timeout: 10000 });
    
    if (events.length > 0) {
      const profileEvent = events[0];
      console.log("Profile fetched:", profileEvent);
      
      // Cache the event
      await putEvent(profileEvent);
      
      // Parse profile data
      const profile = parseProfileFromEvent(profileEvent);
      
      // Notify listeners that an updated profile is available
      try {
        if (typeof window !== "undefined" && window.dispatchEvent) {
          window.dispatchEvent(
            new CustomEvent("profile-updated", {
              detail: { pubkey, profile, event: profileEvent },
            }),
          );
        }
      } catch (e) {
        console.warn("Failed to dispatch profile-updated event", e);
      }
      
      return profile;
    } else {
      throw new Error("No profile found");
    }
  } catch (error) {
    console.error("Failed to fetch profile:", error);
    throw error;
  }
}

// Fetch events
export async function fetchEvents(filters, options = {}) {
  console.log(`Starting event fetch with filters:`, filters);

  const {
    timeout = 30000,
  } = options;

  return new Promise((resolve, reject) => {
    const events = [];
    const timeoutId = setTimeout(() => {
      console.log(`Timeout reached after ${timeout}ms, returning ${events.length} events`);
      sub.close();
      resolve(events);
    }, timeout);

    try {
      const sub = nostrClient.pool.subscribeMany(
        nostrClient.relays,
        filters,
        {
          onevent(event) {
            console.log("Event received:", event);
            events.push(event);
          },
          oneose() {
            console.log(`EOSE received, got ${events.length} events`);
            clearTimeout(timeoutId);
            sub.close();
            resolve(events);
          }
        }
      );
    } catch (error) {
      clearTimeout(timeoutId);
      console.error("Failed to fetch events:", error);
      reject(error);
    }
  });
}

// Fetch all events with timestamp-based pagination (including delete events)
export async function fetchAllEvents(options = {}) {
  const {
    limit = 100,
    since = null,
    until = null,
    authors = null,
    kinds = null,
    ...rest
  } = options;

  const filters = [{ ...rest }];
  
  if (since) filters[0].since = since;
  if (until) filters[0].until = until;
  if (authors) filters[0].authors = authors;
  if (kinds) filters[0].kinds = kinds;
  if (limit) filters[0].limit = limit;
  
  const events = await fetchEvents(filters, { 
    timeout: 30000 
  });
  
  return events;
}

// Fetch user's events with timestamp-based pagination
export async function fetchUserEvents(pubkey, options = {}) {
  const {
    limit = 100,
    since = null,
    until = null
  } = options;

  const filters = [{
    authors: [pubkey]
  }];
  
  if (since) filters[0].since = since;
  if (until) filters[0].until = until;
  if (limit) filters[0].limit = limit;
  
  const events = await fetchEvents(filters, { 
    timeout: 30000 
  });
  
  return events;
}

// NIP-50 search function
export async function searchEvents(searchQuery, options = {}) {
  const {
    limit = 100,
    since = null,
    until = null,
    kinds = null
  } = options;

  const filters = [{
    search: searchQuery
  }];
  
  if (since) filters[0].since = since;
  if (until) filters[0].until = until;
  if (kinds) filters[0].kinds = kinds;
  if (limit) filters[0].limit = limit;
  
  const events = await fetchEvents(filters, { 
    timeout: 30000 
  });
  
  return events;
}

// Fetch a specific event by ID
export async function fetchEventById(eventId, options = {}) {
  const {
    timeout = 10000,
  } = options;

  console.log(`Fetching event by ID: ${eventId}`);

  try {
    const filters = [{
      ids: [eventId]
    }];

    console.log('Fetching event with filters:', filters);

    const events = await fetchEvents(filters, { timeout });

    console.log(`Fetched ${events.length} events`);
    
    // Return the first event if found, null otherwise
    return events.length > 0 ? events[0] : null;
  } catch (error) {
    console.error("Failed to fetch event by ID:", error);
    throw error;
  }
}

// Fetch delete events that target a specific event ID
export async function fetchDeleteEventsByTarget(eventId, options = {}) {
  const {
    timeout = 10000
  } = options;

  console.log(`Fetching delete events for target: ${eventId}`);

  try {
    const filters = [{
      kinds: [5], // Kind 5 is deletion
      '#e': [eventId] // e-tag referencing the target event
    }];

    console.log('Fetching delete events with filters:', filters);

    const events = await fetchEvents(filters, { timeout });

    console.log(`Fetched ${events.length} delete events`);
    
    return events;
  } catch (error) {
    console.error("Failed to fetch delete events:", error);
    throw error;
  }
}

// Initialize client connection
export async function initializeNostrClient() {
  await nostrClient.connect();
}
