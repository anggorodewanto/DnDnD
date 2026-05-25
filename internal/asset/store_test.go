package asset

import "testing"

// TestAssetType_Valid verifies that all recognized asset types report valid and
// unknown types report invalid. TypeTilesetImage is added for multipart
// Tiled-project imports (single-image tileset / image-layer backing files).
func TestAssetType_Valid(t *testing.T) {
	valid := []AssetType{
		TypeMapBackground,
		TypeToken,
		TypeTileset,
		TypeNarration,
		TypeTilesetImage,
	}
	for _, at := range valid {
		if !at.Valid() {
			t.Errorf("expected %q to be valid", at)
		}
	}

	invalid := []AssetType{
		AssetType(""),
		AssetType("bogus"),
		AssetType("map"),
	}
	for _, at := range invalid {
		if at.Valid() {
			t.Errorf("expected %q to be invalid", at)
		}
	}
}
