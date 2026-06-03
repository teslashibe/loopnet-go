package loopnet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// getBytes fetches a path under baseURL and returns raw body + status.
func (c *Client) getBytes(ctx context.Context, path string, query url.Values) ([]byte, int, error) {
	full := baseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	return c.doRetried(ctx, http.MethodGet, full, nil, "")
}

func (c *Client) doRetried(ctx context.Context, method, rawURL string, body []byte, contentType string) ([]byte, int, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			wait := c.backoff(attempt)
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(wait):
			}
		}
		raw, status, err := c.doRequest(ctx, method, rawURL, body, contentType)
		if err == nil {
			return raw, status, nil
		}
		lastErr = err
		if errors.Is(err, ErrRateLimited) {
			continue
		}
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode >= 500 {
			continue
		}
		return nil, status, err
	}
	return nil, 0, lastErr
}

func (c *Client) doRequest(ctx context.Context, method, rawURL string, body []byte, contentType string) ([]byte, int, error) {
	c.waitForGap(ctx)
	if ctx.Err() != nil {
		return nil, 0, ctx.Err()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	c.setCommonHeaders(req, contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%w: reading body: %v", ErrRequestFailed, err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return raw, resp.StatusCode, nil
	case http.StatusUnauthorized:
		return nil, resp.StatusCode, ErrUnauthorized
	case http.StatusForbidden:
		// Akamai 403s carry server: AkamaiGHost and a tiny body.
		if strings.Contains(strings.ToLower(resp.Header.Get("Server")), "akamai") ||
			bytes.Contains(raw, []byte("Reference #")) {
			return nil, resp.StatusCode, fmt.Errorf("%w (HTTP 403)", ErrBotChallenge)
		}
		return nil, resp.StatusCode, ErrForbidden
	case http.StatusNotFound:
		return nil, resp.StatusCode, ErrNotFound
	case http.StatusTooManyRequests:
		c.gapMu.Lock()
		if earliest := time.Now().Add(60 * time.Second); c.lastReqAt.Before(earliest) {
			c.lastReqAt = earliest
		}
		c.gapMu.Unlock()
		return nil, resp.StatusCode, fmt.Errorf("%w: 429", ErrRateLimited)
	default:
		return nil, resp.StatusCode, &HTTPError{StatusCode: resp.StatusCode, Body: truncate(string(raw), 256)}
	}
}

func (c *Client) setCommonHeaders(req *http.Request, contentType string) {
	ua := c.userAgent
	c.authMu.RLock()
	if c.auth.UserAgent != "" {
		ua = c.auth.UserAgent
	}
	cookieHeader := c.auth.CookieHeader
	c.authMu.RUnlock()

	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// Deliberately do NOT set Accept-Encoding. net/http transparently adds
	// "gzip" and decompresses the response only when it sets the header
	// itself; setting it manually here disabled that, so every body came
	// back as raw gzip/br bytes (breaking substring checks and page tools).
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}
}

func (c *Client) backoff(attempt int) time.Duration {
	return time.Duration(math.Pow(2, float64(attempt-1))) * c.retryBase
}

func (c *Client) waitForGap(ctx context.Context) {
	c.gapMu.Lock()
	now := time.Now()
	next := c.lastReqAt.Add(c.minGap)
	if now.After(next) {
		next = now
	}
	c.lastReqAt = next
	c.gapMu.Unlock()
	if wait := time.Until(next); wait > 0 {
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
