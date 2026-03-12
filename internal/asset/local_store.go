package asset

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// typeToDir maps asset types to their storage directory names.
var typeToDir = map[AssetType]string{
	TypeMapBackground: "maps",
	TypeToken:         "tokens",
	TypeTileset:       "tilesets",
	TypeNarration:     "narration",
}

// LocalStore implements Store using the local filesystem.
type LocalStore struct {
	baseDir string
}

// NewLocalStore creates a new LocalStore rooted at baseDir.
func NewLocalStore(baseDir string) *LocalStore {
	return &LocalStore{baseDir: baseDir}
}

// Put stores the content from r under {baseDir}/{campaignID}/{typeDir}/{uuid}.
func (s *LocalStore) Put(ctx context.Context, campaignID uuid.UUID, assetType AssetType, _ string, r io.Reader) (string, error) {
	dir, ok := typeToDir[assetType]
	if !ok {
		return "", fmt.Errorf("unknown asset type: %s", assetType)
	}

	relDir := filepath.Join(campaignID.String(), dir)
	absDir := filepath.Join(s.baseDir, relDir)

	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", fmt.Errorf("creating directory %s: %w", absDir, err)
	}

	fileID := uuid.New()
	relPath := filepath.Join(relDir, fileID.String())
	absPath := filepath.Join(s.baseDir, relPath)

	f, err := os.Create(absPath)
	if err != nil {
		return "", fmt.Errorf("creating file %s: %w", absPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("writing file %s: %w", absPath, err)
	}

	return relPath, nil
}

// Get returns a ReadCloser for the asset at the given storage path.
func (s *LocalStore) Get(ctx context.Context, storagePath string) (io.ReadCloser, error) {
	absPath := filepath.Join(s.baseDir, storagePath)
	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", absPath, err)
	}
	return f, nil
}

// Delete removes the asset at the given storage path.
func (s *LocalStore) Delete(ctx context.Context, storagePath string) error {
	absPath := filepath.Join(s.baseDir, storagePath)
	if err := os.Remove(absPath); err != nil {
		return fmt.Errorf("removing file %s: %w", absPath, err)
	}
	return nil
}

// URL returns the API path for accessing the asset.
func (s *LocalStore) URL(assetID uuid.UUID) string {
	return "/api/assets/" + assetID.String()
}
