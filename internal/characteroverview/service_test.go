package characteroverview

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

type fakeStore struct {
	sheets []CharacterSheet
	err    error
	called uuid.UUID
}

func (f *fakeStore) ListApprovedPartyCharacters(ctx context.Context, campaignID uuid.UUID) ([]CharacterSheet, error) {
	f.called = campaignID
	if f.err != nil {
		return nil, f.err
	}
	return f.sheets, nil
}

func TestPartyLanguages_Empty(t *testing.T) {
	svc := NewService(&fakeStore{})
	got := svc.PartyLanguages(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %+v", got)
	}
}

func TestPartyLanguages_SingleCharacter(t *testing.T) {
	svc := NewService(&fakeStore{})
	sheets := []CharacterSheet{
		{Name: "Aria", Languages: []string{"Common", "Elvish"}},
	}
	got := svc.PartyLanguages(sheets)
	want := []LanguageCoverage{
		{Language: "Common", Characters: []string{"Aria"}},
		{Language: "Elvish", Characters: []string{"Aria"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestPartyLanguages_OverlapSortedAndDeduped(t *testing.T) {
	svc := NewService(&fakeStore{})
	sheets := []CharacterSheet{
		{Name: "Aria", Languages: []string{"Elvish", "Common", "elvish"}},
		{Name: "Fenwick", Languages: []string{"Elvish", "Dwarvish"}},
		{Name: "Bree", Languages: []string{"Common"}},
	}
	got := svc.PartyLanguages(sheets)
	// Expect languages sorted alphabetically, character names sorted alphabetically,
	// dedup by case-insensitive comparison but preserve canonical casing from first occurrence.
	want := []LanguageCoverage{
		{Language: "Common", Characters: []string{"Aria", "Bree"}},
		{Language: "Dwarvish", Characters: []string{"Fenwick"}},
		{Language: "Elvish", Characters: []string{"Aria", "Fenwick"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestPartyLanguages_IgnoresBlankLanguages(t *testing.T) {
	svc := NewService(&fakeStore{})
	sheets := []CharacterSheet{
		{Name: "Aria", Languages: []string{"", "Common", "  "}},
	}
	got := svc.PartyLanguages(sheets)
	want := []LanguageCoverage{
		{Language: "Common", Characters: []string{"Aria"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestListPartyCharacters_Delegates(t *testing.T) {
	campID := uuid.New()
	sheets := []CharacterSheet{{Name: "Aria"}}
	store := &fakeStore{sheets: sheets}
	svc := NewService(store)

	got, err := svc.ListPartyCharacters(context.Background(), campID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.called != campID {
		t.Fatalf("expected campaign %s, got %s", campID, store.called)
	}
	if len(got) != 1 || got[0].Name != "Aria" {
		t.Fatalf("got %+v", got)
	}
}

func TestListPartyCharacters_StoreErrorPropagates(t *testing.T) {
	store := &fakeStore{err: errors.New("boom")}
	svc := NewService(store)
	_, err := svc.ListPartyCharacters(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListPartyCharacters_RejectsNilCampaign(t *testing.T) {
	svc := NewService(&fakeStore{})
	_, err := svc.ListPartyCharacters(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
