// Default Nostr relays for searching
export const DEFAULT_RELAYS = [
  // Use the local relay WebSocket endpoint
  // Automatically use ws:// for http:// and wss:// for https://
  `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/`,
];
