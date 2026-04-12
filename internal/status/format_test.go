package status

import (
	"strings"
	"testing"
)

func TestFormatStatus_NoActiveState(t *testing.T) {
	info := Info{
		CharacterName: "Aria",
		ShortID:       "AR",
	}
	got := FormatStatus(info)
	want := "**Status — Aria (AR)**\n\nNo active effects."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatStatus_WithConditions(t *testing.T) {
	info := Info{
		CharacterName: "Aria",
		ShortID:       "AR",
		Conditions: []ConditionEntry{
			{Name: "Poisoned", RemainingRounds: 3},
			{Name: "Frightened", RemainingRounds: 2},
		},
	}
	got := FormatStatus(info)
	if strings.Contains(got, "No active effects") {
		t.Error("should not show 'No active effects' when conditions are present")
	}
	want := "**Conditions:** Poisoned (3 rounds remaining), Frightened (2 rounds remaining)"
	if !strings.Contains(got, want) {
		t.Errorf("got %q, want to contain %q", got, want)
	}
}

func TestFormatStatus_ConditionNoDuration(t *testing.T) {
	info := Info{
		CharacterName: "Aria",
		ShortID:       "AR",
		Conditions: []ConditionEntry{
			{Name: "Prone"},
		},
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Conditions:** Prone") {
		t.Errorf("got %q, want conditions with Prone (no duration)", got)
	}
	if strings.Contains(got, "rounds remaining") {
		t.Error("should not show rounds remaining for zero-duration condition")
	}
}

func TestFormatStatus_Concentration(t *testing.T) {
	info := Info{
		CharacterName: "Aria",
		ShortID:       "AR",
		Concentration: "Bless",
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Concentration:** Bless") {
		t.Errorf("got %q, want concentration line", got)
	}
}

func TestFormatStatus_TempHP(t *testing.T) {
	info := Info{
		CharacterName: "Aria",
		ShortID:       "AR",
		TempHP:        8,
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Temp HP:** 8") {
		t.Errorf("got %q, want temp hp line", got)
	}
}

func TestFormatStatus_Exhaustion(t *testing.T) {
	info := Info{
		CharacterName: "Aria",
		ShortID:       "AR",
		ExhaustionLevel: 2,
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Exhaustion:** Level 2") {
		t.Errorf("got %q, want exhaustion line", got)
	}
}

func TestFormatStatus_Rage(t *testing.T) {
	info := Info{
		CharacterName:       "Grog",
		ShortID:             "GR",
		IsRaging:            true,
		RageRoundsRemaining: 6,
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Rage:** Active (6 rounds remaining)") {
		t.Errorf("got %q, want rage line", got)
	}
}

func TestFormatStatus_RageNoRounds(t *testing.T) {
	info := Info{
		CharacterName: "Grog",
		ShortID:       "GR",
		IsRaging:      true,
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Rage:** Active") {
		t.Errorf("got %q, want rage active", got)
	}
}

func TestFormatStatus_WildShape(t *testing.T) {
	info := Info{
		CharacterName:     "Elara",
		ShortID:           "EL",
		IsWildShaped:      true,
		WildShapeCreature: "Dire Wolf",
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Wild Shape:** Dire Wolf") {
		t.Errorf("got %q, want wild shape line", got)
	}
}

func TestFormatStatus_BardicInspiration(t *testing.T) {
	info := Info{
		CharacterName:        "Aria",
		ShortID:              "AR",
		BardicInspirationDie: "d8",
		BardicInspirationSrc: "Melody",
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Bardic Inspiration:** d8 (from Melody)") {
		t.Errorf("got %q, want bardic inspiration line", got)
	}
}

func TestFormatStatus_Ki(t *testing.T) {
	info := Info{
		CharacterName: "Monk",
		ShortID:       "MK",
		HasKi:         true,
		KiCurrent:     3,
		KiMax:         5,
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Ki Points:** 3/5") {
		t.Errorf("got %q, want ki line", got)
	}
}

func TestFormatStatus_Sorcery(t *testing.T) {
	info := Info{
		CharacterName:  "Sorc",
		ShortID:        "SC",
		HasSorcery:     true,
		SorceryCurrent: 4,
		SorceryMax:     7,
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Sorcery Points:** 4/7") {
		t.Errorf("got %q, want sorcery line", got)
	}
}

func TestFormatStatus_Reactions(t *testing.T) {
	info := Info{
		CharacterName: "Aria",
		ShortID:       "AR",
		Reactions:     []string{"Shield if hit by ranged attack"},
	}
	got := FormatStatus(info)
	if !strings.Contains(got, `**Reaction Declarations:** "Shield if hit by ranged attack"`) {
		t.Errorf("got %q, want reactions line", got)
	}
}

func TestFormatStatus_ReadiedActions(t *testing.T) {
	info := Info{
		CharacterName:  "Aria",
		ShortID:        "AR",
		ReadiedActions: []string{"Attack goblin if it moves closer"},
	}
	got := FormatStatus(info)
	if !strings.Contains(got, `**Readied Actions:** "Attack goblin if it moves closer"`) {
		t.Errorf("got %q, want readied actions line", got)
	}
}

func TestFormatStatus_MultipleReactions(t *testing.T) {
	info := Info{
		CharacterName: "Aria",
		ShortID:       "AR",
		Reactions:     []string{"Shield if hit", "Counterspell"},
	}
	got := FormatStatus(info)
	if !strings.Contains(got, `"Shield if hit"`) || !strings.Contains(got, `"Counterspell"`) {
		t.Errorf("got %q, want both reactions", got)
	}
}

func TestFormatStatus_AllSections(t *testing.T) {
	info := Info{
		CharacterName: "Aria",
		ShortID:       "AR",
		Conditions: []ConditionEntry{
			{Name: "Poisoned", RemainingRounds: 3},
		},
		Concentration:        "Bless",
		TempHP:               8,
		ExhaustionLevel:      2,
		IsRaging:             true,
		RageRoundsRemaining:  6,
		IsWildShaped:         true,
		WildShapeCreature:    "Dire Wolf",
		BardicInspirationDie: "d8",
		BardicInspirationSrc: "Melody",
		HasKi:                true,
		KiCurrent:            3,
		KiMax:                5,
		HasSorcery:           true,
		SorceryCurrent:       4,
		SorceryMax:           7,
		Reactions:            []string{"Shield if hit"},
		ReadiedActions:       []string{"Attack if goblin moves"},
	}
	got := FormatStatus(info)
	header := "**Status — Aria (AR)**"
	if !strings.HasPrefix(got, header) {
		t.Errorf("missing header, got: %q", got)
	}
	if strings.Contains(got, "No active effects") {
		t.Error("should not show 'No active effects' when sections are present")
	}
}

func TestFormatStatus_NoShortID(t *testing.T) {
	info := Info{
		CharacterName: "Aria",
		Concentration: "Bless",
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Status — Aria**") {
		t.Errorf("got %q, want header without short ID", got)
	}
	if strings.Contains(got, "()") {
		t.Error("should not show empty parentheses")
	}
}

func TestFormatStatus_WildShapeNoCreature(t *testing.T) {
	info := Info{
		CharacterName: "Elara",
		ShortID:       "EL",
		IsWildShaped:  true,
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Wild Shape:** Active") {
		t.Errorf("got %q, want wild shape active without creature", got)
	}
}

func TestFormatStatus_BardicInspirationNoSource(t *testing.T) {
	info := Info{
		CharacterName:        "Aria",
		ShortID:              "AR",
		BardicInspirationDie: "d6",
	}
	got := FormatStatus(info)
	if !strings.Contains(got, "**Bardic Inspiration:** d6") {
		t.Errorf("got %q, want bardic inspiration without source", got)
	}
	if strings.Contains(got, "(from") {
		t.Error("should not show 'from' when source is empty")
	}
}

func TestFormatStatus_OmitsEmptySections(t *testing.T) {
	// Only concentration set — no rage, no ki, etc.
	info := Info{
		CharacterName: "Aria",
		ShortID:       "AR",
		Concentration: "Bless",
	}
	got := FormatStatus(info)
	if strings.Contains(got, "Rage") {
		t.Error("should omit Rage section when not raging")
	}
	if strings.Contains(got, "Ki") {
		t.Error("should omit Ki section when character has no ki")
	}
	if strings.Contains(got, "Wild Shape") {
		t.Error("should omit Wild Shape section when not shifted")
	}
}
