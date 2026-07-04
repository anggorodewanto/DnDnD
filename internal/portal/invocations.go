package portal

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// Feature.Source tags for resolved Warlock class-feature choices. Combat never
// reads Source (it matches on mechanical_effect / name), but the builder's
// edit round-trip reverse-maps by these tags to restore a warlock's picks, and
// the Review step groups by source.
const (
	invocationFeatureSource = "invocation"
	pactBoonFeatureSource   = "pact_boon"
)

// eldritchBlastSpellID mirrors combat's spell id for eldritch blast — the
// prerequisite spell for the *_blast invocations.
const eldritchBlastSpellID = "eldritch-blast"

// pactBoonPlaceholder is the mechanical_effect the warlock seed uses for the
// unresolved Pact Boon feature — aliased to the refdata SSOT so the seed and
// this stripper can never drift. isInvocationPlaceholder matches the invocation
// counterpart (choose_N_eldritch_invocations), kept tolerant because the seed
// bakes the level-scaled count into that slug.
const pactBoonPlaceholder = refdata.ChoosePactBoonEffect

func isInvocationPlaceholder(mechanicalEffect string) bool {
	return strings.HasPrefix(mechanicalEffect, "choose_") &&
		strings.HasSuffix(mechanicalEffect, "_eldritch_invocations")
}

// submissionWarlockLevel returns the warlock class level in the submission
// (0 if the character has no warlock levels). Invocation / pact-boon grants
// scale with warlock class level specifically, not total character level, so a
// multiclassed warlock is gated by its warlock level.
func submissionWarlockLevel(sub CharacterSubmission) int {
	for _, c := range SubmissionClasses(sub) {
		if strings.EqualFold(c.Class, "warlock") {
			return c.Level
		}
	}
	return 0
}

// submissionKnowsEldritchBlast reports whether the submission's chosen spells
// include the eldritch blast cantrip (prerequisite for the *_blast invocations).
func submissionKnowsEldritchBlast(sub CharacterSubmission) bool {
	return slices.Contains(sub.Spells, eldritchBlastSpellID)
}

// checkInvocationPrereq validates one invocation's gates against the character.
func checkInvocationPrereq(inv refdata.Invocation, warlockLevel int, pactBoon string, knowsEB bool) error {
	p := inv.Prereq
	if p.MinWarlockLevel > 0 && warlockLevel < p.MinWarlockLevel {
		return fmt.Errorf("invocation %q requires warlock level %d (have %d)", inv.ID, p.MinWarlockLevel, warlockLevel)
	}
	if p.RequiresPactBoon != "" && pactBoon != p.RequiresPactBoon {
		return fmt.Errorf("invocation %q requires %s", inv.ID, p.RequiresPactBoon)
	}
	if p.RequiresEldritchBlast && !knowsEB {
		return fmt.Errorf("invocation %q requires the eldritch blast cantrip", inv.ID)
	}
	return nil
}

// validateSubmittedClassFeatures rejects an illegal pact-boon / invocation
// selection. A crafted POST could otherwise claim arbitrary invocations, so
// this enforces the rules server-side (mirrors validateSubmittedExpertise):
//
//   - a pact boon must be a real boon id, chosen only at warlock level >= 3;
//   - invocations require warlock levels; the count must not exceed the
//     warlock-level grant; each must be a real, non-duplicated catalog id whose
//     prerequisites (min level, required pact boon, eldritch blast) are met.
//
// Empty selections are always accepted (the common non-warlock case, and a
// warlock who skipped the step).
func validateSubmittedClassFeatures(sub CharacterSubmission) error {
	wl := submissionWarlockLevel(sub)

	if sub.PactBoon != "" {
		if _, ok := refdata.PactBoonByID()[sub.PactBoon]; !ok {
			return fmt.Errorf("unknown pact boon %q", sub.PactBoon)
		}
		if !refdata.PactBoonGranted(wl) {
			return fmt.Errorf("pact boon requires warlock level 3 (have %d)", wl)
		}
	}

	if len(sub.Invocations) == 0 {
		return nil
	}

	grant := refdata.InvocationsKnown(wl)
	if grant == 0 {
		return fmt.Errorf("this character cannot learn eldritch invocations")
	}
	if len(sub.Invocations) > grant {
		return fmt.Errorf("too many invocations: %d chosen but warlock level %d grants %d", len(sub.Invocations), wl, grant)
	}

	byID := refdata.InvocationByID()
	knowsEB := submissionKnowsEldritchBlast(sub)
	seen := make(map[string]struct{}, len(sub.Invocations))
	for _, id := range sub.Invocations {
		inv, ok := byID[id]
		if !ok {
			return fmt.Errorf("unknown invocation %q", id)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate invocation %q", id)
		}
		seen[id] = struct{}{}
		if err := checkInvocationPrereq(inv, wl, sub.PactBoon, knowsEB); err != nil {
			return err
		}
	}
	return nil
}

// keptInvocationsForSubmission returns the invocation catalog entries the
// submission legally keeps, in submission order: capped at the warlock-level
// grant (refdata.InvocationsKnown), each with prerequisites met
// (checkInvocationPrereq), deduplicated, and unknown ids dropped. It is the
// single keep-loop shared by classFeatureFeaturesForSubmission (→ features) and
// invocationGrantedSpellsForSubmission (→ granted spell ids) so the two can
// never drift on which picks actually "count". Returns nil when the character
// cannot learn invocations or picked none.
func keptInvocationsForSubmission(sub CharacterSubmission) []refdata.Invocation {
	wl := submissionWarlockLevel(sub)
	grant := refdata.InvocationsKnown(wl)
	if grant == 0 {
		return nil
	}
	byID := refdata.InvocationByID()
	knowsEB := submissionKnowsEldritchBlast(sub)
	seen := make(map[string]struct{}, len(sub.Invocations))
	var kept []refdata.Invocation
	for _, id := range sub.Invocations {
		if len(kept) >= grant {
			break
		}
		inv, ok := byID[id]
		if !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		if checkInvocationPrereq(inv, wl, sub.PactBoon, knowsEB) != nil {
			continue
		}
		seen[id] = struct{}{}
		kept = append(kept, inv)
	}
	return kept
}

// classFeatureFeaturesForSubmission resolves the submission's pact-boon +
// invocation picks into character.Feature entries. Each invocation persists
// MechanicalEffect = <invocation id>, a CLEAN slug the combat reader matches
// (agonizing_blast). Illegal / unknown picks are dropped defensively as a
// safety net (validation already rejected them upstream), mirroring
// expertiseSkillsForSubmission.
func classFeatureFeaturesForSubmission(sub CharacterSubmission) []character.Feature {
	wl := submissionWarlockLevel(sub)
	var out []character.Feature

	if sub.PactBoon != "" && refdata.PactBoonGranted(wl) {
		if b, ok := refdata.PactBoonByID()[sub.PactBoon]; ok {
			out = append(out, character.Feature{
				Name:             b.Name,
				Source:           pactBoonFeatureSource,
				Level:            3,
				Description:      b.Description,
				MechanicalEffect: b.ID,
			})
		}
	}

	for _, inv := range keptInvocationsForSubmission(sub) {
		out = append(out, character.Feature{
			Name:             inv.Name,
			Source:           invocationFeatureSource,
			Level:            2,
			Description:      inv.Description,
			MechanicalEffect: inv.ID,
		})
	}
	return out
}

// invocationGrantedSpellsForSubmission collects the spell ids the submission's
// kept invocations make castable (Invocation.GrantsSpells), deduplicated and in
// order, EXCLUDING any spell the warlock already learned manually (present in
// sub.Spells). These persist under a SEPARATE character_data "granted_spells"
// key (not merged into "spells"): that keeps the known-spell budget honest
// (validateSpellCount counts len(sub.Spells)) and mirrors the
// AlwaysPreparedSpells precedent. Returns nil when nothing is granted.
func invocationGrantedSpellsForSubmission(sub CharacterSubmission) []string {
	seen := make(map[string]struct{}, len(sub.Spells))
	for _, id := range sub.Spells {
		seen[id] = struct{}{}
	}
	var out []string
	for _, inv := range keptInvocationsForSubmission(sub) {
		for _, spellID := range inv.GrantsSpells {
			if _, dup := seen[spellID]; dup {
				continue
			}
			seen[spellID] = struct{}{}
			out = append(out, spellID)
		}
	}
	return out
}

// injectClassFeatureChoices resolves the submission's pact-boon + invocation
// picks into concrete features and merges them into the derived feature list,
// replacing the unresolved choose_* placeholders that CollectFeatures produced
// from the warlock seed. Returns base unchanged when there are no picks
// (non-warlock, or a warlock who skipped the Class Features step).
func injectClassFeatureChoices(base []character.Feature, sub CharacterSubmission) []character.Feature {
	picks := classFeatureFeaturesForSubmission(sub)
	if len(picks) == 0 {
		return base
	}

	var boonResolved, invResolved bool
	for _, f := range picks {
		switch f.Source {
		case pactBoonFeatureSource:
			boonResolved = true
		case invocationFeatureSource:
			invResolved = true
		}
	}

	out := make([]character.Feature, 0, len(base)+len(picks))
	for _, f := range base {
		if boonResolved && f.MechanicalEffect == pactBoonPlaceholder {
			continue
		}
		if invResolved && isInvocationPlaceholder(f.MechanicalEffect) {
			continue
		}
		out = append(out, f)
	}
	return append(out, picks...)
}

// classFeatureChoicesFromFeatures reverse-maps persisted features back into a
// submission's PactBoon + Invocations (the inverse of injectClassFeatureChoices)
// so an edit round-trip preserves a warlock's picks instead of silently
// resetting them (cf. the builder-edit-resets-live-resources class of bugs).
func classFeatureChoicesFromFeatures(features []character.Feature) (pactBoon string, invocations []string) {
	for _, f := range features {
		switch f.Source {
		case pactBoonFeatureSource:
			pactBoon = f.MechanicalEffect
		case invocationFeatureSource:
			invocations = append(invocations, f.MechanicalEffect)
		}
	}
	return pactBoon, invocations
}
