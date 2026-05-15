package combat

import (
	"encoding/json"

	"github.com/ab/dndnd/internal/dice"
)

// ColToIndex converts column letters (A, B, ..., Z, AA, AB, ...) to a 0-based
// grid index. Exported helper so consumers outside the combat package (e.g.
// discord handlers that need to feed CombatantObscurement) can map a
// combatant's column without duplicating the conversion logic. (E-69)
func ColToIndex(col string) int {
	return colToIndex(col)
}

// ObscurementLevel represents the visibility level in a zone.
type ObscurementLevel int

const (
	// NotObscured means normal visibility (no penalty).
	NotObscured ObscurementLevel = iota
	// LightlyObscured means disadvantage on Perception (sight) checks; Hide is available.
	LightlyObscured
	// HeavilyObscured means effectively Blinded — disadvantage on attacks,
	// advantage for attackers, blocks line-of-sight spells.
	HeavilyObscured
)

// String returns a human-readable label for the obscurement level.
func (o ObscurementLevel) String() string {
	switch o {
	case LightlyObscured:
		return "lightly obscured"
	case HeavilyObscured:
		return "heavily obscured"
	default:
		return "none"
	}
}

// IsMagicalDarkness returns true if the zone type is magical darkness.
func IsMagicalDarkness(zoneType string) bool {
	return zoneType == "magical_darkness"
}

// VisionCapabilities holds the vision-related capabilities of a creature.
type VisionCapabilities struct {
	DarkvisionFt   int
	BlindsightFt   int
	TruesightFt    int
	HasDevilsSight bool
}

// devilsSightRange is the maximum range for Devil's Sight (120ft per RAW).
const devilsSightRange = 120

// EffectiveObscurement applies vision capabilities to reduce the effective
// obscurement level. The zoneType is needed to distinguish darkness-based zones
// (where darkvision helps) from fog/obscurement zones (where it does not).
func EffectiveObscurement(level ObscurementLevel, zoneType string, vision VisionCapabilities, distanceFt int) ObscurementLevel {
	if level == NotObscured {
		return NotObscured
	}

	// Blindsight and truesight penetrate all obscurement types
	if vision.BlindsightFt > 0 && vision.BlindsightFt >= distanceFt {
		return NotObscured
	}
	if vision.TruesightFt > 0 && vision.TruesightFt >= distanceFt {
		return NotObscured
	}

	// Darkness-based zones are affected by darkvision and Devil's Sight;
	// fog/obscurement zones are not.
	isDarknessZone := zoneType == "darkness" || zoneType == "dim_light"

	// Devil's Sight penetrates all darkness (including magical) within 120ft
	if vision.HasDevilsSight && distanceFt <= devilsSightRange {
		if isDarknessZone || zoneType == "magical_darkness" {
			return NotObscured
		}
	}

	// Darkvision only helps with darkness-based zones (not fog/heavy obscurement)
	if vision.DarkvisionFt > 0 && vision.DarkvisionFt >= distanceFt && isDarknessZone {
		// Darkvision downgrades: heavily -> lightly, lightly -> none
		if level == HeavilyObscured {
			return LightlyObscured
		}
		return NotObscured
	}

	return level
}

// ObscurementCheckEffect returns the roll mode and reason string for ability
// checks affected by obscurement. Only Perception (sight-based) checks are
// affected: lightly obscured imposes disadvantage, heavily obscured also
// imposes disadvantage (the auto-fail for blinded is handled separately).
func ObscurementCheckEffect(level ObscurementLevel, checkType string) (dice.RollMode, string) {
	if checkType != "perception" {
		return dice.Normal, ""
	}
	switch level {
	case LightlyObscured:
		return dice.Disadvantage, "lightly obscured: disadvantage on Perception"
	case HeavilyObscured:
		return dice.Disadvantage, "heavily obscured: disadvantage on Perception"
	default:
		return dice.Normal, ""
	}
}

// ObscurementAllowsHide returns true if the obscurement level is sufficient
// for a creature to attempt to hide (lightly or heavily obscured).
func ObscurementAllowsHide(level ObscurementLevel) bool {
	return level >= LightlyObscured
}

// ObscurementReasonString returns a human-readable reason string for combat log
// output, describing the obscurement situation and how vision capabilities
// interact with it. Returns empty string if there is no obscurement effect.
func ObscurementReasonString(zoneType string, vision VisionCapabilities, distanceFt int) string {
	baseLevel := ZoneObscurement(zoneType)
	if baseLevel == NotObscured {
		return ""
	}

	effectiveLevel := EffectiveObscurement(baseLevel, zoneType, vision, distanceFt)

	// If vision fully negates obscurement, no reason needed
	if effectiveLevel == NotObscured {
		return ""
	}

	switch zoneType {
	case "magical_darkness":
		return "magical darkness"
	case "darkness":
		if vision.DarkvisionFt >= distanceFt {
			return "darkness \u2192 dim (darkvision)"
		}
		return "darkness, no darkvision"
	case "dim_light":
		return "dim light, no darkvision"
	case "heavy_obscurement":
		return "heavy obscurement"
	case "light_obscurement":
		return "light obscurement"
	}

	// Unreachable: ZoneObscurement only returns non-NotObscured for the
	// zone types handled above. Kept for compiler satisfaction.
	return ""
}

// CombatantObscurement determines the worst effective obscurement level for a
// combatant at a given 0-based grid position, considering all active zones and
// the combatant's vision capabilities. Distance for vision checks is assumed
// to be 0 (combatant is inside the zone).
func CombatantObscurement(col, row int, zones []ZoneInfo, vision VisionCapabilities) ObscurementLevel {
	worst := NotObscured
	for _, z := range zones {
		baseLevel := ZoneObscurement(z.ZoneType)
		if baseLevel == NotObscured {
			continue
		}
		if !tileInSet(col, row, zoneInfoAffectedTiles(z)) {
			continue
		}
		// Distance 0: combatant is inside the zone
		effective := EffectiveObscurement(baseLevel, z.ZoneType, vision, 0)
		if effective > worst {
			worst = effective
		}
	}
	return worst
}

// zoneInfoAffectedTiles calculates the tiles affected by a ZoneInfo.
func zoneInfoAffectedTiles(z ZoneInfo) []GridPos {
	originCol := colToIndex(z.OriginCol)
	originRow := int(z.OriginRow) - 1

	var dims struct {
		RadiusFt int `json:"radius_ft"`
		WidthFt  int `json:"width_ft"`
		HeightFt int `json:"height_ft"`
		LengthFt int `json:"length_ft"`
		SideFt   int `json:"side_ft"`
	}
	_ = json.Unmarshal(z.Dimensions, &dims)

	switch z.Shape {
	case "circle":
		return SphereAffectedTiles(originCol, originRow, dims.RadiusFt)
	case "square":
		return SquareAffectedTiles(originCol, originRow, dims.SideFt)
	case "rectangle":
		return SquareAffectedTiles(originCol, originRow, dims.WidthFt)
	default:
		return []GridPos{{Col: originCol, Row: originRow}}
	}
}

// ZoneObscurement maps a zone type string to its obscurement level.
func ZoneObscurement(zoneType string) ObscurementLevel {
	switch zoneType {
	case "heavy_obscurement", "magical_darkness", "darkness":
		return HeavilyObscured
	case "dim_light", "light_obscurement":
		return LightlyObscured
	default:
		return NotObscured
	}
}
