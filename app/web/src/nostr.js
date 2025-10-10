import NDK, { NDKPrivateKeySigner } from '@nostr-dev-kit/ndk';
import { DEFAULT_RELAYS } from "./constants.js";

// NDK-based Nostr client wrapper
class NostrClient {
  constructor() {
    this.ndk = new NDK({
      explicitRelayUrls: DEFAULT_RELAYS
    });
    this.isConnected = false;
  }

  async connect() {
    console.log("Starting NDK connection to", DEFAULT_RELAYS.length, "relays...");
    
    try {
      await this.ndk.connect();
      this.isConnected = true;
      console.log("✓ NDK successfully connected to relays");
      
      // Wait a bit for connections to stabilize
      await new Promise((resolve) => setTimeout(resolve, 1000));
    } catch (error) {
      console.error("✗ NDK connection failed:", error);
      throw error;
    }
  }

  async connectToRelay(relayUrl) {
    console.log(`Adding relay to NDK: ${relayUrl}`);
    
    try {
      await this.ndk.addRelay(relayUrl);
      console.log(`✓ Successfully added relay ${relayUrl}`);
      return true;
    } catch (error) {
      console.error(`✗ Failed to add relay ${relayUrl}:`, error);
      return false;
    }
  }

  subscribe(filters, callback) {
    console.log("Creating NDK subscription with filters:", filters);
    
    const subscription = this.ndk.subscribe(filters, {
      closeOnEose: true
    });

    subscription.on('event', (event) => {
      console.log("Event received via NDK:", event);
      callback(event.rawEvent());
    });

    subscription.on('eose', () => {
      console.log("EOSE received via NDK");
      window.dispatchEvent(new CustomEvent('nostr-eose', {
        detail: { subscriptionId: subscription.id }
      }));
    });

    return subscription.id;
  }

  unsubscribe(subscriptionId) {
    console.log(`Closing NDK subscription: ${subscriptionId}`);
    // NDK handles subscription cleanup automatically
  }

  disconnect() {
    console.log("Disconnecting NDK");
    this.ndk.destroy();
    this.isConnected = false;
  }

  // Publish an event using NDK
  async publish(event) {
    console.log("Publishing event via NDK:", event);
    
    try {
      const ndkEvent = this.ndk.createEvent(event);
      await ndkEvent.publish();
      console.log("✓ Event published successfully via NDK");
      return { success: true, okCount: 1, errorCount: 0 };
    } catch (error) {
      console.error("✗ Failed to publish event via NDK:", error);
      throw error;
    }
  }

  // Get NDK instance for advanced usage
  getNDK() {
    return this.ndk;
  }

  // Get signer from NDK
  getSigner() {
    return this.ndk.signer;
  }

  // Set signer for NDK
  setSigner(signer) {
    this.ndk.signer = signer;
  }
}

// Create a global client instance
export const nostrClient = new NostrClient();

// Export the class for creating new instances
export { NostrClient };

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

// Fetch user profile metadata (kind 0) using NDK
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

  // 2) Fetch profile using NDK
  try {
    const ndk = nostrClient.getNDK();
    const user = ndk.getUser({ hexpubkey: pubkey });
    
    // Fetch the latest profile event
    const profileEvent = await user.fetchProfile();
    
    if (profileEvent) {
      console.log("Profile fetched via NDK:", profileEvent);
      
      // Cache the event
      await putEvent(profileEvent.rawEvent());
      
      // Parse profile data
      const profile = parseProfileFromEvent(profileEvent.rawEvent());
      
      // Notify listeners that an updated profile is available
      try {
        if (typeof window !== "undefined" && window.dispatchEvent) {
          window.dispatchEvent(
            new CustomEvent("profile-updated", {
              detail: { pubkey, profile, event: profileEvent.rawEvent() },
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
    console.error("Failed to fetch profile via NDK:", error);
    throw error;
  }
}

// Fetch events using NDK
export async function fetchEvents(filters, options = {}) {
  console.log(`Starting event fetch with filters:`, filters);

  const {
    timeout = 30000,
    limit = null
  } = options;

  try {
    const ndk = nostrClient.getNDK();
    
    // Add limit to filters if specified
    const requestFilters = { ...filters };
    if (limit) {
      requestFilters.limit = limit;
    }

    console.log('Fetching events via NDK with filters:', requestFilters);

    // Use NDK's fetchEvents method
    const events = await ndk.fetchEvents(requestFilters, {
      timeout
    });

    console.log(`Fetched ${events.size} events via NDK`);
    
    // Convert NDK events to raw events
    const rawEvents = Array.from(events).map(event => event.rawEvent());
    
    return rawEvents;
  } catch (error) {
    console.error("Failed to fetch events via NDK:", error);
    throw error;
  }
}

// Fetch all events with timestamp-based pagination using NDK
export async function fetchAllEvents(options = {}) {
  const {
    limit = 100,
    since = null,
    until = null,
    authors = null
  } = options;

  const filters = {};
  
  if (since) filters.since = since;
  if (until) filters.until = until;
  if (authors) filters.authors = authors;
  
  const events = await fetchEvents(filters, { 
    limit: limit,
    timeout: 30000 
  });
  
  return events;
}

// Fetch user's events with timestamp-based pagination using NDK
export async function fetchUserEvents(pubkey, options = {}) {
  const {
    limit = 100,
    since = null,
    until = null
  } = options;

  const filters = {
    authors: [pubkey]
  };
  
  if (since) filters.since = since;
  if (until) filters.until = until;
  
  const events = await fetchEvents(filters, { 
    limit: limit,
    timeout: 30000 
  });
  
  return events;
}

// NIP-50 search function using NDK
export async function searchEvents(searchQuery, options = {}) {
  const {
    limit = 100,
    since = null,
    until = null,
    kinds = null
  } = options;

  const filters = {
    search: searchQuery
  };
  
  if (since) filters.since = since;
  if (until) filters.until = until;
  if (kinds) filters.kinds = kinds;
  
  const events = await fetchEvents(filters, { 
    limit: limit,
    timeout: 30000 
  });
  
  return events;
}


// Initialize client connection
export async function initializeNostrClient() {
  await nostrClient.connect();
}
