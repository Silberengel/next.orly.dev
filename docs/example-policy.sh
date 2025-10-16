#!/bin/bash

# Policy script example for ORLY relay
# This script receives JSON events via stdin and outputs JSON responses via stdout
# Each event includes the original event data plus logged_in_pubkey and ip_address fields

# Read events from stdin (JSONL format)
while IFS= read -r line; do
    # Parse the JSON event
    event_id=$(echo "$line" | jq -r '.id // empty')
    event_kind=$(echo "$line" | jq -r '.kind // empty')
    event_pubkey=$(echo "$line" | jq -r '.pubkey // empty')
    event_content=$(echo "$line" | jq -r '.content // empty')
    logged_in_pubkey=$(echo "$line" | jq -r '.logged_in_pubkey // empty')
    ip_address=$(echo "$line" | jq -r '.ip_address // empty')
    
    # Default action
    action="accept"
    message=""
    
    # Example policy logic:
    # 1. Block events from specific IP addresses
    if [[ "$ip_address" == "192.168.1.100" ]]; then
        action="reject"
        message="blocked IP address"
    fi
    
    # 2. Block events with certain content patterns
    if [[ "$event_content" =~ "spam" ]]; then
        action="reject"
        message="spam content detected"
    fi
    
    # 3. Require authentication for certain kinds
    if [[ "$event_kind" == "3" && -z "$logged_in_pubkey" ]]; then
        action="reject"
        message="authentication required for kind 3"
    fi
    
    # 4. Allow only specific users for kind 3
    if [[ "$event_kind" == "3" && "$event_pubkey" != "npub1example1" && "$event_pubkey" != "npub1example2" ]]; then
        action="reject"
        message="unauthorized user for kind 3"
    fi
    
    # Output JSON response
    echo "{\"id\":\"$event_id\",\"action\":\"$action\",\"msg\":\"$message\"}"
done
