<script>
    export let searchTab = null;
    export let searchResults = new Map();
    export let expandedEvents = new Set();
    export let userRole = "";
    export let userPubkey = "";

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    function loadSearchResults(tabId, query, refresh) {
        dispatch("loadSearchResults", { tabId, query, refresh });
    }

    function handleSearchScroll(event, tabId) {
        dispatch("searchScroll", { event, tabId });
    }

    function toggleEventExpansion(eventId) {
        dispatch("toggleEventExpansion", eventId);
    }

    function deleteEvent(eventId) {
        dispatch("deleteEvent", eventId);
    }

    function copyEventToClipboard(event, e) {
        dispatch("copyEventToClipboard", { event, e });
    }

    function truncatePubkey(pubkey) {
        if (!pubkey) return "";
        return pubkey.slice(0, 8) + "..." + pubkey.slice(-8);
    }

    function getKindName(kind) {
        const kindNames = {
            0: "Profile",
            1: "Text Note",
            2: "Recommend Relay",
            3: "Contacts",
            4: "Encrypted DM",
            5: "Delete",
            6: "Repost",
            7: "Reaction",
            8: "Badge Award",
            16: "Generic Repost",
            40: "Channel Creation",
            41: "Channel Metadata",
            42: "Channel Message",
            43: "Channel Hide Message",
            44: "Channel Mute User",
            1984: "Reporting",
            9734: "Zap Request",
            9735: "Zap",
            10000: "Mute List",
            10001: "Pin List",
            10002: "Relay List",
            22242: "Client Auth",
            24133: "Nostr Connect",
            27235: "HTTP Auth",
            30000: "Categorized People",
            30001: "Categorized Bookmarks",
            30008: "Profile Badges",
            30009: "Badge Definition",
            30017: "Create or update a stall",
            30018: "Create or update a product",
            30023: "Long-form Content",
            30024: "Draft Long-form Content",
            30078: "Application-specific Data",
            30311: "Live Event",
            30315: "User Statuses",
            30402: "Classified Listing",
            30403: "Draft Classified Listing",
            31922: "Date-Based Calendar Event",
            31923: "Time-Based Calendar Event",
            31924: "Calendar",
            31925: "Calendar Event RSVP",
            31989: "Handler recommendation",
            31990: "Handler information",
            34550: "Community Definition",
        };
        return kindNames[kind] || `Kind ${kind}`;
    }

    function formatTimestamp(timestamp) {
        return new Date(timestamp * 1000).toLocaleString();
    }

    function truncateContent(content) {
        if (!content) return "";
        return content.length > 100 ? content.slice(0, 100) + "..." : content;
    }
</script>

{#if searchTab}
    <div class="search-results-view">
        <div class="search-results-header">
            <h2>🔍 Search Results: "{searchTab.query}"</h2>
            <button
                class="refresh-btn"
                on:click={() =>
                    loadSearchResults(searchTab.id, searchTab.query, true)}
                disabled={searchResults.get(searchTab.id)?.isLoading}
            >
                🔄 Refresh
            </button>
        </div>
        <div
            class="search-results-content"
            on:scroll={(e) => handleSearchScroll(e, searchTab.id)}
        >
            {#if searchResults.get(searchTab.id)?.events?.length > 0}
                {#each searchResults.get(searchTab.id).events as event}
                    <div
                        class="search-result-item"
                        class:expanded={expandedEvents.has(event.id)}
                    >
                        <div
                            class="search-result-row"
                            on:click={() => toggleEventExpansion(event.id)}
                            on:keydown={(e) =>
                                e.key === "Enter" &&
                                toggleEventExpansion(event.id)}
                            role="button"
                            tabindex="0"
                        >
                            <div class="search-result-avatar">
                                <div class="avatar-placeholder">👤</div>
                            </div>
                            <div class="search-result-info">
                                <div class="search-result-author">
                                    {truncatePubkey(event.pubkey)}
                                </div>
                                <div class="search-result-kind">
                                    <span class="kind-number">{event.kind}</span
                                    >
                                    <span class="kind-name"
                                        >{getKindName(event.kind)}</span
                                    >
                                </div>
                            </div>
                            <div class="search-result-content">
                                <div class="event-timestamp">
                                    {formatTimestamp(event.created_at)}
                                </div>
                                <div class="event-content-single-line">
                                    {truncateContent(event.content)}
                                </div>
                            </div>
                            {#if userRole === "admin" || userRole === "owner" || (userRole === "write" && event.pubkey && event.pubkey === userPubkey)}
                                <button
                                    class="delete-btn"
                                    on:click|stopPropagation={() =>
                                        deleteEvent(event.id)}
                                >
                                    🗑️
                                </button>
                            {/if}
                        </div>
                        {#if expandedEvents.has(event.id)}
                            <div class="search-result-details">
                                <div class="json-container">
                                    <pre class="event-json">{JSON.stringify(
                                            event,
                                            null,
                                            2,
                                        )}</pre>
                                    <button
                                        class="copy-json-btn"
                                        on:click|stopPropagation={(e) =>
                                            copyEventToClipboard(event, e)}
                                        title="Copy minified JSON to clipboard"
                                    >
                                        📋
                                    </button>
                                </div>
                            </div>
                        {/if}
                    </div>
                {/each}
            {:else if !searchResults.get(searchTab.id)?.isLoading}
                <div class="no-results">
                    <p>No results found for "{searchTab.query}"</p>
                </div>
            {/if}

            {#if searchResults.get(searchTab.id)?.isLoading}
                <div class="loading-search">
                    <div class="spinner"></div>
                    <p>Searching...</p>
                </div>
            {/if}
        </div>
    </div>
{/if}

<style>
    .search-results-view {
        width: 100%;
        height: 100%;
        display: flex;
        flex-direction: column;
    }

    .search-results-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 1em;
        border-bottom: 1px solid var(--border-color);
        background: var(--header-bg);
    }

    .search-results-header h2 {
        margin: 0;
        color: var(--text-color);
        font-size: 1.2rem;
        font-weight: 600;
    }

    .refresh-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        transition: background-color 0.2s;
    }

    .refresh-btn:hover:not(:disabled) {
        background: var(--accent-hover-color);
    }

    .refresh-btn:disabled {
        background: var(--secondary);
        cursor: not-allowed;
    }

    .search-results-content {
        flex: 1;
        overflow-y: auto;
        padding: 1em;
    }

    .search-result-item {
        border: 1px solid var(--border-color);
        border-radius: 8px;
        margin-bottom: 0.5em;
        background: var(--card-bg);
        transition: all 0.2s ease;
    }

    .search-result-item:hover {
        border-color: var(--primary);
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    }

    .search-result-item.expanded {
        border-color: var(--primary);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    }

    .search-result-row {
        display: flex;
        align-items: center;
        padding: 1em;
        cursor: pointer;
        gap: 1em;
    }

    .search-result-avatar {
        flex-shrink: 0;
    }

    .avatar-placeholder {
        width: 40px;
        height: 40px;
        border-radius: 50%;
        background: var(--bg-color);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 1.2em;
        border: 1px solid var(--border-color);
    }

    .search-result-info {
        flex-shrink: 0;
        min-width: 120px;
    }

    .search-result-author {
        font-weight: 600;
        color: var(--text-color);
        font-size: 0.9em;
        font-family: monospace;
    }

    .search-result-kind {
        display: flex;
        align-items: center;
        gap: 0.5em;
        margin-top: 0.25em;
    }

    .kind-number {
        background: var(--primary);
        color: var(--text-color);
        padding: 0.1em 0.4em;
        border-radius: 0.25rem;
        font-size: 0.7em;
        font-weight: 600;
        font-family: monospace;
    }

    .kind-name {
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.8;
    }

    .search-result-content {
        flex: 1;
        min-width: 0;
    }

    .event-timestamp {
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.6;
        margin-bottom: 0.5em;
    }

    .event-content-single-line {
        color: var(--text-color);
        line-height: 1.4;
        word-wrap: break-word;
    }

    .delete-btn {
        background: var(--danger);
        color: var(--text-color);
        border: none;
        padding: 0.5em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        flex-shrink: 0;
        transition: background-color 0.2s;
    }

    .delete-btn:hover {
        background: var(--danger);
        filter: brightness(0.9);
    }

    .search-result-details {
        border-top: 1px solid var(--border-color);
        padding: 1em;
        background: var(--bg-color);
    }

    .json-container {
        position: relative;
    }

    .event-json {
        background: var(--code-bg);
        padding: 1em;
        border: 0;
        font-size: 0.8em;
        line-height: 1.4;
        overflow-x: auto;
        margin: 0;
        color: var(--code-text);
    }

    .copy-json-btn {
        position: absolute;
        top: 0.5em;
        right: 0.5em;
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.25em 0.5em;
        border-radius: 0.25rem;
        cursor: pointer;
        font-size: 0.8em;
        opacity: 0.8;
        transition: opacity 0.2s;
    }

    .copy-json-btn:hover {
        opacity: 1;
    }

    .no-results {
        text-align: center;
        padding: 2em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .loading-search {
        text-align: center;
        padding: 2em;
        color: var(--text-color);
    }

    .spinner {
        width: 20px;
        height: 20px;
        border: 2px solid var(--border-color);
        border-top: 2px solid var(--primary);
        border-radius: 50%;
        animation: spin 1s linear infinite;
        margin: 0 auto 1em;
    }

    @keyframes spin {
        0% {
            transform: rotate(0deg);
        }
        100% {
            transform: rotate(360deg);
        }
    }
</style>
