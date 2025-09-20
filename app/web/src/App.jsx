import React, { useState, useEffect } from 'react';

function App() {
  const [user, setUser] = useState(null);
  const [status, setStatus] = useState('Ready to authenticate');
  const [statusType, setStatusType] = useState('info');

  useEffect(() => {
    // Check authentication status on page load
    checkStatus();
  }, []);

  async function checkStatus() {
    try {
      const response = await fetch('/api/auth/status');
      const data = await response.json();
      if (data.authenticated) {
        setUser(data.pubkey);
        updateStatus(`Already authenticated as: ${data.pubkey.slice(0, 16)}...`, 'success');
        
        // Check permissions if authenticated
        if (data.pubkey) {
          const permResponse = await fetch(`/api/permissions/${data.pubkey}`);
          const permData = await permResponse.json();
          if (permData && permData.permission) {
            setUser({...data, permission: permData.permission});
          }
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
          ['relay', window.location.protocol.replace('http', 'ws') + '//' + window.location.host],
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
        }
      } else {
        updateStatus('Authentication failed: ' + result.error, 'error');
      }
    } catch (error) {
      updateStatus('Authentication request failed: ' + error.message, 'error');
    }
  }

  function logout() {
    setUser(null);
    updateStatus('Logged out', 'info');
  }

  return (
    <div className="container">
      {user?.permission && (
        <div className="header-panel">
          <div className="header-content">
            <img src="/docs/orly.png" alt="Logo" className="header-logo" />
            <div className="user-info">
              {user.permission === "admin" ? "Admin Dashboard" : "Subscriber Dashboard"}
            </div>
            <button className="logout-button" onClick={logout}>✕</button>
          </div>
        </div>
      )}

      <h1>Nostr Relay Authentication</h1>
      <p>Connect to this Nostr relay using your private key or browser extension.</p>
      
      <div className={`status ${statusType}`}>
        {status}
      </div>
      
      <div className="form-group">
        <button onClick={loginWithExtension}>Login with Browser Extension (NIP-07)</button>
      </div>
      
      <div className="form-group">
        <label htmlFor="nsec">Or login with private key (nsec):</label>
        <input type="password" id="nsec" placeholder="nsec1..." />
        <button onClick={() => updateStatus('Private key login not implemented in this basic interface. Please use a proper Nostr client or extension.', 'error')} style={{marginTop: '10px'}}>Login with Private Key</button>
      </div>
      
      <div className="form-group">
        <button onClick={logout} className="danger-button">Logout</button>
      </div>
    </div>
  );
}

export default App;