package tradingview

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// cacheKey is hashable form of (symbol, resolution, bars). Symbol and
// resolution are normalized before keying so equivalent inputs hit
// the same entry.
type cacheKey string

func makeCacheKey(symbol, resolution string, bars int) cacheKey {
	return cacheKey(fmt.Sprintf("%s|%s|%d", symbol, resolution, bars))
}

type cacheEntry struct {
	key       cacheKey
	data      *ChartData
	expiresAt time.Time
}

// lruCache is a small TTL+LRU cache. Entries past their TTL are
// treated as misses and evicted on read.
type lruCache struct {
	mu       sync.Mutex
	max      int
	ll       *list.List
	idx      map[cacheKey]*list.Element
}

func newCache(size int) *lruCache {
	return &lruCache{max: size, ll: list.New(), idx: make(map[cacheKey]*list.Element, size)}
}

func (c *lruCache) get(k cacheKey) (*ChartData, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.idx[k]
	if !ok {
		return nil, false
	}
	e := el.Value.(*cacheEntry)
	if time.Now().After(e.expiresAt) {
		c.ll.Remove(el)
		delete(c.idx, k)
		return nil, false
	}
	c.ll.MoveToFront(el)
	clone := *e.data
	clone.Cached = true
	return &clone, true
}

func (c *lruCache) put(k cacheKey, data *ChartData, ttl time.Duration) {
	if ttl <= 0 || data == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.idx[k]; ok {
		e := el.Value.(*cacheEntry)
		e.data = data
		e.expiresAt = time.Now().Add(ttl)
		c.ll.MoveToFront(el)
		return
	}
	e := &cacheEntry{key: k, data: data, expiresAt: time.Now().Add(ttl)}
	c.idx[k] = c.ll.PushFront(e)
	for c.ll.Len() > c.max {
		old := c.ll.Back()
		if old == nil {
			break
		}
		c.ll.Remove(old)
		delete(c.idx, old.Value.(*cacheEntry).key)
	}
}
