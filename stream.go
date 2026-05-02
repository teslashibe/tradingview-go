package tradingview

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Streamer struct {
	cfg    Config
	log    Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewStreamer(log Logger, cfg Config) *Streamer {
	cfg = cfg.withDefaults()
	if log == nil {
		log = NoopLogger{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Streamer{cfg: cfg, log: log, ctx: ctx, cancel: cancel}
}

func (s *Streamer) Subscribe(symbol, resolution string, onCandle func(Candle)) error {
	sym := NormalizeSymbol(symbol)
	res, ok := NormalizeResolution(resolution)
	if sym == "" {
		return newErr(CodeSymbolUnknown, "symbol is empty", nil)
	}
	if !ok {
		return newErr(CodeInvalidResolution, "unknown resolution: "+resolution, nil)
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for s.ctx.Err() == nil {
			if err := s.stream(s.ctx, sym, res, onCandle); err != nil && s.ctx.Err() == nil {
				s.log.Warnf("tradingview stream reconnecting %s/%s: %v", sym, res, err)
				select {
				case <-s.ctx.Done():
				case <-time.After(3 * time.Second):
				}
			}
		}
	}()
	return nil
}

func (s *Streamer) Close() {
	s.cancel()
	s.wg.Wait()
}

func (s *Streamer) stream(ctx context.Context, symbol, resolution string, onCandle func(Candle)) error {
	c, err := dial(ctx, s.cfg, s.log)
	if err != nil {
		return err
	}
	defer c.close()

	chartID := genSessionID("cs")
	seriesID := genSessionID("sds")
	if err := c.send(tvMessage{Method: "chart_create_session", Params: []any{chartID, ""}}); err != nil {
		return err
	}
	if err := c.send(tvMessage{
		Method: "resolve_symbol",
		Params: []any{chartID, "sds_sym_1", fmt.Sprintf(`={"symbol":"%s","adjustment":"splits"}`, symbol)},
	}); err != nil {
		return err
	}
	if err := c.send(tvMessage{
		Method: "create_series",
		Params: []any{chartID, seriesID, "sds_1", "sds_sym_1", resolution, s.cfg.CandleHistory, ""},
	}); err != nil {
		return err
	}
	s.log.Infof("Connected to %s/%s", symbol, resolution)

	for ctx.Err() == nil {
		_ = c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			return err
		}
		raw := string(msg)
		if pong, ok := heartbeatPong(raw); ok {
			c.writeMu.Lock()
			err := c.ws.WriteMessage(1, []byte(pong))
			c.writeMu.Unlock()
			if err != nil {
				return err
			}
			continue
		}
		for _, frame := range decodeFrames(raw) {
			var env map[string]any
			if err := json.Unmarshal([]byte(frame), &env); err != nil {
				continue
			}
			method, _ := env["m"].(string)
			if method != "du" && method != "timescale_update" {
				continue
			}
			params, _ := env["p"].([]any)
			if len(params) < 2 {
				continue
			}
			payload, _ := params[1].(map[string]any)
			candles := parseCandles(payload)
			s.log.Debugf("TradingView parsed candles=%d", len(candles))
			for _, candle := range candles {
				if onCandle != nil {
					onCandle(candle)
				}
			}
		}
	}
	return ctx.Err()
}
