package mcp

import (
	"context"

	loopnet "github.com/teslashibe/loopnet-go"
	"github.com/teslashibe/mcptool"
)

// Read tools ----------------------------------------------------------------

// NoInput is an empty tool input.
type NoInput struct{}

func getSavedListings(ctx context.Context, c *loopnet.Client, _ NoInput) (any, error) {
	return c.GetSavedListings(ctx)
}

func getWatchlists(ctx context.Context, c *loopnet.Client, _ NoInput) (any, error) {
	return c.GetWatchlists(ctx)
}

// Write tools (guarded) -----------------------------------------------------

// CreateWatchFolderInput creates a new saved-listing folder.
type CreateWatchFolderInput struct {
	Name    string `json:"name" jsonschema:"description=Folder name,required"`
	Type    string `json:"type" jsonschema:"description=ForSale or ForLease,required"`
	Confirm bool   `json:"confirm" jsonschema:"description=Must be true to perform this write,required"`
}

func createWatchFolder(ctx context.Context, c *loopnet.Client, in CreateWatchFolderInput) (any, error) {
	return c.CreateWatchFolder(ctx, in.Name, in.Type, in.Confirm)
}

// SaveListingInput saves a listing into a folder.
type SaveListingInput struct {
	ListingID int    `json:"listing_id" jsonschema:"description=LoopNet listing id,required"`
	FolderID  int    `json:"folder_id" jsonschema:"description=Target watch-folder id"`
	Type      string `json:"type" jsonschema:"description=ForSale or ForLease"`
	Confirm   bool   `json:"confirm" jsonschema:"description=Must be true to perform this write,required"`
}

func saveListing(ctx context.Context, c *loopnet.Client, in SaveListingInput) (any, error) {
	return c.AddListingToWatchFolder(ctx, in.FolderID, in.ListingID, in.Type, in.Confirm)
}

// RemoveSavedListingInput removes a listing from all saved folders.
type RemoveSavedListingInput struct {
	ListingID int  `json:"listing_id" jsonschema:"description=LoopNet listing id to remove,required"`
	Confirm   bool `json:"confirm" jsonschema:"description=Must be true to perform this write,required"`
}

func removeSavedListing(ctx context.Context, c *loopnet.Client, in RemoveSavedListingInput) (any, error) {
	return c.RemoveListingFromAllWatchFolders(ctx, in.ListingID, in.Confirm)
}

var serviceTools = []mcptool.Tool{
	mcptool.Define[*loopnet.Client, NoInput](
		"loopnet_get_saved_listings",
		"Return your saved-listing folders (favorites), grouped ForSale/ForLease, as JSON.",
		"GetSavedListings",
		getSavedListings,
	),
	mcptool.Define[*loopnet.Client, NoInput](
		"loopnet_get_watchlists",
		"Return your saved-listing watch folders (id, name, type, listing ids) as JSON.",
		"GetWatchlists",
		getWatchlists,
	),
	mcptool.Define[*loopnet.Client, CreateWatchFolderInput](
		"loopnet_create_watch_folder",
		"Create a saved-listing folder. Guarded write: requires confirm=true.",
		"CreateWatchFolder",
		createWatchFolder,
	),
	mcptool.Define[*loopnet.Client, SaveListingInput](
		"loopnet_save_listing",
		"Save a listing into a watch folder. Guarded write: requires confirm=true.",
		"SaveListing",
		saveListing,
	),
	mcptool.Define[*loopnet.Client, RemoveSavedListingInput](
		"loopnet_remove_saved_listing",
		"Remove a listing from all saved folders. Guarded write: requires confirm=true.",
		"RemoveSavedListing",
		removeSavedListing,
	),
}
