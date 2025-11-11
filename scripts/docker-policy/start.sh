#!/bin/bash

# Export environment variables
export $(cat /home/orly/env | xargs)

# Make cs-policy.js executable
chmod +x /home/orly/cs-policy.js

# Start the relay
exec /home/orly/.local/bin/orly
