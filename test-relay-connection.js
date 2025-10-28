#!/usr/bin/env node

// Test script to verify websocket connections are not closed prematurely
// This is a Node.js test script that can be run with: node test-relay-connection.js

import { NostrWebSocket } from '@nostr-dev-kit/ndk';

const RELAY = process.env.RELAY || 'ws://localhost:8080';
const MAX_CONNECTIONS = 10;
const TEST_DURATION = 30000; // 30 seconds

let connectionsClosed = 0;
let connectionsOpened = 0;
let messagesReceived = 0;
let errors = 0;

const stats = {
    premature: 0,
    normal: 0,
    errors: 0,
};

class TestConnection {
    constructor(id) {
        this.id = id;
        this.ws = null;
        this.closed = false;
        this.openTime = null;
        this.closeTime = null;
        this.lastError = null;
    }

    connect() {
        return new Promise((resolve, reject) => {
            this.ws = new NostrWebSocket(RELAY);

            this.ws.addEventListener('open', () => {
                this.openTime = Date.now();
                connectionsOpened++;
                console.log(`[Connection ${this.id}] Opened`);
                resolve();
            });

            this.ws.addEventListener('close', (event) => {
                this.closeTime = Date.now();
                this.closed = true;
                connectionsClosed++;
                const duration = this.closeTime - this.openTime;
                console.log(`[Connection ${this.id}] Closed: code=${event.code}, reason="${event.reason || ''}", duration=${duration}ms`);
                
                if (duration < 5000 && event.code !== 1000) {
                    stats.premature++;
                    console.log(`[Connection ${this.id}] PREMATURE CLOSE DETECTED: duration=${duration}ms < 5s`);
                } else {
                    stats.normal++;
                }
            });

            this.ws.addEventListener('error', (error) => {
                this.lastError = error;
                stats.errors++;
                console.error(`[Connection ${this.id}] Error:`, error);
            });

            this.ws.addEventListener('message', (event) => {
                messagesReceived++;
                try {
                    const data = JSON.parse(event.data);
                    console.log(`[Connection ${this.id}] Message:`, data[0]);
                } catch (e) {
                    console.log(`[Connection ${this.id}] Message (non-JSON):`, event.data);
                }
            });

            setTimeout(reject, 5000); // Timeout after 5 seconds if not opened
        });
    }

    sendReq() {
        if (this.ws && !this.closed) {
            this.ws.send(JSON.stringify(['REQ', `test-sub-${this.id}`, { kinds: [1], limit: 10 }]));
            console.log(`[Connection ${this.id}] Sent REQ`);
        }
    }

    close() {
        if (this.ws && !this.closed) {
            this.ws.close();
        }
    }
}

async function runTest() {
    console.log('='.repeat(60));
    console.log('Testing Relay Connection Stability');
    console.log('='.repeat(60));
    console.log(`Relay: ${RELAY}`);
    console.log(`Duration: ${TEST_DURATION}ms`);
    console.log(`Connections: ${MAX_CONNECTIONS}`);
    console.log('='.repeat(60));
    console.log();

    const connections = [];

    // Open connections
    console.log('Opening connections...');
    for (let i = 0; i < MAX_CONNECTIONS; i++) {
        const conn = new TestConnection(i);
        try {
            await conn.connect();
            connections.push(conn);
        } catch (error) {
            console.error(`Failed to open connection ${i}:`, error);
        }
    }

    console.log(`Opened ${connections.length} connections`);
    console.log();

    // Send requests from each connection
    console.log('Sending REQ messages...');
    for (const conn of connections) {
        conn.sendReq();
    }

    // Wait and let connections run
    console.log(`Waiting ${TEST_DURATION / 1000}s...`);
    await new Promise(resolve => setTimeout(resolve, TEST_DURATION));

    // Close all connections
    console.log('Closing all connections...');
    for (const conn of connections) {
        conn.close();
    }

    // Wait for close events
    await new Promise(resolve => setTimeout(resolve, 1000));

    // Print results
    console.log();
    console.log('='.repeat(60));
    console.log('Test Results:');
    console.log('='.repeat(60));
    console.log(`Connections Opened: ${connectionsOpened}`);
    console.log(`Connections Closed: ${connectionsClosed}`);
    console.log(`Messages Received: ${messagesReceived}`);
    console.log();
    console.log('Closure Analysis:');
    console.log(`- Premature Closes: ${stats.premature}`);
    console.log(`- Normal Closes: ${stats.normal}`);
    console.log(`- Errors: ${stats.errors}`);
    console.log('='.repeat(60));

    if (stats.premature > 0) {
        console.error('FAILED: Detected premature connection closures!');
        process.exit(1);
    } else {
        console.log('PASSED: No premature connection closures detected.');
        process.exit(0);
    }
}

runTest().catch(error => {
    console.error('Test failed:', error);
    process.exit(1);
});

