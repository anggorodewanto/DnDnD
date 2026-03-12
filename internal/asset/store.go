package asset

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// AssetType represents the type of asset being stored.
type AssetType string

const (
	TypeMapBackground AssetType = "map_background"
	TypeToken         AssetType = "token"
	TypeTileset       AssetType = "tileset"
	TypeNarration     AssetType = "narration"
)

// Valid reports whether t is a recognized asset type.
func (t AssetType) Valid() bool {
	switch t {
	case TypeMapBackground, TypeToken, TypeTileset, TypeNarration:
		return true
	default:
		return false
	}
}

// Store defines the interface for asset storage backends.
// Implementations can target local filesystem, S3, etc.
type Store interface {
	// Put stores the content from r and returns the relative storage path.
	Put(ctx context.Context, campaignID uuid.UUID, assetType AssetType, filename string, r io.Reader) (storagePath string, err error)

	// Get returns a ReadCloser for the asset at the given storage path.
	Get(ctx context.Context, storagePath string) (io.ReadCloser, error)

	// Delete removes the asset at the given storage path.
	Delete(ctx context.Context, storagePath string) error

	// URL returns a URL for accessing the asset. For local stores this is the API path.
	URL(assetID uuid.UUID) string
}
