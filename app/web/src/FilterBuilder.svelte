<script>
    import { createEventDispatcher } from "svelte";
    import { KIND_NAMES, isValidPubkey, isValidEventId, isValidTagName, formatDateTimeLocal, parseDateTimeLocal } from "./helpers.tsx";
    
    const dispatch = createEventDispatcher();

    // Filter state
    export let searchText = "";
    export let selectedKinds = [];
    export let pubkeys = [];
    export let eventIds = [];
    export let tags = [];
    export let sinceTimestamp = null;
    export let untilTimestamp = null;
    export let limit = null;

    // UI state
    let showKindsPicker = false;
    let kindSearchQuery = "";
    let newPubkey = "";
    let newEventId = "";
    let newTagName = "";
    let newTagValue = "";
    let pubkeyError = "";
    let eventIdError = "";
    let tagNameError = "";

    // Get all available kinds as array
    $: availableKinds = Object.entries(KIND_NAMES).map(([kind, name]) => ({
        kind: parseInt(kind),
        name: name
    })).sort((a, b) => a.kind - b.kind);

    // Filter kinds by search query
    $: filteredKinds = availableKinds.filter(k => 
        k.kind.toString().includes(kindSearchQuery) || 
        k.name.toLowerCase().includes(kindSearchQuery.toLowerCase())
    );

    function toggleKind(kind) {
        if (selectedKinds.includes(kind)) {
            selectedKinds = selectedKinds.filter(k => k !== kind);
        } else {
            selectedKinds = [...selectedKinds, kind].sort((a, b) => a - b);
        }
    }

    function removeKind(kind) {
        selectedKinds = selectedKinds.filter(k => k !== kind);
    }

    function addPubkey() {
        const trimmed = newPubkey.trim();
        if (!trimmed) return;
        
        if (!isValidPubkey(trimmed)) {
            pubkeyError = "Invalid pubkey: must be 64 character hex string";
            return;
        }
        
        if (pubkeys.includes(trimmed)) {
            pubkeyError = "Pubkey already added";
            return;
        }
        
        pubkeys = [...pubkeys, trimmed];
        newPubkey = "";
        pubkeyError = "";
    }

    function removePubkey(pubkey) {
        pubkeys = pubkeys.filter(p => p !== pubkey);
    }

    function addEventId() {
        const trimmed = newEventId.trim();
        if (!trimmed) return;
        
        if (!isValidEventId(trimmed)) {
            eventIdError = "Invalid event ID: must be 64 character hex string";
            return;
        }
        
        if (eventIds.includes(trimmed)) {
            eventIdError = "Event ID already added";
            return;
        }
        
        eventIds = [...eventIds, trimmed];
        newEventId = "";
        eventIdError = "";
    }

    function removeEventId(eventId) {
        eventIds = eventIds.filter(id => id !== eventId);
    }

    function addTag() {
        const trimmedName = newTagName.trim();
        const trimmedValue = newTagValue.trim();
        
        if (!trimmedName || !trimmedValue) return;
        
        if (!isValidTagName(trimmedName)) {
            tagNameError = "Invalid tag name: must be single letter a-z or A-Z";
            return;
        }
        
        // Check if this exact tag already exists
        if (tags.some(t => t.name === trimmedName && t.value === trimmedValue)) {
            tagNameError = "Tag already added";
            return;
        }
        
        tags = [...tags, { name: trimmedName, value: trimmedValue }];
        newTagName = "";
        newTagValue = "";
        tagNameError = "";
    }

    function removeTag(index) {
        tags = tags.filter((_, i) => i !== index);
    }

    function clearAllFilters() {
        searchText = "";
        selectedKinds = [];
        pubkeys = [];
        eventIds = [];
        tags = [];
        sinceTimestamp = null;
        untilTimestamp = null;
        limit = null;
        dispatch("clear");
    }

    function applyFilters() {
        dispatch("apply", {
            searchText,
            selectedKinds,
            pubkeys,
            eventIds,
            tags,
            sinceTimestamp,
            untilTimestamp,
            limit
        });
    }

    // Format timestamp for input
    function getFormattedSince() {
        return sinceTimestamp ? formatDateTimeLocal(sinceTimestamp) : "";
    }

    function getFormattedUntil() {
        return untilTimestamp ? formatDateTimeLocal(untilTimestamp) : "";
    }

    function handleSinceChange(event) {
        const value = event.target.value;
        sinceTimestamp = value ? parseDateTimeLocal(value) : null;
    }

    function handleUntilChange(event) {
        const value = event.target.value;
        untilTimestamp = value ? parseDateTimeLocal(value) : null;
    }
</script>

<div class="filter-builder">
    <!-- Search text input at top -->
    <div class="filter-section">
        <label for="search-text">Search Text (NIP-50)</label>
        <input
            id="search-text"
            type="text"
            bind:value={searchText}
            placeholder="Search events..."
            class="filter-input"
        />
    </div>

    <!-- Kinds picker -->
    <div class="filter-section">
        <label>Event Kinds</label>
        <button 
            class="picker-toggle-btn" 
            on:click={() => showKindsPicker = !showKindsPicker}
        >
            {showKindsPicker ? "▼" : "▶"} Select Kinds ({selectedKinds.length} selected)
        </button>
        
        {#if showKindsPicker}
            <div class="kinds-picker">
                <input
                    type="text"
                    bind:value={kindSearchQuery}
                    placeholder="Search kinds..."
                    class="filter-input kind-search"
                />
                <div class="kinds-list">
                    {#each filteredKinds as { kind, name }}
                        <label class="kind-checkbox">
                            <input
                                type="checkbox"
                                checked={selectedKinds.includes(kind)}
                                on:change={() => toggleKind(kind)}
                            />
                            <span class="kind-number">{kind}</span>
                            <span class="kind-name">{name}</span>
                        </label>
                    {/each}
                </div>
            </div>
        {/if}
        
        <!-- Selected kinds chips -->
        {#if selectedKinds.length > 0}
            <div class="chips-container">
                {#each selectedKinds as kind}
                    <div class="chip">
                        <span class="chip-text">{kind}: {KIND_NAMES[kind] || `Kind ${kind}`}</span>
                        <button class="chip-remove" on:click={() => removeKind(kind)}>×</button>
                    </div>
                {/each}
            </div>
        {/if}
    </div>

    <!-- Authors/Pubkeys -->
    <div class="filter-section">
        <label>Authors (Pubkeys)</label>
        <div class="input-group">
            <input
                type="text"
                bind:value={newPubkey}
                placeholder="64 character hex pubkey..."
                class="filter-input"
                maxlength="64"
                on:keydown={(e) => e.key === 'Enter' && addPubkey()}
            />
            <button class="add-btn" on:click={addPubkey}>Add</button>
        </div>
        {#if pubkeyError}
            <div class="error-message">{pubkeyError}</div>
        {/if}
        {#if pubkeys.length > 0}
            <div class="list-items">
                {#each pubkeys as pubkey}
                    <div class="list-item">
                        <span class="list-item-text">{pubkey}</span>
                        <button class="list-item-remove" on:click={() => removePubkey(pubkey)}>×</button>
                    </div>
                {/each}
            </div>
        {/if}
    </div>

    <!-- Event IDs -->
    <div class="filter-section">
        <label>Event IDs</label>
        <div class="input-group">
            <input
                type="text"
                bind:value={newEventId}
                placeholder="64 character hex event ID..."
                class="filter-input"
                maxlength="64"
                on:keydown={(e) => e.key === 'Enter' && addEventId()}
            />
            <button class="add-btn" on:click={addEventId}>Add</button>
        </div>
        {#if eventIdError}
            <div class="error-message">{eventIdError}</div>
        {/if}
        {#if eventIds.length > 0}
            <div class="list-items">
                {#each eventIds as eventId}
                    <div class="list-item">
                        <span class="list-item-text">{eventId}</span>
                        <button class="list-item-remove" on:click={() => removeEventId(eventId)}>×</button>
                    </div>
                {/each}
            </div>
        {/if}
    </div>

    <!-- Tags -->
    <div class="filter-section">
        <label>Tags (e.g., #e, #p, #a)</label>
        <div class="tag-input-group">
            <span class="hash-prefix">#</span>
            <input
                type="text"
                bind:value={newTagName}
                placeholder="Tag"
                class="filter-input tag-name-input"
                maxlength="1"
            />
            <input
                type="text"
                bind:value={newTagValue}
                placeholder="Value..."
                class="filter-input tag-value-input"
                on:keydown={(e) => e.key === 'Enter' && addTag()}
            />
            <button class="add-btn" on:click={addTag}>Add</button>
        </div>
        {#if tagNameError}
            <div class="error-message">{tagNameError}</div>
        {/if}
        {#if tags.length > 0}
            <div class="list-items">
                {#each tags as tag, index}
                    <div class="list-item">
                        <span class="list-item-text">#{tag.name}: {tag.value}</span>
                        <button class="list-item-remove" on:click={() => removeTag(index)}>×</button>
                    </div>
                {/each}
            </div>
        {/if}
    </div>

    <!-- Since/Until timestamps -->
    <div class="filter-section timestamps-section">
        <div class="timestamp-field">
            <label for="since-timestamp">Since</label>
            <input
                id="since-timestamp"
                type="datetime-local"
                value={getFormattedSince()}
                on:change={handleSinceChange}
                class="filter-input"
            />
            {#if sinceTimestamp}
                <button class="clear-timestamp-btn" on:click={() => sinceTimestamp = null}>×</button>
            {/if}
        </div>
        
        <div class="timestamp-field">
            <label for="until-timestamp">Until</label>
            <input
                id="until-timestamp"
                type="datetime-local"
                value={getFormattedUntil()}
                on:change={handleUntilChange}
                class="filter-input"
            />
            {#if untilTimestamp}
                <button class="clear-timestamp-btn" on:click={() => untilTimestamp = null}>×</button>
            {/if}
        </div>
    </div>

    <!-- Limit -->
    <div class="filter-section">
        <label for="limit">Limit (optional)</label>
        <input
            id="limit"
            type="number"
            bind:value={limit}
            placeholder="Max events to return"
            class="filter-input"
            min="1"
        />
    </div>

    <!-- Action buttons -->
    <div class="filter-actions">
        <button class="apply-btn" on:click={applyFilters}>🔍 Apply Filters</button>
        <button class="clear-btn" on:click={clearAllFilters}>🧹 Clear All</button>
    </div>
</div>

<style>
    .filter-builder {
        padding: 1em;
        background: var(--bg-color);
        border-bottom: 1px solid var(--border-color);
    }

    .filter-section {
        margin-bottom: 1.25em;
    }

    .filter-section label {
        display: block;
        margin-bottom: 0.5em;
        font-weight: 600;
        color: var(--text-color);
        font-size: 0.9em;
    }

    .filter-input {
        width: 100%;
        padding: 0.6em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-size: 0.9em;
        box-sizing: border-box;
    }

    .filter-input:focus {
        outline: none;
        border-color: var(--primary);
        box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.15);
    }

    .picker-toggle-btn {
        width: 100%;
        padding: 0.6em;
        background: var(--secondary);
        color: var(--text-color);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        text-align: left;
        transition: background-color 0.2s;
    }

    .picker-toggle-btn:hover {
        background: var(--accent-hover-color);
    }

    .kinds-picker {
        margin-top: 0.5em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        padding: 0.5em;
        background: var(--card-bg);
    }

    .kind-search {
        margin-bottom: 0.5em;
    }

    .kinds-list {
        max-height: 300px;
        overflow-y: auto;
    }

    .kind-checkbox {
        display: flex;
        align-items: center;
        padding: 0.4em;
        cursor: pointer;
        border-radius: 4px;
        transition: background-color 0.2s;
    }

    .kind-checkbox:hover {
        background: var(--bg-color);
    }

    .kind-checkbox input[type="checkbox"] {
        margin-right: 0.5em;
        cursor: pointer;
    }

    .kind-number {
        background: var(--primary);
        color: var(--text-color);
        padding: 0.1em 0.4em;
        border-radius: 3px;
        font-size: 0.8em;
        font-weight: 600;
        font-family: monospace;
        margin-right: 0.5em;
        min-width: 40px;
        text-align: center;
        display: inline-block;
    }

    .kind-name {
        font-size: 0.85em;
        color: var(--text-color);
    }

    .chips-container {
        display: flex;
        flex-wrap: wrap;
        gap: 0.5em;
        margin-top: 0.5em;
    }

    .chip {
        display: inline-flex;
        align-items: center;
        background: var(--primary);
        color: var(--text-color);
        padding: 0.3em 0.6em;
        border-radius: 16px;
        font-size: 0.85em;
        gap: 0.5em;
    }

    .chip-text {
        font-weight: 500;
    }

    .chip-remove {
        background: transparent;
        border: none;
        color: var(--text-color);
        cursor: pointer;
        padding: 0;
        font-size: 1.2em;
        line-height: 1;
        opacity: 0.8;
        transition: opacity 0.2s;
    }

    .chip-remove:hover {
        opacity: 1;
    }

    .input-group {
        display: flex;
        gap: 0.5em;
    }

    .input-group .filter-input {
        flex: 1;
    }

    .add-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.6em 1.2em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        font-weight: 600;
        transition: background-color 0.2s;
        white-space: nowrap;
    }

    .add-btn:hover {
        background: var(--accent-hover-color);
    }

    .error-message {
        color: var(--danger);
        font-size: 0.85em;
        margin-top: 0.25em;
    }

    .list-items {
        margin-top: 0.5em;
        display: flex;
        flex-direction: column;
        gap: 0.5em;
    }

    .list-item {
        display: flex;
        align-items: center;
        padding: 0.5em;
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        gap: 0.5em;
    }

    .list-item-text {
        flex: 1;
        font-family: monospace;
        font-size: 0.85em;
        color: var(--text-color);
        word-break: break-all;
    }

    .list-item-remove {
        background: var(--danger);
        color: var(--text-color);
        border: none;
        padding: 0.25em 0.5em;
        border-radius: 3px;
        cursor: pointer;
        font-size: 1.2em;
        line-height: 1;
        transition: background-color 0.2s;
    }

    .list-item-remove:hover {
        filter: brightness(0.9);
    }

    .tag-input-group {
        display: flex;
        gap: 0.5em;
        align-items: center;
    }

    .hash-prefix {
        font-weight: 700;
        font-size: 1.2em;
        color: var(--text-color);
    }

    .tag-name-input {
        width: 50px;
    }

    .tag-value-input {
        flex: 1;
    }

    .timestamps-section {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 1em;
    }

    .timestamp-field {
        position: relative;
    }

    .clear-timestamp-btn {
        position: absolute;
        right: 0.5em;
        top: 2em;
        background: var(--danger);
        color: var(--text-color);
        border: none;
        padding: 0.25em 0.5em;
        border-radius: 3px;
        cursor: pointer;
        font-size: 1em;
        line-height: 1;
        transition: background-color 0.2s;
    }

    .clear-timestamp-btn:hover {
        filter: brightness(0.9);
    }

    .filter-actions {
        display: flex;
        gap: 1em;
        padding-top: 1em;
        border-top: 1px solid var(--border-color);
    }

    .apply-btn,
    .clear-btn {
        flex: 1;
        padding: 0.75em 1em;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 1em;
        font-weight: 600;
        transition: all 0.2s;
    }

    .apply-btn {
        background: var(--primary);
        color: var(--text-color);
    }

    .apply-btn:hover {
        background: var(--accent-hover-color);
        transform: translateY(-1px);
        box-shadow: 0 2px 8px rgba(0, 123, 255, 0.3);
    }

    .clear-btn {
        background: var(--secondary);
        color: var(--text-color);
    }

    .clear-btn:hover {
        background: var(--danger);
        transform: translateY(-1px);
    }

    /* Responsive design */
    @media (max-width: 768px) {
        .timestamps-section {
            grid-template-columns: 1fr;
        }
    }
</style>

