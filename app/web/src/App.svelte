<script>
    import LoginModal from './LoginModal.svelte';
    import { initializeNostrClient, fetchUserProfile } from './nostr.js';
    
    export let name;

    let isDarkTheme = false;
    let showLoginModal = false;
    let isLoggedIn = false;
    let userPubkey = '';
    let authMethod = '';
    let userProfile = null;
    let showSettingsDrawer = false;
    let selectedTab = 'export';
    let isSearchMode = false;
    let searchQuery = '';
    let searchTabs = [];

    // Safely render "about" text: convert double newlines to a single HTML line break
    function escapeHtml(str) {
        return String(str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }

    $: aboutHtml = userProfile?.about
        ? escapeHtml(userProfile.about).replace(/\n{2,}/g, '<br>')
        : '';

    // Load theme preference from localStorage on component initialization
    if (typeof localStorage !== 'undefined') {
        const savedTheme = localStorage.getItem('isDarkTheme');
        if (savedTheme !== null) {
            isDarkTheme = JSON.parse(savedTheme);
        }
        
        // Check for existing authentication
        const storedAuthMethod = localStorage.getItem('nostr_auth_method');
        const storedPubkey = localStorage.getItem('nostr_pubkey');
        
        if (storedAuthMethod && storedPubkey) {
            isLoggedIn = true;
            userPubkey = storedPubkey;
            authMethod = storedAuthMethod;
        }
    }

    const baseTabs = [
        {id: 'export', icon: '📤', label: 'Export'},
        {id: 'import', icon: '💾', label: 'Import'},
        {id: 'myevents', icon: '👤', label: 'My Events'},
        {id: 'allevents', icon: '📡', label: 'All Events'},
        {id: 'sprocket', icon: '⚙️', label: 'Sprocket'},
    ];

    $: tabs = [...baseTabs, ...searchTabs];

    function selectTab(tabId) {
        selectedTab = tabId;
    }

    function toggleTheme() {
        isDarkTheme = !isDarkTheme;
        // Save theme preference to localStorage
        if (typeof localStorage !== 'undefined') {
            localStorage.setItem('isDarkTheme', JSON.stringify(isDarkTheme));
        }
    }

    
    function openLoginModal() {
        if (!isLoggedIn) {
            showLoginModal = true;
        }
    }
    
    async function handleLogin(event) {
        const { method, pubkey, privateKey, signer } = event.detail;
        isLoggedIn = true;
        userPubkey = pubkey;
        authMethod = method;
        showLoginModal = false;
        
        // Initialize Nostr client and fetch profile
        try {
            await initializeNostrClient();
            userProfile = await fetchUserProfile(pubkey);
            console.log('Profile loaded:', userProfile);
        } catch (error) {
            console.error('Failed to load profile:', error);
        }
    }
    
    function handleLogout() {
        isLoggedIn = false;
        userPubkey = '';
        authMethod = '';
        userProfile = null;
        showSettingsDrawer = false;
        
        // Clear stored authentication
        if (typeof localStorage !== 'undefined') {
            localStorage.removeItem('nostr_auth_method');
            localStorage.removeItem('nostr_pubkey');
            localStorage.removeItem('nostr_privkey');
        }
    }
    
    function closeLoginModal() {
        showLoginModal = false;
    }
    
    function openSettingsDrawer() {
        showSettingsDrawer = true;
    }
    
    function closeSettingsDrawer() {
        showSettingsDrawer = false;
    }

    function toggleSearchMode() {
        isSearchMode = !isSearchMode;
        if (!isSearchMode) {
            searchQuery = '';
        }
    }

    function handleSearchKeydown(event) {
        if (event.key === 'Enter' && searchQuery.trim()) {
            createSearchTab(searchQuery.trim());
            searchQuery = '';
            isSearchMode = false;
        } else if (event.key === 'Escape') {
            isSearchMode = false;
            searchQuery = '';
        }
    }

    function createSearchTab(query) {
        const searchTabId = `search-${Date.now()}`;
        const newSearchTab = {
            id: searchTabId,
            icon: '❌',
            label: query,
            isSearchTab: true,
            query: query
        };
        searchTabs = [...searchTabs, newSearchTab];
        selectedTab = searchTabId;
    }

    function closeSearchTab(tabId) {
        searchTabs = searchTabs.filter(tab => tab.id !== tabId);
        if (selectedTab === tabId) {
            selectedTab = 'export'; // Fall back to export tab
        }
    }


    $: if (typeof document !== 'undefined') {
        if (isDarkTheme) {
            document.body.classList.add('dark-theme');
        } else {
            document.body.classList.remove('dark-theme');
        }
    }

    // Auto-fetch profile when user is logged in but profile is missing
    $: if (isLoggedIn && userPubkey && !userProfile) {
        fetchProfileIfMissing();
    }

    async function fetchProfileIfMissing() {
        if (!isLoggedIn || !userPubkey || userProfile) {
            return; // Don't fetch if not logged in, no pubkey, or profile already exists
        }
        
        try {
            console.log('Auto-fetching profile for:', userPubkey);
            await initializeNostrClient();
            userProfile = await fetchUserProfile(userPubkey);
            console.log('Profile auto-loaded:', userProfile);
        } catch (error) {
            console.error('Failed to auto-load profile:', error);
        }
    }
</script>

<!-- Header -->
<header class="main-header" class:dark-theme={isDarkTheme}>
    <div class="header-content">
        <img src="/orly.png" alt="Orly Logo" class="logo"/>
        {#if isSearchMode}
            <div class="search-input-container">
                <input 
                    type="text" 
                    class="search-input"
                    bind:value={searchQuery}
                    on:keydown={handleSearchKeydown}
                    placeholder="Search..."
                    autofocus
                />
            </div>
        {:else}
            <div class="header-title">
                <span class="app-title">ORLY?</span>
            </div>
        {/if}
        <button class="search-btn" on:click={toggleSearchMode}>
            🔍
        </button>
        <button class="theme-toggle-btn" on:click={toggleTheme}>
            {isDarkTheme ? '☀️' : '🌙'}
        </button>
        {#if isLoggedIn}
            <div class="user-info">
                <button class="user-profile-btn" on:click={openSettingsDrawer}>
                    {#if userProfile?.picture}
                        <img src={userProfile.picture} alt="User avatar" class="user-avatar" />
                    {:else}
                        <div class="user-avatar-placeholder">👤</div>
                    {/if}
                    <span class="user-name">
                        {userProfile?.name || userPubkey.slice(0, 8) + '...'}
                    </span>
                </button>
                <button class="logout-btn" on:click={handleLogout}>🚪</button>
            </div>
        {:else}
            <button class="login-btn" on:click={openLoginModal}>📥</button>
        {/if}
    </div>
</header>

<!-- Main Content Area -->
<div class="app-container" class:dark-theme={isDarkTheme}>
    <!-- Sidebar -->
    <aside class="sidebar" class:dark-theme={isDarkTheme}>
        <div class="sidebar-content">
            <div class="tabs">
                {#each tabs as tab}
                    <button class="tab" class:active={selectedTab === tab.id}
                         on:click={() => selectTab(tab.id)}>
                        {#if tab.isSearchTab}
                            <span class="tab-icon close-icon" on:click|stopPropagation={() => closeSearchTab(tab.id)} on:keydown={(e) => e.key === 'Enter' && closeSearchTab(tab.id)} role="button" tabindex="0">{tab.icon}</span>
                        {:else}
                            <span class="tab-icon">{tab.icon}</span>
                        {/if}
                        <span class="tab-label">{tab.label}</span>
                    </button>
                {/each}
            </div>
        </div>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
        <h1>Hello {name}!</h1>
    </main>
</div>

<!-- Settings Drawer -->
{#if showSettingsDrawer}
    <div class="drawer-overlay" on:click={closeSettingsDrawer} on:keydown={(e) => e.key === 'Escape' && closeSettingsDrawer()} role="button" tabindex="0">
        <div class="settings-drawer" class:dark-theme={isDarkTheme} on:click|stopPropagation on:keydown|stopPropagation>
            <div class="drawer-header">
                <h2>Settings</h2>
                <button class="close-btn" on:click={closeSettingsDrawer}>✕</button>
            </div>
            <div class="drawer-content">
                {#if userProfile}
                    <div class="profile-section">
                        <div class="profile-hero">
                            {#if userProfile.banner}
                                <img src={userProfile.banner} alt="Profile banner" class="profile-banner" />
                            {/if}
                            <!-- Avatar overlaps the bottom edge of the banner by 50% -->
                            {#if userProfile.picture}
                                <img src={userProfile.picture} alt="User avatar" class="profile-avatar overlap" />
                            {:else}
                                <div class="profile-avatar-placeholder overlap">👤</div>
                            {/if}
                            <!-- Username and nip05 to the right of the avatar, above the bottom edge -->
                            <div class="name-row">
                                <h3 class="profile-username">{userProfile.name || 'Unknown User'}</h3>
                                {#if userProfile.nip05}
                                    <span class="profile-nip05-inline">{userProfile.nip05}</span>
                                {/if}
                            </div>
                        </div>

                        <!-- About text in a box underneath, with avatar overlapping its top edge -->
                        {#if userProfile.about}
                            <div class="about-card">
                                <p class="profile-about">{@html aboutHtml}</p>
                            </div>
                        {/if}
                    </div>
                {:else if isLoggedIn && userPubkey}
                    <div class="profile-loading-section">
                        <h3>Profile Loading</h3>
                        <p>Your profile metadata is being loaded...</p>
                        <button class="retry-profile-btn" on:click={fetchProfileIfMissing}>
                            Retry Loading Profile
                        </button>
                        <div class="user-pubkey-display">
                            <strong>Public Key:</strong> {userPubkey.slice(0, 16)}...{userPubkey.slice(-8)}
                        </div>
                    </div>
                {/if}
                <!-- Additional settings can be added here -->
            </div>
        </div>
    </div>
{/if}

<!-- Login Modal -->
<LoginModal 
    bind:showModal={showLoginModal}
    {isDarkTheme}
    on:login={handleLogin}
    on:close={closeLoginModal}
/>

<style>
    :global(body) {
        margin: 0;
        padding: 0;
        --bg-color: #ddd;
        --header-bg: #eee;
        --border-color: #dee2e6;
        --text-color: #444444;
        --input-border: #ccc;
        --button-bg: #ddd;
        --button-hover-bg: #eee;
        --primary: #00BCD4;
        --warning: #ff3e00;
        --tab-inactive-bg: #bbb;
    }

    :global(body.dark-theme) {
        --bg-color: #263238;
        --header-bg: #1e272c;
        --border-color: #404040;
        --text-color: #ffffff;
        --input-border: #555;
        --button-bg: #263238;
        --button-hover-bg: #1e272c;
        --primary: #00BCD4;
        --warning: #ff3e00;
        --tab-inactive-bg: #1a1a1a;
    }

    /* Header Styles */
    .main-header {
        height: 3em;
        background-color: var(--header-bg);
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        z-index: 1000;
        color: var(--text-color);
    }

    .header-content {
        height: 100%;
        display: flex;
        align-items: center;
        padding: 0;
        gap: 0;
    }

    .logo {
        height: 2.5em;
        width: 2.5em;
        object-fit: contain;
        flex-shrink: 0;
    }

    .header-title {
        flex: 1;
        height: 100%;
        display: flex;
        align-items: center;
        gap: 0;
        padding: 0 1rem;
    }

    .app-title {
        font-size: 1em;
        font-weight: 600;
        color: var(--text-color);
    }

    .search-input-container {
        flex: 1;
        height: 100%;
        display: flex;
        align-items: center;
        padding: 0 1rem;
    }

    .search-input {
        width: 100%;
        height: 2em;
        padding: 0.5rem;
        border: 1px solid var(--input-border);
        border-radius: 4px;
        background: var(--bg-color);
        color: var(--text-color);
        font-size: 1em;
        outline: none;
    }

    .search-input:focus {
        border-color: var(--primary);
    }

    .search-btn {
        border: 0 none;
        border-radius: 0;
        display: flex;
        align-items: center;
        background-color: var(--button-hover-bg);
        cursor: pointer;
        color: var(--text-color);
        height: 3em;
        width: auto;
        min-width: 3em;
        flex-shrink: 0;
        line-height: 1;
        transition: background-color 0.2s;
        justify-content: center;
        padding: 1em 1em 1em 1em;
        margin: 0;
    }

    .search-btn:hover {
        background-color: var(--button-bg);
    }

    .theme-toggle-btn {
        border: 0 none;
        border-radius: 0;
        display: flex;
        align-items: center;
        background-color: var(--button-hover-bg);
        cursor: pointer;
        color: var(--text-color);
        height: 3em;
        width: auto;
        min-width: 3em;
        flex-shrink: 0;
        line-height: 1;
        transition: background-color 0.2s;
        justify-content: center;
        padding: 1em 1em 1em 1em;
        margin: 0;
    }

    .theme-toggle-btn:hover {
        background-color: var(--button-bg);
    }

    .login-btn {
        border: 0 none;
        border-radius: 0;
        display: flex;
        align-items: center;
        background-color: var(--primary);
        cursor: pointer;
        color: var(--text-color);
        height: 3em;
        width: auto;
        min-width: 3em;
        flex-shrink: 0;
        line-height: 1;
        transition: background-color 0.2s;
        justify-content: center;
        padding: 1em 1em 1em 1em;
        margin: 0;
    }

    .login-btn:hover {
        background-color: #0056b3;
    }

    /* App Container */
    .app-container {
        display: flex;
        margin-top: 3em;
        height: calc(100vh - 3em);
    }

    /* Sidebar Styles */
    .sidebar {
        position: fixed;
        left: 0;
        top: 3em;
        bottom: 0;
        width: 200px;
        background-color: var(--header-bg);
        color: var(--text-color);
        z-index: 100;
    }

    .sidebar-content {
        height: 100%;
        display: flex;
        flex-direction: column;
        padding: 0;
    }

    .tabs {
        display: flex;
        flex-direction: column;
    }

    .tab {
        height: 3em;
        display: flex;
        align-items: center;
        padding: 0 1rem;
        cursor: pointer;
        border: none;
        background: transparent;
        color: var(--text-color);
        transition: background-color 0.2s ease;
        gap: 0.75rem;
        text-align: left;
        width: 100%;
    }

    .tab:hover {
        background-color: var(--bg-color);
    }

    .tab.active {
        background-color: var(--bg-color);
    }

    .tab-icon {
        font-size: 1.2em;
        flex-shrink: 0;
        width: 1.5em;
        text-align: center;
    }

    .tab-label {
        font-size: 0.9em;
        font-weight: 500;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        flex: 1;
    }

    .close-icon {
        cursor: pointer;
        transition: opacity 0.2s;
    }

    .close-icon:hover {
        opacity: 0.7;
    }

    /* Main Content */
    .main-content {
        position: fixed;
        left: 200px;
        top: 3em;
        right: 0;
        bottom: 0;
        padding: 2rem;
        overflow-y: auto;
        background-color: var(--bg-color);
        color: var(--text-color);
    }

    .main-content h1 {
        color: #ff3e00;
        text-transform: uppercase;
        font-size: 4em;
        font-weight: 100;
        text-align: center;
    }

    @media (max-width: 640px) {
        .header-content {
            padding: 0;
        }

        .sidebar {
            width: 160px;
        }

        .main-content {
            left: 160px;
            padding: 1rem;
        }

        .main-content h1 {
            font-size: 2.5em;
        }
    }
    
    /* User Info Styles */
    .user-info {
        display: flex;
        align-items: flex-start;
        padding: 0;
        height:3em;
    }
    
    
    .logout-btn {
        padding: 0;
        border: none;
        border-radius: 0;
        background-color: var(--warning);
        color: white;
        cursor: pointer;
        flex-shrink: 0;
        height: 3em;
        width: 3em;
        display: flex;
        align-items: center;
        justify-content: center;
    }
    
    /*.logout-btn:hover {*/
    /*    background: ;*/
    /*}*/
    
    /* User Profile Button */
    .user-profile-btn {
        border: 0 none;
        border-radius: 0;
        display: flex;
        align-items: center;
        background-color: var(--button-hover-bg);
        cursor: pointer;
        color: var(--text-color);
        height: 3em;
        width: auto;
        min-width: 3em;
        flex-shrink: 0;
        line-height: 1;
        transition: background-color 0.2s;
        justify-content: center;
        padding: 1em 1em 1em 1em;
        margin: 0;
    }
    
    .user-profile-btn:hover {
        background-color: var(--button-bg);
    }
    
    .user-avatar, .user-avatar-placeholder {
        width: 1em;
        height: 1em;
        object-fit: cover;
    }
    
    .user-avatar-placeholder {
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 0.5em;
        padding: 0.5em;
    }
    
    .user-name {
        font-size: 0.8rem;
        font-weight: 500;
        max-width: 100px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    
    /* Settings Drawer */
    .drawer-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0, 0, 0, 0.5);
        z-index: 1000;
        display: flex;
        justify-content: flex-end;
    }
    
    .settings-drawer {
        width: 640px;
        height: 100%;
        background: var(--bg-color);
        /*border-left: 1px solid var(--border-color);*/
        overflow-y: auto;
        animation: slideIn 0.3s ease;
    }
    
    @keyframes slideIn {
        from { transform: translateX(100%); }
        to { transform: translateX(0); }
    }
    
    .drawer-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        background: var(--header-bg);
    }
    
    .drawer-header h2 {
        margin: 0;
        color: var(--text-color);
        font-size: 1em;
        padding: 1rem;
    }
    
    .close-btn {
        background: none;
        border: none;
        font-size: 1em;
        cursor: pointer;
        color: var(--text-color);
        padding: 0.5em;
        transition: background-color 0.2s;
        align-items: center;
    }
    
    .close-btn:hover {
        background: var(--button-hover-bg);
    }
    
    .profile-section {
        margin-bottom: 2rem;
    }

    .profile-hero {
        position: relative;
    }
    
    .profile-banner {
        width: 100%;
        height: 160px;
        object-fit: cover;
        border-radius: 0;
        display: block;
    }

    /* Avatar sits half over the bottom edge of the banner */
    .profile-avatar, .profile-avatar-placeholder {
        width: 72px;
        height: 72px;
        border-radius: 50%;
        object-fit: cover;
        flex-shrink: 0;
        box-shadow: 0 2px 8px rgba(0,0,0,0.25);
        border: 2px solid var(--bg-color);
    }

    .overlap {
        position: absolute;
        left: 12px;
        bottom: -36px; /* half out of the banner */
        z-index: 2;
        background: var(--button-hover-bg);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 1.5rem;
    }

    /* Username and nip05 on the banner, to the right of avatar */
    .name-row {
        position: absolute;
        left: calc(12px + 72px + 12px);
        bottom: 8px;
        right: 12px;
        display: flex;
        align-items: baseline;
        gap: 8px;
        z-index: 1;
  }

    .profile-username {
        margin: 0;
        font-size: 1.1rem;
        color: #000; /* contrasting over banner */
        text-shadow: 0 3px 6px rgba(255,255,255,1);
    }

    .profile-nip05-inline {
        font-size: 0.85rem;
        color: #000; /* subtle but contrasting */
        font-family: monospace;
        opacity: 0.95;
        text-shadow: 0 3px 6px rgba(255,255,255,1);
    }

    /* About box below with overlap space for avatar */
    .about-card {
        background: var(--header-bg);
        padding: 12px 12px 12px 96px; /* offset text from overlapping avatar */
        position: relative;
        word-break: auto-phrase;
    }

    .profile-about {
        margin: 0;
        color: var(--text-color);
        font-size: 0.9rem;
        line-height: 1.4;
    }

    .profile-loading-section {
        padding: 1rem;
        text-align: center;
    }

    .profile-loading-section h3 {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        font-size: 1.1rem;
    }

    .profile-loading-section p {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        opacity: 0.8;
    }

    .retry-profile-btn {
        padding: 0.5rem 1rem;
        background: var(--primary);
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9rem;
        margin-bottom: 1rem;
        transition: background-color 0.2s;
    }

    .retry-profile-btn:hover {
        background: #00ACC1;
    }

    .user-pubkey-display {
        font-family: monospace;
        font-size: 0.8rem;
        color: var(--text-color);
        opacity: 0.7;
        background: var(--button-bg);
        padding: 0.5rem;
        border-radius: 4px;
        word-break: break-all;
    }
    
    @media (max-width: 640px) {
        .settings-drawer {
            width: 100%;
        }
        
        .name-row {
            left: calc(8px + 56px + 8px);
            bottom: 6px;
            right: 8px;
            gap: 6px;
        }

        .profile-username { font-size: 1rem; }
        .profile-nip05-inline { font-size: 0.8rem; }
    }
</style>