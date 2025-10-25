<script>
    import LoginModal from "./LoginModal.svelte";
    import ManagedACL from "./ManagedACL.svelte";
    import {
        initializeNostrClient,
        fetchUserProfile,
        fetchAllEvents,
        fetchUserEvents,
        searchEvents,
        fetchEventById,
        fetchDeleteEventsByTarget,
        nostrClient,
        NostrClient,
        Nip07Signer,
        PrivateKeySigner,
    } from "./nostr.js";
    import { publishEventWithAuth } from "./websocket-auth.js";

    let isDarkTheme = false;
    let showLoginModal = false;
    let isLoggedIn = false;
    let userPubkey = "";
    let authMethod = "";
    let userProfile = null;
    let userRole = "";
    let userSigner = null;
    let showSettingsDrawer = false;
    let selectedTab = "export";
    let isSearchMode = false;
    let searchQuery = "";
    let searchTabs = [];
    let allEvents = [];
    let selectedFile = null;
    let expandedEvents = new Set();
    let isLoadingEvents = false;
    let hasMoreEvents = true;
    let eventsPerPage = 100;
    let oldestEventTimestamp = null; // For timestamp-based pagination
    let newestEventTimestamp = null; // For loading newer events

    // Search results state
    let searchResults = new Map(); // Map of searchTabId -> { events, isLoading, hasMore, oldestTimestamp }
    let isLoadingSearch = false;

    // Screen-filling events view state
    let eventsPerScreen = 20; // Default, will be calculated based on screen size

    // Global events cache system
    let globalEventsCache = []; // All events cache
    let globalCacheTimestamp = 0;
    const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes

    // Events filter toggle
    let showOnlyMyEvents = false;

    // My Events state
    let myEvents = [];
    let isLoadingMyEvents = false;
    let hasMoreMyEvents = true;
    let oldestMyEventTimestamp = null;
    let newestMyEventTimestamp = null;

    // Sprocket management state
    let sprocketScript = "";
    let sprocketStatus = null;
    let sprocketVersions = [];
    let isLoadingSprocket = false;
    let sprocketMessage = "";
    let sprocketMessageType = "info";
    let sprocketEnabled = false;
    let sprocketUploadFile = null;

    // ACL mode
    let aclMode = "";

    // Compose tab state
    let composeEventJson = "";

    // Recovery tab state
    let recoverySelectedKind = null;
    let recoveryCustomKind = "";
    let recoveryEvents = [];
    let isLoadingRecovery = false;
    let recoveryHasMore = true;
    let recoveryOldestTimestamp = null;
    let recoveryNewestTimestamp = null;

    // Replaceable kinds for the recovery dropdown
    const replaceableKinds = [
        { value: 0, label: "Profile Metadata (0)" },
        { value: 3, label: "Follow List (3)" },
        { value: 10000, label: "Relay List Metadata (10000)" },
        { value: 10001, label: "Mute List (10001)" },
        { value: 10002, label: "Pin List (10002)" },
        { value: 10003, label: "Bookmark List (10003)" },
        { value: 10004, label: "Communities List (10004)" },
        { value: 10005, label: "Public Chats List (10005)" },
        { value: 10006, label: "Blocked Relays List (10006)" },
        { value: 10007, label: "Search Relays List (10007)" },
        { value: 10015, label: "Interests List (10015)" },
        { value: 10030, label: "Emoji Sets (10030)" },
        { value: 30000, label: "Categorized People List (30000)" },
        { value: 30001, label: "Categorized Bookmark List (30001)" },
        { value: 30002, label: "Relay Sets (30002)" },
        { value: 30003, label: "Bookmark Sets (30003)" },
        { value: 30004, label: "Curation Sets (30004)" },
        { value: 30008, label: "Profile Badges (30008)" },
        { value: 30009, label: "Badge Definition (30009)" },
        { value: 30015, label: "Interest Sets (30015)" },
        { value: 30017, label: "Stall Definition (30017)" },
        { value: 30018, label: "Product Definition (30018)" },
        { value: 30019, label: "Marketplace UI/UX (30019)" },
        { value: 30020, label: "Product Sold as Auction (30020)" },
        { value: 30023, label: "Article (30023)" },
        { value: 30024, label: "Draft Long-form Content (30024)" },
        { value: 30030, label: "Emoji Sets (30030)" },
        { value: 30078, label: "Application Specific Data (30078)" },
        { value: 30311, label: "Live Event (30311)" },
        { value: 30315, label: "User Statuses (30315)" },
        { value: 30402, label: "Classified Listing (30402)" },
        { value: 30403, label: "Draft Classified Listing (30403)" },
        { value: 31922, label: "Date-based Calendar Event (31922)" },
        { value: 31923, label: "Time-based Calendar Event (31923)" },
        { value: 31924, label: "Calendar (31924)" },
        { value: 31925, label: "Calendar Event RSVP (31925)" },
        { value: 31989, label: "Handler Recommendation (31989)" },
        { value: 31990, label: "Handler Information (31990)" },
        { value: 32123, label: "WaveLake Track (32123)" },
        { value: 34550, label: "Community Definition (34550)" },
    ];

    // Kind name mapping based on NIP specification
    // Matches official Nostr event kinds from https://github.com/nostr-protocol/nips
    const kindNames = {
        0: "User Metadata",
        1: "Short Text Note",
        2: "Recommend Relay",
        3: "Follows",
        4: "Encrypted Direct Messages",
        5: "Event Deletion Request",
        6: "Repost",
        7: "Reaction",
        8: "Badge Award",
        9: "Chat Message",
        10: "Group Chat Threaded Reply",
        11: "Thread",
        12: "Group Thread Reply",
        13: "Seal",
        14: "Direct Message",
        15: "File Message",
        16: "Generic Repost",
        17: "Reaction to a website",
        20: "Picture",
        40: "Channel Creation",
        41: "Channel Metadata",
        42: "Channel Message",
        43: "Channel Hide Message",
        44: "Channel Mute User",
        1021: "Bid",
        1022: "Bid Confirmation",
        1040: "OpenTimestamps",
        1063: "File Metadata",
        1311: "Live Chat Message",
        1971: "Problem Tracker",
        1984: "Reporting",
        1985: "Label",
        4550: "Community Post Approval",
        5000: "Job Request",
        5999: "Job Request",
        6000: "Job Result",
        6999: "Job Result",
        7000: "Job Feedback",
        9041: "Zap Goal",
        9734: "Zap Request",
        9735: "Zap",
        9882: "Highlights",
        10000: "Mute list",
        10001: "Pin list",
        10002: "Relay List Metadata",
        10003: "Bookmarks list",
        10004: "Communities list",
        10005: "Public Chats list",
        10006: "Blocked Relays list",
        10007: "Search Relays list",
        10015: "Interests",
        10030: "User Emoji list",
        10050: "DM relays",
        10096: "File Storage Server List",
        13194: "Wallet Service Info",
        21000: "Lightning pub RPC",
        22242: "Client Authentication",
        23194: "Wallet Request",
        23195: "Wallet Response",
        23196: "Wallet Notification",
        23197: "Wallet Notification",
        24133: "Nostr Connect",
        27235: "HTTP Auth",
        30000: "Follow sets",
        30001: "Generic lists",
        30002: "Relay sets",
        30003: "Bookmark sets",
        30004: "Curation sets",
        30008: "Profile Badges",
        30009: "Badge Definition",
        30015: "Interest sets",
        30017: "Stall Definition",
        30018: "Product Definition",
        30019: "Marketplace UI/UX",
        30020: "Product sold as an auction",
        30023: "Long-form Content",
        30024: "Draft Long-form Content",
        30030: "Emoji sets",
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
        34235: "Video Event Horizontal",
        34236: "Video Event Vertical",
        34550: "Community Definition",
    };

    function getKindName(kind) {
        return kindNames[kind] || `Kind ${kind}`;
    }

    function truncatePubkey(pubkey) {
        if (!pubkey) return "unknown";
        return pubkey.slice(0, 8) + "..." + pubkey.slice(-8);
    }

    function truncateContent(content, maxLength = 100) {
        if (!content) return "";
        return content.length > maxLength
            ? content.slice(0, maxLength) + "..."
            : content;
    }

    function formatTimestamp(timestamp) {
        if (!timestamp) return "";
        return new Date(timestamp * 1000).toLocaleString();
    }

    function toggleEventExpansion(eventId) {
        if (expandedEvents.has(eventId)) {
            expandedEvents.delete(eventId);
        } else {
            expandedEvents.add(eventId);
        }
        expandedEvents = expandedEvents; // Trigger reactivity
    }

    async function copyEventToClipboard(eventData, clickEvent) {
        try {
            // Create minified JSON (no indentation)
            const minifiedJson = JSON.stringify(eventData);
            await navigator.clipboard.writeText(minifiedJson);

            // Show temporary feedback
            const button = clickEvent.target.closest(".copy-json-btn");
            if (button) {
                const originalText = button.textContent;
                button.textContent = "✅";
                button.style.backgroundColor = "#4CAF50";
                setTimeout(() => {
                    button.textContent = originalText;
                    button.style.backgroundColor = "";
                }, 2000);
            }
        } catch (error) {
            console.error("Failed to copy to clipboard:", error);
            // Fallback for older browsers
            try {
                const textArea = document.createElement("textarea");
                textArea.value = JSON.stringify(eventData);
                document.body.appendChild(textArea);
                textArea.select();
                document.execCommand("copy");
                document.body.removeChild(textArea);

                const button = clickEvent.target.closest(".copy-json-btn");
                if (button) {
                    const originalText = button.textContent;
                    button.textContent = "✅";
                    button.style.backgroundColor = "#4CAF50";
                    setTimeout(() => {
                        button.textContent = originalText;
                        button.style.backgroundColor = "";
                    }, 2000);
                }
            } catch (fallbackError) {
                console.error("Fallback copy also failed:", fallbackError);
                alert("Failed to copy to clipboard. Please copy manually.");
            }
        }
    }

    async function handleToggleChange() {
        // Toggle state is already updated by bind:checked
        console.log("Toggle changed, showOnlyMyEvents:", showOnlyMyEvents);

        // Reload events with the new filter
        const authors =
            showOnlyMyEvents && isLoggedIn && userPubkey ? [userPubkey] : null;
        await loadAllEvents(true, authors);
    }

    // Events are filtered server-side, but add client-side filtering as backup
    // Sort events by created_at timestamp (newest first)
    $: filteredEvents = (
        showOnlyMyEvents && isLoggedIn && userPubkey
            ? allEvents.filter(
                  (event) => event.pubkey && event.pubkey === userPubkey,
              )
            : allEvents
    ).sort((a, b) => b.created_at - a.created_at);

    async function deleteEvent(eventId) {
        if (!isLoggedIn) {
            alert("Please log in first");
            return;
        }

        // Find the event to check if user can delete it
        const event = allEvents.find((e) => e.id === eventId);
        if (!event) {
            alert("Event not found");
            return;
        }

        // Check permissions: admin/owner can delete any event, write users can only delete their own events
        const canDelete =
            userRole === "admin" ||
            userRole === "owner" ||
            (userRole === "write" &&
                event.pubkey &&
                event.pubkey === userPubkey);

        if (!canDelete) {
            alert("You do not have permission to delete this event");
            return;
        }

        if (!confirm("Are you sure you want to delete this event?")) {
            return;
        }

        try {
            // Check if signer is available
            if (!userSigner) {
                throw new Error("Signer not available for signing");
            }

            // Create the delete event template (unsigned)
            const deleteEventTemplate = {
                kind: 5,
                created_at: Math.floor(Date.now() / 1000),
                tags: [["e", eventId]], // e-tag referencing the event to delete
                content: "",
                // Don't set pubkey - let the signer set it
            };

            console.log("Created delete event template:", deleteEventTemplate);
            console.log("User pubkey:", userPubkey);
            console.log("Target event:", event);
            console.log("Target event pubkey:", event.pubkey);

            // Sign the event using the signer
            const signedDeleteEvent =
                await userSigner.signEvent(deleteEventTemplate);
            console.log("Signed delete event:", signedDeleteEvent);
            console.log(
                "Signed delete event pubkey:",
                signedDeleteEvent.pubkey,
            );
            console.log("Delete event tags:", signedDeleteEvent.tags);

            // Publish to the ORLY relay using WebSocket authentication
            const relayUrl = `wss://${window.location.host}`;

            try {
                const result = await publishEventWithAuth(
                    relayUrl,
                    signedDeleteEvent,
                    userSigner,
                    userPubkey,
                );

                if (result.success) {
                    console.log(
                        "Delete event published successfully to ORLY relay",
                    );
                } else {
                    console.error(
                        "Failed to publish delete event:",
                        result.reason,
                    );
                }
            } catch (error) {
                console.error("Error publishing delete event:", error);
            }

            // Determine if we should publish to external relays
            // Only publish to external relays if:
            // 1. User is deleting their own event, OR
            // 2. User is admin/owner AND deleting their own event
            const isDeletingOwnEvent =
                event.pubkey && event.pubkey === userPubkey;
            const isAdminOrOwner = userRole === "admin" || userRole === "owner";
            const shouldPublishToExternalRelays = isDeletingOwnEvent;

            if (shouldPublishToExternalRelays) {
                // Publish the delete event to all relays (including external ones)
                const result = await nostrClient.publish(signedDeleteEvent);
                console.log("Delete event published:", result);

                if (result.success && result.okCount > 0) {
                    // Wait a moment for the deletion to propagate
                    await new Promise((resolve) => setTimeout(resolve, 2000));

                    // Verify the event was actually deleted by trying to fetch it
                    try {
                        const deletedEvent = await fetchEventById(eventId, {
                            timeout: 5000,
                        });
                        if (deletedEvent) {
                            console.warn(
                                "Event still exists after deletion attempt:",
                                deletedEvent,
                            );
                            alert(
                                `Warning: Delete event was accepted by ${result.okCount} relay(s), but the event still exists on the relay. This may indicate the relay does not properly handle delete events.`,
                            );
                        } else {
                            console.log(
                                "Event successfully deleted and verified",
                            );
                        }
                    } catch (fetchError) {
                        console.log(
                            "Could not fetch event after deletion (likely deleted):",
                            fetchError.message,
                        );
                    }

                    // Also verify that the delete event has been saved
                    try {
                        const deleteEvents = await fetchDeleteEventsByTarget(
                            eventId,
                            { timeout: 5000 },
                        );
                        if (deleteEvents.length > 0) {
                            console.log(
                                `Delete event verification: Found ${deleteEvents.length} delete event(s) targeting ${eventId}`,
                            );
                            // Check if our delete event is among them
                            const ourDeleteEvent = deleteEvents.find(
                                (de) => de.pubkey && de.pubkey === userPubkey,
                            );
                            if (ourDeleteEvent) {
                                console.log(
                                    "Our delete event found in database:",
                                    ourDeleteEvent.id,
                                );
                            } else {
                                console.warn(
                                    "Our delete event not found in database, but other delete events exist",
                                );
                            }
                        } else {
                            console.warn(
                                "No delete events found in database for target event:",
                                eventId,
                            );
                        }
                    } catch (deleteFetchError) {
                        console.log(
                            "Could not verify delete event in database:",
                            deleteFetchError.message,
                        );
                    }

                    // Remove from local lists
                    allEvents = allEvents.filter(
                        (event) => event.id !== eventId,
                    );
                    myEvents = myEvents.filter((event) => event.id !== eventId);

                    // Remove from global cache
                    globalEventsCache = globalEventsCache.filter(
                        (event) => event.id !== eventId,
                    );

                    // Remove from search results cache
                    for (const [tabId, searchResult] of searchResults) {
                        if (searchResult.events) {
                            searchResult.events = searchResult.events.filter(
                                (event) => event.id !== eventId,
                            );
                            searchResults.set(tabId, searchResult);
                        }
                    }

                    // Update persistent state
                    savePersistentState();

                    // Reload events to show the new delete event at the top
                    console.log("Reloading events to show delete event...");
                    const authors =
                        showOnlyMyEvents && isLoggedIn && userPubkey
                            ? [userPubkey]
                            : null;
                    await loadAllEvents(true, authors);

                    alert(
                        `Event deleted successfully (accepted by ${result.okCount} relay(s))`,
                    );
                } else {
                    throw new Error("No relays accepted the delete event");
                }
            } else {
                // Admin/owner deleting someone else's event - only publish to local relay
                // We need to publish only to the local relay, not external ones
                const localRelayUrl = `wss://${window.location.host}/`;

                // Create a modified client that only connects to the local relay
                const localClient = new NostrClient();
                await localClient.connectToRelay(localRelayUrl);

                const result = await localClient.publish(signedDeleteEvent);
                console.log(
                    "Delete event published to local relay only:",
                    result,
                );

                if (result.success && result.okCount > 0) {
                    // Wait a moment for the deletion to propagate
                    await new Promise((resolve) => setTimeout(resolve, 2000));

                    // Verify the event was actually deleted by trying to fetch it
                    try {
                        const deletedEvent = await fetchEventById(eventId, {
                            timeout: 5000,
                        });
                        if (deletedEvent) {
                            console.warn(
                                "Event still exists after deletion attempt:",
                                deletedEvent,
                            );
                            alert(
                                `Warning: Delete event was accepted by ${result.okCount} relay(s), but the event still exists on the relay. This may indicate the relay does not properly handle delete events.`,
                            );
                        } else {
                            console.log(
                                "Event successfully deleted and verified",
                            );
                        }
                    } catch (fetchError) {
                        console.log(
                            "Could not fetch event after deletion (likely deleted):",
                            fetchError.message,
                        );
                    }

                    // Also verify that the delete event has been saved
                    try {
                        const deleteEvents = await fetchDeleteEventsByTarget(
                            eventId,
                            { timeout: 5000 },
                        );
                        if (deleteEvents.length > 0) {
                            console.log(
                                `Delete event verification: Found ${deleteEvents.length} delete event(s) targeting ${eventId}`,
                            );
                            // Check if our delete event is among them
                            const ourDeleteEvent = deleteEvents.find(
                                (de) => de.pubkey && de.pubkey === userPubkey,
                            );
                            if (ourDeleteEvent) {
                                console.log(
                                    "Our delete event found in database:",
                                    ourDeleteEvent.id,
                                );
                            } else {
                                console.warn(
                                    "Our delete event not found in database, but other delete events exist",
                                );
                            }
                        } else {
                            console.warn(
                                "No delete events found in database for target event:",
                                eventId,
                            );
                        }
                    } catch (deleteFetchError) {
                        console.log(
                            "Could not verify delete event in database:",
                            deleteFetchError.message,
                        );
                    }

                    // Remove from local lists
                    allEvents = allEvents.filter(
                        (event) => event.id !== eventId,
                    );
                    myEvents = myEvents.filter((event) => event.id !== eventId);

                    // Remove from global cache
                    globalEventsCache = globalEventsCache.filter(
                        (event) => event.id !== eventId,
                    );

                    // Remove from search results cache
                    for (const [tabId, searchResult] of searchResults) {
                        if (searchResult.events) {
                            searchResult.events = searchResult.events.filter(
                                (event) => event.id !== eventId,
                            );
                            searchResults.set(tabId, searchResult);
                        }
                    }

                    // Update persistent state
                    savePersistentState();

                    // Reload events to show the new delete event at the top
                    console.log("Reloading events to show delete event...");
                    const authors =
                        showOnlyMyEvents && isLoggedIn && userPubkey
                            ? [userPubkey]
                            : null;
                    await loadAllEvents(true, authors);

                    alert(
                        `Event deleted successfully (local relay only - admin/owner deleting other user's event)`,
                    );
                } else {
                    throw new Error(
                        "Local relay did not accept the delete event",
                    );
                }
            }
        } catch (error) {
            console.error("Failed to delete event:", error);
            alert("Failed to delete event: " + error.message);
        }
    }

    // Safely render "about" text: convert double newlines to a single HTML line break
    function escapeHtml(str) {
        return String(str)
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;")
            .replace(/'/g, "&#39;");
    }

    // Recovery tab functions
    async function loadRecoveryEvents() {
        const kindToUse = recoveryCustomKind
            ? parseInt(recoveryCustomKind)
            : recoverySelectedKind;
        if (!kindToUse || !isLoggedIn) return;

        console.log(
            "Loading recovery events for kind:",
            kindToUse,
            "user:",
            userPubkey,
        );
        isLoadingRecovery = true;
        try {
            // Fetch multiple versions using limit parameter
            // For replaceable events, limit > 1 returns multiple versions
            const filters = {
                kinds: [kindToUse],
                authors: [userPubkey],
                limit: 100, // Get up to 100 versions
            };

            if (recoveryOldestTimestamp) {
                filters.until = recoveryOldestTimestamp;
            }

            console.log("Recovery filters:", filters);
            const events = await fetchAllEvents(filters);
            console.log("Recovery events received:", events.length);
            console.log(
                "Recovery events kinds:",
                events.map((e) => e.kind),
            );

            if (recoveryOldestTimestamp) {
                // Append to existing events
                recoveryEvents = [...recoveryEvents, ...events];
            } else {
                // Replace events
                recoveryEvents = events;
            }

            if (events.length > 0) {
                recoveryOldestTimestamp = Math.min(
                    ...events.map((e) => e.created_at),
                );
                recoveryHasMore = events.length === 100;
            } else {
                recoveryHasMore = false;
            }
        } catch (error) {
            console.error("Failed to load recovery events:", error);
        } finally {
            isLoadingRecovery = false;
        }
    }

    async function repostEvent(event) {
        if (!isLoggedIn || !userSigner) {
            alert("Please log in to repost events");
            return;
        }

        try {
            // Create a new event with the same content but current timestamp
            const newEvent = {
                kind: event.kind,
                content: event.content,
                tags: event.tags,
                created_at: Math.floor(Date.now() / 1000),
                pubkey: userPubkey,
            };

            // Sign and publish the event
            const result = await publishEventWithAuth(
                DEFAULT_RELAYS[0],
                newEvent,
                userSigner,
                userPubkey,
            );

            if (result.success) {
                alert("Event reposted successfully!");
                // Reload the recovery events to show the new current version
                recoveryEvents = [];
                recoveryOldestTimestamp = null;
                await loadRecoveryEvents();
            } else {
                alert("Failed to repost event");
            }
        } catch (error) {
            console.error("Failed to repost event:", error);
            alert("Failed to repost event: " + error.message);
        }
    }

    function selectRecoveryKind(kind) {
        recoverySelectedKind = kind;
        recoveryCustomKind = ""; // Clear custom kind when selecting from dropdown
        recoveryEvents = [];
        recoveryOldestTimestamp = null;
        recoveryHasMore = true;
        loadRecoveryEvents();
    }

    function handleCustomKindInput() {
        if (recoveryCustomKind) {
            recoverySelectedKind = null; // Clear dropdown selection when using custom
            recoveryEvents = [];
            recoveryOldestTimestamp = null;
            recoveryHasMore = true;
            loadRecoveryEvents();
        }
    }

    function isCurrentVersion(event) {
        // Find all events with the same kind and pubkey
        const sameKindEvents = recoveryEvents.filter(
            (e) => e.kind === event.kind && e.pubkey === event.pubkey,
        );

        // Check if this event has the highest timestamp
        const maxTimestamp = Math.max(
            ...sameKindEvents.map((e) => e.created_at),
        );
        return event.created_at === maxTimestamp;
    }

    // Always show all versions - no filtering needed

    $: aboutHtml = userProfile?.about
        ? escapeHtml(userProfile.about).replace(/\n{2,}/g, "<br>")
        : "";

    // Load theme preference from localStorage on component initialization
    if (typeof localStorage !== "undefined") {
        const savedTheme = localStorage.getItem("isDarkTheme");
        if (savedTheme !== null) {
            isDarkTheme = JSON.parse(savedTheme);
        }

        // Check for existing authentication
        const storedAuthMethod = localStorage.getItem("nostr_auth_method");
        const storedPubkey = localStorage.getItem("nostr_pubkey");

        if (storedAuthMethod && storedPubkey) {
            isLoggedIn = true;
            userPubkey = storedPubkey;
            authMethod = storedAuthMethod;

            // Restore signer for extension method
            if (storedAuthMethod === "extension" && window.nostr) {
                userSigner = window.nostr;
            }

            // Fetch user role for already logged in users
            fetchUserRole();
            fetchACLMode();
        }

        // Load persistent app state
        loadPersistentState();

        // Load sprocket configuration
        loadSprocketConfig();
    }

    function savePersistentState() {
        if (typeof localStorage === "undefined") return;

        const state = {
            selectedTab,
            expandedEvents: Array.from(expandedEvents),
            globalEventsCache,
            globalCacheTimestamp,
            hasMoreEvents,
            oldestEventTimestamp,
        };

        localStorage.setItem("app_state", JSON.stringify(state));
    }

    function loadPersistentState() {
        if (typeof localStorage === "undefined") return;

        try {
            const savedState = localStorage.getItem("app_state");
            if (savedState) {
                const state = JSON.parse(savedState);

                // Restore tab state
                if (
                    state.selectedTab &&
                    baseTabs.some((tab) => tab.id === state.selectedTab)
                ) {
                    selectedTab = state.selectedTab;
                }

                // Restore expanded events
                if (state.expandedEvents) {
                    expandedEvents = new Set(state.expandedEvents);
                }

                // Restore cache data

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
            console.error("Failed to load persistent state:", error);
        }
    }

    function restoreEventsFromCache() {
        // Restore global events cache
        if (
            globalEventsCache.length > 0 &&
            isCacheValid(globalCacheTimestamp)
        ) {
            allEvents = globalEventsCache;
        }
    }

    function isCacheValid(timestamp) {
        if (!timestamp) return false;
        return Date.now() - timestamp < CACHE_DURATION;
    }

    function updateGlobalCache(events) {
        globalEventsCache = events.sort((a, b) => b.created_at - a.created_at);
        globalCacheTimestamp = Date.now();
        savePersistentState();
    }

    function clearCache() {
        globalEventsCache = [];
        globalCacheTimestamp = 0;
        savePersistentState();
    }

    // Sprocket management functions
    async function loadSprocketConfig() {
        try {
            const response = await fetch("/api/sprocket/config", {
                method: "GET",
                headers: {
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                const config = await response.json();
                sprocketEnabled = config.enabled;
            }
        } catch (error) {
            console.error("Error loading sprocket config:", error);
        }
    }

    async function loadSprocketStatus() {
        if (!isLoggedIn || userRole !== "owner" || !sprocketEnabled) return;

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/status", {
                method: "GET",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("GET", "/api/sprocket/status")}`,
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                sprocketStatus = await response.json();
            } else {
                showSprocketMessage("Failed to load sprocket status", "error");
            }
        } catch (error) {
            showSprocketMessage(
                `Error loading sprocket status: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function loadSprocket() {
        if (!isLoggedIn || userRole !== "owner") return;

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/status", {
                method: "GET",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("GET", "/api/sprocket/status")}`,
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                const status = await response.json();
                sprocketScript = status.script_content || "";
                sprocketStatus = status;
                showSprocketMessage("Script loaded successfully", "success");
            } else {
                showSprocketMessage("Failed to load script", "error");
            }
        } catch (error) {
            showSprocketMessage(
                `Error loading script: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function saveSprocket() {
        if (!isLoggedIn || userRole !== "owner") return;

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/update", {
                method: "POST",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("POST", "/api/sprocket/update")}`,
                    "Content-Type": "text/plain",
                },
                body: sprocketScript,
            });

            if (response.ok) {
                showSprocketMessage(
                    "Script saved and updated successfully",
                    "success",
                );
                await loadSprocketStatus();
                await loadVersions();
            } else {
                const errorText = await response.text();
                showSprocketMessage(
                    `Failed to save script: ${errorText}`,
                    "error",
                );
            }
        } catch (error) {
            showSprocketMessage(
                `Error saving script: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function restartSprocket() {
        if (!isLoggedIn || userRole !== "owner") return;

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/restart", {
                method: "POST",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("POST", "/api/sprocket/restart")}`,
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                showSprocketMessage(
                    "Sprocket restarted successfully",
                    "success",
                );
                await loadSprocketStatus();
            } else {
                const errorText = await response.text();
                showSprocketMessage(
                    `Failed to restart sprocket: ${errorText}`,
                    "error",
                );
            }
        } catch (error) {
            showSprocketMessage(
                `Error restarting sprocket: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function deleteSprocket() {
        if (!isLoggedIn || userRole !== "owner") return;

        if (
            !confirm(
                "Are you sure you want to delete the sprocket script? This will stop the current process.",
            )
        ) {
            return;
        }

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/update", {
                method: "POST",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("POST", "/api/sprocket/update")}`,
                    "Content-Type": "text/plain",
                },
                body: "", // Empty body deletes the script
            });

            if (response.ok) {
                sprocketScript = "";
                showSprocketMessage(
                    "Sprocket script deleted successfully",
                    "success",
                );
                await loadSprocketStatus();
                await loadVersions();
            } else {
                const errorText = await response.text();
                showSprocketMessage(
                    `Failed to delete script: ${errorText}`,
                    "error",
                );
            }
        } catch (error) {
            showSprocketMessage(
                `Error deleting script: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function loadVersions() {
        if (!isLoggedIn || userRole !== "owner") return;

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/versions", {
                method: "GET",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("GET", "/api/sprocket/versions")}`,
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                sprocketVersions = await response.json();
            } else {
                showSprocketMessage("Failed to load versions", "error");
            }
        } catch (error) {
            showSprocketMessage(
                `Error loading versions: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function loadVersion(version) {
        if (!isLoggedIn || userRole !== "owner") return;

        sprocketScript = version.content;
        showSprocketMessage(`Loaded version: ${version.name}`, "success");
    }

    async function deleteVersion(filename) {
        if (!isLoggedIn || userRole !== "owner") return;

        if (!confirm(`Are you sure you want to delete version ${filename}?`)) {
            return;
        }

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/delete-version", {
                method: "POST",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("POST", "/api/sprocket/delete-version")}`,
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ filename }),
            });

            if (response.ok) {
                showSprocketMessage(
                    `Version ${filename} deleted successfully`,
                    "success",
                );
                await loadVersions();
            } else {
                const errorText = await response.text();
                showSprocketMessage(
                    `Failed to delete version: ${errorText}`,
                    "error",
                );
            }
        } catch (error) {
            showSprocketMessage(
                `Error deleting version: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    function showSprocketMessage(message, type = "info") {
        sprocketMessage = message;
        sprocketMessageType = type;

        // Auto-hide message after 5 seconds
        setTimeout(() => {
            sprocketMessage = "";
        }, 5000);
    }

    function handleSprocketFileSelect(event) {
        sprocketUploadFile = event.target.files[0];
    }

    async function uploadSprocketScript() {
        if (!isLoggedIn || userRole !== "owner" || !sprocketUploadFile) return;

        try {
            isLoadingSprocket = true;

            // Read the file content
            const fileContent = await sprocketUploadFile.text();

            // Upload the script
            const response = await fetch("/api/sprocket/update", {
                method: "POST",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("POST", "/api/sprocket/update")}`,
                    "Content-Type": "text/plain",
                },
                body: fileContent,
            });

            if (response.ok) {
                sprocketScript = fileContent;
                showSprocketMessage(
                    "Script uploaded and updated successfully",
                    "success",
                );
                await loadSprocketStatus();
                await loadVersions();
            } else {
                const errorText = await response.text();
                showSprocketMessage(
                    `Failed to upload script: ${errorText}`,
                    "error",
                );
            }
        } catch (error) {
            showSprocketMessage(
                `Error uploading script: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
            sprocketUploadFile = null;
            // Clear the file input
            const fileInput = document.getElementById("sprocket-upload-file");
            if (fileInput) {
                fileInput.value = "";
            }
        }
    }

    const baseTabs = [
        { id: "export", icon: "📤", label: "Export" },
        { id: "import", icon: "💾", label: "Import", requiresAdmin: true },
        { id: "events", icon: "📡", label: "Events" },
        { id: "compose", icon: "✏️", label: "Compose" },
        { id: "recovery", icon: "🔄", label: "Recovery" },
        {
            id: "managed-acl",
            icon: "🛡️",
            label: "Managed ACL",
            requiresOwner: true,
        },
        { id: "sprocket", icon: "⚙️", label: "Sprocket", requiresOwner: true },
    ];

    // Filter tabs based on user permissions
    $: filteredBaseTabs = baseTabs.filter((tab) => {
        if (
            tab.requiresAdmin &&
            (!isLoggedIn || (userRole !== "admin" && userRole !== "owner"))
        ) {
            return false;
        }
        if (tab.requiresOwner && (!isLoggedIn || userRole !== "owner")) {
            return false;
        }
        // Hide sprocket tab if not enabled
        if (tab.id === "sprocket" && !sprocketEnabled) {
            return false;
        }
        // Hide managed ACL tab if not in managed mode
        if (tab.id === "managed-acl" && aclMode !== "managed") {
            return false;
        }
        // Debug logging for managed ACL tab
        if (tab.id === "managed-acl") {
            console.log("Managed ACL tab check:", {
                isLoggedIn,
                userRole,
                requiresOwner: tab.requiresOwner,
                aclMode,
            });
        }
        return true;
    });

    $: tabs = [...filteredBaseTabs, ...searchTabs];

    // Debug logging for tabs
    $: console.log("Tabs debug:", {
        isLoggedIn,
        userRole,
        aclMode,
        filteredBaseTabs: filteredBaseTabs.map((t) => t.id),
        allTabs: tabs.map((t) => t.id),
    });

    function selectTab(tabId) {
        selectedTab = tabId;

        // Load sprocket data when switching to sprocket tab
        if (
            tabId === "sprocket" &&
            isLoggedIn &&
            userRole === "owner" &&
            sprocketEnabled
        ) {
            loadSprocketStatus();
            loadVersions();
        }

        savePersistentState();
    }

    function toggleTheme() {
        isDarkTheme = !isDarkTheme;
        // Save theme preference to localStorage
        if (typeof localStorage !== "undefined") {
            localStorage.setItem("isDarkTheme", JSON.stringify(isDarkTheme));
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

            // Set up NDK signer based on authentication method
            if (method === "extension" && signer) {
                // Extension signer (NIP-07 compatible)
                nostrClient.setSigner(signer);
            } else if (method === "nsec" && privateKey) {
                // Private key signer for nsec
                const keySigner = new PrivateKeySigner(privateKey);
                nostrClient.setSigner(keySigner);
            }

            userProfile = await fetchUserProfile(pubkey);
            console.log("Profile loaded:", userProfile);
        } catch (error) {
            console.error("Failed to load profile:", error);
        }

        // Fetch user role/permissions
        await fetchUserRole();
        await fetchACLMode();
    }

    function handleLogout() {
        isLoggedIn = false;
        userPubkey = "";
        authMethod = "";
        userProfile = null;
        userRole = "";
        userSigner = null;
        showSettingsDrawer = false;

        // Clear events
        myEvents = [];
        allEvents = [];

        // Clear cache
        clearCache();

        // Clear stored authentication
        if (typeof localStorage !== "undefined") {
            localStorage.removeItem("nostr_auth_method");
            localStorage.removeItem("nostr_pubkey");
            localStorage.removeItem("nostr_privkey");
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
            searchQuery = "";
        }
    }

    function handleSearchKeydown(event) {
        if (event.key === "Enter" && searchQuery.trim()) {
            createSearchTab(searchQuery.trim());
            searchQuery = "";
            isSearchMode = false;
        } else if (event.key === "Escape") {
            isSearchMode = false;
            searchQuery = "";
        }
    }

    function createSearchTab(query) {
        const searchTabId = `search-${Date.now()}`;
        const newSearchTab = {
            id: searchTabId,
            icon: "🔍",
            label: query,
            isSearchTab: true,
            query: query,
        };
        searchTabs = [...searchTabs, newSearchTab];
        selectedTab = searchTabId;

        // Initialize search results for this tab
        searchResults.set(searchTabId, {
            events: [],
            isLoading: false,
            hasMore: true,
            oldestTimestamp: null,
        });

        // Start loading search results
        loadSearchResults(searchTabId, query);
    }

    function closeSearchTab(tabId) {
        searchTabs = searchTabs.filter((tab) => tab.id !== tabId);
        searchResults.delete(tabId); // Clean up search results
        if (selectedTab === tabId) {
            selectedTab = "export"; // Fall back to export tab
        }
    }

    async function loadSearchResults(searchTabId, query, reset = true) {
        const searchResult = searchResults.get(searchTabId);
        if (!searchResult || searchResult.isLoading) return;

        // Update loading state
        searchResult.isLoading = true;
        searchResults.set(searchTabId, searchResult);

        try {
            const options = {
                limit: reset ? 100 : 200,
                until: reset
                    ? Math.floor(Date.now() / 1000)
                    : searchResult.oldestTimestamp,
            };

            console.log(
                "Loading search results for query:",
                query,
                "with options:",
                options,
            );
            const events = await searchEvents(query, options);
            console.log("Received search results:", events.length, "events");

            if (reset) {
                searchResult.events = events.sort(
                    (a, b) => b.created_at - a.created_at,
                );
            } else {
                searchResult.events = [...searchResult.events, ...events].sort(
                    (a, b) => b.created_at - a.created_at,
                );
            }

            // Update oldest timestamp for next pagination
            if (events.length > 0) {
                const oldestInBatch = Math.min(
                    ...events.map((e) => e.created_at),
                );
                if (
                    !searchResult.oldestTimestamp ||
                    oldestInBatch < searchResult.oldestTimestamp
                ) {
                    searchResult.oldestTimestamp = oldestInBatch;
                }
            }

            searchResult.hasMore = events.length === (reset ? 100 : 200);
            searchResult.isLoading = false;
            searchResults.set(searchTabId, searchResult);
        } catch (error) {
            console.error("Failed to load search results:", error);
            searchResult.isLoading = false;
            searchResults.set(searchTabId, searchResult);
            alert("Failed to load search results: " + error.message);
        }
    }

    async function loadMoreSearchResults(searchTabId) {
        const searchTab = searchTabs.find((tab) => tab.id === searchTabId);
        if (searchTab) {
            await loadSearchResults(searchTabId, searchTab.query, false);
        }
    }

    function handleSearchScroll(event, searchTabId) {
        const { scrollTop, scrollHeight, clientHeight } = event.target;
        const threshold = 100; // Load more when 100px from bottom

        if (scrollHeight - scrollTop - clientHeight < threshold) {
            const searchResult = searchResults.get(searchTabId);
            if (
                searchResult &&
                !searchResult.isLoading &&
                searchResult.hasMore
            ) {
                loadMoreSearchResults(searchTabId);
            }
        }
    }

    $: if (typeof document !== "undefined") {
        if (isDarkTheme) {
            document.body.classList.add("dark-theme");
        } else {
            document.body.classList.remove("dark-theme");
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
            console.log("Auto-fetching profile for:", userPubkey);
            await initializeNostrClient();
            userProfile = await fetchUserProfile(userPubkey);
            console.log("Profile auto-loaded:", userProfile);
        } catch (error) {
            console.error("Failed to auto-load profile:", error);
        }
    }

    async function fetchUserRole() {
        if (!isLoggedIn || !userPubkey) {
            userRole = "";
            return;
        }

        try {
            const response = await fetch(`/api/permissions/${userPubkey}`);
            if (response.ok) {
                const data = await response.json();
                userRole = data.permission || "";
                console.log("User role loaded:", userRole);
                console.log("Is owner?", userRole === "owner");
            } else {
                console.error("Failed to fetch user role:", response.status);
                userRole = "";
            }
        } catch (error) {
            console.error("Error fetching user role:", error);
            userRole = "";
        }
    }

    async function fetchACLMode() {
        try {
            const response = await fetch("/api/acl-mode");
            if (response.ok) {
                const data = await response.json();
                aclMode = data.acl_mode || "";
                console.log("ACL mode loaded:", aclMode);
            } else {
                console.error("Failed to fetch ACL mode:", response.status);
                aclMode = "";
            }
        } catch (error) {
            console.error("Error fetching ACL mode:", error);
            aclMode = "";
        }
    }

    // Export functionality
    async function exportEvents(pubkeys = []) {
        if (!isLoggedIn) {
            alert("Please log in first");
            return;
        }

        // Check permissions for exporting all events
        if (
            pubkeys.length === 0 &&
            userRole !== "admin" &&
            userRole !== "owner"
        ) {
            alert("Admin or owner permission required to export all events");
            return;
        }

        try {
            const authHeader = await createNIP98AuthHeader(
                "/api/export",
                "POST",
            );
            const response = await fetch("/api/export", {
                method: "POST",
                headers: {
                    Authorization: authHeader,
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ pubkeys }),
            });

            if (!response.ok) {
                throw new Error(
                    `Export failed: ${response.status} ${response.statusText}`,
                );
            }

            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;

            // Get filename from response headers or use default
            const contentDisposition = response.headers.get(
                "Content-Disposition",
            );
            let filename = "events.jsonl";
            if (contentDisposition) {
                const filenameMatch =
                    contentDisposition.match(/filename="([^"]+)"/);
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
            console.error("Export failed:", error);
            alert("Export failed: " + error.message);
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
        if (!isLoggedIn || (userRole !== "admin" && userRole !== "owner")) {
            alert("Admin or owner permission required");
            return;
        }

        if (!selectedFile) {
            alert("Please select a file");
            return;
        }

        try {
            const authHeader = await createNIP98AuthHeader(
                "/api/import",
                "POST",
            );
            const formData = new FormData();
            formData.append("file", selectedFile);

            const response = await fetch("/api/import", {
                method: "POST",
                headers: {
                    Authorization: authHeader,
                },
                body: formData,
            });

            if (!response.ok) {
                throw new Error(
                    `Import failed: ${response.status} ${response.statusText}`,
                );
            }

            const result = await response.json();
            alert("Import started successfully");
            selectedFile = null;
            document.getElementById("import-file").value = "";
        } catch (error) {
            console.error("Import failed:", error);
            alert("Import failed: " + error.message);
        }
    }

    // Events loading functionality
    async function loadMyEvents(reset = false) {
        if (!isLoggedIn) {
            alert("Please log in first");
            return;
        }

        if (isLoadingMyEvents) return;

        // Always load fresh data when feed becomes visible (reset = true)
        // Skip cache check to ensure fresh data every time

        isLoadingMyEvents = true;

        // Reset timestamps when doing a fresh load
        if (reset) {
            oldestMyEventTimestamp = null;
            newestMyEventTimestamp = null;
        }

        try {
            // Use WebSocket REQ to fetch user events with timestamp-based pagination
            // Load 1000 events on initial load, otherwise use 200 for pagination
            const events = await fetchUserEvents(userPubkey, {
                limit: reset ? 1000 : 200,
                until: reset ? null : oldestMyEventTimestamp,
            });

            if (reset) {
                myEvents = events.sort((a, b) => b.created_at - a.created_at);
            } else {
                myEvents = [...myEvents, ...events].sort(
                    (a, b) => b.created_at - a.created_at,
                );
            }

            // Update oldest timestamp for next pagination
            if (events.length > 0) {
                const oldestInBatch = Math.min(
                    ...events.map((e) => e.created_at),
                );
                if (
                    !oldestMyEventTimestamp ||
                    oldestInBatch < oldestMyEventTimestamp
                ) {
                    oldestMyEventTimestamp = oldestInBatch;
                }
            }

            hasMoreMyEvents = events.length === (reset ? 1000 : 200);

            // Auto-load more events if content doesn't fill viewport and more events are available
            // Only do this on initial load (reset = true) to avoid interfering with scroll-based loading
            if (reset && hasMoreMyEvents) {
                setTimeout(() => {
                    // Only check viewport if we're currently on the My Events tab
                    if (selectedTab === "myevents") {
                        const eventsContainers = document.querySelectorAll(
                            ".events-view-content",
                        );
                        // The My Events container should be the first one (before All Events)
                        const myEventsContainer = eventsContainers[0];
                        if (
                            myEventsContainer &&
                            myEventsContainer.scrollHeight <=
                                myEventsContainer.clientHeight
                        ) {
                            // Content doesn't fill viewport, load more automatically
                            loadMoreMyEvents();
                        }
                    }
                }, 100); // Small delay to ensure DOM is updated
            }
        } catch (error) {
            console.error("Failed to load events:", error);
            alert("Failed to load events: " + error.message);
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
        const threshold = 100; // Load more when 100px from bottom

        if (scrollHeight - scrollTop - clientHeight < threshold) {
            loadMoreMyEvents();
        }
    }

    async function loadAllEvents(reset = false, authors = null) {
        if (
            !isLoggedIn ||
            (userRole !== "write" &&
                userRole !== "admin" &&
                userRole !== "owner")
        ) {
            alert("Write, admin, or owner permission required");
            return;
        }

        if (isLoadingEvents) return;

        // Always load fresh data when feed becomes visible (reset = true)
        // Skip cache check to ensure fresh data every time

        isLoadingEvents = true;

        // Reset timestamps when doing a fresh load
        if (reset) {
            oldestEventTimestamp = null;
            newestEventTimestamp = null;
        }

        try {
            // Use Nostr WebSocket to fetch events with timestamp-based pagination
            // Load 100 events on initial load, otherwise use 200 for pagination
            console.log(
                "Loading events with authors filter:",
                authors,
                "including delete events",
            );
            const events = await fetchAllEvents({
                limit: reset ? 100 : 200,
                until: reset
                    ? Math.floor(Date.now() / 1000)
                    : oldestEventTimestamp,
                authors: authors,
            });
            console.log("Received events:", events.length, "events");
            if (authors && events.length > 0) {
                const nonUserEvents = events.filter(
                    (event) => event.pubkey && event.pubkey !== userPubkey,
                );
                if (nonUserEvents.length > 0) {
                    console.warn(
                        "Server returned non-user events:",
                        nonUserEvents.length,
                        "out of",
                        events.length,
                    );
                }
            }

            if (reset) {
                allEvents = events.sort((a, b) => b.created_at - a.created_at);
                // Update global cache
                updateGlobalCache(events);
            } else {
                allEvents = [...allEvents, ...events].sort(
                    (a, b) => b.created_at - a.created_at,
                );
                // Update global cache with all events
                updateGlobalCache(allEvents);
            }

            // Update oldest timestamp for next pagination
            if (events.length > 0) {
                const oldestInBatch = Math.min(
                    ...events.map((e) => e.created_at),
                );
                if (
                    !oldestEventTimestamp ||
                    oldestInBatch < oldestEventTimestamp
                ) {
                    oldestEventTimestamp = oldestInBatch;
                }
            }

            hasMoreEvents = events.length === (reset ? 1000 : 200);

            // Auto-load more events if content doesn't fill viewport and more events are available
            // Only do this on initial load (reset = true) to avoid interfering with scroll-based loading
            if (reset && hasMoreEvents) {
                setTimeout(() => {
                    // Only check viewport if we're currently on the All Events tab
                    if (selectedTab === "events") {
                        const eventsContainers = document.querySelectorAll(
                            ".events-view-content",
                        );
                        // The All Events container should be the first one (only container now)
                        const allEventsContainer = eventsContainers[0];
                        if (
                            allEventsContainer &&
                            allEventsContainer.scrollHeight <=
                                allEventsContainer.clientHeight
                        ) {
                            // Content doesn't fill viewport, load more automatically
                            loadMoreEvents();
                        }
                    }
                }, 100); // Small delay to ensure DOM is updated
            }
        } catch (error) {
            console.error("Failed to load events:", error);
            alert("Failed to load events: " + error.message);
        } finally {
            isLoadingEvents = false;
        }
    }

    async function loadMoreEvents() {
        await loadAllEvents(false);
    }

    function handleScroll(event) {
        const { scrollTop, scrollHeight, clientHeight } = event.target;
        const threshold = 100; // Load more when 100px from bottom

        if (scrollHeight - scrollTop - clientHeight < threshold) {
            loadMoreEvents();
        }
    }

    // Load events when events tab is selected (only if no events loaded yet)
    $: if (
        selectedTab === "events" &&
        isLoggedIn &&
        (userRole === "write" ||
            userRole === "admin" ||
            userRole === "owner") &&
        allEvents.length === 0
    ) {
        const authors = showOnlyMyEvents && userPubkey ? [userPubkey] : null;
        loadAllEvents(true, authors);
    }

    // NIP-98 authentication helper
    async function createNIP98AuthHeader(url, method) {
        if (!isLoggedIn || !userPubkey) {
            throw new Error("Not logged in");
        }

        // Create NIP-98 auth event
        const authEvent = {
            kind: 27235,
            created_at: Math.floor(Date.now() / 1000),
            tags: [
                ["u", window.location.origin + url],
                ["method", method.toUpperCase()],
            ],
            content: "",
            pubkey: userPubkey,
        };

        let signedEvent;

        if (userSigner && authMethod === "extension") {
            // Use the signer from the extension
            try {
                signedEvent = await userSigner.signEvent(authEvent);
            } catch (error) {
                throw new Error(
                    "Failed to sign with extension: " + error.message,
                );
            }
        } else if (authMethod === "nsec") {
            // For nsec method, we need to implement proper signing
            // For now, create a mock signature (in production, use proper crypto)
            authEvent.id = "mock-id-" + Date.now();
            authEvent.sig = "mock-signature-" + Date.now();
            signedEvent = authEvent;
        } else {
            throw new Error("No valid signer available");
        }

        // Encode as base64
        const eventJson = JSON.stringify(signedEvent);
        const base64Event = btoa(eventJson);

        return `Nostr ${base64Event}`;
    }

    // NIP-98 authentication helper (for sprocket functions)
    async function createNIP98Auth(method, url) {
        if (!isLoggedIn || !userPubkey) {
            throw new Error("Not logged in");
        }

        // Create NIP-98 auth event
        const authEvent = {
            kind: 27235,
            created_at: Math.floor(Date.now() / 1000),
            tags: [
                ["u", window.location.origin + url],
                ["method", method.toUpperCase()],
            ],
            content: "",
            pubkey: userPubkey,
        };

        let signedEvent;

        if (userSigner && authMethod === "extension") {
            // Use the signer from the extension
            try {
                signedEvent = await userSigner.signEvent(authEvent);
            } catch (error) {
                throw new Error(
                    "Failed to sign with extension: " + error.message,
                );
            }
        } else if (authMethod === "nsec") {
            // For nsec method, we need to implement proper signing
            // For now, create a mock signature (in production, use proper crypto)
            authEvent.id = "mock-id-" + Date.now();
            authEvent.sig = "mock-signature-" + Date.now();
            signedEvent = authEvent;
        } else {
            throw new Error("No valid signer available");
        }

        // Encode as base64
        const eventJson = JSON.stringify(signedEvent);
        const base64Event = btoa(eventJson);

        return base64Event;
    }

    // Compose tab functions
    function reformatJson() {
        try {
            if (!composeEventJson.trim()) {
                alert("Please enter some JSON to reformat");
                return;
            }
            const parsed = JSON.parse(composeEventJson);
            composeEventJson = JSON.stringify(parsed, null, 2);
        } catch (error) {
            alert("Invalid JSON: " + error.message);
        }
    }

    async function signEvent() {
        try {
            if (!composeEventJson.trim()) {
                alert("Please enter an event to sign");
                return;
            }

            if (!isLoggedIn || !userPubkey) {
                alert("Please log in to sign events");
                return;
            }

            if (!userSigner) {
                alert(
                    "No signer available. Please log in with a valid authentication method.",
                );
                return;
            }

            const event = JSON.parse(composeEventJson);

            // Update event with current user's pubkey and timestamp
            event.pubkey = userPubkey;
            event.created_at = Math.floor(Date.now() / 1000);

            // Remove any existing id and sig to ensure fresh signing
            delete event.id;
            delete event.sig;

            // Sign the event using the real signer
            const signedEvent = await userSigner.signEvent(event);

            // Update the compose area with the signed event
            composeEventJson = JSON.stringify(signedEvent, null, 2);

            // Show success feedback
            alert("Event signed successfully!");
        } catch (error) {
            console.error("Error signing event:", error);
            alert("Error signing event: " + error.message);
        }
    }

    async function publishEvent() {
        try {
            if (!composeEventJson.trim()) {
                alert("Please enter an event to publish");
                return;
            }

            if (!isLoggedIn) {
                alert("Please log in to publish events");
                return;
            }

            if (!userSigner) {
                alert(
                    "No signer available. Please log in with a valid authentication method.",
                );
                return;
            }

            const event = JSON.parse(composeEventJson);

            // Validate that the event has required fields
            if (!event.id || !event.sig) {
                alert(
                    'Event must be signed before publishing. Please click "Sign" first.',
                );
                return;
            }

            // Publish to the ORLY relay using WebSocket (same address as current page)
            const relayUrl = `wss://${window.location.host}`;

            // Use the authentication module to publish the event
            const result = await publishEventWithAuth(
                relayUrl,
                event,
                userSigner,
                userPubkey,
            );

            if (result.success) {
                alert("Event published successfully to ORLY relay!");
                // Optionally clear the editor after successful publish
                // composeEventJson = '';
            } else {
                alert(
                    `Event publishing failed: ${result.reason || "Unknown error"}`,
                );
            }
        } catch (error) {
            console.error("Error publishing event:", error);
            alert("Error publishing event: " + error.message);
        }
    }
</script>

<!-- Header -->
<header class="main-header" class:dark-theme={isDarkTheme}>
    <div class="header-content">
        <img src="/orly.png" alt="ORLY Logo" class="logo" />
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
        <button class="search-btn" on:click={toggleSearchMode}> 🔍 </button>
        <button class="theme-toggle-btn" on:click={toggleTheme}>
            {isDarkTheme ? "☀️" : "🌙"}
        </button>
        {#if isLoggedIn}
            <div class="user-info">
                <button class="user-profile-btn" on:click={openSettingsDrawer}>
                    {#if userProfile?.picture}
                        <img
                            src={userProfile.picture}
                            alt="User avatar"
                            class="user-avatar"
                        />
                    {:else}
                        <div class="user-avatar-placeholder">👤</div>
                    {/if}
                    <span class="user-name">
                        {userProfile?.name || userPubkey.slice(0, 8) + "..."}
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
                    <button
                        class="tab"
                        class:active={selectedTab === tab.id}
                        on:click={() => selectTab(tab.id)}
                    >
                        <span class="tab-icon">{tab.icon}</span>
                        <span class="tab-label">{tab.label}</span>
                        {#if tab.isSearchTab}
                            <span
                                class="tab-close-icon"
                                on:click|stopPropagation={() =>
                                    closeSearchTab(tab.id)}
                                on:keydown={(e) =>
                                    e.key === "Enter" && closeSearchTab(tab.id)}
                                role="button"
                                tabindex="0">✕</span
                            >
                        {/if}
                    </button>
                {/each}
            </div>
        </div>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
        {#if selectedTab === "export"}
            {#if isLoggedIn}
                <div class="export-section">
                    <h2>Export My Events</h2>
                    <p>Download your personal events as a JSONL file.</p>
                    <button class="export-btn" on:click={exportMyEvents}>
                        📤 Export My Events
                    </button>
                </div>
                {#if userRole === "admin" || userRole === "owner"}
                    <div class="export-section">
                        <h3>Export All Events</h3>
                        <p>
                            Download the complete database as a JSONL file. This
                            includes all events from all users.
                        </p>
                        <button class="export-btn" on:click={exportAllEvents}>
                            📤 Export All Events
                        </button>
                    </div>
                {/if}
            {:else}
                <div class="login-prompt">
                    <p>Please log in to access export functionality.</p>
                    <button class="login-btn" on:click={openLoginModal}
                        >Log In</button
                    >
                </div>
            {/if}
        {:else if selectedTab === "import"}
            <div class="import-view">
                {#if isLoggedIn && (userRole === "admin" || userRole === "owner")}
                    <h2>Import Events</h2>
                    <p>
                        Upload a JSONL file to import events into the database.
                    </p>
                    <input
                        type="file"
                        id="import-file"
                        accept=".jsonl,.txt"
                        on:change={handleFileSelect}
                    />
                    <button
                        class="import-btn"
                        on:click={importEvents}
                        disabled={!selectedFile}
                    >
                        📥 Import Events
                    </button>
                {:else if isLoggedIn}
                    <div class="permission-denied">
                        <p>
                            ❌ Admin or owner permission required for import
                            functionality.
                        </p>
                    </div>
                {:else}
                    <div class="login-prompt">
                        <p>Please log in to access import functionality.</p>
                        <button class="login-btn" on:click={openLoginModal}
                            >Log In</button
                        >
                    </div>
                {/if}
            </div>
        {:else if selectedTab === "events"}
            <div class="events-view-container">
                {#if isLoggedIn && (userRole === "write" || userRole === "admin" || userRole === "owner")}
                    <div class="events-view-content" on:scroll={handleScroll}>
                        {#if filteredEvents.length > 0}
                            {#each filteredEvents as event}
                                <div
                                    class="events-view-item"
                                    class:expanded={expandedEvents.has(
                                        event.id,
                                    )}
                                >
                                    <div
                                        class="events-view-row"
                                        on:click={() =>
                                            toggleEventExpansion(event.id)}
                                        on:keydown={(e) =>
                                            e.key === "Enter" &&
                                            toggleEventExpansion(event.id)}
                                        role="button"
                                        tabindex="0"
                                    >
                                        <div class="events-view-avatar">
                                            <div class="avatar-placeholder">
                                                👤
                                            </div>
                                        </div>
                                        <div class="events-view-info">
                                            <div class="events-view-author">
                                                {truncatePubkey(event.pubkey)}
                                            </div>
                                            <div class="events-view-kind">
                                                <span
                                                    class="kind-number"
                                                    class:delete-event={event.kind ===
                                                        5}>{event.kind}</span
                                                >
                                                <span class="kind-name"
                                                    >{getKindName(
                                                        event.kind,
                                                    )}</span
                                                >
                                            </div>
                                        </div>
                                        <div class="events-view-content">
                                            <div class="event-timestamp">
                                                {formatTimestamp(
                                                    event.created_at,
                                                )}
                                            </div>
                                            {#if event.kind === 5}
                                                <div class="delete-event-info">
                                                    <span
                                                        class="delete-event-label"
                                                        >🗑️ Delete Event</span
                                                    >
                                                    {#if event.tags && event.tags.length > 0}
                                                        <div
                                                            class="delete-targets"
                                                        >
                                                            {#each event.tags.filter((tag) => tag[0] === "e") as eTag}
                                                                <span
                                                                    class="delete-target"
                                                                    >Target: {eTag[1].slice(
                                                                        0,
                                                                        8,
                                                                    )}...{eTag[1].slice(
                                                                        -8,
                                                                    )}</span
                                                                >
                                                            {/each}
                                                        </div>
                                                    {/if}
                                                </div>
                                            {:else}
                                                <div
                                                    class="event-content-single-line"
                                                >
                                                    {truncateContent(
                                                        event.content,
                                                    )}
                                                </div>
                                            {/if}
                                        </div>
                                        {#if event.kind !== 5 && (userRole === "admin" || userRole === "owner" || (userRole === "write" && event.pubkey && event.pubkey === userPubkey))}
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
                                        <div class="events-view-details">
                                            <div class="json-container">
                                                <pre
                                                    class="event-json">{JSON.stringify(
                                                        event,
                                                        null,
                                                        2,
                                                    )}</pre>
                                                <button
                                                    class="copy-json-btn"
                                                    on:click|stopPropagation={(
                                                        e,
                                                    ) =>
                                                        copyEventToClipboard(
                                                            event,
                                                            e,
                                                        )}
                                                    title="Copy minified JSON to clipboard"
                                                >
                                                    📋
                                                </button>
                                            </div>
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
                {:else}
                    <div class="permission-denied">
                        <p>
                            ❌ Write, admin, or owner permission required to
                            view all events.
                        </p>
                    </div>
                {/if}
                {#if isLoggedIn && (userRole === "write" || userRole === "admin" || userRole === "owner")}
                    <div class="events-view-header">
                        <div class="events-view-toggle">
                            <label class="toggle-container">
                                <input
                                    type="checkbox"
                                    bind:checked={showOnlyMyEvents}
                                    on:change={() => handleToggleChange()}
                                />
                                <span class="toggle-slider"></span>
                                <span class="toggle-label"
                                    >Only show my events</span
                                >
                            </label>
                        </div>
                        <div class="events-view-buttons">
                            <button
                                class="refresh-btn"
                                on:click={() => {
                                    const authors =
                                        showOnlyMyEvents && userPubkey
                                            ? [userPubkey]
                                            : null;
                                    loadAllEvents(false, authors);
                                }}
                                disabled={isLoadingEvents}
                            >
                                🔄 Load More
                            </button>
                            <button
                                class="reload-btn"
                                on:click={() => {
                                    const authors =
                                        showOnlyMyEvents && userPubkey
                                            ? [userPubkey]
                                            : null;
                                    loadAllEvents(true, authors);
                                }}
                                disabled={isLoadingEvents}
                            >
                                {#if isLoadingEvents}
                                    <div class="spinner"></div>
                                {:else}
                                    🔄
                                {/if}
                            </button>
                        </div>
                    </div>
                {/if}
            </div>
        {:else if selectedTab === "compose"}
            <div class="compose-view">
                <div class="compose-header">
                    <button
                        class="compose-btn reformat-btn"
                        on:click={reformatJson}>Reformat</button
                    >
                    <button class="compose-btn sign-btn" on:click={signEvent}
                        >Sign</button
                    >
                    <button
                        class="compose-btn publish-btn"
                        on:click={publishEvent}>Publish</button
                    >
                </div>
                <div class="compose-editor">
                    <textarea
                        bind:value={composeEventJson}
                        class="compose-textarea"
                        placeholder="Enter your Nostr event JSON here..."
                        spellcheck="false"
                    ></textarea>
                </div>
            </div>
        {:else if selectedTab === "managed-acl"}
            <div class="managed-acl-view">
                {#if aclMode !== "managed"}
                    <div class="acl-mode-warning">
                        <h3>⚠️ Managed ACL Mode Not Active</h3>
                        <p>
                            To use the Managed ACL interface, you need to set
                            the ACL mode to "managed" in your relay
                            configuration.
                        </p>
                        <p>
                            Current ACL mode: <strong
                                >{aclMode || "unknown"}</strong
                            >
                        </p>
                        <p>
                            Please set <code>ORLY_ACL_MODE=managed</code> in your
                            environment variables and restart the relay.
                        </p>
                    </div>
                {:else if isLoggedIn && userRole === "owner"}
                    <ManagedACL {userSigner} {userPubkey} />
                {:else}
                    <div class="access-denied">
                        <p>
                            Please log in with owner permissions to access
                            managed ACL configuration.
                        </p>
                        <button class="login-btn" on:click={openLoginModal}
                            >Log In</button
                        >
                    </div>
                {/if}
            </div>
        {:else if selectedTab === "sprocket"}
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
                                    disabled={isLoadingSprocket ||
                                        !sprocketUploadFile}
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
                                    <span class="status-value"
                                        >{sprocketStatus.pid}</span
                                    >
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
                                            {new Date(
                                                version.modified,
                                            ).toLocaleString()}
                                            {#if version.is_current}
                                                <span class="current-badge"
                                                    >Current</span
                                                >
                                            {/if}
                                        </div>
                                    </div>
                                    <div class="version-actions">
                                        <button
                                            class="version-btn load-btn"
                                            on:click={() =>
                                                loadVersion(version)}
                                            disabled={isLoadingSprocket}
                                        >
                                            📥 Load
                                        </button>
                                        {#if !version.is_current}
                                            <button
                                                class="version-btn delete-btn"
                                                on:click={() =>
                                                    deleteVersion(version.name)}
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
                        <p>
                            ❌ Owner permission required for sprocket
                            management.
                        </p>
                        <p>
                            To enable sprocket functionality, set the <code
                                >ORLY_OWNERS</code
                            > environment variable with your npub when starting the
                            relay.
                        </p>
                        <p>
                            Current user role: <strong
                                >{userRole || "none"}</strong
                            >
                        </p>
                    </div>
                {:else}
                    <div class="login-prompt">
                        <p>Please log in to access sprocket management.</p>
                        <button class="login-btn" on:click={openLoginModal}
                            >Log In</button
                        >
                    </div>
                {/if}
            </div>
        {:else if selectedTab === "recovery"}
            <div class="recovery-tab">
                <div class="recovery-header">
                    <h2>🔄 Event Recovery</h2>
                    <p>Search and recover old versions of replaceable events</p>
                </div>

                <div class="recovery-controls">
                    <div class="kind-selector">
                        <label for="recovery-kind">Select Event Kind:</label>
                        <select
                            id="recovery-kind"
                            bind:value={recoverySelectedKind}
                            on:change={() =>
                                selectRecoveryKind(recoverySelectedKind)}
                        >
                            <option value={null}
                                >Choose a replaceable kind...</option
                            >
                            {#each replaceableKinds as kind}
                                <option value={kind.value}>{kind.label}</option>
                            {/each}
                        </select>
                    </div>

                    <div class="custom-kind-input">
                        <label for="custom-kind"
                            >Or enter custom kind number:</label
                        >
                        <input
                            id="custom-kind"
                            type="number"
                            bind:value={recoveryCustomKind}
                            on:input={handleCustomKindInput}
                            placeholder="e.g., 10001"
                            min="0"
                        />
                    </div>
                </div>

                {#if recoverySelectedKind || recoveryCustomKind}
                    <div class="recovery-results">
                        {#if isLoadingRecovery}
                            <div class="loading">Loading events...</div>
                        {:else if recoveryEvents.length === 0}
                            <div class="no-events">
                                No events found for this kind
                            </div>
                        {:else}
                            <div class="events-list">
                                {#each recoveryEvents as event}
                                    {@const isCurrent = isCurrentVersion(event)}
                                    <div
                                        class="event-item"
                                        class:old-version={!isCurrent}
                                    >
                                        <div class="event-header">
                                            <span class="event-kind"
                                                >Kind {event.kind}</span
                                            >
                                            <span class="event-timestamp">
                                                {new Date(
                                                    event.created_at * 1000,
                                                ).toLocaleString()}
                                            </span>
                                            {#if !isCurrent}
                                                <button
                                                    class="repost-button"
                                                    on:click={() =>
                                                        repostEvent(event)}
                                                >
                                                    🔄 Repost
                                                </button>
                                            {/if}
                                        </div>

                                        <div class="event-content">
                                            {#if event.kind === 0}
                                                <!-- Profile metadata -->
                                                {#if event.content}
                                                    <div
                                                        class="profile-content"
                                                    >
                                                        <pre>{JSON.stringify(
                                                                JSON.parse(
                                                                    event.content,
                                                                ),
                                                                null,
                                                                2,
                                                            )}</pre>
                                                    </div>
                                                {/if}
                                            {:else if event.kind === 3}
                                                <!-- Follow list -->
                                                <div class="follow-list">
                                                    <p>
                                                        Follows {event.tags.filter(
                                                            (t) => t[0] === "p",
                                                        ).length} people
                                                    </p>
                                                    <div class="follow-tags">
                                                        {#each event.tags.filter((t) => t[0] === "p") as tag}
                                                            <span
                                                                class="pubkey-tag"
                                                                >{tag[1].substring(
                                                                    0,
                                                                    8,
                                                                )}...</span
                                                            >
                                                        {/each}
                                                    </div>
                                                </div>
                                            {:else}
                                                <!-- Generic content -->
                                                <div class="generic-content">
                                                    <pre>{event.content}</pre>
                                                </div>
                                            {/if}
                                        </div>

                                        <div class="event-tags">
                                            {#each event.tags as tag}
                                                <span class="tag"
                                                    >{tag[0]}: {tag[1]}</span
                                                >
                                            {/each}
                                        </div>
                                    </div>
                                {/each}
                            </div>

                            {#if recoveryHasMore}
                                <button
                                    class="load-more"
                                    on:click={loadRecoveryEvents}
                                    disabled={isLoadingRecovery}
                                >
                                    Load More Events
                                </button>
                            {/if}
                        {/if}
                    </div>
                {/if}
            </div>
        {:else if searchTabs.some((tab) => tab.id === selectedTab)}
            {#each searchTabs as searchTab}
                {#if searchTab.id === selectedTab}
                    <div class="search-results-view">
                        <div class="search-results-header">
                            <h2>🔍 Search Results: "{searchTab.query}"</h2>
                            <button
                                class="refresh-btn"
                                on:click={() =>
                                    loadSearchResults(
                                        searchTab.id,
                                        searchTab.query,
                                        true,
                                    )}
                                disabled={searchResults.get(searchTab.id)
                                    ?.isLoading}
                            >
                                🔄 Refresh
                            </button>
                        </div>
                        <div
                            class="search-results-content"
                            on:scroll={(e) =>
                                handleSearchScroll(e, searchTab.id)}
                        >
                            {#if searchResults.get(searchTab.id)?.events?.length > 0}
                                {#each searchResults.get(searchTab.id).events as event}
                                    <div
                                        class="search-result-item"
                                        class:expanded={expandedEvents.has(
                                            event.id,
                                        )}
                                    >
                                        <div
                                            class="search-result-row"
                                            on:click={() =>
                                                toggleEventExpansion(event.id)}
                                            on:keydown={(e) =>
                                                e.key === "Enter" &&
                                                toggleEventExpansion(event.id)}
                                            role="button"
                                            tabindex="0"
                                        >
                                            <div class="search-result-avatar">
                                                <div class="avatar-placeholder">
                                                    👤
                                                </div>
                                            </div>
                                            <div class="search-result-info">
                                                <div
                                                    class="search-result-author"
                                                >
                                                    {truncatePubkey(
                                                        event.pubkey,
                                                    )}
                                                </div>
                                                <div class="search-result-kind">
                                                    <span class="kind-number"
                                                        >{event.kind}</span
                                                    >
                                                    <span class="kind-name"
                                                        >{getKindName(
                                                            event.kind,
                                                        )}</span
                                                    >
                                                </div>
                                            </div>
                                            <div class="search-result-content">
                                                <div class="event-timestamp">
                                                    {formatTimestamp(
                                                        event.created_at,
                                                    )}
                                                </div>
                                                <div
                                                    class="event-content-single-line"
                                                >
                                                    {truncateContent(
                                                        event.content,
                                                    )}
                                                </div>
                                            </div>
                                            {#if event.kind !== 5 && (userRole === "admin" || userRole === "owner" || (userRole === "write" && event.pubkey && event.pubkey === userPubkey))}
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
                                                    <pre
                                                        class="event-json">{JSON.stringify(
                                                            event,
                                                            null,
                                                            2,
                                                        )}</pre>
                                                    <button
                                                        class="copy-json-btn"
                                                        on:click|stopPropagation={(
                                                            e,
                                                        ) =>
                                                            copyEventToClipboard(
                                                                event,
                                                                e,
                                                            )}
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
                                <div class="no-search-results">
                                    <p>
                                        No search results found for "{searchTab.query}".
                                    </p>
                                </div>
                            {/if}

                            {#if searchResults.get(searchTab.id)?.isLoading}
                                <div class="loading-search-results">
                                    <div class="loading-spinner"></div>
                                    <p>Searching...</p>
                                </div>
                            {/if}

                            {#if !searchResults.get(searchTab.id)?.hasMore && searchResults.get(searchTab.id)?.events?.length > 0}
                                <div class="end-of-search-results">
                                    <p>No more search results to load.</p>
                                </div>
                            {/if}
                        </div>
                    </div>
                {/if}
            {/each}
        {:else}
            <div class="welcome-message">
                {#if isLoggedIn}
                    <p>
                        Welcome {userProfile?.name ||
                            userPubkey.slice(0, 8) + "..."}
                    </p>
                {:else}
                    <p>Log in to access your user dashboard</p>
                {/if}
            </div>
        {/if}
    </main>
</div>

<!-- Settings Drawer -->
{#if showSettingsDrawer}
    <div
        class="drawer-overlay"
        on:click={closeSettingsDrawer}
        on:keydown={(e) => e.key === "Escape" && closeSettingsDrawer()}
        role="button"
        tabindex="0"
    >
        <div
            class="settings-drawer"
            class:dark-theme={isDarkTheme}
            on:click|stopPropagation
            on:keydown|stopPropagation
        >
            <div class="drawer-header">
                <h2>Settings</h2>
                <button class="close-btn" on:click={closeSettingsDrawer}
                    >✕</button
                >
            </div>
            <div class="drawer-content">
                {#if userProfile}
                    <div class="profile-section">
                        <div class="profile-hero">
                            {#if userProfile.banner}
                                <img
                                    src={userProfile.banner}
                                    alt="Profile banner"
                                    class="profile-banner"
                                />
                            {/if}
                            <!-- Logout button floating in top-right corner of banner -->
                            <button
                                class="logout-btn floating"
                                on:click={handleLogout}>Log out</button
                            >
                            <!-- Avatar overlaps the bottom edge of the banner by 50% -->
                            {#if userProfile.picture}
                                <img
                                    src={userProfile.picture}
                                    alt="User avatar"
                                    class="profile-avatar overlap"
                                />
                            {:else}
                                <div class="profile-avatar-placeholder overlap">
                                    👤
                                </div>
                            {/if}
                            <!-- Username and nip05 to the right of the avatar, above the bottom edge -->
                            <div class="name-row">
                                <h3 class="profile-username">
                                    {userProfile.name || "Unknown User"}
                                </h3>
                                {#if userProfile.nip05}
                                    <span class="profile-nip05-inline"
                                        >{userProfile.nip05}</span
                                    >
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
                        <button
                            class="retry-profile-btn"
                            on:click={fetchProfileIfMissing}
                        >
                            Retry Loading Profile
                        </button>
                        <div class="user-pubkey-display">
                            <strong>Public Key:</strong>
                            {userPubkey.slice(0, 16)}...{userPubkey.slice(-8)}
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
        --primary: #00bcd4;
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
        --primary: #00bcd4;
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
        transition: opacity 0.2s ease;
    }

    .logo:hover {
        content: url("/favicon.png");
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
        background-color: #4caf50;
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
        padding: 0.5em 1em;
    }

    .login-btn:hover {
        background-color: #45a049;
    }

    .acl-mode-warning {
        padding: 1em;
        background-color: #fff3cd;
        border: 1px solid #ffeaa7;
        border-radius: 8px;
        color: #856404;
        margin: 20px 0;
    }

    .acl-mode-warning h3 {
        margin: 0 0 15px 0;
        color: #856404;
    }

    .acl-mode-warning p {
        margin: 10px 0;
        line-height: 1.5;
    }

    .acl-mode-warning code {
        background-color: #f8f9fa;
        padding: 2px 6px;
        border-radius: 4px;
        font-family: monospace;
        color: #495057;
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

    .tab-close-icon {
        cursor: pointer;
        transition: opacity 0.2s;
        font-size: 0.8em;
        margin-left: auto;
        padding: 0.25rem;
        border-radius: 0.25rem;
        flex-shrink: 0;
    }

    .tab-close-icon:hover {
        opacity: 0.7;
        background-color: var(--warning);
        color: white;
    }

    /* Main Content */
    .main-content {
        position: fixed;
        left: 200px;
        top: 3em;
        right: 0;
        bottom: 0;
        padding: 1em;
        overflow-y: auto;
        background-color: var(--bg-color);
        color: var(--text-color);
        display: flex;
        align-items: flex-start;
        justify-content: flex-start;
        flex-direction: column;
        display: flex;
    }

    .welcome-message {
        text-align: center;
    }

    .welcome-message p {
        font-size: 1.2rem;
    }

    /* Sprocket Styles */
    .sprocket-view {
        width: 100%;
        max-width: 1200px;
        margin: 0;
        padding: 20px;
        background: var(--header-bg);
        color: var(--text-color);
        border-radius: 8px;
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

    .sprocket-controls {
        display: flex;
        gap: 0.5rem;
    }

    .sprocket-upload-section {
        margin-bottom: 1rem;
        padding: 1rem;
        background-color: var(--bg-color);
        border-radius: 6px;
        border: 1px solid var(--border-color);
    }

    .sprocket-upload-section h4 {
        margin: 0 0 0.75rem 0;
        color: var(--text-color);
        font-size: 1rem;
        font-weight: 500;
    }

    .upload-controls {
        display: flex;
        gap: 0.5rem;
        align-items: center;
    }

    .upload-controls input[type="file"] {
        flex: 1;
        padding: 0.5rem;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--bg-color);
        color: var(--text-color);
        font-size: 0.9rem;
    }

    .sprocket-btn.upload-btn {
        background-color: #8b5cf6;
        color: white;
    }

    .sprocket-btn.upload-btn:hover:not(:disabled) {
        background-color: #7c3aed;
    }

    .sprocket-status {
        display: flex;
        gap: 1rem;
        margin-bottom: 1rem;
        padding: 0.75rem;
        background-color: var(--bg-color);
        border-radius: 6px;
        border: 1px solid var(--border-color);
    }

    .status-item {
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    .status-label {
        font-size: 0.8rem;
        color: var(--text-muted);
        font-weight: 500;
    }

    .status-value {
        font-size: 0.9rem;
        font-weight: 600;
    }

    .status-value.running {
        color: #22c55e;
    }

    .script-editor-container {
        margin-bottom: 1rem;
    }

    .script-editor {
        width: 100%;
        height: 300px;
        padding: 1rem;
        border: 1px solid var(--border-color);
        border-radius: 6px;
        background-color: var(--bg-color);
        color: var(--text-color);
        font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace;
        font-size: 0.9rem;
        line-height: 1.4;
        resize: vertical;
        outline: none;
    }

    .script-editor:focus {
        border-color: var(--primary-color);
        box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
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

    .sprocket-btn {
        padding: 0.5rem 1rem;
        border: none;
        border-radius: 6px;
        font-size: 0.9rem;
        font-weight: 500;
        cursor: pointer;
        transition: all 0.2s ease;
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .sprocket-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .sprocket-btn.save-btn {
        background-color: #22c55e;
        color: white;
    }

    .sprocket-btn.save-btn:hover:not(:disabled) {
        background-color: #16a34a;
    }

    .sprocket-btn.load-btn {
        background-color: #3b82f6;
        color: white;
    }

    .sprocket-btn.load-btn:hover:not(:disabled) {
        background-color: #2563eb;
    }

    .sprocket-btn.restart-btn {
        background-color: #f59e0b;
        color: white;
    }

    .sprocket-btn.restart-btn:hover:not(:disabled) {
        background-color: #d97706;
    }

    .sprocket-btn.delete-btn {
        background-color: #ef4444;
        color: white;
    }

    .sprocket-btn.delete-btn:hover:not(:disabled) {
        background-color: #dc2626;
    }

    .sprocket-btn.refresh-btn {
        background-color: #6b7280;
        color: white;
    }

    .sprocket-btn.refresh-btn:hover:not(:disabled) {
        background-color: #4b5563;
    }

    .sprocket-message {
        padding: 0.75rem;
        border-radius: 6px;
        font-size: 0.9rem;
        font-weight: 500;
        background-color: #dbeafe;
        color: #1e40af;
        border: 1px solid #93c5fd;
    }

    .sprocket-message.error {
        background-color: #fee2e2;
        color: #dc2626;
        border-color: #fca5a5;
    }

    .versions-list {
        display: flex;
        flex-direction: column;
        gap: 0.75rem;
        margin-bottom: 1rem;
    }

    .version-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 1rem;
        background-color: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        transition: all 0.2s ease;
    }

    .version-item.current {
        border-color: var(--primary-color);
        background-color: rgba(59, 130, 246, 0.05);
    }

    .version-item:hover {
        border-color: var(--primary-color);
    }

    .version-info {
        flex: 1;
    }

    .version-name {
        font-weight: 600;
        font-size: 0.9rem;
        margin-bottom: 0.25rem;
    }

    .version-date {
        font-size: 0.8rem;
        color: var(--text-muted);
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .current-badge {
        background-color: var(--primary-color);
        color: white;
        padding: 0.125rem 0.5rem;
        border-radius: 12px;
        font-size: 0.7rem;
        font-weight: 500;
    }

    .version-actions {
        display: flex;
        gap: 0.5rem;
    }

    .version-btn {
        padding: 0.375rem 0.75rem;
        border: none;
        border-radius: 4px;
        font-size: 0.8rem;
        font-weight: 500;
        cursor: pointer;
        transition: all 0.2s ease;
        display: flex;
        align-items: center;
        gap: 0.25rem;
    }

    .version-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .version-btn.load-btn {
        background-color: #3b82f6;
        color: white;
    }

    .version-btn.load-btn:hover:not(:disabled) {
        background-color: #2563eb;
    }

    .version-btn.delete-btn {
        background-color: #ef4444;
        color: white;
    }

    .version-btn.delete-btn:hover:not(:disabled) {
        background-color: #dc2626;
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
        height: 3em;
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
        padding: 0;
        margin: 0;
    }

    .user-profile-btn:hover {
        background-color: var(--button-bg);
        padding: 0;
    }

    .user-avatar,
    .user-avatar-placeholder {
        width: 2.5em;
        height: 2.5em;
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
        font-size: 1.2em;
        font-weight: 500;
        max-width: 100px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        padding: 0.5em;
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
        from {
            transform: translateX(100%);
        }
        to {
            transform: translateX(0);
        }
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
    .profile-avatar,
    .profile-avatar-placeholder {
        width: 72px;
        height: 72px;
        border-radius: 50%;
        object-fit: cover;
        flex-shrink: 0;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
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
        text-shadow: 0 3px 6px rgba(255, 255, 255, 1);
    }

    .profile-nip05-inline {
        font-size: 0.85rem;
        color: #000; /* subtle but contrasting */
        font-family: monospace;
        opacity: 0.95;
        text-shadow: 0 3px 6px rgba(255, 255, 255, 1);
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
        background: #00acc1;
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
    .import-view {
        padding: 1em;
        max-width: 32em;
        margin: 0;
        background: var(--header-bg);
        color: var(--text-color);
        border-radius: 8px;
    }

    /* Managed ACL View */
    .managed-acl-view {
        padding: 20px;
        max-width: 1200px;
        margin: 0;
        background: var(--header-bg);
        color: var(--text-color);
        border-radius: 8px;
    }

    .compose-view {
        position: fixed;
        top: 3em;
        left: 200px;
        right: 0;
        bottom: 0;
        display: flex;
        flex-direction: column;
        background: transparent;
    }

    .compose-header {
        display: flex;
        gap: 0.5em;
        padding: 0.5em;
        background: transparent;
        border-bottom: 1px solid var(--border-color);
    }

    .compose-btn {
        padding: 0.5em 1em;
        border: 1px solid var(--border-color);
        border-radius: 0.25rem;
        background: var(--button-bg);
        color: var(--button-text);
        cursor: pointer;
        font-size: 0.9rem;
        transition:
            background-color 0.2s,
            border-color 0.2s;
    }

    .compose-btn:hover {
        background: var(--button-hover-bg);
        border-color: var(--button-hover-border);
    }

    .publish-btn {
        background: var(--accent-color, #007bff);
        color: white;
        border-color: var(--accent-color, #007bff);
    }

    .publish-btn:hover {
        background: var(--accent-hover-color, #0056b3);
        border-color: var(--accent-hover-color, #0056b3);
    }

    .compose-editor {
        flex: 1;
        padding: 0.5em;
    }

    .compose-textarea {
        width: 100%;
        height: 100%;
        border: 1px solid var(--border-color);
        border-radius: 0.25rem;
        background: var(--bg-color);
        color: var(--text-color);
        font-family: "Courier New", monospace;
        font-size: 0.9rem;
        padding: 1rem;
        resize: none;
        outline: none;
    }

    .compose-textarea:focus {
        border-color: var(--accent-color, #007bff);
        box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
    }

    .import-view h2 {
        margin: 0 0 2rem 0;
        color: var(--text-color);
        font-size: 1.5rem;
        font-weight: 600;
    }

    .export-section,
    .import-section {
        background: var(--header-bg);
        padding: 1em;
        border-radius: 8px;
        margin-bottom: 1.5rem;
        width: 32em;
    }

    .export-section h3,
    .import-section h3 {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        font-size: 1.2rem;
        font-weight: 500;
    }

    .export-section p,
    .import-section p {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        opacity: 0.8;
        line-height: 1.5;
    }

    .events-view-buttons {
        display: flex;
        gap: 0.5rem;
        align-items: center;
    }

    .export-btn,
    .import-btn,
    .refresh-btn,
    .reload-btn {
        padding: 0.5rem 1rem;
        background: var(--primary);
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.875rem;
        font-weight: 500;
        transition: background-color 0.2s;
        display: inline-flex;
        align-items: center;
        gap: 0.25rem;
        height: 2em;
    }

    .export-btn:hover,
    .import-btn:hover,
    .refresh-btn:hover,
    .reload-btn:hover {
        background: #00acc1;
    }

    .reload-btn {
        min-width: 2em;
        justify-content: center;
    }

    .spinner {
        width: 1em;
        height: 1em;
        border: 2px solid transparent;
        border-top: 2px solid currentColor;
        border-radius: 50%;
        animation: spin 1s linear infinite;
    }

    @keyframes spin {
        0% {
            transform: rotate(0deg);
        }
        100% {
            transform: rotate(360deg);
        }
    }

    .export-btn:disabled,
    .import-btn:disabled {
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
        padding: 1em;
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
        padding: 1em;
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

    /* Events View Container */
    .events-view-container {
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

    .events-view-header {
        position: absolute;
        bottom: 0;
        left: 0;
        right: 0;
        padding: 0.5rem 1rem;
        background: var(--header-bg);
        border-top: 1px solid var(--border-color);
        display: flex;
        justify-content: space-between;
        align-items: center;
        height: 2.5em;
        z-index: 10;
    }

    .events-view-toggle {
        flex: 1;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .toggle-container {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        cursor: pointer;
        font-size: 0.875rem;
        color: var(--text-color);
    }

    .toggle-container input[type="checkbox"] {
        display: none;
    }

    .toggle-slider {
        position: relative;
        width: 2.5em;
        height: 1.25em;
        background: var(--border-color);
        border-radius: 1.25em;
        transition: background-color 0.3s;
    }

    .toggle-slider::before {
        content: "";
        position: absolute;
        top: 0.125em;
        left: 0.125em;
        width: 1em;
        height: 1em;
        background: white;
        border-radius: 50%;
        transition: transform 0.3s;
    }

    .toggle-container input[type="checkbox"]:checked + .toggle-slider {
        background: var(--primary);
    }

    .toggle-container input[type="checkbox"]:checked + .toggle-slider::before {
        transform: translateX(1.25em);
    }

    .toggle-label {
        font-size: 0.875rem;
        font-weight: 500;
        user-select: none;
    }

    .events-view-content {
        flex: 1;
        overflow-y: auto;
        padding: 0;
    }

    .events-view-item {
        border-bottom: 1px solid var(--border-color);
        transition: background-color 0.2s;
    }

    .events-view-item:hover {
        background: var(--button-hover-bg);
    }

    .events-view-item.expanded {
        background: var(--button-hover-bg);
    }

    .events-view-row {
        display: flex;
        align-items: center;
        padding: 0.4rem 1rem;
        cursor: pointer;
        gap: 0.75rem;
        min-height: 2rem;
    }

    .events-view-avatar {
        flex-shrink: 0;
        width: 1.5rem;
        height: 1.5rem;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .avatar-placeholder {
        width: 1.5rem;
        height: 1.5rem;
        border-radius: 50%;
        background: var(--button-bg);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 0.7rem;
    }

    .events-view-info {
        flex-shrink: 0;
        width: 12rem;
        display: flex;
        flex-direction: column;
        gap: 0.1rem;
    }

    .events-view-author {
        font-family: monospace;
        font-size: 0.8rem;
        color: var(--text-color);
        opacity: 0.8;
    }

    .events-view-kind {
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

    .events-view-content {
        flex: 1;
        color: var(--text-color);
        font-size: 0.9rem;
        line-height: 1.3;
        word-break: break-word;
    }

    .event-timestamp {
        font-size: 0.75rem;
        color: var(--text-color);
        opacity: 0.7;
        margin-bottom: 0.25rem;
        font-weight: 500;
    }

    .event-content-single-line {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        line-height: 1.2;
    }

    .delete-btn {
        flex-shrink: 0;
        background: none;
        border: none;
        cursor: pointer;
        padding: 0.2rem;
        border-radius: 0.25rem;
        transition: background-color 0.2s;
        font-size: 1.6rem;
        display: flex;
        align-items: center;
        justify-content: center;
        width: 1.5rem;
        height: 1.5rem;
    }

    .delete-btn:hover {
        background: var(--warning);
        color: white;
    }

    .kind-number.delete-event {
        background: var(--warning);
    }

    .delete-event-info {
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    .delete-event-label {
        font-weight: 500;
        color: var(--warning);
    }

    .delete-targets {
        display: flex;
        flex-direction: column;
        gap: 0.125rem;
    }

    .delete-target {
        font-size: 0.75rem;
        font-family: monospace;
        color: var(--text-color);
        opacity: 0.7;
    }

    .events-view-details {
        border-top: 1px solid var(--border-color);
        background: var(--header-bg);
        padding: 1rem;
    }

    .json-container {
        position: relative;
    }

    .copy-json-btn {
        position: absolute;
        top: 0.5rem;
        right: 0.5rem;
        background: var(--button-bg);
        color: var(--button-text);
        border: 1px solid var(--border-color);
        border-radius: 0.25rem;
        padding: 0.5rem 1rem;
        font-size: 1.6rem;
        cursor: pointer;
        transition:
            background-color 0.2s,
            border-color 0.2s;
        z-index: 10;
        opacity: 0.8;
        width: auto;
        height: auto;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .copy-json-btn:hover {
        background: var(--button-hover-bg);
        border-color: var(--button-hover-border);
        opacity: 1;
    }

    .event-json {
        background: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 0.25rem;
        padding: 1rem;
        margin: 0;
        font-family: "Courier New", monospace;
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
        0% {
            transform: rotate(0deg);
        }
        100% {
            transform: rotate(360deg);
        }
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

    /* Search Results Styles */
    .search-results-view {
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

    .search-results-header {
        padding: 0.5rem 1rem;
        background: var(--header-bg);
        border-bottom: 1px solid var(--border-color);
        flex-shrink: 0;
        display: flex;
        justify-content: space-between;
        align-items: center;
        height: 2.5em;
    }

    .search-results-header h2 {
        margin: 0;
        font-size: 1rem;
        font-weight: 600;
        color: var(--text-color);
    }

    .search-results-content {
        flex: 1;
        overflow-y: auto;
        padding: 0;
    }

    .search-result-item {
        border-bottom: 1px solid var(--border-color);
        transition: background-color 0.2s;
    }

    .search-result-item:hover {
        background: var(--button-hover-bg);
    }

    .search-result-item.expanded {
        background: var(--button-hover-bg);
    }

    .search-result-row {
        display: flex;
        align-items: center;
        padding: 0.75rem 1rem;
        cursor: pointer;
        gap: 0.75rem;
        min-height: 3rem;
    }

    .search-result-avatar {
        flex-shrink: 0;
        width: 2rem;
        height: 2rem;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .search-result-info {
        flex-shrink: 0;
        width: 12rem;
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    .search-result-author {
        font-family: monospace;
        font-size: 0.8rem;
        color: var(--text-color);
        opacity: 0.8;
    }

    .search-result-kind {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .search-result-content {
        flex: 1;
        color: var(--text-color);
        font-size: 0.9rem;
        line-height: 1.3;
        word-break: break-word;
    }

    .search-result-content .event-timestamp {
        font-size: 0.75rem;
        color: var(--text-color);
        opacity: 0.7;
        margin-bottom: 0.25rem;
        font-weight: 500;
    }

    .search-result-content .event-content-single-line {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        line-height: 1.2;
    }

    .search-result-details {
        border-top: 1px solid var(--border-color);
        background: var(--header-bg);
        padding: 1rem;
    }

    .no-search-results {
        padding: 2rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.7;
    }

    .no-search-results p {
        margin: 0;
        font-size: 1rem;
    }

    .loading-search-results {
        padding: 2rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.7;
    }

    .loading-search-results p {
        margin: 0;
        font-size: 0.9rem;
    }

    .end-of-search-results {
        padding: 1rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.5;
        font-size: 0.8rem;
        border-top: 1px solid var(--border-color);
    }

    .end-of-search-results p {
        margin: 0;
    }

    @media (max-width: 1280px) {
        .sidebar {
            width: 60px;
        }

        .main-content {
            left: 60px;
        }

        .events-view-container {
            left: 60px;
        }

        .compose-view {
            left: 60px;
        }

        .search-results-view {
            left: 60px;
        }

        .tab-label {
            display: none;
        }

        .tab {
            justify-content: center;
            padding: 0 0.5rem;
        }

        .tab-icon {
            width: 2em;
            height: 2em;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 0.25rem;
            background-color: var(--bg-color);
        }

        .tab.active .tab-icon {
            background-color: var(--primary);
            color: white;
        }
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

        .profile-username {
            font-size: 1rem;
        }
        .profile-nip05-inline {
            font-size: 0.8rem;
        }

        .import-view {
            padding: 1em;
        }

        .managed-acl-view {
            padding: 1rem;
        }

        .export-section,
        .import-section {
            padding: 1em;
            width: 32em;
        }

        .events-view-container {
            left: 160px;
        }

        .compose-view {
            left: 160px;
        }

        .events-view-info {
            width: 8rem;
        }

        .events-view-author {
            font-size: 0.7rem;
        }

        .kind-name {
            font-size: 0.7rem;
        }

        .events-view-content {
            font-size: 0.8rem;
        }

        .search-results-view {
            left: 160px;
        }

        .search-result-info {
            width: 8rem;
        }

        .search-result-author {
            font-size: 0.7rem;
        }

        .search-result-content {
            font-size: 0.8rem;
        }
    }

    /* Recovery Tab Styles */
    .recovery-tab {
        padding: 20px;
        max-width: 1200px;
        margin: 0 auto;
    }

    .recovery-header {
        margin-bottom: 30px;
        text-align: center;
    }

    .recovery-header h2 {
        margin: 0 0 10px 0;
        color: var(--text-color);
    }

    .recovery-header p {
        margin: 0;
        color: var(--text-color);
        opacity: 0.7;
    }

    .recovery-controls {
        display: flex;
        gap: 20px;
        align-items: center;
        margin-bottom: 30px;
        padding: 20px;
        background: var(--bg-color);
        border-radius: 8px;
        flex-wrap: wrap;
    }

    .kind-selector {
        display: flex;
        flex-direction: column;
        gap: 5px;
    }

    .kind-selector label {
        font-weight: 500;
        color: var(--text-color);
    }

    .kind-selector select {
        padding: 8px 12px;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--bg-color);
        color: var(--text-color);
        min-width: 300px;
    }

    .custom-kind-input {
        display: flex;
        flex-direction: column;
        gap: 5px;
    }

    .custom-kind-input label {
        font-weight: 500;
        color: var(--text-color);
    }

    .custom-kind-input input {
        padding: 8px 12px;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--bg-color);
        color: var(--text-color);
        min-width: 200px;
    }

    .custom-kind-input input::placeholder {
        color: var(--text-color);
        opacity: 0.6;
    }

    .recovery-results {
        margin-top: 20px;
    }

    .loading,
    .no-events {
        text-align: center;
        padding: 40px;
        color: var(--text-color);
        opacity: 0.7;
    }

    .events-list {
        display: flex;
        flex-direction: column;
        gap: 15px;
    }

    .event-item {
        background: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        padding: 20px;
        transition: all 0.2s ease;
    }

    .event-item.old-version {
        opacity: 0.7;
        border-color: #ffc107;
        background: rgba(255, 193, 7, 0.1);
    }

    .event-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 15px;
        flex-wrap: wrap;
        gap: 10px;
    }

    .event-kind {
        font-weight: 600;
        color: var(--primary);
    }

    .event-timestamp {
        color: var(--text-color);
        font-size: 0.9em;
        opacity: 0.7;
    }

    .repost-button {
        background: var(--primary);
        color: white;
        border: none;
        padding: 6px 12px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        transition: background 0.2s ease;
    }

    .repost-button:hover {
        background: #00acc1;
    }

    .event-content {
        margin-bottom: 15px;
    }

    .profile-content pre,
    .generic-content pre {
        background: var(--bg-color);
        padding: 15px;
        border-radius: 4px;
        overflow-x: auto;
        font-size: 0.9em;
        margin: 0;
        border: 1px solid var(--border-color);
    }

    .follow-list p {
        margin: 0 0 10px 0;
        font-weight: 500;
        color: var(--text-color);
    }

    .follow-tags {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
    }

    .pubkey-tag {
        background: var(--bg-color);
        padding: 4px 8px;
        border-radius: 4px;
        font-family: monospace;
        font-size: 0.8em;
        border: 1px solid var(--border-color);
        color: var(--text-color);
    }

    .event-tags {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
    }

    .tag {
        background: var(--bg-color);
        padding: 4px 8px;
        border-radius: 4px;
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.7;
        border: 1px solid var(--border-color);
    }

    .load-more {
        width: 100%;
        padding: 12px;
        background: var(--primary);
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 1em;
        margin-top: 20px;
        transition: background 0.2s ease;
    }

    .load-more:hover:not(:disabled) {
        background: #00acc1;
    }

    .load-more:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    /* Dark theme adjustments for recovery tab */
    :global(body.dark-theme) .event-item.old-version {
        background: rgba(255, 193, 7, 0.1);
        border-color: #ffc107;
    }
</style>
