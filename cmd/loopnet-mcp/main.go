// Command loopnet-mcp is a stdio MCP server.
//
// Config: ~/.loopnet-mcp/config.json
//
//	{
//	  "cookie_header": "paste from browser DevTools → Network → any request on www.loopnet.com → Request Headers → cookie",
//	  "user_agent":    "paste the same browser's User-Agent string"
//	}
//
// Env override: LOOPNET_COOKIE_HEADER, LOOPNET_USER_AGENT.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	loopnet "github.com/teslashibe/loopnet-go"
	lnmcp "github.com/teslashibe/loopnet-go/mcp"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type configFile struct {
	CookieHeader string `json:"cookie_header"`
	UserAgent    string `json:"user_agent,omitempty"`
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".loopnet-mcp", "config.json")
}

func loadAuth() (loopnet.Auth, error) {
	var cfg configFile
	data, err := os.ReadFile(defaultConfigPath())
	if err != nil && !os.IsNotExist(err) {
		return loopnet.Auth{}, fmt.Errorf("read config: %w", err)
	}
	if data != nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return loopnet.Auth{}, fmt.Errorf("parse config: %w", err)
		}
	}
	if v := os.Getenv("LOOPNET_COOKIE_HEADER"); v != "" {
		cfg.CookieHeader = v
	}
	if v := os.Getenv("LOOPNET_USER_AGENT"); v != "" {
		cfg.UserAgent = v
	}
	if cfg.CookieHeader == "" {
		return loopnet.Auth{}, fmt.Errorf(
			"loopnet CookieHeader not set. Paste your browser cookie into %s as 'cookie_header'. "+
				"Akamai blocks all programmatic requests without a real browser cookie set.",
			defaultConfigPath())
	}
	return loopnet.Auth{
		CookieHeader: cfg.CookieHeader,
		UserAgent:    cfg.UserAgent,
	}, nil
}

func main() {
	log.SetOutput(os.Stderr)
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "loopnet-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	auth, err := loadAuth()
	if err != nil {
		return err
	}
	client, err := loopnet.New(auth)
	if err != nil {
		return fmt.Errorf("init client: %w", err)
	}
	s := server.NewMCPServer("loopnet-mcp", "0.1.0", server.WithToolCapabilities(true))
	for _, t := range (lnmcp.Provider{}).Tools() {
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
