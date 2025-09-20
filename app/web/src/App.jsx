import React, { useState, useEffect } from 'react';

function App() {
  const [user, setUser] = useState(null);
  const [status, setStatus] = useState('Ready to authenticate');
  const [statusType, setStatusType] = useState('info');
  const [profileData, setProfileData] = useState(null);

  const [checkingAuth, setCheckingAuth] = useState(true);

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
    switch (statusType) {
      case 'success':
        return base + ' bg-green-100 text-green-800';
      case 'error':
        return base + ' bg-red-100 text-red-800';
      case 'info':
      default:
        return base + ' bg-cyan-100 text-cyan-800';
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

      ws.onopen = () => {
        try {
          const req = [
            'REQ',
            subId,
            { kinds: [0], authors: [pubkeyHex] }
          ];
          ws.send(JSON.stringify(req));
        } catch (_) {}
      };

      ws.onmessage = (msg) => {
        try {
          const data = JSON.parse(msg.data);
          const type = data[0];
          if (type === 'EVENT' && data[1] === subId) {
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
    updateStatus('Logged out', 'info');
  }

  // Prevent UI flash: wait until we checked auth status
  if (checkingAuth) {
    return null;
  }

  return (
    <>
      {user?.permission ? (
        // Logged in view with user profile
        <div className="sticky top-0 left-0 w-full bg-gray-100 z-50 h-16 flex items-center overflow-hidden">
          <div className="flex items-center h-full w-full box-border">
            <div className="relative overflow-hidden flex flex-grow items-center justify-start h-full">
              {profileData?.banner && (
                <div className="absolute inset-0 opacity-70 bg-cover bg-center" style={{ backgroundImage: `url(${profileData.banner})` }}></div>
              )}
              <div className="relative z-10 p-2 flex items-center h-full">
                {profileData?.picture && <img src={profileData.picture} alt="User Avatar" className="h-full aspect-square w-auto rounded-full object-cover border-2 border-white mr-2 shadow box-border" />}
                <div>
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
              <button className="bg-transparent text-gray-500 border-0 text-2xl cursor-pointer flex items-center justify-center h-full aspect-square shrink-0 hover:bg-transparent hover:text-gray-800" onClick={logout}>✕</button>
            </div>
          </div>
        </div>
      ) : (
        // Not logged in view - shows the login form
        <div className="max-w-3xl mx-auto mt-5 p-6 bg-gray-100 rounded">
          <h1 className="text-2xl font-bold mb-2">Nostr Relay Authentication</h1>
          <p className="mb-4">Connect to this Nostr relay using your private key or browser extension.</p>

          <div className={statusClassName()}>
            {status}
          </div>

          <div className="mb-5">
            <button className="bg-blue-600 text-white px-5 py-3 rounded hover:bg-blue-700" onClick={loginWithExtension}>Login with Browser Extension (NIP-07)</button>
          </div>

          <div className="mb-5">
            <label className="block mb-1 font-bold" htmlFor="nsec">Or login with private key (nsec):</label>
            <input className="w-full p-2 border border-gray-300 rounded" type="password" id="nsec" placeholder="nsec1..." />
            <button className="mt-2 bg-red-600 text-white px-5 py-2 rounded hover:bg-red-700" onClick={() => updateStatus('Private key login not implemented in this basic interface. Please use a proper Nostr client or extension.', 'error')}>Login with Private Key</button>
          </div>
        </div>
      )}
    </>
  );
}

export default App;