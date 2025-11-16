package querycache

import (
	"container/list"
	"sync"
	"time"

	"lol.mleku.dev/log"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
)

const (
	// DefaultMaxSize is the default maximum cache size in bytes (512 MB)
	DefaultMaxSize = 512 * 1024 * 1024
	// DefaultMaxAge is the default maximum age for cache entries
	DefaultMaxAge = 5 * time.Minute
)

// EventCacheEntry represents a cached set of events for a filter
type EventCacheEntry struct {
	FilterKey   string
	Events      event.S   // Slice of events
	TotalSize   int       // Estimated size in bytes
	LastAccess  time.Time
	CreatedAt   time.Time
	listElement *list.Element
}

// EventCache caches event.S results from database queries
type EventCache struct {
	mu sync.RWMutex

	entries map[string]*EventCacheEntry
	lruList *list.List

	currentSize int64
	maxSize     int64
	maxAge      time.Duration

	hits          uint64
	misses        uint64
	evictions     uint64
	invalidations uint64
}

// NewEventCache creates a new event cache
func NewEventCache(maxSize int64, maxAge time.Duration) *EventCache {
	if maxSize <= 0 {
		maxSize = DefaultMaxSize
	}
	if maxAge <= 0 {
		maxAge = DefaultMaxAge
	}

	c := &EventCache{
		entries: make(map[string]*EventCacheEntry),
		lruList: list.New(),
		maxSize: maxSize,
		maxAge:  maxAge,
	}

	go c.cleanupExpired()

	return c
}

// Get retrieves cached events for a filter
func (c *EventCache) Get(f *filter.F) (events event.S, found bool) {
	filterKey := string(f.Serialize())

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[filterKey]
	if !exists {
		c.misses++
		return nil, false
	}

	// Check if expired
	if time.Since(entry.CreatedAt) > c.maxAge {
		c.removeEntry(entry)
		c.misses++
		return nil, false
	}

	// Update access time and move to front
	entry.LastAccess = time.Now()
	c.lruList.MoveToFront(entry.listElement)

	c.hits++
	log.D.F("event cache HIT: filter=%s events=%d", filterKey[:min(50, len(filterKey))], len(entry.Events))

	return entry.Events, true
}

// Put stores events in the cache
func (c *EventCache) Put(f *filter.F, events event.S) {
	if len(events) == 0 {
		return
	}

	filterKey := string(f.Serialize())

	// Estimate size: each event is roughly 500 bytes on average
	estimatedSize := len(events) * 500

	// Don't cache if too large
	if int64(estimatedSize) > c.maxSize {
		log.W.F("event cache: entry too large: %d bytes", estimatedSize)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if existing, exists := c.entries[filterKey]; exists {
		c.currentSize -= int64(existing.TotalSize)
		existing.Events = events
		existing.TotalSize = estimatedSize
		existing.LastAccess = time.Now()
		existing.CreatedAt = time.Now()
		c.currentSize += int64(estimatedSize)
		c.lruList.MoveToFront(existing.listElement)
		return
	}

	// Evict if necessary
	for c.currentSize+int64(estimatedSize) > c.maxSize && c.lruList.Len() > 0 {
		oldest := c.lruList.Back()
		if oldest != nil {
			oldEntry := oldest.Value.(*EventCacheEntry)
			c.removeEntry(oldEntry)
			c.evictions++
		}
	}

	// Create new entry
	entry := &EventCacheEntry{
		FilterKey:  filterKey,
		Events:     events,
		TotalSize:  estimatedSize,
		LastAccess: time.Now(),
		CreatedAt:  time.Now(),
	}

	entry.listElement = c.lruList.PushFront(entry)
	c.entries[filterKey] = entry
	c.currentSize += int64(estimatedSize)

	log.D.F("event cache PUT: filter=%s events=%d size=%d total=%d/%d",
		filterKey[:min(50, len(filterKey))], len(events), estimatedSize, c.currentSize, c.maxSize)
}

// Invalidate clears all entries (called when new events are stored)
func (c *EventCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) > 0 {
		cleared := len(c.entries)
		c.entries = make(map[string]*EventCacheEntry)
		c.lruList = list.New()
		c.currentSize = 0
		c.invalidations += uint64(cleared)
		log.T.F("event cache INVALIDATE: cleared %d entries", cleared)
	}
}

// removeEntry removes an entry (must be called with lock held)
func (c *EventCache) removeEntry(entry *EventCacheEntry) {
	delete(c.entries, entry.FilterKey)
	c.lruList.Remove(entry.listElement)
	c.currentSize -= int64(entry.TotalSize)
}

// cleanupExpired removes expired entries periodically
func (c *EventCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		var toRemove []*EventCacheEntry

		for _, entry := range c.entries {
			if now.Sub(entry.CreatedAt) > c.maxAge {
				toRemove = append(toRemove, entry)
			}
		}

		for _, entry := range toRemove {
			c.removeEntry(entry)
		}

		if len(toRemove) > 0 {
			log.D.F("event cache cleanup: removed %d expired entries", len(toRemove))
		}

		c.mu.Unlock()
	}
}

// CacheStats holds cache performance metrics
type CacheStats struct {
	Entries       int
	CurrentSize   int64
	MaxSize       int64
	Hits          uint64
	Misses        uint64
	HitRate       float64
	Evictions     uint64
	Invalidations uint64
}

// Stats returns cache statistics
func (c *EventCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return CacheStats{
		Entries:       len(c.entries),
		CurrentSize:   c.currentSize,
		MaxSize:       c.maxSize,
		Hits:          c.hits,
		Misses:        c.misses,
		HitRate:       hitRate,
		Evictions:     c.evictions,
		Invalidations: c.invalidations,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
