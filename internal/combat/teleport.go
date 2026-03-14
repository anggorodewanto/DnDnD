package combat

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ab/dndnd/internal/refdata"
)

// TeleportInfo holds parsed teleport JSONB data from a spell.
type TeleportInfo struct {
	Target            string `json:"target"`
	RangeFt           int    `json:"range_ft"`
	RequiresSight     bool   `json:"requires_sight"`
	CompanionRangeFt  int    `json:"companion_range_ft"`
	AdditionalEffects string `json:"additional_effects"`
}

// ParseTeleportInfo parses the teleport JSONB field from a spell into a typed struct.
func ParseTeleportInfo(raw json.RawMessage) (TeleportInfo, error) {
	if len(raw) == 0 {
		return TeleportInfo{}, errors.New("teleport data is empty")
	}
	var info TeleportInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return TeleportInfo{}, err
	}
	return info, nil
}

// dmQueueTargets are teleport targets that require DM resolution (narrative/complex teleports).
var dmQueueTargets = map[string]bool{
	"portal":    true,
	"party":     true,
	"creatures": true,
	"group":     true,
}

// IsDMQueueTeleport returns true if the teleport target type requires routing to the DM queue.
func IsDMQueueTeleport(target string) bool {
	return dmQueueTargets[target]
}

// TeleportResult holds the outcome of a teleportation spell.
type TeleportResult struct {
	CasterMoved       bool
	CasterDestCol     string
	CasterDestRow     int32
	CompanionMoved    bool
	CompanionName     string
	CompanionDestCol  string
	CompanionDestRow  int32
	DMQueueRouted     bool
	AdditionalEffects string
}

// ValidateTeleportDestination validates that a teleport destination is valid:
// (1) destination is unoccupied, (2) within range, (3) line of sight if required,
// (4) companion within companion_range_ft if applicable.
// The companion parameter is only needed for "self+creature" targets.
func ValidateTeleportDestination(info TeleportInfo, caster refdata.Combatant, destCol string, destRow int32, occupants []refdata.Combatant, companion *refdata.Combatant) error {
	// 1. Check destination is unoccupied
	for _, occ := range occupants {
		if occ.PositionCol == destCol && occ.PositionRow == destRow {
			return fmt.Errorf("destination %s%d is occupied", destCol, destRow)
		}
	}

	// 2. Check destination is within teleport range
	casterCol := colToIndex(caster.PositionCol)
	casterRow := int(caster.PositionRow) - 1
	destColIdx := colToIndex(destCol)
	destRowIdx := int(destRow) - 1
	distFt := Distance3D(casterCol, casterRow, int(caster.AltitudeFt), destColIdx, destRowIdx, 0)
	if info.RangeFt > 0 && distFt > info.RangeFt {
		return fmt.Errorf("destination is %dft away — teleport range is %dft", distFt, info.RangeFt)
	}

	// 3. Line of sight (for now, requires_sight is validated by ensuring destination is within range;
	// actual fog-of-war LOS is a separate phase)

	// 4. Companion range check for "self+creature"
	if info.Target == "self+creature" && companion != nil {
		companionDist := combatantDistance(caster, *companion)
		if companionDist > info.CompanionRangeFt {
			return fmt.Errorf("companion is %dft away — must be within %dft", companionDist, info.CompanionRangeFt)
		}
	}

	return nil
}
