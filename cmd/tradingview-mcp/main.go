// Command tradingview-mcp is a stdio MCP server exposing the
// tradingview-go SDK to any MCP host (Cursor, Claude Desktop, etc.).
//
// The TradingView free public endpoint requires no auth, so this
// binary is configuration-free in the common case. Optional config
// lives at ~/.tradingview-mcp/config.json (or matching env vars) for
// users who want to point at the prodata endpoint with a paid token.
//
//	{
//	  "host":        "prodata.tradingview.com",
//	  "auth_token":  "your-paid-token",
//	  "pool_size":   8,
//	  "enable_cache": false
//	}
//
// Env-var overrides: TRADINGVIEW_HOST, TRADINGVIEW_AUTH_TOKEN,
// TRADINGVIEW_POOL_SIZE, TRADINGVIEW_ENABLE_CACHE.
//
// Add to ~/.cursor/mcp.json:
//
//	{"mcpServers":{"tradingview":{"command":"/Users/you/bin/tradingview-mcp"}}}
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	tradingview "github.com/teslashibe/tradingview-go"
	tvmcp "github.com/teslashibe/tradingview-go/mcp"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type configFile struct {
	Host        string `json:"host,omitempty"`
	AuthToken   string `json:"auth_token,omitempty"`
	PoolSize    int    `json:"pool_size,omitempty"`
	EnableCache bool   `json:"enable_cache,omitempty"`
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tradingview-mcp", "config.json")
}

func loadConfig() (tradingview.Config, error) {
	var cfg configFile

	data, err := os.ReadFile(defaultConfigPath())
	if err != nil && !os.IsNotExist(err) {
		return tradingview.Config{}, fmt.Errorf("read config: %w", err)
	}
	if data != nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return tradingview.Config{}, fmt.Errorf("parse config: %w", err)
		}
	}

	if v := os.Getenv("TRADINGVIEW_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("TRADINGVIEW_AUTH_TOKEN"); v != "" {
		cfg.AuthToken = v
	}
	if v := os.Getenv("TRADINGVIEW_POOL_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.PoolSize = n
		}
	}
	if v := os.Getenv("TRADINGVIEW_ENABLE_CACHE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.EnableCache = b
		}
	}

	out := tradingview.Config{
		Host:        cfg.Host,
		AuthToken:   cfg.AuthToken,
		PoolSize:    cfg.PoolSize,
		EnableCache: cfg.EnableCache,
	}
	return out, nil
}

func main() {
	log.SetOutput(os.Stderr)
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tradingview-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	client := tradingview.New(cfg, nil)
	defer client.Close()

	s := server.NewMCPServer(
		"tradingview-mcp",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	provider := tvmcp.Provider{}
	for _, t := range provider.Tools() {
		t := t
		rawSchema, err := json.Marshal(t.InputSchema)
		if err != nil {
			return fmt.Errorf("marshal schema for %s: %w", t.Name, err)
		}
		tool := mcp.NewToolWithRawSchema(t.Name, t.Description, rawSchema)
		s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			raw, err := json.Marshal(req.Params.Arguments)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result, invokeErr := t.Invoke(ctx, client, raw)
			if invokeErr != nil {
				return mcp.NewToolResultError(invokeErr.Error()), nil
			}
			out, err := json.Marshal(result)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(out)), nil
		})
	}

	return server.ServeStdio(s)
}
