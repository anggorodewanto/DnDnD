package combat

import "github.com/ab/dndnd/internal/dice"

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
	if vision.BlindsightFt >= distanceFt {
		return NotObscured
	}
	if vision.TruesightFt >= distanceFt {
		return NotObscured
	}

	// Devil's Sight penetrates all darkness (including magical) within 120ft
	if vision.HasDevilsSight && distanceFt <= devilsSightRange {
		if zoneType == "darkness" || zoneType == "magical_darkness" || zoneType == "dim_light" {
			return NotObscured
		}
	}

	// Darkvision only helps with darkness-based zones (not fog/heavy obscurement)
	isDarknessZone := zoneType == "darkness" || zoneType == "dim_light"
	if vision.DarkvisionFt >= distanceFt && isDarknessZone {
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
	default:
		return ""
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
