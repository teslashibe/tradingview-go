# tradingview-go

A small, production-grade Go SDK for fetching OHLCV market data from
TradingView's public WebSocket endpoint, plus a REST symbol-search
helper. Designed for backend / agent use cases where many
symbol+resolution slabs are pulled per second and call latency
matters.

Built around a single concurrency-safe `Client` with a pooled
WebSocket layer and a TTL+LRU cache. Streaming is intentionally
omitted — see [doc.go](./doc.go).

## Install

```bash
go get github.com/teslashibe/tradingview-go
```

Requires Go 1.25+.

## Usage

```go
import (
    "context"
    "fmt"
    tv "github.com/teslashibe/tradingview-go"
)

func main() {
    c := tv.New(tv.Config{}, nil)
    defer c.Close()

    data, err := c.Fetch(context.Background(), "BINANCE:BTCUSDT", "15", 200)
    if err != nil {
        panic(err)
    }
    fmt.Printf("got %d candles\n", len(data.Candles))
}
```

### FetchMulti — multi-resolution in one call

```go
out, _ := c.FetchMulti(ctx, "BINANCE:BTCUSDT",
    []string{"1", "5", "15", "60", "240", "D"}, 200)
for r, slab := range out.Slabs {
    fmt.Printf("%s: %d candles\n", r, len(slab.Candles))
}
```

### SearchSymbols — let the agent discover what's available

```go
matches, _ := c.SearchSymbols(ctx, "BTC", tv.SearchOptions{Type: "crypto"})
for _, m := range matches {
    fmt.Println(m.Full(), "—", m.Description) // "BINANCE:BTCUSDT — Bitcoin / USD"
}
```

## Configuration

`tv.DefaultConfig()` is fine for most uses. The knobs that matter:

| Field | Default | Notes |
|---|---|---|
| `Host` | `data.tradingview.com` | Free anonymous endpoint. Set `prodata.tradingview.com` if you have a paid token. |
| `AuthToken` | `unauthorized_user_token` | Required by the protocol; the free endpoint accepts the literal string. |
| `PoolSize` | 8 | Max concurrent WebSocket connections; also caps `FetchMulti` parallelism. |
| `FetchTimeout` | 20s | Per-fetch ceiling. |
| `IdleConnTimeout` | 5m | Reap idle pooled conns. |
| `EnableCache` | `false` | Off by default for freshest live bars. Flip on if duplicate fetches or rate limits become a problem. |
| `CacheTTLs` | resolution-proportional | 1m=5s … 1D=30m. Only consulted when `EnableCache=true`. |
| `CacheSize` | 256 | LRU entry cap. |

## Errors

Every method returns `*tradingview.Error` with a stable `Code` field
(`symbol_unknown`, `invalid_resolution`, `upstream_timeout`,
`upstream_protocol`, `upstream_http`, `closed`, …) so callers can
react programmatically.

## Testing

```bash
go test ./...                          # unit tests, no network
go test -tags integration ./...        # hits the real TradingView endpoints
```

## Attribution

Protocol-level details (frame encoding, candle parsing, resolution
mapping) are derived from
[gopher-lab/tradingview-go](https://github.com/gopher-lab/tradingview-go),
also MIT-licensed. See [NOTICE](./NOTICE).

## License

MIT — see [LICENSE](./LICENSE).
