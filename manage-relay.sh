#!/bin/bash
# Stella's Orly Relay Management Script
# Uses docker-compose.yml directly for configuration

set -e

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SCRIPT_DIR"

# Configuration from docker-compose.yml
RELAY_SERVICE="stella-relay"
CONTAINER_NAME="stella-nostr-relay"
RELAY_URL="ws://127.0.0.1:7777"
HTTP_URL="http://127.0.0.1:7777"
RELAY_DATA_DIR="/home/madmin/.local/share/orly-relay"

# Change to project directory for docker-compose commands
cd "$PROJECT_DIR"

case "${1:-}" in
    "start")
        echo "🚀 Starting Stella's Orly Relay..."
        docker compose up -d stella-relay
        echo "✅ Relay started!"
        ;;
    "stop")
        echo "⏹️  Stopping Stella's Orly Relay..."
        docker compose down
        echo "✅ Relay stopped!"
        ;;
    "restart")
        echo "🔄 Restarting Stella's Orly Relay..."
        docker compose restart stella-relay
        echo "✅ Relay restarted!"
        ;;
    "status")
        echo "📊 Stella's Orly Relay Status:"
        docker compose ps stella-relay
        ;;
    "logs")
        echo "📜 Stella's Orly Relay Logs:"
        docker compose logs -f stella-relay
        ;;
    "test")
        echo "🧪 Testing relay connection..."
        if curl -s -I "$HTTP_URL" | grep -q "426 Upgrade Required"; then
            echo "✅ Relay is responding correctly!"
            echo "📡 WebSocket URL: $RELAY_URL"
            echo "🌐 HTTP URL: $HTTP_URL"
        else
            echo "❌ Relay is not responding correctly"
            echo "   Expected: 426 Upgrade Required"
            echo "   URL: $HTTP_URL"
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
        echo "   Container: $CONTAINER_NAME"
        echo "   WebSocket URL: $RELAY_URL"
        echo "   HTTP URL: $HTTP_URL"
        echo "   Data Directory: $RELAY_DATA_DIR"
        echo "   Config Directory: $PROJECT_DIR"
        echo ""
        echo "🐳 Docker Information:"
        echo "   Compose File: $PROJECT_DIR/docker-compose.yml"
        echo "   Container Status:"
        docker compose ps stella-relay 2>/dev/null || echo "   Not running"
        echo ""
        echo "💡 Configuration:"
        echo "   All settings are defined in docker-compose.yml"
        echo "   Use 'docker compose config' to see parsed configuration"
        ;;
    "docker-logs")
        echo "🐳 Docker Container Logs:"
        docker compose logs -f stella-relay 2>/dev/null || echo "❌ Container not found or not running"
        ;;
    "docker-status")
        echo "🐳 Docker Container Status:"
        docker compose ps stella-relay
        ;;
    "docker-restart")
        echo "🔄 Restarting Docker Container..."
        docker compose restart stella-relay
        echo "✅ Container restarted!"
        ;;
    "docker-update")
        echo "🔄 Updating and restarting Docker Container..."
        docker compose pull stella-relay
        docker compose up -d stella-relay
        echo "✅ Container updated and restarted!"
        ;;
    "docker-build")
        echo "🔨 Building Docker Container..."
        docker compose build stella-relay
        echo "✅ Container built!"
        ;;
    "docker-down")
        echo "⏹️  Stopping Docker Container..."
        docker compose down
        echo "✅ Container stopped!"
        ;;
    "docker-config")
        echo "📋 Docker Compose Configuration:"
        docker compose config
        ;;
    *)
        echo "🌲 Stella's Orly Relay Management Script"
        echo ""
        echo "Usage: $0 [COMMAND]"
        echo ""
        echo "Commands:"
        echo "  start          Start the relay"
        echo "  stop           Stop the relay"
        echo "  restart        Restart the relay"
        echo "  status         Show relay status"
        echo "  logs           Show relay logs (follow mode)"
        echo "  test           Test relay connection"
        echo "  enable         Enable auto-start at boot"
        echo "  disable        Disable auto-start at boot"
        echo "  info           Show relay information"
        echo ""
        echo "Docker Commands:"
        echo "  docker-logs    Show Docker container logs"
        echo "  docker-status Show Docker container status"
        echo "  docker-restart Restart Docker container only"
        echo "  docker-update Update and restart container"
        echo "  docker-build  Build Docker container"
        echo "  docker-down   Stop Docker container"
        echo "  docker-config Show Docker Compose configuration"
        echo ""
        echo "Examples:"
        echo "  $0 start          # Start the relay"
        echo "  $0 status         # Check if it's running"
        echo "  $0 test           # Test WebSocket connection"
        echo "  $0 logs           # Watch real-time logs"
        echo "  $0 docker-logs    # Watch Docker container logs"
        echo "  $0 docker-update  # Update and restart container"
        echo ""
        echo "🌲 Crafted in the digital forest by Stella ✨"
        ;;
esac
