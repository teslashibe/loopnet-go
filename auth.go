package loopnet

import (
	"context"
	"fmt"
	"net/url"
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

// accountPath is the authenticated account dashboard. LoopNet retired the
// old /myloopnet/ path (it now 404s); /account/ is the live equivalent.
const accountPath = "/account/"

// GetMe fetches the account dashboard and confirms it renders the signed-in
// user's data. LoopNet has no JSON /me endpoint, and the /account/ page is
// tricky: even when logged OUT it still embeds a hidden "Log in to access
// your VIP LoopNet" dialog, so a naive "log in" substring check yields false
// negatives. Instead we look for the session's own identity echoed in the
// page — the AssociateID (from the UserInfo_AssociateID cookie) and/or the
// account email (LastUserEmail cookie) only appear when authenticated.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	body, _, err := c.getBytes(ctx, accountPath, nil)
	if err != nil {
		return nil, fmt.Errorf("GetMe: %w", err)
	}
	html := string(body)

	c.authMu.RLock()
	cookie := c.auth.CookieHeader
	c.authMu.RUnlock()

	associateID := cookieValue(cookie, "UserInfo_AssociateID")
	email := decodeCookie(cookieValue(cookie, "LastUserEmail"))

	loggedIn := false
	switch {
	case associateID != "" && strings.Contains(html, associateID):
		loggedIn = true
	case email != "" && strings.Contains(strings.ToLower(html), strings.ToLower(email)):
		loggedIn = true
	}

	u := &User{LoggedIn: loggedIn, DisplayName: email}
	if !loggedIn {
		return u, fmt.Errorf("%w: %s does not show a signed-in session", ErrUnauthorized, accountPath)
	}
	return u, nil
}

// cookieValue pulls a single cookie's value out of a "k=v; k=v" header.
func cookieValue(header, name string) string {
	for _, part := range strings.Split(header, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] == name {
			return kv[1]
		}
	}
	return ""
}

// decodeCookie percent-decodes a cookie value (LastUserEmail is URL-encoded,
// e.g. margie%40pardeeproperties.com). Falls back to the raw value on error.
func decodeCookie(v string) string {
	if v == "" {
		return ""
	}
	if dec, err := url.QueryUnescape(v); err == nil {
		return dec
	}
	return v
}
