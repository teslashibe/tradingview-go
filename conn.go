package tradingview

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// conn is one authenticated TradingView WebSocket connection. The
// pool reuses these across fetches; each fetch creates its own chart
// session on the conn so series IDs never collide.
//
// conn is intentionally not safe for concurrent fetches — the pool
// guarantees a single fetch at a time per conn.
type conn struct {
	ws  *websocket.Conn
	cfg Config
	log Logger

	// writeMu guards ws writes, which can race with the read loop's
	// pong-on-heartbeat. Reads from one fetch at a time, so no read mu.
	writeMu sync.Mutex
}

// dial opens a new WebSocket and completes the auth handshake. It
// retries up to dialMaxAttempts on transient handshake failures —
// TradingView occasionally drops a connection when many handshakes
// land in the same millisecond, e.g. during a FetchMulti burst.
func dial(ctx context.Context, cfg Config, log Logger) (*conn, error) {
	const dialMaxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < dialMaxAttempts; attempt++ {
		if attempt > 0 {
			// Small jittered backoff so a re-dial does not collide
			// with the same wave of simultaneous attempts.
			delay := time.Duration(50+rand.Intn(200)) * time.Millisecond * time.Duration(attempt)
			select {
			case <-ctx.Done():
				return nil, newErr(CodeUpstreamTimeout, "dial canceled", ctx.Err())
			case <-time.After(delay):
			}
		}
		c, err := dialOnce(ctx, cfg, log)
		if err == nil {
			return c, nil
		}
		lastErr = err
		log.Debugf("tradingview: dial attempt %d/%d failed: %v", attempt+1, dialMaxAttempts, err)
	}
	return nil, lastErr
}

func dialOnce(ctx context.Context, cfg Config, log Logger) (*conn, error) {
	u := url.URL{
		Scheme:   "wss",
		Host:     cfg.Host,
		Path:     "/socket.io/websocket",
		RawQuery: "type=chart",
	}
	d := websocket.Dialer{HandshakeTimeout: cfg.HandshakeTimeout}
	hdr := http.Header{
		"Origin":     {"https://www.tradingview.com"},
		"User-Agent": {cfg.UserAgent},
	}
	ws, _, err := d.DialContext(ctx, u.String(), hdr)
	if err != nil {
		return nil, newErr(CodeUpstreamHTTP, "websocket dial failed", err)
	}
	c := &conn{ws: ws, cfg: cfg, log: log}
	if err := c.authenticate(ctx); err != nil {
		_ = ws.Close()
		return nil, err
	}
	return c, nil
}

// authenticate consumes the server hello and sends set_auth_token.
// Chart sessions are created per-fetch, not here, so the conn is
// reusable across symbols.
func (c *conn) authenticate(ctx context.Context) error {
	if dl, ok := ctx.Deadline(); ok {
		_ = c.ws.SetReadDeadline(dl)
	} else {
		_ = c.ws.SetReadDeadline(time.Now().Add(c.cfg.HandshakeTimeout))
	}
	_, hello, err := c.ws.ReadMessage()
	if err != nil {
		return newErr(CodeUpstreamProtocol, "read hello", err)
	}
	if !strings.Contains(string(hello), "~m~") {
		return newErr(CodeUpstreamProtocol, fmt.Sprintf("unexpected hello: %.80s", hello), nil)
	}
	if err := c.send(tvMessage{
		Method: "set_auth_token",
		Params: []any{c.cfg.AuthToken},
	}); err != nil {
		return newErr(CodeUpstreamProtocol, "send auth", err)
	}
	return nil
}

// fetchSeries opens a fresh chart session, requests bars, drains until
// candles arrive, and tears the session down. Caller must hold the
// pool's exclusive lease on this conn.
func (c *conn) fetchSeries(ctx context.Context, symbol, resolution string, bars int) ([]Candle, error) {
	chartID := genSessionID("cs")
	seriesID := genSessionID("sds")

	if err := c.send(tvMessage{Method: "chart_create_session", Params: []any{chartID, ""}}); err != nil {
		return nil, newErr(CodeUpstreamProtocol, "create chart session", err)
	}
	defer func() {
		// Best-effort cleanup; if the conn is broken the pool will
		// drop it on next acquire.
		_ = c.send(tvMessage{Method: "chart_delete_session", Params: []any{chartID}})
	}()

	if err := c.send(tvMessage{
		Method: "resolve_symbol",
		Params: []any{
			chartID,
			"sds_sym_1",
			fmt.Sprintf(`={"symbol":"%s","adjustment":"splits"}`, symbol),
		},
	}); err != nil {
		return nil, newErr(CodeUpstreamProtocol, "resolve symbol", err)
	}

	if err := c.send(tvMessage{
		Method: "create_series",
		Params: []any{chartID, seriesID, "sds_1", "sds_sym_1", resolution, bars, ""},
	}); err != nil {
		return nil, newErr(CodeUpstreamProtocol, "create series", err)
	}

	deadline := time.Now().Add(c.cfg.FetchTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, newErr(CodeUpstreamTimeout, "context canceled", ctx.Err())
		default:
		}

		_ = c.ws.SetReadDeadline(deadline)
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			return nil, newErr(CodeUpstreamProtocol, "read", err)
		}
		raw := string(msg)

		if pong, ok := heartbeatPong(raw); ok {
			c.writeMu.Lock()
			err := c.ws.WriteMessage(websocket.TextMessage, []byte(pong))
			c.writeMu.Unlock()
			if err != nil {
				return nil, newErr(CodeUpstreamProtocol, "pong", err)
			}
			continue
		}

		for _, frame := range decodeFrames(raw) {
			var env map[string]any
			if err := json.Unmarshal([]byte(frame), &env); err != nil {
				continue
			}
			method, _ := env["m"].(string)
			if method == "symbol_error" || method == "series_error" || method == "critical_error" {
				return nil, newErr(CodeSymbolUnknown, fmt.Sprintf("%s: %s", method, truncate(frame, 200)), nil)
			}
			if method != "du" && method != "timescale_update" {
				continue
			}
			params, _ := env["p"].([]any)
			if len(params) < 2 {
				continue
			}
			payload, _ := params[1].(map[string]any)
			if candles := parseCandles(payload); len(candles) > 0 {
				return candles, nil
			}
		}
	}
	return nil, newErr(CodeUpstreamTimeout, "no candle data within fetch timeout", nil)
}

// send is the only goroutine-safe way to write a frame.
func (c *conn) send(msg tvMessage) error {
	frame, err := encodeFrame(msg)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteMessage(websocket.TextMessage, []byte(frame))
}

func (c *conn) close() error {
	if c == nil || c.ws == nil {
		return nil
	}
	return c.ws.Close()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
