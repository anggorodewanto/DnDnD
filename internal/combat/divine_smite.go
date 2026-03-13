package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// SmiteDiceCount returns the number of d8s for a Divine Smite at the given slot level.
// 1st=2d8, 2nd=3d8, 3rd=4d8, 4th+=5d8 (max 5d8).
func SmiteDiceCount(slotLevel int) int {
	count := 1 + slotLevel // 1st=2, 2nd=3, etc.
	if count > 5 {
		return 5
	}
	return count
}

// SlotInfo represents the current and max values for a spell slot level.
type SlotInfo struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

// AvailableSmiteSlots returns a sorted list of spell slot levels that have at least 1 current slot.
func AvailableSmiteSlots(slots map[string]SlotInfo) []int {
	if len(slots) == 0 {
		return nil
	}
	var levels []int
	for key, info := range slots {
		if info.Current <= 0 {
			continue
		}
		level, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		levels = append(levels, level)
	}
	if len(levels) == 0 {
		return nil
	}
	sort.Ints(levels)
	return levels
}

// IsSmiteEligible returns true if the attack result is a melee weapon hit.
func IsSmiteEligible(result AttackResult) bool {
	return result.Hit && result.IsMelee
}

// SmiteDamageFormula returns the total dice count and formatted dice string for Divine Smite.
// Accounts for slot level, undead/fiend bonus (+1d8), and critical hit (doubles all dice).
func SmiteDamageFormula(slotLevel int, isUndead bool, isCrit bool) (int, string) {
	count := SmiteDiceCount(slotLevel)
	if isUndead {
		count++
	}
	if isCrit {
		count *= 2
	}
	return count, fmt.Sprintf("%dd8", count)
}

// ParseSpellSlots parses the character's spell_slots JSONB into a map of SlotInfo.
func ParseSpellSlots(raw []byte) (map[string]SlotInfo, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var slots map[string]SlotInfo
	if err := json.Unmarshal(raw, &slots); err != nil {
		return nil, fmt.Errorf("parsing spell_slots: %w", err)
	}
	return slots, nil
}

// HasFeatureByName checks whether a character's features JSON includes a feature
// with the given name (case-insensitive).
func HasFeatureByName(featuresJSON []byte, name string) bool {
	if len(featuresJSON) == 0 {
		return false
	}
	var features []CharacterFeature
	if err := json.Unmarshal(featuresJSON, &features); err != nil {
		return false
	}
	for _, f := range features {
		if strings.EqualFold(f.Name, name) {
			return true
		}
	}
	return false
}

// ordinal returns the ordinal suffix for a slot level (1st, 2nd, 3rd, 4th, etc.)
func ordinal(n int) string {
	switch n {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	default:
		return fmt.Sprintf("%dth", n)
	}
}

// FormatSmiteCombatLog formats the Divine Smite combat log line.
func FormatSmiteCombatLog(slotLevel int, isUndead bool, isCrit bool, diceStr string, damage int) string {
	slotLabel := fmt.Sprintf("%s-level slot", ordinal(slotLevel))
	if isCrit {
		slotLabel += ", crit"
	}

	suffix := ""
	if isUndead && isCrit {
		suffix = " (doubled) +2d8 vs undead"
	} else if isUndead {
		suffix = " +1d8 vs undead"
	} else if isCrit {
		suffix = " (doubled)"
	}

	return fmt.Sprintf("⚡ Divine Smite (%s) — %s radiant%s: %d", slotLabel, diceStr, suffix, damage)
}

// DivineSmiteCommand holds the inputs for a Divine Smite after a melee weapon hit.
type DivineSmiteCommand struct {
	Attacker     refdata.Combatant
	Target       refdata.Combatant
	SlotLevel    int
	IsCritical   bool
	AttackResult AttackResult
}

// DivineSmiteResult holds the output of a Divine Smite.
type DivineSmiteResult struct {
	SmiteDamage    int                `json:"smite_damage"`
	SmiteDice      string             `json:"smite_dice"`
	SlotLevel      int                `json:"slot_level"`
	IsUndead       bool               `json:"is_undead"`
	IsCritical     bool               `json:"is_critical"`
	SlotsRemaining map[string]SlotInfo `json:"slots_remaining"`
	CombatLog      string             `json:"combat_log"`
}

// isUndeadOrFiend checks if a creature is undead or fiend by type.
func isUndeadOrFiend(creatureType string) bool {
	lower := strings.ToLower(creatureType)
	return lower == "undead" || lower == "fiend"
}

// DivineSmite handles the Divine Smite on-hit prompt resolution.
// Validates paladin has the feature, slot is available, attack was melee hit,
// deducts spell slot, rolls damage, and returns result.
func (s *Service) DivineSmite(ctx context.Context, cmd DivineSmiteCommand, roller *dice.Roller) (DivineSmiteResult, error) {
	if !cmd.Attacker.CharacterID.Valid {
		return DivineSmiteResult{}, fmt.Errorf("Divine Smite requires a character (not NPC)")
	}

	// Validate attack was a melee hit
	if !IsSmiteEligible(cmd.AttackResult) {
		return DivineSmiteResult{}, fmt.Errorf("Divine Smite requires a melee weapon hit")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return DivineSmiteResult{}, fmt.Errorf("getting character: %w", err)
	}

	// Validate paladin has Divine Smite feature
	if !HasFeatureByName(char.Features.RawMessage, "Divine Smite") {
		return DivineSmiteResult{}, fmt.Errorf("character does not have Divine Smite")
	}

	// Parse spell slots
	slots, err := ParseSpellSlots(char.SpellSlots.RawMessage)
	if err != nil {
		return DivineSmiteResult{}, err
	}

	// Validate slot level is available
	slotKey := strconv.Itoa(cmd.SlotLevel)
	info, ok := slots[slotKey]
	if !ok || info.Current <= 0 {
		return DivineSmiteResult{}, fmt.Errorf("no %s-level spell slots remaining", ordinal(cmd.SlotLevel))
	}

	// Check if target is undead or fiend
	isUndead := false
	if cmd.Target.CreatureRefID.Valid && cmd.Target.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, cmd.Target.CreatureRefID.String)
		if err == nil {
			isUndead = isUndeadOrFiend(creature.Type)
		}
	}

	// Calculate dice
	diceCount, diceStr := SmiteDamageFormula(cmd.SlotLevel, isUndead, cmd.IsCritical)

	// Roll damage
	rollExpr := fmt.Sprintf("%dd8", diceCount)
	rollResult, err := roller.Roll(rollExpr)
	if err != nil {
		return DivineSmiteResult{}, fmt.Errorf("rolling smite damage: %w", err)
	}

	// Deduct spell slot
	info.Current--
	slots[slotKey] = info
	slotsJSON, err := json.Marshal(slots)
	if err != nil {
		return DivineSmiteResult{}, fmt.Errorf("marshaling spell_slots: %w", err)
	}
	if _, err := s.store.UpdateCharacterSpellSlots(ctx, refdata.UpdateCharacterSpellSlotsParams{
		ID:         char.ID,
		SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
	}); err != nil {
		return DivineSmiteResult{}, fmt.Errorf("updating spell_slots: %w", err)
	}

	combatLog := FormatSmiteCombatLog(cmd.SlotLevel, isUndead, cmd.IsCritical, diceStr, rollResult.Total)

	return DivineSmiteResult{
		SmiteDamage:    rollResult.Total,
		SmiteDice:      diceStr,
		SlotLevel:      cmd.SlotLevel,
		IsUndead:       isUndead,
		IsCritical:     cmd.IsCritical,
		SlotsRemaining: slots,
		CombatLog:      combatLog,
	}, nil
}
