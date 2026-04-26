//go:build integration

package tradingview

import (
	"context"
	"testing"
	"time"
)

// These tests hit the real TradingView endpoints. Run with:
//
//	go test -tags integration ./...
//
// They are excluded from the default test build so CI does not depend
// on TradingView availability.

func TestIntegrationFetchBTC(t *testing.T) {
	c := New(Config{}, nil)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := c.Fetch(ctx, "BINANCE:BTCUSDT", "15", 50)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(data.Candles) == 0 {
		t.Fatal("no candles")
	}
	if data.Candles[0].Close <= 0 {
		t.Errorf("bogus close: %v", data.Candles[0])
	}
	t.Logf("got %d candles, latency=%dms", len(data.Candles), data.LatencyMs)
}

func TestIntegrationCacheHit(t *testing.T) {
	// Cache is opt-in; this test verifies it works when enabled.
	c := New(Config{EnableCache: true}, nil)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	a, err := c.Fetch(ctx, "BINANCE:ETHUSDT", "60", 30)
	if err != nil {
		t.Fatalf("fetch a: %v", err)
	}
	if a.Cached {
		t.Error("first fetch reported cached")
	}
	b, err := c.Fetch(ctx, "BINANCE:ETHUSDT", "60", 30)
	if err != nil {
		t.Fatalf("fetch b: %v", err)
	}
	if !b.Cached {
		t.Error("second fetch should be cached")
	}
}

func TestIntegrationCacheOffByDefault(t *testing.T) {
	c := New(Config{}, nil)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	a, err := c.Fetch(ctx, "BINANCE:SOLUSDT", "15", 20)
	if err != nil {
		t.Fatalf("fetch a: %v", err)
	}
	b, err := c.Fetch(ctx, "BINANCE:SOLUSDT", "15", 20)
	if err != nil {
		t.Fatalf("fetch b: %v", err)
	}
	if a.Cached || b.Cached {
		t.Errorf("cache should be off by default; got a.Cached=%v b.Cached=%v", a.Cached, b.Cached)
	}
}

func TestIntegrationFetchMulti(t *testing.T) {
	c := New(Config{}, nil)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := c.FetchMulti(ctx, "BINANCE:BTCUSDT", []string{"1", "5", "15", "60", "240", "D"}, 50)
	if err != nil {
		t.Fatalf("multi: %v", err)
	}
	for _, r := range []string{"1", "5", "15", "60", "240", "D"} {
		slab, ok := out.Slabs[r]
		if !ok {
			t.Errorf("missing resolution %s (errors: %v)", r, out.Errors)
			continue
		}
		if len(slab.Candles) == 0 {
			t.Errorf("resolution %s: no candles", r)
		}
	}
	t.Logf("multi latency: %dms", out.LatencyMs)
}

func TestIntegrationSearchSymbols(t *testing.T) {
	c := New(Config{}, nil)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	matches, err := c.SearchSymbols(ctx, "BTC", SearchOptions{Type: "crypto", Limit: 5})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no matches")
	}
	for _, m := range matches {
		if m.Symbol == "" {
			t.Errorf("empty symbol in match: %+v", m)
		}
	}
	t.Logf("got %d matches; first: %+v", len(matches), matches[0])
}
