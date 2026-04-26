package tradingviewmcp

import (
	"errors"

	"github.com/teslashibe/mcptool"
	tradingview "github.com/teslashibe/tradingview-go"
)

// wrapErr converts a tradingview-package error into a structured
// mcptool.Error using the SDK's stable Code values. Unknown errors
// fall through so the host harness reports them as internal_error.
func wrapErr(err error, op string) error {
	if err == nil {
		return nil
	}
	var tvErr *tradingview.Error
	if errors.As(err, &tvErr) {
		return &mcptool.Error{
			Code:    tvErr.Code,
			Message: op + ": " + tvErr.Message,
		}
	}
	return err
}

func defaultInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}
