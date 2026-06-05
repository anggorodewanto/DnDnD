package portal

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
)

// validateSubmittedSkills rejects an illegal set of submitted skill
// proficiencies. A crafted POST can otherwise claim arbitrary skills (e.g. all
// 18); this enforces the SRD budgets server-side.
//
// Validation runs only when the client explicitly sent skills. An empty
// sub.Skills leaves selection to the trusted class+background fallback in the
// submit path, so it is always accepted here.
//
// Budgets are computed from:
//   - BG: the background's fixed skills (backgroundSkillProficiencies).
//   - RF: race fixed skills (proficiency_<skill> trait codes).
//   - RC: race "choose any N" count (gain_<N>_skill_proficiencies trait codes).
//   - CL: the starting (first) class's skill_choices {from, choose}.
//
// raceTraits is the race's parsed trait list (mechanical_effect codes); pass
// nil when race data is unavailable — RF then resolves to ∅ and RC to 0.
func validateSubmittedSkills(sub CharacterSubmission, raceTraits []character.Feature) error {
	if len(sub.Skills) == 0 {
		return nil
	}

	bg := backgroundSkillProficiencies(sub.Background)
	rf := parseRaceFixedSkills(raceTraits)
	rc := parseRaceChooseCount(raceTraits)
	classFrom, classChoose := classSkillChoices(SubmissionClasses(sub))
	locked := unionSkills(bg, rf)

	return validateSkillSelection(sub.Skills, locked, classFrom, classChoose, rc)
}

// validateSkillSelection is the pure validation core. It returns nil when the
// submitted set is legal under the given budgets and a descriptive error
// otherwise. Under-selection is allowed (the UI enforces completeness); only
// over-selection / off-list / missing-locked / non-canonical / duplicates are
// rejected.
func validateSkillSelection(submitted, locked, classFrom []string, classChoose, raceChoose int) error {
	if err := validateCanonicalUnique(submitted); err != nil {
		return err
	}
	submittedSet := toSet(submitted)
	if err := validateLockedPresent(submittedSet, locked); err != nil {
		return err
	}
	return validateChoiceBudgets(submittedSet, locked, classFrom, classChoose, raceChoose)
}

// validateCanonicalUnique fails when any slug is not one of the canonical 18
// skills or when a slug appears more than once.
func validateCanonicalUnique(submitted []string) error {
	seen := make(map[string]struct{}, len(submitted))
	for _, s := range submitted {
		if _, ok := character.SkillAbilityMap[s]; !ok {
			return fmt.Errorf("skill %q is not a valid skill", s)
		}
		if _, dup := seen[s]; dup {
			return fmt.Errorf("duplicate skill %q in submission", s)
		}
		seen[s] = struct{}{}
	}
	return nil
}

// validateLockedPresent fails when a background or race-fixed skill is absent
// from the submitted set.
func validateLockedPresent(submittedSet map[string]struct{}, locked []string) error {
	for _, s := range locked {
		if _, ok := submittedSet[s]; !ok {
			return fmt.Errorf("required skill %q (granted by background/race) is missing", s)
		}
	}
	return nil
}

// validateChoiceBudgets enforces the single budget inequality:
//
//	max(0, len(inPool)-CN) + len(outPool) <= RC
//
// where CH = submitted \ locked, classPool = classFrom \ locked,
// inPool = CH ∩ classPool, outPool = CH \ classPool.
func validateChoiceBudgets(submittedSet map[string]struct{}, locked, classFrom []string, classChoose, raceChoose int) error {
	lockedSet := toSet(locked)
	classPool := diffSet(toSet(classFrom), lockedSet)

	var inPool, outPool int
	var outExample string
	for s := range submittedSet {
		if _, isLocked := lockedSet[s]; isLocked {
			continue
		}
		if _, inClass := classPool[s]; inClass {
			inPool++
			continue
		}
		outPool++
		if outExample == "" || s < outExample {
			outExample = s
		}
	}

	// When the race grants no free picks every chosen skill must come from the
	// class list, so an off-list skill can be named precisely.
	if raceChoose == 0 && outPool > 0 {
		return fmt.Errorf("skill %q is not selectable for this class", outExample)
	}

	overClassCap := inPool - classChoose
	if overClassCap < 0 {
		overClassCap = 0
	}
	if overClassCap+outPool <= raceChoose {
		return nil
	}
	return fmt.Errorf(
		"too many skill proficiencies: %d class picks (max %d) and %d off-list picks exceed the race budget of %d",
		inPool, classChoose, outPool, raceChoose,
	)
}

// classSkillChoices returns the SRD skill_choices {from, choose} for the
// starting (first) class. Mirrors the seed data in
// internal/refdata/seed_classes.go. Multiclassing grants no skill
// proficiencies, so only the first class is consulted.
func classSkillChoices(classes []character.ClassEntry) (from []string, choose int) {
	if len(classes) == 0 {
		return nil, 0
	}
	switch strings.ToLower(classes[0].Class) {
	case "barbarian":
		return []string{"animal-handling", "athletics", "intimidation", "nature", "perception", "survival"}, 2
	case "bard":
		return []string{"acrobatics", "animal-handling", "arcana", "athletics", "deception", "history", "insight", "intimidation", "investigation", "medicine", "nature", "perception", "performance", "persuasion", "religion", "sleight-of-hand", "stealth", "survival"}, 3
	case "cleric":
		return []string{"history", "insight", "medicine", "persuasion", "religion"}, 2
	case "druid":
		return []string{"arcana", "animal-handling", "insight", "medicine", "nature", "perception", "religion", "survival"}, 2
	case "fighter":
		return []string{"acrobatics", "animal-handling", "athletics", "history", "insight", "intimidation", "perception", "survival"}, 2
	case "monk":
		return []string{"acrobatics", "athletics", "history", "insight", "religion", "stealth"}, 2
	case "paladin":
		return []string{"athletics", "insight", "intimidation", "medicine", "persuasion", "religion"}, 2
	case "ranger":
		return []string{"animal-handling", "athletics", "insight", "investigation", "nature", "perception", "stealth", "survival"}, 3
	case "rogue":
		return []string{"acrobatics", "athletics", "deception", "insight", "intimidation", "investigation", "perception", "performance", "persuasion", "sleight-of-hand", "stealth"}, 4
	case "sorcerer":
		return []string{"arcana", "deception", "insight", "intimidation", "persuasion", "religion"}, 2
	case "warlock":
		return []string{"arcana", "deception", "history", "intimidation", "investigation", "nature", "religion"}, 2
	case "wizard":
		return []string{"arcana", "history", "insight", "investigation", "medicine", "religion"}, 2
	default:
		return nil, 0
	}
}

// parseRaceFixedSkills extracts race-granted fixed skills from trait
// mechanical_effect codes. Each effect string may pack several comma-separated
// codes (e.g. "advantage_saves_charmed,immune_magical_sleep"); only
// "proficiency_<skill>" codes whose skill is one of the canonical 18 are
// returned (weapon/tool proficiencies are ignored). Underscores in the code are
// mapped to hyphens to match the canonical slugs (e.g. sleight-of-hand).
func parseRaceFixedSkills(traits []character.Feature) []string {
	var fixed []string
	seen := make(map[string]struct{})
	for _, t := range traits {
		for _, code := range strings.Split(t.MechanicalEffect, ",") {
			skill, ok := fixedSkillFromCode(code)
			if !ok {
				continue
			}
			if _, dup := seen[skill]; dup {
				continue
			}
			seen[skill] = struct{}{}
			fixed = append(fixed, skill)
		}
	}
	return fixed
}

// fixedSkillFromCode maps a "proficiency_<skill>" mechanical-effect code to a
// canonical skill slug. Returns ok=false for non-skill proficiencies.
func fixedSkillFromCode(code string) (string, bool) {
	const prefix = "proficiency_"
	code = strings.TrimSpace(code)
	if !strings.HasPrefix(code, prefix) {
		return "", false
	}
	skill := strings.ReplaceAll(strings.TrimPrefix(code, prefix), "_", "-")
	if _, ok := character.SkillAbilityMap[skill]; !ok {
		return "", false
	}
	return skill, true
}

// parseRaceChooseCount sums the N across all "gain_<N>_skill_proficiencies"
// trait codes (0 for most races, 2 for half-elf).
func parseRaceChooseCount(traits []character.Feature) int {
	total := 0
	for _, t := range traits {
		for _, code := range strings.Split(t.MechanicalEffect, ",") {
			total += chooseCountFromCode(code)
		}
	}
	return total
}

// chooseCountFromCode parses "gain_<N>_skill_proficiencies" into N, or 0 when
// the code does not match.
func chooseCountFromCode(code string) int {
	const prefix = "gain_"
	const suffix = "_skill_proficiencies"
	code = strings.TrimSpace(code)
	if !strings.HasPrefix(code, prefix) || !strings.HasSuffix(code, suffix) {
		return 0
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(code, prefix), suffix)
	n := 0
	if _, err := fmt.Sscanf(mid, "%d", &n); err != nil {
		return 0
	}
	return n
}

// unionSkills returns the de-duplicated union of two skill slices.
func unionSkills(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []string
	for _, s := range append(append([]string{}, a...), b...) {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// toSet builds a set from a slice.
func toSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, s := range items {
		set[s] = struct{}{}
	}
	return set
}

// diffSet returns the elements of a not present in b.
func diffSet(a, b map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(a))
	for s := range a {
		if _, ok := b[s]; ok {
			continue
		}
		out[s] = struct{}{}
	}
	return out
}
