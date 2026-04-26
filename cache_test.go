package tradingview

import (
	"testing"
	"time"
)

func TestCacheHitMiss(t *testing.T) {
	c := newCache(3)
	k := makeCacheKey("BINANCE:BTCUSDT", "15", 100)
	if _, ok := c.get(k); ok {
		t.Fatal("empty cache should miss")
	}
	c.put(k, &ChartData{Symbol: "BINANCE:BTCUSDT"}, 50*time.Millisecond)
	hit, ok := c.get(k)
	if !ok || !hit.Cached {
		t.Fatalf("want cached hit, got ok=%v hit=%+v", ok, hit)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	c := newCache(3)
	k := makeCacheKey("X", "1", 10)
	c.put(k, &ChartData{Symbol: "X"}, 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if _, ok := c.get(k); ok {
		t.Fatal("expected expiry")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c := newCache(2)
	c.put(makeCacheKey("A", "1", 1), &ChartData{}, time.Hour)
	c.put(makeCacheKey("B", "1", 1), &ChartData{}, time.Hour)
	c.put(makeCacheKey("C", "1", 1), &ChartData{}, time.Hour)
	if _, ok := c.get(makeCacheKey("A", "1", 1)); ok {
		t.Error("A should have been evicted")
	}
	if _, ok := c.get(makeCacheKey("B", "1", 1)); !ok {
		t.Error("B should still be present")
	}
	if _, ok := c.get(makeCacheKey("C", "1", 1)); !ok {
		t.Error("C should still be present")
	}
}

func TestCacheZeroTTLNoOp(t *testing.T) {
	c := newCache(2)
	k := makeCacheKey("A", "1", 1)
	c.put(k, &ChartData{}, 0)
	if _, ok := c.get(k); ok {
		t.Error("zero ttl should not be stored")
	}
}
