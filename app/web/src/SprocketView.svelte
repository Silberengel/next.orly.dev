<script>
    export let isLoggedIn = false;
    export let userRole = "";
    export let sprocketStatus = null;
    export let isLoadingSprocket = false;
    export let sprocketUploadFile = null;
    export let sprocketScript = "";
    export let sprocketMessage = "";
    export let sprocketMessageType = "";
    export let sprocketVersions = [];

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    function restartSprocket() {
        dispatch("restartSprocket");
    }

    function deleteSprocket() {
        dispatch("deleteSprocket");
    }

    function handleSprocketFileSelect(event) {
        dispatch("sprocketFileSelect", event);
    }

    function uploadSprocketScript() {
        dispatch("uploadSprocketScript");
    }

    function saveSprocket() {
        dispatch("saveSprocket");
    }

    function loadSprocket() {
        dispatch("loadSprocket");
    }

    function loadVersions() {
        dispatch("loadVersions");
    }

    function loadVersion(version) {
        dispatch("loadVersion", version);
    }

    function deleteVersion(versionName) {
        dispatch("deleteVersion", versionName);
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }
</script>

<div class="sprocket-view">
    <h2>Sprocket Script Management</h2>
    {#if isLoggedIn && userRole === "owner"}
        <div class="sprocket-section">
            <div class="sprocket-header">
                <h3>Script Editor</h3>
                <div class="sprocket-controls">
                    <button
                        class="sprocket-btn restart-btn"
                        on:click={restartSprocket}
                        disabled={isLoadingSprocket}
                    >
                        🔄 Restart
                    </button>
                    <button
                        class="sprocket-btn delete-btn"
                        on:click={deleteSprocket}
                        disabled={isLoadingSprocket ||
                            !sprocketStatus?.script_exists}
                    >
                        🗑️ Delete Script
                    </button>
                </div>
            </div>

            <div class="sprocket-upload-section">
                <h4>Upload Script</h4>
                <div class="upload-controls">
                    <input
                        type="file"
                        id="sprocket-upload-file"
                        accept=".sh,.bash"
                        on:change={handleSprocketFileSelect}
                        disabled={isLoadingSprocket}
                    />
                    <button
                        class="sprocket-btn upload-btn"
                        on:click={uploadSprocketScript}
                        disabled={isLoadingSprocket || !sprocketUploadFile}
                    >
                        📤 Upload & Update
                    </button>
                </div>
            </div>

            <div class="sprocket-status">
                <div class="status-item">
                    <span class="status-label">Status:</span>
                    <span
                        class="status-value"
                        class:running={sprocketStatus?.is_running}
                    >
                        {sprocketStatus?.is_running
                            ? "🟢 Running"
                            : "🔴 Stopped"}
                    </span>
                </div>
                {#if sprocketStatus?.pid}
                    <div class="status-item">
                        <span class="status-label">PID:</span>
                        <span class="status-value">{sprocketStatus.pid}</span>
                    </div>
                {/if}
                <div class="status-item">
                    <span class="status-label">Script:</span>
                    <span class="status-value"
                        >{sprocketStatus?.script_exists
                            ? "✅ Exists"
                            : "❌ Not found"}</span
                    >
                </div>
            </div>

            <div class="script-editor-container">
                <textarea
                    class="script-editor"
                    bind:value={sprocketScript}
                    placeholder="#!/bin/bash&#10;# Enter your sprocket script here..."
                    disabled={isLoadingSprocket}
                ></textarea>
            </div>

            <div class="script-actions">
                <button
                    class="sprocket-btn save-btn"
                    on:click={saveSprocket}
                    disabled={isLoadingSprocket}
                >
                    💾 Save & Update
                </button>
                <button
                    class="sprocket-btn load-btn"
                    on:click={loadSprocket}
                    disabled={isLoadingSprocket}
                >
                    📥 Load Current
                </button>
            </div>

            {#if sprocketMessage}
                <div
                    class="sprocket-message"
                    class:error={sprocketMessageType === "error"}
                >
                    {sprocketMessage}
                </div>
            {/if}
        </div>

        <div class="sprocket-section">
            <h3>Script Versions</h3>
            <div class="versions-list">
                {#each sprocketVersions as version}
                    <div
                        class="version-item"
                        class:current={version.is_current}
                    >
                        <div class="version-info">
                            <div class="version-name">
                                {version.name}
                            </div>
                            <div class="version-date">
                                {new Date(version.modified).toLocaleString()}
                                {#if version.is_current}
                                    <span class="current-badge">Current</span>
                                {/if}
                            </div>
                        </div>
                        <div class="version-actions">
                            <button
                                class="version-btn load-btn"
                                on:click={() => loadVersion(version)}
                                disabled={isLoadingSprocket}
                            >
                                📥 Load
                            </button>
                            {#if !version.is_current}
                                <button
                                    class="version-btn delete-btn"
                                    on:click={() => deleteVersion(version.name)}
                                    disabled={isLoadingSprocket}
                                >
                                    🗑️ Delete
                                </button>
                            {/if}
                        </div>
                    </div>
                {/each}
            </div>

            <button
                class="sprocket-btn refresh-btn"
                on:click={loadVersions}
                disabled={isLoadingSprocket}
            >
                🔄 Refresh Versions
            </button>
        </div>
    {:else if isLoggedIn}
        <div class="permission-denied">
            <p>❌ Owner permission required for sprocket management.</p>
            <p>
                To enable sprocket functionality, set the <code
                    >ORLY_OWNERS</code
                > environment variable with your npub when starting the relay.
            </p>
            <p>
                Current user role: <strong>{userRole || "none"}</strong>
            </p>
        </div>
    {:else}
        <div class="login-prompt">
            <p>Please log in to access sprocket management.</p>
            <button class="login-btn" on:click={openLoginModal}>Log In</button>
        </div>
    {/if}
</div>

<style>
    .sprocket-view {
        width: 100%;
        max-width: 1200px;
        margin: 0;
        padding: 20px;
        background: var(--header-bg);
        color: var(--text-color);
        border-radius: 8px;
    }

    .sprocket-view h2 {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
        font-size: 1.8rem;
        font-weight: 600;
    }

    .sprocket-section {
        background-color: var(--card-bg);
        border-radius: 8px;
        padding: 1em;
        margin-bottom: 1.5rem;
        border: 1px solid var(--border-color);
        width: 32em;
    }

    .sprocket-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1rem;
    }

    .sprocket-header h3 {
        margin: 0;
        color: var(--text-color);
        font-size: 1.2rem;
        font-weight: 600;
    }

    .sprocket-controls {
        display: flex;
        gap: 0.5rem;
    }

    .sprocket-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        transition: background-color 0.2s;
        display: flex;
        align-items: center;
        gap: 0.25em;
    }

    .sprocket-btn:hover:not(:disabled) {
        background: var(--accent-hover-color);
    }

    .sprocket-btn:disabled {
        background: var(--secondary);
        cursor: not-allowed;
    }

    .restart-btn {
        background: var(--warning);
    }

    .restart-btn:hover:not(:disabled) {
        background: var(--warning);
        filter: brightness(0.9);
    }

    .delete-btn {
        background: var(--danger);
    }

    .delete-btn:hover:not(:disabled) {
        background: var(--danger);
        filter: brightness(0.9);
    }

    .sprocket-upload-section {
        margin-bottom: 1.5rem;
    }

    .sprocket-upload-section h4 {
        margin: 0 0 0.5rem 0;
        color: var(--text-color);
        font-size: 1rem;
        font-weight: 600;
    }

    .upload-controls {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
    }

    #sprocket-upload-file {
        padding: 0.5em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
    }

    .upload-btn {
        background: var(--success);
        align-self: flex-start;
    }

    .upload-btn:hover:not(:disabled) {
        background: var(--success);
        filter: brightness(0.9);
    }

    .sprocket-status {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
        margin-bottom: 1.5rem;
        padding: 1rem;
        background: var(--bg-color);
        border-radius: 4px;
        border: 1px solid var(--border-color);
    }

    .status-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .status-label {
        font-weight: 600;
        color: var(--text-color);
    }

    .status-value {
        color: var(--text-color);
    }

    .status-value.running {
        color: var(--success);
    }

    .script-editor-container {
        margin-bottom: 1.5rem;
    }

    .script-editor {
        width: 100%;
        height: 300px;
        padding: 1em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-family: monospace;
        font-size: 0.9em;
        line-height: 1.4;
        resize: vertical;
    }

    .script-editor:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .script-actions {
        display: flex;
        gap: 0.5rem;
        margin-bottom: 1rem;
    }

    .save-btn {
        background: var(--success);
    }

    .save-btn:hover:not(:disabled) {
        background: var(--success);
        filter: brightness(0.9);
    }

    .load-btn {
        background: var(--info);
    }

    .load-btn:hover:not(:disabled) {
        background: var(--info);
        filter: brightness(0.9);
    }

    .sprocket-message {
        padding: 1rem;
        border-radius: 4px;
        margin-top: 1rem;
        background: var(--success-bg);
        color: var(--success-text);
        border: 1px solid var(--success);
    }

    .sprocket-message.error {
        background: var(--danger-bg);
        color: var(--danger-text);
        border: 1px solid var(--danger);
    }

    .versions-list {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
        margin-bottom: 1rem;
    }

    .version-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.75rem;
        background: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 4px;
    }

    .version-item.current {
        border-color: var(--primary);
        background: var(--primary-bg);
    }

    .version-info {
        flex: 1;
    }

    .version-name {
        font-weight: 600;
        color: var(--text-color);
        margin-bottom: 0.25rem;
    }

    .version-date {
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.7;
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .current-badge {
        background: var(--primary);
        color: var(--text-color);
        padding: 0.1em 0.4em;
        border-radius: 0.25rem;
        font-size: 0.7em;
        font-weight: 600;
    }

    .version-actions {
        display: flex;
        gap: 0.25rem;
    }

    .version-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.25em 0.5em;
        border-radius: 0.25rem;
        cursor: pointer;
        font-size: 0.8em;
        transition: background-color 0.2s;
    }

    .version-btn:hover:not(:disabled) {
        background: var(--accent-hover-color);
    }

    .version-btn:disabled {
        background: var(--secondary);
        cursor: not-allowed;
    }

    .version-btn.delete-btn {
        background: var(--danger);
    }

    .version-btn.delete-btn:hover:not(:disabled) {
        background: var(--danger);
        filter: brightness(0.9);
    }

    .refresh-btn {
        background: var(--info);
    }

    .refresh-btn:hover:not(:disabled) {
        background: var(--info);
        filter: brightness(0.9);
    }

    .permission-denied,
    .login-prompt {
        text-align: center;
        padding: 2em;
        background-color: var(--card-bg);
        border-radius: 8px;
        border: 1px solid var(--border-color);
        color: var(--text-color);
    }

    .permission-denied p,
    .login-prompt p {
        margin: 0 0 1rem 0;
        line-height: 1.4;
    }

    .permission-denied code {
        background: var(--code-bg);
        padding: 0.2em 0.4em;
        border-radius: 0.25rem;
        font-family: monospace;
        font-size: 0.9em;
    }

    .login-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.75em 1.5em;
        border-radius: 4px;
        cursor: pointer;
        font-weight: bold;
        font-size: 0.9em;
        transition: background-color 0.2s;
    }

    .login-btn:hover {
        background: var(--accent-hover-color);
    }
</style>
