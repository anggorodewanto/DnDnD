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

// grantedSpellsForSubmission unions invocation grants + Pact-of-the-Tome bonus
// cantrips into the SEPARATE granted-spells store, deduped against ONE shared
// seen set (seeded with the manually-known spells), order-preserving.
func TestGrantedSpellsForSubmission_IncludesTomeCantrips(t *testing.T) {
	// Warlock 3 with Pact of the Tome: the three bonus cantrips flow through
	// exactly, order-preserved.
	sub := warlockSub(3, nil, "pact_of_the_tome", nil)
	sub.TomeCantrips = []string{"guidance", "minor-illusion", "prestidigitation"}
	got := grantedSpellsForSubmission(sub)
	want := []string{"guidance", "minor-illusion", "prestidigitation"}
	if !slices.Equal(got, want) {
		t.Errorf("granted = %v, want %v", got, want)
	}

	// A non-tome boon ignores tome cantrips entirely.
	nonTome := warlockSub(3, nil, "pact_of_the_blade", nil)
	nonTome.TomeCantrips = []string{"guidance"}
	if got := grantedSpellsForSubmission(nonTome); got != nil {
		t.Errorf("non-tome boon granted = %v, want nil", got)
	}

	// Empty tome cantrips grant nothing.
	empty := warlockSub(3, nil, "pact_of_the_tome", nil)
	if got := grantedSpellsForSubmission(empty); got != nil {
		t.Errorf("empty tome cantrips granted = %v, want nil", got)
	}

	// A tome cantrip already learned manually (present in sub.Spells) is
	// excluded — no double-store.
	dup := warlockSub(3, []string{"guidance"}, "pact_of_the_tome", nil)
	dup.TomeCantrips = []string{"guidance", "minor-illusion"}
	if got := grantedSpellsForSubmission(dup); !slices.Equal(got, []string{"minor-illusion"}) {
		t.Errorf("granted = %v, want [minor-illusion] (guidance already known)", got)
	}

	// Union across ONE shared seen set: an invocation granting mage-armor plus a
	// tome list that repeats it emits it once (invocation first, then the rest).
	union := warlockSub(5, nil, "pact_of_the_tome", []string{"armor_of_shadows"})
	union.TomeCantrips = []string{"mage-armor", "minor-illusion"}
	if got := grantedSpellsForSubmission(union); !slices.Equal(got, []string{"mage-armor", "minor-illusion"}) {
		t.Errorf("union granted = %v, want [mage-armor minor-illusion]", got)
	}
}

// classFeatureFeaturesForSubmission stamps the kept tome cantrips onto the Pact
// Boon feature's Choices so the picks persist and round-trip. Empty / non-tome
// selections must NOT stamp Choices (byte-identical to pre-fix behavior).
func TestClassFeatureFeaturesForSubmission_StampsTomeChoices(t *testing.T) {
	sub := warlockSub(3, nil, "pact_of_the_tome", nil)
	sub.TomeCantrips = []string{"guidance", "minor-illusion", "prestidigitation"}
	boon := pactBoonFeature(t, classFeatureFeaturesForSubmission(sub))
	want := []string{"guidance", "minor-illusion", "prestidigitation"}
	if got := boon.Choices["tome_cantrips"]; !slices.Equal(got, want) {
		t.Errorf("boon Choices[tome_cantrips] = %v, want %v", got, want)
	}

	// Backward compat: no tome cantrips => nil Choices (no new bytes).
	bare := warlockSub(3, nil, "pact_of_the_tome", nil)
	if c := pactBoonFeature(t, classFeatureFeaturesForSubmission(bare)).Choices; c != nil {
		t.Errorf("empty tome cantrips must not stamp Choices, got %v", c)
	}

	// A non-tome boon never carries tome choices even if the field is populated.
	blade := warlockSub(3, nil, "pact_of_the_blade", nil)
	blade.TomeCantrips = []string{"guidance"}
	if c := pactBoonFeature(t, classFeatureFeaturesForSubmission(blade)).Choices; c != nil {
		t.Errorf("non-tome boon must not stamp tome choices, got %v", c)
	}
}

// A full round-trip: a tome-warlock submission resolves to features carrying the
// tome Choices, and reconstructing a submission from those persisted features
// recovers TomeCantrips (mirrors TestSubmissionFromCharacter_RestoresInvocationsAndBoon).
func TestSubmissionFromCharacter_RestoresTomeCantrips(t *testing.T) {
	feats := []character.Feature{
		{Name: "Pact Magic", Source: "warlock", MechanicalEffect: "pact_magic_cha"},
		{Name: "Pact of the Tome", Source: "pact_boon", MechanicalEffect: "pact_of_the_tome",
			Choices: map[string][]string{"tome_cantrips": {"guidance", "minor-illusion", "prestidigitation"}}},
	}
	blob, _ := json.Marshal(feats)
	ch := refdata.Character{
		Name:     "Tomelock",
		Race:     "human",
		Features: pqtype.NullRawMessage{RawMessage: blob, Valid: true},
	}

	sub := submissionFromCharacter(ch)

	if sub.PactBoon != "pact_of_the_tome" {
		t.Errorf("PactBoon = %q, want pact_of_the_tome", sub.PactBoon)
	}
	want := []string{"guidance", "minor-illusion", "prestidigitation"}
	if !slices.Equal(sub.TomeCantrips, want) {
		t.Errorf("TomeCantrips = %v, want %v", sub.TomeCantrips, want)
	}
}

func TestValidateSubmittedClassFeatures_TomeCantrips(t *testing.T) {
	legal := warlockSub(3, nil, "pact_of_the_tome", nil)
	legal.TomeCantrips = []string{"guidance", "minor-illusion", "prestidigitation"}
	if err := validateSubmittedClassFeatures(legal); err != nil {
		t.Fatalf("legal tome selection rejected: %v", err)
	}

	reject := map[string]func() CharacterSubmission{
		"tome cantrips on a non-tome boon": func() CharacterSubmission {
			s := warlockSub(3, nil, "pact_of_the_blade", nil)
			s.TomeCantrips = []string{"guidance"}
			return s
		},
		"tome cantrips with no boon at all": func() CharacterSubmission {
			s := warlockSub(3, nil, "", nil)
			s.TomeCantrips = []string{"guidance"}
			return s
		},
		"more than three cantrips": func() CharacterSubmission {
			s := warlockSub(3, nil, "pact_of_the_tome", nil)
			s.TomeCantrips = []string{"a", "b", "c", "d"}
			return s
		},
		"duplicate cantrip": func() CharacterSubmission {
			s := warlockSub(3, nil, "pact_of_the_tome", nil)
			s.TomeCantrips = []string{"guidance", "guidance"}
			return s
		},
		"empty cantrip id": func() CharacterSubmission {
			s := warlockSub(3, nil, "pact_of_the_tome", nil)
			s.TomeCantrips = []string{"guidance", ""}
			return s
		},
	}
	for name, build := range reject {
		if err := validateSubmittedClassFeatures(build()); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}

// pactBoonFeature returns the emitted Pact Boon feature (fails if absent).
func pactBoonFeature(t *testing.T, features []character.Feature) character.Feature {
	t.Helper()
	for _, f := range features {
		if f.Source == pactBoonFeatureSource {
			return f
		}
	}
	t.Fatalf("no pact boon feature in %+v", features)
	return character.Feature{}
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
