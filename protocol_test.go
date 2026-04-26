package tradingview

import (
	"encoding/json"
	"testing"
)

func TestEncodeFrameRoundTrip(t *testing.T) {
	frame, err := encodeFrame(tvMessage{Method: "set_auth_token", Params: []any{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	parts := decodeFrames(frame)
	if len(parts) != 1 {
		t.Fatalf("want 1 part, got %d (%q)", len(parts), parts)
	}
	var m tvMessage
	if err := json.Unmarshal([]byte(parts[0]), &m); err != nil {
		t.Fatal(err)
	}
	if m.Method != "set_auth_token" {
		t.Errorf("method=%q", m.Method)
	}
}

func TestDecodeFramesBatched(t *testing.T) {
	raw := `~m~10~m~{"a":"b"}~m~10~m~{"c":"d"}`
	parts := decodeFrames(raw)
	if len(parts) != 2 {
		t.Fatalf("want 2, got %d: %v", len(parts), parts)
	}
}

func TestHeartbeatPong(t *testing.T) {
	pong, ok := heartbeatPong(`~m~5~m~~h~42`)
	if !ok {
		t.Fatal("expected pong")
	}
	want := "~m~5~m~~h~42"
	if pong != want {
		t.Errorf("pong=%q want %q", pong, want)
	}
}

func TestNormalizeResolution(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"15", "15", true},
		{"15m", "15", true},
		{"1H", "60", true},
		{"4h", "240", true},
		{"1D", "D", true},
		{"D", "D", true},
		{"daily", "D", true},
		{"weekly", "W", true},
		{"monthly", "M", true},
		{"", "", false},
		{"7", "", false},
		{"banana", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeResolution(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeResolution(%q) = (%q,%v) want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNormalizeSymbol(t *testing.T) {
	if got := NormalizeSymbol("  binance:btcusdt  "); got != "BINANCE:BTCUSDT" {
		t.Errorf("got %q", got)
	}
	if got := NormalizeSymbol("eth"); got != "ETH" {
		t.Errorf("got %q", got)
	}
}

func TestParseCandles(t *testing.T) {
	payload := map[string]any{
		"sds_1": map[string]any{
			"s": []any{
				map[string]any{"v": []any{1.7e9, 100.0, 110.0, 95.0, 105.0, 1234.5}},
				map[string]any{"v": []any{1.7e9 + 60, 105.0, 115.0, 100.0, 112.0, 4321.0}},
			},
		},
	}
	got := parseCandles(payload)
	if len(got) != 2 {
		t.Fatalf("got %d candles", len(got))
	}
	if got[0].Open != 100.0 || got[0].Close != 105.0 || got[1].Volume != 4321.0 {
		t.Errorf("unexpected candles: %+v", got)
	}
}
