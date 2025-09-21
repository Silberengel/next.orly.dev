import React, { useState, useEffect, useRef } from 'react';

function App() {
  const [user, setUser] = useState(null);
  const [status, setStatus] = useState('Ready to authenticate');
  const [statusType, setStatusType] = useState('info');
  const [profileData, setProfileData] = useState(null);
  
  // Theme state for dark/light mode
  const [isDarkMode, setIsDarkMode] = useState(false);

  const [checkingAuth, setCheckingAuth] = useState(true);

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
                {profileData?.picture && <img src={profileData.picture} alt="User Avatar" className={`h-full aspect-square w-auto rounded-full object-cover border-2 ${getThemeClasses('border-white', 'border-gray-600')} mr-2 shadow box-border`} />}
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
              <div className={`text-lg font-bold flex items-center ${getTextClass()}`}>Welcome</div>
              <p className={getTextClass()}>here you can configure all the things</p>
            </div>

            {/* Export only my events */}
            <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
              <div className="w-full flex items-center justify-end p-2 bg-gray-900 rounded-lg">
                <div className="pr-2 m-2 w-full">
                  <div className={`text-base font-bold mb-1 ${getTextClass()}`}>Export My Events</div>
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
            </div>

            {user.permission === "admin" && (
              <>
                <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
                  <div className="flex items-center justify-between p-2 m-4 bg-gray-900 round">
                    <div className="pr-2 w-full">
                      <div className={`text-base font-bold mb-1 ${getTextClass()}`}>Export All Events (admin)</div>
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
                </div>

                {/* Export specific pubkeys (admin) */}
                <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
                  <div className="w-full flex items-start justify-between gap-4 m-2 p-2 bg-gray-900 rounded-lg">
                    {/* Left: title and help text */}
                    <div className="flex-1 pr-2 w-full">
                      <div className={`text-base font-bold mb-1 ${getTextClass()}`}>Export Specific Pubkeys (admin)</div>
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
                </div>
                <div className={`m-2 p-2 ${getPanelBgClass()} rounded-lg w-full`}>
                  <div className="flex items-center justify-between p-2 bg-gray-900 rounded-lg">
                    <div className="pr-2 w-full">
                      <div className={`text-base font-bold mb-1 ${getTextClass()}`}>Import Events (admin)</div>
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
                </div>
              </>
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
      <div className={`flex-grow ${getThemeClasses('bg-gray-100', 'bg-gray-900')}`}></div>
    </div>
  );
}

export default App;