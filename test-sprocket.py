#!/usr/bin/env python3
"""
Test sprocket script that processes Nostr events via stdin/stdout JSONL protocol.
This script demonstrates various filtering criteria for testing purposes.
"""

import json
import sys
import re
from datetime import datetime

def process_event(event_json):
    """
    Process a single event and return the appropriate response.
    
    Args:
        event_json (dict): The parsed event JSON
        
    Returns:
        dict: Response with id, action, and msg fields
    """
    event_id = event_json.get('id', '')
    event_kind = event_json.get('kind', 0)
    event_content = event_json.get('content', '')
    event_pubkey = event_json.get('pubkey', '')
    event_tags = event_json.get('tags', [])
    
    # Test criteria 1: Reject events containing "spam" in content
    if 'spam' in event_content.lower():
        return {
            'id': event_id,
            'action': 'reject',
            'msg': 'Content contains spam'
        }
    
    # Test criteria 2: Shadow reject events with kind 9999 (test kind)
    if event_kind == 9999:
        return {
            'id': event_id,
            'action': 'shadowReject',
            'msg': ''
        }
    
    # Test criteria 3: Reject events with certain hashtags
    for tag in event_tags:
        if len(tag) >= 2 and tag[0] == 't':  # hashtag
            hashtag = tag[1].lower()
            if hashtag in ['blocked', 'rejected', 'test-block']:
                return {
                    'id': event_id,
                    'action': 'reject',
                    'msg': f'Hashtag "{hashtag}" is not allowed'
                }
    
    # Test criteria 4: Shadow reject events from specific pubkeys (first 8 chars)
    blocked_prefixes = ['00000000', '11111111', '22222222']  # Test prefixes
    pubkey_prefix = event_pubkey[:8] if len(event_pubkey) >= 8 else event_pubkey
    if pubkey_prefix in blocked_prefixes:
        return {
            'id': event_id,
            'action': 'shadowReject',
            'msg': ''
        }
    
    # Test criteria 5: Reject events that are too long
    if len(event_content) > 1000:
        return {
            'id': event_id,
            'action': 'reject',
            'msg': 'Content too long (max 1000 characters)'
        }
    
    # Test criteria 6: Reject events with invalid timestamps (too old or too new)
    try:
        event_time = event_json.get('created_at', 0)
        current_time = int(datetime.now().timestamp())
        
        # Reject events more than 1 hour old
        if current_time - event_time > 3600:
            return {
                'id': event_id,
                'action': 'reject',
                'msg': 'Event timestamp too old'
            }
        
        # Reject events more than 5 minutes in the future
        if event_time - current_time > 300:
            return {
                'id': event_id,
                'action': 'reject',
                'msg': 'Event timestamp too far in future'
            }
    except (ValueError, TypeError):
        pass  # Ignore timestamp errors
    
    # Default: accept the event
    return {
        'id': event_id,
        'action': 'accept',
        'msg': ''
    }

def main():
    """Main function to process events from stdin."""
    try:
        # Read events from stdin
        for line in sys.stdin:
            line = line.strip()
            if not line:
                continue
                
            try:
                # Parse the event JSON
                event = json.loads(line)
                
                # Process the event
                response = process_event(event)
                
                # Output the response as JSONL
                print(json.dumps(response), flush=True)
                
            except json.JSONDecodeError as e:
                # Log error to stderr but continue processing
                print(f"Error parsing JSON: {e}", file=sys.stderr)
                continue
            except Exception as e:
                # Log error to stderr but continue processing
                print(f"Error processing event: {e}", file=sys.stderr)
                continue
                
    except KeyboardInterrupt:
        # Graceful shutdown
        sys.exit(0)
    except Exception as e:
        print(f"Fatal error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()
