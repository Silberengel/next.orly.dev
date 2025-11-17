<script>
    import { createEventDispatcher } from "svelte";
    import { prettyPrintFilter } from "./helpers.tsx";
    
    const dispatch = createEventDispatcher();

    export let filter = {};
    export let showFilter = true;

    $: filterJson = prettyPrintFilter(filter);
    $: hasFilter = Object.keys(filter).length > 0;

    function handleSweep() {
        dispatch("sweep");
    }
</script>

{#if showFilter && hasFilter}
    <div class="filter-display">
        <div class="filter-display-header">
            <h3>Active Filter</h3>
            <button class="sweep-btn" on:click={handleSweep} title="Clear filter">
                🧹 Sweep
            </button>
        </div>
        <div class="filter-json-container">
            <pre class="filter-json">{filterJson}</pre>
        </div>
    </div>
{/if}

<style>
    .filter-display {
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        margin: 1em;
        overflow: hidden;
    }

    .filter-display-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.75em 1em;
        background: var(--bg-color);
        border-bottom: 1px solid var(--border-color);
    }

    .filter-display-header h3 {
        margin: 0;
        font-size: 1em;
        font-weight: 600;
        color: var(--text-color);
    }

    .sweep-btn {
        background: var(--danger);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        font-weight: 600;
        transition: all 0.2s;
    }

    .sweep-btn:hover {
        filter: brightness(0.9);
        transform: translateY(-1px);
        box-shadow: 0 2px 8px rgba(255, 0, 0, 0.3);
    }

    .filter-json-container {
        padding: 1em;
        max-height: 200px;
        overflow: auto;
    }

    .filter-json {
        background: var(--code-bg);
        padding: 1em;
        border-radius: 4px;
        font-family: 'Courier New', Courier, monospace;
        font-size: 0.85em;
        line-height: 1.5;
        color: var(--code-text);
        margin: 0;
        white-space: pre-wrap;
        word-wrap: break-word;
        word-break: break-all;
        overflow-wrap: anywhere;
    }

    /* Custom scrollbar for json container */
    .filter-json-container::-webkit-scrollbar {
        width: 8px;
        height: 8px;
    }

    .filter-json-container::-webkit-scrollbar-track {
        background: var(--bg-color);
    }

    .filter-json-container::-webkit-scrollbar-thumb {
        background: var(--border-color);
        border-radius: 4px;
    }

    .filter-json-container::-webkit-scrollbar-thumb:hover {
        background: var(--primary);
    }
</style>

