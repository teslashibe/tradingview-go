package tradingview

import "testing"

func TestStripEm(t *testing.T) {
	if got := stripEm("<em>BTC</em>USD"); got != "BTCUSD" {
		t.Errorf("got %q", got)
	}
	if got := stripEm("plain"); got != "plain" {
		t.Errorf("got %q", got)
	}
}

func TestSymbolMatchFull(t *testing.T) {
	cases := []struct {
		m    SymbolMatch
		want string
	}{
		{SymbolMatch{Symbol: "BTCUSDT", Prefix: "BINANCE"}, "BINANCE:BTCUSDT"},
		{SymbolMatch{Symbol: "AAPL"}, "AAPL"},
	}
	for _, c := range cases {
		if got := c.m.Full(); got != c.want {
			t.Errorf("Full()=%q want %q", got, c.want)
		}
	}
}
