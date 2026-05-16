package loopnet

import (
	"errors"
	"fmt"
)

// Sentinel errors.
var (
	ErrInvalidAuth   = errors.New("loopnet: missing or invalid auth credentials (paste browser CookieHeader)")
	ErrUnauthorized  = errors.New("loopnet: unauthorized (session expired — re-paste cookies)")
	ErrForbidden     = errors.New("loopnet: forbidden")
	ErrNotFound      = errors.New("loopnet: not found")
	ErrRateLimited   = errors.New("loopnet: rate limited")
	ErrInvalidParams = errors.New("loopnet: invalid parameters")
	ErrRequestFailed = errors.New("loopnet: request failed")
	ErrBotChallenge  = errors.New("loopnet: Akamai bot challenge — refresh _abck/bm_sz/ak_bmsc cookies from a real browser session")
)

// HTTPError is returned for unexpected non-2xx HTTP responses.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("loopnet: HTTP %d: %s", e.StatusCode, e.Body)
}
