import React, { useState, useEffect, useRef } from 'react';
import JSONPretty from 'react-json-pretty';

function PrettyJSONView({ jsonString, maxHeightClass = 'max-h-64' }) {
  let data;
  try {
    data = JSON.parse(jsonString);
  } catch (_) {
    data = jsonString;
  }
  return (
    <div
      className={`text-xs p-2 rounded overflow-auto ${maxHeightClass} break-all break-words whitespace-pre-wrap bg-gray-950 text-white`}
      style={{ overflowWrap: 'anywhere', wordBreak: 'break-word' }}
    >
      <JSONPretty data={data} space={2} />
    </div>
  );
}

function App() {
  const [user, setUser] = useState(null);
  const [status, setStatus] = useState('Ready to authenticate');
  const [statusType, setStatusType] = useState('info');
  const [profileData, setProfileData] = useState(null);

  // Theme state for dark/light mode
  const [isDarkMode, setIsDarkMode] = useState(false);

  const [checkingAuth, setCheckingAuth] = useState(true);

  // Events log state
  const [events, setEvents] = useState([]);
  const [eventsLoading, setEventsLoading] = useState(false);
  const [eventsOffset, setEventsOffset] = useState(0);
  const [eventsHasMore, setEventsHasMore] = useState(true);
  const [expandedEventId, setExpandedEventId] = useState(null);

  // All Events log state (admin)
  const [allEvents, setAllEvents] = useState([]);
  const [allEventsLoading, setAllEventsLoading] = useState(false);
  const [allEventsOffset, setAllEventsOffset] = useState(0);
  const [allEventsHasMore, setAllEventsHasMore] = useState(true);
  const [expandedAllEventId, setExpandedAllEventId] = useState(null);

  // Search state
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchOffset, setSearchOffset] = useState(0);
  const [searchHasMore, setSearchHasMore] = useState(true);
  const [expandedSearchEventId, setExpandedSearchEventId] = useState(null);

  // Profile cache for All Events Log
  const [profileCache, setProfileCache] = useState({});

  // Function to fetch and cache profile metadata for an author
  async function fetchAndCacheProfile(pubkeyHex) {
    if (!pubkeyHex || profileCache[pubkeyHex]) {
      return profileCache[pubkeyHex] || null;
    }

    try {
      const profile = await fetchKind0FromRelay(pubkeyHex);
      if (profile) {
        setProfileCache(prev => ({
          ...prev,
          [pubkeyHex]: {
            name: profile.name || `user:${pubkeyHex.slice(0, 8)}`,
            display_name: profile.display_name,
            picture: profile.picture,
            about: profile.about
          }
        }));
        return profile;
      }
    } catch (error) {
      console.log('Error fetching profile for', pubkeyHex.slice(0, 8), ':', error);
    }
    return null;
  }

  // Function to fetch profiles for all events in a batch
  async function fetchProfilesForEvents(events) {
    const uniqueAuthors = [...new Set(events.map(event => event.author).filter(Boolean))];
    const fetchPromises = uniqueAuthors.map(author => fetchAndCacheProfile(author));
    await Promise.allSettled(fetchPromises);
  }

  // Section revealer states
  const [expandedSections, setExpandedSections] = useState({
    welcome: true,
    exportMine: false,
    exportAll: false,
    exportSpecific: false,
    importEvents: false,
    search: true,
    eventsLog: false,
    allEventsLog: false
  });


  // Login view layout measurements
  const titleRef = useRef(null);
  const fileInputRef = useRef(null);
  const [loginPadding, setLoginPadding] = useState(16); // default fallback padding in px

  useEffect(() => {
    function updatePadding() {
      if (titleRef.current) {
        const h = titleRef.current.offsetHeight || 0;
        // Pad area around the text by half the title text height
        setLoginPadding(Math.max(0, Math.round(h / 2)));
      }
    }
    updatePadding();
    window.addEventListener('resize', updatePadding);
    return () => window.removeEventListener('resize', updatePadding);
  }, []);

  // Effect to detect and track system theme preference
  useEffect(() => {
    // Check if the browser supports prefers-color-scheme
    const darkModeMediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

    // Set the initial theme based on system preference
    setIsDarkMode(darkModeMediaQuery.matches);

    // Add listener to respond to system theme changes
    const handleThemeChange = (e) => {
      setIsDarkMode(e.matches);
    };

    // Modern browsers
    darkModeMediaQuery.addEventListener('change', handleThemeChange);

    // Cleanup listener on component unmount
    return () => {
      darkModeMediaQuery.removeEventListener('change', handleThemeChange);
    };
  }, []);

  useEffect(() => {
    // Check authentication status on page load
    (async () => {
      await checkStatus();
      setCheckingAuth(false);
    })();
  }, []);

  // Effect to fetch profile when user changes
  useEffect(() => {
    if (user?.pubkey) {
      fetchUserProfile(user.pubkey);
    }
  }, [user?.pubkey]);

  // Effect to fetch initial events when user is authenticated
  useEffect(() => {
    if (user?.pubkey) {
      fetchEvents(true); // true = reset
      // Also fetch all events if user is admin
      if (user.permission === 'admin') {
        fetchAllEvents(true); // true = reset
      }
    }
  }, [user?.pubkey, user?.permission]);

  function relayURL() {
    try {
      return window.location.protocol.replace('http', 'ws') + '//' + window.location.host;
    } catch (_) {
      return 'ws://localhost:3333';
    }
  }

  async function checkStatus() {
    try {
      const response = await fetch('/api/auth/status');
      const data = await response.json();
      if (data.authenticated && data.pubkey) {
        // Fetch permission first, then set user and profile
        try {
          const permResponse = await fetch(`/api/permissions/${data.pubkey}`);
          const permData = await permResponse.json();
          if (permData && permData.permission) {
            const fullUser = { pubkey: data.pubkey, permission: permData.permission };
            setUser(fullUser);
            updateStatus(`Already authenticated as: ${data.pubkey.slice(0, 16)}...`, 'success');
            // Fire and forget profile fetch
            fetchUserProfile(data.pubkey);
          }
        } catch (_) {
          // ignore permission fetch errors
        }
      }
    } catch (error) {
      // Ignore errors for status check
    }
  }

  function updateStatus(message, type = 'info') {
    setStatus(message);
    setStatusType(type);
  }

  function statusClassName() {
    const base = 'mt-5 mb-5 p-3 rounded';

    // Return theme-appropriate status classes
    switch (statusType) {
      case 'success':
        return base + ' ' + getThemeClasses('bg-green-100 text-green-800', 'bg-green-900 text-green-100');
      case 'error':
        return base + ' ' + getThemeClasses('bg-red-100 text-red-800', 'bg-red-900 text-red-100');
      case 'info':
      default:
        return base + ' ' + getThemeClasses('bg-cyan-100 text-cyan-800', 'bg-cyan-900 text-cyan-100');
    }
  }

  async function getChallenge() {
    try {
      const response = await fetch('/api/auth/challenge');
      const data = await response.json();
      return data.challenge;
    } catch (error) {
      updateStatus('Failed to get authentication challenge: ' + error.message, 'error');
      throw error;
    }
  }

  async function loginWithExtension() {
    if (!window.nostr) {
      updateStatus('No Nostr extension found. Please install a NIP-07 compatible extension like nos2x or Alby.', 'error');
      return;
    }

    try {
      updateStatus('Connecting to extension...', 'info');

      // Get public key from extension
      const pubkey = await window.nostr.getPublicKey();

      // Get challenge from server
      const challenge = await getChallenge();

      // Create authentication event
      const authEvent = {
        kind: 22242,
        created_at: Math.floor(Date.now() / 1000),
        tags: [
          ['relay', relayURL()],
          ['challenge', challenge]
        ],
        content: ''
      };

      // Sign the event with extension
      const signedEvent = await window.nostr.signEvent(authEvent);

      // Send to server
      await authenticate(signedEvent);

    } catch (error) {
      updateStatus('Extension login failed: ' + error.message, 'error');
    }
  }

  async function fetchKind0FromRelay(pubkeyHex, timeoutMs = 4000) {
    return new Promise((resolve) => {
      let resolved = false;
      let events = [];
      let ws;
      let reqSent = false;
      
      try {
        ws = new WebSocket(relayURL());
      } catch (e) {
        resolve(null);
        return;
      }

      const subId = 'profile-' + Math.random().toString(36).slice(2);
      const timer = setTimeout(() => {
        if (ws && ws.readyState === 1) {
          try { ws.close(); } catch (_) {}
        }
        if (!resolved) {
          resolved = true;
          resolve(null);
        }
      }, timeoutMs);

      const sendRequest = () => {
        if (!reqSent && ws && ws.readyState === 1) {
          try {
            const req = [
              'REQ',
              subId,
              { kinds: [0], authors: [pubkeyHex] }
            ];
            ws.send(JSON.stringify(req));
            reqSent = true;
          } catch (_) {}
        }
      };

      ws.onopen = () => {
        sendRequest();
      };

      ws.onmessage = async (msg) => {
        try {
          const data = JSON.parse(msg.data);
          const type = data[0];
          
          if (type === 'AUTH') {
            // Handle authentication challenge
            const challenge = data[1];
            if (!window.nostr) {
              clearTimeout(timer);
              if (!resolved) {
                resolved = true;
                resolve(null);
              }
              return;
            }

            try {
              // Create authentication event
              const authEvent = {
                kind: 22242,
                created_at: Math.floor(Date.now() / 1000),
                tags: [
                  ['relay', relayURL()],
                  ['challenge', challenge]
                ],
                content: ''
              };

              // Sign the auth event with extension
              const signedAuthEvent = await window.nostr.signEvent(authEvent);

              // Send AUTH response
              const authMessage = ['AUTH', signedAuthEvent];
              console.log('DEBUG: Sending AUTH response for profile fetch challenge:', challenge.slice(0, 16) + '...');
              ws.send(JSON.stringify(authMessage));
            } catch (authError) {
              clearTimeout(timer);
              if (!resolved) {
                resolved = true;
                resolve(null);
              }
            }
          } else if (type === 'EVENT' && data[1] === subId) {
            const event = data[2];
            if (event && event.kind === 0 && event.content) {
              events.push(event);
            }
          } else if (type === 'EOSE' && data[1] === subId) {
            try {
              ws.send(JSON.stringify(['CLOSE', subId]));
            } catch (_) {}
            try { ws.close(); } catch (_) {}
            clearTimeout(timer);
            if (!resolved) {
              resolved = true;
              if (events.length) {
                const latest = events.reduce((a, b) => (a.created_at > b.created_at ? a : b));
                try {
                  const meta = JSON.parse(latest.content);
                  resolve(meta || null);
                } catch (_) {
                  resolve(null);
                }
              } else {
                resolve(null);
              }
            }
          } else if (type === 'CLOSED' && data[1] === subId) {
            const message = data[2] || '';
            if (message.includes('auth-required') && !reqSent) {
              // Wait for AUTH challenge, request will be sent after authentication
              return;
            }
            // Subscription was closed, finish processing
            clearTimeout(timer);
            if (!resolved) {
              resolved = true;
              if (events.length) {
                const latest = events.reduce((a, b) => (a.created_at > b.created_at ? a : b));
                try {
                  const meta = JSON.parse(latest.content);
                  resolve(meta || null);
                } catch (_) {
                  resolve(null);
                }
              } else {
                resolve(null);
              }
            }
          } else if (type === 'OK' && data[1] && data[1].length === 64 && !reqSent) {
            // This might be an OK response to our AUTH event
            // Send the original request now that we're authenticated
            sendRequest();
          }
        } catch (_) {
          // ignore malformed messages
        }
      };

      ws.onerror = () => {
        try { ws.close(); } catch (_) {}
        clearTimeout(timer);
        if (!resolved) {
          resolved = true;
          resolve(null);
        }
      };

      ws.onclose = () => {
        clearTimeout(timer);
        if (!resolved) {
          resolved = true;
          if (events.length) {
            const latest = events.reduce((a, b) => (a.created_at > b.created_at ? a : b));
            try {
              const meta = JSON.parse(latest.content);
              resolve(meta || null);
            } catch (_) {
              resolve(null);
            }
          } else {
            resolve(null);
          }
        }
      };
    });
  }

  // Function to fetch user profile metadata (kind 0)
  async function fetchUserProfile(pubkeyHex) {
    try {
      // Create a simple placeholder with the pubkey
      const placeholderProfile = {
        name: `user:${pubkeyHex.slice(0, 8)}`,
        about: 'No profile data available'
      };

      // Always set the placeholder profile first
      setProfileData(placeholderProfile);

      // First, try to get profile kind:0 from the relay itself
      let relayMetadata = null;
      try {
        relayMetadata = await fetchKind0FromRelay(pubkeyHex);
      } catch (_) {}

      if (relayMetadata) {
        const parsed = typeof relayMetadata === 'string' ? JSON.parse(relayMetadata) : relayMetadata;
        setProfileData({
          name: parsed.name || placeholderProfile.name,
          display_name: parsed.display_name,
          picture: parsed.picture,
          banner: parsed.banner,
          about: parsed.about || placeholderProfile.about
        });
        return parsed;
      }

      // Fallback: try extension metadata if available
      if (window.nostr && window.nostr.getPublicKey) {
        try {
          if (window.nostr.getUserMetadata) {
            const metadata = await window.nostr.getUserMetadata();
            if (metadata) {
              try {
                const parsedMetadata = typeof metadata === 'string' ? JSON.parse(metadata) : metadata;
                setProfileData({
                  name: parsedMetadata.name || placeholderProfile.name,
                  display_name: parsedMetadata.display_name,
                  picture: parsedMetadata.picture,
                  banner: parsedMetadata.banner,
                  about: parsedMetadata.about || placeholderProfile.about
                });
                return parsedMetadata;
              } catch (parseError) {
                console.log('Error parsing user metadata:', parseError);
              }
            }
          }
        } catch (nostrError) {
          console.log('Could not get profile from extension:', nostrError);
        }
      }

      return placeholderProfile;
    } catch (error) {
      console.error('Error handling profile data:', error);
      return null;
    }
  }

  async function authenticate(signedEvent) {
    try {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(signedEvent)
      });

      const result = await response.json();

      if (result.success) {
        setUser(result.pubkey);
        updateStatus('Successfully authenticated as: ' + result.pubkey.slice(0, 16) + '...', 'success');

        // Check permissions after login
        const permResponse = await fetch(`/api/permissions/${result.pubkey}`);
        const permData = await permResponse.json();
        if (permData && permData.permission) {
          setUser({pubkey: result.pubkey, permission: permData.permission});

          // Fetch user profile data
          await fetchUserProfile(result.pubkey);
        }
      } else {
        updateStatus('Authentication failed: ' + result.error, 'error');
      }
    } catch (error) {
      updateStatus('Authentication request failed: ' + error.message, 'error');
    }
  }

  async function logout() {
    try {
      await fetch('/api/auth/logout', { method: 'POST' });
    } catch (_) {}
    setUser(null);
    setProfileData(null);
    // Clear events state
    setEvents([]);
    setEventsOffset(0);
    setEventsHasMore(true);
    setExpandedEventId(null);
    // Clear all events state
    setAllEvents([]);
    setAllEventsOffset(0);
    setAllEventsHasMore(true);
    setExpandedAllEventId(null);
    updateStatus('Logged out', 'info');
  }

  // WebSocket-based function to fetch events from relay
  async function fetchEventsFromRelay(reset = false, limit = 50, timeoutMs = 10000) {
    if (!user?.pubkey) return;
    if (eventsLoading) return;
    if (!reset && !eventsHasMore) return;

    console.log('DEBUG: fetchEventsFromRelay called, reset:', reset, 'offset:', eventsOffset);
    setEventsLoading(true);

    return new Promise((resolve) => {
      let resolved = false;
      let receivedEvents = [];
      let ws;
      let reqSent = false;

      try {
        ws = new WebSocket(relayURL());
      } catch (e) {
        console.error('Failed to create WebSocket:', e);
        setEventsLoading(false);
        resolve();
        return;
      }

      const subId = 'events-' + Math.random().toString(36).slice(2);
      const timer = setTimeout(() => {
        if (ws && ws.readyState === 1) {
          try { ws.close(); } catch (_) {}
        }
        if (!resolved) {
          resolved = true;
          console.log('DEBUG: WebSocket timeout, received events:', receivedEvents.length);
          processEventsResponse(receivedEvents, reset);
          resolve();
        }
      }, timeoutMs);

      const sendRequest = () => {
        if (!reqSent && ws && ws.readyState === 1) {
          try {
            // Request events from the authenticated user
            const req = [
              'REQ',
              subId,
              { authors: [user.pubkey] }
            ];
            console.log('DEBUG: Sending WebSocket request:', req);
            ws.send(JSON.stringify(req));
            reqSent = true;
          } catch (e) {
            console.error('Failed to send WebSocket request:', e);
          }
        }
      };

      ws.onopen = () => {
        sendRequest();
      };

      ws.onmessage = async (msg) => {
        try {
          const data = JSON.parse(msg.data);
          const type = data[0];
          console.log('DEBUG: WebSocket message:', type, data.length > 2 ? 'with event' : '');

          if (type === 'AUTH') {
            // Handle authentication challenge
            const challenge = data[1];
            if (!window.nostr) {
              console.error('Authentication required but no Nostr extension found');
              clearTimeout(timer);
              if (!resolved) {
                resolved = true;
                processEventsResponse(receivedEvents, reset);
                resolve();
              }
              return;
            }

            try {
              // Create authentication event
              const authEvent = {
                kind: 22242,
                created_at: Math.floor(Date.now() / 1000),
                tags: [
                  ['relay', relayURL()],
                  ['challenge', challenge]
                ],
                content: ''
              };

              // Sign the auth event with extension
              const signedAuthEvent = await window.nostr.signEvent(authEvent);

              // Send AUTH response
              const authMessage = ['AUTH', signedAuthEvent];
              console.log('DEBUG: Sending AUTH response for events fetch challenge:', challenge.slice(0, 16) + '...');
              ws.send(JSON.stringify(authMessage));
            } catch (authError) {
              console.error('Failed to authenticate:', authError);
              clearTimeout(timer);
              if (!resolved) {
                resolved = true;
                processEventsResponse(receivedEvents, reset);
                resolve();
              }
            }
          } else if (type === 'EVENT' && data[1] === subId) {
            const event = data[2];
            if (event) {
              // Convert to the expected format
              const formattedEvent = {
                id: event.id,
                kind: event.kind,
                created_at: event.created_at,
                content: event.content || '',
                raw_json: JSON.stringify(event)
              };
              receivedEvents.push(formattedEvent);
            }
          } else if (type === 'EOSE' && data[1] === subId) {
            try {
              ws.send(JSON.stringify(['CLOSE', subId]));
            } catch (_) {}
            try { ws.close(); } catch (_) {}
            clearTimeout(timer);
            if (!resolved) {
              resolved = true;
              console.log('DEBUG: EOSE received, processing events:', receivedEvents.length);
              processEventsResponse(receivedEvents, reset);
              resolve();
            }
          } else if (type === 'CLOSED' && data[1] === subId) {
            const message = data[2] || '';
            console.log('DEBUG: Subscription closed:', message);
            if (message.includes('auth-required') && !reqSent) {
              // Wait for AUTH challenge, request will be sent after authentication
              return;
            }
            // Subscription was closed, finish processing
            clearTimeout(timer);
            if (!resolved) {
              resolved = true;
              processEventsResponse(receivedEvents, reset);
              resolve();
            }
          } else if (type === 'OK' && data[1] && data[1].length === 64 && !reqSent) {
            // This might be an OK response to our AUTH event
            // Send the original request now that we're authenticated
            sendRequest();
          }
        } catch (e) {
          console.error('Error parsing WebSocket message:', e);
        }
      };

      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        try { ws.close(); } catch (_) {}
        clearTimeout(timer);
        if (!resolved) {
          resolved = true;
          processEventsResponse(receivedEvents, reset);
          resolve();
        }
      };

      ws.onclose = () => {
        clearTimeout(timer);
        if (!resolved) {
          resolved = true;
          console.log('DEBUG: WebSocket closed, processing events:', receivedEvents.length);
          processEventsResponse(receivedEvents, reset);
          resolve();
        }
      };
    });
  }

  // Helper function to filter out deleted events and process delete events
  function filterDeletedEvents(events) {
    // Find all delete events (kind 5)
    const deleteEvents = events.filter(event => event.kind === 5);
    
    // Extract the event IDs that have been deleted
    const deletedEventIds = new Set();
    deleteEvents.forEach(deleteEvent => {
      try {
        const originalEvent = JSON.parse(deleteEvent.raw_json);
        // Look for 'e' tags in the delete event that reference deleted events
        if (originalEvent.tags) {
          originalEvent.tags.forEach(tag => {
            if (tag[0] === 'e' && tag[1]) {
              deletedEventIds.add(tag[1]);
            }
          });
        }
      } catch (error) {
        console.error('Error parsing delete event:', error);
      }
    });

    // Filter out events that have been deleted, but keep delete events themselves
    const filteredEvents = events.filter(event => {
      // Always show delete events (kind 5)
      if (event.kind === 5) {
        return true;
      }
      // Hide events that have been deleted
      return !deletedEventIds.has(event.id);
    });

    console.log('DEBUG: Filtered events - original:', events.length, 'filtered:', filteredEvents.length, 'deleted IDs:', deletedEventIds.size);
    return filteredEvents;
  }

  function processEventsResponse(receivedEvents, reset) {
    try {
      // Filter out deleted events and ensure delete events are included
      const filteredEvents = filterDeletedEvents(receivedEvents);
      
      // Sort events by created_at in descending order (newest first)
      const sortedEvents = filteredEvents.sort((a, b) => b.created_at - a.created_at);

      // Apply pagination manually since we get all events from WebSocket
      const currentOffset = reset ? 0 : eventsOffset;
      const limit = 50;
      const paginatedEvents = sortedEvents.slice(currentOffset, currentOffset + limit);

      console.log('DEBUG: Processing events - total:', sortedEvents.length, 'paginated:', paginatedEvents.length, 'offset:', currentOffset);

      if (reset) {
        setEvents(paginatedEvents);
        setEventsOffset(paginatedEvents.length);
      } else {
        setEvents(prev => [...prev, ...paginatedEvents]);
        setEventsOffset(prev => prev + paginatedEvents.length);
      }

      // Check if there are more events available
      setEventsHasMore(currentOffset + paginatedEvents.length < sortedEvents.length);

      console.log('DEBUG: Events updated, displayed count:', paginatedEvents.length, 'has more:', currentOffset + paginatedEvents.length < sortedEvents.length);
    } catch (error) {
      console.error('Error processing events response:', error);
    } finally {
      setEventsLoading(false);
    }
  }

  // WebSocket-based function to fetch all events from relay (admin)
  async function fetchAllEventsFromRelay(reset = false, limit = 50, timeoutMs = 10000) {
    if (!user?.pubkey || user.permission !== 'admin') return;
    if (allEventsLoading) return;
    if (!reset && !allEventsHasMore) return;

    console.log('DEBUG: fetchAllEventsFromRelay called, reset:', reset, 'offset:', allEventsOffset);
    setAllEventsLoading(true);

    return new Promise((resolve) => {
      let resolved = false;
      let receivedEvents = [];
      let ws;
      let reqSent = false;

      try {
        ws = new WebSocket(relayURL());
      } catch (e) {
        console.error('Failed to create WebSocket:', e);
        setAllEventsLoading(false);
        resolve();
        return;
      }

      const subId = 'allevents-' + Math.random().toString(36).slice(2);
      const timer = setTimeout(() => {
        if (ws && ws.readyState === 1) {
          try { ws.close(); } catch (_) {}
        }
        if (!resolved) {
          resolved = true;
          console.log('DEBUG: WebSocket timeout, received all events:', receivedEvents.length);
          processAllEventsResponse(receivedEvents, reset);
          resolve();
        }
      }, timeoutMs);

      const sendRequest = () => {
        if (!reqSent && ws && ws.readyState === 1) {
          try {
            // Request all events (no authors filter for admin)
            const req = [
              'REQ',
              subId,
              {}
            ];
            console.log('DEBUG: Sending WebSocket request for all events:', req);
            ws.send(JSON.stringify(req));
            reqSent = true;
          } catch (e) {
            console.error('Failed to send WebSocket request:', e);
          }
        }
      };

      ws.onopen = () => {
        sendRequest();
      };

      ws.onmessage = async (msg) => {
        try {
          const data = JSON.parse(msg.data);
          const type = data[0];
          console.log('DEBUG: WebSocket message:', type, data.length > 2 ? 'with event' : '');

          if (type === 'AUTH') {
            // Handle authentication challenge
            const challenge = data[1];
            if (!window.nostr) {
              console.error('Authentication required but no Nostr extension found');
              clearTimeout(timer);
              if (!resolved) {
                resolved = true;
                processAllEventsResponse(receivedEvents, reset);
                resolve();
              }
              return;
            }

            try {
              // Create authentication event
              const authEvent = {
                kind: 22242,
                created_at: Math.floor(Date.now() / 1000),
                tags: [
                  ['relay', relayURL()],
                  ['challenge', challenge]
                ],
                content: ''
              };

              // Sign the auth event with extension
              const signedAuthEvent = await window.nostr.signEvent(authEvent);

              // Send AUTH response
              const authMessage = ['AUTH', signedAuthEvent];
              console.log('DEBUG: Sending AUTH response for all events fetch challenge:', challenge.slice(0, 16) + '...');
              ws.send(JSON.stringify(authMessage));
            } catch (authError) {
              console.error('Failed to authenticate:', authError);
              clearTimeout(timer);
              if (!resolved) {
                resolved = true;
                processAllEventsResponse(receivedEvents, reset);
                resolve();
              }
            }
          } else if (type === 'EVENT' && data[1] === subId) {
            const event = data[2];
            if (event) {
              // Convert to the expected format
              const formattedEvent = {
                id: event.id,
                kind: event.kind,
                created_at: event.created_at,
                content: event.content || '',
                author: event.pubkey || '',
                raw_json: JSON.stringify(event)
              };
              receivedEvents.push(formattedEvent);
            }
          } else if (type === 'EOSE' && data[1] === subId) {
            try {
              ws.send(JSON.stringify(['CLOSE', subId]));
            } catch (_) {}
            try { ws.close(); } catch (_) {}
            clearTimeout(timer);
            if (!resolved) {
              resolved = true;
              console.log('DEBUG: EOSE received, processing all events:', receivedEvents.length);
              processAllEventsResponse(receivedEvents, reset);
              resolve();
            }
          } else if (type === 'CLOSED' && data[1] === subId) {
            const message = data[2] || '';
            console.log('DEBUG: All events subscription closed:', message);
            if (message.includes('auth-required') && !reqSent) {
              // Wait for AUTH challenge, request will be sent after authentication
              return;
            }
            // Subscription was closed, finish processing
            clearTimeout(timer);
            if (!resolved) {
              resolved = true;
              processAllEventsResponse(receivedEvents, reset);
              resolve();
            }
          } else if (type === 'OK' && data[1] && data[1].length === 64 && !reqSent) {
            // This might be an OK response to our AUTH event
            // Send the original request now that we're authenticated
            sendRequest();
          }
        } catch (e) {
          console.error('Error parsing WebSocket message:', e);
        }
      };

      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        try { ws.close(); } catch (_) {}
        clearTimeout(timer);
        if (!resolved) {
          resolved = true;
          processAllEventsResponse(receivedEvents, reset);
          resolve();
        }
      };

      ws.onclose = () => {
        clearTimeout(timer);
        if (!resolved) {
          resolved = true;
          console.log('DEBUG: WebSocket closed, processing all events:', receivedEvents.length);
          processAllEventsResponse(receivedEvents, reset);
          resolve();
        }
      };
    });
  }

  function processAllEventsResponse(receivedEvents, reset) {
    try {
      // Filter out deleted events and ensure delete events are included
      const filteredEvents = filterDeletedEvents(receivedEvents);
      
      // Sort events by created_at in descending order (newest first)
      const sortedEvents = filteredEvents.sort((a, b) => b.created_at - a.created_at);

      // Apply pagination manually since we get all events from WebSocket
      const currentOffset = reset ? 0 : allEventsOffset;
      const limit = 50;
      const paginatedEvents = sortedEvents.slice(currentOffset, currentOffset + limit);

      console.log('DEBUG: Processing all events - total:', sortedEvents.length, 'paginated:', paginatedEvents.length, 'offset:', currentOffset);

      if (reset) {
        setAllEvents(paginatedEvents);
        setAllEventsOffset(paginatedEvents.length);
      } else {
        setAllEvents(prev => [...prev, ...paginatedEvents]);
        setAllEventsOffset(prev => prev + paginatedEvents.length);
      }

      // Check if there are more events available
      setAllEventsHasMore(currentOffset + paginatedEvents.length < sortedEvents.length);

      // Fetch profiles for the new events
      fetchProfilesForEvents(paginatedEvents);

      console.log('DEBUG: All events updated, displayed count:', paginatedEvents.length, 'has more:', currentOffset + paginatedEvents.length < sortedEvents.length);
    } catch (error) {
      console.error('Error processing all events response:', error);
    } finally {
      setAllEventsLoading(false);
    }
  }

  // Search functions
  function processSearchResponse(receivedEvents, reset) {
    try {
      const filtered = filterDeletedEvents(receivedEvents);
      const sorted = filtered.sort((a, b) => b.created_at - a.created_at);
      const currentOffset = reset ? 0 : searchOffset;
      const limit = 50;
      const page = sorted.slice(currentOffset, currentOffset + limit);
      if (reset) {
        setSearchResults(page);
        setSearchOffset(page.length);
      } else {
        setSearchResults(prev => [...prev, ...page]);
        setSearchOffset(prev => prev + page.length);
      }
      setSearchHasMore(currentOffset + page.length < sorted.length);
      // fetch profiles for authors in search results
      fetchProfilesForEvents(page);
    } catch (e) {
      console.error('Error processing search results:', e);
    } finally {
      setSearchLoading(false);
    }
  }

  async function fetchSearchResultsFromRelay(query, reset = true, limit = 50, timeoutMs = 10000) {
    if (!query || !query.trim()) {
      // clear results on empty query when resetting
      if (reset) {
        setSearchResults([]);
        setSearchOffset(0);
        setSearchHasMore(true);
      }
      return;
    }
    if (searchLoading) return;
    if (!reset && !searchHasMore) return;

    setSearchLoading(true);

    return new Promise((resolve) => {
      let resolved = false;
      let receivedEvents = [];
      let ws;
      let reqSent = false;

      try {
        ws = new WebSocket(relayURL());
      } catch (e) {
        console.error('Failed to create WebSocket:', e);
        setSearchLoading(false);
        resolve();
        return;
      }

      const subId = 'search-' + Math.random().toString(36).slice(2);
      const timer = setTimeout(() => {
        if (ws && ws.readyState === 1) {
          try { ws.close(); } catch (_) {}
        }
        if (!resolved) {
          resolved = true;
          processSearchResponse(receivedEvents, reset);
          resolve();
        }
      }, timeoutMs);

      const sendRequest = () => {
        if (!reqSent && ws && ws.readyState === 1) {
          try {
            const req = ['REQ', subId, { search: query }];
            ws.send(JSON.stringify(req));
            reqSent = true;
          } catch (e) {
            console.error('Failed to send WebSocket request:', e);
          }
        }
      };

      ws.onopen = () => sendRequest();

      ws.onmessage = async (msg) => {
        try {
          const data = JSON.parse(msg.data);
          const type = data[0];
          if (type === 'AUTH') {
            const challenge = data[1];
            if (!window.nostr) {
              clearTimeout(timer);
              if (!resolved) {
                resolved = true;
                processSearchResponse(receivedEvents, reset);
                resolve();
              }
              return;
            }
            try {
              const authEvent = { kind: 22242, created_at: Math.floor(Date.now()/1000), tags: [['relay', relayURL()], ['challenge', challenge]], content: '' };
              const signed = await window.nostr.signEvent(authEvent);
              ws.send(JSON.stringify(['AUTH', signed]));
            } catch (authErr) {
              console.error('Search auth failed:', authErr);
              clearTimeout(timer);
              if (!resolved) {
                resolved = true;
                processSearchResponse(receivedEvents, reset);
                resolve();
              }
            }
          } else if (type === 'EVENT' && data[1] === subId) {
            const ev = data[2];
            if (ev) {
              receivedEvents.push({
                id: ev.id,
                kind: ev.kind,
                created_at: ev.created_at,
                content: ev.content || '',
                author: ev.pubkey || '',
                raw_json: JSON.stringify(ev)
              });
            }
          } else if (type === 'EOSE' && data[1] === subId) {
            try { ws.send(JSON.stringify(['CLOSE', subId])); } catch (_) {}
            try { ws.close(); } catch (_) {}
            clearTimeout(timer);
            if (!resolved) {
              resolved = true;
              processSearchResponse(receivedEvents, reset);
              resolve();
            }
          } else if (type === 'CLOSED' && data[1] === subId) {
            clearTimeout(timer);
            if (!resolved) {
              resolved = true;
              processSearchResponse(receivedEvents, reset);
              resolve();
            }
          } else if (type === 'OK' && data[1] && data[1].length === 64 && !reqSent) {
            sendRequest();
          }
        } catch (e) {
          console.error('Search WS message parse error:', e);
        }
      };

      ws.onerror = (err) => {
        console.error('Search WS error:', err);
        try { ws.close(); } catch (_) {}
        clearTimeout(timer);
        if (!resolved) {
          resolved = true;
          processSearchResponse(receivedEvents, reset);
          resolve();
        }
      };

      ws.onclose = () => {
        clearTimeout(timer);
        if (!resolved) {
          resolved = true;
          processSearchResponse(receivedEvents, reset);
          resolve();
        }
      };
    });
  }

  function toggleSearchEventExpansion(eventId) {
    setExpandedSearchEventId(current => current === eventId ? null : eventId);
  }

  // Events log functions
  async function fetchEvents(reset = false) {
    await fetchEventsFromRelay(reset);
    // Also fetch user's own profile for My Events Log
    if (user?.pubkey) {
      await fetchAndCacheProfile(user.pubkey);
    }
  }

  async function fetchAllEvents(reset = false) {
    await fetchAllEventsFromRelay(reset);
  }

  function toggleEventExpansion(eventId) {
    setExpandedEventId(current => current === eventId ? null : eventId);
  }

  function toggleAllEventExpansion(eventId) {
    setExpandedAllEventId(current => current === eventId ? null : eventId);
  }

  function copyEventJSON(eventJSON) {
    try {
      // Ensure minified JSON is copied regardless of input format
      let toCopy = eventJSON;
      try {
        toCopy = JSON.stringify(JSON.parse(eventJSON));
      } catch (_) {
        // if not valid JSON string, fall back to original
      }
      navigator.clipboard.writeText(toCopy);
    } catch (error) {
      // Fallback for older browsers
      const textArea = document.createElement('textarea');
      let toCopy = eventJSON;
      try {
        toCopy = JSON.stringify(JSON.parse(eventJSON));
      } catch (_) {}
      textArea.value = toCopy;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
    }
  }

  function truncateContent(content, maxLength = 100) {
    if (!content || content.length <= maxLength) return content;
    return content.substring(0, maxLength) + '...';
  }

  function formatTimestamp(timestamp) {
    const date = new Date(timestamp * 1000);
    return date.toLocaleString();
  }

  // Function to delete an event by publishing a kind 5 delete event
  async function deleteEvent(eventId, eventRawJson, eventAuthor = null) {
    if (!user?.pubkey) {
      updateStatus('Must be logged in to delete events', 'error');
      return;
    }

    if (!window.nostr) {
      updateStatus('Nostr extension not found', 'error');
      return;
    }

    try {
      // Parse the original event to get its details
      const originalEvent = JSON.parse(eventRawJson);

      // Permission check: users can only delete their own events, admins can delete any event
      const isOwnEvent = originalEvent.pubkey === user.pubkey;
      const isAdmin = user.permission === 'admin';

      if (!isOwnEvent && !isAdmin) {
        updateStatus('You can only delete your own events', 'error');
        return;
      }

      // Construct the delete event (kind 5) according to NIP-09
      const deleteEventTemplate = {
        kind: 5,
        created_at: Math.floor(Date.now() / 1000),
        tags: [
          ['e', originalEvent.id],
          ['k', originalEvent.kind.toString()]
        ],
        content: isOwnEvent ? 'Deleted by author' : 'Deleted by admin'
      };

      // Sign the delete event with extension
      const signedDeleteEvent = await window.nostr.signEvent(deleteEventTemplate);

      // Publish the delete event to the relay via WebSocket
      await publishEventToRelay(signedDeleteEvent);

      updateStatus('Delete event published successfully', 'success');

      // Refresh the event lists to reflect the deletion
      if (isOwnEvent) {
        fetchEvents(true); // Refresh My Events
      }
      if (isAdmin) {
        fetchAllEvents(true); // Refresh All Events
      }

    } catch (error) {
      updateStatus('Failed to delete event: ' + error.message, 'error');
    }
  }

  // Function to publish an event to the relay via WebSocket
  async function publishEventToRelay(event, timeoutMs = 5000) {
    return new Promise((resolve, reject) => {
      let resolved = false;
      let ws;
      let eventSent = false;
      // Track auth flow so we can respond and then retry the original event
      let awaitingAuth = false;
      let authSent = false;
      let resentAfterAuth = false;

      try {
        ws = new WebSocket(relayURL());
      } catch (e) {
        reject(new Error('Failed to create WebSocket connection'));
        return;
      }

      const timer = setTimeout(() => {
        console.log('DEBUG: Timeout occurred - eventSent:', eventSent, 'resolved:', resolved, 'ws.readyState:', ws?.readyState);
        if (ws && ws.readyState === 1) {
          try { ws.close(); } catch (_) {}
        }
        if (!resolved) {
          resolved = true;
          reject(new Error('Timeout publishing event - no status received'));
        }
      }, timeoutMs);

      const sendEvent = () => {
        if (!eventSent && ws && ws.readyState === 1) {
          try {
            const eventMessage = ['EVENT', event];
            console.log('DEBUG: Sending event to relay:', event.id, 'kind:', event.kind);
            ws.send(JSON.stringify(eventMessage));
            eventSent = true;
          } catch (e) {
            clearTimeout(timer);
            if (!resolved) {
              resolved = true;
              reject(new Error('Failed to send event: ' + e.message));
            }
          }
        }
      };

      ws.onopen = () => {
        sendEvent();
      };

      ws.onmessage = async (msg) => {
        try {
          const data = JSON.parse(msg.data);
          const type = data[0];
          
          // Debug logging to understand what the relay is sending
          console.log('DEBUG: publishEventToRelay received message:', data);

          if (type === 'NOTICE') {
            const message = data[1] || '';
            // Some relays announce auth requirement via NOTICE
            if (/auth/i.test(message)) {
              console.log('DEBUG: Relay NOTICE indicates auth required');
              awaitingAuth = true;
            }
            return;
          }

          if (type === 'AUTH') {
            // Handle authentication challenge
            const challenge = data[1];
            if (!window.nostr) {
              clearTimeout(timer);
              if (!resolved) {
                resolved = true;
                reject(new Error('Authentication required but no Nostr extension found'));
              }
              return;
            }

            try {
              // Create authentication event
              const authEvent = {
                kind: 22242,
                created_at: Math.floor(Date.now() / 1000),
                tags: [
                  ['relay', relayURL()],
                  ['challenge', challenge]
                ],
                content: ''
              };

              // Sign the auth event with extension
              const signedAuthEvent = await window.nostr.signEvent(authEvent);

              // Send AUTH response
              const authMessage = ['AUTH', signedAuthEvent];
              ws.send(JSON.stringify(authMessage));
              authSent = true;

              // After sending AUTH, resend the original event if it was previously rejected/blocked by auth
              if (awaitingAuth && !resentAfterAuth) {
                console.log('DEBUG: AUTH sent, resending original event');
                // allow sendEvent to send again
                eventSent = false;
                resentAfterAuth = true;
                sendEvent();
              }
            } catch (authError) {
              clearTimeout(timer);
              if (!resolved) {
                resolved = true;
                reject(new Error('Failed to authenticate: ' + authError.message));
              }
            }
          } else if (type === 'OK') {
            const eventId = data[1];
            const accepted = data[2];
            const message = data[3] || '';

            console.log('DEBUG: OK message - eventId:', eventId, 'expected:', event.id, 'match:', eventId === event.id);

            if (eventId === event.id) {
              if (accepted) {
                clearTimeout(timer);
                try { ws.close(); } catch (_) {}
                if (!resolved) {
                  resolved = true;
                  resolve();
                }
              } else {
                // If auth is required, wait for AUTH flow then resend
                if (/auth/i.test(message)) {
                  console.log('DEBUG: OK rejection indicates auth required, waiting for AUTH challenge');
                  awaitingAuth = true;
                  return; // don't resolve/reject yet, wait for AUTH
                }
                clearTimeout(timer);
                try { ws.close(); } catch (_) {}
                if (!resolved) {
                  resolved = true;
                  reject(new Error('Event rejected: ' + message));
                }
              }
            } else {
              // Some relays may send an OK related to AUTH or other events
              if (authSent && awaitingAuth && !resentAfterAuth && accepted) {
                console.log('DEBUG: OK after AUTH, resending original event');
                eventSent = false;
                resentAfterAuth = true;
                sendEvent();
              }
            }
          }
        } catch (e) {
          // Ignore malformed messages
        }
      };

      ws.onerror = (error) => {
        clearTimeout(timer);
        try { ws.close(); } catch (_) {}
        if (!resolved) {
          resolved = true;
          reject(new Error('WebSocket error'));
        }
      };

      ws.onclose = () => {
        clearTimeout(timer);
        if (!resolved) {
          resolved = true;
          reject(new Error('WebSocket connection closed'));
        }
      };
    });
  }

  // Section revealer functions
  function toggleSection(sectionKey) {
    setExpandedSections(prev => ({
      ...prev,
      [sectionKey]: !prev[sectionKey]
    }));
  }


  function handleImportButton() {
    try {
      fileInputRef?.current?.click();
    } catch (_) {}
  }

  async function handleImportChange(e) {
    const file = e?.target?.files && e.target.files[0];
    if (!file) return;
    try {
      updateStatus('Uploading import file...', 'info');
      const fd = new FormData();
      fd.append('file', file);
      const res = await fetch('/api/import', { method: 'POST', body: fd });
      if (res.ok) {
        updateStatus('Import started. Processing will continue in the background.', 'success');
      } else {
        const txt = await res.text();
        updateStatus('Import failed: ' + txt, 'error');
      }
    } catch (err) {
      updateStatus('Import failed: ' + (err?.message || String(err)), 'error');
    } finally {
      // reset input so selecting the same file again works
      if (e && e.target) e.target.value = '';
    }
  }

  // =========================
  // Export Specific Pubkeys UI state and handlers (admin)
  // =========================
  const [exportPubkeys, setExportPubkeys] = useState([{ value: '' }]);

  function isHex64(str) {
    if (!str) return false;
    const s = String(str).trim();
    return /^[0-9a-fA-F]{64}$/.test(s);
  }

  function normalizeHex(str) {
    return String(str || '').trim();
  }

  function addExportPubkeyField() {
    // Add new field at the end of the list so it appears downwards
    setExportPubkeys((arr) => [...arr, { value: '' }]);
  }

  function removeExportPubkeyField(idx) {
    setExportPubkeys((arr) => arr.filter((_, i) => i !== idx));
  }

  function changeExportPubkey(idx, val) {
    const v = normalizeHex(val);
    setExportPubkeys((arr) => arr.map((item, i) => (i === idx ? { value: v } : item)));
  }

  function validExportPubkeys() {
    return exportPubkeys
      .map((p) => normalizeHex(p.value))
      .filter((v) => v.length > 0 && isHex64(v));
  }

  function canExportSpecific() {
    // Enable only if every opened field is non-empty and a valid 64-char hex
    if (!exportPubkeys || exportPubkeys.length === 0) return false;
    return exportPubkeys.every((p) => {
      const v = normalizeHex(p.value);
      return v.length === 64 && isHex64(v);
    });
  }

  function handleExportSpecific() {
    const vals = validExportPubkeys();
    if (!vals.length) return;
    const qs = vals.map((v) => `pubkey=${encodeURIComponent(v)}`).join('&');
    try {
      window.location.href = `/api/export?${qs}`;
    } catch (_) {}
  }

  // Theme utility functions for conditional styling
  function getThemeClasses(lightClass, darkClass) {
    return isDarkMode ? darkClass : lightClass;
  }

  // Get background color class for container panels
  function getPanelBgClass() {
    return getThemeClasses('bg-gray-200', 'bg-gray-800');
  }

  // Get text color class for standard text
  function getTextClass() {
    return getThemeClasses('text-gray-700', 'text-gray-300');
  }

  // Get background color for buttons
  function getButtonBgClass() {
    return getThemeClasses('bg-gray-100', 'bg-gray-700');
  }

  // Get text color for buttons
  function getButtonTextClass() {
    return getThemeClasses('text-gray-500', 'text-gray-300');
  }

  // Get hover classes for buttons
  function getButtonHoverClass() {
    return getThemeClasses('hover:text-gray-800', 'hover:text-gray-100');
  }

  // Prevent UI flash: wait until we checked auth status
  if (checkingAuth) {
    return null;
  }

  return (
    <div className={`min-h-screen ${getThemeClasses('bg-gray-100', 'bg-gray-900')}`}>
      {user?.permission ? (
        <>
          {/* Logged in view with user profile */}
          <div className={`sticky top-0 left-0 w-full ${getThemeClasses('bg-gray-100', 'bg-gray-900')} z-50 h-16 flex items-center overflow-hidden`}>
          <div className="flex items-center h-full w-full box-border">
            <div className="relative overflow-hidden flex flex-grow items-center justify-start h-full">
              {profileData?.banner && (
                <div className="absolute inset-0 opacity-70 bg-cover bg-center" style={{ backgroundImage: `url(${profileData.banner})` }}></div>
              )}
              <div className="relative z-10 p-2 flex items-center h-full">
                {profileData?.picture && <img src={profileData.picture} alt="User Avatar" className={`w-16 h-16 rounded-full object-cover border-2 ${getThemeClasses('border-white', 'border-gray-600')} mr-2 shadow box-border`} />}
                <div className={getTextClass()}>
                  <div className="font-bold text-base block">
                    {profileData?.display_name || profileData?.name || user.pubkey.slice(0, 8)}
                    {profileData?.name && profileData?.display_name && ` (${profileData.name})`}
                  </div>
                  <div className="font-bold text-lg text-left">
                    {user.permission === "admin" ? "Admin Dashboard" : "Subscriber Dashboard"}
                  </div>
                </div>
              </div>
            </div>
            <div className="flex items-center justify-end shrink-0 h-full">
              <button className={`bg-transparent ${getButtonTextClass()} border-0 text-2xl cursor-pointer flex items-center justify-center h-full aspect-square shrink-0 hover:bg-transparent ${getButtonHoverClass()}`} onClick={logout}>✕</button>
            </div>
          </div>
        </div>
          {/* Dashboard content container - stacks vertically and fills remaining space */}
          <div className="flex-grow overflow-y-auto p-4">
            {/* Hidden file input for import (admin) */}
            <input
              type="file"
              ref={fileInputRef}
              onChange={handleImportChange}
              accept=".json,.jsonl,text/plain,application/x-ndjson,application/json"
              style={{ display: 'none' }}
            />
            <div className={`m-2 p-2 w-full ${getPanelBgClass()} rounded-lg`}>
              <div
                className={`text-lg font-bold flex items-center justify-between cursor-pointer p-2 ${getTextClass()} ${getThemeClasses('hover:bg-gray-300', 'hover:bg-gray-700')} rounded`}
                onClick={() => toggleSection('welcome')}
              >
                <span>Welcome</span>
                <span className="text-xl">
                  {expandedSections.welcome ? '▼' : '▶'}
                </span>
              </div>
              {expandedSections.welcome && (
                <div className="p-2">
                  <p className={getTextClass()}>here you can configure all the things</p>
                </div>
              )}
            </div>

            {/* Export only my events */}
            <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
              <div
                className={`text-lg font-bold flex items-center justify-between cursor-pointer p-2 ${getTextClass()} ${getThemeClasses('hover:bg-gray-300', 'hover:bg-gray-700')} rounded`}
                onClick={() => toggleSection('exportMine')}
              >
                <span>Export My Events</span>
                <span className="text-xl">
                  {expandedSections.exportMine ? '▼' : '▶'}
                </span>
              </div>
              {expandedSections.exportMine && (
                <div className="w-full flex items-center justify-end p-2 bg-gray-900 rounded-lg mt-2">
                  <div className="pr-2 m-2 w-full">
                    <p className={`text-sm w-full ${getTextClass()}`}>Download your own events as line-delimited JSON (JSONL/NDJSON). Only events you authored will be included.</p>
                  </div>
                  <button
                    className={`${getButtonBgClass()} ${getButtonTextClass()} border-0 text-2xl cursor-pointer flex items-center justify-center h-full aspect-square shrink-0 hover:bg-transparent ${getButtonHoverClass()}`}
                    onClick={() => { window.location.href = '/api/export/mine'; }}
                    aria-label="Download my events as JSONL"
                    title="Download my events"
                  >
                    ⤓
                  </button>
                </div>
              )}
            </div>

            {user.permission === "admin" && (
              <>
                <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
                  <div
                    className={`text-lg font-bold flex items-center justify-between cursor-pointer p-2 ${getTextClass()} ${getThemeClasses('hover:bg-gray-300', 'hover:bg-gray-700')} rounded`}
                    onClick={() => toggleSection('exportAll')}
                  >
                    <span>Export All Events (admin)</span>
                    <span className="text-xl">
                      {expandedSections.exportAll ? '▼' : '▶'}
                    </span>
                  </div>
                  {expandedSections.exportAll && (
                    <div className="flex items-center justify-between p-2 m-4 bg-gray-900 round mt-2">
                      <div className="pr-2 w-full">
                        <p className={`text-sm ${getTextClass()}`}>Download all stored events as line-delimited JSON (JSONL/NDJSON). This may take a while on large databases.</p>
                      </div>
                      <button
                        className={`${getButtonBgClass()} ${getButtonTextClass()} border-0 text-2xl cursor-pointer flex m-2 items-center justify-center h-full aspect-square shrink-0 hover:bg-transparent ${getButtonHoverClass()}`}
                        onClick={() => { window.location.href = '/api/export'; }}
                        aria-label="Download all events as JSONL"
                        title="Download all events"
                      >
                        ⤓
                      </button>
                    </div>
                  )}
                </div>

                {/* Export specific pubkeys (admin) */}
                <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
                  <div
                    className={`text-lg font-bold flex items-center justify-between cursor-pointer p-2 ${getTextClass()} ${getThemeClasses('hover:bg-gray-300', 'hover:bg-gray-700')} rounded`}
                    onClick={() => toggleSection('exportSpecific')}
                  >
                    <span>Export Specific Pubkeys (admin)</span>
                    <span className="text-xl">
                      {expandedSections.exportSpecific ? '▼' : '▶'}
                    </span>
                  </div>
                  {expandedSections.exportSpecific && (
                    <div className="w-full flex items-start justify-between gap-4 m-2 p-2 bg-gray-900 rounded-lg mt-2">
                      {/* Left: title and help text */}
                      <div className="flex-1 pr-2 w-full">
                        <p className={`text-sm ${getTextClass()}`}>Enter one or more author pubkeys (64-character hex). Only valid entries will be exported.</p>
                        {/* Right: controls (buttons stacked vertically + list below) */}
                        <div className="flex flex-col items-end gap-2 self-end justify-end p-2">
                          <button
                            className={`${getButtonBgClass()} ${getTextClass()} text-base p-4 rounded m-2 ${getThemeClasses('hover:bg-gray-200', 'hover:bg-gray-600')}`}
                            onClick={addExportPubkeyField}
                            title="Add another pubkey"
                            type="button"
                          >
                            + Add
                          </button>
                        </div>
                        <div className="flex flex-col items-end gap-2 min-w-[320px] justify-end p-2">

                          <div className="gap-2 justify-end">
                            {exportPubkeys.map((item, idx) => {
                              const v = (item?.value || '').trim();
                              const valid = v.length === 0 ? true : isHex64(v);
                              return (
                                <div key={idx} className="flex items-center gap-2 ">
                                  <input
                                    type="text"
                                    inputMode="text"
                                    autoComplete="off"
                                    spellCheck="false"
                                    className={`flex-1 text-sm px-2 py-1 border rounded outline-none ${valid 
                                      ? getThemeClasses('border-gray-300 bg-white text-gray-900 focus:ring-2 focus:ring-blue-200', 'border-gray-600 bg-gray-700 text-gray-100 focus:ring-2 focus:ring-blue-500') 
                                      : getThemeClasses('border-red-500 bg-red-50 text-red-800', 'border-red-700 bg-red-900 text-red-200')}`}
                                    placeholder="e.g., 64-hex pubkey"
                                    value={v}
                                    onChange={(e) => changeExportPubkey(idx, e.target.value)}
                                  />
                                  <button
                                    className={`${getButtonBgClass()} ${getTextClass()} px-2 py-1 rounded ${getThemeClasses('hover:bg-gray-200', 'hover:bg-gray-600')}`}
                                    onClick={() => removeExportPubkeyField(idx)}
                                    title="Remove this pubkey"
                                    type="button"
                                  >
                                    ✕
                                  </button>
                                </div>
                              );
                            })}
                          </div>

                        </div>
                        <div className="flex justify-end items-end gap-2 self-end">
                          <button
                            className={`${getThemeClasses('bg-blue-600', 'bg-blue-500')} text-white px-3 py-1 rounded disabled:opacity-50 disabled:cursor-not-allowed ${canExportSpecific() ? getThemeClasses('hover:bg-blue-700', 'hover:bg-blue-600') : ''}`}
                            onClick={handleExportSpecific}
                            disabled={!canExportSpecific()}
                            title={canExportSpecific() ? 'Download events for specified pubkeys' : 'Enter a valid 64-character hex pubkey in every field'}
                            type="button"
                          >
                            Export
                          </button>

                        </div>
                      </div>


                    </div>
                  )}
                </div>
                <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
                  <div
                    className={`text-lg font-bold flex items-center justify-between cursor-pointer p-2 ${getTextClass()} ${getThemeClasses('hover:bg-gray-300', 'hover:bg-gray-700')} rounded`}
                    onClick={() => toggleSection('importEvents')}
                  >
                    <span>Import Events (admin)</span>
                    <span className="text-xl">
                      {expandedSections.importEvents ? '▼' : '▶'}
                    </span>
                  </div>
                  {expandedSections.importEvents && (
                    <div className="flex items-center justify-between p-2 bg-gray-900 rounded-lg mt-2">
                      <div className="pr-2 w-full">
                        <p className={`text-sm ${getTextClass()}`}>Upload events in line-delimited JSON (JSONL/NDJSON) to import into the database.</p>
                      </div>
                      <button
                        className={`${getButtonBgClass()} ${getButtonTextClass()} border-0 text-2xl cursor-pointer flex items-center justify-center h-full aspect-square shrink-0 hover:bg-transparent ${getButtonHoverClass()}`}
                        onClick={handleImportButton}
                        aria-label="Import events from JSONL"
                        title="Import events"
                      >
                        ↥
                      </button>
                    </div>
                  )}
                </div>
              </>
            )}
            {/* Search */}
            <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
              <div
                className={`text-lg font-bold flex items-center justify-between cursor-pointer p-2 ${getTextClass()} ${getThemeClasses('hover:bg-gray-300', 'hover:bg-gray-700')} rounded`}
                onClick={() => toggleSection('search')}
              >
                <span>Search</span>
                <span className="text-xl">
                  {expandedSections.search ? '▼' : '▶'}
                </span>
              </div>
              {expandedSections.search && (
                <div className="p-2 bg-gray-900 rounded-lg mt-2">
                  <div className="flex gap-2 items-center mb-3">
                    <input
                      type="text"
                      placeholder="Search notes..."
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      onKeyDown={(e) => { if (e.key === 'Enter') { fetchSearchResultsFromRelay(searchQuery, true); } }}
                      className={`${getThemeClasses('bg-white text-black border-gray-300', 'bg-gray-800 text-white border-gray-600')} border rounded px-3 py-2 flex-grow`}
                    />
                    <button
                      className={`${getThemeClasses('bg-blue-600 hover:bg-blue-700', 'bg-blue-500 hover:bg-blue-600')} text-white px-4 py-2 rounded`}
                      onClick={() => fetchSearchResultsFromRelay(searchQuery, true)}
                      disabled={searchLoading}
                      title="Search"
                    >
                      {searchLoading ? 'Searching…' : 'Search'}
                    </button>
                  </div>

                  <div className="space-y-2">
                    {searchResults.length === 0 && !searchLoading && (
                      <div className={`text-center py-4 ${getTextClass()}`}>No results</div>
                    )}

                    {searchResults.map((event) => (
                      <div key={event.id} className={`border rounded p-3 ${getThemeClasses('border-gray-300 bg-white', 'border-gray-600 bg-gray-800')}`}>
                        <div className="cursor-pointer" onClick={() => toggleSearchEventExpansion(event.id)}>
                          <div className="flex items-center justify-between w-full">
                            <div className="flex items-center gap-6 w-full">
                              <div className="flex items-center gap-3 min-w-0">
                                {event.author && profileCache[event.author] && (
                                  <>
                                    {profileCache[event.author].picture && (
                                      <img
                                        src={profileCache[event.author].picture}
                                        alt={profileCache[event.author].display_name || profileCache[event.author].name || 'User avatar'}
                                        className={`w-8 h-8 rounded-full object-cover border h-16 ${getThemeClasses('border-gray-300', 'border-gray-600')}`}
                                        onError={(e) => { e.currentTarget.style.display = 'none'; }}
                                      />
                                    )}
                                    <div className="flex flex-col flex-grow w-full">
                                      <span className={`text-sm font-medium ${getTextClass()}`}>
                                        {profileCache[event.author].display_name || profileCache[event.author].name || `${event.author.slice(0, 8)}...`}
                                      </span>
                                      {profileCache[event.author].display_name && profileCache[event.author].name && (
                                        <span className={`text-xs ${getTextClass()} opacity-70`}>
                                          {profileCache[event.author].name}
                                        </span>
                                      )}
                                    </div>
                                  </>
                                )}
                                {event.author && !profileCache[event.author] && (
                                  <span className={`text-sm font-medium ${getTextClass()}`}>
                                    {`${event.author.slice(0, 8)}...`}
                                  </span>
                                )}
                              </div>

                              <div className="flex items-center gap-3">
                                <span className={`font-mono text-sm px-2 py-1 rounded ${getThemeClasses('bg-blue-100 text-blue-800', 'bg-blue-900 text-blue-200')}`}>
                                  Kind {event.kind}
                                </span>
                                <span className={`text-sm ${getTextClass()}`}>
                                  {formatTimestamp(event.created_at)}
                                </span>
                              </div>
                            </div>
                            <div className="justify-end ml-auto rounded-full h-16 w-16 flex items-center justify-center">
                              <div className={`text-white text-xs px-4 py-4 rounded flex flex-grow items-center ${getThemeClasses('text-gray-700', 'text-gray-300')}`}>
                                {expandedSearchEventId === event.id ? '▼' : ' '}
                              </div>
                              <button
                                className="bg-red-600 hover:bg-red-700 text-white text-xs px-1 py-1 rounded flex items-center"
                                onClick={(e) => { e.stopPropagation(); deleteEvent(event.id, event.raw_json, event.author); }}
                                title="Delete this event"
                              >
                                🗑️
                              </button>
                            </div>
                          </div>

                          {event.content && (
                            <div className={`mt-2 text-sm ${getTextClass()}`}>
                              {truncateContent(event.content)}
                            </div>
                          )}
                        </div>

                        {expandedSearchEventId === event.id && (
                          <div className={`mt-3 p-3 rounded ${getThemeClasses('bg-gray-100', 'bg-gray-900')}`} onClick={(e) => e.stopPropagation()}>
                            <div className="flex items-center justify-between mb-2">
                              <span className={`text-sm font-semibold ${getTextClass()}`}>Raw JSON</span>
                              <button
                                className={`${getThemeClasses('bg-gray-200 hover:bg-gray-300 text-black', 'bg-gray-800 hover:bg-gray-700 text-white')} text-xs px-2 py-1 rounded`}
                                onClick={() => copyEventJSON(event.raw_json)}
                              >
                                Copy JSON
                              </button>
                            </div>
                            <PrettyJSONView jsonString={event.raw_json} maxHeightClass="max-h-64" />
                          </div>
                        )}
                      </div>
                    ))}

                    {!searchLoading && searchHasMore && searchResults.length > 0 && (
                      <div className="text-center py-4">
                        <button
                          className={`${getThemeClasses('bg-blue-600 hover:bg-blue-700', 'bg-blue-500 hover:bg-blue-600')} text-white px-4 py-2 rounded`}
                          onClick={() => fetchSearchResultsFromRelay(searchQuery, false)}
                        >
                          Load More
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>

            {/* My Events Log */}
            <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
              <div
                className={`text-lg font-bold flex items-center justify-between cursor-pointer p-2 ${getTextClass()} ${getThemeClasses('hover:bg-gray-300', 'hover:bg-gray-700')} rounded`}
                onClick={() => toggleSection('eventsLog')}
              >
                <span>My Events Log</span>
                <span className="text-xl">
                  {expandedSections.eventsLog ? '▼' : '▶'}
                </span>
              </div>
              {expandedSections.eventsLog && (
                <div className="p-2 bg-gray-900 rounded-lg mt-2">
                  <div className="mb-4">
                    <p className={`text-sm ${getTextClass()}`}>View all your events in reverse chronological order. Click on any event to view its raw JSON.</p>
                  </div>

                  <div
                      className="block"
                      style={{
                        position: 'relative'
                      }}
                  >
                    {events.length === 0 && !eventsLoading ? (
                        <div className={`text-center py-4 ${getTextClass()}`}>No events found</div>
                    ) : (
                        <div className="space-y-2">
                          {events.map((event) => (
                              <div key={event.id} className={`border rounded p-3 ${getThemeClasses('border-gray-300 bg-white', 'border-gray-600 bg-gray-800')}`}>
                                <div
                                    className="cursor-pointer"
                                    onClick={() => toggleEventExpansion(event.id)}
                                >
                                  <div className="flex items-center justify-between w-full">
                                    <div className="flex items-center gap-6 w-full">
                                      {/* User avatar and info - separated with more space */}
                                      <div className="flex items-center gap-3 min-w-0">
                                        {user?.pubkey && profileCache[user.pubkey] && (
                                          <>
                                            {profileCache[user.pubkey].picture && (
                                              <img
                                                src={profileCache[user.pubkey].picture}
                                                alt={profileCache[user.pubkey].display_name || profileCache[user.pubkey].name || 'User avatar'}
                                                className={`w-8 h-8 rounded-full object-cover border h-16 ${getThemeClasses('border-gray-300', 'border-gray-600')}`}
                                                onError={(e) => {
                                                  e.currentTarget.style.display = 'none';
                                                }}
                                              />
                                            )}
                                            <div className="flex flex-col flex-grow w-full">
                                              <span className={`text-sm font-medium ${getTextClass()}`}>
                                                {profileCache[user.pubkey].display_name || profileCache[user.pubkey].name || `${user.pubkey.slice(0, 8)}...`}
                                              </span>
                                              {profileCache[user.pubkey].display_name && profileCache[user.pubkey].name && (
                                                <span className={`text-xs ${getTextClass()} opacity-70`}>
                                                  {profileCache[user.pubkey].name}
                                                </span>
                                              )}
                                            </div>
                                          </>
                                        )}
                                        {user?.pubkey && !profileCache[user.pubkey] && (
                                          <span className={`text-sm font-medium ${getTextClass()}`}>
                                            {`${user.pubkey.slice(0, 8)}...`}
                                          </span>
                                        )}
                                      </div>

                                      {/* Event metadata - separated to the right */}
                                      <div className="flex items-center gap-3">
                                        <span className={`font-mono text-sm px-2 py-1 rounded ${getThemeClasses('bg-blue-100 text-blue-800', 'bg-blue-900 text-blue-200')}`}>
                                          Kind {event.kind}
                                        </span>
                                        <span className={`text-sm ${getTextClass()}`}>
                                          {formatTimestamp(event.created_at)}
                                        </span>
                                      </div>
                                    </div>
                                    <div className="flex items-center gap-2 ml-auto">
                                       <div className={`text-lg rounded p-16 m-16 ${getThemeClasses('text-gray-700', 'text-gray-300')}`}>
                                          {expandedEventId === event.id ? '▼' : ' '}
                                        </div>
                                        <button
                                        className="bg-red-600 hover:bg-red-700 text-white text-xs px-1 py-1 rounded flex items-center"
                                        onClick={(e) => {
                                          e.stopPropagation();
                                          deleteEvent(event.id, event.raw_json);
                                        }}
                                        title="Delete this event"
                                      >
                                        🗑️
                                      </button>
                                    </div>
                                  </div>

                                  {event.content && (
                                      <div className={`mt-2 text-sm ${getTextClass()}`}>
                                        {truncateContent(event.content)}
                                      </div>
                                  )}
                                </div>

                                {expandedEventId === event.id && (
                                    <div className="mt-3 border-t pt-3">
                                      <div className="flex items-center justify-between mb-2">
                                        <span className={`text-sm font-medium ${getTextClass()}`}>Raw JSON:</span>
                                        <button
                                            className={`${getThemeClasses('bg-green-600 hover:bg-green-700', 'bg-green-500 hover:bg-green-600')} text-white text-xs px-2 py-1 rounded`}
                                            onClick={(e) => {
                                              e.stopPropagation();
                                              copyEventJSON(event.raw_json);
                                            }}
                                            title="Copy minified JSON"
                                        >
                                          Copy
                                        </button>
                                      </div>
                                      <PrettyJSONView jsonString={event.raw_json} maxHeightClass="max-h-40" />
                                    </div>
                                )}
                              </div>
                          ))}

                          {eventsLoading && (
                              <div className={`text-center py-4 ${getTextClass()}`}>
                                <div className="text-sm">Loading more events...</div>
                              </div>
                          )}

                          {!eventsLoading && eventsHasMore && (
                              <div className="text-center py-4">
                                <button
                                    className={`${getThemeClasses('bg-blue-600 hover:bg-blue-700', 'bg-blue-500 hover:bg-blue-600')} text-white px-4 py-2 rounded`}
                                    onClick={() => fetchEvents(false)}
                                >
                                  Load More
                                </button>
                              </div>
                          )}
                        </div>
                    )}
                  </div>
                </div>
              )}
            </div>

            {/* All Events Log (admin only) */}
            {user.permission === "admin" && (
              <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
                <div
                  className={`text-lg font-bold flex items-center justify-between cursor-pointer p-2 ${getTextClass()} ${getThemeClasses('hover:bg-gray-300', 'hover:bg-gray-700')} rounded`}
                  onClick={() => toggleSection('allEventsLog')}
                >
                  <span>All Events Log (admin)</span>
                  <span className="text-xl">
                    {expandedSections.allEventsLog ? '▼' : '▶'}
                  </span>
                </div>
                {expandedSections.allEventsLog && (
                  <div className="p-2 bg-gray-900 rounded-lg mt-2 w-full">
                    <div className="mb-4">
                      <p className={`text-sm ${getTextClass()}`}>View all events from all users in reverse chronological order. Click on any event to view its raw JSON.</p>
                    </div>

                    <div
                        className="block"
                        style={{
                          position: 'relative'
                        }}
                    >
                      {allEvents.length === 0 && !allEventsLoading ? (
                          <div className={`text-center py-4 ${getTextClass()}`}>No events found</div>
                      ) : (
                          <div className="space-y-2">
                            {allEvents.map((event) => (
                                <div key={event.id} className={`border rounded p-3 ${getThemeClasses('border-gray-300 bg-white', 'border-gray-600 bg-gray-800')}`}>
                                  <div
                                      className="cursor-pointer"
                                      onClick={() => toggleAllEventExpansion(event.id)}
                                  >
                                    <div className="flex items-center justify-between w-full">
                                      <div className="flex items-center gap-6 w-full">
                                        {/* User avatar and info - separated with more space */}
                                        <div className="flex items-center gap-3 min-w-0">
                                          {event.author && profileCache[event.author] && (
                                            <>
                                              {profileCache[event.author].picture && (
                                                <img
                                                  src={profileCache[event.author].picture}
                                                  alt={profileCache[event.author].display_name || profileCache[event.author].name || 'User avatar'}
                                                  className={`w-8 h-8 rounded-full object-cover border h-16 ${getThemeClasses('border-gray-300', 'border-gray-600')}`}
                                                  onError={(e) => {
                                                    e.currentTarget.style.display = 'none';
                                                  }}
                                                />
                                              )}
                                              <div className="flex flex-col flex-grow w-full">
                                                <span className={`text-sm font-medium ${getTextClass()}`}>
                                                  {profileCache[event.author].display_name || profileCache[event.author].name || `${event.author.slice(0, 8)}...`}
                                                </span>
                                                {profileCache[event.author].display_name && profileCache[event.author].name && (
                                                  <span className={`text-xs ${getTextClass()} opacity-70`}>
                                                    {profileCache[event.author].name}
                                                  </span>
                                                )}
                                              </div>
                                            </>
                                          )}
                                          {event.author && !profileCache[event.author] && (
                                            <span className={`text-sm font-medium ${getTextClass()}`}>
                                              {`${event.author.slice(0, 8)}...`}
                                            </span>
                                          )}
                                        </div>

                                        {/* Event metadata - separated to the right */}
                                        <div className="flex items-center gap-3">
                                          <span className={`font-mono text-sm px-2 py-1 rounded ${getThemeClasses('bg-blue-100 text-blue-800', 'bg-blue-900 text-blue-200')}`}>
                                            Kind {event.kind}
                                          </span>
                                          <span className={`text-sm ${getTextClass()}`}>
                                            {formatTimestamp(event.created_at)}
                                          </span>
                                        </div>
                                      </div>
                                      <div className="justify-end ml-auto rounded-full h-16 w-16 flex items-center justify-center">
                                         <div className={`text-white text-xs px-4 py-4 rounded flex flex-grow items-center ${getThemeClasses('text-gray-700', 'text-gray-300')}`}>
                                            {expandedAllEventId === event.id ? '▼' : ' '}
                                          </div>
                                          <button
                                          className="bg-red-600 hover:bg-red-700 text-white text-xs px-1 py-1 rounded flex items-center"
                                          onClick={(e) => {
                                            e.stopPropagation();
                                            deleteEvent(event.id, event.raw_json, event.author);
                                          }}
                                          title="Delete this event"
                                        >
                                          🗑️
                                        </button>
                                      </div>
                                    </div>

                                    {event.content && (
                                        <div className={`mt-2 text-sm ${getTextClass()}`}>
                                          {truncateContent(event.content)}
                                        </div>
                                    )}
                                  </div>

                                  {expandedAllEventId === event.id && (
                                      <div className="mt-3 border-t pt-3">
                                        <div className="flex items-center justify-between mb-2">
                                          <span className={`text-sm font-medium ${getTextClass()}`}>Raw JSON:</span>
                                          <button
                                              className={`${getThemeClasses('bg-green-600 hover:bg-green-700', 'bg-green-500 hover:bg-green-600')} text-white text-xs px-2 py-1 rounded`}
                                              onClick={(e) => {
                                                e.stopPropagation();
                                                copyEventJSON(event.raw_json);
                                              }}
                                              title="Copy minified JSON"
                                          >
                                            Copy
                                          </button>
                                        </div>
                                        <PrettyJSONView jsonString={event.raw_json} maxHeightClass="max-h-40" />
                                      </div>
                                  )}
                                </div>
                            ))}

                            {allEventsLoading && (
                                <div className={`text-center py-4 ${getTextClass()}`}>
                                  <div className="text-sm">Loading more events...</div>
                                </div>
                            )}

                            {!allEventsLoading && allEventsHasMore && (
                                <div className="text-center py-4">
                                  <button
                                      className={`${getThemeClasses('bg-blue-600 hover:bg-blue-700', 'bg-blue-500 hover:bg-blue-600')} text-white px-4 py-2 rounded`}
                                      onClick={() => fetchAllEvents(false)}
                                  >
                                    Load More
                                  </button>
                                </div>
                            )}
                          </div>
                      )}
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Empty flex grow box to ensure background fills entire viewport */}
            <div className={`flex-grow ${getThemeClasses('bg-gray-100', 'bg-gray-900')}`}></div>
          </div>
        </>
      ) : (
        // Not logged in view - shows the login form
        <div className="w-full h-full flex items-center justify-center">
          <div
            className={getThemeClasses('bg-gray-100', 'bg-gray-900')}
            style={{ width: '800px', maxWidth: '100%', boxSizing: 'border-box', padding: `${loginPadding}px` }}
          >
            <div className="flex items-center gap-3 mb-3">
              <img
                src="/orly.png"
                alt="Orly logo"
                className="object-contain"
                style={{ width: '4rem', height: '4rem' }}
                onError={(e) => {
                  // fallback to repo docs image if public asset missing
                  e.currentTarget.onerror = null;
                  e.currentTarget.src = "/docs/orly.png";
                }}
              />
              <h1 ref={titleRef} className={`text-2xl font-bold p-2 ${getTextClass()}`}>ORLY🦉 Dashboard Login</h1>
            </div>

            <p className={`mb-4 ${getTextClass()}`}>Authenticate to this Nostr relay using your browser extension.</p>

            <div className={statusClassName()}>
              {status}
            </div>

            <div className="mb-5">
              <button className={`${getThemeClasses('bg-blue-600', 'bg-blue-500')} text-white px-5 py-3 rounded ${getThemeClasses('hover:bg-blue-700', 'hover:bg-blue-600')}`} onClick={loginWithExtension}>Login with Browser Extension (NIP-07)</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;