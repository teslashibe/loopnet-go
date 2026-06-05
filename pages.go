package loopnet

import (
	"context"
	"regexp"
	"strings"
)

var titleRe = regexp.MustCompile(`(?is)<title>\s*(.*?)\s*</title>`)

// retiredPathAliases maps LoopNet paths that have been retired (they now
// return HTTP 404) to their confirmed-live replacement. Only the retired
// /myloopnet/ dashboard root is confirmed here: it maps to /account/, the
// live dashboard the rest of this client already uses (see auth.go).
//
// The modern equivalents of the old /myloopnet/saved-searches/ and
// /myloopnet/saved-listings/ sub-pages are NOT yet confirmed and so are
// deliberately omitted — they need a live audit against a real
// authenticated LoopNet account before being added here or advertised as
// example paths.
var retiredPathAliases = map[string]string{
	"/myloopnet":  accountPath,
	"/myloopnet/": accountPath,
}

// normalizePath rewrites known retired paths to their live equivalent so a
// caller using a documented-but-retired path still reaches a valid page.
// Unknown paths are returned unchanged. Read-only; affects URL only.
func normalizePath(path string) string {
	if alias, ok := retiredPathAliases[strings.TrimRight(path, "/")]; ok {
		return alias
	}
	if alias, ok := retiredPathAliases[path]; ok {
		return alias
	}
	return path
}

// GetPath fetches an arbitrary authenticated path under www.loopnet.com
// and returns the raw HTML response.
//
// Confirmed-live paths (READ-ONLY, your-account-only):
//
//   - /account/             — account dashboard (the live replacement for
//     the retired /myloopnet/ root; see auth.go)
//
// NEEDS LIVE AUDIT: LoopNet retired the /myloopnet/... tree (it 404s). The
// modern equivalents of saved-searches/saved-listings/saved-comparables
// must be confirmed against a real authenticated account before being
// advertised here. The known retired /myloopnet/ root is normalized to
// /account/ via normalizePath.
//
// DO NOT use this for wide-area listing search scraping — see package
// doc for the CoStar operational warning.
func (c *Client) GetPath(ctx context.Context, path string) (*PageResult, error) {
	path = normalizePath(path)
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
