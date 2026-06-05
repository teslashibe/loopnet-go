package loopnet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// LoopNet's authenticated SPA is backed by a JSON REST surface under
// https://www.loopnet.com/services/*. These were reverse-engineered from the
// account bundles + verified against a live session (teslashibe/smore#137).
// The methods below wrap the read and write endpoints with typed Go signatures.
//
// Auth is the same pasted-cookie session as the rest of the client; the calls
// send the XHR headers the SPA uses (Accept: application/json,
// X-Requested-With: XMLHttpRequest). All writes go through writeGuard so a
// caller cannot mutate the account without opting in.
const (
	svcFavoritesFolders  = "/services/favorites/folders"
	svcWatchlistings     = "/services/search/watchlistings"
	svcCreateWatchFolder = "/services/search/createwatchfolder"
	svcAddListing        = "/services/search/addlistingtowatchfolder"
	svcChangeListing     = "/services/search/changelistingtowatchfolder"
	svcRemoveListingAll  = "/services/search/removelistingfromallwatchfolders/"
	svcSendToFriend      = "/services/contact/send2friend/"
)

// WatchFolder is a saved-listing folder as returned by GetWatchlists.
type WatchFolder struct {
	ID          int    `json:"Id"`
	Name        string `json:"Name"`
	IsDefault   bool   `json:"IsDefault"`
	Type        string `json:"Type"` // "ForSale" | "ForLease"
	ListingIDs  []int  `json:"ListingIds"`
	DateCreated string `json:"DateCreated"`
	DateUpdated string `json:"DateUpdated"`
}

// serviceJSON performs an authenticated JSON request against a /services/*
// endpoint. body is marshaled to JSON when non-nil; out (when non-nil) receives
// the decoded JSON response.
func (c *Client) serviceJSON(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%w: marshal body: %v", ErrInvalidParams, err)
		}
		reader = bytes.NewReader(raw)
	}

	c.waitForGap(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}

	c.authMu.RLock()
	ua := c.userAgent
	if c.auth.UserAgent != "" {
		ua = c.auth.UserAgent
	}
	cookie := c.auth.CookieHeader
	c.authMu.RUnlock()

	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", baseURL+"/")
	if reader != nil {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		// ok
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		if bytes.Contains(raw, []byte("Reference #")) {
			return fmt.Errorf("%w (HTTP 403)", ErrBotChallenge)
		}
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return &HTTPError{StatusCode: resp.StatusCode, Body: truncate(string(raw), 256)}
	}

	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%w: decode %s: %v", ErrRequestFailed, path, err)
		}
	}
	return nil
}

// --- Reads ---

// GetSavedListings returns the user's saved-listing folders (favorites),
// grouped by ForSale/ForLease, as raw JSON for full fidelity. Verified live.
func (c *Client) GetSavedListings(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.serviceJSON(ctx, http.MethodGet, svcFavoritesFolders, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetWatchlists returns the user's saved-listing watch folders (Id, Name, Type,
// ListingIds). Verified live.
func (c *Client) GetWatchlists(ctx context.Context) ([]WatchFolder, error) {
	var out []WatchFolder
	if err := c.serviceJSON(ctx, http.MethodGet, svcWatchlistings, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- Writes (guarded) ---

// writeGuard blocks account-mutating calls unless the caller explicitly opts
// in. CoStar litigates automated access; writes must be deliberate.
func (c *Client) writeGuard(confirm bool) error {
	if !confirm {
		return fmt.Errorf("%w: this is an account-mutating write; pass confirm=true to proceed", ErrInvalidParams)
	}
	return nil
}

// CreateWatchFolder creates a new saved-listing folder.
// Payload (from the SPA): {Name, IsDefault:false, Type}. Type is "ForSale" or
// "ForLease". Guarded write.
func (c *Client) CreateWatchFolder(ctx context.Context, name, folderType string, confirm bool) (json.RawMessage, error) {
	if err := c.writeGuard(confirm); err != nil {
		return nil, err
	}
	if name == "" || folderType == "" {
		return nil, fmt.Errorf("%w: name and type are required", ErrInvalidParams)
	}
	body := map[string]any{"Name": name, "IsDefault": false, "Type": folderType}
	var out json.RawMessage
	if err := c.serviceJSON(ctx, http.MethodPost, svcCreateWatchFolder, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddListingToWatchFolder saves a listing into a folder.
// Payload (from the SPA): {FolderId, ListingId, Type}. Guarded write.
func (c *Client) AddListingToWatchFolder(ctx context.Context, folderID, listingID int, folderType string, confirm bool) (json.RawMessage, error) {
	if err := c.writeGuard(confirm); err != nil {
		return nil, err
	}
	if listingID == 0 {
		return nil, fmt.Errorf("%w: listingID is required", ErrInvalidParams)
	}
	body := map[string]any{"FolderId": folderID, "ListingId": listingID, "Type": folderType}
	var out json.RawMessage
	if err := c.serviceJSON(ctx, http.MethodPost, svcAddListing, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveListingFromAllWatchFolders removes a listing from every saved folder.
// The SPA POSTs to .../removelistingfromallwatchfolders/{listingId} (no body).
// Guarded write.
func (c *Client) RemoveListingFromAllWatchFolders(ctx context.Context, listingID int, confirm bool) (json.RawMessage, error) {
	if err := c.writeGuard(confirm); err != nil {
		return nil, err
	}
	if listingID == 0 {
		return nil, fmt.Errorf("%w: listingID is required", ErrInvalidParams)
	}
	var out json.RawMessage
	if err := c.serviceJSON(ctx, http.MethodPost, svcRemoveListingAll+strconv.Itoa(listingID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
