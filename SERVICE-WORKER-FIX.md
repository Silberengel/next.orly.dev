# Service Worker Certificate Caching Fix

## 🚨 **Problem**
When accessing Jumble from the ImWald landing page, the service worker serves a cached self-signed certificate instead of the new Let's Encrypt certificate.

## ⚡ **Solutions**

### **Option 1: Force Service Worker Update**
Add this to your Jumble app's service worker or main JavaScript:

```javascript
// Force service worker update and certificate refresh
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.getRegistrations().then(function(registrations) {
    for(let registration of registrations) {
      registration.update(); // Force update
    }
  });
}

// Clear all caches on certificate update
if ('caches' in window) {
  caches.keys().then(function(names) {
    for (let name of names) {
      caches.delete(name);
    }
  });
}
```

### **Option 2: Update Service Worker Cache Strategy**
In your service worker file, add cache busting for SSL-sensitive requests:

```javascript
// In your service worker
self.addEventListener('fetch', function(event) {
  // Don't cache HTTPS requests that might have certificate issues
  if (event.request.url.startsWith('https://') && 
      event.request.url.includes('imwald.eu')) {
    event.respondWith(
      fetch(event.request, { cache: 'no-store' })
    );
    return;
  }
  
  // Your existing fetch handling...
});
```

### **Option 3: Version Your Service Worker**
Update your service worker with a new version number:

```javascript
// At the top of your service worker
const CACHE_VERSION = 'v2.0.1'; // Increment this when certificates change
const CACHE_NAME = `jumble-cache-${CACHE_VERSION}`;

// Clear old caches
self.addEventListener('activate', function(event) {
  event.waitUntil(
    caches.keys().then(function(cacheNames) {
      return Promise.all(
        cacheNames.map(function(cacheName) {
          if (cacheName !== CACHE_NAME) {
            return caches.delete(cacheName);
          }
        })
      );
    })
  );
});
```

### **Option 4: Add Cache Headers**
In your Plesk Apache config for Jumble, add:

```apache
# Prevent service worker from caching SSL-sensitive content
Header always set Cache-Control "no-cache, no-store, must-revalidate"
Header always set Pragma "no-cache"
Header always set Expires "0"

# Only for service worker file
<Files "sw.js">
    Header always set Cache-Control "no-cache, no-store, must-revalidate"
</Files>
```

## 🧹 **Immediate User Fix**

For users experiencing the certificate issue:

1. **Clear browser data** for jumble.imwald.eu
2. **Unregister service worker**:
   - F12 → Application → Service Workers → Unregister
3. **Hard refresh**: Ctrl+Shift+R
4. **Or use incognito mode** to test

---

This will prevent the service worker from serving stale certificate data.
