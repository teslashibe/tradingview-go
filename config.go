package tradingview

import "time"

// Config controls every tunable knob. Use DefaultConfig and override
// the few fields you care about.
type Config struct {
	// Host is the TradingView WebSocket host. The free anonymous
	// endpoint is data.tradingview.com; prodata.tradingview.com
	// requires a paid auth token. Default: data.tradingview.com.
	Host string

	// AuthToken is sent in the set_auth_token handshake. The free
	// endpoint accepts "unauthorized_user_token". Default: that.
	AuthToken string

	// PoolSize is the maximum number of authenticated WebSocket
	// connections kept in the pool. Connections are dialed lazily.
	// Default: 8.
	PoolSize int

	// HandshakeTimeout caps the WS upgrade handshake. Default: 10s.
	HandshakeTimeout time.Duration

	// FetchTimeout caps a single Fetch (handshake + subscribe + read).
	// Default: 20s.
	FetchTimeout time.Duration

	// IdleConnTimeout closes pool entries that go unused for this
	// long. Default: 5 minutes.
	IdleConnTimeout time.Duration

	// SearchTimeout caps a single SearchSymbols REST call. Default: 10s.
	SearchTimeout time.Duration

	// CacheTTLs maps a TradingView resolution string ("1", "5", "15",
	// "60", "240", "D", "W", "M") to its cache TTL. Resolutions not
	// listed here are not cached. Use DisableCache to opt out
	// entirely. The default ladder favors freshness on short bars
	// and reuse on long bars.
	CacheTTLs map[string]time.Duration

	// CacheSize is the maximum number of (symbol, resolution, bars)
	// entries held in the LRU. Default: 256.
	CacheSize int

	// EnableCache opts in to the TTL+LRU cache. Default off: for
	// short-horizon trading the freshness loss (5s+ on the live bar)
	// outweighs the per-call latency saved. Flip on if you observe
	// duplicate fetches inside a single reasoning turn or hit
	// upstream rate limits.
	EnableCache bool

	// CandleHistory is the default bar count requested by Streamer.
	// Default: 300.
	CandleHistory int

	// UserAgent is sent with WebSocket and REST requests. Default: a
	// generic Mozilla string (TradingView rejects programmatic UAs).
	UserAgent string
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		Host:             "data.tradingview.com",
		AuthToken:        "unauthorized_user_token",
		PoolSize:         8,
		HandshakeTimeout: 10 * time.Second,
		FetchTimeout:     20 * time.Second,
		IdleConnTimeout:  5 * time.Minute,
		SearchTimeout:    10 * time.Second,
		CacheSize:        256,
		CandleHistory:    300,
		CacheTTLs: map[string]time.Duration{
			"1":   5 * time.Second,
			"5":   15 * time.Second,
			"15":  30 * time.Second,
			"30":  45 * time.Second,
			"60":  60 * time.Second,
			"120": 2 * time.Minute,
			"240": 5 * time.Minute,
			"D":   30 * time.Minute,
			"W":   2 * time.Hour,
			"M":   6 * time.Hour,
		},
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
	}
}

// withDefaults fills any zero-valued field on c with its DefaultConfig
// counterpart so callers can pass a partial Config.
func (c Config) withDefaults() Config {
	d := DefaultConfig()
	if c.Host == "" {
		c.Host = d.Host
	}
	if c.AuthToken == "" {
		c.AuthToken = d.AuthToken
	}
	if c.PoolSize <= 0 {
		c.PoolSize = d.PoolSize
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = d.HandshakeTimeout
	}
	if c.FetchTimeout <= 0 {
		c.FetchTimeout = d.FetchTimeout
	}
	if c.IdleConnTimeout <= 0 {
		c.IdleConnTimeout = d.IdleConnTimeout
	}
	if c.SearchTimeout <= 0 {
		c.SearchTimeout = d.SearchTimeout
	}
	if c.CacheSize <= 0 {
		c.CacheSize = d.CacheSize
	}
	if c.CacheTTLs == nil {
		c.CacheTTLs = d.CacheTTLs
	}
	if c.CandleHistory <= 0 {
		c.CandleHistory = d.CandleHistory
	}
	if c.UserAgent == "" {
		c.UserAgent = d.UserAgent
	}
	return c
}
