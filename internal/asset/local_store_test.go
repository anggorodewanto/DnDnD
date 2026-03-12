package asset

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStore_Put(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalStore(baseDir)

	campaignID := uuid.New()
	content := []byte("hello world")

	storagePath, err := store.Put(context.Background(), campaignID, TypeMapBackground, "test.png", bytes.NewReader(content))
	require.NoError(t, err)

	// Storage path should contain campaign ID and type directory
	assert.Contains(t, storagePath, campaignID.String())
	assert.Contains(t, storagePath, "maps")

	// File should exist on disk
	fullPath := filepath.Join(baseDir, storagePath)
	data, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	// Filename should be a UUID (not original name) to avoid collisions
	parts := strings.Split(storagePath, "/")
	filename := parts[len(parts)-1]
	_, err = uuid.Parse(filename)
	assert.NoError(t, err, "filename should be a UUID")
}

func TestLocalStore_Get(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalStore(baseDir)

	campaignID := uuid.New()
	content := []byte("get me back")

	storagePath, err := store.Put(context.Background(), campaignID, TypeToken, "icon.png", bytes.NewReader(content))
	require.NoError(t, err)

	rc, err := store.Get(context.Background(), storagePath)
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestLocalStore_Get_NotFound(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalStore(baseDir)

	_, err := store.Get(context.Background(), "nonexistent/path")
	assert.Error(t, err)
}

func TestLocalStore_Delete(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalStore(baseDir)

	campaignID := uuid.New()
	content := []byte("delete me")

	storagePath, err := store.Put(context.Background(), campaignID, TypeTileset, "tile.png", bytes.NewReader(content))
	require.NoError(t, err)

	// File exists
	fullPath := filepath.Join(baseDir, storagePath)
	_, err = os.Stat(fullPath)
	require.NoError(t, err)

	// Delete
	err = store.Delete(context.Background(), storagePath)
	require.NoError(t, err)

	// File gone
	_, err = os.Stat(fullPath)
	assert.True(t, os.IsNotExist(err))
}

func TestLocalStore_Delete_NotFound(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalStore(baseDir)

	err := store.Delete(context.Background(), "nonexistent/path")
	assert.Error(t, err)
}

func TestLocalStore_URL(t *testing.T) {
	store := NewLocalStore("/tmp")
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	assert.Equal(t, "/api/assets/11111111-1111-1111-1111-111111111111", store.URL(id))
}

func TestLocalStore_Put_UnknownType(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalStore(baseDir)

	_, err := store.Put(context.Background(), uuid.New(), AssetType("invalid"), "f.png", bytes.NewReader([]byte("x")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown asset type")
}

func TestLocalStore_Put_AllTypes(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalStore(baseDir)
	campaignID := uuid.New()

	tests := []struct {
		assetType AssetType
		wantDir   string
	}{
		{TypeMapBackground, "maps"},
		{TypeToken, "tokens"},
		{TypeTileset, "tilesets"},
		{TypeNarration, "narration"},
	}

	for _, tt := range tests {
		t.Run(string(tt.assetType), func(t *testing.T) {
			path, err := store.Put(context.Background(), campaignID, tt.assetType, "f.bin", bytes.NewReader([]byte("data")))
			require.NoError(t, err)
			assert.Contains(t, path, tt.wantDir)
		})
	}
}

func TestLocalStore_ImplementsStore(t *testing.T) {
	var _ Store = (*LocalStore)(nil)
}

func TestLocalStore_Put_MkdirFails(t *testing.T) {
	// Use a path that can't be created (file as parent)
	baseDir := t.TempDir()
	// Create a file where a directory needs to be
	blockFile := filepath.Join(baseDir, "block")
	require.NoError(t, os.WriteFile(blockFile, []byte("x"), 0o644))

	store := NewLocalStore(blockFile + "/nested")
	_, err := store.Put(context.Background(), uuid.New(), TypeToken, "f.png", bytes.NewReader([]byte("x")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating directory")
}

func TestLocalStore_Put_ReadError(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalStore(baseDir)

	_, err := store.Put(context.Background(), uuid.New(), TypeToken, "f.png", &errReader{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "writing file")
}

type errReader struct{}

func (e *errReader) Read(p []byte) (int, error) {
	return 0, errors.New("read failure")
}
