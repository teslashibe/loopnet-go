// Package mcp exposes loopnet-go as a set of MCP tools.
package mcp

import "github.com/teslashibe/mcptool"

// Provider implements [mcptool.Provider] for loopnet-go.
type Provider struct{}

// Platform returns "loopnet".
func (Provider) Platform() string { return "loopnet" }

// Tools returns every MCP tool, in registration order.
func (Provider) Tools() []mcptool.Tool {
	out := make([]mcptool.Tool, 0, len(authTools)+len(serviceTools)+len(listingTools)+len(pageTools))
	out = append(out, authTools...)
	out = append(out, serviceTools...)
	out = append(out, listingTools...)
	out = append(out, pageTools...)
	return out
}
