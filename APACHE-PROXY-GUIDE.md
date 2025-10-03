# Apache Reverse Proxy Guide for Docker Apps

**Complete guide for WebSocket-enabled applications - covers both Plesk and Standard Apache**
**Updated with real-world troubleshooting solutions and latest Orly relay improvements**

## 🎯 **What This Solves**
- WebSocket connection failures (`NS_ERROR_WEBSOCKET_CONNECTION_REFUSED`)
- Nostr relay connectivity issues (`HTTP 426` instead of WebSocket upgrade)
- Docker container proxy configuration
- SSL certificate integration
- Plesk configuration conflicts and virtual host precedence issues
- **NEW**: WebSocket scheme validation errors (`expected 'ws' got 'wss'`)
- **NEW**: Proxy-friendly relay configuration with enhanced CORS headers
- **NEW**: Improved error handling for malformed client data

## 🐳 **Step 1: Deploy Your Docker Application**

### **For Stella's Orly Relay (Latest Version with Proxy Improvements):**
```bash
# Pull and run the relay with enhanced proxy support
docker run -d \
  --name orly-relay \
  --restart unless-stopped \
  -p 127.0.0.1:7777:7777 \
  -v /data/orly-relay:/data \
  -e ORLY_OWNERS=npub1v30tsz9vw6ylpz63g0a702nj3xa26t3m7p5us8f2y2sd8v6cnsvq465zjx \
  -e ORLY_ADMINS=npub1v30tsz9vw6ylpz63g0a702nj3xa26t3m7p5us8f2y2sd8v6cnsvq465zjx,npub1l5sga6xg72phsz5422ykujprejwud075ggrr3z2hwyrfgr7eylqstegx9z \
  -e ORLY_RELAY_URL=wss://your-domain.com \
  -e ORLY_ACL_MODE=follows \
  -e ORLY_SPIDER_MODE=follows \
  -e ORLY_SPIDER_FREQUENCY=1h \
  -e ORLY_SUBSCRIPTION_ENABLED=false \
  silberengel/next-orly:latest

# Test the relay
curl -I http://127.0.0.1:7777
# Should return: HTTP/1.1 200 OK with enhanced CORS headers
```

### **For Web Apps (like Jumble):**
```bash
# Run with fixed port for easier proxy setup
docker run -d \
  --name jumble-app \
  --restart unless-stopped \
  -p 127.0.0.1:3000:80 \
  -e NODE_ENV=production \
  silberengel/imwald-jumble:latest

# Test the app
curl -I http://127.0.0.1:3000
```

## 🔧 **Step 2A: PLESK Configuration**

### **For Your Friend's Standard Apache Setup:**

**Tell your friend to create `/etc/apache2/sites-available/domain.conf`:**

```apache
<VirtualHost *:443>
    ServerName your-domain.com
    
    # SSL Configuration (Let's Encrypt)
    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/your-domain.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/your-domain.com/privkey.pem
    
    # Enable required modules first:
    # sudo a2enmod proxy proxy_http proxy_wstunnel rewrite headers ssl
    
    # Proxy settings
    ProxyPreserveHost On
    ProxyRequests Off
    
    # WebSocket upgrade handling - CRITICAL for apps with WebSockets
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} websocket [NC]
    RewriteCond %{HTTP:Connection} upgrade [NC]
    RewriteRule ^/?(.*) "ws://127.0.0.1:PORT/$1" [P,L]
    
    # Regular HTTP proxy
    ProxyPass / http://127.0.0.1:PORT/
    ProxyPassReverse / http://127.0.0.1:PORT/
    
    # Headers for modern web apps
    Header always set X-Forwarded-Proto "https"
    Header always set X-Forwarded-Port "443"
    Header always set X-Forwarded-For %{REMOTE_ADDR}s
    
    # Security headers
    Header always set Strict-Transport-Security "max-age=63072000; includeSubDomains"
    Header always set X-Content-Type-Options nosniff
    Header always set X-Frame-Options SAMEORIGIN
</VirtualHost>

# Redirect HTTP to HTTPS
<VirtualHost *:80>
    ServerName your-domain.com
    Redirect permanent / https://your-domain.com/
</VirtualHost>
```

**Then enable it:**
```bash
sudo a2ensite domain.conf
sudo systemctl reload apache2
```

### **For Plesk Users (You):**

⚠️ **Important**: Plesk often doesn't apply Apache directives correctly through the interface. If the interface method fails, use the "Direct Apache Override" method below.

#### **Method 1: Plesk Interface (Try First)**

1. **Go to Plesk** → Websites & Domains → **your-domain.com**
2. **Click "Apache & nginx Settings"**
3. **DISABLE nginx** (uncheck "Proxy mode" and "Smart static files processing")
4. **Clear HTTP section** (leave empty)
5. **In HTTPS section, add:**

**For Nostr Relay (port 7777):**
```apache
ProxyRequests Off
ProxyPreserveHost On
ProxyPass / ws://127.0.0.1:7777/
ProxyPassReverse / ws://127.0.0.1:7777/
Header always set Access-Control-Allow-Origin "*"
```

6. **Click "Apply"** and wait 60 seconds

#### **Method 2: Direct Apache Override (If Plesk Interface Fails)**

If Plesk doesn't apply your configuration (common issue), bypass it entirely:

```bash
# Create direct Apache override
sudo tee /etc/apache2/conf-available/relay-override.conf << 'EOF'
<VirtualHost YOUR_SERVER_IP:443>
    ServerName your-domain.com
    ServerAlias www.your-domain.com
    ServerAlias ipv4.your-domain.com
    
    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/your-domain.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/your-domain.com/privkey.pem
    
    DocumentRoot /var/www/relay
    
    # For Nostr relay - proxy everything to WebSocket
    ProxyRequests Off
    ProxyPreserveHost On
    ProxyPass / ws://127.0.0.1:7777/
    ProxyPassReverse / ws://127.0.0.1:7777/
    
    # CORS headers
    Header always set Access-Control-Allow-Origin "*"
    Header always set Access-Control-Allow-Headers "Origin, X-Requested-With, Content-Type, Accept, Authorization"
    
    # Logging
    ErrorLog /var/log/apache2/relay-error.log
    CustomLog /var/log/apache2/relay-access.log combined
</VirtualHost>
EOF

# Enable the override
sudo a2enconf relay-override
sudo mkdir -p /var/www/relay
sudo systemctl restart apache2

# Remove Plesk config if it conflicts
sudo rm /etc/apache2/plesk.conf.d/vhosts/your-domain.com.conf
```

#### **Method 3: Debugging Plesk Issues**

If configurations aren't being applied:

```bash
# Check if Plesk applied your config
grep -E "(ProxyPass|proxy)" /etc/apache2/plesk.conf.d/vhosts/your-domain.com.conf

# Check virtual host precedence
apache2ctl -S | grep your-domain.com

# Check Apache modules
apache2ctl -M | grep -E "(proxy|rewrite)"
```

#### **For Web Apps (port 3000 or 32768):**
```apache
ProxyPreserveHost On
ProxyRequests Off

# WebSocket upgrade handling
RewriteEngine On
RewriteCond %{HTTP:Upgrade} websocket [NC]
RewriteCond %{HTTP:Connection} upgrade [NC]
RewriteRule ^/?(.*) "ws://127.0.0.1:32768/$1" [P,L]

# Regular HTTP proxy
ProxyPass / http://127.0.0.1:32768/
ProxyPassReverse / http://127.0.0.1:32768/

# Headers
ProxyAddHeaders On
Header always set X-Forwarded-Proto "https"
Header always set X-Forwarded-Port "443"
```

### **Method B: Direct Apache Override (RECOMMENDED for Plesk)**

⚠️ **Use this if Plesk interface doesn't work** (common issue):

```bash
# Create direct Apache override with your server's IP
sudo tee /etc/apache2/conf-available/relay-override.conf << 'EOF'
<VirtualHost YOUR_SERVER_IP:443>
    ServerName your-domain.com
    ServerAlias www.your-domain.com
    ServerAlias ipv4.your-domain.com
    
    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/your-domain.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/your-domain.com/privkey.pem
    
    DocumentRoot /var/www/relay
    
    # For Nostr relay - proxy everything to WebSocket
    ProxyRequests Off
    ProxyPreserveHost On
    ProxyPass / ws://127.0.0.1:7777/
    ProxyPassReverse / ws://127.0.0.1:7777/
    
    # CORS headers
    Header always set Access-Control-Allow-Origin "*"
    
    # Logging
    ErrorLog /var/log/apache2/relay-error.log
    CustomLog /var/log/apache2/relay-access.log combined
</VirtualHost>
EOF

# Enable override and create directory
sudo a2enconf relay-override
sudo mkdir -p /var/www/relay
sudo systemctl restart apache2

# Remove conflicting Plesk config if needed
sudo rm /etc/apache2/plesk.conf.d/vhosts/your-domain.com.conf
```

## ⚡ **Step 3: Enable Required Modules**

In Plesk, you might need to enable modules. SSH to your server:

```bash
# Enable Apache modules
sudo a2enmod proxy
sudo a2enmod proxy_http
sudo a2enmod proxy_wstunnel
sudo a2enmod rewrite
sudo a2enmod headers
sudo systemctl restart apache2
```

## 🆕 **Step 4: Latest Orly Relay Improvements**

### **Enhanced Proxy Support**
The latest Orly relay includes several proxy improvements:

1. **Flexible WebSocket Scheme Handling**: Accepts both `ws://` and `wss://` schemes for authentication
2. **Enhanced CORS Headers**: Better compatibility with web applications
3. **Improved Error Handling**: More robust handling of malformed client data
4. **Proxy-Aware Logging**: Better debugging information for proxy setups

### **Key Environment Variables**
```bash
# Essential for proxy setups
ORLY_RELAY_URL=wss://your-domain.com  # Must match your public URL
ORLY_ACL_MODE=follows                  # Enable follows-based access control
ORLY_SPIDER_MODE=follows              # Enable content syncing from other relays
ORLY_SUBSCRIPTION_ENABLED=false      # Disable payment requirements
```

### **Testing the Enhanced Relay**
```bash
# Test local connectivity
curl -I http://127.0.0.1:7777

# Expected response includes enhanced CORS headers:
# Access-Control-Allow-Credentials: true
# Access-Control-Max-Age: 86400
# Vary: Origin, Access-Control-Request-Method, Access-Control-Request-Headers
```

## ⚡ **Step 4: Alternative - Nginx in Plesk**

If Apache keeps giving issues, switch to Nginx in Plesk:

1. Go to Plesk → Websites & Domains → orly-relay.imwald.eu
2. Click "Apache & nginx Settings"
3. Enable "nginx" and set it to serve static files
4. In "Additional nginx directives" add:

```nginx
location / {
    proxy_pass http://127.0.0.1:7777;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

## 🧪 **Testing**

After making changes:

1. **Apply settings** in Plesk
2. **Wait 30 seconds** for changes to take effect
3. **Test WebSocket**:
   ```bash
   # From your server
   echo '["REQ","test",{}]' | websocat wss://orly-relay.imwald.eu/
   ```

## 🎯 **Expected Result**

- ✅ No more "websocket error" in browser console
- ✅ `wss://orly-relay.imwald.eu/` connects successfully
- ✅ Jumble app can publish notes

## 🚨 **Real-World Troubleshooting Guide**

*Based on actual deployment experience with Plesk and WebSocket issues*

### **Critical Issues & Solutions:**

#### **🔴 HTTP 503 Service Unavailable**
- **Cause**: Docker container not running
- **Check**: `docker ps | grep relay`
- **Fix**: `docker start container-name`

#### **🔴 HTTP 426 Instead of WebSocket Upgrade**
- **Cause**: Apache using `http://` proxy instead of `ws://`
- **Fix**: Use `ProxyPass / ws://127.0.0.1:7777/` (not `http://`)

#### **🔴 Plesk Configuration Not Applied**
- **Symptom**: Config not in `/etc/apache2/plesk.conf.d/vhosts/domain.conf`
- **Solution**: Use Direct Apache Override method (bypass Plesk interface)

#### **🔴 Virtual Host Conflicts**
- **Check**: `apache2ctl -S | grep domain.com`
- **Fix**: Remove Plesk config: `sudo rm /etc/apache2/plesk.conf.d/vhosts/domain.conf`

#### **🔴 Nginx Intercepting (Plesk)**
- **Symptom**: Response shows `Server: nginx`
- **Fix**: Disable nginx in Plesk settings

### **Debug Commands:**
```bash
# Essential debugging
docker ps | grep relay                   # Container running?
curl -I http://127.0.0.1:7777           # Local relay (should return 200 with CORS headers)
apache2ctl -S | grep domain.com         # Virtual host precedence
grep ProxyPass /etc/apache2/plesk.conf.d/vhosts/domain.conf  # Config applied?

# WebSocket testing
echo '["REQ","test",{}]' | websocat wss://domain.com/     # Root path
echo '["REQ","test",{}]' | websocat wss://domain.com/ws/  # /ws/ path

# Check relay logs for proxy information
docker logs relay-name | grep -i "proxy info"
docker logs relay-name | grep -i "websocket connection"
```

## 🚨 **Latest Troubleshooting Solutions**

### **WebSocket Scheme Validation Errors**
**Problem**: `"HTTP Scheme incorrect: expected 'ws' got 'wss'"`

**Solution**: Use the latest Orly relay image with enhanced proxy support:
```bash
# Pull the latest image with proxy improvements
docker pull silberengel/next-orly:latest

# Restart with the latest image
docker stop orly-relay && docker rm orly-relay
# Then run with the configuration above
```

### **Malformed Client Data Errors**
**Problem**: `"invalid hex array size, got 2 expect 64"`

**Solution**: These are client-side issues, not server problems. The latest relay handles them gracefully:
- The relay now sends helpful error messages to clients
- Malformed requests are logged but don't crash the relay
- Normal operations continue despite client errors

### **Follows ACL Not Working**
**Problem**: Only owners can write, admins can't write

**Solution**: Ensure proper configuration:
```bash
# Check ACL configuration
docker exec orly-relay env | grep ACL

# Should show: ORLY_ACL_MODE=follows
# If not, restart with explicit configuration
```

### **Spider Not Syncing Content**
**Problem**: Spider enabled but not pulling events

**Solution**: Check for relay lists and follow events:
```bash
# Check spider status
docker logs orly-relay | grep -i spider

# Look for relay discovery
docker logs orly-relay | grep -i "relay URLs"

# Check for follow events
docker logs orly-relay | grep -i "kind.*3"
```

### **Working Solution (Proven):**
```apache
<VirtualHost SERVER_IP:443>
    ServerName domain.com
    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/domain.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/domain.com/privkey.pem
    DocumentRoot /var/www/relay
    
    # Direct WebSocket proxy - this is the key!
    ProxyRequests Off
    ProxyPreserveHost On
    ProxyPass / ws://127.0.0.1:7777/
    ProxyPassReverse / ws://127.0.0.1:7777/
    
    Header always set Access-Control-Allow-Origin "*"
</VirtualHost>
```

---

**Key Lessons**: 
1. Plesk interface often fails to apply Apache directives
2. Use `ws://` proxy for Nostr relays, not `http://`
3. Direct Apache config files are more reliable than Plesk interface
4. Always check virtual host precedence with `apache2ctl -S`
5. **NEW**: Use the latest Orly relay image for better proxy compatibility
6. **NEW**: Enhanced CORS headers improve web app compatibility
7. **NEW**: Flexible WebSocket scheme handling eliminates authentication errors
8. **NEW**: Improved error handling makes the relay more robust

## 🎉 **Summary of Latest Improvements**

### **Enhanced Proxy Support**
- ✅ Flexible WebSocket scheme validation (accepts both `ws://` and `wss://`)
- ✅ Enhanced CORS headers for better web app compatibility
- ✅ Improved error handling for malformed client data
- ✅ Proxy-aware logging for better debugging

### **Spider and ACL Features**
- ✅ Follows-based access control (`ORLY_ACL_MODE=follows`)
- ✅ Content syncing from other relays (`ORLY_SPIDER_MODE=follows`)
- ✅ No payment requirements (`ORLY_SUBSCRIPTION_ENABLED=false`)

### **Production Ready**
- ✅ Robust error handling
- ✅ Enhanced logging and debugging
- ✅ Better client compatibility
- ✅ Improved proxy support

**The latest Orly relay is now fully optimized for proxy environments and provides a much better user experience!**
