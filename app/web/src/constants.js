// Default Nostr relays for searching
export const DEFAULT_RELAYS = [
  // Use the local relay WebSocket endpoint
  `wss://${window.location.host}/ws`,
  // Fallback to external relays if local fails
  "wss://relay.damus.io",
  "wss://relay.nostr.band",
  "wss://nos.lol",
  "wss://relay.nostr.net",
  "wss://relay.minibits.cash",
  "wss://relay.coinos.io/",
  "wss://nwc.primal.net",
  "wss://relay.orly.dev",
];
