package portal

import (
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
)

// InitFeatureUses computes the initial feature_uses map for a newly created
// character based on their class entries. Each limited-use feature is seeded
// with Current == Max and the correct Recharge key.
func InitFeatureUses(classes []character.ClassEntry, scores character.AbilityScores) map[string]character.FeatureUse {
	uses := make(map[string]character.FeatureUse)
	for _, ce := range classes {
		cls := strings.ToLower(ce.Class)
		switch cls {
		case "barbarian":
			max := combat.RageUsesPerDay(ce.Level)
			if max > 0 {
				uses[combat.FeatureKeyRage] = character.FeatureUse{Current: max, Max: max, Recharge: "long"}
			}
		case "monk":
			if ce.Level >= 2 {
				uses[combat.FeatureKeyKi] = character.FeatureUse{Current: ce.Level, Max: ce.Level, Recharge: "short"}
			}
		case "cleric", "paladin":
			if max := combat.ChannelDivinityMaxUses(ce.Class, ce.Level); max > 0 {
				uses[combat.FeatureKeyChannelDivinity] = character.FeatureUse{Current: max, Max: max, Recharge: "short"}
			}
			if cls == "paladin" && ce.Level >= 1 {
				max := combat.LayOnHandsPoolMax(ce.Level)
				uses[combat.FeatureKeyLayOnHands] = character.FeatureUse{Current: max, Max: max, Recharge: "long"}
			}
		case "bard":
			max := combat.BardicInspirationMaxUses(scores.CHA)
			recharge := combat.BardicInspirationRechargeType(ce.Level)
			uses[combat.FeatureKeyBardicInspiration] = character.FeatureUse{Current: max, Max: max, Recharge: recharge}
		case "fighter":
			if ce.Level >= 2 {
				surgeMax := 1
				if ce.Level >= 17 {
					surgeMax = 2
				}
				uses[combat.FeatureKeyActionSurge] = character.FeatureUse{Current: surgeMax, Max: surgeMax, Recharge: "short"}
				uses["second-wind"] = character.FeatureUse{Current: 1, Max: 1, Recharge: "short"}
			}
		case "druid":
			if ce.Level >= 2 {
				uses[combat.FeatureKeyWildShape] = character.FeatureUse{Current: 2, Max: 2, Recharge: "short"}
			}
		case "sorcerer":
			if ce.Level >= 2 {
				uses[combat.FeatureKeySorceryPoints] = character.FeatureUse{Current: ce.Level, Max: ce.Level, Recharge: "long"}
			}
		}
	}
	return uses
}
