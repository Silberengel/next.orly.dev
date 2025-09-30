#!/bin/bash
# WebSocket Debug Script for Stella's Orly Relay

echo "🔍 Debugging WebSocket Connection for orly-relay.imwald.eu"
echo "=================================================="

echo ""
echo "📋 Step 1: Check if relay container is running"
echo "----------------------------------------------"
docker ps | grep -E "(stella|relay|orly)" || echo "❌ No relay containers found"

echo ""
echo "📋 Step 2: Test local relay connection"
echo "--------------------------------------"
if curl -s -I http://127.0.0.1:7777 | grep -q "426"; then
    echo "✅ Local relay responding correctly (HTTP 426)"
else
    echo "❌ Local relay not responding correctly"
    curl -I http://127.0.0.1:7777
fi

echo ""
echo "📋 Step 3: Check Apache modules"
echo "------------------------------"
if apache2ctl -M 2>/dev/null | grep -q "proxy_wstunnel"; then
    echo "✅ proxy_wstunnel module enabled"
else
    echo "❌ proxy_wstunnel module NOT enabled"
    echo "Run: sudo a2enmod proxy_wstunnel"
fi

if apache2ctl -M 2>/dev/null | grep -q "rewrite"; then
    echo "✅ rewrite module enabled"
else
    echo "❌ rewrite module NOT enabled"
    echo "Run: sudo a2enmod rewrite"
fi

echo ""
echo "📋 Step 4: Check Plesk Apache configuration"
echo "------------------------------------------"
if [ -f "/etc/apache2/plesk.conf.d/vhosts/orly-relay.imwald.eu.conf" ]; then
    echo "✅ Plesk config file exists"
    echo "Current proxy configuration:"
    grep -E "(Proxy|Rewrite|proxy|rewrite)" /etc/apache2/plesk.conf.d/vhosts/orly-relay.imwald.eu.conf || echo "❌ No proxy/rewrite rules found"
else
    echo "❌ Plesk config file not found"
fi

echo ""
echo "📋 Step 5: Test WebSocket connections"
echo "------------------------------------"

# Test with curl first (simpler)
echo "Testing HTTP upgrade request to local relay..."
if curl -s -I -H "Connection: Upgrade" -H "Upgrade: websocket" http://127.0.0.1:7777 | grep -q "426\|101"; then
    echo "✅ Local relay accepts upgrade requests"
else
    echo "❌ Local relay doesn't accept upgrade requests"
fi

echo "Testing HTTP upgrade request to remote relay..."
if curl -s -I -H "Connection: Upgrade" -H "Upgrade: websocket" https://orly-relay.imwald.eu | grep -q "426\|101"; then
    echo "✅ Remote relay accepts upgrade requests"
else
    echo "❌ Remote relay doesn't accept upgrade requests"
    echo "This indicates Apache proxy issue"
fi

# Try to install websocat if not available
if ! command -v websocat >/dev/null 2>&1; then
    echo ""
    echo "📥 Installing websocat for proper WebSocket testing..."
    if wget -q https://github.com/vi/websocat/releases/download/v1.12.0/websocat.x86_64-unknown-linux-musl -O websocat 2>/dev/null; then
        chmod +x websocat
        echo "✅ websocat installed"
    else
        echo "❌ Could not install websocat (no internet or wget issue)"
        echo "Manual install: wget https://github.com/vi/websocat/releases/download/v1.12.0/websocat.x86_64-unknown-linux-musl -O websocat && chmod +x websocat"
    fi
fi

# Test with websocat if available
if command -v ./websocat >/dev/null 2>&1; then
    echo ""
    echo "Testing actual WebSocket connection..."
    echo "Local WebSocket test:"
    timeout 3 bash -c 'echo "[\"REQ\",\"test\",{}]" | ./websocat ws://127.0.0.1:7777/' 2>/dev/null || echo "❌ Local WebSocket failed"
    
    echo "Remote WebSocket test (ignoring SSL):"
    timeout 3 bash -c 'echo "[\"REQ\",\"test\",{}]" | ./websocat --insecure wss://orly-relay.imwald.eu/' 2>/dev/null || echo "❌ Remote WebSocket failed"
fi

echo ""
echo "📋 Step 6: Check ports and connections"
echo "------------------------------------"
echo "Ports listening on 7777:"
netstat -tlnp 2>/dev/null | grep :7777 || ss -tlnp 2>/dev/null | grep :7777 || echo "❌ No process listening on port 7777"

echo ""
echo "📋 Step 7: Test SSL certificate"
echo "------------------------------"
echo "Certificate issuer:"
echo | openssl s_client -connect orly-relay.imwald.eu:443 -servername orly-relay.imwald.eu 2>/dev/null | openssl x509 -noout -issuer 2>/dev/null || echo "❌ SSL test failed"

echo ""
echo "🎯 RECOMMENDED NEXT STEPS:"
echo "========================="
echo "1. If proxy_wstunnel is missing: sudo a2enmod proxy_wstunnel && sudo systemctl restart apache2"
echo "2. If no proxy rules found: Add configuration in Plesk Apache & nginx Settings"
echo "3. If local WebSocket fails: Check if relay container is actually running"
echo "4. If remote WebSocket fails but local works: Apache proxy configuration issue"
echo ""
echo "🔧 Try this simple Plesk configuration:"
echo "ProxyPass / http://127.0.0.1:7777/"
echo "ProxyPassReverse / http://127.0.0.1:7777/"
