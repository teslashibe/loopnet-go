package loopnet

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Listing search + detail over LoopNet's SSR pages, fetched through the
// Chrome-fingerprinted strict transport (tlsfetch.go) and parsed from the
// embedded JSON-LD + placard DOM. LoopNet renders results server-side (no
// JSON data API), so we extract structured data from the HTML rather than a
// gateway. Verified live: SRP yields ~25 listings/page; detail pages carry
// schema.org JSON-LD with price/address/broker/images.

// ListingSummary is one result card parsed from a search results page. Fields
// are best-effort from the placard <article> (gtm-* attrs + header text +
// event-model coords); empty strings mean the card didn't expose that field.
type ListingSummary struct {
	ID           string   `json:"id"`
	URL          string   `json:"url"`
	Headline     string   `json:"headline,omitempty"`
	Address      string   `json:"address,omitempty"`
	City         string   `json:"city,omitempty"`
	State        string   `json:"state,omitempty"`
	Zip          string   `json:"zip,omitempty"`
	SizeType     string   `json:"size_type,omitempty"` // e.g. "63,000 SF Industrial"
	PropertyType string   `json:"property_type,omitempty"`
	ListingType  string   `json:"listing_type,omitempty"` // FS (for sale) / FL (for lease)
	Price        string   `json:"price,omitempty"`
	ImageURL     string   `json:"image_url,omitempty"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
}

// ListingDetail is the structured data parsed from a single listing page.
type ListingDetail struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Price       string   `json:"price,omitempty"`
	Currency    string   `json:"currency,omitempty"`
	Address     string   `json:"address,omitempty"`
	City        string   `json:"city,omitempty"`
	State       string   `json:"state,omitempty"`
	Postal      string   `json:"postal_code,omitempty"`
	Brokers     []Broker `json:"brokers,omitempty"`
	Images      []string `json:"images,omitempty"`
}

// Broker is a listing agent + their firm.
type Broker struct {
	Name string `json:"name"`
	Firm string `json:"firm,omitempty"`
}

// SearchParams describes a listing search. Geography is a LoopNet slug such as
// "los-angeles-ca" or "usa"; ListingType is "for-sale" or "for-lease";
// Category defaults to "commercial-real-estate". Page is 1-based.
type SearchParams struct {
	Geography   string
	ListingType string
	Category    string
	Page        int
}

func (p SearchParams) path() (string, error) {
	geo := strings.Trim(strings.TrimSpace(p.Geography), "/")
	if geo == "" {
		return "", fmt.Errorf("%w: geography is required (e.g. \"los-angeles-ca\" or \"usa\")", ErrInvalidParams)
	}
	lt := strings.TrimSpace(p.ListingType)
	if lt == "" {
		lt = "for-sale"
	}
	if lt != "for-sale" && lt != "for-lease" {
		return "", fmt.Errorf("%w: listing_type must be \"for-sale\" or \"for-lease\"", ErrInvalidParams)
	}
	cat := strings.TrimSpace(p.Category)
	if cat == "" {
		cat = "commercial-real-estate"
	}
	path := fmt.Sprintf("/search/%s/%s/%s/", url.PathEscape(cat), url.PathEscape(geo), lt)
	if p.Page > 1 {
		path += fmt.Sprintf("?sk=%d&page=%d", p.Page, p.Page)
	}
	return path, nil
}

var (
	listingLinkRe = regexp.MustCompile(`https://www\.loopnet\.com/Listing/[^"'\s]+?/(\d+)/`)
	ldJSONRe      = regexp.MustCompile(`(?is)<script[^>]+type="application/ld\+json"[^>]*>(.*?)</script>`)
	titleTagRe    = regexp.MustCompile(`(?is)<title>\s*(.*?)\s*</title>`)
	listingIDRe   = regexp.MustCompile(`/(\d+)/?$`)

	// Per-placard field extractors (run within a single <article> chunk).
	cardIDRe       = regexp.MustCompile(`data-id='(\d+)'`)
	cardLeftH6Re   = regexp.MustCompile(`(?is)class="left-h6"[^>]*>(.*?)</a>`)
	cardLeftH4Re   = regexp.MustCompile(`(?is)class="left-h4"[^>]*>(.*?)</a>`)
	cardRightH4Re  = regexp.MustCompile(`(?is)class="right-h4"[^>]*>(.*?)</a>`)
	cardPropTypeRe = regexp.MustCompile(`gtm-listing-property-type-name="([^"]*)"`)
	cardListTypeRe = regexp.MustCompile(`gtm-listing-search-type="([^"]*)"`)
	cardCityRe     = regexp.MustCompile(`gtm-listing-city="([^"]*)"`)
	cardStateRe    = regexp.MustCompile(`gtm-listing-state="([^"]*)"`)
	cardZipRe      = regexp.MustCompile(`gtm-listing-zip="([^"]*)"`)
	cardPriceRe    = regexp.MustCompile(`\$[0-9][0-9,]*(?:\.[0-9]+)?(?:/SF(?:/YR|/MO)?)?`)
	cardImageRe    = regexp.MustCompile(`https://images1\.loopnet\.com/[^"')]+\.jpg`)
	// Coordinates come from the placard-event-model JSON (HTML-attr-encoded).
	coordRe = regexp.MustCompile(`ListingID&quot;:(\d+),&quot;Latitude&quot;:(null|-?[\d.]+),&quot;Longitude&quot;:(null|-?[\d.]+)`)

	tagStripRe = regexp.MustCompile(`(?s)<[^>]+>`)
)

// SearchListings fetches one page of search results and returns the listing
// cards (id, url, name/address). Order is preserved (de-duplicated by id).
func (c *Client) SearchListings(ctx context.Context, p SearchParams) ([]ListingSummary, error) {
	path, err := p.path()
	if err != nil {
		return nil, err
	}
	body, _, err := c.fetchStrict(ctx, baseURL+path)
	if err != nil {
		return nil, err
	}
	out := parseSearchResults(string(body))
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no listings parsed from %s (page layout may have changed, or 0 results)", ErrNotFound, path)
	}
	return out, nil
}

// parseSearchResults extracts ordered, de-duplicated listing cards from SRP
// HTML. Each placard <article> is parsed for its id, URL, header text
// (headline/address/size+type), gtm-* attributes (city/state/zip/type), price,
// and lead image; coordinates are merged from the placard-event-model JSON.
func parseSearchResults(htmlStr string) []ListingSummary {
	coords := parseCoords(htmlStr) // id -> [lat,lng]

	seen := map[string]bool{}
	var out []ListingSummary
	// Split into per-card chunks. Each chunk after the first holds one
	// <article> (its fields are the first matches within the chunk).
	for _, chunk := range strings.Split(htmlStr, "<article") {
		idm := cardIDRe.FindStringSubmatch(chunk)
		if idm == nil {
			continue
		}
		id := idm[1]
		if seen[id] {
			continue
		}
		seen[id] = true

		s := ListingSummary{ID: id}
		if m := listingLinkRe.FindString(chunk); m != "" {
			s.URL = m
		}
		s.Headline = cleanText(firstGroup(cardLeftH6Re, chunk))
		s.Address = cleanText(firstGroup(cardLeftH4Re, chunk))
		s.SizeType = cleanText(firstGroup(cardRightH4Re, chunk))
		s.PropertyType = firstGroup(cardPropTypeRe, chunk)
		s.ListingType = firstGroup(cardListTypeRe, chunk)
		s.City = firstGroup(cardCityRe, chunk)
		s.State = firstGroup(cardStateRe, chunk)
		s.Zip = firstGroup(cardZipRe, chunk)
		s.Price = cardPriceRe.FindString(chunk)
		s.ImageURL = cardImageRe.FindString(chunk)
		if s.Address == "" && s.URL != "" {
			s.Address = addressFromListingURL(s.URL)
		}
		if c, ok := coords[id]; ok {
			lat, lng := c[0], c[1]
			s.Latitude, s.Longitude = &lat, &lng
		}
		out = append(out, s)
	}
	return out
}

// parseCoords reads the placard-event-model JSON for per-listing lat/long.
func parseCoords(htmlStr string) map[string][2]float64 {
	out := map[string][2]float64{}
	for _, m := range coordRe.FindAllStringSubmatch(htmlStr, -1) {
		if m[2] == "null" || m[3] == "null" {
			continue
		}
		lat, err1 := strconv.ParseFloat(m[2], 64)
		lng, err2 := strconv.ParseFloat(m[3], 64)
		if err1 == nil && err2 == nil {
			out[m[1]] = [2]float64{lat, lng}
		}
	}
	return out
}

func firstGroup(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// cleanText strips inner tags + unescapes HTML entities from a captured node.
func cleanText(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(tagStripRe.ReplaceAllString(s, "")))
}

// GetListing fetches a single listing page (by numeric id or full URL) and
// returns its structured detail parsed from JSON-LD.
func (c *Client) GetListing(ctx context.Context, idOrURL string) (*ListingDetail, error) {
	target := strings.TrimSpace(idOrURL)
	if target == "" {
		return nil, fmt.Errorf("%w: listing id or url is required", ErrInvalidParams)
	}
	if !strings.HasPrefix(target, "http") {
		if _, err := strconv.Atoi(target); err != nil {
			return nil, fmt.Errorf("%w: pass a numeric listing id or a full /Listing/ URL", ErrInvalidParams)
		}
		// LoopNet accepts a slug-less canonical id URL and 301s to the slug.
		target = fmt.Sprintf("%s/Listing/%s/", baseURL, target)
	}
	body, _, err := c.fetchStrict(ctx, target)
	if err != nil {
		return nil, err
	}
	d := parseListingDetail(string(body))
	d.URL = target
	if m := listingIDRe.FindStringSubmatch(strings.TrimRight(target, "/") + "/"); len(m) == 2 {
		d.ID = m[1]
	}
	if d.Name == "" && d.Address == "" {
		return nil, fmt.Errorf("%w: no structured data parsed from listing page", ErrNotFound)
	}
	return d, nil
}

// parseListingDetail walks the page's JSON-LD blocks to assemble a detail.
func parseListingDetail(htmlStr string) *ListingDetail {
	d := &ListingDetail{}
	for _, node := range jsonLDNodes(htmlStr) {
		applyLDNode(node, d)
	}
	if d.Name == "" {
		if m := titleTagRe.FindStringSubmatch(htmlStr); len(m) == 2 {
			d.Name = strings.TrimSpace(html.UnescapeString(m[1]))
		}
	}
	return d
}

// --- JSON-LD helpers -------------------------------------------------------

// jsonLDNodes returns each <script type="application/ld+json"> payload decoded
// into a generic structure (object or array), skipping ones that don't parse.
func jsonLDNodes(htmlStr string) []any {
	var out []any
	for _, m := range ldJSONRe.FindAllStringSubmatch(htmlStr, -1) {
		raw := strings.TrimSpace(html.UnescapeString(m[1]))
		var v any
		if json.Unmarshal([]byte(raw), &v) == nil {
			out = append(out, v)
		}
	}
	return out
}

// applyLDNode fills d from a listing-detail JSON-LD node (Product/Place/Offer/
// PostalAddress/RealEstateAgent), recursing into nested objects/arrays.
func applyLDNode(node any, d *ListingDetail) {
	switch n := node.(type) {
	case []any:
		for _, e := range n {
			applyLDNode(e, d)
		}
	case map[string]any:
		switch typeStr(n["@type"]) {
		case "Product", "Residence", "Place", "RealEstateListing", "Apartment", "SingleFamilyResidence":
			if s, _ := n["name"].(string); s != "" && d.Name == "" {
				d.Name = strings.TrimSpace(s)
			}
			if s, _ := n["description"].(string); s != "" && d.Description == "" {
				d.Description = strings.TrimSpace(s)
			}
		case "Offer":
			if d.Price == "" {
				if s := stringField(n["price"]); s != "" {
					d.Price = s
				}
			}
			if s, _ := n["priceCurrency"].(string); s != "" && d.Currency == "" {
				d.Currency = s
			}
		case "PostalAddress":
			if s, _ := n["streetAddress"].(string); s != "" && d.Address == "" {
				d.Address = strings.TrimSpace(s)
			}
			if s, _ := n["addressLocality"].(string); s != "" && d.City == "" {
				d.City = strings.TrimSpace(s)
			}
			if s, _ := n["addressRegion"].(string); s != "" && d.State == "" {
				d.State = strings.TrimSpace(s)
			}
			if s, _ := n["postalCode"].(string); s != "" && d.Postal == "" {
				d.Postal = strings.TrimSpace(s)
			}
		case "RealEstateAgent", "Person":
			if name, _ := n["name"].(string); name != "" {
				b := Broker{Name: strings.TrimSpace(name)}
				if mo, ok := n["memberOf"].(map[string]any); ok {
					b.Firm, _ = mo["name"].(string)
				}
				if !hasBroker(d.Brokers, b.Name) {
					d.Brokers = append(d.Brokers, b)
				}
			}
		case "ImageObject":
			if u, _ := n["url"].(string); u != "" && !contains(d.Images, u) {
				d.Images = append(d.Images, u)
			}
		}
		for _, v := range n {
			switch v.(type) {
			case []any, map[string]any:
				applyLDNode(v, d)
			}
		}
	}
}

// --- small utilities -------------------------------------------------------

func typeStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

// stringField renders an LD value that may be a string, number, or nested
// PriceSpecification into a display string.
func stringField(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case map[string]any:
		return stringField(t["price"])
	}
	return ""
}

func addressFromListingURL(u string) string {
	// .../Listing/3250-Glendale-Blvd-Los-Angeles-CA/29645158/
	parts := strings.Split(strings.Trim(u, "/"), "/")
	for i, p := range parts {
		if p == "Listing" && i+1 < len(parts) {
			return strings.ReplaceAll(parts[i+1], "-", " ")
		}
	}
	return ""
}

func hasBroker(bs []Broker, name string) bool {
	for _, b := range bs {
		if strings.EqualFold(b.Name, name) {
			return true
		}
	}
	return false
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
