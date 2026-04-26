package tradingviewmcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	tradingview "github.com/teslashibe/tradingview-go"
)

// SearchSymbolsInput is the typed input for tradingview_search_symbols.
type SearchSymbolsInput struct {
	Query    string `json:"query" jsonschema:"description=Free-text query (e.g. 'BTC', 'ethereum', 'ES futures').,required"`
	Type     string `json:"type,omitempty" jsonschema:"description=Asset class filter: crypto|stock|forex|futures|index|bond|economic|fund. Empty = all classes."`
	Exchange string `json:"exchange,omitempty" jsonschema:"description=Exchange filter (e.g. BINANCE, COINBASE, NASDAQ). Empty = all exchanges."`
	Limit    int    `json:"limit,omitempty" jsonschema:"description=Maximum matches to return,minimum=1,maximum=50,default=10"`
}

func searchSymbols(ctx context.Context, c *tradingview.Client, in SearchSymbolsInput) (any, error) {
	matches, err := c.SearchSymbols(ctx, in.Query, tradingview.SearchOptions{
		Type:     in.Type,
		Exchange: in.Exchange,
		Limit:    defaultInt(in.Limit, 10),
	})
	if err != nil {
		return nil, wrapErr(err, "search_symbols")
	}
	return mcptool.PageOf(matches, "", defaultInt(in.Limit, 10)), nil
}

var searchTools = []mcptool.Tool{
	mcptool.Define[*tradingview.Client, SearchSymbolsInput](
		"tradingview_search_symbols",
		"Search TradingView's symbol catalogue; use the returned prefix:symbol (e.g. BINANCE:BTCUSDT) with fetch_candles.",
		"SearchSymbols",
		searchSymbols,
	),
}
