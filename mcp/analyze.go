package tradingviewmcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	tradingview "github.com/teslashibe/tradingview-go"
	"github.com/teslashibe/tradingview-go/ta"
)

// AnalyzeInput drives tradingview_analyze: multi-timeframe, multi-indicator
// technical analysis for one symbol.
type AnalyzeInput struct {
	Symbol      string   `json:"symbol" jsonschema:"description=TradingView symbol (e.g. BINANCE:BTCUSDT). Use tradingview_search_symbols to resolve it first.,required"`
	Resolutions []string `json:"resolutions" jsonschema:"description=Timeframes to analyze in parallel (wire form 1/5/15/60/240/D/W or aliases 1m/15m/1h/4h/1d/1w).,required,minItems=1,maxItems=10"`
	Bars        int      `json:"bars,omitempty" jsonschema:"description=Bars per timeframe to compute indicators over (>=50 required for long-period indicators).,minimum=50,maximum=500,default=300"`
	Indicators  []string `json:"indicators,omitempty" jsonschema:"description=Subset of indicator ids to run (rsi macd ema bollinger atr supertrend vwap obv adx stochrsi divergence sr volume_profile fib price_context). Empty = all."`
}

// IndicatorInput drives tradingview_indicator: one indicator with custom
// parameters on a single timeframe.
type IndicatorInput struct {
	Symbol     string   `json:"symbol" jsonschema:"description=TradingView symbol (e.g. BINANCE:BTCUSDT).,required"`
	Resolution string   `json:"resolution" jsonschema:"description=Bar resolution (1/5/15/60/240/D/W or aliases 1m/15m/1h/4h/1d/1w).,required"`
	Indicator  string   `json:"indicator" jsonschema:"description=Indicator id: rsi macd ema bollinger atr supertrend vwap obv adx stochrsi divergence sr volume_profile fib price_context.,required"`
	Bars       int      `json:"bars,omitempty" jsonschema:"description=Bars to compute over (>=50).,minimum=50,maximum=5000,default=300"`
	Period     int      `json:"period,omitempty" jsonschema:"description=Lookback period override (rsi/atr/adx/bollinger/vwap/supertrend/stochrsi)."`
	Fast       int      `json:"fast,omitempty" jsonschema:"description=MACD fast EMA period (default 12)."`
	Slow       int      `json:"slow,omitempty" jsonschema:"description=MACD slow EMA period (default 26)."`
	Signal     int      `json:"signal,omitempty" jsonschema:"description=MACD signal EMA period (default 9)."`
	StdDev     float64  `json:"std_dev,omitempty" jsonschema:"description=Bollinger standard-deviation multiplier (default 2.0)."`
	Multiplier float64  `json:"multiplier,omitempty" jsonschema:"description=Supertrend ATR multiplier (default 3.0)."`
	Periods    []int    `json:"periods,omitempty" jsonschema:"description=EMA periods (default 20,50,200)."`
	Lookback   int      `json:"lookback,omitempty" jsonschema:"description=Lookback window for sr/volume_profile/fib/price_context."`
}

// AnalyzeOutput is the compact multi-timeframe analysis result.
type AnalyzeOutput struct {
	Symbol     string                    `json:"symbol"`
	Timeframes []ta.TimeframeAnnotations `json:"timeframes"`
}

// IndicatorOutput is a single-indicator reading on one timeframe.
type IndicatorOutput struct {
	Symbol      string          `json:"symbol"`
	Resolution  string          `json:"resolution"`
	Indicator   string          `json:"indicator"`
	Annotations []ta.Annotation `json:"annotations"`
}

func analyze(ctx context.Context, c *tradingview.Client, in AnalyzeInput) (any, error) {
	multi, err := c.FetchMulti(ctx, in.Symbol, in.Resolutions, defaultInt(in.Bars, 300))
	if err != nil {
		return nil, wrapErr(err, "analyze")
	}
	return AnalyzeOutput{Symbol: in.Symbol, Timeframes: ta.EnrichMulti(multi, in.Indicators)}, nil
}

func indicator(ctx context.Context, c *tradingview.Client, in IndicatorInput) (any, error) {
	data, err := c.Fetch(ctx, in.Symbol, in.Resolution, defaultInt(in.Bars, 300))
	if err != nil {
		return nil, wrapErr(err, "indicator")
	}
	anns, err := ta.Compute(in.Indicator, data.Candles, in.Resolution, ta.Params{
		Period:     in.Period,
		Fast:       in.Fast,
		Slow:       in.Slow,
		Signal:     in.Signal,
		StdDev:     in.StdDev,
		Multiplier: in.Multiplier,
		Periods:    in.Periods,
		Lookback:   in.Lookback,
	})
	if err != nil {
		return nil, &mcptool.Error{Code: "invalid_input", Message: err.Error()}
	}
	return IndicatorOutput{Symbol: in.Symbol, Resolution: in.Resolution, Indicator: in.Indicator, Annotations: anns}, nil
}

var analyzeTools = []mcptool.Tool{
	mcptool.Define[*tradingview.Client, AnalyzeInput](
		"tradingview_analyze",
		"Multi-timeframe technical analysis: fetches candles and returns RSI/MACD/EMA/etc readings per timeframe.",
		"",
		analyze,
	),
	mcptool.Define[*tradingview.Client, IndicatorInput](
		"tradingview_indicator",
		"Compute one indicator with custom parameters on a single timeframe and return its reading + state.",
		"",
		indicator,
	),
}
