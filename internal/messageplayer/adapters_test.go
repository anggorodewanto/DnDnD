package messageplayer

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

type fakeLookupRefdata struct {
	id     uuid.UUID
	result refdata.PlayerCharacter
	err    error
}

func (f *fakeLookupRefdata) GetPlayerCharacter(ctx context.Context, id uuid.UUID) (refdata.PlayerCharacter, error) {
	f.id = id
	return f.result, f.err
}

func TestPlayerLookupAdapter_Success(t *testing.T) {
	pcID := uuid.New()
	campID := uuid.New()
	fake := &fakeLookupRefdata{result: refdata.PlayerCharacter{
		ID:            pcID,
		CampaignID:    campID,
		DiscordUserID: "user-42",
	}}
	adapter := NewPlayerLookupAdapter(fake)
	info, err := adapter.LookupPlayer(context.Background(), pcID)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if info.DiscordUserID != "user-42" || info.CampaignID != campID {
		t.Fatalf("info = %+v", info)
	}
	if fake.id != pcID {
		t.Fatalf("expected lookup id %s, got %s", pcID, fake.id)
	}
}

func TestPlayerLookupAdapter_NotFound(t *testing.T) {
	fake := &fakeLookupRefdata{err: sql.ErrNoRows}
	adapter := NewPlayerLookupAdapter(fake)
	_, err := adapter.LookupPlayer(context.Background(), uuid.New())
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestPlayerLookupAdapter_OtherError(t *testing.T) {
	fake := &fakeLookupRefdata{err: errors.New("boom")}
	adapter := NewPlayerLookupAdapter(fake)
	_, err := adapter.LookupPlayer(context.Background(), uuid.New())
	if err == nil || errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
