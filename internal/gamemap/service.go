package gamemap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

const (
	// SoftLimitDimension is the max dimension before auto-downscaling.
	SoftLimitDimension = 100
	// HardLimitDimension is the absolute max dimension.
	HardLimitDimension = 200
	// StandardTileSize is the tile size for maps <= SoftLimitDimension.
	StandardTileSize = 48
	// LargeTileSize is the auto-downscaled tile size for maps > SoftLimitDimension.
	LargeTileSize = 32
)

// SizeCategory describes the map size tier.
type SizeCategory string

const (
	SizeCategoryStandard SizeCategory = "standard"
	SizeCategoryLarge    SizeCategory = "large"
)

// TilesetRef represents a reference to an external tileset file.
type TilesetRef struct {
	Name      string `json:"name"`
	SourceURL string `json:"source_url"`
	FirstGID  int    `json:"first_gid"`
}

// CreateMapInput holds the parameters for creating a map.
type CreateMapInput struct {
	CampaignID        uuid.UUID
	Name              string
	Width             int
	Height            int
	TiledJSON         json.RawMessage
	BackgroundImageID uuid.NullUUID
	TilesetRefs       []TilesetRef
}

// Store defines the database operations needed by the map service.
type Store interface {
	CreateMap(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error)
	GetMapByID(ctx context.Context, id uuid.UUID) (refdata.Map, error)
	ListMapsByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Map, error)
	UpdateMap(ctx context.Context, arg refdata.UpdateMapParams) (refdata.Map, error)
	DeleteMap(ctx context.Context, id uuid.UUID) error
}

// Service manages map CRUD and validation.
type Service struct {
	store Store
}

// NewService creates a new map Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateMap validates input and creates a new map.
// Returns the created map, the size category, and any error.
func (s *Service) CreateMap(ctx context.Context, input CreateMapInput) (refdata.Map, SizeCategory, error) {
	if err := validateMapFields(input.Name, input.Width, input.Height, input.TiledJSON); err != nil {
		return refdata.Map{}, "", err
	}

	category := classifySize(input.Width, input.Height)

	tilesetRefsJSON, err := marshalTilesetRefs(input.TilesetRefs)
	if err != nil {
		return refdata.Map{}, "", fmt.Errorf("marshaling tileset_refs: %w", err)
	}

	m, err := s.store.CreateMap(ctx, refdata.CreateMapParams{
		CampaignID:        input.CampaignID,
		Name:              input.Name,
		WidthSquares:      int32(input.Width),
		HeightSquares:     int32(input.Height),
		TiledJson:         input.TiledJSON,
		BackgroundImageID: input.BackgroundImageID,
		TilesetRefs:       tilesetRefsJSON,
	})
	if err != nil {
		return refdata.Map{}, "", fmt.Errorf("creating map: %w", err)
	}

	return m, category, nil
}

// GetByID retrieves a map by its ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
	return s.store.GetMapByID(ctx, id)
}

// ListByCampaignID lists all maps for a campaign.
func (s *Service) ListByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Map, error) {
	return s.store.ListMapsByCampaignID(ctx, campaignID)
}

// UpdateMapInput holds the parameters for updating a map.
type UpdateMapInput struct {
	ID                uuid.UUID
	Name              string
	Width             int
	Height            int
	TiledJSON         json.RawMessage
	BackgroundImageID uuid.NullUUID
	TilesetRefs       []TilesetRef
}

// UpdateMap validates input and updates an existing map.
func (s *Service) UpdateMap(ctx context.Context, input UpdateMapInput) (refdata.Map, SizeCategory, error) {
	if err := validateMapFields(input.Name, input.Width, input.Height, input.TiledJSON); err != nil {
		return refdata.Map{}, "", err
	}

	category := classifySize(input.Width, input.Height)

	tilesetRefsJSON, err := marshalTilesetRefs(input.TilesetRefs)
	if err != nil {
		return refdata.Map{}, "", fmt.Errorf("marshaling tileset_refs: %w", err)
	}

	m, err := s.store.UpdateMap(ctx, refdata.UpdateMapParams{
		ID:                input.ID,
		Name:              input.Name,
		WidthSquares:      int32(input.Width),
		HeightSquares:     int32(input.Height),
		TiledJson:         input.TiledJSON,
		BackgroundImageID: input.BackgroundImageID,
		TilesetRefs:       tilesetRefsJSON,
	})
	if err != nil {
		return refdata.Map{}, "", fmt.Errorf("updating map: %w", err)
	}

	return m, category, nil
}

// DeleteMap deletes a map by its ID.
func (s *Service) DeleteMap(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteMap(ctx, id)
}

// TileSizeForCategory returns the appropriate tile size for a size category.
func TileSizeForCategory(cat SizeCategory) int {
	if cat == SizeCategoryLarge {
		return LargeTileSize
	}
	return StandardTileSize
}

// validateMapFields checks name, dimensions, and tiled JSON.
func validateMapFields(name string, width, height int, tiledJSON json.RawMessage) error {
	if err := validateDimensions(width, height); err != nil {
		return err
	}
	if name == "" {
		return errors.New("name must not be empty")
	}
	if len(tiledJSON) == 0 {
		return errors.New("tiled_json must not be empty")
	}
	return nil
}

// validateDimensions checks that dimensions are within bounds.
func validateDimensions(width, height int) error {
	if width < 1 || height < 1 {
		return fmt.Errorf("dimensions must be positive (got %dx%d)", width, height)
	}
	if width > HardLimitDimension || height > HardLimitDimension {
		return fmt.Errorf("dimensions %dx%d exceeds hard limit of %dx%d", width, height, HardLimitDimension, HardLimitDimension)
	}
	return nil
}

// classifySize returns the size category for given dimensions.
func classifySize(width, height int) SizeCategory {
	if width > SoftLimitDimension || height > SoftLimitDimension {
		return SizeCategoryLarge
	}
	return SizeCategoryStandard
}

// marshalTilesetRefs marshals tileset refs to a NullRawMessage.
func marshalTilesetRefs(refs []TilesetRef) (pqtype.NullRawMessage, error) {
	if refs == nil {
		return pqtype.NullRawMessage{}, nil
	}
	data, err := json.Marshal(refs)
	if err != nil {
		return pqtype.NullRawMessage{}, err
	}
	return pqtype.NullRawMessage{RawMessage: data, Valid: true}, nil
}
