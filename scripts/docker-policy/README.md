# ORLY Policy Engine Docker Test

This directory contains a Docker-based test environment to verify that the `cs-policy.js` script is executed by the ORLY relay's policy engine when events are received.

## Test Structure

```
test-docker-policy/
├── Dockerfile           # Ubuntu 22.04.5 based image
├── docker-compose.yml   # Container orchestration
├── cs-policy.js         # Policy script that writes to a file
├── policy.json          # Policy configuration pointing to the script
├── env                  # Environment variables for ORLY
├── start.sh            # Container startup script
├── test-policy.sh      # Automated test runner
└── README.md           # This file
```

## What the Test Does

1. **Builds** an Ubuntu 22.04.5 Docker image with ORLY relay
2. **Configures** the policy engine with `cs-policy.js`
3. **Starts** the relay with policy engine enabled
4. **Sends** a test event to the relay
5. **Verifies** that `cs-policy.js` created `/home/orly/cs-policy-output.txt`
6. **Reports** success or failure

## How cs-policy.js Works

The policy script writes a timestamped message to `/home/orly/cs-policy-output.txt` each time it's executed:

```javascript
#!/usr/bin/env node
const fs = require('fs')
const filePath = '/home/orly/cs-policy-output.txt'

if (fs.existsSync(filePath)) {
    fs.appendFileSync(filePath, `${Date.now()}: Hey there!\n`)
} else {
    fs.writeFileSync(filePath, `${Date.now()}: Hey there!\n`)
}
```

## Quick Start

Run the automated test:

```bash
./test-docker-policy/test-policy.sh
```

## Manual Testing

### 1. Build and Start Container

```bash
cd /home/mleku/src/next.orly.dev
docker-compose -f test-docker-policy/docker-compose.yml up -d
```

### 2. Check Relay Logs

```bash
docker logs orly-policy-test -f
```

### 3. Send Test Event

```bash
# Using websocat
echo '["EVENT",{"id":"test123","pubkey":"4db2c42f3c02079dd6feae3f88f6c8693940a00ade3cc8e5d72050bd6e577cd5","created_at":'$(date +%s)',"kind":1,"tags":[],"content":"Test","sig":"0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"}]' | websocat ws://localhost:8777
```

### 4. Verify Output File

```bash
# Check if file exists
docker exec orly-policy-test test -f /home/orly/cs-policy-output.txt && echo "File exists!"

# View contents
docker exec orly-policy-test cat /home/orly/cs-policy-output.txt
```

### 5. Cleanup

```bash
# Stop container
docker-compose -f test-docker-policy/docker-compose.yml down

# Remove volumes
docker-compose -f test-docker-policy/docker-compose.yml down -v
```

## Troubleshooting

### Policy Script Not Running

Check if policy is enabled:
```bash
docker exec orly-policy-test cat /home/orly/env | grep POLICY
```

Check policy configuration:
```bash
docker exec orly-policy-test cat /home/orly/.config/ORLY/policy.json
```

### Node.js Issues

Verify Node.js is installed:
```bash
docker exec orly-policy-test node --version
```

Test the script manually:
```bash
docker exec orly-policy-test node /home/orly/cs-policy.js
docker exec orly-policy-test cat /home/orly/cs-policy-output.txt
```

### Relay Not Starting

View full logs:
```bash
docker logs orly-policy-test
```

Check if relay is listening:
```bash
docker exec orly-policy-test netstat -tlnp | grep 8777
```

## Expected Output

When successful, you should see:

```
✓ SUCCESS: cs-policy-output.txt file exists!

Output file contents:
1704123456789: Hey there!

✓ Policy script is working correctly!
```

Each line in the output file represents one execution of the policy script, with a Unix timestamp.

## Configuration Files

### env
Environment variables for ORLY relay:
- `ORLY_PORT=8777` - WebSocket port
- `ORLY_POLICY_ENABLED=true` - Enable policy engine
- `ORLY_LOG_LEVEL=debug` - Verbose logging

### policy.json
Policy configuration:
```json
{
  "script": "/home/orly/cs-policy.js"
}
```

Points to the policy script that will be executed for each event.
