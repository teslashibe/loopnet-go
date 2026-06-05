package mcp

import (
	"context"

	loopnet "github.com/teslashibe/loopnet-go"
	"github.com/teslashibe/mcptool"
)

// SearchListingsInput drives a LoopNet commercial-listing search.
type SearchListingsInput struct {
	Geography   string `json:"geography" jsonschema:"description=LoopNet geo slug e.g. los-angeles-ca or usa,required"`
	ListingType string `json:"listing_type,omitempty" jsonschema:"description=for-sale or for-lease (default for-sale)"`
	Category    string `json:"category,omitempty" jsonschema:"description=category slug (default commercial-real-estate)"`
	Page        int    `json:"page,omitempty" jsonschema:"description=1-based results page"`
}

func searchListings(ctx context.Context, c *loopnet.Client, in SearchListingsInput) (any, error) {
	return c.SearchListings(ctx, loopnet.SearchParams{
		Geography:   in.Geography,
		ListingType: in.ListingType,
		Category:    in.Category,
		Page:        in.Page,
	})
}

// GetListingInput fetches one listing's structured detail.
type GetListingInput struct {
	Listing string `json:"listing" jsonschema:"description=Numeric LoopNet listing id or full /Listing/ URL,required"`
}

func getListing(ctx context.Context, c *loopnet.Client, in GetListingInput) (any, error) {
	return c.GetListing(ctx, in.Listing)
}

var listingTools = []mcptool.Tool{
	mcptool.Define[*loopnet.Client, SearchListingsInput](
		"loopnet_search_listings",
		"Search LoopNet commercial listings by geography + type; returns listing ids/urls/addresses as JSON.",
		"SearchListings",
		searchListings,
	),
	mcptool.Define[*loopnet.Client, GetListingInput](
		"loopnet_get_listing",
		"Fetch one LoopNet listing's structured detail (price, address, brokers, images) by id or URL.",
		"GetListing",
		getListing,
	),
}
