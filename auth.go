package loopnet

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Login is a no-op confirmation step today. Akamai's challenge cookies
// (_abck, bm_sz, ak_bmsc) cannot be issued to a non-browser caller, so
// "programmatic email/password login" against loopnet.com is not
// achievable from this client. Login() therefore simply validates the
// pasted CookieHeader by calling GetMe — if the dashboard is reachable
// and shows the My LoopNet header, the cookies are good.
func (c *Client) Login(ctx context.Context) (*User, error) {
	return c.GetMe(ctx)
}

var titleRe = regexp.MustCompile(`(?is)<title>\s*(.*?)\s*</title>`)

// GetMe fetches /myloopnet/ and looks for an authenticated header.
// Returns ErrUnauthorized if the dashboard redirects to /account/login
// or comes back without account chrome.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	body, _, err := c.getBytes(ctx, "/myloopnet/", nil)
	if err != nil {
		if errors.Is(err, ErrBotChallenge) {
			return nil, fmt.Errorf("GetMe: %w", err)
		}
		return nil, fmt.Errorf("GetMe: %w", err)
	}
	html := strings.ToLower(string(body))
	loggedIn := strings.Contains(html, "log out") ||
		strings.Contains(html, "sign out") ||
		strings.Contains(html, "myloopnet")

	u := &User{LoggedIn: loggedIn}
	if m := titleRe.FindStringSubmatch(string(body)); len(m) > 1 {
		u.DisplayName = strings.TrimSpace(m[1])
	}
	if !loggedIn {
		return u, fmt.Errorf("%w: /myloopnet/ does not look authenticated", ErrUnauthorized)
	}
	return u, nil
}
