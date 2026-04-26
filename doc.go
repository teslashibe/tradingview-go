// Package tradingview is a Go SDK for fetching OHLCV market data from
// TradingView's public WebSocket endpoint, plus a REST symbol-search
// helper. It is designed for agent / backend use cases where many
// symbol+resolution slabs are pulled per second and call latency
// matters.
//
// The package exposes a single concurrency-safe [Client] with three
// methods:
//
//   - Fetch          one symbol, one resolution
//   - FetchMulti     one symbol, several resolutions in parallel
//   - SearchSymbols  free-text discovery against the public symbol-search REST endpoint
//
// Internally Client maintains a pool of authenticated WebSocket
// connections (re-used across fetches) and a TTL cache keyed by
// (symbol, resolution, bars). The cache is intentionally short-lived
// so the agent never reasons over very stale data; see [DefaultConfig]
// for the per-resolution TTL ladder.
//
// The default host is data.tradingview.com (anonymous, free). Point
// [Config.Host] at prodata.tradingview.com if you have an authenticated
// token.
//
// The streaming model offered by the upstream gopher-lab SDK is
// intentionally omitted — for an LLM agent reasoning over short
// horizons, polling Fetch / FetchMulti is simpler and gives equivalent
// freshness given the cache TTLs. Re-introduce a streamer here if
// intra-bar latency ever becomes a bottleneck.
package tradingview
