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
// Useful paths (READ-ONLY, your-account-only):
//
//   - /myloopnet/                      — dashboard
//   - /myloopnet/saved-searches/       — saved searches
//   - /myloopnet/saved-listings/       — saved listings
//   - /myloopnet/saved-comparables/    — saved comps
//   - /myloopnet/dashboard/activity    — recent activity
//   - /myloopnet/messages              — inbox
//   - /myloopnet/profile/              — profile
//   - /account/preferences/            — account settings
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
