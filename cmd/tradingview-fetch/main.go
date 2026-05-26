// Command tradingview-fetch is a small CLI wrapper around the
// tradingview-go SDK for ad-hoc fetches. Intended for shell scripts,
// dev/test loops, and quick agent-side analytics that don't want to
// spin up the MCP server.
//
// Usage:
//
//	tradingview-fetch -symbol COINBASE:ZECUSD -resolution 4h -bars 300
//	tradingview-fetch -symbol COINBASE:ZECUSD -resolutions 1h,4h,1d,1w -bars 300
//	tradingview-fetch -search ZEC -type crypto -exchange COINBASE
//
// Output is a single JSON document on stdout.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tradingview "github.com/teslashibe/tradingview-go"
)

func main() {
	var (
		symbol      = flag.String("symbol", "", "TradingView symbol, e.g. COINBASE:ZECUSD")
		resolution  = flag.String("resolution", "", "Single resolution, e.g. 4h")
		resolutions = flag.String("resolutions", "", "Comma-separated resolutions, e.g. 1h,4h,1d")
		bars        = flag.Int("bars", 300, "Bars to fetch")
		searchQ     = flag.String("search", "", "Run a symbol search instead of a fetch")
		searchType  = flag.String("type", "", "Asset class filter for search (crypto, stock, ...)")
		searchEx    = flag.String("exchange", "", "Exchange filter for search")
		searchLimit = flag.Int("limit", 25, "Max search hits")
		timeout     = flag.Duration("timeout", 30*time.Second, "Overall request timeout")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := tradingview.New(tradingview.Config{}, nil)
	defer client.Close()

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if *searchQ != "" {
		matches, err := client.SearchSymbols(ctx, *searchQ, tradingview.SearchOptions{
			Type:     *searchType,
			Exchange: *searchEx,
			Limit:    *searchLimit,
		})
		if err != nil {
			fail("search: %v", err)
		}
		_ = enc.Encode(matches)
		return
	}

	if *symbol == "" {
		fail("must provide -symbol (or -search)")
	}

	if *resolutions != "" {
		parts := splitCSV(*resolutions)
		out, err := client.FetchMulti(ctx, *symbol, parts, *bars)
		if err != nil {
			fail("fetch_multi: %v", err)
		}
		_ = enc.Encode(out)
		return
	}

	if *resolution == "" {
		fail("must provide -resolution or -resolutions")
	}
	out, err := client.Fetch(ctx, *symbol, *resolution, *bars)
	if err != nil {
		fail("fetch: %v", err)
	}
	_ = enc.Encode(out)
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func fail(format string, args ...any) {
	fmt.Fprintln(os.Stderr, "tradingview-fetch:", fmt.Sprintf(format, args...))
	os.Exit(1)
}
