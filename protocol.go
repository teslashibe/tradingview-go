package tradingview

// Protocol-level helpers. Adapted from gopher-lab/tradingview-go (MIT)
// — see NOTICE. The TradingView WebSocket protocol is not officially
// documented; this file captures the bits we need: SockJS-style
// length-prefix framing, JSON RPC envelope, candle parsing, and
// resolution / symbol normalization.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var frameRegex = regexp.MustCompile(`~m~\d+~m~([^~]+|~h~\d+)`)

// encodeFrame wraps a JSON message in TradingView's length-prefix frame.
func encodeFrame(msg tvMessage) (string, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("~m~%d~m~%s", len(body), body), nil
}

// decodeFrames splits a raw WS message into its JSON / heartbeat parts.
// TradingView batches multiple inner messages into one outer frame.
func decodeFrames(raw string) []string {
	matches := frameRegex.FindAllStringSubmatch(raw, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}
	return out
}

// heartbeatPong builds a ~h~N pong frame matching the server's ping.
func heartbeatPong(raw string) (string, bool) {
	const tag = "~h~"
	i := strings.Index(raw, tag)
	if i < 0 {
		return "", false
	}
	rest := raw[i+len(tag):]
	end := strings.IndexByte(rest, '~')
	if end < 0 {
		end = len(rest)
	}
	num := rest[:end]
	body := tag + num
	return fmt.Sprintf("~m~%d~m~%s", len(body), body), true
}

// parseCandles pulls OHLCV bars out of a "du" or "timescale_update" payload.
// The TradingView shape is:
//
//	{ "sds_1": { "s": [ { "v": [ts, o, h, l, c, v] }, ... ] } }
func parseCandles(payload map[string]any) []Candle {
	var out []Candle
	for _, v := range payload {
		series, ok := v.(map[string]any)
		if !ok {
			continue
		}
		bars, ok := series["s"].([]any)
		if !ok {
			continue
		}
		for _, item := range bars {
			bar, ok := item.(map[string]any)
			if !ok {
				continue
			}
			values, ok := bar["v"].([]any)
			if !ok || len(values) < 6 {
				continue
			}
			out = append(out, Candle{
				Timestamp: int64(asFloat(values[0])),
				Open:      asFloat(values[1]),
				High:      asFloat(values[2]),
				Low:       asFloat(values[3]),
				Close:     asFloat(values[4]),
				Volume:    asFloat(values[5]),
			})
		}
	}
	return out
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

// ValidResolutions is the set the upstream protocol accepts.
var ValidResolutions = map[string]struct{}{
	"1": {}, "3": {}, "5": {}, "15": {}, "30": {},
	"45": {}, "60": {}, "120": {}, "180": {}, "240": {},
	"D": {}, "1D": {}, "W": {}, "1W": {}, "M": {}, "1M": {},
}

// friendlyToWire translates user-friendly aliases ("15m", "1h", "1d")
// to the TradingView wire format.
var friendlyToWire = map[string]string{
	"1M": "1", "1MIN": "1",
	"3M": "3", "3MIN": "3",
	"5M": "5", "5MIN": "5",
	"15M": "15", "15MIN": "15",
	"30M": "30", "30MIN": "30",
	"45M": "45", "45MIN": "45",
	"1H": "60", "60M": "60",
	"2H": "120",
	"3H": "180",
	"4H": "240",
	"1D": "D", "DAILY": "D",
	"1W": "W", "WEEKLY": "W",
	"1MN": "M", "MONTHLY": "M",
}

// NormalizeResolution canonicalizes a user-supplied resolution to the
// TradingView wire form, returning ("", false) if the input is unknown.
func NormalizeResolution(r string) (string, bool) {
	r = strings.ToUpper(strings.TrimSpace(r))
	if r == "" {
		return "", false
	}
	if mapped, ok := friendlyToWire[r]; ok {
		return mapped, true
	}
	if _, ok := ValidResolutions[r]; ok {
		return r, true
	}
	return "", false
}

// NormalizeSymbol trims and uppercases. It does not invent an exchange
// prefix — pass-through is the contract; use Client.SearchSymbols to
// discover canonical "EXCHANGE:SYMBOL" forms.
func NormalizeSymbol(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

var sessionCounter atomic.Int64

// genSessionID returns a unique per-process session id with the given
// prefix ("cs", "qs", "sds").
func genSessionID(prefix string) string {
	n := sessionCounter.Add(1)
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), n)
}
