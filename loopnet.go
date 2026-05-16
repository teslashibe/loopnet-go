// Package loopnet is a Go client + MCP tool surface for loopnet.com
// (CoStar Group). There is no public end-user API.
//
// Bot protection
//
// loopnet.com sits behind Akamai Bot Manager. Plain server-side
// requests return HTTP 403 (AkamaiGHost) on every path, including
// /account/login/ and /. The only practical authentication path is
// to paste a complete browser Cookie header — including the Akamai
// challenge cookies (_abck, bm_sz, ak_bmsc) AND the LoopNet auth
// cookies set after login — captured from a real Chrome session.
//
// Operational note
//
// CoStar (LoopNet's owner) actively monitors and litigates automated
// access to its properties. This client is intentionally read-only,
// rate-limited (default 1 req / 1.5s), and designed strictly for
// programmatic use of the authenticated user's own account data.
// Do NOT enable concurrent fan-out or wide-area crawling against it.
package loopnet

import (
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

const (
	baseURL          = "https://www.loopnet.com"
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	defaultRetries   = 2
	defaultRetryBase = 750 * time.Millisecond
	// Deliberately conservative — Akamai will tighten the screws on
	// bursty traffic from a single cookie set.
	defaultMinGap = 1500 * time.Millisecond
)

// Client talks to loopnet.com via a pasted browser cookie header.
type Client struct {
	auth       Auth
	httpClient *http.Client
	userAgent  string
	maxRetries int
	retryBase  time.Duration
	minGap     time.Duration

	gapMu     sync.Mutex
	lastReqAt time.Time

	authMu sync.RWMutex
}

// Option configures a Client.
type Option func(*Client)

// WithUserAgent overrides the default browser User-Agent. Must match
// the browser session you grabbed the cookies from — Akamai
// fingerprints UA + cookie together.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

// WithRetry sets retry policy.
func WithRetry(maxRetries int, base time.Duration) Option {
	return func(c *Client) {
		c.maxRetries = maxRetries
		c.retryBase = base
	}
}

// WithHTTPClient overrides the default http.Client. Nil is ignored.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithMinRequestGap sets the minimum gap between consecutive requests.
// The default of 1500ms is intentionally conservative for Akamai.
func WithMinRequestGap(d time.Duration) Option {
	return func(c *Client) { c.minGap = d }
}

// New constructs a Client. Auth.CookieHeader is required.
func New(auth Auth, opts ...Option) (*Client, error) {
	if auth.CookieHeader == "" {
		return nil, ErrInvalidAuth
	}
	jar, _ := cookiejar.New(nil)
	c := &Client{
		auth:       auth,
		httpClient: &http.Client{Timeout: 30 * time.Second, Jar: jar},
		userAgent:  defaultUserAgent,
		maxRetries: defaultRetries,
		retryBase:  defaultRetryBase,
		minGap:     defaultMinGap,
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// AuthSnapshot returns the cached auth credentials.
func (c *Client) AuthSnapshot() Auth {
	c.authMu.RLock()
	defer c.authMu.RUnlock()
	return c.auth
}
