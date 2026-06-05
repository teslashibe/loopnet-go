package loopnet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// isAkamaiInterstitial reports whether a 200 body is an Akamai Bot Manager
// active-challenge page rather than real content. These are short and carry
// the sec-if-cpt-container scaffold / "protected by Akamai" marker.
func isAkamaiInterstitial(body []byte) bool {
	if len(body) > 12000 {
		return false // real pages are far larger; cheap short-circuit
	}
	return bytes.Contains(body, []byte("sec-if-cpt-container")) ||
		bytes.Contains(body, []byte("sec-bc-tile-container")) ||
		bytes.Contains(body, []byte("protected by</p>")) ||
		bytes.Contains(body, []byte("scf-akamai-logo"))
}

// LoopNet's listing-search (/search/) and listing-detail (/Listing/) pages
// sit behind Akamai Bot Manager's strict ruleset. Plain net/http (and even a
// real headless browser) is rejected with HTTP 403 "Access Denied" — Akamai
// fingerprints the TLS ClientHello + HTTP/2 settings + header order, none of
// which stdlib net/http reproduces. (The lighter /account/ + /services/*
// surface in client.go is NOT subject to this, which is why those use the
// stdlib transport.)
//
// The fix is a TLS/HTTP-2 fingerprint that matches a real Chrome. We use
// bogdanfinn/tls-client with the Chrome_146 profile plus the exact Chrome
// header set in Chrome's header order. Verified against a live session: this
// returns 200 with the full SSR HTML where stdlib/curl/curl-impersonate with a
// lesser profile got 403.
//
// Cookies still matter: the strict paths require a valid Akamai sensor cookie
// set (the bm_* / trust family) minted by a real browser running Akamai's JS.
// Those arrive via the pasted/synced CookieHeader (cookie-sync keeps them
// fresh from the user's real browser). When they go stale the server answers
// 403; we surface that as ErrBotChallenge so the caller can prompt a refresh.

// strictClientProfile is the Chrome version whose JA3/HTTP2 fingerprint we
// present. Keep this close to the UA's Chrome major version.
var strictClientProfile = profiles.Chrome_146

type strictFetcher struct {
	once   sync.Once
	client tls_client.HttpClient
	err    error
}

func (c *Client) initStrict() {
	c.strict.once.Do(func() {
		c.strict.client, c.strict.err = tls_client.NewHttpClient(
			tls_client.NewNoopLogger(),
			tls_client.WithClientProfile(strictClientProfile),
			tls_client.WithCookieJar(tls_client.NewCookieJar()),
			tls_client.WithTimeoutSeconds(45),
		)
	})
}

// fetchStrict GETs an Akamai-protected path on www.loopnet.com using the
// Chrome-fingerprinted transport and returns the body + status. It applies the
// same inter-request gap as the stdlib transport and maps the Akamai 403
// "Access Denied" body to ErrBotChallenge.
func (c *Client) fetchStrict(ctx context.Context, rawURL string) ([]byte, int, error) {
	c.initStrict()
	if c.strict.err != nil {
		return nil, 0, fmt.Errorf("%w: tls-client init: %v", ErrRequestFailed, c.strict.err)
	}
	c.waitForGap(ctx)
	if ctx.Err() != nil {
		return nil, 0, ctx.Err()
	}

	c.authMu.RLock()
	ua := c.userAgent
	if c.auth.UserAgent != "" {
		ua = c.auth.UserAgent
	}
	cookie := c.auth.CookieHeader
	c.authMu.RUnlock()

	req, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	// Full Chrome request-header set in Chrome's header order. Akamai checks
	// both presence and order; a minimal set is rejected even with a correct
	// TLS profile.
	req.Header = fhttp.Header{
		"sec-ch-ua":                 {`"Google Chrome";v="147", "Not.A/Brand";v="8", "Chromium";v="147"`},
		"sec-ch-ua-mobile":          {"?0"},
		"sec-ch-ua-platform":        {`"macOS"`},
		"upgrade-insecure-requests": {"1"},
		"user-agent":                {ua},
		"accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"sec-fetch-site":            {"same-origin"},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-user":            {"?1"},
		"sec-fetch-dest":            {"document"},
		"referer":                   {baseURL + "/account/"},
		"accept-language":           {"en-US,en;q=0.9"},
		"cookie":                    {cookie},
		fhttp.HeaderOrderKey: {
			"sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "upgrade-insecure-requests",
			"user-agent", "accept", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-user",
			"sec-fetch-dest", "referer", "accept-language", "cookie",
		},
	}

	resp, err := c.strict.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%w: reading body: %v", ErrRequestFailed, err)
	}

	switch resp.StatusCode {
	case fhttp.StatusOK, fhttp.StatusNotModified:
		// Akamai sometimes answers 200 with an *active JS challenge*
		// interstitial (a tiny "Powered and protected by Akamai" /
		// sec-if-cpt-container page that runs sensor JS) instead of the real
		// content. Only a JS-executing browser can solve it; a stale sensor
		// cookie set triggers it. Detect and surface as ErrBotChallenge so we
		// don't try to parse the stub as a listing.
		if isAkamaiInterstitial(body) {
			return body, resp.StatusCode, fmt.Errorf("%w: Akamai JS-challenge interstitial on %s (sensor cookies need a browser refresh via cookie-sync)", ErrBotChallenge, rawURL)
		}
		return body, resp.StatusCode, nil
	case fhttp.StatusForbidden:
		// Akamai "Access Denied" (edgesuite) — stale/missing sensor cookies.
		return body, resp.StatusCode, fmt.Errorf("%w: Akamai 403 on %s (refresh the browser session via cookie-sync to re-mint bm_* sensor cookies)", ErrBotChallenge, rawURL)
	case fhttp.StatusUnauthorized:
		return body, resp.StatusCode, ErrUnauthorized
	case fhttp.StatusNotFound:
		return body, resp.StatusCode, ErrNotFound
	default:
		return body, resp.StatusCode, &HTTPError{StatusCode: resp.StatusCode, Body: truncate(string(body), 256)}
	}
}
