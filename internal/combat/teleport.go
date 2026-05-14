package combat

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// Teleport target constants matching the spell JSONB "target" field.
const (
	TeleportTargetSelf         = "self"
	TeleportTargetSelfCreature = "self+creature"
	TeleportTargetCreature     = "creature"
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

type TeleportSightOptions struct {
	Walls    []renderer.WallSegment
	FogOfWar *renderer.FogOfWar
}

var ErrTeleportNoLineOfSight = errors.New("target has full cover — no line of sight")

// ValidateTeleportDestination validates that a teleport destination is valid:
// (1) destination is unoccupied, (2) within range,
// (3) companion within companion_range_ft if applicable.
// The companion parameter is only needed for "self+creature" targets.
func ValidateTeleportDestination(info TeleportInfo, caster refdata.Combatant, destCol string, destRow int32, occupants []refdata.Combatant, companion *refdata.Combatant) error {
	for _, occ := range occupants {
		if occ.PositionCol == destCol && occ.PositionRow == destRow {
			return fmt.Errorf("destination %s%d is occupied", destCol, destRow)
		}
	}

	distFt := positionDistance(caster.PositionCol, caster.PositionRow, caster.AltitudeFt, destCol, destRow, 0)
	if info.RangeFt > 0 && distFt > info.RangeFt {
		return fmt.Errorf("destination is %dft away — teleport range is %dft", distFt, info.RangeFt)
	}

	if info.Target == TeleportTargetSelfCreature && companion != nil {
		companionDist := combatantDistance(caster, *companion)
		if companionDist > info.CompanionRangeFt {
			return fmt.Errorf("companion is %dft away — must be within %dft", companionDist, info.CompanionRangeFt)
		}
	}

	return nil
}

func ValidateTeleportDestinationWithSight(info TeleportInfo, caster refdata.Combatant, destCol string, destRow int32, occupants []refdata.Combatant, companion *refdata.Combatant, sight TeleportSightOptions) error {
	if err := ValidateTeleportDestination(info, caster, destCol, destRow, occupants, companion); err != nil {
		return err
	}

	if !info.RequiresSight {
		return nil
	}

	// Fail closed: if requires_sight is true but no sight context was supplied,
	// reject the cast rather than silently allowing it.
	if sight.FogOfWar == nil && len(sight.Walls) == 0 {
		return ErrTeleportNoLineOfSight
	}

	destColIndex := colToIndex(destCol)
	destRowIndex := int(destRow) - 1
	if sight.FogOfWar != nil && sight.FogOfWar.StateAt(destColIndex, destRowIndex) != renderer.Visible {
		return ErrTeleportNoLineOfSight
	}

	casterCol := colToIndex(caster.PositionCol)
	casterRow := int(caster.PositionRow) - 1
	if len(sight.Walls) > 0 && CalculateCover(casterCol, casterRow, destColIndex, destRowIndex, sight.Walls, nil) == CoverFull {
		return ErrTeleportNoLineOfSight
	}

	return nil
}

// positionDistance returns the 3D distance in feet between two grid positions.
func positionDistance(col1 string, row1 int32, alt1 int32, col2 string, row2 int32, alt2 int32) int {
	return Distance3D(
		colToIndex(col1), int(row1)-1, int(alt1),
		colToIndex(col2), int(row2)-1, int(alt2),
	)
}
