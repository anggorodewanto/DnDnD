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

	// status-edit fakes
	char      refdata.Character
	charErr   error
	combatant refdata.Combatant
	combErr   error
	vitalsArg refdata.UpdateCharacterVitalsParams
	vitalsErr error
}

func (f *fakeRefdata) ListPlayerCharactersByStatus(ctx context.Context, arg refdata.ListPlayerCharactersByStatusParams) ([]refdata.ListPlayerCharactersByStatusRow, error) {
	f.arg = arg
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

func (f *fakeRefdata) GetCharacter(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
	return f.char, f.charErr
}

func (f *fakeRefdata) GetActiveCombatantByCharacterID(_ context.Context, _ uuid.NullUUID) (refdata.Combatant, error) {
	// Default to "not in combat" unless a test configures otherwise.
	if f.combErr == nil && f.combatant.ID == uuid.Nil {
		return refdata.Combatant{}, sql.ErrNoRows
	}
	return f.combatant, f.combErr
}

func (f *fakeRefdata) UpdateCharacterVitals(_ context.Context, arg refdata.UpdateCharacterVitalsParams) (refdata.Character, error) {
	f.vitalsArg = arg
	return refdata.Character{}, f.vitalsErr
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

func TestDBStore_ListApprovedPartyCharacters_OverlaysLiveCombatHP(t *testing.T) {
	charID := uuid.New()
	fake := &fakeRefdata{
		rows: []refdata.ListPlayerCharactersByStatusRow{
			{CharacterID: charID, CharacterName: "Aria", HpMax: 24, HpCurrent: 24, TempHp: 0},
		},
		// Live combat snapshot: bloodied to 19/24 with 5 temp HP.
		combatant: refdata.Combatant{ID: uuid.New(), HpMax: 24, HpCurrent: 19, TempHp: 5},
	}
	store := NewDBStore(fake)

	got, err := store.ListApprovedPartyCharacters(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got[0].HPCurrent != 19 || got[0].HPMax != 24 || got[0].TempHP != 5 {
		t.Fatalf("expected live combat HP 19/24 (+5 temp), got %d/%d (+%d temp)",
			got[0].HPCurrent, got[0].HPMax, got[0].TempHP)
	}
}

func TestDBStore_ListApprovedPartyCharacters_NoCombatKeepsRowHP(t *testing.T) {
	fake := &fakeRefdata{
		rows: []refdata.ListPlayerCharactersByStatusRow{
			{CharacterID: uuid.New(), CharacterName: "Bree", HpMax: 24, HpCurrent: 24, TempHp: 0},
		},
		// combatant left zero -> fake returns sql.ErrNoRows -> out of combat.
	}
	store := NewDBStore(fake)

	got, err := store.ListApprovedPartyCharacters(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got[0].HPCurrent != 24 || got[0].HPMax != 24 || got[0].TempHP != 0 {
		t.Fatalf("expected character-row HP 24/24 (+0 temp), got %d/%d (+%d temp)",
			got[0].HPCurrent, got[0].HPMax, got[0].TempHP)
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
