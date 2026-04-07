package narration

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// AssetLookup is the minimal interface implemented by *asset.Service that
// AssetAttachmentResolver depends on. It lives here so narration does not
// import the asset package directly (and to keep test doubles trivial).
type AssetLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (refdata.Asset, error)
	URL(assetID uuid.UUID) string
}

// AssetAttachmentResolver adapts an AssetLookup into an AttachmentResolver.
// It verifies the asset row exists before returning its URL so that a caller
// cannot reference a deleted asset.
type AssetAttachmentResolver struct {
	assets AssetLookup
}

// NewAssetAttachmentResolver constructs an AssetAttachmentResolver.
func NewAssetAttachmentResolver(assets AssetLookup) *AssetAttachmentResolver {
	return &AssetAttachmentResolver{assets: assets}
}

// AttachmentURL returns the URL for an attachment asset, or ok=false if the
// asset does not exist.
func (r *AssetAttachmentResolver) AttachmentURL(assetID uuid.UUID) (string, bool) {
	if _, err := r.assets.GetByID(context.Background(), assetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false
		}
		return "", false
	}
	return r.assets.URL(assetID), true
}

// CampaignLookup is the minimal interface implemented by *campaign.Service
// that CampaignResolverAdapter depends on.
type CampaignLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
}

// CampaignResolverAdapter adapts a CampaignLookup into a CampaignResolver.
type CampaignResolverAdapter struct {
	campaigns CampaignLookup
}

// NewCampaignResolverAdapter constructs a CampaignResolverAdapter.
func NewCampaignResolverAdapter(campaigns CampaignLookup) *CampaignResolverAdapter {
	return &CampaignResolverAdapter{campaigns: campaigns}
}

// GuildIDForCampaign returns the Discord guild ID associated with a campaign.
func (a *CampaignResolverAdapter) GuildIDForCampaign(ctx context.Context, id uuid.UUID) (string, error) {
	c, err := a.campaigns.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return c.GuildID, nil
}
