// Helper functions and constants for the ORLY dashboard

// Comprehensive kind names mapping
export const KIND_NAMES = {
  0: "Profile Metadata",
  1: "Text Note",
  2: "Recommend Relay",
  3: "Contacts",
  4: "Encrypted DM",
  5: "Delete Request",
  6: "Repost",
  7: "Reaction",
  8: "Badge Award",
  16: "Generic Repost",
  40: "Channel Creation",
  41: "Channel Metadata",
  42: "Channel Message",
  43: "Channel Hide Message",
  44: "Channel Mute User",
  1063: "File Metadata",
  1311: "Live Chat Message",
  1984: "Reporting",
  1985: "Label",
  9734: "Zap Request",
  9735: "Zap Receipt",
  10000: "Mute List",
  10001: "Pin List",
  10002: "Relay List Metadata",
  10003: "Bookmark List",
  10004: "Communities List",
  10005: "Public Chats List",
  10006: "Blocked Relays List",
  10007: "Search Relays List",
  10009: "User Groups",
  10015: "Interests List",
  10030: "User Emoji List",
  13194: "Wallet Info",
  22242: "Client Auth",
  23194: "Wallet Request",
  23195: "Wallet Response",
  24133: "Nostr Connect",
  27235: "HTTP Auth",
  30000: "Categorized People List",
  30001: "Categorized Bookmarks",
  30002: "Categorized Relay List",
  30003: "Bookmark Sets",
  30004: "Curation Sets",
  30005: "Video Sets",
  30008: "Profile Badges",
  30009: "Badge Definition",
  30015: "Interest Sets",
  30017: "Create/Update Stall",
  30018: "Create/Update Product",
  30019: "Marketplace UI/UX",
  30020: "Product Sold As Auction",
  30023: "Long-form Content",
  30024: "Draft Long-form Content",
  30030: "Emoji Sets",
  30063: "Release Artifact Sets",
  30078: "Application-specific Data",
  30311: "Live Event",
  30315: "User Statuses",
  30388: "Slide Set",
  30402: "Classified Listing",
  30403: "Draft Classified Listing",
  30617: "Repository Announcement",
  30618: "Repository State Announcement",
  30818: "Wiki Article",
  30819: "Redirects",
  31922: "Date-Based Calendar Event",
  31923: "Time-Based Calendar Event",
  31924: "Calendar",
  31925: "Calendar Event RSVP",
  31989: "Handler Recommendation",
  31990: "Handler Information",
  34550: "Community Definition",
  34551: "Community Post Approval",
};

// Get human-readable kind name
export function getKindName(kind) {
  return KIND_NAMES[kind] || `Kind ${kind}`;
}

// Validate hex string (for pubkeys and event IDs)
export function isValidHex(str, length = null) {
  if (!str || typeof str !== "string") return false;
  const hexRegex = /^[0-9a-fA-F]+$/;
  if (!hexRegex.test(str)) return false;
  if (length && str.length !== length) return false;
  return true;
}

// Validate pubkey (64 character hex)
export function isValidPubkey(pubkey) {
  return isValidHex(pubkey, 64);
}

// Validate event ID (64 character hex)
export function isValidEventId(eventId) {
  return isValidHex(eventId, 64);
}

// Validate tag name (single letter a-zA-Z)
export function isValidTagName(tagName) {
  return /^[a-zA-Z]$/.test(tagName);
}

// Format timestamp to localized string
export function formatTimestamp(timestamp) {
  return new Date(timestamp * 1000).toLocaleString();
}

// Format timestamp for datetime-local input
export function formatDateTimeLocal(timestamp) {
  const date = new Date(timestamp * 1000);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${year}-${month}-${day}T${hours}:${minutes}`;
}

// Parse datetime-local input to unix timestamp
export function parseDateTimeLocal(dateTimeString) {
  return Math.floor(new Date(dateTimeString).getTime() / 1000);
}

// Truncate pubkey for display
export function truncatePubkey(pubkey) {
  if (!pubkey) return "";
  return pubkey.slice(0, 8) + "..." + pubkey.slice(-8);
}

// Truncate content for display
export function truncateContent(content, maxLength = 100) {
  if (!content) return "";
  return content.length > maxLength ? content.slice(0, maxLength) + "..." : content;
}

// Build Nostr filter from form data
export function buildFilter({
  searchText = null,
  kinds = [],
  authors = [],
  ids = [],
  tags = [],
  since = null,
  until = null,
  limit = null,
}) {
  const filter = {};
  
  if (searchText && searchText.trim()) {
    filter.search = searchText.trim();
  }
  
  if (kinds && kinds.length > 0) {
    filter.kinds = kinds;
  }
  
  if (authors && authors.length > 0) {
    filter.authors = authors;
  }
  
  if (ids && ids.length > 0) {
    filter.ids = ids;
  }
  
  // Add tag filters (e.g., #e, #p, #a)
  if (tags && tags.length > 0) {
    tags.forEach(tag => {
      if (tag.name && tag.value) {
        const tagKey = `#${tag.name}`;
        if (!filter[tagKey]) {
          filter[tagKey] = [];
        }
        filter[tagKey].push(tag.value);
      }
    });
  }
  
  if (since) {
    filter.since = since;
  }
  
  if (until) {
    filter.until = until;
  }
  
  if (limit && limit > 0) {
    filter.limit = limit;
  }
  
  return filter;
}

// Pretty print JSON with word breaking for long strings
export function prettyPrintFilter(filter) {
  return JSON.stringify(filter, null, 2);
}

