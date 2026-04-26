package tradingviewmcp_test

import (
	"reflect"
	"testing"

	"github.com/teslashibe/mcptool"
	tradingview "github.com/teslashibe/tradingview-go"
	tradingviewmcp "github.com/teslashibe/tradingview-go/mcp"
)

// TestEveryClientMethodIsWrappedOrExcluded is the canonical drift
// detector: every exported method on *tradingview.Client must either
// be wrapped by a Tool or listed in Excluded with a reason. Adding a
// new method without either breaks CI.
func TestEveryClientMethodIsWrappedOrExcluded(t *testing.T) {
	rep := mcptool.Coverage(
		reflect.TypeOf(&tradingview.Client{}),
		tradingviewmcp.Provider{}.Tools(),
		tradingviewmcp.Excluded,
	)
	if len(rep.Missing) > 0 {
		t.Fatalf("client methods missing MCP exposure (add a tool or list in excluded.go): %v", rep.Missing)
	}
	if len(rep.UnknownExclusions) > 0 {
		t.Fatalf("excluded.go references methods that don't exist on *Client: %v", rep.UnknownExclusions)
	}
}

// TestToolsValidate enforces naming + schema + description rules on
// every tool. Same ValidateTools the host harness runs at boot.
func TestToolsValidate(t *testing.T) {
	if err := mcptool.ValidateTools(tradingviewmcp.Provider{}.Tools()); err != nil {
		t.Fatal(err)
	}
}

// TestProviderPlatform locks the platform identifier. Changing it
// would break every host that stores credentials or configuration
// keyed by platform id.
func TestProviderPlatform(t *testing.T) {
	if got := (tradingviewmcp.Provider{}).Platform(); got != "tradingview" {
		t.Fatalf("Platform() = %q, want %q", got, "tradingview")
	}
}
