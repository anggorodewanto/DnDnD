package portal

import (
	"encoding/json"
	"testing"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// metaSub builds a single-class submission with metamagic picks.
func metaSub(class string, level int, metamagics ...string) CharacterSubmission {
	return CharacterSubmission{
		Name:      "Test " + class,
		Race:      "human",
		Class:     class,
		Classes:   []character.ClassEntry{{Class: class, Level: level, IsPrimary: true}},
		Skills:    []string{"arcana", "deception"},
		Metamagic: metamagics,
	}
}

func TestSubmissionSorcererLevel(t *testing.T) {
	cases := []struct {
		name string
		sub  CharacterSubmission
		want int
	}{
		{"single-class sorcerer", metaSub("sorcerer", 5), 5},
		{"non-sorcerer", metaSub("wizard", 10), 0},
		{"multiclass sorcerer3/fighter1", CharacterSubmission{
			Classes: []character.ClassEntry{
				{Class: "fighter", Level: 1, IsPrimary: true},
				{Class: "sorcerer", Level: 3},
			},
		}, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := submissionSorcererLevel(tc.sub); got != tc.want {
				t.Errorf("submissionSorcererLevel = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestMetamagicFeaturesForSubmission(t *testing.T) {
	t.Run("sorcerer L3 resolves two picks", func(t *testing.T) {
		out := metamagicFeaturesForSubmission(metaSub("sorcerer", 3, "quickened", "twinned"))
		if len(out) != 2 {
			t.Fatalf("want 2 features, got %d: %+v", len(out), out)
		}
		assertFeature(t, out, "Quickened Spell", "metamagic", "quickened")
		assertFeature(t, out, "Twinned Spell", "metamagic", "twinned")
	})
	t.Run("over-grant is capped", func(t *testing.T) {
		// L3 grants 2; a third pick is dropped defensively.
		out := metamagicFeaturesForSubmission(metaSub("sorcerer", 3, "quickened", "twinned", "subtle"))
		if len(out) != 2 {
			t.Errorf("want 2 (capped), got %d", len(out))
		}
	})
	t.Run("unknown id dropped", func(t *testing.T) {
		out := metamagicFeaturesForSubmission(metaSub("sorcerer", 3, "not_a_metamagic", "subtle"))
		if len(out) != 1 || out[0].MechanicalEffect != "subtle" {
			t.Errorf("unknown id should be dropped, got %+v", out)
		}
	})
	t.Run("duplicate dropped", func(t *testing.T) {
		out := metamagicFeaturesForSubmission(metaSub("sorcerer", 3, "subtle", "subtle"))
		if len(out) != 1 {
			t.Errorf("duplicate should collapse, got %d", len(out))
		}
	})
	t.Run("non-sorcerer resolves nothing", func(t *testing.T) {
		if out := metamagicFeaturesForSubmission(metaSub("wizard", 10, "quickened")); out != nil {
			t.Errorf("non-sorcerer should resolve no metamagic, got %+v", out)
		}
	})
	t.Run("sorcerer below L3 resolves nothing", func(t *testing.T) {
		if out := metamagicFeaturesForSubmission(metaSub("sorcerer", 2, "quickened")); out != nil {
			t.Errorf("sorcerer below L3 should resolve no metamagic, got %+v", out)
		}
	})
}

func TestValidateSubmittedMetamagic(t *testing.T) {
	cases := map[string]struct {
		sub     CharacterSubmission
		wantErr bool
	}{
		"legal two picks":         {metaSub("sorcerer", 3, "quickened", "twinned"), false},
		"empty is accepted":       {metaSub("sorcerer", 3), false},
		"unknown id":              {metaSub("sorcerer", 3, "not_a_metamagic"), true},
		"too many for level":      {metaSub("sorcerer", 3, "quickened", "twinned", "subtle"), true},
		"non-sorcerer with picks": {metaSub("wizard", 10, "quickened"), true},
		"sorcerer below L3":       {metaSub("sorcerer", 2, "quickened"), true},
		"duplicate":               {metaSub("sorcerer", 3, "subtle", "subtle"), true},
		"L10 allows three":        {metaSub("sorcerer", 10, "quickened", "twinned", "subtle"), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateSubmittedMetamagic(tc.sub)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateSubmittedMetamagic err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestInjectClassFeatureChoices_ResolvesMetamagic(t *testing.T) {
	base := []character.Feature{
		{Name: "Metamagic", Source: "sorcerer", Level: 3, MechanicalEffect: refdata.ChooseMetamagicEffect},
		{Name: "Font of Magic", Source: "sorcerer", Level: 2, MechanicalEffect: "sorcery_points_equal_sorcerer_level"},
	}
	sub := metaSub("sorcerer", 3, "quickened", "twinned")

	out := injectClassFeatureChoices(base, sub)

	for _, f := range out {
		if f.MechanicalEffect == refdata.ChooseMetamagicEffect {
			t.Error("metamagic placeholder should be removed once resolved")
		}
	}
	assertFeature(t, out, "Quickened Spell", "metamagic", "quickened")
	assertFeature(t, out, "Twinned Spell", "metamagic", "twinned")
	// Untouched base feature survives.
	assertFeature(t, out, "Font of Magic", "sorcerer", "sorcery_points_equal_sorcerer_level")
}

func TestInjectClassFeatureChoices_NoMetamagicPickLeavesPlaceholder(t *testing.T) {
	base := []character.Feature{
		{Name: "Metamagic", Source: "sorcerer", Level: 3, MechanicalEffect: refdata.ChooseMetamagicEffect},
	}
	sub := metaSub("sorcerer", 3) // skipped the pick
	out := injectClassFeatureChoices(base, sub)
	if len(out) != 1 || out[0].MechanicalEffect != refdata.ChooseMetamagicEffect {
		t.Errorf("placeholder must remain when no metamagic is picked: %+v", out)
	}
}

// The load-bearing end-to-end contract: a persisted metamagic feature must fire
// the combat cast gate's HasMetamagic reader (clean-slug match).
func TestInjectClassFeatureChoices_MetamagicFiresCombatGate(t *testing.T) {
	sub := metaSub("sorcerer", 3, "quickened", "twinned")
	out := injectClassFeatureChoices(nil, sub)

	blob, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal features: %v", err)
	}
	features := pqtype.NullRawMessage{RawMessage: blob, Valid: true}
	if !combat.HasMetamagic(features, "quickened") {
		t.Fatal("combat.HasMetamagic did not match the persisted quickened feature — clean-slug contract broken")
	}
	if combat.HasMetamagic(features, "subtle") {
		t.Fatal("combat.HasMetamagic matched a metamagic the sorcerer did not pick")
	}
}

func TestSubmissionFromCharacter_RestoresMetamagic(t *testing.T) {
	feats := []character.Feature{
		{Name: "Font of Magic", Source: "sorcerer", MechanicalEffect: "sorcery_points_equal_sorcerer_level"},
		{Name: "Quickened Spell", Source: "metamagic", MechanicalEffect: "quickened"},
		{Name: "Twinned Spell", Source: "metamagic", MechanicalEffect: "twinned"},
	}
	blob, _ := json.Marshal(feats)
	ch := refdata.Character{
		Name:     "Elara",
		Race:     "human",
		Features: pqtype.NullRawMessage{RawMessage: blob, Valid: true},
	}

	sub := submissionFromCharacter(ch)

	if len(sub.Metamagic) != 2 || sub.Metamagic[0] != "quickened" || sub.Metamagic[1] != "twinned" {
		t.Errorf("Metamagic = %v, want [quickened twinned]", sub.Metamagic)
	}
}
