package tradingviewmcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	tradingview "github.com/teslashibe/tradingview-go"
)

// FetchCandlesInput is the typed input for tradingview_fetch_candles.
type FetchCandlesInput struct {
	Symbol     string `json:"symbol" jsonschema:"description=TradingView symbol (e.g. BINANCE:BTCUSDT). Use tradingview_search_symbols to discover canonical EXCHANGE:SYMBOL forms.,required"`
	Resolution string `json:"resolution" jsonschema:"description=Bar resolution. Accepts wire form (1, 5, 15, 60, 240, D, W, M) or friendly aliases (1m, 15m, 1h, 4h, 1d, 1w, 1mn).,required"`
	Bars       int    `json:"bars,omitempty" jsonschema:"description=Number of historical bars to return. The last bar is the currently-forming (unclosed) candle.,minimum=1,maximum=5000,default=300"`
}

// FetchMultiInput is the typed input for tradingview_fetch_multi.
type FetchMultiInput struct {
	Symbol      string   `json:"symbol" jsonschema:"description=TradingView symbol (e.g. BINANCE:BTCUSDT).,required"`
	Resolutions []string `json:"resolutions" jsonschema:"description=Resolutions to fetch in parallel; same accepted formats as fetch_candles.,required,minItems=1,maxItems=12"`
	Bars        int      `json:"bars,omitempty" jsonschema:"description=Bars per resolution. The last bar in each slab is the currently-forming candle.,minimum=1,maximum=500,default=200"`
}

func fetchCandles(ctx context.Context, c *tradingview.Client, in FetchCandlesInput) (any, error) {
	data, err := c.Fetch(ctx, in.Symbol, in.Resolution, defaultInt(in.Bars, 300))
	if err != nil {
		return nil, wrapErr(err, "fetch_candles")
	}
	return data, nil
}

func fetchMulti(ctx context.Context, c *tradingview.Client, in FetchMultiInput) (any, error) {
	data, err := c.FetchMulti(ctx, in.Symbol, in.Resolutions, defaultInt(in.Bars, 200))
	if err != nil {
		return nil, wrapErr(err, "fetch_multi")
	}
	return data, nil
}

var fetchTools = []mcptool.Tool{
	mcptool.Define[*tradingview.Client, FetchCandlesInput](
		"tradingview_fetch_candles",
		"Fetch OHLCV candles for one symbol/resolution. The last bar is the currently-forming (unclosed) candle.",
		"Fetch",
		fetchCandles,
	),
	mcptool.Define[*tradingview.Client, FetchMultiInput](
		"tradingview_fetch_multi",
		"Fetch OHLCV for one symbol across several resolutions in parallel. Prefer over repeated fetch_candles calls.",
		"FetchMulti",
		fetchMulti,
	),
}
