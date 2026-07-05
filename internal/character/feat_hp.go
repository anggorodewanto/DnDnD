package character

import "encoding/json"

// FeatFlatHPPerLevelSlug is the mechanical-effect type a feat carries when it
// grants a flat number of hit points per character level. Tough
// (refdata/seed_feats.go) is the only such seeded feat today: +2 HP per level.
const FeatFlatHPPerLevelSlug = "hp_plus_2_per_level"

// FeatFlatHPBonus returns the flat hit-point-maximum bonus the given features
// grant a character of totalLevel: +2 per level for each feature carrying
// FeatFlatHPPerLevelSlug (Tough), 0 otherwise. Detection is by the serialized
// mechanical-effect slug, not the feat name, so any future feat with the same
// effect earns it for free.
//
// This is the single source of truth for the flat-feat-HP rule, shared by the
// level-up feat-acquisition seam (levelup.ApplyFeat, which bumps the persisted
// HP the moment a feat is gained) and the portal builder rebuild (which
// recomputes HPMax from the feats-agnostic CalculateHP and must re-add this).
// COV-9 / COV-17 S2.
func FeatFlatHPBonus(features []Feature, totalLevel int32) int32 {
	var bonus int32
	for _, f := range features {
		if featHasFlatHPPerLevel(f) {
			bonus += 2 * totalLevel
		}
	}
	return bonus
}

// featHasFlatHPPerLevel reports whether a feature's serialized MechanicalEffect
// carries the flat-HP-per-level slug. The effect is stored as a JSON array of
// {effect_type: ...} maps (levelup writes it via specializeFeatEffects); an
// empty or unparseable value simply grants no bonus.
func featHasFlatHPPerLevel(f Feature) bool {
	if f.MechanicalEffect == "" {
		return false
	}
	var effects []map[string]string
	if err := json.Unmarshal([]byte(f.MechanicalEffect), &effects); err != nil {
		return false
	}
	for _, effect := range effects {
		if effect["effect_type"] == FeatFlatHPPerLevelSlug {
			return true
		}
	}
	return false
}
