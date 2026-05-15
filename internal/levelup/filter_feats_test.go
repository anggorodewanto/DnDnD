package levelup

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
)

func TestFilterEligibleFeats_ExcludesOwnedAndUnmetPrereqs(t *testing.T) {
	allFeats := []FeatInfo{
		{ID: "alert", Name: "Alert"},
		{ID: "grappler", Name: "Grappler", Prerequisites: FeatPrerequisites{Ability: map[string]int{"str": 13}}},
		{ID: "war-caster", Name: "War Caster", Prerequisites: FeatPrerequisites{Spellcasting: true}},
		{ID: "owned-feat", Name: "Owned Feat"},
	}
	scores := character.AbilityScores{STR: 10, DEX: 14, CON: 12, INT: 16, WIS: 12, CHA: 8}
	ownedFeatIDs := []string{"owned-feat"}

	got := FilterEligibleFeats(allFeats, scores, nil, false, ownedFeatIDs)

	// Should only include "alert": grappler needs STR 13, war-caster needs spellcasting, owned-feat is already owned
	if len(got) != 1 {
		t.Fatalf("got %d feats, want 1: %+v", len(got), got)
	}
	if got[0].ID != "alert" {
		t.Errorf("got feat ID %q, want %q", got[0].ID, "alert")
	}
}
