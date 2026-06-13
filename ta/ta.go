// Package ta computes technical-analysis indicators over TradingView OHLCV
// candles and renders them as compact, human-readable annotations
// ({Indicator, Value, State}) rather than raw numeric series. This keeps
// multi-timeframe, multi-indicator analysis cheap to pass through an LLM:
// the model reasons over interpreted signals ("RSI(14) [240] = 62.3 ->
// Approaching overbought") instead of hundreds of bars.
//
// The indicator math is delegated to github.com/Bitvested/ta.go; this package
// only adapts it to tradingview.Candle and adds interpretation.
package ta

import (
	"fmt"
	"math"
	"sort"
	"strings"

	tradingview "github.com/teslashibe/tradingview-go"

	bands "github.com/Bitvested/ta.go/src/bands"
	experimental "github.com/Bitvested/ta.go/src/experimental"
	indicators "github.com/Bitvested/ta.go/src/indicators"
	ma "github.com/Bitvested/ta.go/src/moving_averages"
	stats "github.com/Bitvested/ta.go/src/statistics"
)

// minBars is the smallest candle count an enricher will run on; below this the
// longer-period indicators (EMA200, etc.) are undefined.
const minBars = 50

// Annotation is one indicator reading: a label, the computed value(s), and a
// terse interpretation of what the value implies.
type Annotation struct {
	Indicator string `json:"indicator"`
	Value     string `json:"value"`
	State     string `json:"state"`
}

// TimeframeAnnotations groups one timeframe's indicator readings.
type TimeframeAnnotations struct {
	Timeframe   string       `json:"timeframe"`
	Bars        int          `json:"bars"`
	Annotations []Annotation `json:"annotations"`
}

// Enricher computes annotations from a candle series for one timeframe.
type Enricher interface {
	// Name is the stable indicator id used by the single-indicator API.
	Name() string
	Enrich(candles []tradingview.Candle, timeframe string) []Annotation
}

// DefaultPipeline returns the standard ordered enricher set with sensible
// defaults. Callers mutate the returned slice freely; it is freshly built.
func DefaultPipeline() []Enricher {
	return []Enricher{
		&RSIEnricher{Period: 14},
		&MACDEnricher{Fast: 12, Slow: 26, Signal: 9},
		&EMAEnricher{Periods: []int{20, 50, 200}},
		&BollingerEnricher{Period: 20, StdDev: 2.0},
		&ATREnricher{Period: 14},
		&SupertrendEnricher{Period: 10, Multiplier: 3.0},
		&VWAPEnricher{Period: 20},
		&OBVEnricher{},
		&ADXEnricher{Period: 14},
		&StochRSIEnricher{RSIPeriod: 14, StochPeriod: 14, SmoothK: 3, SmoothD: 3},
		&DivergenceEnricher{Period: 14},
		&SupportResistanceEnricher{Lookback: 50},
		&VolumeProfileEnricher{Bins: 24, Lookback: 100},
		&FibLevelsEnricher{Lookback: 200},
		&PriceContextEnricher{Lookback: 200},
	}
}

// IndicatorNames returns the ids accepted by Compute / the indicators filter,
// in pipeline order.
func IndicatorNames() []string {
	p := DefaultPipeline()
	out := make([]string, 0, len(p))
	for _, e := range p {
		out = append(out, e.Name())
	}
	return out
}

// EnrichTimeframe runs the given pipeline (or DefaultPipeline if nil) over one
// timeframe's candles. Series shorter than minBars yield no annotations.
func EnrichTimeframe(candles []tradingview.Candle, timeframe string, pipeline []Enricher) []Annotation {
	if len(candles) < minBars {
		return nil
	}
	if pipeline == nil {
		pipeline = DefaultPipeline()
	}
	var out []Annotation
	for _, e := range pipeline {
		out = append(out, e.Enrich(candles, timeframe)...)
	}
	return out
}

// EnrichMulti runs the pipeline across every timeframe in a FetchMulti result,
// returning one group per timeframe sorted by timeframe priority. If only is
// non-empty, only those indicator ids run.
func EnrichMulti(multi *tradingview.MultiChartData, only []string) []TimeframeAnnotations {
	if multi == nil {
		return nil
	}
	pipeline := filterPipeline(DefaultPipeline(), only)
	groups := make([]TimeframeAnnotations, 0, len(multi.Slabs))
	for tf, slab := range multi.Slabs {
		if slab == nil {
			continue
		}
		groups = append(groups, TimeframeAnnotations{
			Timeframe:   tf,
			Bars:        len(slab.Candles),
			Annotations: EnrichTimeframe(slab.Candles, tf, pipeline),
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return tfRank(groups[i].Timeframe) < tfRank(groups[j].Timeframe)
	})
	return groups
}

// Compute runs a single named indicator with optional parameter overrides over
// one timeframe's candles. Unknown names return an error listing valid ids.
func Compute(name string, candles []tradingview.Candle, timeframe string, p Params) ([]Annotation, error) {
	e, err := enricherFor(strings.ToLower(strings.TrimSpace(name)), p)
	if err != nil {
		return nil, err
	}
	if len(candles) < minBars {
		return nil, fmt.Errorf("ta: need at least %d candles for %q, got %d", minBars, name, len(candles))
	}
	return e.Enrich(candles, timeframe), nil
}

// Params carries optional per-indicator overrides for Compute. Zero values
// fall back to the indicator's default.
type Params struct {
	Period     int     `json:"period,omitempty"`
	Fast       int     `json:"fast,omitempty"`
	Slow       int     `json:"slow,omitempty"`
	Signal     int     `json:"signal,omitempty"`
	StdDev     float64 `json:"std_dev,omitempty"`
	Multiplier float64 `json:"multiplier,omitempty"`
	Periods    []int   `json:"periods,omitempty"`
	Lookback   int     `json:"lookback,omitempty"`
}

func filterPipeline(pipeline []Enricher, only []string) []Enricher {
	if len(only) == 0 {
		return pipeline
	}
	want := make(map[string]bool, len(only))
	for _, n := range only {
		want[strings.ToLower(strings.TrimSpace(n))] = true
	}
	var out []Enricher
	for _, e := range pipeline {
		if want[e.Name()] {
			out = append(out, e)
		}
	}
	return out
}

func enricherFor(name string, p Params) (Enricher, error) {
	pi := func(v, d int) int {
		if v > 0 {
			return v
		}
		return d
	}
	pf := func(v, d float64) float64 {
		if v > 0 {
			return v
		}
		return d
	}
	switch name {
	case "rsi":
		return &RSIEnricher{Period: pi(p.Period, 14)}, nil
	case "macd":
		return &MACDEnricher{Fast: pi(p.Fast, 12), Slow: pi(p.Slow, 26), Signal: pi(p.Signal, 9)}, nil
	case "ema":
		periods := p.Periods
		if len(periods) == 0 {
			periods = []int{20, 50, 200}
		}
		return &EMAEnricher{Periods: periods}, nil
	case "bollinger":
		return &BollingerEnricher{Period: pi(p.Period, 20), StdDev: pf(p.StdDev, 2.0)}, nil
	case "atr":
		return &ATREnricher{Period: pi(p.Period, 14)}, nil
	case "supertrend":
		return &SupertrendEnricher{Period: pi(p.Period, 10), Multiplier: pf(p.Multiplier, 3.0)}, nil
	case "vwap":
		return &VWAPEnricher{Period: pi(p.Period, 20)}, nil
	case "obv":
		return &OBVEnricher{}, nil
	case "adx":
		return &ADXEnricher{Period: pi(p.Period, 14)}, nil
	case "stochrsi":
		return &StochRSIEnricher{RSIPeriod: pi(p.Period, 14), StochPeriod: 14, SmoothK: 3, SmoothD: 3}, nil
	case "divergence":
		return &DivergenceEnricher{Period: pi(p.Period, 14)}, nil
	case "sr":
		return &SupportResistanceEnricher{Lookback: pi(p.Lookback, 50)}, nil
	case "volume_profile":
		return &VolumeProfileEnricher{Bins: 24, Lookback: pi(p.Lookback, 100)}, nil
	case "fib":
		return &FibLevelsEnricher{Lookback: pi(p.Lookback, 200)}, nil
	case "price_context":
		return &PriceContextEnricher{Lookback: pi(p.Lookback, 200)}, nil
	default:
		return nil, fmt.Errorf("ta: unknown indicator %q; valid: %s", name, strings.Join(IndicatorNames(), ", "))
	}
}

// --- candle slice helpers ---

func closes(c []tradingview.Candle) []float64 {
	out := make([]float64, len(c))
	for i, b := range c {
		out[i] = b.Close
	}
	return out
}

func hlc(c []tradingview.Candle) [][]float64 {
	out := make([][]float64, len(c))
	for i, b := range c {
		out[i] = []float64{b.High, b.Close, b.Low}
	}
	return out
}

func closeVolume(c []tradingview.Candle) [][]float64 {
	out := make([][]float64, len(c))
	for i, b := range c {
		out[i] = []float64{b.Close, b.Volume}
	}
	return out
}

func volumeClose(c []tradingview.Candle) [][]float64 {
	out := make([][]float64, len(c))
	for i, b := range c {
		out[i] = []float64{b.Volume, b.Close}
	}
	return out
}

func last(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	return s[len(s)-1]
}

func findSwing(c []tradingview.Candle) (high, low float64) {
	high, low = c[0].High, c[0].Low
	for _, b := range c {
		if b.High > high {
			high = b.High
		}
		if b.Low < low {
			low = b.Low
		}
	}
	return high, low
}

var tfOrder = map[string]int{
	"1": 1, "5": 2, "15": 3, "30": 4, "60": 5, "120": 6, "240": 7, "D": 8, "W": 9, "M": 10,
}

func tfRank(tf string) int {
	if r, ok := tfOrder[tf]; ok {
		return r
	}
	return 99
}

// --- Enrichers ---

type RSIEnricher struct{ Period int }

func (e *RSIEnricher) Name() string { return "rsi" }
func (e *RSIEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	v := last(indicators.Rsi(closes(c), e.Period))
	return []Annotation{{Indicator: fmt.Sprintf("RSI(%d) [%s]", e.Period, tf), Value: fmt.Sprintf("%.1f", v), State: rsiState(v)}}
}

func rsiState(v float64) string {
	switch {
	case v >= 70:
		return "Overbought"
	case v >= 60:
		return "Approaching overbought"
	case v <= 30:
		return "Oversold"
	case v <= 40:
		return "Approaching oversold"
	default:
		return "Neutral"
	}
}

type MACDEnricher struct{ Fast, Slow, Signal int }

func (e *MACDEnricher) Name() string { return "macd" }
func (e *MACDEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	cl := closes(c)
	macd := last(indicators.Macd(cl, e.Fast, e.Slow))
	sig := last(indicators.MacdSignal(cl, e.Fast, e.Slow, e.Signal))
	hist := last(indicators.MacdBars(cl, e.Fast, e.Slow, e.Signal))
	state := "Neutral"
	switch {
	case macd > sig && hist > 0:
		state = "Bullish (MACD above signal, positive histogram)"
	case macd < sig && hist < 0:
		state = "Bearish (MACD below signal, negative histogram)"
	case macd > sig && hist < 0:
		state = "Bearish momentum weakening"
	case macd < sig && hist > 0:
		state = "Bullish momentum weakening"
	}
	return []Annotation{{
		Indicator: fmt.Sprintf("MACD(%d/%d/%d) [%s]", e.Fast, e.Slow, e.Signal, tf),
		Value:     fmt.Sprintf("MACD=%.4f Signal=%.4f Hist=%.4f", macd, sig, hist),
		State:     state,
	}}
}

type EMAEnricher struct{ Periods []int }

func (e *EMAEnricher) Name() string { return "ema" }
func (e *EMAEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	cl := closes(c)
	price := cl[len(cl)-1]
	vals := make(map[int]float64)
	for _, p := range e.Periods {
		if len(cl) < p {
			continue
		}
		vals[p] = last(ma.Ema(cl, p))
	}
	valStr := fmt.Sprintf("Price=%.4f", price)
	for _, p := range e.Periods {
		if v, ok := vals[p]; ok {
			valStr += fmt.Sprintf(" EMA%d=%.4f", p, v)
		}
	}
	return []Annotation{{Indicator: fmt.Sprintf("EMA(%v) [%s]", e.Periods, tf), Value: valStr, State: emaState(price, vals)}}
}

func emaState(price float64, vals map[int]float64) string {
	ema20, has20 := vals[20]
	ema50, has50 := vals[50]
	ema200, has200 := vals[200]
	if has200 {
		if price > ema200 && has20 && ema20 > ema50 {
			return "Strong bullish (price above 200, 20>50)"
		}
		if price < ema200 && has20 && ema20 < ema50 {
			return "Strong bearish (price below 200, 20<50)"
		}
	}
	if has20 && has50 {
		if price > ema20 && ema20 > ema50 {
			return "Bullish structure (price > 20 > 50)"
		}
		if price < ema20 && ema20 < ema50 {
			return "Bearish structure (price < 20 < 50)"
		}
		if price > ema20 && price < ema50 {
			return "Recovery attempt (above 20, below 50)"
		}
	}
	return "Mixed/Transitional"
}

type BollingerEnricher struct {
	Period int
	StdDev float64
}

func (e *BollingerEnricher) Name() string { return "bollinger" }
func (e *BollingerEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	cl := closes(c)
	bb := bands.Bands(cl, e.Period, e.StdDev)
	if len(bb) == 0 {
		return nil
	}
	lb := bb[len(bb)-1]
	// bands.Bands orders the triple high-to-low; normalize so upper/lower
	// are correct regardless of the library's ordering convention.
	middle := lb[1]
	upper, lower := lb[0], lb[2]
	if lower > upper {
		upper, lower = lower, upper
	}
	price := cl[len(cl)-1]
	width := (upper - lower) / middle * 100
	state := "Within bands"
	switch {
	case price >= upper:
		state = "At upper band (potential resistance/breakout)"
	case price <= lower:
		state = "At lower band (potential support/bounce)"
	case width < 5:
		state = "Squeeze (low volatility, expansion imminent)"
	}
	return []Annotation{{
		Indicator: fmt.Sprintf("Bollinger(%d,%.1f) [%s]", e.Period, e.StdDev, tf),
		Value:     fmt.Sprintf("Upper=%.4f Mid=%.4f Lower=%.4f Width=%.1f%%", upper, middle, lower, width),
		State:     state,
	}}
}

type ATREnricher struct{ Period int }

func (e *ATREnricher) Name() string { return "atr" }
func (e *ATREnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	v := last(indicators.Atr(hlc(c), e.Period))
	price := c[len(c)-1].Close
	pct := (v / price) * 100
	return []Annotation{{Indicator: fmt.Sprintf("ATR(%d) [%s]", e.Period, tf), Value: fmt.Sprintf("%.4f (%.2f%% of price)", v, pct), State: atrState(pct)}}
}

func atrState(pct float64) string {
	switch {
	case pct > 5:
		return "Extremely volatile"
	case pct > 3:
		return "High volatility"
	case pct > 1.5:
		return "Moderate volatility"
	default:
		return "Low volatility"
	}
}

type SupertrendEnricher struct {
	Period     int
	Multiplier float64
}

func (e *SupertrendEnricher) Name() string { return "supertrend" }
func (e *SupertrendEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	st := indicators.Supertrend(hlc(c), e.Period, e.Multiplier)
	if len(st) == 0 {
		return nil
	}
	lst := st[len(st)-1]
	state := "Bullish (price above supertrend)"
	if lst[1] < 0 {
		state = "Bearish (price below supertrend)"
	}
	return []Annotation{{Indicator: fmt.Sprintf("Supertrend(%d,%.1f) [%s]", e.Period, e.Multiplier, tf), Value: fmt.Sprintf("%.4f", lst[0]), State: state}}
}

type VWAPEnricher struct{ Period int }

func (e *VWAPEnricher) Name() string { return "vwap" }
func (e *VWAPEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	v := last(indicators.Vwap(closeVolume(c), e.Period))
	price := c[len(c)-1].Close
	state := "Price above VWAP (bullish bias)"
	if price < v {
		state = "Price below VWAP (bearish bias)"
	}
	return []Annotation{{Indicator: fmt.Sprintf("VWAP(%d) [%s]", e.Period, tf), Value: fmt.Sprintf("%.4f (Price=%.4f)", v, price), State: state}}
}

type OBVEnricher struct{}

func (e *OBVEnricher) Name() string { return "obv" }
func (e *OBVEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	obv := indicators.Obv(volumeClose(c))
	if len(obv) < 20 {
		return nil
	}
	cur, prev := obv[len(obv)-1], obv[len(obv)-10]
	state := "Flat (no volume conviction)"
	switch {
	case cur > prev*1.1:
		state = "Rising (accumulation)"
	case cur < prev*0.9:
		state = "Falling (distribution)"
	}
	return []Annotation{{Indicator: fmt.Sprintf("OBV [%s]", tf), Value: fmt.Sprintf("%.0f (10-bar delta: %.0f)", cur, cur-prev), State: state}}
}

type ADXEnricher struct{ Period int }

func (e *ADXEnricher) Name() string { return "adx" }
func (e *ADXEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	v := last(indicators.Adx(hlc(c), e.Period))
	state := "No trend"
	switch {
	case v >= 50:
		state = "Extremely strong trend"
	case v >= 25:
		state = "Strong trend"
	case v >= 20:
		state = "Emerging trend"
	}
	return []Annotation{{Indicator: fmt.Sprintf("ADX(%d) [%s]", e.Period, tf), Value: fmt.Sprintf("%.1f", v), State: state}}
}

type StochRSIEnricher struct{ RSIPeriod, StochPeriod, SmoothK, SmoothD int }

func (e *StochRSIEnricher) Name() string { return "stochrsi" }
func (e *StochRSIEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	sr := indicators.StochRsi(closes(c), e.RSIPeriod, e.StochPeriod, e.SmoothK, e.SmoothD)
	if len(sr) == 0 {
		return nil
	}
	lsr := sr[len(sr)-1]
	k, d := lsr[0], lsr[1]
	state := "Neutral"
	switch {
	case k > 80 && d > 80:
		state = "Overbought"
	case k < 20 && d < 20:
		state = "Oversold"
	case k > d && k < 30:
		state = "Bullish crossover from oversold"
	case k < d && k > 70:
		state = "Bearish crossover from overbought"
	}
	return []Annotation{{Indicator: fmt.Sprintf("StochRSI(%d,%d) [%s]", e.RSIPeriod, e.StochPeriod, tf), Value: fmt.Sprintf("K=%.1f D=%.1f", k, d), State: state}}
}

type DivergenceEnricher struct{ Period int }

func (e *DivergenceEnricher) Name() string { return "divergence" }
func (e *DivergenceEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	v := last(indicators.Rsi_divergence(closes(c), e.Period, indicators.Rsi))
	if v == 0 {
		return nil
	}
	state := "Bullish divergence detected"
	if v < 0 {
		state = "Bearish divergence detected"
	}
	return []Annotation{{Indicator: fmt.Sprintf("RSI Divergence [%s]", tf), Value: fmt.Sprintf("%.2f", v), State: state}}
}

type SupportResistanceEnricher struct{ Lookback int }

func (e *SupportResistanceEnricher) Name() string { return "sr" }
func (e *SupportResistanceEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	if len(c) < e.Lookback {
		return nil
	}
	cl := closes(c[len(c)-e.Lookback:])
	price := cl[len(cl)-1]
	recentHigh := stats.RecentHigh(cl, e.Lookback)
	recentLow := stats.RecentLow(cl, e.Lookback)
	supLevel := experimental.LineCalc(len(cl)-1, experimental.Support(cl, recentLow))
	resLevel := experimental.LineCalc(len(cl)-1, experimental.Resistance(cl, recentHigh))
	distToSup := ((price - supLevel) / price) * 100
	distToRes := ((resLevel - price) / price) * 100
	return []Annotation{{
		Indicator: fmt.Sprintf("S/R Levels [%s]", tf),
		Value:     fmt.Sprintf("Support=%.4f (%.1f%% away) Resistance=%.4f (%.1f%% away)", supLevel, distToSup, resLevel, distToRes),
		State:     srState(distToSup, distToRes),
	}}
}

func srState(distSup, distRes float64) string {
	switch {
	case distSup < 1:
		return "Near support (potential bounce zone)"
	case distRes < 1:
		return "Near resistance (potential rejection zone)"
	case distSup < distRes:
		return "Closer to support"
	default:
		return "Closer to resistance"
	}
}

type VolumeProfileEnricher struct {
	Bins     int
	Lookback int
}

func (e *VolumeProfileEnricher) Name() string { return "volume_profile" }
func (e *VolumeProfileEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	if len(c) < e.Lookback {
		return nil
	}
	window := c[len(c)-e.Lookback:]
	poc, vah, val := e.compute(window)
	price := c[len(c)-1].Close
	state := "Mid-range"
	switch {
	case price >= vah:
		state = "Above Value Area (potential breakout or mean-revert back)"
	case price <= val:
		state = "Below Value Area (potential breakdown or mean-revert up)"
	case math.Abs(price-poc)/price < 0.005:
		state = "At POC (high-volume node, strong magnet)"
	}
	return []Annotation{{Indicator: fmt.Sprintf("Volume Profile(%d) [%s]", e.Lookback, tf), Value: fmt.Sprintf("POC=%.4f VAH=%.4f VAL=%.4f", poc, vah, val), State: state}}
}

func (e *VolumeProfileEnricher) compute(c []tradingview.Candle) (poc, vah, val float64) {
	if len(c) == 0 {
		return 0, 0, 0
	}
	minPrice, maxPrice := c[0].Low, c[0].High
	for _, b := range c {
		if b.Low < minPrice {
			minPrice = b.Low
		}
		if b.High > maxPrice {
			maxPrice = b.High
		}
	}
	if maxPrice == minPrice {
		return minPrice, maxPrice, minPrice
	}
	binSize := (maxPrice - minPrice) / float64(e.Bins)
	volAt := make([]float64, e.Bins)
	total := 0.0
	for _, b := range c {
		startBin := int((b.Low - minPrice) / binSize)
		endBin := int((b.High - minPrice) / binSize)
		if startBin < 0 {
			startBin = 0
		}
		if endBin >= e.Bins {
			endBin = e.Bins - 1
		}
		volPer := b.Volume / float64(endBin-startBin+1)
		for i := startBin; i <= endBin; i++ {
			volAt[i] += volPer
			total += volPer
		}
	}
	pocBin, maxVol := 0, 0.0
	for i, v := range volAt {
		if v > maxVol {
			maxVol, pocBin = v, i
		}
	}
	poc = minPrice + (float64(pocBin)+0.5)*binSize
	target := total * 0.70
	acc := volAt[pocBin]
	lowBin, highBin := pocBin, pocBin
	for acc < target {
		expandLow, expandHigh := lowBin-1, highBin+1
		var lowVol, highVol float64
		if expandLow >= 0 {
			lowVol = volAt[expandLow]
		}
		if expandHigh < e.Bins {
			highVol = volAt[expandHigh]
		}
		if lowVol == 0 && highVol == 0 {
			break
		}
		if highVol >= lowVol && expandHigh < e.Bins {
			highBin = expandHigh
			acc += highVol
		} else if expandLow >= 0 {
			lowBin = expandLow
			acc += lowVol
		} else if expandHigh < e.Bins {
			highBin = expandHigh
			acc += highVol
		}
	}
	val = minPrice + float64(lowBin)*binSize
	vah = minPrice + float64(highBin+1)*binSize
	return poc, vah, val
}

type FibLevelsEnricher struct{ Lookback int }

func (e *FibLevelsEnricher) Name() string { return "fib" }
func (e *FibLevelsEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	if len(c) < e.Lookback {
		return nil
	}
	window := c[len(c)-e.Lookback:]
	high, low := findSwing(window)
	if high <= low {
		return nil
	}
	price := c[len(c)-1].Close
	rng := high - low
	r382, r500, r618, r786 := high-rng*0.382, high-rng*0.5, high-rng*0.618, high-rng*0.786
	ext1272, ext1414, ext1618, ext2000, ext2618 := high+rng*0.272, high+rng*0.414, high+rng*0.618, high+rng*1.0, high+rng*1.618
	return []Annotation{{
		Indicator: fmt.Sprintf("Fib Levels [%s]", tf),
		Value: fmt.Sprintf("Swing: %.2f->%.2f | Retrace: 0.382=%.2f 0.5=%.2f 0.618=%.2f 0.786=%.2f | Ext: 1.272=%.2f 1.414=%.2f 1.618=%.2f 2.0=%.2f 2.618=%.2f",
			low, high, r382, r500, r618, r786, ext1272, ext1414, ext1618, ext2000, ext2618),
		State: fibState(price, high, low, r382, r500, r618, r786),
	}}
}

func fibState(price, high, low, r382, r500, r618, r786 float64) string {
	switch {
	case price >= high:
		return "Above swing high (price discovery, extension targets active: 1.272, 1.618)"
	case price >= r382:
		return "Shallow pullback (above 0.382 retrace, strong trend intact)"
	case price >= r500:
		return "Mid pullback (between 0.382 and 0.5 retrace)"
	case price >= r618:
		return "Deep pullback (golden pocket 0.5-0.618, high-probability bounce zone)"
	case price >= r786:
		return "Very deep pullback (between 0.618 and 0.786, trend weakening)"
	case price <= low:
		return "Below swing low (structure broken)"
	default:
		return "Below 0.786 retrace (trend likely failing)"
	}
}

type PriceContextEnricher struct{ Lookback int }

func (e *PriceContextEnricher) Name() string { return "price_context" }
func (e *PriceContextEnricher) Enrich(c []tradingview.Candle, tf string) []Annotation {
	if len(c) < e.Lookback {
		return nil
	}
	window := c[len(c)-e.Lookback:]
	high, low := findSwing(window)
	openPrice := window[0].Open
	price := c[len(c)-1].Close
	pctFromHigh := ((price - high) / high) * 100
	pctFromLow := ((price - low) / low) * 100
	velocity := ((price - openPrice) / openPrice) * 100
	return []Annotation{{
		Indicator: fmt.Sprintf("Price Context [%s]", tf),
		Value: fmt.Sprintf("Range: %.2f->%.2f (%dbar) | Now: %.2f | From High: %.1f%% | From Low: %.1f%% | Velocity: %.1f%%",
			low, high, e.Lookback, price, pctFromHigh, pctFromLow, velocity),
		State: contextState(pctFromHigh, pctFromLow, velocity),
	}}
}

func contextState(pctFromHigh, pctFromLow, velocity float64) string {
	switch {
	case pctFromHigh > -2:
		if velocity > 100 {
			return "At/near range high in parabolic uptrend, price discovery imminent"
		}
		return "At/near range high, breakout or rejection imminent"
	case pctFromHigh > -10:
		if velocity > 50 {
			return "Pullback within strong uptrend, high-probability continuation zone"
		}
		return "Near range high, testing resistance"
	case pctFromLow < 5:
		return "At/near range low, capitulation or accumulation zone"
	case pctFromHigh < -50:
		return "Deep in range from highs, bear market structure or accumulation"
	default:
		return "Mid-range, no edge from positional context alone"
	}
}
