package portal

import (
	"fmt"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// submissionSorcererLevel returns the sorcerer class level in the submission
// (0 if the character has no sorcerer levels). Metamagic grants scale with
// sorcerer class level specifically, not total character level, so a
// multiclassed sorcerer is gated by its sorcerer level.
func submissionSorcererLevel(sub CharacterSubmission) int {
	return submissionClassLevel(sub, "sorcerer")
}

// metamagicFeaturesForSubmission resolves the submission's metamagic picks into
// character.Feature entries: capped at the sorcerer-level grant
// (refdata.MetamagicKnown), deduplicated, unknown ids dropped. Each persists
// MechanicalEffect = <metamagic id>, the CLEAN slug the combat cast gate reads
// (combat.HasMetamagic). Illegal / over-grant picks are dropped defensively as a
// safety net (validation rejects them upstream), mirroring
// keptInvocationsForSubmission — and the resolution path (Preview) reaches here
// without calling validate*.
func metamagicFeaturesForSubmission(sub CharacterSubmission) []character.Feature {
	grant := refdata.MetamagicKnown(submissionSorcererLevel(sub))
	if grant == 0 {
		return nil
	}
	byID := refdata.MetamagicByID()
	seen := make(map[string]struct{}, len(sub.Metamagic))
	var out []character.Feature
	for _, id := range sub.Metamagic {
		if len(out) >= grant {
			break
		}
		m, ok := byID[id]
		if !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, character.Feature{
			Name:             m.Name,
			Source:           metamagicFeatureSource,
			Level:            3,
			Description:      m.Description,
			MechanicalEffect: m.ID,
		})
	}
	return out
}

// validateSubmittedMetamagic rejects an illegal metamagic selection server-side
// (a crafted POST could otherwise claim metamagic on a non-sorcerer, exceed the
// grant, or name an inert/unknown option). An empty selection is always accepted
// (the common non-sorcerer case, and a sorcerer who skipped the step). Mirrors
// the invocation branch of validateSubmittedClassFeatures.
func validateSubmittedMetamagic(sub CharacterSubmission) error {
	if len(sub.Metamagic) == 0 {
		return nil
	}
	sorcLevel := submissionSorcererLevel(sub)
	grant := refdata.MetamagicKnown(sorcLevel)
	if grant == 0 {
		return fmt.Errorf("this character cannot learn metamagic options")
	}
	if len(sub.Metamagic) > grant {
		return fmt.Errorf("too many metamagic options: %d chosen but sorcerer level %d grants %d", len(sub.Metamagic), sorcLevel, grant)
	}
	byID := refdata.MetamagicByID()
	seen := make(map[string]struct{}, len(sub.Metamagic))
	for _, id := range sub.Metamagic {
		if _, ok := byID[id]; !ok {
			return fmt.Errorf("unknown metamagic option %q", id)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate metamagic option %q", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}
