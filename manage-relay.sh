#!/bin/bash
# Stella's Orly Relay Management Script

set -e

RELAY_SERVICE="stella-relay"
RELAY_URL="ws://127.0.0.1:7777"

case "${1:-}" in
    "start")
        echo "🚀 Starting Stella's Orly Relay..."
        sudo systemctl start $RELAY_SERVICE
        echo "✅ Relay started!"
        ;;
    "stop")
        echo "⏹️  Stopping Stella's Orly Relay..."
        sudo systemctl stop $RELAY_SERVICE
        echo "✅ Relay stopped!"
        ;;
    "restart")
        echo "🔄 Restarting Stella's Orly Relay..."
        sudo systemctl restart $RELAY_SERVICE
        echo "✅ Relay restarted!"
        ;;
    "status")
        echo "📊 Stella's Orly Relay Status:"
        sudo systemctl status $RELAY_SERVICE --no-pager
        ;;
    "logs")
        echo "📜 Stella's Orly Relay Logs:"
        sudo journalctl -u $RELAY_SERVICE -f --no-pager
        ;;
    "test")
        echo "🧪 Testing relay connection..."
        if curl -s -I http://127.0.0.1:7777 | grep -q "426 Upgrade Required"; then
            echo "✅ Relay is responding correctly!"
            echo "📡 WebSocket URL: $RELAY_URL"
        else
            echo "❌ Relay is not responding correctly"
            exit 1
        fi
        ;;
    "enable")
        echo "🔧 Enabling relay to start at boot..."
        sudo systemctl enable $RELAY_SERVICE
        echo "✅ Relay will start automatically at boot!"
        ;;
    "disable")
        echo "🔧 Disabling relay auto-start..."
        sudo systemctl disable $RELAY_SERVICE
        echo "✅ Relay will not start automatically at boot!"
        ;;
    "info")
        echo "📋 Stella's Orly Relay Information:"
        echo "   Service: $RELAY_SERVICE"
        echo "   WebSocket URL: $RELAY_URL"
        echo "   HTTP URL: http://127.0.0.1:7777"
        echo "   Data Directory: /home/madmin/.local/share/orly-relay"
        echo "   Config Directory: $(pwd)"
        echo ""
        echo "🔑 Admin NPubs:"
        echo "   Stella: npub1v30tsz9vw6ylpz63g0a702nj3xa26t3m7p5us8f2y2sd8v6cnsvq465zjx"
        echo "   Admin2: npub1l5sga6xg72phsz5422ykujprejwud075ggrr3z2hwyrfgr7eylqstegx9z"
        ;;
    *)
        echo "🌲 Stella's Orly Relay Management Script"
        echo ""
        echo "Usage: $0 [COMMAND]"
        echo ""
        echo "Commands:"
        echo "  start     Start the relay"
        echo "  stop      Stop the relay"
        echo "  restart   Restart the relay"
        echo "  status    Show relay status"
        echo "  logs      Show relay logs (follow mode)"
        echo "  test      Test relay connection"
        echo "  enable    Enable auto-start at boot"
        echo "  disable   Disable auto-start at boot"
        echo "  info      Show relay information"
        echo ""
        echo "Examples:"
        echo "  $0 start      # Start the relay"
        echo "  $0 status     # Check if it's running"
        echo "  $0 test       # Test WebSocket connection"
        echo "  $0 logs       # Watch real-time logs"
        echo ""
        echo "🌲 Crafted in the digital forest by Stella ✨"
        ;;
esac
