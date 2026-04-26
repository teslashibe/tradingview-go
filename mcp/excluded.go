package tradingviewmcp

// Excluded lists *tradingview.Client methods intentionally not exposed
// as MCP tools. The value is a one-line human reason the coverage
// test in mcp_test.go reports when a new method is added without
// either a tool wrapping it or an entry here.
//
// Keep this list tight: when an exclusion is no longer justified, add
// a tool and delete the entry.
var Excluded = map[string]string{
	"Close": "lifecycle owned by the host process; not an agent action",
}
