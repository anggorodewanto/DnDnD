package messageplayer

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// fakeLookupRefdata fakes the refdata surface the adapter resolves a
// character_id through: characters → campaign, then the player_characters row
// for (campaign, character).
type fakeLookupRefdata struct {
	char    refdata.Character
	charErr error
	pc      refdata.PlayerCharacter
	pcErr   error

	gotCharID    uuid.UUID
	gotByCharArg refdata.GetPlayerCharacterByCharacterParams
}

func (f *fakeLookupRefdata) GetCharacter(_ context.Context, id uuid.UUID) (refdata.Character, error) {
	f.gotCharID = id
	return f.char, f.charErr
}

func (f *fakeLookupRefdata) GetPlayerCharacterByCharacter(_ context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error) {
	f.gotByCharArg = arg
	return f.pc, f.pcErr
}

// LookupPlayer resolves by character_id (what the dashboard dropdown sends),
// not the player_characters PK — the live bug that returned "player character
// not found" for every /message-player send.
func TestPlayerLookupAdapter_Success(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()
	pcRowID := uuid.New()
	fake := &fakeLookupRefdata{
		char: refdata.Character{ID: charID, CampaignID: campID},
		pc: refdata.PlayerCharacter{
			ID:            pcRowID,
			CampaignID:    campID,
			CharacterID:   charID,
			DiscordUserID: "user-42",
		},
	}
	adapter := NewPlayerLookupAdapter(fake)
	info, err := adapter.LookupPlayer(context.Background(), charID)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if info.DiscordUserID != "user-42" || info.CampaignID != campID {
		t.Fatalf("info = %+v", info)
	}
	// Regression guard for the FK-violation bug: RowID must be the
	// player_characters PK (the dm_player_messages FK target), not left nil.
	// A nil RowID makes InsertDMMessage 500 on
	// dm_player_messages_player_character_id_fkey even though the DM was sent.
	if info.RowID != pcRowID {
		t.Fatalf("RowID = %s, want player_characters PK %s", info.RowID, pcRowID)
	}
	// Regression guard: the player_characters row is resolved by character_id
	// (+ that character's campaign), not by the PK passed in.
	if fake.gotCharID != charID {
		t.Fatalf("expected GetCharacter(%s), got %s", charID, fake.gotCharID)
	}
	if fake.gotByCharArg.CharacterID != charID || fake.gotByCharArg.CampaignID != campID {
		t.Fatalf("expected lookup by (campaign %s, character %s), got %+v", campID, charID, fake.gotByCharArg)
	}
}

func TestPlayerLookupAdapter_CharacterNotFound(t *testing.T) {
	fake := &fakeLookupRefdata{charErr: sql.ErrNoRows}
	adapter := NewPlayerLookupAdapter(fake)
	_, err := adapter.LookupPlayer(context.Background(), uuid.New())
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestPlayerLookupAdapter_NoPlayerCharacterRow(t *testing.T) {
	charID := uuid.New()
	fake := &fakeLookupRefdata{
		char:  refdata.Character{ID: charID, CampaignID: uuid.New()},
		pcErr: sql.ErrNoRows,
	}
	adapter := NewPlayerLookupAdapter(fake)
	_, err := adapter.LookupPlayer(context.Background(), charID)
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestPlayerLookupAdapter_OtherError(t *testing.T) {
	fake := &fakeLookupRefdata{charErr: errors.New("boom")}
	adapter := NewPlayerLookupAdapter(fake)
	_, err := adapter.LookupPlayer(context.Background(), uuid.New())
	if err == nil || errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
