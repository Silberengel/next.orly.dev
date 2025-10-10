import { DEFAULT_RELAYS } from "./constants.js";

// Simple WebSocket relay manager
class NostrClient {
  constructor() {
    this.relays = new Map();
    this.subscriptions = new Map();
  }

  async connect() {
    console.log("Starting connection to", DEFAULT_RELAYS.length, "relays...");

    const connectionPromises = DEFAULT_RELAYS.map((relayUrl) => {
      return new Promise((resolve) => {
        try {
          console.log(`Attempting to connect to ${relayUrl}`);
          const ws = new WebSocket(relayUrl);

          ws.onopen = () => {
            console.log(`✓ Successfully connected to ${relayUrl}`);
            resolve(true);
          };

          ws.onerror = (error) => {
            console.error(`✗ Error connecting to ${relayUrl}:`, error);
            resolve(false);
          };

          ws.onclose = (event) => {
            console.warn(
              `Connection closed to ${relayUrl}:`,
              event.code,
              event.reason,
            );
          };

          ws.onmessage = (event) => {
            console.log(`Message from ${relayUrl}:`, event.data);
            try {
              this.handleMessage(relayUrl, JSON.parse(event.data));
            } catch (error) {
              console.error(
                `Failed to parse message from ${relayUrl}:`,
                error,
                event.data,
              );
            }
          };

          this.relays.set(relayUrl, ws);

          // Timeout after 5 seconds
          setTimeout(() => {
            if (ws.readyState !== WebSocket.OPEN) {
              console.warn(`Connection timeout for ${relayUrl}`);
              resolve(false);
            }
          }, 5000);
        } catch (error) {
          console.error(`Failed to create WebSocket for ${relayUrl}:`, error);
          resolve(false);
        }
      });
    });

    const results = await Promise.all(connectionPromises);
    const successfulConnections = results.filter(Boolean).length;
    console.log(
      `Connected to ${successfulConnections}/${DEFAULT_RELAYS.length} relays`,
    );

    // Wait a bit more for connections to stabilize
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }

  async connectToRelay(relayUrl) {
    console.log(`Connecting to single relay: ${relayUrl}`);

    return new Promise((resolve) => {
      try {
        console.log(`Attempting to connect to ${relayUrl}`);
        const ws = new WebSocket(relayUrl);

        ws.onopen = () => {
          console.log(`✓ Successfully connected to ${relayUrl}`);
          resolve(true);
        };

        ws.onerror = (error) => {
          console.error(`✗ Error connecting to ${relayUrl}:`, error);
          resolve(false);
        };

        ws.onclose = (event) => {
          console.warn(
            `Connection closed to ${relayUrl}:`,
            event.code,
            event.reason,
          );
        };

        ws.onmessage = (event) => {
          console.log(`Message from ${relayUrl}:`, event.data);
          try {
            this.handleMessage(relayUrl, JSON.parse(event.data));
          } catch (error) {
            console.error(
              `Failed to parse message from ${relayUrl}:`,
              error,
              event.data,
            );
          }
        };

        this.relays.set(relayUrl, ws);

        // Timeout after 5 seconds
        setTimeout(() => {
          if (ws.readyState !== WebSocket.OPEN) {
            console.warn(`Connection timeout for ${relayUrl}`);
            resolve(false);
          }
        }, 5000);
      } catch (error) {
        console.error(`Failed to create WebSocket for ${relayUrl}:`, error);
        resolve(false);
      }
    });
  }

  handleMessage(relayUrl, message) {
    console.log(`Processing message from ${relayUrl}:`, message);
    const [type, subscriptionId, event, ...rest] = message;

    console.log(`Message type: ${type}, subscriptionId: ${subscriptionId}`);

    if (type === "EVENT") {
      console.log(`Received EVENT for subscription ${subscriptionId}:`, event);
      if (this.subscriptions.has(subscriptionId)) {
        console.log(
          `Found callback for subscription ${subscriptionId}, executing...`,
        );
        const callback = this.subscriptions.get(subscriptionId);
        callback(event);
      } else {
        console.warn(`No callback found for subscription ${subscriptionId}`);
      }
    } else if (type === "EOSE") {
      console.log(
        `End of stored events for subscription ${subscriptionId} from ${relayUrl}`,
      );
      // Dispatch EOSE event for fetchEvents function
      if (this.subscriptions.has(subscriptionId)) {
        window.dispatchEvent(new CustomEvent('nostr-eose', {
          detail: { subscriptionId, relayUrl }
        }));
      }
    } else if (type === "NOTICE") {
      console.warn(`Notice from ${relayUrl}:`, subscriptionId);
    } else {
      console.log(`Unknown message type ${type} from ${relayUrl}:`, message);
    }
  }

  subscribe(filters, callback) {
    const subscriptionId = Math.random().toString(36).substring(7);
    console.log(
      `Creating subscription ${subscriptionId} with filters:`,
      filters,
    );

    this.subscriptions.set(subscriptionId, callback);

    const subscription = ["REQ", subscriptionId, filters];
    console.log(`Subscription message:`, JSON.stringify(subscription));

    let sentCount = 0;
    for (const [relayUrl, ws] of this.relays) {
      console.log(
        `Checking relay ${relayUrl}, readyState: ${ws.readyState} (${ws.readyState === WebSocket.OPEN ? "OPEN" : "NOT OPEN"})`,
      );
      if (ws.readyState === WebSocket.OPEN) {
        try {
          ws.send(JSON.stringify(subscription));
          console.log(`✓ Sent subscription to ${relayUrl}`);
          sentCount++;
        } catch (error) {
          console.error(`✗ Failed to send subscription to ${relayUrl}:`, error);
        }
      } else {
        console.warn(`✗ Cannot send to ${relayUrl}, connection not ready`);
      }
    }

    console.log(
      `Subscription ${subscriptionId} sent to ${sentCount}/${this.relays.size} relays`,
    );
    return subscriptionId;
  }

  unsubscribe(subscriptionId) {
    this.subscriptions.delete(subscriptionId);

    const closeMessage = ["CLOSE", subscriptionId];

    for (const [relayUrl, ws] of this.relays) {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(closeMessage));
      }
    }
  }

  disconnect() {
    for (const [relayUrl, ws] of this.relays) {
      ws.close();
    }
    this.relays.clear();
    this.subscriptions.clear();
  }

  // Publish an event to all connected relays
  async publish(event) {
    return new Promise((resolve, reject) => {
      const eventMessage = ["EVENT", event];
      console.log("Publishing event:", eventMessage);
      
      let publishedCount = 0;
      let okCount = 0;
      let errorCount = 0;
      const totalRelays = this.relays.size;
      
      if (totalRelays === 0) {
        reject(new Error("No relays connected"));
        return;
      }

      const handleResponse = (relayUrl, success) => {
        if (success) {
          okCount++;
        } else {
          errorCount++;
        }
        
        if (okCount + errorCount === totalRelays) {
          if (okCount > 0) {
            resolve({ success: true, okCount, errorCount });
          } else {
            reject(new Error(`All relays rejected the event. Errors: ${errorCount}`));
          }
        }
      };

      // Set up a temporary listener for OK responses
      const originalHandleMessage = this.handleMessage.bind(this);
      this.handleMessage = (relayUrl, message) => {
        if (message[0] === "OK" && message[1] === event.id) {
          const success = message[2] === true;
          console.log(`Relay ${relayUrl} response:`, success ? "OK" : "REJECTED", message[3] || "");
          handleResponse(relayUrl, success);
        }
        // Call original handler for other messages
        originalHandleMessage(relayUrl, message);
      };

      // Send to all connected relays
      for (const [relayUrl, ws] of this.relays) {
        if (ws.readyState === WebSocket.OPEN) {
          try {
            ws.send(JSON.stringify(eventMessage));
            publishedCount++;
            console.log(`Event sent to ${relayUrl}`);
          } catch (error) {
            console.error(`Failed to send event to ${relayUrl}:`, error);
            handleResponse(relayUrl, false);
          }
        } else {
          console.warn(`Relay ${relayUrl} is not open, skipping`);
          handleResponse(relayUrl, false);
        }
      }

      // Restore original handler after timeout
      setTimeout(() => {
        this.handleMessage = originalHandleMessage;
        if (okCount + errorCount < totalRelays) {
          reject(new Error("Timeout waiting for relay responses"));
        }
      }, 10000); // 10 second timeout
    });
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

// Fetch user profile metadata (kind 0)
export async function fetchUserProfile(pubkey) {
  return new Promise(async (resolve, reject) => {
    console.log(`Starting profile fetch for pubkey: ${pubkey}`);

    let resolved = false;
    let newestEvent = null;
    let debounceTimer = null;
    let overallTimer = null;
    let subscriptionId = null;

    function cleanup() {
      if (subscriptionId) {
        try {
          nostrClient.unsubscribe(subscriptionId);
        } catch {}
      }
      if (debounceTimer) clearTimeout(debounceTimer);
      if (overallTimer) clearTimeout(overallTimer);
    }

    // 1) Try cached profile first and resolve immediately if present
    try {
      const cachedEvent = await getLatestProfileEvent(pubkey);
      if (cachedEvent) {
        console.log("Using cached profile event");
        const profile = parseProfileFromEvent(cachedEvent);
        resolved = true; // resolve immediately with cache
        resolve(profile);
      }
    } catch (e) {
      console.warn("Failed to load cached profile", e);
    }

    // 2) Set overall timeout
    overallTimer = setTimeout(() => {
      if (!newestEvent) {
        console.log("Profile fetch timeout reached");
        if (!resolved) reject(new Error("Profile fetch timeout"));
      } else if (!resolved) {
        resolve(parseProfileFromEvent(newestEvent));
      }
      cleanup();
    }, 15000);

    // 3) Wait a bit to ensure connections are ready and then subscribe without limit
    setTimeout(() => {
      console.log("Starting subscription after connection delay...");
      subscriptionId = nostrClient.subscribe(
        {
          kinds: [0],
          authors: [pubkey],
        },
        (event) => {
          // Collect all kind 0 events and pick the newest by created_at
          if (!event || event.kind !== 0) return;
          console.log("Profile event received:", event);

          if (
            !newestEvent ||
            (event.created_at || 0) > (newestEvent.created_at || 0)
          ) {
            newestEvent = event;
          }

          // Debounce to wait for more relays; then finalize selection
          if (debounceTimer) clearTimeout(debounceTimer);
          debounceTimer = setTimeout(async () => {
            try {
              if (newestEvent) {
                await putEvent(newestEvent); // cache newest only
                const profile = parseProfileFromEvent(newestEvent);

                // Notify listeners that an updated profile is available
                try {
                  if (typeof window !== "undefined" && window.dispatchEvent) {
                    window.dispatchEvent(
                      new CustomEvent("profile-updated", {
                        detail: { pubkey, profile, event: newestEvent },
                      }),
                    );
                  }
                } catch (e) {
                  console.warn("Failed to dispatch profile-updated event", e);
                }

                if (!resolved) {
                  resolve(profile);
                  resolved = true;
                }
              }
            } finally {
              cleanup();
            }
          }, 800);
        },
      );
    }, 2000);
  });
}

// Fetch events using WebSocket REQ envelopes
export async function fetchEvents(filters, options = {}) {
  return new Promise(async (resolve, reject) => {
    console.log(`Starting event fetch with filters:`, filters);

    let resolved = false;
    let events = [];
    let debounceTimer = null;
    let overallTimer = null;
    let subscriptionId = null;
    let eoseReceived = false;

    const {
      timeout = 30000,
      debounceDelay = 1000,
      limit = null
    } = options;

    function cleanup() {
      if (subscriptionId) {
        try {
          nostrClient.unsubscribe(subscriptionId);
        } catch {}
      }
      if (debounceTimer) clearTimeout(debounceTimer);
      if (overallTimer) clearTimeout(overallTimer);
    }

    // Set overall timeout
    overallTimer = setTimeout(() => {
      if (!resolved) {
        console.log("Event fetch timeout reached");
        if (events.length > 0) {
          resolve(events);
        } else {
          reject(new Error("Event fetch timeout"));
        }
        resolved = true;
      }
      cleanup();
    }, timeout);

    // Subscribe to events
    setTimeout(() => {
      console.log("Starting event subscription...");
      
      // Add limit to filters if specified
      const requestFilters = { ...filters };
      if (limit) {
        requestFilters.limit = limit;
      }

      console.log('Sending REQ with filters:', requestFilters);

      subscriptionId = nostrClient.subscribe(
        requestFilters,
        (event) => {
          if (!event) return;
          console.log("Event received:", event);
          
          // Check if we already have this event (deduplication)
          const existingEvent = events.find(e => e.id === event.id);
          if (!existingEvent) {
            events.push(event);
          }

          // If we have a limit and reached it, resolve immediately
          if (limit && events.length >= limit) {
            if (!resolved) {
              resolve(events.slice(0, limit));
              resolved = true;
            }
            cleanup();
            return;
          }

          // Debounce to wait for more events
          if (debounceTimer) clearTimeout(debounceTimer);
          debounceTimer = setTimeout(() => {
            if (eoseReceived && !resolved) {
              resolve(events);
              resolved = true;
              cleanup();
            }
          }, debounceDelay);
        },
      );

      // Listen for EOSE events
      const handleEOSE = (event) => {
        if (event.detail.subscriptionId === subscriptionId) {
          console.log("EOSE received for subscription", subscriptionId);
          eoseReceived = true;
          
          // If we haven't resolved yet and have events, resolve now
          if (!resolved && events.length > 0) {
            resolve(events);
            resolved = true;
            cleanup();
          }
        }
      };

      // Add EOSE listener
      window.addEventListener('nostr-eose', handleEOSE);

      // Cleanup EOSE listener
      const originalCleanup = cleanup;
      cleanup = () => {
        window.removeEventListener('nostr-eose', handleEOSE);
        originalCleanup();
      };
    }, 1000);
  });
}

// Fetch all events with timestamp-based pagination
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

// Fetch user's events with timestamp-based pagination
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

// NIP-50 search function
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
