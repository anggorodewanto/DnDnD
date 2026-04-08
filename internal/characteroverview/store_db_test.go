package characteroverview

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

type fakeRefdata struct {
	arg  refdata.ListPlayerCharactersByStatusParams
	rows []refdata.ListPlayerCharactersByStatusRow
	err  error
}

func (f *fakeRefdata) ListPlayerCharactersByStatus(ctx context.Context, arg refdata.ListPlayerCharactersByStatusParams) ([]refdata.ListPlayerCharactersByStatusRow, error) {
	f.arg = arg
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

func TestDBStore_ListApprovedPartyCharacters_MapsRows(t *testing.T) {
	campID := uuid.New()
	pcID := uuid.New()
	charID := uuid.New()
	classesJSON := json.RawMessage(`[{"name":"Wizard","level":3}]`)
	abilityJSON := json.RawMessage(`{"str":8}`)
	fake := &fakeRefdata{rows: []refdata.ListPlayerCharactersByStatusRow{
		{
			ID:            pcID,
			CampaignID:    campID,
			CharacterID:   charID,
			DiscordUserID: "user-1",
			CharacterName: "Aria",
			Race:          "Elf",
			Level:         3,
			Classes:       classesJSON,
			HpMax:         22,
			HpCurrent:     18,
			Ac:            13,
			SpeedFt:       30,
			AbilityScores: abilityJSON,
			Languages:     []string{"Common", "Elvish"},
			DdbUrl:        sql.NullString{String: "http://ddb/1", Valid: true},
		},
	}}
	store := NewDBStore(fake)

	got, err := store.ListApprovedPartyCharacters(context.Background(), campID)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if fake.arg.CampaignID != campID || fake.arg.Status != "approved" {
		t.Fatalf("wrong args: %+v", fake.arg)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	g := got[0]
	if g.PlayerCharacterID != pcID || g.CharacterID != charID {
		t.Fatalf("ids wrong: %+v", g)
	}
	if g.Name != "Aria" || g.Race != "Elf" || g.Level != 3 {
		t.Fatalf("fields wrong: %+v", g)
	}
	if g.DDBURL != "http://ddb/1" {
		t.Fatalf("ddb_url = %q", g.DDBURL)
	}
	if len(g.Languages) != 2 || g.Languages[0] != "Common" {
		t.Fatalf("languages = %+v", g.Languages)
	}
	if string(g.Classes) != string(classesJSON) {
		t.Fatalf("classes = %s", g.Classes)
	}
}

func TestDBStore_ListApprovedPartyCharacters_NullDDBURL(t *testing.T) {
	fake := &fakeRefdata{rows: []refdata.ListPlayerCharactersByStatusRow{
		{CharacterName: "Bree", DdbUrl: sql.NullString{Valid: false}},
	}}
	store := NewDBStore(fake)
	got, err := store.ListApprovedPartyCharacters(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got[0].DDBURL != "" {
		t.Fatalf("expected empty ddb_url, got %q", got[0].DDBURL)
	}
}

func TestDBStore_ListApprovedPartyCharacters_ErrorPropagates(t *testing.T) {
	fake := &fakeRefdata{err: errors.New("boom")}
	store := NewDBStore(fake)
	_, err := store.ListApprovedPartyCharacters(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}
