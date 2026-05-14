package miro

import (
	"sync"
	"testing"
	"time"
)

func TestNilCacheIsNoOp(t *testing.T) {
	var c *Cache
	c.Put("k", []byte("v")) // must not panic
	if got, ok := c.Get("k"); ok || got != nil {
		t.Errorf("nil Cache Get = (%v, %v), want (nil, false)", got, ok)
	}
	if c.Len() != 0 {
		t.Errorf("nil Cache Len = %d, want 0", c.Len())
	}
}

func TestZeroMaxOrTTLDisables(t *testing.T) {
	cases := []struct {
		name string
		max  int
		ttl  time.Duration
	}{
		{"zero max", 0, time.Second},
		{"negative max", -1, time.Second},
		{"zero ttl", 16, 0},
		{"negative ttl", 16, -time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if c := NewCache(tc.max, tc.ttl); c != nil {
				t.Errorf("NewCache(%d, %v) = non-nil; want nil to share the no-op path",
					tc.max, tc.ttl)
			}
		})
	}
}

func TestCacheRoundTrip(t *testing.T) {
	c := NewCache(4, time.Minute)
	c.Put("a", []byte("one"))
	c.Put("b", []byte("two"))

	if got, ok := c.Get("a"); !ok || string(got) != "one" {
		t.Errorf("Get(a) = (%q, %v), want (one, true)", got, ok)
	}
	if got, ok := c.Get("b"); !ok || string(got) != "two" {
		t.Errorf("Get(b) = (%q, %v), want (two, true)", got, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Errorf("Get(missing) hit; want miss")
	}
}

func TestCacheRefreshesOnRePut(t *testing.T) {
	c := NewCache(4, time.Minute)
	c.Put("k", []byte("v1"))
	c.Put("k", []byte("v2"))
	if c.Len() != 1 {
		t.Errorf("Len after re-Put = %d, want 1", c.Len())
	}
	if got, ok := c.Get("k"); !ok || string(got) != "v2" {
		t.Errorf("Get(k) after re-Put = (%q, %v), want (v2, true)", got, ok)
	}
}

func TestCacheEvictsLeastRecentlyUsed(t *testing.T) {
	c := NewCache(2, time.Minute)
	c.Put("a", []byte("1"))
	c.Put("b", []byte("2"))
	// Touch a so b becomes the LRU candidate.
	if _, ok := c.Get("a"); !ok {
		t.Fatal("Get(a) missed before eviction trigger")
	}
	c.Put("c", []byte("3"))

	if _, ok := c.Get("b"); ok {
		t.Errorf("b survived eviction; LRU policy not enforced")
	}
	if _, ok := c.Get("a"); !ok {
		t.Errorf("a evicted; LRU touch did not protect it")
	}
	if _, ok := c.Get("c"); !ok {
		t.Errorf("c missing after Put")
	}
	if c.Len() != 2 {
		t.Errorf("Len = %d, want 2", c.Len())
	}
}

func TestCacheExpiresEntries(t *testing.T) {
	c := NewCache(4, 100*time.Millisecond)
	// Inject a clock so the test doesn't depend on wall-clock sleep.
	now := time.Now()
	c.now = func() time.Time { return now }

	c.Put("k", []byte("v"))
	if _, ok := c.Get("k"); !ok {
		t.Fatal("Get(k) missed inside TTL")
	}

	// Advance past TTL.
	now = now.Add(101 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Errorf("Get(k) hit after TTL expiry")
	}
	if c.Len() != 0 {
		t.Errorf("expired entry not evicted; Len = %d, want 0", c.Len())
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	c := NewCache(64, time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('a' + id%8))
			for j := 0; j < 100; j++ {
				c.Put(key, []byte("v"))
				_, _ = c.Get(key)
			}
		}(i)
	}
	wg.Wait()
	// No assertion on Len: the point is the -race detector seeing no
	// data race across Put/Get on a shared Cache.
}
