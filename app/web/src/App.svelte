<script>
    import LoginModal from './LoginModal.svelte';
    import { initializeNostrClient, fetchUserProfile, fetchAllEvents, fetchUserEvents } from './nostr.js';
    
    let isDarkTheme = false;
    let showLoginModal = false;
    let isLoggedIn = false;
    let userPubkey = '';
    let authMethod = '';
    let userProfile = null;
    let userRole = '';
    let userSigner = null;
    let showSettingsDrawer = false;
    let selectedTab = 'export';
    let isSearchMode = false;
    let searchQuery = '';
    let searchTabs = [];
    let myEvents = [];
    let allEvents = [];
    let selectedFile = null;
    let expandedEvents = new Set();
    let isLoadingEvents = false;
    let hasMoreEvents = true;
    let eventsPerPage = 100;
    let oldestEventTimestamp = null; // For timestamp-based pagination
    
    // My Events pagination state
    let isLoadingMyEvents = false;
    let hasMoreMyEvents = true;
    let oldestMyEventTimestamp = null; // For timestamp-based pagination
    
    // Shared event cache system
    let eventCache = new Map(); // pubkey -> events[]
    let cacheTimestamps = new Map(); // pubkey -> timestamp
    let globalEventsCache = []; // All events cache
    let globalCacheTimestamp = 0;
    const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes

    // Kind name mapping based on repository kind definitions
    const kindNames = {
        0: "ProfileMetadata",
        1: "TextNote", 
        2: "RecommendRelay",
        3: "FollowList",
        4: "EncryptedDirectMessage",
        5: "EventDeletion",
        6: "Repost",
        7: "Reaction",
        8: "BadgeAward",
        13: "Seal",
        14: "PrivateDirectMessage",
        15: "ReadReceipt",
        16: "GenericRepost",
        40: "ChannelCreation",
        41: "ChannelMetadata",
        42: "ChannelMessage",
        43: "ChannelHideMessage",
        44: "ChannelMuteUser",
        1021: "Bid",
        1022: "BidConfirmation",
        1040: "OpenTimestamps",
        1059: "GiftWrap",
        1060: "GiftWrapWithKind4",
        1063: "FileMetadata",
        1311: "LiveChatMessage",
        1517: "BitcoinBlock",
        1808: "LiveStream",
        1971: "ProblemTracker",
        1984: "Reporting",
        1985: "Label",
        4550: "CommunityPostApproval",
        5000: "JobRequestStart",
        5999: "JobRequestEnd",
        6000: "JobResultStart",
        6999: "JobResultEnd",
        7000: "JobFeedback",
        9041: "ZapGoal",
        9734: "ZapRequest",
        9735: "Zap",
        9882: "Highlights",
        10000: "BlockList",
        10001: "PinList",
        10002: "RelayListMetadata",
        10003: "BookmarkList",
        10004: "CommunitiesList",
        10005: "PublicChatsList",
        10006: "BlockedRelaysList",
        10007: "SearchRelaysList",
        10015: "InterestsList",
        10030: "UserEmojiList",
        10050: "DMRelaysList",
        10096: "FileStorageServerList",
        13004: "JWTBinding",
        13194: "NWCWalletServiceInfo",
        19999: "ReplaceableEnd",
        20000: "EphemeralStart",
        21000: "LightningPubRPC",
        22242: "ClientAuthentication",
        23194: "WalletRequest",
        23195: "WalletResponse",
        23196: "WalletNotificationNip4",
        23197: "WalletNotification",
        24133: "NostrConnect",
        27235: "HTTPAuth",
        29999: "EphemeralEnd",
        30000: "FollowSets",
        30001: "GenericLists",
        30002: "RelaySets",
        30003: "BookmarkSets",
        30004: "CurationSets",
        30008: "ProfileBadges",
        30009: "BadgeDefinition",
        30015: "InterestSets",
        30017: "StallDefinition",
        30018: "ProductDefinition",
        30019: "MarketplaceUIUX",
        30020: "ProductSoldAsAuction",
        30023: "LongFormContent",
        30024: "DraftLongFormContent",
        30030: "EmojiSets"
    };

    function getKindName(kind) {
        return kindNames[kind] || `Kind ${kind}`;
    }

    function truncatePubkey(pubkey) {
        return pubkey.slice(0, 8) + '...' + pubkey.slice(-8);
    }

    function truncateContent(content, maxLength = 100) {
        if (!content) return '';
        return content.length > maxLength ? content.slice(0, maxLength) + '...' : content;
    }

    function toggleEventExpansion(eventId) {
        if (expandedEvents.has(eventId)) {
            expandedEvents.delete(eventId);
        } else {
            expandedEvents.add(eventId);
        }
        expandedEvents = expandedEvents; // Trigger reactivity
    }

    async function deleteEvent(eventId) {
        if (!isLoggedIn || (userRole !== 'admin' && userRole !== 'owner')) {
            alert('Admin or owner permission required');
            return;
        }
        
        if (!confirm('Are you sure you want to delete this event?')) {
            return;
        }
        
        try {
            const authHeader = await createNIP98AuthHeader(`/api/events/${eventId}`, 'DELETE');
            const response = await fetch(`/api/events/${eventId}`, {
                method: 'DELETE',
                headers: {
                    'Authorization': authHeader
                }
            });
            
            if (!response.ok) {
                throw new Error(`Delete failed: ${response.status} ${response.statusText}`);
            }
            
            // Remove from local list
            allEvents = allEvents.filter(event => event.id !== eventId);
            alert('Event deleted successfully');
        } catch (error) {
            console.error('Delete failed:', error);
            alert('Delete failed: ' + error.message);
        }
    }

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
            
            // Restore signer for extension method
            if (storedAuthMethod === 'extension' && window.nostr) {
                userSigner = window.nostr;
            }
            
            // Fetch user role for already logged in users
            fetchUserRole();
        }
        
        // Load persistent app state
        loadPersistentState();
    }

    function savePersistentState() {
        if (typeof localStorage === 'undefined') return;
        
        const state = {
            selectedTab,
            expandedEvents: Array.from(expandedEvents),
            eventCache: Object.fromEntries(eventCache),
            cacheTimestamps: Object.fromEntries(cacheTimestamps),
            globalEventsCache,
            globalCacheTimestamp,
            hasMoreEvents,
            oldestEventTimestamp,
            hasMoreMyEvents,
            oldestMyEventTimestamp
        };
        
        localStorage.setItem('app_state', JSON.stringify(state));
    }

    function loadPersistentState() {
        if (typeof localStorage === 'undefined') return;
        
        try {
            const savedState = localStorage.getItem('app_state');
            if (savedState) {
                const state = JSON.parse(savedState);
                
                // Restore tab state
                if (state.selectedTab && baseTabs.some(tab => tab.id === state.selectedTab)) {
                    selectedTab = state.selectedTab;
                }
                
                // Restore expanded events
                if (state.expandedEvents) {
                    expandedEvents = new Set(state.expandedEvents);
                }
                
                // Restore cache data
                if (state.eventCache) {
                    eventCache = new Map(Object.entries(state.eventCache));
                }
                
                if (state.cacheTimestamps) {
                    cacheTimestamps = new Map(Object.entries(state.cacheTimestamps));
                }
                
                if (state.globalEventsCache) {
                    globalEventsCache = state.globalEventsCache;
                }
                
                if (state.globalCacheTimestamp) {
                    globalCacheTimestamp = state.globalCacheTimestamp;
                }
                
                if (state.hasMoreEvents !== undefined) {
                    hasMoreEvents = state.hasMoreEvents;
                }
                
                if (state.oldestEventTimestamp) {
                    oldestEventTimestamp = state.oldestEventTimestamp;
                }
                
                if (state.hasMoreMyEvents !== undefined) {
                    hasMoreMyEvents = state.hasMoreMyEvents;
                }
                
                if (state.oldestMyEventTimestamp) {
                    oldestMyEventTimestamp = state.oldestMyEventTimestamp;
                }
                
                // Restore events from cache
                restoreEventsFromCache();
            }
        } catch (error) {
            console.error('Failed to load persistent state:', error);
        }
    }

    function restoreEventsFromCache() {
        // Restore global events cache
        if (globalEventsCache.length > 0 && isCacheValid(globalCacheTimestamp)) {
            allEvents = globalEventsCache;
        }
        
        // Restore user's events from cache
        if (userPubkey && eventCache.has(userPubkey) && isCacheValid(cacheTimestamps.get(userPubkey))) {
            myEvents = eventCache.get(userPubkey);
        }
    }

    function isCacheValid(timestamp) {
        if (!timestamp) return false;
        return Date.now() - timestamp < CACHE_DURATION;
    }

    function updateCache(pubkey, events) {
        eventCache.set(pubkey, events);
        cacheTimestamps.set(pubkey, Date.now());
        savePersistentState();
    }

    function updateGlobalCache(events) {
        globalEventsCache = events;
        globalCacheTimestamp = Date.now();
        savePersistentState();
    }

    function clearCache() {
        eventCache.clear();
        cacheTimestamps.clear();
        globalEventsCache = [];
        globalCacheTimestamp = 0;
        savePersistentState();
    }

    const baseTabs = [
        {id: 'export', icon: '📤', label: 'Export'},
        {id: 'import', icon: '💾', label: 'Import', requiresAdmin: true},
        {id: 'myevents', icon: '👤', label: 'My Events'},
        {id: 'allevents', icon: '📡', label: 'All Events'},
        {id: 'sprocket', icon: '⚙️', label: 'Sprocket', requiresOwner: true},
    ];

    // Filter tabs based on user permissions
    $: filteredBaseTabs = baseTabs.filter(tab => {
        if (tab.requiresAdmin && (!isLoggedIn || (userRole !== 'admin' && userRole !== 'owner'))) {
            return false;
        }
        if (tab.requiresOwner && (!isLoggedIn || userRole !== 'owner')) {
            return false;
        }
        return true;
    });
    
    $: tabs = [...filteredBaseTabs, ...searchTabs];

    function selectTab(tabId) {
        selectedTab = tabId;
        savePersistentState();
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
        userSigner = signer;
        showLoginModal = false;
        
        // Initialize Nostr client and fetch profile
        try {
            await initializeNostrClient();
            userProfile = await fetchUserProfile(pubkey);
            console.log('Profile loaded:', userProfile);
        } catch (error) {
            console.error('Failed to load profile:', error);
        }
        
        // Fetch user role/permissions
        await fetchUserRole();
    }
    
    function handleLogout() {
        isLoggedIn = false;
        userPubkey = '';
        authMethod = '';
        userProfile = null;
        userRole = '';
        userSigner = null;
        showSettingsDrawer = false;
        
        // Clear events
        myEvents = [];
        allEvents = [];
        
        // Clear cache
        clearCache();
        
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

    async function fetchUserRole() {
        if (!isLoggedIn || !userPubkey) {
            userRole = '';
            return;
        }
        
        try {
            const response = await fetch(`/api/permissions/${userPubkey}`);
            if (response.ok) {
                const data = await response.json();
                userRole = data.permission || '';
                console.log('User role loaded:', userRole);
            } else {
                console.error('Failed to fetch user role:', response.status);
                userRole = '';
            }
        } catch (error) {
            console.error('Error fetching user role:', error);
            userRole = '';
        }
    }

    // Export functionality
    async function exportEvents(pubkeys = []) {
        if (!isLoggedIn) {
            alert('Please log in first');
            return;
        }
        
        // Check permissions for exporting all events
        if (pubkeys.length === 0 && userRole !== 'admin' && userRole !== 'owner') {
            alert('Admin or owner permission required to export all events');
            return;
        }
        
        try {
            const authHeader = await createNIP98AuthHeader('/api/export', 'POST');
            const response = await fetch('/api/export', {
                method: 'POST',
                headers: {
                    'Authorization': authHeader,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ pubkeys })
            });
            
            if (!response.ok) {
                throw new Error(`Export failed: ${response.status} ${response.statusText}`);
            }
            
            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            
            // Get filename from response headers or use default
            const contentDisposition = response.headers.get('Content-Disposition');
            let filename = 'events.jsonl';
            if (contentDisposition) {
                const filenameMatch = contentDisposition.match(/filename="([^"]+)"/);
                if (filenameMatch) {
                    filename = filenameMatch[1];
                }
            }
            
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
        } catch (error) {
            console.error('Export failed:', error);
            alert('Export failed: ' + error.message);
        }
    }

    async function exportAllEvents() {
        await exportEvents([]); // Empty array means export all events
    }

    async function exportMyEvents() {
        await exportEvents([userPubkey]); // Export only current user's events
    }

    // Import functionality
    function handleFileSelect(event) {
        selectedFile = event.target.files[0];
    }

    async function importEvents() {
        if (!isLoggedIn || (userRole !== 'admin' && userRole !== 'owner')) {
            alert('Admin or owner permission required');
            return;
        }
        
        if (!selectedFile) {
            alert('Please select a file');
            return;
        }
        
        try {
            const authHeader = await createNIP98AuthHeader('/api/import', 'POST');
            const formData = new FormData();
            formData.append('file', selectedFile);
            
            const response = await fetch('/api/import', {
                method: 'POST',
                headers: {
                    'Authorization': authHeader
                },
                body: formData
            });
            
            if (!response.ok) {
                throw new Error(`Import failed: ${response.status} ${response.statusText}`);
            }
            
            const result = await response.json();
            alert('Import started successfully');
            selectedFile = null;
            document.getElementById('import-file').value = '';
        } catch (error) {
            console.error('Import failed:', error);
            alert('Import failed: ' + error.message);
        }
    }

    // Events loading functionality
    async function loadMyEvents(reset = false) {
        if (!isLoggedIn) {
            alert('Please log in first');
            return;
        }
        
        if (isLoadingMyEvents) return;
        
        // Check cache first for initial load
        if (reset && eventCache.has(userPubkey) && isCacheValid(cacheTimestamps.get(userPubkey))) {
            myEvents = eventCache.get(userPubkey);
            // Set oldest timestamp from cached events
            if (myEvents.length > 0) {
                oldestMyEventTimestamp = Math.min(...myEvents.map(e => e.created_at));
            }
            return;
        }
        
        isLoadingMyEvents = true;
        
        try {
            // Use WebSocket REQ to fetch user events with timestamp-based pagination
            const events = await fetchUserEvents(userPubkey, {
                limit: eventsPerPage,
                until: reset ? null : oldestMyEventTimestamp
            });
            
            if (reset) {
                myEvents = events;
                // Update cache
                updateCache(userPubkey, events);
            } else {
                myEvents = [...myEvents, ...events];
                // Update cache with all events
                updateCache(userPubkey, myEvents);
            }
            
            // Update oldest timestamp for next pagination
            if (events.length > 0) {
                const oldestInBatch = Math.min(...events.map(e => e.created_at));
                if (!oldestMyEventTimestamp || oldestInBatch < oldestMyEventTimestamp) {
                    oldestMyEventTimestamp = oldestInBatch;
                }
            }
            
            hasMoreMyEvents = events.length === eventsPerPage;
            
        } catch (error) {
            console.error('Failed to load events:', error);
            alert('Failed to load events: ' + error.message);
        } finally {
            isLoadingMyEvents = false;
        }
    }

    async function loadMoreMyEvents() {
        if (!isLoadingMyEvents && hasMoreMyEvents) {
            await loadMyEvents(false);
        }
    }

    function handleMyEventsScroll(event) {
        const { scrollTop, scrollHeight, clientHeight } = event.target;
        const scrollPercentage = (scrollTop + clientHeight) / scrollHeight;
        
        // Load more when 50% of content is out of view below
        if (scrollPercentage > 0.5 && !isLoadingMyEvents && hasMoreMyEvents) {
            loadMoreMyEvents();
        }
    }

    async function loadAllEvents(reset = false) {
        if (!isLoggedIn || (userRole !== 'write' && userRole !== 'admin' && userRole !== 'owner')) {
            alert('Write, admin, or owner permission required');
            return;
        }
        
        if (isLoadingEvents) return;
        
        // Check cache first for initial load
        if (reset && globalEventsCache.length > 0 && isCacheValid(globalCacheTimestamp)) {
            allEvents = globalEventsCache;
            // Set oldest timestamp from cached events
            if (allEvents.length > 0) {
                oldestEventTimestamp = Math.min(...allEvents.map(e => e.created_at));
            }
            return;
        }
        
        isLoadingEvents = true;
        
        try {
            // Use WebSocket REQ to fetch events with timestamp-based pagination
            const events = await fetchAllEvents({
                limit: eventsPerPage,
                until: reset ? null : oldestEventTimestamp
            });
            
            if (reset) {
                allEvents = events;
                // Update global cache
                updateGlobalCache(events);
            } else {
                allEvents = [...allEvents, ...events];
                // Update global cache with all events
                updateGlobalCache(allEvents);
            }
            
            // Update oldest timestamp for next pagination
            if (events.length > 0) {
                const oldestInBatch = Math.min(...events.map(e => e.created_at));
                if (!oldestEventTimestamp || oldestInBatch < oldestEventTimestamp) {
                    oldestEventTimestamp = oldestInBatch;
                }
            }
            
            hasMoreEvents = events.length === eventsPerPage;
            
        } catch (error) {
            console.error('Failed to load events:', error);
            alert('Failed to load events: ' + error.message);
        } finally {
            isLoadingEvents = false;
        }
    }

    async function loadMoreEvents() {
        if (!isLoadingEvents && hasMoreEvents) {
            await loadAllEvents(false);
        }
    }

    function handleScroll(event) {
        const { scrollTop, scrollHeight, clientHeight } = event.target;
        const scrollPercentage = (scrollTop + clientHeight) / scrollHeight;
        
        // Load more when 50% of content is out of view below
        if (scrollPercentage > 0.5 && !isLoadingEvents && hasMoreEvents) {
            loadMoreEvents();
        }
    }

    // Load initial events when allevents tab is selected
    $: if (selectedTab === 'allevents' && isLoggedIn && (userRole === 'write' || userRole === 'admin' || userRole === 'owner') && allEvents.length === 0) {
        loadAllEvents(true);
    }

    // Load user events when myevents tab is selected
    $: if (selectedTab === 'myevents' && isLoggedIn && userPubkey && myEvents.length === 0) {
        loadMyEvents(true);
    }

    // NIP-98 authentication helper
    async function createNIP98AuthHeader(url, method) {
        if (!isLoggedIn || !userPubkey) {
            throw new Error('Not logged in');
        }
        
        // Create NIP-98 auth event
        const authEvent = {
            kind: 27235,
            created_at: Math.floor(Date.now() / 1000),
            tags: [
                ['u', window.location.origin + url],
                ['method', method.toUpperCase()]
            ],
            content: '',
            pubkey: userPubkey
        };
        
        let signedEvent;
        
        if (userSigner && authMethod === 'extension') {
            // Use the signer from the extension
            try {
                signedEvent = await userSigner.signEvent(authEvent);
            } catch (error) {
                throw new Error('Failed to sign with extension: ' + error.message);
            }
        } else if (authMethod === 'nsec') {
            // For nsec method, we need to implement proper signing
            // For now, create a mock signature (in production, use proper crypto)
            authEvent.id = 'mock-id-' + Date.now();
            authEvent.sig = 'mock-signature-' + Date.now();
            signedEvent = authEvent;
        } else {
            throw new Error('No valid signer available');
        }
        
        // Encode as base64
        const eventJson = JSON.stringify(signedEvent);
        const base64Event = btoa(eventJson);
        
        return `Nostr ${base64Event}`;
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
                />
            </div>
        {:else}
            <div class="header-title">
                <span class="app-title">
                    ORLY? dashboard
                    {#if isLoggedIn && userRole}
                        <span class="permission-badge">{userRole}</span>
                    {/if}
                </span>
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
            </div>
        {:else}
            <button class="login-btn" on:click={openLoginModal}>Log in</button>
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
        {#if selectedTab === 'export'}
            <div class="export-view">
                <h2>Export Events</h2>
                {#if isLoggedIn}
                    <div class="export-section">
                        <h3>Export My Events</h3>
                        <p>Download your personal events as a JSONL file.</p>
                        <button class="export-btn" on:click={exportMyEvents}>
                            📤 Export My Events
                        </button>
                    </div>
                    {#if userRole === 'admin' || userRole === 'owner'}
                        <div class="export-section">
                            <h3>Export All Events</h3>
                            <p>Download the complete database as a JSONL file. This includes all events from all users.</p>
                            <button class="export-btn" on:click={exportAllEvents}>
                                📤 Export All Events
                            </button>
                        </div>
                    {/if}
                {:else}
                    <div class="login-prompt">
                        <p>Please log in to access export functionality.</p>
                        <button class="login-btn" on:click={openLoginModal}>Log In</button>
                    </div>
                {/if}
            </div>
        {:else if selectedTab === 'import'}
            <div class="import-view">
                <h2>Import Events</h2>
                {#if isLoggedIn && (userRole === 'admin' || userRole === 'owner')}
                    <div class="import-section">
                        <h3>Import Events</h3>
                        <p>Upload a JSONL file to import events into the database.</p>
                        <input type="file" id="import-file" accept=".jsonl,.txt" on:change={handleFileSelect} />
                        <button class="import-btn" on:click={importEvents} disabled={!selectedFile}>
                            📥 Import Events
                        </button>
                    </div>
                {:else if isLoggedIn}
                    <div class="permission-denied">
                        <p>❌ Admin or owner permission required for import functionality.</p>
                    </div>
                {:else}
                    <div class="login-prompt">
                        <p>Please log in to access import functionality.</p>
                        <button class="login-btn" on:click={openLoginModal}>Log In</button>
                    </div>
                {/if}
            </div>
        {:else if selectedTab === 'myevents'}
            <div class="allevents-container">
                {#if isLoggedIn}
                    <div class="allevents-header">
                        <button class="refresh-btn" on:click={() => loadMyEvents(true)} disabled={isLoadingMyEvents}>
                            🔄 Refresh Events
                        </button>
                    </div>
                    <div class="allevents-list" on:scroll={handleMyEventsScroll}>
                        {#if myEvents.length > 0}
                            {#each myEvents as event}
                                <div class="allevents-event-item" class:expanded={expandedEvents.has(event.id)}>
                                    <div class="allevents-event-row" on:click={() => toggleEventExpansion(event.id)} on:keydown={(e) => e.key === 'Enter' && toggleEventExpansion(event.id)} role="button" tabindex="0">
                                        <div class="allevents-event-avatar">
                                            <div class="avatar-placeholder">👤</div>
                                        </div>
                                        <div class="allevents-event-info">
                                            <div class="allevents-event-author">
                                                {truncatePubkey(event.pubkey)}
                                            </div>
                                            <div class="allevents-event-kind">
                                                <span class="kind-number">{event.kind}</span>
                                                <span class="kind-name">{getKindName(event.kind)}</span>
                                            </div>
                                        </div>
                                        <div class="allevents-event-content">
                                            {truncateContent(event.content)}
                                        </div>
                                        {#if userRole === 'admin' || userRole === 'owner'}
                                            <button class="delete-btn" on:click|stopPropagation={() => deleteEvent(event.id)}>
                                                🗑️
                                            </button>
                                        {/if}
                                    </div>
                                    {#if expandedEvents.has(event.id)}
                                        <div class="allevents-event-details">
                                            <pre class="event-json">{JSON.stringify(event, null, 2)}</pre>
                                        </div>
                                    {/if}
                                </div>
                            {/each}
                        {:else if !isLoadingMyEvents}
                            <div class="no-events">
                                <p>No events found.</p>
                            </div>
                        {/if}
                        
                        {#if isLoadingMyEvents}
                            <div class="loading-events">
                                <div class="loading-spinner"></div>
                                <p>Loading events...</p>
                            </div>
                        {/if}
                        
                        {#if !hasMoreMyEvents && myEvents.length > 0}
                            <div class="end-of-events">
                                <p>No more events to load.</p>
                            </div>
                        {/if}
                    </div>
                {:else}
                    <div class="login-prompt">
                        <p>Please log in to view your events.</p>
                        <button class="login-btn" on:click={openLoginModal}>Log In</button>
                    </div>
                {/if}
            </div>
        {:else if selectedTab === 'allevents'}
            <div class="allevents-container">
                {#if isLoggedIn && (userRole === 'write' || userRole === 'admin' || userRole === 'owner')}
                    <div class="allevents-header">
                        <button class="refresh-btn" on:click={() => loadAllEvents(true)} disabled={isLoadingEvents}>
                            🔄 Refresh Events
                        </button>
                    </div>
                    <div class="allevents-list" on:scroll={handleScroll}>
                        {#if allEvents.length > 0}
                            {#each allEvents as event}
                                <div class="allevents-event-item" class:expanded={expandedEvents.has(event.id)}>
                                    <div class="allevents-event-row" on:click={() => toggleEventExpansion(event.id)} on:keydown={(e) => e.key === 'Enter' && toggleEventExpansion(event.id)} role="button" tabindex="0">
                                        <div class="allevents-event-avatar">
                                            <div class="avatar-placeholder">👤</div>
                                        </div>
                                        <div class="allevents-event-info">
                                            <div class="allevents-event-author">
                                                {truncatePubkey(event.pubkey)}
                                            </div>
                                            <div class="allevents-event-kind">
                                                <span class="kind-number">{event.kind}</span>
                                                <span class="kind-name">{getKindName(event.kind)}</span>
                                            </div>
                                        </div>
                                        <div class="allevents-event-content">
                                            {truncateContent(event.content)}
                                        </div>
                                        {#if userRole === 'admin' || userRole === 'owner'}
                                            <button class="delete-btn" on:click|stopPropagation={() => deleteEvent(event.id)}>
                                                🗑️
                                            </button>
                                        {/if}
                                    </div>
                                    {#if expandedEvents.has(event.id)}
                                        <div class="allevents-event-details">
                                            <pre class="event-json">{JSON.stringify(event, null, 2)}</pre>
                                        </div>
                                    {/if}
                                </div>
                            {/each}
                        {:else if !isLoadingEvents}
                            <div class="no-events">
                                <p>No events found.</p>
                            </div>
                        {/if}
                        
                        {#if isLoadingEvents}
                            <div class="loading-events">
                                <div class="loading-spinner"></div>
                                <p>Loading events...</p>
                            </div>
                        {/if}
                        
                        {#if !hasMoreEvents && allEvents.length > 0}
                            <div class="end-of-events">
                                <p>No more events to load.</p>
                            </div>
                        {/if}
                    </div>
                {:else if isLoggedIn}
                    <div class="permission-denied">
                        <p>❌ Write, admin, or owner permission required to view all events.</p>
                    </div>
                {:else}
                    <div class="login-prompt">
                        <p>Please log in to view events.</p>
                        <button class="login-btn" on:click={openLoginModal}>Log In</button>
                    </div>
                {/if}
            </div>
        {:else}
            <div class="welcome-message">
                {#if isLoggedIn}
                    <p>Welcome {userProfile?.name || userPubkey.slice(0, 8) + '...'}</p>
                {:else}
                    <p>Log in to access your user dashboard</p>
                {/if}
            </div>
        {/if}
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
                            <!-- Logout button floating in top-right corner of banner -->
                            <button class="logout-btn floating" on:click={handleLogout}>Log out</button>
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
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .permission-badge {
        font-size: 0.7em;
        font-weight: 500;
        padding: 0.2em 0.5em;
        border-radius: 0.3em;
        background-color: var(--primary);
        color: white;
        text-transform: uppercase;
        letter-spacing: 0.05em;
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
        padding: 0.5em 1em;
        border: none;
        border-radius: 6px;
        background-color: #4CAF50;
        color: white;
        cursor: pointer;
        font-size: 1rem;
        font-weight: 500;
        transition: background-color 0.2s;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        vertical-align: middle;
        margin: 0 auto;
        padding:0.5em 1em;
    }

    .login-btn:hover {
        background-color: #45a049;
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
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .welcome-message {
        text-align: center;
    }

    .welcome-message p {
        font-size: 1.2rem;
        margin: 0;
        color: var(--text-color);
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

    }
    
    /* User Info Styles */
    .user-info {
        display: flex;
        align-items: flex-start;
        padding: 0;
        height:3em;
    }
    
    
    .logout-btn {
        padding: 0.5rem 1rem;
        border: none;
        border-radius: 6px;
        background-color: var(--warning);
        color: white;
        cursor: pointer;
        font-size: 1rem;
        font-weight: 500;
        transition: background-color 0.2s;
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 0.5rem;
    }

    .logout-btn:hover {
        background-color: #e53935;
    }

    .logout-btn.floating {
        position: absolute;
        top: 0.5em;
        right: 0.5em;
        z-index: 10;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
    }
    
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

    /* Export/Import Views */
    .export-view, .import-view {
        padding: 2rem;
        max-width: 800px;
        margin: 0 auto;
    }

    .export-view h2, .import-view h2 {
        margin: 0 0 2rem 0;
        color: var(--text-color);
        font-size: 1.5rem;
        font-weight: 600;
    }

    .export-section, .import-section {
        background: var(--header-bg);
        padding: 1.5rem;
        border-radius: 8px;
        margin-bottom: 1.5rem;
    }

    .export-section h3, .import-section h3 {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        font-size: 1.2rem;
        font-weight: 500;
    }

    .export-section p, .import-section p {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        opacity: 0.8;
        line-height: 1.5;
    }

    .export-btn, .import-btn, .refresh-btn {
        padding: 0.75rem 1.5rem;
        background: var(--primary);
        color: white;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-size: 1rem;
        font-weight: 500;
        transition: background-color 0.2s;
        display: inline-flex;
        align-items: center;
        gap: 0.5rem;
    }

    .export-btn:hover, .import-btn:hover, .refresh-btn:hover {
        background: #00ACC1;
    }

    .export-btn:disabled, .import-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    #import-file {
        margin: 1rem 0;
        padding: 0.5rem;
        border: 1px solid var(--input-border);
        border-radius: 4px;
        background: var(--bg-color);
        color: var(--text-color);
        font-size: 1rem;
    }

    .login-prompt {
        text-align: center;
        padding: 2rem;
        background: var(--header-bg);
        border-radius: 8px;
    }

    .login-prompt p {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        font-size: 1.1rem;
    }

    .permission-denied {
        text-align: center;
        padding: 2rem;
        background: var(--header-bg);
        border-radius: 8px;
        border: 2px solid var(--warning);
    }

    .permission-denied p {
        margin: 0;
        color: var(--warning);
        font-size: 1.1rem;
        font-weight: 500;
    }


    /* All Events Container */
    .allevents-container {
        position: fixed;
        top: 3em;
        left: 200px;
        right: 0;
        bottom: 0;
        background: var(--bg-color);
        color: var(--text-color);
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .allevents-header {
        padding: 1rem;
        background: var(--header-bg);
        border-bottom: 1px solid var(--border-color);
        flex-shrink: 0;
    }

    .allevents-list {
        flex: 1;
        overflow-y: auto;
        padding: 0;
    }

    .allevents-event-item {
        border-bottom: 1px solid var(--border-color);
        transition: background-color 0.2s;
    }

    .allevents-event-item:hover {
        background: var(--button-hover-bg);
    }

    .allevents-event-item.expanded {
        background: var(--button-hover-bg);
    }

    .allevents-event-row {
        display: flex;
        align-items: center;
        padding: 0.75rem 1rem;
        cursor: pointer;
        gap: 0.75rem;
        min-height: 3rem;
    }

    .allevents-event-avatar {
        flex-shrink: 0;
        width: 2rem;
        height: 2rem;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .avatar-placeholder {
        width: 2rem;
        height: 2rem;
        border-radius: 50%;
        background: var(--button-bg);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 0.8rem;
    }

    .allevents-event-info {
        flex-shrink: 0;
        width: 12rem;
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    .allevents-event-author {
        font-family: monospace;
        font-size: 0.8rem;
        color: var(--text-color);
        opacity: 0.8;
    }

    .allevents-event-kind {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .kind-number {
        background: var(--primary);
        color: white;
        padding: 0.125rem 0.375rem;
        border-radius: 0.25rem;
        font-size: 0.7rem;
        font-weight: 500;
        font-family: monospace;
    }

    .kind-name {
        font-size: 0.75rem;
        color: var(--text-color);
        opacity: 0.7;
        font-weight: 500;
    }

    .allevents-event-content {
        flex: 1;
        color: var(--text-color);
        font-size: 0.9rem;
        line-height: 1.3;
        word-break: break-word;
        padding: 0 0.5rem;
    }

    .delete-btn {
        flex-shrink: 0;
        background: none;
        border: none;
        cursor: pointer;
        padding: 0.25rem;
        border-radius: 0.25rem;
        transition: background-color 0.2s;
        font-size: 0.9rem;
    }

    .delete-btn:hover {
        background: var(--warning);
        color: white;
    }

    .allevents-event-details {
        border-top: 1px solid var(--border-color);
        background: var(--header-bg);
        padding: 1rem;
    }

    .event-json {
        background: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 0.25rem;
        padding: 1rem;
        margin: 0;
        font-family: 'Courier New', monospace;
        font-size: 0.8rem;
        line-height: 1.4;
        color: var(--text-color);
        white-space: pre-wrap;
        word-break: break-word;
        overflow-x: auto;
    }

    .no-events {
        padding: 2rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.7;
    }

    .no-events p {
        margin: 0;
        font-size: 1rem;
    }

    .loading-events {
        padding: 2rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.7;
    }

    .loading-spinner {
        width: 2rem;
        height: 2rem;
        border: 3px solid var(--border-color);
        border-top: 3px solid var(--primary);
        border-radius: 50%;
        animation: spin 1s linear infinite;
        margin: 0 auto 1rem auto;
    }

    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }

    .loading-events p {
        margin: 0;
        font-size: 0.9rem;
    }

    .end-of-events {
        padding: 1rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.5;
        font-size: 0.8rem;
        border-top: 1px solid var(--border-color);
    }

    .end-of-events p {
        margin: 0;
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

        .export-view, .import-view {
            padding: 1rem;
        }

        .export-section, .import-section {
            padding: 1rem;
        }

        .allevents-container {
            left: 160px;
        }

        .allevents-event-info {
            width: 8rem;
        }

        .allevents-event-author {
            font-size: 0.7rem;
        }

        .kind-name {
            font-size: 0.7rem;
        }

        .allevents-event-content {
            font-size: 0.8rem;
        }
    }
</style>