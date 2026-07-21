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
	// metamagicFeatureSource tags a resolved Sorcerer Metamagic pick (COV-15).
	// Combat reads it by mechanical_effect (combat.HasMetamagic); the builder
	// edit round-trip reverse-maps by this source, mirroring invocations.
	metamagicFeatureSource = "metamagic"
	// featFeatureSource tags a feature written by the level-up ASI-feat flow
	// (levelup.ApplyFeat). The builder rebuild preserves these across an edit
	// rather than regenerating them (they are not builder-form choices). COV-17 S1.
	featFeatureSource = "feat"
)

// eldritchBlastSpellID mirrors combat's spell id for eldritch blast — the
// prerequisite spell for the *_blast invocations.
const eldritchBlastSpellID = "eldritch-blast"

// pactOfTheTomeID is the pact-boon id whose Book of Shadows grants three at-will
// bonus cantrips. Those picks travel in CharacterSubmission.TomeCantrips → the
// SEPARATE granted-spells store (grantedSpellsForSubmission), never the counted
// spells list, exactly like invocation grants.
const pactOfTheTomeID = "pact_of_the_tome"

// maxTomeCantrips is the Book of Shadows cantrip grant (Pact of the Tome: three
// cantrips of your choice). Server validation caps TomeCantrips at this.
const maxTomeCantrips = 3

// tomeCantripsChoiceKey is the character.Feature.Choices key under which the
// resolved Pact Boon feature persists the tome cantrips, so an edit round-trip
// (classFeatureChoicesFromFeatures) can restore them.
const tomeCantripsChoiceKey = "tome_cantrips"

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

// submissionClassLevel returns the level of the named class in the submission
// (0 if absent). Class-feature grants (invocations, pact boon, metamagic) scale
// with the specific class level, not total character level, so a multiclassed
// character is gated by that class's level.
func submissionClassLevel(sub CharacterSubmission, className string) int {
	for _, c := range SubmissionClasses(sub) {
		if strings.EqualFold(c.Class, className) {
			return c.Level
		}
	}
	return 0
}

// submissionWarlockLevel returns the warlock class level in the submission
// (0 if the character has no warlock levels).
func submissionWarlockLevel(sub CharacterSubmission) int {
	return submissionClassLevel(sub, "warlock")
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

	if err := validateTomeCantrips(sub); err != nil {
		return err
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

// validateTomeCantrips rejects an illegal Pact-of-the-Tome bonus-cantrip
// selection (mirrors the invocation validation style): cantrips chosen on any
// boon other than pact_of_the_tome, more than the three-cantrip Book of Shadows
// grant, and duplicate or empty ids. Like the invocation validator it does NOT
// verify each id names a real cantrip. An empty selection is always accepted.
func validateTomeCantrips(sub CharacterSubmission) error {
	if len(sub.TomeCantrips) == 0 {
		return nil
	}
	if sub.PactBoon != pactOfTheTomeID {
		return fmt.Errorf("tome cantrips require %s", pactOfTheTomeID)
	}
	if len(sub.TomeCantrips) > maxTomeCantrips {
		return fmt.Errorf("too many tome cantrips: %d chosen but Pact of the Tome grants %d", len(sub.TomeCantrips), maxTomeCantrips)
	}
	seen := make(map[string]struct{}, len(sub.TomeCantrips))
	for _, id := range sub.TomeCantrips {
		if id == "" {
			return fmt.Errorf("tome cantrip id must not be empty")
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate tome cantrip %q", id)
		}
		seen[id] = struct{}{}
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
			boon := character.Feature{
				Name:             b.Name,
				Source:           pactBoonFeatureSource,
				Level:            3,
				Description:      b.Description,
				MechanicalEffect: b.ID,
			}
			// Pact of the Tome: persist the kept bonus cantrips on the feature so
			// the picks round-trip through an edit. Empty selection leaves Choices
			// nil — byte-identical to a boon without tome cantrips.
			if b.ID == pactOfTheTomeID {
				if kept := tomeGrantedSpellsForSubmission(sub); len(kept) > 0 {
					boon.Choices = map[string][]string{tomeCantripsChoiceKey: kept}
				}
			}
			out = append(out, boon)
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
	// COV-15: the Fighter/Paladin/Ranger fighting-style pick resolves the same way
	// (concrete feature carrying the combat-read slug, replacing choose_fighting_style).
	if style, ok := fightingStyleFeatureForSubmission(sub); ok {
		out = append(out, style)
	}
	// COV-15: the Sorcerer metamagic picks resolve as a multi-select (mirrors
	// invocations), each carrying the clean slug the combat cast gate reads,
	// replacing choose_2_metamagic_options.
	out = append(out, metamagicFeaturesForSubmission(sub)...)
	return out
}

// submittedSpellSet returns a set of the submission's manually-known spell ids.
// It seeds the granted-spell collectors so a granted id the warlock already
// learned by hand is never double-stored under "granted_spells".
func submittedSpellSet(sub CharacterSubmission) map[string]struct{} {
	seen := make(map[string]struct{}, len(sub.Spells))
	for _, id := range sub.Spells {
		seen[id] = struct{}{}
	}
	return seen
}

// appendDeduped appends each id in ids to out, skipping ids already recorded in
// seen (a manually-known spell or an earlier grant) and recording the rest.
// Callers threading ONE seen set across several grant sources therefore dedup
// across all of them; order is preserved.
func appendDeduped(seen map[string]struct{}, out, ids []string) []string {
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// tomeCantripPicks returns the raw Pact-of-the-Tome bonus cantrips the
// submission carries — sub.TomeCantrips, but ONLY when the chosen boon is
// pact_of_the_tome (any other boon, or none, learns no Book of Shadows cantrips).
// Deduplication and manual-known exclusion are the caller's job (appendDeduped).
func tomeCantripPicks(sub CharacterSubmission) []string {
	if sub.PactBoon != pactOfTheTomeID {
		return nil
	}
	return sub.TomeCantrips
}

// invocationGrantedSpellsForSubmission collects the spell ids the submission's
// kept invocations make castable (Invocation.GrantsSpells), deduplicated and in
// order, EXCLUDING any spell the warlock already learned manually (present in
// sub.Spells). These persist under a SEPARATE character_data "granted_spells"
// key (not merged into "spells"): that keeps the known-spell budget honest
// (validateSpellCount counts len(sub.Spells)) and mirrors the
// AlwaysPreparedSpells precedent. Returns nil when nothing is granted.
func invocationGrantedSpellsForSubmission(sub CharacterSubmission) []string {
	seen := submittedSpellSet(sub)
	var out []string
	for _, inv := range keptInvocationsForSubmission(sub) {
		out = appendDeduped(seen, out, inv.GrantsSpells)
	}
	return out
}

// tomeGrantedSpellsForSubmission collects the Pact-of-the-Tome bonus cantrips
// the submission keeps: tomeCantripPicks, deduplicated and in order, EXCLUDING
// any id already in sub.Spells. It mirrors invocationGrantedSpellsForSubmission
// and is the single source of the "kept tome cantrips" — reused to stamp the
// Pact Boon feature's Choices (so they round-trip). Returns nil for any other
// boon, or when no cantrips were picked.
func tomeGrantedSpellsForSubmission(sub CharacterSubmission) []string {
	return appendDeduped(submittedSpellSet(sub), nil, tomeCantripPicks(sub))
}

// grantedSpellsForSubmission is the unified granted-spells collector: it unions
// invocation grants (Invocation.GrantsSpells) and Pact-of-the-Tome bonus
// cantrips (tomeCantripPicks) into the SEPARATE character_data "granted_spells"
// store, threading ONE shared seen set (seeded with the manually-known
// sub.Spells) so an id offered by both sources is emitted once. Invocation
// grants come first, then tome cantrips; order within each source is preserved.
// Returns nil when nothing is granted. Keeping these out of "spells" keeps the
// known-spell budget honest and lets both grants survive a builder rebuild PUT.
func grantedSpellsForSubmission(sub CharacterSubmission) []string {
	seen := submittedSpellSet(sub)
	var out []string
	for _, inv := range keptInvocationsForSubmission(sub) {
		out = appendDeduped(seen, out, inv.GrantsSpells)
	}
	return appendDeduped(seen, out, tomeCantripPicks(sub))
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

	var boonResolved, invResolved, styleResolved, metamagicResolved bool
	for _, f := range picks {
		switch f.Source {
		case pactBoonFeatureSource:
			boonResolved = true
		case invocationFeatureSource:
			invResolved = true
		case fightingStyleFeatureSource:
			styleResolved = true
		case metamagicFeatureSource:
			metamagicResolved = true
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
		if styleResolved && f.MechanicalEffect == refdata.ChooseFightingStyleEffect {
			continue
		}
		if metamagicResolved && f.MechanicalEffect == refdata.ChooseMetamagicEffect {
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
func classFeatureChoicesFromFeatures(features []character.Feature) (pactBoon string, invocations []string, fightingStyle string, metamagic []string, tomeCantrips []string) {
	for _, f := range features {
		switch f.Source {
		case pactBoonFeatureSource:
			pactBoon = f.MechanicalEffect
			// Pact of the Tome persists its bonus cantrips on the feature's
			// Choices; restore them so an edit round-trip re-grants them.
			if f.MechanicalEffect == pactOfTheTomeID {
				tomeCantrips = append(tomeCantrips, f.Choices[tomeCantripsChoiceKey]...)
			}
		case invocationFeatureSource:
			invocations = append(invocations, f.MechanicalEffect)
		case fightingStyleFeatureSource:
			fightingStyle = f.MechanicalEffect
		case metamagicFeatureSource:
			metamagic = append(metamagic, f.MechanicalEffect)
		}
	}
	return pactBoon, invocations, fightingStyle, metamagic, tomeCantrips
}
