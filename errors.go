package tradingview

import "fmt"

// Error code constants. Stable strings so callers (e.g. the MCP
// adapter) can switch on them.
const (
	CodeSymbolUnknown     = "symbol_unknown"
	CodeInvalidResolution = "invalid_resolution"
	CodeInvalidBars       = "invalid_bars"
	CodeUpstreamTimeout   = "upstream_timeout"
	CodeUpstreamProtocol  = "upstream_protocol"
	CodeUpstreamHTTP      = "upstream_http"
	CodeRateLimited       = "rate_limited"
	CodeClosed            = "closed"
)

// Error is the structured error type returned by every Client method.
// Match on Code; Message is human-readable; Cause wraps the underlying
// error (use errors.Is / errors.As).
type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("tradingview: %s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("tradingview: %s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

func newErr(code, msg string, cause error) *Error {
	return &Error{Code: code, Message: msg, Cause: cause}
}
