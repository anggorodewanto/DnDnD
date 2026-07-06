package portal

import (
	"fmt"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// fightingStyleFeatureSource tags a resolved Fighting Style pick. Combat never
// reads Source (it matches on mechanical_effect), but the builder edit round-trip
// reverse-maps by it (classFeatureChoicesFromFeatures) and the Review step groups
// by source — mirroring invocationFeatureSource / pactBoonFeatureSource.
const fightingStyleFeatureSource = "fighting_style"

// submissionFightingStyleGrant reports whether the submission's classes grant the
// Fighting Style feature, and at what level (for the resolved feature's Level
// stamp). A multiclass character is granted if ANY class meets its threshold (per
// refdata.FightingStyleGrantLevel); the returned level is that class's grant level.
func submissionFightingStyleGrant(sub CharacterSubmission) (level int, granted bool) {
	for _, c := range SubmissionClasses(sub) {
		grant, ok := refdata.FightingStyleGrantLevel(c.Class)
		if !ok || c.Level < grant {
			continue
		}
		return grant, true
	}
	return 0, false
}

// fightingStyleFeatureForSubmission resolves the submission's fighting-style pick
// into a concrete character.Feature whose MechanicalEffect is the clean-slug the
// combat engine reads (e.g. "archery"). Returns ok=false when no style was picked,
// the class grants none, or the pick is unknown (validation rejects these upstream;
// this is the defensive safety net, mirroring classFeatureFeaturesForSubmission).
func fightingStyleFeatureForSubmission(sub CharacterSubmission) (character.Feature, bool) {
	if sub.FightingStyle == "" {
		return character.Feature{}, false
	}
	level, granted := submissionFightingStyleGrant(sub)
	if !granted {
		return character.Feature{}, false
	}
	style, ok := refdata.FightingStyleByID()[sub.FightingStyle]
	if !ok {
		return character.Feature{}, false
	}
	return character.Feature{
		Name:             style.Name,
		Source:           fightingStyleFeatureSource,
		Level:            level,
		Description:      style.Description,
		MechanicalEffect: style.ID,
	}, true
}

// validateSubmittedFightingStyle rejects an illegal fighting-style pick server-side
// (a crafted POST could otherwise claim a style on a class that grants none, or an
// unknown/inert style id). An empty pick is always accepted (the builder leaves it
// unset for non-granting classes and for players who skip the step). Mirrors
// validateSubmittedExpertise / validateSubmittedClassFeatures.
func validateSubmittedFightingStyle(sub CharacterSubmission) error {
	if sub.FightingStyle == "" {
		return nil
	}
	if _, ok := refdata.FightingStyleByID()[sub.FightingStyle]; !ok {
		return fmt.Errorf("unknown fighting style %q", sub.FightingStyle)
	}
	if _, granted := submissionFightingStyleGrant(sub); !granted {
		return fmt.Errorf("fighting style %q: this class/level does not grant a fighting style", sub.FightingStyle)
	}
	return nil
}
