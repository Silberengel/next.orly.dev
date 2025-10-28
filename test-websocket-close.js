import { NostrWebSocket } from '@nostr-dev-kit/ndk';

const RELAY = process.env.RELAY || 'ws://localhost:8080';

async function testConnectionClosure() {
    console.log('Testing websocket connection closure issues...');
    console.log('Connecting to:', RELAY);

    // Create multiple connections to test concurrency
    const connections = [];
    const results = { connected: 0, closed: 0, errors: 0 };

    for (let i = 0; i < 5; i++) {
        const ws = new NostrWebSocket(RELAY);
        
        ws.addEventListener('open', () => {
            console.log(`Connection ${i} opened`);
            results.connected++;
        });

        ws.addEventListener('close', (event) => {
            console.log(`Connection ${i} closed:`, event.code, event.reason);
            results.closed++;
        });

        ws.addEventListener('error', (error) => {
            console.error(`Connection ${i} error:`, error);
            results.errors++;
        });

        connections.push(ws);
    }

    // Wait a bit then send REQs
    await new Promise(resolve => setTimeout(resolve, 1000));

    // Send some REQ messages
    for (const ws of connections) {
        ws.send(JSON.stringify(['REQ', 'test-sub', { kinds: [1] }]));
    }

    // Wait and observe behavior
    await new Promise(resolve => setTimeout(resolve, 5000));

    console.log('\nTest Results:');
    console.log(`- Connected: ${results.connected}`);
    console.log(`- Closed prematurely: ${results.closed}`);
    console.log(`- Errors: ${results.errors}`);

    // Close all connections
    for (const ws of connections) {
        ws.close();
    }
}

testConnectionClosure().catch(console.error);

