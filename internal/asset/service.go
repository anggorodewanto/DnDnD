package asset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// validTypes is the set of allowed asset types.
var validTypes = map[AssetType]bool{
	TypeMapBackground: true,
	TypeToken:         true,
	TypeTileset:       true,
	TypeNarration:     true,
}

// DBStore defines the database operations needed by the asset service.
type DBStore interface {
	CreateAsset(ctx context.Context, arg refdata.CreateAssetParams) (refdata.Asset, error)
	GetAssetByID(ctx context.Context, id uuid.UUID) (refdata.Asset, error)
	DeleteAsset(ctx context.Context, id uuid.UUID) error
	ListAssetsByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Asset, error)
}

// UploadInput holds the parameters for uploading an asset.
type UploadInput struct {
	CampaignID   uuid.UUID
	Type         AssetType
	OriginalName string
	MimeType     string
	Content      io.Reader
}

// Service manages asset CRUD, coordinating between database and file storage.
type Service struct {
	db    DBStore
	store Store
}

// NewService creates a new asset Service.
func NewService(db DBStore, store Store) *Service {
	return &Service{db: db, store: store}
}

// Upload validates input, stores the file, and creates the DB record.
func (s *Service) Upload(ctx context.Context, input UploadInput) (refdata.Asset, error) {
	if err := validateUpload(input); err != nil {
		return refdata.Asset{}, err
	}

	// Buffer content to get byte size
	var buf bytes.Buffer
	size, err := io.Copy(&buf, input.Content)
	if err != nil {
		return refdata.Asset{}, fmt.Errorf("reading content: %w", err)
	}

	storagePath, err := s.store.Put(ctx, input.CampaignID, input.Type, input.OriginalName, &buf)
	if err != nil {
		return refdata.Asset{}, fmt.Errorf("storing file: %w", err)
	}

	asset, err := s.db.CreateAsset(ctx, refdata.CreateAssetParams{
		CampaignID:   input.CampaignID,
		Type:         string(input.Type),
		OriginalName: input.OriginalName,
		MimeType:     input.MimeType,
		ByteSize:     size,
		StoragePath:  storagePath,
	})
	if err != nil {
		// Best-effort cleanup of the stored file
		_ = s.store.Delete(ctx, storagePath)
		return refdata.Asset{}, fmt.Errorf("creating asset record: %w", err)
	}

	return asset, nil
}

// GetByID retrieves an asset record by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
	return s.db.GetAssetByID(ctx, id)
}

// OpenFile retrieves an asset record and opens its file for reading.
func (s *Service) OpenFile(ctx context.Context, id uuid.UUID) (refdata.Asset, io.ReadCloser, error) {
	asset, err := s.db.GetAssetByID(ctx, id)
	if err != nil {
		return refdata.Asset{}, nil, fmt.Errorf("getting asset: %w", err)
	}

	rc, err := s.store.Get(ctx, asset.StoragePath)
	if err != nil {
		return refdata.Asset{}, nil, fmt.Errorf("opening file: %w", err)
	}

	return asset, rc, nil
}

// Delete removes the asset from both storage and database.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	asset, err := s.db.GetAssetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting asset: %w", err)
	}

	if err := s.store.Delete(ctx, asset.StoragePath); err != nil {
		return fmt.Errorf("deleting file: %w", err)
	}

	if err := s.db.DeleteAsset(ctx, id); err != nil {
		return fmt.Errorf("deleting asset record: %w", err)
	}

	return nil
}

// URL returns the URL for accessing an asset.
func (s *Service) URL(assetID uuid.UUID) string {
	return s.store.URL(assetID)
}

// validateUpload checks upload input for validity.
func validateUpload(input UploadInput) error {
	if !validTypes[input.Type] {
		return errors.New("invalid asset type: " + string(input.Type))
	}
	if input.OriginalName == "" {
		return errors.New("original_name must not be empty")
	}
	if input.MimeType == "" {
		return errors.New("mime_type must not be empty")
	}
	if input.Content == nil {
		return errors.New("content must not be nil")
	}
	return nil
}
