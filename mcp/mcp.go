// Package tradingviewmcp exposes github.com/teslashibe/tradingview-go
// as a set of mcptool.Tool values backing a single mcptool.Provider.
//
// The tool surface:
//
//   - tradingview_fetch_candles   single symbol, single resolution
//   - tradingview_fetch_multi     single symbol, several resolutions in parallel
//   - tradingview_search_symbols  free-text discovery against TradingView's REST endpoint
//   - tradingview_analyze         multi-timeframe, multi-indicator TA (RSI/MACD/EMA/…)
//   - tradingview_indicator       one indicator with custom params on one timeframe
//
// All tools take a *tradingview.Client and are safe to share across
// concurrent MCP requests; the underlying client owns its WebSocket
// connection pool.
//
// Registration into an agent-setup style harness is a single line in
// the harness's mcp/platforms wiring file.
package tradingviewmcp

import "github.com/teslashibe/mcptool"

// Provider implements mcptool.Provider for tradingview-go. Zero value
// is ready to use.
type Provider struct{}

// Platform returns "tradingview". Tool names are prefixed accordingly
// (tradingview_fetch_candles, tradingview_search_symbols, ...).
func (Provider) Platform() string { return "tradingview" }

// Tools returns every tradingview_* tool exposed by this provider.
// Order is cosmetic; the host registry sorts by name.
func (Provider) Tools() []mcptool.Tool {
	out := make([]mcptool.Tool, 0, 5)
	out = append(out, fetchTools...)
	out = append(out, searchTools...)
	out = append(out, analyzeTools...)
	return out
}
