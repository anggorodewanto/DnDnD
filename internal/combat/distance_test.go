package combat

import (
	"testing"

	"github.com/ab/dndnd/internal/refdata"
)

func TestFormatDistance(t *testing.T) {
	got := FormatDistance(25, "You", "Goblin #1 (G1)")
	expected := "\U0001f4cf You are 25ft from Goblin #1 (G1)."
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatDistanceTwoTargets(t *testing.T) {
	got := FormatDistance(15, "Goblin #1 (G1)", "Aria (AR)")
	expected := "\U0001f4cf Goblin #1 (G1) is 15ft from Aria (AR)."
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatRangeRejection(t *testing.T) {
	got := FormatRangeRejection(65, 60)
	expected := "\u274c Target is out of range \u2014 65ft away (max 60ft)."
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestResolveTarget_MatchByCoordinate(t *testing.T) {
	combatants := []refdata.Combatant{
		{ShortID: "G1", DisplayName: "Goblin #1", PositionCol: "A", PositionRow: 1},
		{ShortID: "AR", DisplayName: "Aria", PositionCol: "D", PositionRow: 4},
	}

	got, err := ResolveTarget("D4", combatants)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ShortID != "AR" {
		t.Errorf("expected AR, got %s", got.ShortID)
	}
}

func TestResolveTarget_NotFound(t *testing.T) {
	combatants := []refdata.Combatant{
		{ShortID: "G1", DisplayName: "Goblin #1", PositionCol: "A", PositionRow: 1},
	}

	_, err := ResolveTarget("ZZ", combatants)
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestResolveTarget_MatchByShortID(t *testing.T) {
	combatants := []refdata.Combatant{
		{ShortID: "G1", DisplayName: "Goblin #1", PositionCol: "A", PositionRow: 1},
		{ShortID: "AR", DisplayName: "Aria", PositionCol: "D", PositionRow: 4},
	}

	got, err := ResolveTarget("g1", combatants)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ShortID != "G1" {
		t.Errorf("expected G1, got %s", got.ShortID)
	}
}
