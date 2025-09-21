# WebSocket Connection Debug Guide

## 🚨 **Current Issue**
`wss://orly-relay.imwald.eu/` returns `NS_ERROR_WEBSOCKET_CONNECTION_REFUSED`

## 🔍 **Debug Steps**

### **Step 1: Verify Relay is Running**
```bash
# On your server
curl -I http://127.0.0.1:7777
# Should return: HTTP/1.1 426 Upgrade Required

docker ps | grep stella
# Should show running container
```

### **Step 2: Test Apache Modules**
```bash
# Check if WebSocket modules are enabled
apache2ctl -M | grep -E "(proxy|rewrite)"

# If missing, enable them:
sudo a2enmod proxy
sudo a2enmod proxy_http
sudo a2enmod proxy_wstunnel
sudo a2enmod rewrite
sudo a2enmod headers
sudo systemctl restart apache2
```

### **Step 3: Check Apache Configuration**
```bash
# Check what Plesk generated
sudo cat /etc/apache2/plesk.conf.d/vhosts/orly-relay.imwald.eu.conf

# Look for proxy and rewrite rules
grep -E "(Proxy|Rewrite)" /etc/apache2/plesk.conf.d/vhosts/orly-relay.imwald.eu.conf
```

### **Step 4: Test Direct WebSocket Connection**
```bash
# Test if the issue is Apache or the relay itself
echo '["REQ","test",{}]' | websocat ws://127.0.0.1:7777/

# If that works, the issue is Apache proxy
# If that fails, the issue is the relay
```

### **Step 5: Check Apache Error Logs**
```bash
# Watch Apache errors in real-time
sudo tail -f /var/log/apache2/error.log

# Then try connecting to wss://orly-relay.imwald.eu/ and see what errors appear
```

## 🔧 **Specific Plesk Fix**

Based on your current status, try this **exact configuration** in Plesk:

### **Go to Apache & nginx Settings for orly-relay.imwald.eu:**

**Clear both HTTP and HTTPS sections, then add to HTTPS:**

```apache
# Enable proxy
ProxyRequests Off
ProxyPreserveHost On

# WebSocket handling - the key part
RewriteEngine On
RewriteCond %{HTTP:Upgrade} =websocket [NC]
RewriteCond %{HTTP:Connection} upgrade [NC]
RewriteRule /(.*) ws://127.0.0.1:7777/$1 [P,L]

# Fallback for regular HTTP
RewriteCond %{HTTP:Upgrade} !=websocket [NC]
RewriteRule /(.*) http://127.0.0.1:7777/$1 [P,L]

# Headers
ProxyAddHeaders On
```

### **Alternative Simpler Version:**
If the above doesn't work, try just:

```apache
ProxyPass / http://127.0.0.1:7777/
ProxyPassReverse / http://127.0.0.1:7777/
ProxyPass /ws ws://127.0.0.1:7777/
ProxyPassReverse /ws ws://127.0.0.1:7777/
```

## 🧪 **Testing Commands**

```bash
# Test the WebSocket after each change
echo '["REQ","test",{}]' | websocat wss://orly-relay.imwald.eu/

# Check what's actually being served
curl -v https://orly-relay.imwald.eu/ 2>&1 | grep -E "(HTTP|upgrade|connection)"
```

## 🎯 **Expected Fix**

The issue is likely that Apache isn't properly handling the WebSocket upgrade request. The `proxy_wstunnel` module and correct rewrite rules should fix this.

Try the **simpler ProxyPass version first** - it's often more reliable in Plesk environments.
