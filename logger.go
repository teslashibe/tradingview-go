package tradingview

// Logger is a tiny structured-logger interface. Pass nil to New and
// you'll get NoopLogger; bring your own (zap, slog, logrus) by
// implementing the four methods.
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// NoopLogger drops every message.
type NoopLogger struct{}

func (NoopLogger) Debugf(string, ...any) {}
func (NoopLogger) Infof(string, ...any)  {}
func (NoopLogger) Warnf(string, ...any)  {}
func (NoopLogger) Errorf(string, ...any) {}
