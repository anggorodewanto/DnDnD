package portal

import (
	"encoding/json"
	"testing"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// styleSub builds a single-class submission for a fighting-style-granting class.
func styleSub(class string, level int, style string) CharacterSubmission {
	return CharacterSubmission{
		Name:          "Test " + class,
		Race:          "human",
		Class:         class,
		Classes:       []character.ClassEntry{{Class: class, Level: level, IsPrimary: true}},
		Skills:        []string{"athletics", "perception"},
		FightingStyle: style,
	}
}

func TestSubmissionFightingStyleGrant(t *testing.T) {
	cases := []struct {
		name      string
		sub       CharacterSubmission
		wantLevel int
		wantOK    bool
	}{
		{"fighter L1 grants at 1", styleSub("fighter", 1, ""), 1, true},
		{"paladin L1 below grant", styleSub("paladin", 1, ""), 0, false},
		{"paladin L2 grants at 2", styleSub("paladin", 2, ""), 2, true},
		{"ranger L2 grants at 2", styleSub("ranger", 2, ""), 2, true},
		{"wizard never grants", styleSub("wizard", 20, ""), 0, false},
		{"multiclass fighter1/wizard grants", CharacterSubmission{
			Classes: []character.ClassEntry{
				{Class: "wizard", Level: 5, IsPrimary: true},
				{Class: "fighter", Level: 1},
			},
		}, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			level, ok := submissionFightingStyleGrant(tc.sub)
			if ok != tc.wantOK || level != tc.wantLevel {
				t.Errorf("submissionFightingStyleGrant = (%d,%v), want (%d,%v)", level, ok, tc.wantLevel, tc.wantOK)
			}
		})
	}
}

func TestValidateSubmittedFightingStyle(t *testing.T) {
	cases := map[string]struct {
		sub     CharacterSubmission
		wantErr bool
	}{
		"legal fighter archery":       {styleSub("fighter", 1, "archery"), false},
		"legal paladin L2 defense":    {styleSub("paladin", 2, "defense"), false},
		"empty is always accepted":    {styleSub("wizard", 5, ""), false},
		"unknown style id":            {styleSub("fighter", 1, "not_a_style"), true},
		"style on non-granting class": {styleSub("wizard", 5, "archery"), true},
		"style on paladin below L2":   {styleSub("paladin", 1, "archery"), true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateSubmittedFightingStyle(tc.sub)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateSubmittedFightingStyle err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestInjectClassFeatureChoices_ResolvesFightingStyle(t *testing.T) {
	base := []character.Feature{
		{Name: "Fighting Style", Source: "fighter", Level: 1, MechanicalEffect: refdata.ChooseFightingStyleEffect},
		{Name: "Second Wind", Source: "fighter", Level: 1, MechanicalEffect: "bonus_action_heal_1d10_plus_level"},
	}
	sub := styleSub("fighter", 1, "archery")

	out := injectClassFeatureChoices(base, sub)

	for _, f := range out {
		if f.MechanicalEffect == refdata.ChooseFightingStyleEffect {
			t.Error("fighting-style placeholder should be removed once resolved")
		}
	}
	assertFeature(t, out, "Archery", "fighting_style", "archery")
	// Untouched base feature survives.
	assertFeature(t, out, "Second Wind", "fighter", "bonus_action_heal_1d10_plus_level")
}

func TestInjectClassFeatureChoices_NoFightingStylePickLeavesPlaceholder(t *testing.T) {
	base := []character.Feature{
		{Name: "Fighting Style", Source: "fighter", Level: 1, MechanicalEffect: refdata.ChooseFightingStyleEffect},
	}
	sub := styleSub("fighter", 1, "") // skipped the pick
	out := injectClassFeatureChoices(base, sub)
	if len(out) != 1 || out[0].MechanicalEffect != refdata.ChooseFightingStyleEffect {
		t.Errorf("placeholder must remain when no style is picked: %+v", out)
	}
}

// The load-bearing end-to-end contract: a persisted fighting-style feature must
// fire the combat engine's HasFightingStyle reader (clean-slug match).
func TestInjectClassFeatureChoices_FightingStyleFiresCombatReader(t *testing.T) {
	sub := styleSub("fighter", 1, "two_weapon_fighting")
	out := injectClassFeatureChoices(nil, sub)

	blob, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal features: %v", err)
	}
	features := pqtype.NullRawMessage{RawMessage: blob, Valid: true}
	if !combat.HasFightingStyle(features, "two_weapon_fighting") {
		t.Fatal("combat.HasFightingStyle did not match the persisted two_weapon_fighting feature — clean-slug contract broken")
	}
}

func TestSubmissionFromCharacter_RestoresFightingStyle(t *testing.T) {
	feats := []character.Feature{
		{Name: "Second Wind", Source: "fighter", MechanicalEffect: "bonus_action_heal_1d10_plus_level"},
		{Name: "Dueling", Source: "fighting_style", MechanicalEffect: "dueling"},
	}
	blob, _ := json.Marshal(feats)
	ch := refdata.Character{
		Name:     "Bree",
		Race:     "human",
		Features: pqtype.NullRawMessage{RawMessage: blob, Valid: true},
	}

	sub := submissionFromCharacter(ch)

	if sub.FightingStyle != "dueling" {
		t.Errorf("FightingStyle = %q, want dueling", sub.FightingStyle)
	}
}
