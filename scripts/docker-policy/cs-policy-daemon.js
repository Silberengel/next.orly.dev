#!/usr/bin/env node

const fs = require('fs');
const readline = require('readline');

const filePath = '/home/orly/cs-policy-output.txt';

// Create readline interface to read from stdin
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false
});

// Log that script started
fs.appendFileSync(filePath, `${Date.now()}: Policy script started\n`);

// Process each line of input (policy events)
rl.on('line', (line) => {
  try {
    // Log that we received an event
    fs.appendFileSync(filePath, `${Date.now()}: Received event: ${line.substring(0, 100)}...\n`);

    // Parse the policy event
    const event = JSON.parse(line);

    // Log event details including access type
    const accessType = event.access_type || 'unknown';
    const eventKind = event.kind || 'unknown';
    fs.appendFileSync(filePath, `${Date.now()}: Event ID: ${event.id || 'unknown'}, Kind: ${eventKind}, Access: ${accessType}\n`);

    // Respond with "accept" to allow the event
    const response = {
      id: event.id,
      action: "accept",
      msg: ""
    };

    console.log(JSON.stringify(response));
  } catch (err) {
    // Log errors
    fs.appendFileSync(filePath, `${Date.now()}: Error: ${err.message}\n`);

    // Reject on error
    console.log(JSON.stringify({
      action: "reject",
      msg: "Policy script error"
    }));
  }
});

rl.on('close', () => {
  fs.appendFileSync(filePath, `${Date.now()}: Policy script stopped\n`);
});
