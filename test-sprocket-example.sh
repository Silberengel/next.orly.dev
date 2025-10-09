#!/bin/bash

# Example sprocket script that demonstrates event processing
# This script reads JSON events from stdin and outputs JSONL responses

# Read events from stdin line by line
while IFS= read -r line; do
    # Parse the event JSON
    event_id=$(echo "$line" | jq -r '.id')
    event_kind=$(echo "$line" | jq -r '.kind')
    event_content=$(echo "$line" | jq -r '.content')
    
    # Example policy: reject events with certain content
    if [[ "$event_content" == *"spam"* ]]; then
        echo "{\"id\":\"$event_id\",\"action\":\"reject\",\"msg\":\"content contains spam\"}"
        continue
    fi
    
    # Example policy: shadow reject events from certain kinds
    if [[ "$event_kind" == "9999" ]]; then
        echo "{\"id\":\"$event_id\",\"action\":\"shadowReject\",\"msg\":\"\"}"
        continue
    fi
    
    # Default: accept the event
    echo "{\"id\":\"$event_id\",\"action\":\"accept\",\"msg\":\"\"}"
    
done
