package narration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

type stubAssets struct {
	get refdata.Asset
	err error
	url string
}

func (s *stubAssets) GetByID(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
	return s.get, s.err
}

func (s *stubAssets) URL(id uuid.UUID) string { return s.url }

func TestAssetAttachmentResolver_Found(t *testing.T) {
	id := uuid.New()
	r := NewAssetAttachmentResolver(&stubAssets{get: refdata.Asset{ID: id}, url: "/api/assets/" + id.String()})
	url, ok := r.AttachmentURL(id)
	if !ok || url != "/api/assets/"+id.String() {
		t.Fatalf("url=%q ok=%v", url, ok)
	}
}

func TestAssetAttachmentResolver_NotFound(t *testing.T) {
	r := NewAssetAttachmentResolver(&stubAssets{err: sql.ErrNoRows})
	_, ok := r.AttachmentURL(uuid.New())
	if ok {
		t.Fatalf("expected not found")
	}
}

func TestAssetAttachmentResolver_OtherError(t *testing.T) {
	r := NewAssetAttachmentResolver(&stubAssets{err: errors.New("boom")})
	_, ok := r.AttachmentURL(uuid.New())
	if ok {
		t.Fatalf("expected not ok on error")
	}
}

type stubCampaigns struct {
	got refdata.Campaign
	err error
}

func (s *stubCampaigns) GetByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	return s.got, s.err
}

func TestCampaignResolverAdapter_Success(t *testing.T) {
	a := NewCampaignResolverAdapter(&stubCampaigns{got: refdata.Campaign{GuildID: "g-42"}})
	g, err := a.GuildIDForCampaign(context.Background(), uuid.New())
	if err != nil || g != "g-42" {
		t.Fatalf("g=%q err=%v", g, err)
	}
}

func TestCampaignResolverAdapter_Error(t *testing.T) {
	a := NewCampaignResolverAdapter(&stubCampaigns{err: errors.New("nope")})
	_, err := a.GuildIDForCampaign(context.Background(), uuid.New())
	if err == nil {
		t.Fatalf("expected error")
	}
}
