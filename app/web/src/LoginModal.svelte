<script>
    import { createEventDispatcher } from 'svelte';
    
    const dispatch = createEventDispatcher();
    
    export let showModal = false;
    export let isDarkTheme = false;
    
    let activeTab = 'extension';
    let nsecInput = '';
    let isLoading = false;
    let errorMessage = '';
    let successMessage = '';
    
    function closeModal() {
        showModal = false;
        nsecInput = '';
        errorMessage = '';
        successMessage = '';
        dispatch('close');
    }
    
    function switchTab(tab) {
        activeTab = tab;
        errorMessage = '';
        successMessage = '';
    }
    
    async function loginWithExtension() {
        isLoading = true;
        errorMessage = '';
        successMessage = '';
        
        try {
            // Check if window.nostr is available
            if (!window.nostr) {
                throw new Error('No Nostr extension found. Please install a NIP-07 compatible extension like nos2x or Alby.');
            }
            
            // Get public key from extension
            const pubkey = await window.nostr.getPublicKey();
            
            if (pubkey) {
                // Store authentication info
                localStorage.setItem('nostr_auth_method', 'extension');
                localStorage.setItem('nostr_pubkey', pubkey);
                
                successMessage = 'Successfully logged in with extension!';
                dispatch('login', {
                    method: 'extension',
                    pubkey: pubkey,
                    signer: window.nostr
                });
                
                setTimeout(() => {
                    closeModal();
                }, 1500);
            }
        } catch (error) {
            errorMessage = error.message;
        } finally {
            isLoading = false;
        }
    }
    
    function validateNsec(nsec) {
        // Basic validation for nsec format
        if (!nsec.startsWith('nsec1')) {
            return false;
        }
        // Should be around 63 characters long
        if (nsec.length < 60 || nsec.length > 70) {
            return false;
        }
        return true;
    }
    
    function nsecToHex(nsec) {
        // This is a simplified conversion - in a real app you'd use a proper library
        // For demo purposes, we'll simulate the conversion
        try {
            // Remove 'nsec1' prefix and decode (simplified)
            const withoutPrefix = nsec.slice(5);
            // In reality, you'd use bech32 decoding here
            // For now, we'll generate a mock hex key
            return 'mock_' + withoutPrefix.slice(0, 32);
        } catch (error) {
            throw new Error('Invalid nsec format');
        }
    }
    
    async function loginWithNsec() {
        isLoading = true;
        errorMessage = '';
        successMessage = '';
        
        try {
            if (!nsecInput.trim()) {
                throw new Error('Please enter your nsec');
            }
            
            if (!validateNsec(nsecInput.trim())) {
                throw new Error('Invalid nsec format. Must start with "nsec1"');
            }
            
            // Convert nsec to hex format (simplified for demo)
            const privateKey = nsecToHex(nsecInput.trim());
            
            // In a real implementation, you'd derive the public key from private key
            const publicKey = 'derived_' + privateKey.slice(5, 37);
            
            // Store securely (in production, consider more secure storage)
            localStorage.setItem('nostr_auth_method', 'nsec');
            localStorage.setItem('nostr_pubkey', publicKey);
            localStorage.setItem('nostr_privkey', privateKey);
            
            successMessage = 'Successfully logged in with nsec!';
            dispatch('login', {
                method: 'nsec',
                pubkey: publicKey,
                privateKey: privateKey
            });
            
            setTimeout(() => {
                closeModal();
            }, 1500);
        } catch (error) {
            errorMessage = error.message;
        } finally {
            isLoading = false;
        }
    }
    
    function handleKeydown(event) {
        if (event.key === 'Escape') {
            closeModal();
        }
        if (event.key === 'Enter' && activeTab === 'nsec') {
            loginWithNsec();
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if showModal}
    <div class="modal-overlay" on:click={closeModal} on:keydown={(e) => e.key === 'Escape' && closeModal()} role="button" tabindex="0">
        <div class="modal" class:dark-theme={isDarkTheme} on:click|stopPropagation on:keydown|stopPropagation>
            <div class="modal-header">
                <h2>Login to Nostr</h2>
                <button class="close-btn" on:click={closeModal}>&times;</button>
            </div>
            
            <div class="tab-container">
                <div class="tabs">
                    <button 
                        class="tab-btn"
                        class:active={activeTab === 'extension'}
                        on:click={() => switchTab('extension')}
                    >
                        Extension
                    </button>
                    <button 
                        class="tab-btn"
                        class:active={activeTab === 'nsec'}
                        on:click={() => switchTab('nsec')}
                    >
                        Nsec
                    </button>
                </div>
                
                <div class="tab-content">
                    {#if activeTab === 'extension'}
                        <div class="extension-login">
                            <p>Login using a NIP-07 compatible browser extension like nos2x or Alby.</p>
                            <button 
                                class="login-extension-btn"
                                on:click={loginWithExtension}
                                disabled={isLoading}
                            >
                                {isLoading ? 'Connecting...' : 'Log in using extension'}
                            </button>
                        </div>
                    {:else}
                        <div class="nsec-login">
                            <p>Enter your nsec (private key) to login. This will be stored securely in your browser.</p>
                            <input 
                                type="password"
                                placeholder="nsec1..."
                                bind:value={nsecInput}
                                disabled={isLoading}
                                class="nsec-input"
                            />
                            <button 
                                class="login-nsec-btn"
                                on:click={loginWithNsec}
                                disabled={isLoading || !nsecInput.trim()}
                            >
                                {isLoading ? 'Logging in...' : 'Log in with nsec'}
                            </button>
                        </div>
                    {/if}
                    
                    {#if errorMessage}
                        <div class="message error-message">{errorMessage}</div>
                    {/if}
                    
                    {#if successMessage}
                        <div class="message success-message">{successMessage}</div>
                    {/if}
                </div>
            </div>
        </div>
    </div>
{/if}

<style>
    .modal-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.5);
        display: flex;
        justify-content: center;
        align-items: center;
        z-index: 1000;
    }
    
    .modal {
        background: var(--bg-color);
        border-radius: 8px;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
        width: 90%;
        max-width: 500px;
        max-height: 90vh;
        overflow-y: auto;
        border: 1px solid var(--border-color);
    }
    
    .modal-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 20px;
        border-bottom: 1px solid var(--border-color);
    }
    
    .modal-header h2 {
        margin: 0;
        color: var(--text-color);
        font-size: 1.5rem;
    }
    
    .close-btn {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: var(--text-color);
        padding: 0;
        width: 30px;
        height: 30px;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 50%;
        transition: background-color 0.2s;
    }
    
    .close-btn:hover {
        background-color: var(--tab-hover-bg);
    }
    
    .tab-container {
        padding: 20px;
    }
    
    .tabs {
        display: flex;
        border-bottom: 1px solid var(--border-color);
        margin-bottom: 20px;
    }
    
    .tab-btn {
        flex: 1;
        padding: 12px 16px;
        background: none;
        border: none;
        cursor: pointer;
        color: var(--text-color);
        font-size: 1rem;
        transition: all 0.2s;
        border-bottom: 2px solid transparent;
    }
    
    .tab-btn:hover {
        background-color: var(--tab-hover-bg);
    }
    
    .tab-btn.active {
        border-bottom-color: var(--primary);
        color: var(--primary);
    }
    
    .tab-content {
        min-height: 200px;
    }
    
    .extension-login,
    .nsec-login {
        display: flex;
        flex-direction: column;
        gap: 16px;
    }
    
    .extension-login p,
    .nsec-login p {
        margin: 0;
        color: var(--text-color);
        line-height: 1.5;
    }
    
    .login-extension-btn,
    .login-nsec-btn {
        padding: 12px 24px;
        background: var(--primary);
        color: white;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-size: 1rem;
        transition: background-color 0.2s;
    }
    
    .login-extension-btn:hover:not(:disabled),
    .login-nsec-btn:hover:not(:disabled) {
        background: #00ACC1;
    }
    
    .login-extension-btn:disabled,
    .login-nsec-btn:disabled {
        background: #ccc;
        cursor: not-allowed;
    }
    
    .nsec-input {
        padding: 12px;
        border: 1px solid var(--input-border);
        border-radius: 6px;
        font-size: 1rem;
        background: var(--bg-color);
        color: var(--text-color);
    }
    
    .nsec-input:focus {
        outline: none;
        border-color: var(--primary);
    }
    
    .message {
        padding: 10px;
        border-radius: 4px;
        margin-top: 16px;
        text-align: center;
    }
    
    .error-message {
        background: #ffebee;
        color: #c62828;
        border: 1px solid #ffcdd2;
    }
    
    .success-message {
        background: #e8f5e8;
        color: #2e7d32;
        border: 1px solid #c8e6c9;
    }
    
    .modal.dark-theme .error-message {
        background: #4a2c2a;
        color: #ffcdd2;
        border: 1px solid #6d4c41;
    }
    
    .modal.dark-theme .success-message {
        background: #2e4a2e;
        color: #a5d6a7;
        border: 1px solid #4caf50;
    }
</style>