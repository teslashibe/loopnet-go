package mcp

import (
	"context"

	loopnet "github.com/teslashibe/loopnet-go"
	"github.com/teslashibe/mcptool"
)

// GetPathInput is the typed input for loopnet_get_path.
type GetPathInput struct {
	Path string `json:"path" jsonschema:"description=Path under www.loopnet.com (e.g. /account/). Retired /myloopnet/ is remapped to /account/.,required"`
}

func getPath(ctx context.Context, c *loopnet.Client, in GetPathInput) (any, error) {
	return c.GetPath(ctx, in.Path)
}

var pageTools = []mcptool.Tool{
	mcptool.Define[*loopnet.Client, GetPathInput](
		"loopnet_get_path",
		"Fetch raw HTML for an authenticated path under www.loopnet.com. Read-only; intended for your-account dashboards only.",
		"GetPath",
		getPath,
	),
}
