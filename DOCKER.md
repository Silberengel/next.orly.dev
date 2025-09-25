# Docker Deployment Guide

## Quick Start

### 1. Basic Relay Setup

```bash
# Build and start the relay
docker-compose up -d

# View logs
docker-compose logs -f stella-relay

# Stop the relay
docker-compose down
```

### 2. With Nginx Proxy (for SSL/domain setup)

```bash
# Start relay with nginx proxy
docker-compose --profile proxy up -d

# Configure SSL certificates in nginx/ssl/
# Then update nginx/nginx.conf to enable HTTPS
```

## Configuration

### Environment Variables

Copy `env.example` to `.env` and customize:

```bash
cp env.example .env
# Edit .env with your settings
```

Key settings:
- `ORLY_OWNERS`: Owner npubs (comma-separated, full control)
- `ORLY_ADMINS`: Admin npubs (comma-separated, deletion permissions)
- `ORLY_PORT`: Port to listen on (default: 7777)
- `ORLY_MAX_CONNECTIONS`: Max concurrent connections
- `ORLY_CONCURRENT_WORKERS`: CPU cores for concurrent processing (0 = auto)

### Data Persistence

The relay data is stored in `./data` directory which is mounted as a volume.

### Performance Tuning

Based on the v0.4.8 optimizations:
- Concurrent event publishing using all CPU cores
- Optimized BadgerDB access patterns
- Configurable batch sizes and cache settings

## Development

### Local Build

```bash
# Pull the latest image (recommended)
docker pull silberengel/orly-relay:latest

# Or build locally if needed
docker build -t silberengel/orly-relay:latest .

# Run with custom settings
docker run -p 7777:7777 -v $(pwd)/data:/data silberengel/orly-relay:latest
```

### Testing

```bash
# Test WebSocket connection
websocat ws://localhost:7777

# Run stress tests (if available in cmd/stresstest)
go run ./cmd/stresstest -relay ws://localhost:7777
```

## Production Deployment

### SSL Setup

1. Get SSL certificates (Let's Encrypt recommended)
2. Place certificates in `nginx/ssl/`
3. Update `nginx/nginx.conf` to enable HTTPS
4. Start with proxy profile: `docker-compose --profile proxy up -d`

### Monitoring

- Health checks are configured for both services
- Logs are rotated (max 10MB, 3 files)
- Resource limits are set to prevent runaway processes

### Security

- Runs as non-root user (uid 1000)
- Rate limiting configured in nginx
- Configurable authentication and event size limits

## Troubleshooting

### Common Issues (Real-World Experience)

#### **Container Issues:**
1. **Port already in use**: Change `ORLY_PORT` in docker-compose.yml
2. **Permission denied**: Ensure `./data` directory is writable
3. **Container won't start**: Check logs with `docker logs container-name`

#### **WebSocket Issues:**
4. **HTTP 426 instead of WebSocket upgrade**: 
   - Use `ws://127.0.0.1:7777` in proxy config, not `http://`
   - Ensure `proxy_wstunnel` module is enabled
5. **Connection refused in browser but works with websocat**:
   - Clear browser cache and service workers
   - Try incognito mode
   - Add CORS headers to Apache/nginx config

#### **Plesk-Specific Issues:**
6. **Plesk not applying Apache directives**:
   - Check if config appears in `/etc/apache2/plesk.conf.d/vhosts/domain.conf`
   - Use direct Apache override if Plesk interface fails
7. **Virtual host conflicts**:
   - Check precedence with `apache2ctl -S`
   - Remove conflicting Plesk configs if needed

#### **SSL Certificate Issues:**
8. **Self-signed certificate after Let's Encrypt**:
   - Plesk might not be using the correct certificate
   - Import Let's Encrypt certs into Plesk or use direct Apache config

### Debug Commands

```bash
# Container debugging
docker ps | grep relay
docker logs stella-relay
curl -I http://127.0.0.1:7777  # Should return HTTP 426

# WebSocket testing
echo '["REQ","test",{}]' | websocat wss://domain.com/
echo '["REQ","test",{}]' | websocat wss://domain.com/ws/

# Apache debugging (for reverse proxy issues)
apache2ctl -S | grep domain.com
apache2ctl -M | grep -E "(proxy|rewrite)"
grep ProxyPass /etc/apache2/plesk.conf.d/vhosts/domain.conf
```

### Logs

```bash
# View relay logs
docker-compose logs -f stella-relay

# View nginx logs (if using proxy)
docker-compose logs -f nginx

# Apache logs (for reverse proxy debugging)
sudo tail -f /var/log/apache2/error.log
sudo tail -f /var/log/apache2/domain-error.log
```

### Working Reverse Proxy Config

**For Apache (direct config file):**
```apache
<VirtualHost SERVER_IP:443>
    ServerName domain.com
    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/domain.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/domain.com/privkey.pem
    
    # Direct WebSocket proxy for Nostr relay
    ProxyRequests Off
    ProxyPreserveHost On
    ProxyPass / ws://127.0.0.1:7777/
    ProxyPassReverse / ws://127.0.0.1:7777/
    
    Header always set Access-Control-Allow-Origin "*"
</VirtualHost>
```

---

*Crafted for Stella's digital forest* 🌲
