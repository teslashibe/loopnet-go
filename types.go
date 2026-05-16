package loopnet

// Auth carries the credentials needed to talk to loopnet.com.
//
// Email + Password are stored for future use but are NOT used today —
// Akamai's challenge cookies cannot be produced programmatically.
// CookieHeader is the working auth path: paste the entire `cookie:`
// request header from any authenticated DevTools → Network request on
// www.loopnet.com.
type Auth struct {
	Email        string
	Password     string
	CookieHeader string

	// UserAgent must match the User-Agent of the browser session the
	// CookieHeader was captured from. Akamai fingerprints UA+cookie
	// together and will rotate _abck out from under you if they
	// disagree. If empty, the client falls back to a recent Chrome UA.
	UserAgent string
}

// User is the minimal "current user" stub. LoopNet's /myloopnet
// dashboard has no JSON /me endpoint; we infer login state from
// dashboard HTML.
type User struct {
	DisplayName string `json:"display_name,omitempty"`
	LoggedIn    bool   `json:"logged_in"`
}

// PageResult is the envelope returned by raw page fetches.
type PageResult struct {
	URL          string `json:"url"`
	StatusCode   int    `json:"status_code"`
	ContentBytes int    `json:"content_bytes"`
	Title        string `json:"title,omitempty"`
	HTML         string `json:"html,omitempty"`
}
