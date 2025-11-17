#!/bin/bash

# ORLY Policy Script Example
# This script demonstrates advanced policy logic including:
# - IP address blocking
# - Content filtering
# - Authentication requirements
# - User-specific permissions
# - Age validation (complementing built-in age checks)

# Configuration
BLOCKED_IPS=("127.0.0.1" "192.168.1.100")
BLOCKED_WORDS=("spam" "scam" "phishing")
TRUSTED_USERS=("746573742d7075626b6579" "abcdef1234567890abcdef1234567890abcdef12")
ADMIN_USERS=("746573742d7075626b6579")

# Function to check if IP is blocked
is_ip_blocked() {
    local ip="$1"
    for blocked_ip in "${BLOCKED_IPS[@]}"; do
        if [[ "$ip" == "$blocked_ip" ]]; then
            return 0
        fi
    done
    return 1
}

# Function to check for blocked words
contains_blocked_words() {
    local content="$1"
    local lower_content=$(echo "$content" | tr '[:upper:]' '[:lower:]')
    
    for word in "${BLOCKED_WORDS[@]}"; do
        if [[ "$lower_content" == *"$word"* ]]; then
            return 0
        fi
    done
    return 1
}

# Function to check if user is trusted
is_trusted_user() {
    local pubkey="$1"
    for trusted_user in "${TRUSTED_USERS[@]}"; do
        if [[ "$pubkey" == "$trusted_user" ]]; then
            return 0
        fi
    done
    return 1
}

# Function to check if user is admin
is_admin_user() {
    local pubkey="$1"
    for admin_user in "${ADMIN_USERS[@]}"; do
        if [[ "$pubkey" == "$admin_user" ]]; then
            return 0
        fi
    done
    return 1
}

# Function to validate event age (additional to built-in checks)
validate_event_age() {
    local created_at="$1"
    local current_time=$(date +%s)
    local age=$((current_time - created_at))
    
    # Additional age validation beyond built-in checks
    # Reject events older than 7 days for certain kinds
    if [[ $age -gt 604800 ]]; then
        return 1
    fi
    
    return 0
}

# Main policy logic
while IFS= read -r line; do
    # Parse JSON input
    event_id=$(echo "$line" | jq -r '.id // empty')
    pubkey=$(echo "$line" | jq -r '.pubkey // empty')
    kind=$(echo "$line" | jq -r '.kind // empty')
    content=$(echo "$line" | jq -r '.content // empty')
    created_at=$(echo "$line" | jq -r '.created_at // empty')
    logged_in_pubkey=$(echo "$line" | jq -r '.logged_in_pubkey // empty')
    ip_address=$(echo "$line" | jq -r '.ip_address // empty')
    
    # Default to accept
    action="accept"
    msg=""
    
    # Check IP blocking
    if is_ip_blocked "$ip_address"; then
        action="reject"
        msg="IP address blocked"
        echo "{\"id\":\"$event_id\",\"action\":\"$action\",\"msg\":\"$msg\"}"
        continue
    fi
    
    # Check for blocked words in content
    if contains_blocked_words "$content"; then
        action="reject"
        msg="Content contains blocked words"
        echo "{\"id\":\"$event_id\",\"action\":\"$action\",\"msg\":\"$msg\"}"
        continue
    fi
    
    # Additional age validation
    if ! validate_event_age "$created_at"; then
        action="reject"
        msg="Event too old (additional validation)"
        echo "{\"id\":\"$event_id\",\"action\":\"$action\",\"msg\":\"$msg\"}"
        continue
    fi
    
    # Kind-specific rules
    case "$kind" in
        "4")  # Direct messages
            # Require authentication for DMs
            if [[ -z "$logged_in_pubkey" ]]; then
                action="reject"
                msg="Authentication required for direct messages"
            fi
            ;;
        "40"|"41"|"42"|"43"|"44")  # Channel events
            # Require authentication for channel events
            if [[ -z "$logged_in_pubkey" ]]; then
                action="reject"
                msg="Authentication required for channel events"
            fi
            ;;
        "9735")  # Zap receipts
            # Only allow trusted users to post zap receipts
            if ! is_trusted_user "$pubkey"; then
                action="reject"
                msg="Only trusted users can post zap receipts"
            fi
            ;;
    esac
    
    # Admin bypass for certain operations
    if is_admin_user "$pubkey"; then
        # Admins can bypass most restrictions
        action="accept"
        msg="Admin bypass"
    fi
    
    # Output decision
    echo "{\"id\":\"$event_id\",\"action\":\"$action\",\"msg\":\"$msg\"}"
    
done