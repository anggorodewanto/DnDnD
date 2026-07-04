package portal

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// warlockSub builds a submission for a single-class warlock at the given level
// with the given chosen spells / boon / invocations.
func warlockSub(level int, spells []string, boon string, invocations []string) CharacterSubmission {
	return CharacterSubmission{
		Name:        "Test Warlock",
		Race:        "human",
		Class:       "warlock",
		Subclass:    "fiend",
		Classes:     []character.ClassEntry{{Class: "warlock", Subclass: "fiend", Level: level, IsPrimary: true}},
		Skills:      []string{"arcana", "deception"},
		Spells:      spells,
		PactBoon:    boon,
		Invocations: invocations,
	}
}

func TestValidateSubmittedClassFeatures_AcceptsLegalWarlock(t *testing.T) {
	// Vale-like: warlock 4, knows eldritch blast, takes Agonizing Blast.
	sub := warlockSub(4, []string{"eldritch-blast"}, "", []string{"agonizing_blast"})
	if err := validateSubmittedClassFeatures(sub); err != nil {
		t.Fatalf("legal warlock rejected: %v", err)
	}
}

func TestValidateSubmittedClassFeatures_RejectsInvocationOnNonWarlock(t *testing.T) {
	sub := CharacterSubmission{
		Name:        "Fighter",
		Class:       "fighter",
		Classes:     []character.ClassEntry{{Class: "fighter", Level: 4, IsPrimary: true}},
		Invocations: []string{"devils_sight"},
	}
	if err := validateSubmittedClassFeatures(sub); err == nil {
		t.Fatal("expected error: a non-warlock cannot learn invocations")
	}
}

func TestValidateSubmittedClassFeatures_RejectsTooMany(t *testing.T) {
	// Warlock 2 knows 2 invocations; 3 is illegal.
	sub := warlockSub(2, []string{"eldritch-blast"}, "", []string{"agonizing_blast", "devils_sight", "beguiling_influence"})
	if err := validateSubmittedClassFeatures(sub); err == nil {
		t.Fatal("expected error: too many invocations for warlock level 2")
	}
}

func TestValidateSubmittedClassFeatures_RejectsUnmetPrereqs(t *testing.T) {
	cases := map[string]CharacterSubmission{
		"agonizing blast without eldritch blast": warlockSub(4, []string{"hex"}, "", []string{"agonizing_blast"}),
		"thirsting blade below level 5":          warlockSub(4, nil, "pact_of_the_blade", []string{"thirsting_blade"}),
		"book of ancient secrets w/o tome boon":  warlockSub(4, nil, "pact_of_the_blade", []string{"book_of_ancient_secrets"}),
		"pact boon before level 3":               warlockSub(2, nil, "pact_of_the_blade", nil),
		"unknown invocation":                     warlockSub(4, nil, "", []string{"not_a_real_invocation"}),
		"unknown pact boon":                      warlockSub(4, nil, "pact_of_nothing", nil),
		"duplicate invocation":                   warlockSub(4, nil, "", []string{"devils_sight", "devils_sight"}),
	}
	for name, sub := range cases {
		if err := validateSubmittedClassFeatures(sub); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}

func TestValidateSubmittedClassFeatures_AcceptsPactGatedWhenMet(t *testing.T) {
	// Warlock 5 with Pact of the Blade may take Thirsting Blade.
	sub := warlockSub(5, nil, "pact_of_the_blade", []string{"thirsting_blade"})
	if err := validateSubmittedClassFeatures(sub); err != nil {
		t.Fatalf("legal pact-gated invocation rejected: %v", err)
	}
}

func TestInjectClassFeatureChoices_ResolvesAgonizingBlastAndReplacesPlaceholders(t *testing.T) {
	base := []character.Feature{
		{Name: "Pact Magic", Source: "warlock", Level: 1, MechanicalEffect: "pact_magic_cha"},
		{Name: "Eldritch Invocations", Source: "warlock", Level: 2, MechanicalEffect: "choose_2_eldritch_invocations"},
		{Name: "Pact Boon", Source: "warlock", Level: 3, MechanicalEffect: "choose_pact_boon"},
	}
	sub := warlockSub(4, []string{"eldritch-blast"}, "pact_of_the_blade", []string{"agonizing_blast"})

	out := injectClassFeatureChoices(base, sub)

	// Placeholders resolved => removed.
	for _, f := range out {
		if f.MechanicalEffect == "choose_2_eldritch_invocations" {
			t.Error("invocation placeholder should be removed once resolved")
		}
		if f.MechanicalEffect == "choose_pact_boon" {
			t.Error("pact-boon placeholder should be removed once resolved")
		}
	}
	// Concrete features present with clean slugs.
	assertFeature(t, out, "Agonizing Blast", "invocation", "agonizing_blast")
	assertFeature(t, out, "Pact of the Blade", "pact_boon", "pact_of_the_blade")
	// Untouched base feature survives.
	assertFeature(t, out, "Pact Magic", "warlock", "pact_magic_cha")
}

func TestInjectClassFeatureChoices_NoPicksLeavesBaseUnchanged(t *testing.T) {
	base := []character.Feature{
		{Name: "Eldritch Invocations", Source: "warlock", Level: 2, MechanicalEffect: "choose_2_eldritch_invocations"},
	}
	// Warlock who skipped the step: placeholders must remain.
	sub := warlockSub(4, []string{"eldritch-blast"}, "", nil)
	out := injectClassFeatureChoices(base, sub)
	if len(out) != 1 || out[0].MechanicalEffect != "choose_2_eldritch_invocations" {
		t.Errorf("base should be unchanged when no picks: %+v", out)
	}
}

// The load-bearing end-to-end contract: a persisted invocation feature must
// fire the combat engine's Agonizing Blast reader (clean-slug match).
func TestInjectClassFeatureChoices_FiresCombatReader(t *testing.T) {
	sub := warlockSub(4, []string{"eldritch-blast"}, "", []string{"agonizing_blast"})
	out := injectClassFeatureChoices(nil, sub)

	blob, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal features: %v", err)
	}
	features := pqtype.NullRawMessage{RawMessage: blob, Valid: true}
	if !combat.HasInvocation(features, "agonizing_blast") {
		t.Fatal("combat.HasInvocation did not match the persisted agonizing_blast feature — clean-slug contract broken")
	}
}

func TestSubmissionFromCharacter_RestoresInvocationsAndBoon(t *testing.T) {
	feats := []character.Feature{
		{Name: "Pact Magic", Source: "warlock", MechanicalEffect: "pact_magic_cha"},
		{Name: "Agonizing Blast", Source: "invocation", MechanicalEffect: "agonizing_blast"},
		{Name: "Devil's Sight", Source: "invocation", MechanicalEffect: "devils_sight"},
		{Name: "Pact of the Blade", Source: "pact_boon", MechanicalEffect: "pact_of_the_blade"},
	}
	blob, _ := json.Marshal(feats)
	ch := refdata.Character{
		Name:     "Vale",
		Race:     "human",
		Features: pqtype.NullRawMessage{RawMessage: blob, Valid: true},
	}

	sub := submissionFromCharacter(ch)

	if sub.PactBoon != "pact_of_the_blade" {
		t.Errorf("PactBoon = %q, want pact_of_the_blade", sub.PactBoon)
	}
	if len(sub.Invocations) != 2 {
		t.Fatalf("Invocations = %v, want 2 entries", sub.Invocations)
	}
	got := map[string]bool{}
	for _, id := range sub.Invocations {
		got[id] = true
	}
	if !got["agonizing_blast"] || !got["devils_sight"] {
		t.Errorf("Invocations = %v, want agonizing_blast + devils_sight", sub.Invocations)
	}
}

// classFeatureFeaturesForSubmission is the defensive safety net that runs even
// if validation were bypassed: it must drop unknown / duplicate / unmet-prereq
// picks, cap at the grant, and ignore a pact boon below level 3.
func TestClassFeatureFeaturesForSubmission_DefensiveFilter(t *testing.T) {
	// Warlock 2 (grant 2, no pact boon yet), no eldritch blast known.
	sub := warlockSub(2, nil, "pact_of_the_blade", []string{
		"agonizing_blast",       // prereq fail: no eldritch blast -> dropped
		"not_a_real_invocation", // unknown -> dropped
		"devils_sight",          // kept #1
		"devils_sight",          // duplicate -> dropped
		"beguiling_influence",   // kept #2
		"eldritch_spear",        // reached at cap -> break
	})

	out := classFeatureFeaturesForSubmission(sub)

	// Pact boon ignored below level 3.
	for _, f := range out {
		if f.Source == pactBoonFeatureSource {
			t.Errorf("pact boon should be ignored below level 3, got %q", f.Name)
		}
	}
	var kept []string
	for _, f := range out {
		kept = append(kept, f.MechanicalEffect)
	}
	if len(kept) != 2 {
		t.Fatalf("kept %v, want exactly 2 (devils_sight, beguiling_influence)", kept)
	}
	want := map[string]bool{"devils_sight": true, "beguiling_influence": true}
	for _, id := range kept {
		if !want[id] {
			t.Errorf("unexpected kept invocation %q", id)
		}
	}
}

// keptInvocationsForSubmission is the shared keep-loop factored out of
// classFeatureFeaturesForSubmission: same grant-cap + prereq + dedup logic,
// returning the kept catalog entries in submission order.
func TestKeptInvocationsForSubmission_CapPrereqDedup(t *testing.T) {
	// Warlock 2 (grant 2), no eldritch blast known.
	sub := warlockSub(2, nil, "", []string{
		"mask_of_many_faces",    // kept #1
		"not_a_real_invocation", // unknown -> dropped
		"armor_of_shadows",      // kept #2
		"armor_of_shadows",      // duplicate -> dropped
		"eldritch_sight",        // grant cap reached -> break
	})

	kept := keptInvocationsForSubmission(sub)

	var ids []string
	for _, inv := range kept {
		ids = append(ids, inv.ID)
	}
	want := []string{"mask_of_many_faces", "armor_of_shadows"}
	if !slices.Equal(ids, want) {
		t.Errorf("kept ids = %v, want %v", ids, want)
	}
}

// invocationGrantedSpellsForSubmission collects the spell ids the kept
// invocations grant, deduped, order-preserving, excluding manually-known spells.
func TestInvocationGrantedSpellsForSubmission_CollectsInOrder(t *testing.T) {
	// Warlock 5 (grant 3): mask_of_many_faces -> disguise-self,
	// armor_of_shadows -> mage-armor. Neither requires eldritch blast.
	sub := warlockSub(5, nil, "", []string{"mask_of_many_faces", "armor_of_shadows"})

	got := invocationGrantedSpellsForSubmission(sub)
	want := []string{"disguise-self", "mage-armor"}
	if !slices.Equal(got, want) {
		t.Errorf("granted = %v, want %v", got, want)
	}
}

func TestInvocationGrantedSpellsForSubmission_ExcludesManuallyKnown(t *testing.T) {
	// disguise-self already learned manually -> not re-granted; mage-armor stays.
	sub := warlockSub(5, []string{"disguise-self"}, "", []string{"mask_of_many_faces", "armor_of_shadows"})

	got := invocationGrantedSpellsForSubmission(sub)
	want := []string{"mage-armor"}
	if !slices.Equal(got, want) {
		t.Errorf("granted = %v, want %v", got, want)
	}
}

func TestInvocationGrantedSpellsForSubmission_NonWarlockAndEmptyNil(t *testing.T) {
	nonWarlock := CharacterSubmission{
		Name:        "Fighter",
		Class:       "fighter",
		Classes:     []character.ClassEntry{{Class: "fighter", Level: 5, IsPrimary: true}},
		Invocations: []string{"mask_of_many_faces"},
	}
	if got := invocationGrantedSpellsForSubmission(nonWarlock); got != nil {
		t.Errorf("non-warlock granted = %v, want nil", got)
	}
	if got := invocationGrantedSpellsForSubmission(warlockSub(5, nil, "", nil)); got != nil {
		t.Errorf("empty picks granted = %v, want nil", got)
	}
}

func TestInvocationGrantedSpellsForSubmission_GrantCapRespected(t *testing.T) {
	// Warlock 2 (grant 2): eldritch_sight is the 3rd pick, capped out, so its
	// detect-magic grant must NOT contribute.
	sub := warlockSub(2, nil, "", []string{"mask_of_many_faces", "armor_of_shadows", "eldritch_sight"})

	got := invocationGrantedSpellsForSubmission(sub)
	want := []string{"disguise-self", "mage-armor"}
	if !slices.Equal(got, want) {
		t.Errorf("granted = %v, want %v (detect-magic must be capped out)", got, want)
	}
}

func assertFeature(t *testing.T, features []character.Feature, name, source, mechEffect string) {
	t.Helper()
	for _, f := range features {
		if f.Name == name {
			if f.Source != source {
				t.Errorf("%s: source = %q, want %q", name, f.Source, source)
			}
			if f.MechanicalEffect != mechEffect {
				t.Errorf("%s: mechanical_effect = %q, want %q", name, f.MechanicalEffect, mechEffect)
			}
			return
		}
	}
	t.Errorf("feature %q not found in %+v", name, features)
}
