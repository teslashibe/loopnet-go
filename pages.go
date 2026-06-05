package loopnet

import (
	"context"
	"regexp"
	"strings"
)

var titleRe = regexp.MustCompile(`(?is)<title>\s*(.*?)\s*</title>`)

// GetPath fetches an arbitrary authenticated path under www.loopnet.com
// and returns the raw HTML response.
//
// Confirmed-live paths (READ-ONLY, your-account-only), verified against a
// real authenticated account (teslashibe/smore#137):
//
//   - /account/    — account dashboard (HTTP 200)
//   - /myloopnet/  — account dashboard (HTTP 200; serves the same dashboard
//     as /account/). NOTE: /myloopnet/ is NOT retired — the original bug
//     report's premise was wrong; both roots return 200.
//
// What actually 404s: the deep /myloopnet/saved-searches/,
// /myloopnet/saved-listings/ (and the /account/ equivalents) sub-paths.
// These do not exist as server-rendered pages — LoopNet renders saved
// searches/listings client-side in its SPA, so there is no fetchable HTML
// URL for them. get_path forwards the caller's path verbatim; a 404 from
// LoopNet surfaces as ErrNotFound.
//
// DO NOT use this for wide-area listing search scraping — see package
// doc for the CoStar operational warning.
func (c *Client) GetPath(ctx context.Context, path string) (*PageResult, error) {
	body, status, err := c.getBytes(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	p := &PageResult{
		URL:          baseURL + path,
		StatusCode:   status,
		ContentBytes: len(body),
		HTML:         string(body),
	}
	if m := titleRe.FindStringSubmatch(p.HTML); len(m) > 1 {
		p.Title = strings.TrimSpace(m[1])
	}
	return p, nil
}
