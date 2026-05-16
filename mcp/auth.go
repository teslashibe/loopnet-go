package mcp

import (
	"context"

	loopnet "github.com/teslashibe/loopnet-go"
	"github.com/teslashibe/mcptool"
)

// LoginInput is the typed input for loopnet_login.
type LoginInput struct{}

func login(ctx context.Context, c *loopnet.Client, _ LoginInput) (any, error) {
	return c.Login(ctx)
}

// GetMeInput is the typed input for loopnet_get_me.
type GetMeInput struct{}

func getMe(ctx context.Context, c *loopnet.Client, _ GetMeInput) (any, error) {
	return c.GetMe(ctx)
}

var authTools = []mcptool.Tool{
	mcptool.Define[*loopnet.Client, LoginInput](
		"loopnet_login",
		"Validate the pasted browser CookieHeader by fetching /myloopnet/ and confirming the dashboard renders.",
		"Login",
		login,
	),
	mcptool.Define[*loopnet.Client, GetMeInput](
		"loopnet_get_me",
		"Confirm the cached session is alive and return the dashboard title from /myloopnet/.",
		"GetMe",
		getMe,
	),
}
