package levelup

import "github.com/ab/dndnd/internal/character"

// FilterEligibleFeats returns only feats the character is eligible for:
// excludes already-owned feats and those with unmet prerequisites.
func FilterEligibleFeats(
	feats []FeatInfo,
	scores character.AbilityScores,
	armorProficiencies []string,
	isSpellcaster bool,
	ownedFeatIDs []string,
) []FeatInfo {
	owned := make(map[string]bool, len(ownedFeatIDs))
	for _, id := range ownedFeatIDs {
		owned[id] = true
	}
	var out []FeatInfo
	for _, f := range feats {
		if owned[f.ID] {
			continue
		}
		if ok, _ := CheckFeatPrerequisites(f.Prerequisites, scores, armorProficiencies, isSpellcaster); !ok {
			continue
		}
		out = append(out, f)
	}
	return out
}
