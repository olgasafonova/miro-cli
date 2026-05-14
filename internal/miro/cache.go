package miro

import (
	"container/list"
	"sync"
	"time"
)

// DefaultCacheTTL is the freshness window for cached GET responses. Sixty
// seconds is short enough that interactive workflows (rename a board, refetch)
// don't see weeks-old data, long enough that a script issuing a dozen list_*
// calls in a tight loop pays Miro's quota only once.
const DefaultCacheTTL = 60 * time.Second

// DefaultCacheEntries is the maximum number of cached responses retained in
// the LRU. Sized to cover a moderate interactive session (a few dozen boards,
// each with a handful of GETs) without unbounded growth in long-running
// processes that embed the client.
const DefaultCacheEntries = 256

// Cache is an in-process LRU+TTL response cache keyed by method+path. It is
// safe for concurrent use. Construct with NewCache; the zero value and a nil
// *Cache are both no-ops (Get always misses, Put silently drops) so callers
// can wire the cache unconditionally and let configuration decide whether it
// engages.
//
// Cache stores raw response bodies. Callers (Client.Do) decode on hit, so
// the same cached bytes serve callers asking for different out types as long
// as the bytes are valid JSON for each.
type Cache struct {
	max int
	ttl time.Duration

	// now lets tests advance time without sleeping. Production uses time.Now;
	// tests override.
	now func() time.Time

	mu      sync.Mutex
	entries map[string]*list.Element
	order   *list.List // front = most recently used, back = LRU candidate
}

type cacheEntry struct {
	key     string
	body    []byte
	expires time.Time
}

// NewCache returns a Cache holding up to maxEntries items, each evicted
// after ttl elapses since its insertion. maxEntries <= 0 or ttl <= 0
// disables the cache: Get/Put become no-ops just like a nil *Cache. This
// lets --no-cache and --cache-ttl=0 share one code path.
func NewCache(maxEntries int, ttl time.Duration) *Cache {
	if maxEntries <= 0 || ttl <= 0 {
		return nil
	}
	return &Cache{
		max:     maxEntries,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]*list.Element, maxEntries),
		order:   list.New(),
	}
}

// Get returns the cached body for key. The bool reports a hit; a hit on an
// expired entry returns (nil, false) and removes the entry. A nil *Cache
// always misses, allowing callers to skip nil-checks at the call site.
func (c *Cache) Get(key string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	entry := el.Value.(*cacheEntry)
	if c.now().After(entry.expires) {
		c.order.Remove(el)
		delete(c.entries, key)
		return nil, false
	}
	c.order.MoveToFront(el)
	return entry.body, true
}

// Put inserts (or refreshes) the cached body for key, evicting the
// least-recently-used entry if the cache is full. A nil *Cache is a no-op.
// body is stored by reference; callers must not mutate it after Put.
func (c *Cache) Put(key string, body []byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	expires := c.now().Add(c.ttl)
	if el, ok := c.entries[key]; ok {
		entry := el.Value.(*cacheEntry)
		entry.body = body
		entry.expires = expires
		c.order.MoveToFront(el)
		return
	}

	entry := &cacheEntry{key: key, body: body, expires: expires}
	el := c.order.PushFront(entry)
	c.entries[key] = el

	for c.order.Len() > c.max {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.order.Remove(back)
		delete(c.entries, back.Value.(*cacheEntry).key)
	}
}

// Len reports the number of live entries. Useful for tests and metrics; a
// nil *Cache returns 0.
func (c *Cache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
