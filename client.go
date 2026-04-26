package tradingview

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	defaultBars = 300
	maxBars     = 5000
)

// Client is the public entry point. Construct with New, share across
// goroutines, defer Close on shutdown.
type Client struct {
	cfg   Config
	log   Logger
	pool  *connPool
	cache *lruCache
	http  *http.Client

	mu     sync.Mutex
	closed bool
}

// New returns a configured Client. Pass an empty Config for defaults;
// pass nil for log to disable logging.
func New(cfg Config, log Logger) *Client {
	cfg = cfg.withDefaults()
	if log == nil {
		log = NoopLogger{}
	}
	c := &Client{
		cfg:  cfg,
		log:  log,
		pool: newConnPool(cfg, log),
		http: &http.Client{Timeout: cfg.SearchTimeout + 5*time.Second},
	}
	if cfg.EnableCache {
		c.cache = newCache(cfg.CacheSize)
	}
	return c
}

// Close releases all pooled connections. Subsequent calls return
// ErrClosed.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	return c.pool.close()
}

// Fetch returns up to bars OHLCV candles for symbol at resolution.
// Symbol is passed through to TradingView verbatim after upper-casing
// and trimming; use SearchSymbols to discover canonical
// "EXCHANGE:SYMBOL" forms.
//
// If the result is in cache and unexpired, no network call is made.
func (c *Client) Fetch(ctx context.Context, symbol, resolution string, bars int) (*ChartData, error) {
	if c.isClosed() {
		return nil, newErr(CodeClosed, "client closed", nil)
	}
	sym := NormalizeSymbol(symbol)
	if sym == "" {
		return nil, newErr(CodeSymbolUnknown, "symbol is empty", nil)
	}
	res, ok := NormalizeResolution(resolution)
	if !ok {
		return nil, newErr(CodeInvalidResolution, "unknown resolution: "+resolution, nil)
	}
	if bars <= 0 {
		bars = defaultBars
	}
	if bars > maxBars {
		return nil, newErr(CodeInvalidBars, "bars exceeds max (5000)", nil)
	}

	key := makeCacheKey(sym, res, bars)
	if c.cache != nil {
		if hit, ok := c.cache.get(key); ok {
			return hit, nil
		}
	}

	start := time.Now()
	candles, err := c.fetchOnce(ctx, sym, res, bars)
	if err != nil {
		return nil, err
	}
	out := &ChartData{
		Symbol:     sym,
		Resolution: res,
		Candles:    candles,
		FetchedAt:  time.Now(),
		LatencyMs:  time.Since(start).Milliseconds(),
	}
	if c.cache != nil {
		if ttl, ok := c.cfg.CacheTTLs[res]; ok {
			c.cache.put(key, out, ttl)
		}
	}
	return out, nil
}

// fetchOnce acquires a conn, runs one fetch, and releases the conn.
func (c *Client) fetchOnce(ctx context.Context, sym, res string, bars int) ([]Candle, error) {
	conn, err := c.pool.acquire(ctx)
	if err != nil {
		return nil, err
	}
	candles, err := conn.fetchSeries(ctx, sym, res, bars)
	if err != nil {
		c.pool.release(conn, false)
		return nil, err
	}
	c.pool.release(conn, true)
	return candles, nil
}

// FetchMulti retrieves several resolutions of the same symbol in
// parallel, capped by the connection pool size. A failure on one
// resolution does not abort the others; check MultiChartData.Errors
// for partial failures.
func (c *Client) FetchMulti(ctx context.Context, symbol string, resolutions []string, barsPer int) (*MultiChartData, error) {
	if c.isClosed() {
		return nil, newErr(CodeClosed, "client closed", nil)
	}
	if len(resolutions) == 0 {
		return nil, newErr(CodeInvalidResolution, "no resolutions provided", nil)
	}
	start := time.Now()
	out := &MultiChartData{
		Symbol: NormalizeSymbol(symbol),
		Slabs:  make(map[string]*ChartData, len(resolutions)),
		Errors: make(map[string]string),
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(c.cfg.PoolSize)

	var mu sync.Mutex
	for _, r := range resolutions {
		r := r
		g.Go(func() error {
			data, err := c.Fetch(gctx, symbol, r, barsPer)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				out.Errors[r] = err.Error()
				return nil // soft-fail; collect others
			}
			out.Slabs[data.Resolution] = data
			return nil
		})
	}
	_ = g.Wait()

	out.FetchedAt = time.Now()
	out.LatencyMs = time.Since(start).Milliseconds()
	if len(out.Errors) == 0 {
		out.Errors = nil
	}
	return out, nil
}

func (c *Client) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}
