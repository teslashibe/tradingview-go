package tradingview

import "time"

// Candle is a single OHLCV bar. Timestamp is unix seconds at the bar
// open. Volume is in base-asset units (TradingView convention).
type Candle struct {
	Timestamp int64   `json:"t"`
	Open      float64 `json:"o"`
	High      float64 `json:"h"`
	Low       float64 `json:"l"`
	Close     float64 `json:"c"`
	Volume    float64 `json:"v"`
}

// ChartData is the result of a single-resolution Fetch.
type ChartData struct {
	Symbol     string    `json:"symbol"`
	Resolution string    `json:"resolution"`
	Candles    []Candle  `json:"candles"`
	FetchedAt  time.Time `json:"fetched_at"`
	Cached     bool      `json:"cached"`
	LatencyMs  int64     `json:"latency_ms"`
}

// MultiChartData is the result of FetchMulti: one ChartData per
// resolution, plus per-resolution errors so a partial failure does
// not lose the slabs that did succeed.
type MultiChartData struct {
	Symbol    string                 `json:"symbol"`
	Slabs     map[string]*ChartData  `json:"slabs"`
	Errors    map[string]string      `json:"errors,omitempty"`
	FetchedAt time.Time              `json:"fetched_at"`
	LatencyMs int64                  `json:"latency_ms"`
}

// SymbolMatch is a single hit from SearchSymbols. Use the Full() helper
// to get the canonical "EXCHANGE:SYMBOL" string suitable for Fetch.
type SymbolMatch struct {
	Symbol      string   `json:"symbol"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Exchange    string   `json:"exchange"`
	Prefix      string   `json:"prefix,omitempty"`
	Currency    string   `json:"currency,omitempty"`
	SourceID    string   `json:"source_id,omitempty"`
	TypeSpecs   []string `json:"typespecs,omitempty"`
}

// Full returns the canonical "PREFIX:SYMBOL" form expected by Fetch.
// Falls back to the bare symbol if no prefix is present.
func (m SymbolMatch) Full() string {
	if m.Prefix != "" {
		return m.Prefix + ":" + m.Symbol
	}
	return m.Symbol
}

// SearchOptions narrows a SearchSymbols query.
type SearchOptions struct {
	// Type filters by asset class: "crypto", "stock", "forex",
	// "futures", "index", "bond", "economic", "fund". Empty = all.
	Type string
	// Exchange filters by exchange identifier (e.g. "BINANCE"). Empty = all.
	Exchange string
	// Lang is the human-readable language for descriptions; default "en".
	Lang string
	// Limit caps the number of returned matches client-side. 0 = no cap.
	Limit int
}

// tvMessage is the shape of a TradingView protocol RPC. Lowercase
// field names match the wire format.
type tvMessage struct {
	Method string `json:"m"`
	Params []any  `json:"p"`
}
