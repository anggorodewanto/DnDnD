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

func TestResolveTargetPos_BareCoordinate(t *testing.T) {
	combatants := []refdata.Combatant{
		{ShortID: "G1", DisplayName: "Goblin #1", PositionCol: "A", PositionRow: 1},
	}

	// "f6" is an empty tile — no combatant stands there.
	pos, err := ResolveTargetPos("f6", combatants)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos.Col != 5 || pos.Row != 5 {
		t.Errorf("expected col=5,row=5, got col=%d,row=%d", pos.Col, pos.Row)
	}
	if pos.AltFt != 0 {
		t.Errorf("expected ground altitude 0, got %d", pos.AltFt)
	}
	if pos.Label != "F6" {
		t.Errorf("expected label F6, got %q", pos.Label)
	}
}

func TestResolveTargetPos_Combatant(t *testing.T) {
	combatants := []refdata.Combatant{
		{ShortID: "AR", DisplayName: "Aria", PositionCol: "D", PositionRow: 4, AltitudeFt: 30},
	}

	pos, err := ResolveTargetPos("AR", combatants)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos.Col != 3 || pos.Row != 3 {
		t.Errorf("expected D4 col=3,row=3, got col=%d,row=%d", pos.Col, pos.Row)
	}
	if pos.AltFt != 30 {
		t.Errorf("expected altitude 30, got %d", pos.AltFt)
	}
	if pos.Label != "Aria (AR)" {
		t.Errorf("expected label 'Aria (AR)', got %q", pos.Label)
	}
}

func TestResolveTargetPos_OccupiedCoordinatePrefersCombatant(t *testing.T) {
	combatants := []refdata.Combatant{
		{ShortID: "AR", DisplayName: "Aria", PositionCol: "D", PositionRow: 4, AltitudeFt: 10},
	}

	pos, err := ResolveTargetPos("D4", combatants)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos.Label != "Aria (AR)" {
		t.Errorf("expected combatant label for occupied tile, got %q", pos.Label)
	}
	if pos.AltFt != 10 {
		t.Errorf("expected combatant altitude 10, got %d", pos.AltFt)
	}
}

func TestResolveTargetPos_Invalid(t *testing.T) {
	_, err := ResolveTargetPos("@@", nil)
	if err == nil {
		t.Fatal("expected error for invalid target")
	}
}

func TestDistanceBetween(t *testing.T) {
	a := TargetPos{Col: 0, Row: 0, AltFt: 0}
	b := TargetPos{Col: 5, Row: 0, AltFt: 0}
	if got := DistanceBetween(a, b); got != 25 {
		t.Errorf("expected 25ft, got %d", got)
	}
}
