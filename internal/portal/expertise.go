package portal

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
)

// classExpertiseGrant returns the number of SKILL Expertise picks the given
// class grants at the given level (5e RAW, skill expertise only — thieves'
// tools are out of scope). Only Rogue and Bard grant expertise:
//
//   - Rogue: 2 skills at level 1, +2 more at level 6.
//   - Bard:  2 skills at level 3, +2 more at level 10.
//
// Every other class — and a Rogue/Bard below its first gate — grants 0. The
// class name is matched case-insensitively, mirroring classSkillChoices.
func classExpertiseGrant(class string, level int) int {
	switch strings.ToLower(class) {
	case "rogue":
		if level >= 6 {
			return 4
		}
		if level >= 1 {
			return 2
		}
		return 0
	case "bard":
		if level >= 10 {
			return 4
		}
		if level >= 3 {
			return 2
		}
		return 0
	default:
		return 0
	}
}

// submissionExpertiseGrant returns the expertise grant for a submission's
// starting (first) class entry at its level. Multiclassing does not stack
// expertise sources here — only the starting class is consulted, matching how
// classSkillChoices treats the first class as the proficiency source.
func submissionExpertiseGrant(sub CharacterSubmission) int {
	classes := SubmissionClasses(sub)
	if len(classes) == 0 {
		return 0
	}
	return classExpertiseGrant(classes[0].Class, classes[0].Level)
}

// validateSubmittedExpertise rejects an illegal set of submitted Expertise
// skills. A crafted POST can otherwise claim double proficiency on arbitrary
// skills; this enforces the SRD rules server-side:
//
//   - Each expertise slug must be a canonical skill, with no duplicates.
//   - Each expertise skill must also be a submitted proficiency (you can only
//     gain expertise in a skill you are proficient in).
//   - The count must not exceed the class+level grant (0 for non-expert
//     classes, so any expertise on a Fighter/Wizard/etc. is rejected).
//
// An empty expertise list is always accepted (the builder leaves it unset for
// the common case).
func validateSubmittedExpertise(sub CharacterSubmission) error {
	if len(sub.Expertise) == 0 {
		return nil
	}

	if err := validateCanonicalUnique(sub.Expertise); err != nil {
		return err
	}

	profSet := toSet(sub.Skills)
	for _, s := range sub.Expertise {
		if _, ok := profSet[s]; !ok {
			return fmt.Errorf("expertise skill %q must be a proficient skill", s)
		}
	}

	grant := submissionExpertiseGrant(sub)
	if len(sub.Expertise) > grant {
		return fmt.Errorf("too many expertise skills: %d chosen but this class/level grants %d", len(sub.Expertise), grant)
	}
	return nil
}

// expertiseSkillsForSubmission filters the proficient skills down to a legal
// expertise set as a safety net (parallels reconcileSkills): it keeps only
// proficient, canonical, de-duplicated picks, capped at the class+level grant.
// Returns nil when the submission grants no expertise.
func expertiseSkillsForSubmission(sub CharacterSubmission) []string {
	grant := submissionExpertiseGrant(sub)
	if grant == 0 || len(sub.Expertise) == 0 {
		return nil
	}

	profSet := toSet(sub.Skills)
	seen := make(map[string]struct{}, len(sub.Expertise))
	var kept []string
	for _, s := range sub.Expertise {
		if len(kept) >= grant {
			break
		}
		if _, canonical := character.SkillAbilityMap[s]; !canonical {
			continue
		}
		if _, ok := profSet[s]; !ok {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		kept = append(kept, s)
	}
	return kept
}
