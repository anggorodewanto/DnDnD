package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// CounterspellOutcome represents the result of a Counterspell resolution.
type CounterspellOutcome string

const (
	CounterspellCountered  CounterspellOutcome = "countered"
	CounterspellFailed     CounterspellOutcome = "failed"
	CounterspellNeedsCheck CounterspellOutcome = "needs_check"
	CounterspellPassed     CounterspellOutcome = "passed"
	CounterspellForfeited  CounterspellOutcome = "forfeited"
)

// CounterspellPrompt represents the prompt sent to the player with available slot levels.
// EnemyCastLevel is intentionally NOT included — it is hidden until the ability check step.
type CounterspellPrompt struct {
	DeclarationID  uuid.UUID
	CasterName     string
	EnemySpellName string
	AvailableSlots []int
}

// CounterspellResult holds the outcome of a Counterspell resolution step.
type CounterspellResult struct {
	Outcome          CounterspellOutcome
	CasterName       string
	EnemySpellName   string
	EnemyCastLevel   int
	SlotUsed         int
	DC               int // DC for ability check (10 + enemy spell level), set when needs_check
	DeclarationID    uuid.UUID
	EnemySlotRefunded bool // true when the countered caster's slot was automatically refunded (SR-046)
}

// ErrSubtleSpellNotCounterspellable is returned by TriggerCounterspell when
// the enemy cast was flagged Subtle (Subtle Spell metamagic). Per spec line
// 948, Subtle Spell suppresses the V/S components — Counterspell cannot be
// triggered against it. (med-29 / Phase 72)
var ErrSubtleSpellNotCounterspellable = errors.New("counterspell cannot trigger against a subtle spell")
var ErrCounterspellSlotTooLow = errors.New("counterspell requires a spell slot of level 3 or higher")

// TriggerCounterspell is called by the DM from the Active Reactions Panel.
// It validates the declaration, looks up available slots, stores enemy spell info,
// and returns a prompt with enemy spell name (but NOT cast level) and available slot levels.
//
// enemyCasterID identifies the combatant whose spell is being counterspelled.
// When counterspell succeeds, their spell slot is automatically refunded (SR-046).
//
// med-29 / Phase 72: when isSubtle is true, return ErrSubtleSpellNotCounterspellable
// without prompting — the Subtle Spell metamagic bypasses Counterspell.
func (s *Service) TriggerCounterspell(ctx context.Context, declarationID uuid.UUID, enemySpellName string, enemyCastLevel int, isSubtle bool, enemyCasterID uuid.UUID) (CounterspellPrompt, error) {
	if isSubtle {
		return CounterspellPrompt{}, ErrSubtleSpellNotCounterspellable
	}
	decl, err := s.store.GetReactionDeclaration(ctx, declarationID)
	if err != nil {
		return CounterspellPrompt{}, fmt.Errorf("getting reaction declaration: %w", err)
	}
	if decl.Status != "active" {
		return CounterspellPrompt{}, fmt.Errorf("declaration is not active (status=%q)", decl.Status)
	}

	combatant, err := s.store.GetCombatant(ctx, decl.CombatantID)
	if err != nil {
		return CounterspellPrompt{}, fmt.Errorf("getting combatant: %w", err)
	}
	if !combatant.CharacterID.Valid {
		return CounterspellPrompt{}, fmt.Errorf("only player characters can use Counterspell")
	}

	char, err := s.store.GetCharacter(ctx, combatant.CharacterID.UUID)
	if err != nil {
		return CounterspellPrompt{}, fmt.Errorf("getting character: %w", err)
	}

	slots, err := parseIntKeyedSlots(char.SpellSlots.RawMessage)
	if err != nil {
		return CounterspellPrompt{}, fmt.Errorf("parsing spell slots: %w", err)
	}
	pactSlots, _ := parsePactMagicSlots(char.PactMagicSlots.RawMessage)

	available := AvailableCounterspellSlots(slots, pactSlots)
	if len(available) == 0 {
		return CounterspellPrompt{}, fmt.Errorf("no spell slots available for Counterspell (level 3+)")
	}

	if _, err := s.store.UpdateReactionDeclarationCounterspellPrompt(ctx, refdata.UpdateReactionDeclarationCounterspellPromptParams{
		ID:                        declarationID,
		CounterspellEnemySpell:    sql.NullString{String: enemySpellName, Valid: true},
		CounterspellEnemyLevel:    sql.NullInt32{Int32: int32(enemyCastLevel), Valid: true},
		CounterspellEnemyCasterID: uuid.NullUUID{UUID: enemyCasterID, Valid: enemyCasterID != uuid.Nil},
	}); err != nil {
		return CounterspellPrompt{}, fmt.Errorf("updating counterspell prompt: %w", err)
	}

	return CounterspellPrompt{
		DeclarationID:  declarationID,
		CasterName:     combatant.DisplayName,
		EnemySpellName: enemySpellName,
		AvailableSlots: available,
	}, nil
}

// ResolveCounterspell is called when the player picks a slot level for Counterspell.
// It deducts the slot, consumes the reaction, and determines the outcome:
// - If slot >= enemy cast level: auto-counter (success)
// - If slot < enemy cast level: needs ability check (DC = 10 + enemy spell level)
func (s *Service) ResolveCounterspell(ctx context.Context, declarationID uuid.UUID, chosenSlotLevel int) (CounterspellResult, error) {
	if chosenSlotLevel < 3 {
		return CounterspellResult{}, ErrCounterspellSlotTooLow
	}

	decl, err := s.store.GetReactionDeclaration(ctx, declarationID)
	if err != nil {
		return CounterspellResult{}, fmt.Errorf("getting reaction declaration: %w", err)
	}
	if decl.CounterspellStatus.String != "prompted" {
		return CounterspellResult{}, fmt.Errorf("counterspell not in prompted state (status=%q)", decl.CounterspellStatus.String)
	}

	combatant, err := s.store.GetCombatant(ctx, decl.CombatantID)
	if err != nil {
		return CounterspellResult{}, fmt.Errorf("getting combatant: %w", err)
	}
	if !combatant.CharacterID.Valid {
		return CounterspellResult{}, fmt.Errorf("only player characters can use Counterspell")
	}

	char, err := s.store.GetCharacter(ctx, combatant.CharacterID.UUID)
	if err != nil {
		return CounterspellResult{}, fmt.Errorf("getting character: %w", err)
	}

	slots, err := parseIntKeyedSlots(char.SpellSlots.RawMessage)
	if err != nil {
		return CounterspellResult{}, fmt.Errorf("parsing spell slots: %w", err)
	}
	pactSlots, _ := parsePactMagicSlots(char.PactMagicSlots.RawMessage)

	if pactSlots.Current > 0 && pactSlots.SlotLevel == chosenSlotLevel {
		if _, err := s.deductAndPersistPactSlot(ctx, char.ID, pactSlots); err != nil {
			return CounterspellResult{}, fmt.Errorf("deducting pact slot: %w", err)
		}
	} else {
		if _, err := s.deductAndPersistSlot(ctx, char.ID, slots, chosenSlotLevel); err != nil {
			return CounterspellResult{}, fmt.Errorf("deducting spell slot: %w", err)
		}
	}

	if _, err := s.ResolveReaction(ctx, declarationID); err != nil {
		return CounterspellResult{}, fmt.Errorf("resolving reaction: %w", err)
	}

	enemyCastLevel := int(decl.CounterspellEnemyLevel.Int32)

	result := CounterspellResult{
		CasterName:     combatant.DisplayName,
		EnemySpellName: decl.CounterspellEnemySpell.String,
		EnemyCastLevel: enemyCastLevel,
		SlotUsed:       chosenSlotLevel,
		DeclarationID:  declarationID,
	}

	resolvedParams := refdata.UpdateReactionDeclarationCounterspellResolvedParams{
		ID:                   declarationID,
		CounterspellSlotUsed: sql.NullInt32{Int32: int32(chosenSlotLevel), Valid: true},
	}

	if chosenSlotLevel >= enemyCastLevel {
		result.Outcome = CounterspellCountered
		resolvedParams.CounterspellStatus = sql.NullString{String: "countered", Valid: true}
	} else {
		dc := 10 + enemyCastLevel
		result.Outcome = CounterspellNeedsCheck
		result.DC = dc
		resolvedParams.CounterspellStatus = sql.NullString{String: "needs_check", Valid: true}
		resolvedParams.CounterspellDc = sql.NullInt32{Int32: int32(dc), Valid: true}
	}

	if _, err := s.store.UpdateReactionDeclarationCounterspellResolved(ctx, resolvedParams); err != nil {
		return CounterspellResult{}, fmt.Errorf("updating counterspell %s: %w", resolvedParams.CounterspellStatus.String, err)
	}

	if result.Outcome == CounterspellCountered {
		refunded, err := s.refundCounterspelledSlot(ctx, decl.CounterspellEnemyCasterID, enemyCastLevel)
		if err != nil {
			return CounterspellResult{}, fmt.Errorf("refunding counterspelled slot: %w", err)
		}
		result.EnemySlotRefunded = refunded
	}

	return result, nil
}

// ResolveCounterspellCheck is called after the player rolls their spellcasting ability check.
// If checkTotal >= DC: counter succeeds. Otherwise: counter fails (slot already spent).
func (s *Service) ResolveCounterspellCheck(ctx context.Context, declarationID uuid.UUID, checkTotal int) (CounterspellResult, error) {
	decl, err := s.store.GetReactionDeclaration(ctx, declarationID)
	if err != nil {
		return CounterspellResult{}, fmt.Errorf("getting reaction declaration: %w", err)
	}
	if decl.CounterspellStatus.String != "needs_check" {
		return CounterspellResult{}, fmt.Errorf("counterspell not in needs_check state (status=%q)", decl.CounterspellStatus.String)
	}

	combatant, err := s.store.GetCombatant(ctx, decl.CombatantID)
	if err != nil {
		return CounterspellResult{}, fmt.Errorf("getting combatant: %w", err)
	}

	dc := int(decl.CounterspellDc.Int32)
	result := CounterspellResult{
		CasterName:     combatant.DisplayName,
		EnemySpellName: decl.CounterspellEnemySpell.String,
		EnemyCastLevel: int(decl.CounterspellEnemyLevel.Int32),
		SlotUsed:       int(decl.CounterspellSlotUsed.Int32),
		DC:             dc,
		DeclarationID:  declarationID,
	}

	if checkTotal >= dc {
		result.Outcome = CounterspellCountered
	} else {
		result.Outcome = CounterspellFailed
	}

	statusStr := string(result.Outcome)
	if _, err := s.store.UpdateReactionDeclarationCounterspellResolved(ctx, refdata.UpdateReactionDeclarationCounterspellResolvedParams{
		ID:                   declarationID,
		CounterspellSlotUsed: sql.NullInt32{Int32: int32(result.SlotUsed), Valid: true},
		CounterspellStatus:   sql.NullString{String: statusStr, Valid: true},
		CounterspellDc:       sql.NullInt32{Int32: int32(dc), Valid: true},
	}); err != nil {
		return CounterspellResult{}, fmt.Errorf("updating counterspell check result: %w", err)
	}

	if result.Outcome == CounterspellCountered {
		refunded, err := s.refundCounterspelledSlot(ctx, decl.CounterspellEnemyCasterID, result.EnemyCastLevel)
		if err != nil {
			return CounterspellResult{}, fmt.Errorf("refunding counterspelled slot: %w", err)
		}
		result.EnemySlotRefunded = refunded
	}

	return result, nil
}

// PassCounterspell is called when the player passes on the Counterspell prompt.
// The reaction is NOT consumed. The declaration stays active.
func (s *Service) PassCounterspell(ctx context.Context, declarationID uuid.UUID) (CounterspellResult, error) {
	decl, err := s.store.GetReactionDeclaration(ctx, declarationID)
	if err != nil {
		return CounterspellResult{}, fmt.Errorf("getting reaction declaration: %w", err)
	}
	if decl.CounterspellStatus.String != "prompted" {
		return CounterspellResult{}, fmt.Errorf("counterspell not in prompted state (status=%q)", decl.CounterspellStatus.String)
	}

	combatant, err := s.store.GetCombatant(ctx, decl.CombatantID)
	if err != nil {
		return CounterspellResult{}, fmt.Errorf("getting combatant: %w", err)
	}

	if _, err := s.store.UpdateReactionDeclarationCounterspellResolved(ctx, refdata.UpdateReactionDeclarationCounterspellResolvedParams{
		ID:               declarationID,
		CounterspellStatus: sql.NullString{String: "passed", Valid: true},
	}); err != nil {
		return CounterspellResult{}, fmt.Errorf("updating counterspell passed: %w", err)
	}

	return CounterspellResult{
		Outcome:        CounterspellPassed,
		CasterName:     combatant.DisplayName,
		EnemySpellName: decl.CounterspellEnemySpell.String,
		DeclarationID:  declarationID,
	}, nil
}

// ForfeitCounterspell is called when the player does not respond within the timeout.
// The reaction IS consumed, but the slot is NOT spent.
func (s *Service) ForfeitCounterspell(ctx context.Context, declarationID uuid.UUID) (CounterspellResult, error) {
	decl, err := s.store.GetReactionDeclaration(ctx, declarationID)
	if err != nil {
		return CounterspellResult{}, fmt.Errorf("getting reaction declaration: %w", err)
	}
	if decl.CounterspellStatus.String != "prompted" {
		return CounterspellResult{}, fmt.Errorf("counterspell not in prompted state (status=%q)", decl.CounterspellStatus.String)
	}

	combatant, err := s.store.GetCombatant(ctx, decl.CombatantID)
	if err != nil {
		return CounterspellResult{}, fmt.Errorf("getting combatant: %w", err)
	}

	if _, err := s.ResolveReaction(ctx, declarationID); err != nil {
		return CounterspellResult{}, fmt.Errorf("resolving reaction for forfeit: %w", err)
	}

	if _, err := s.store.UpdateReactionDeclarationCounterspellResolved(ctx, refdata.UpdateReactionDeclarationCounterspellResolvedParams{
		ID:               declarationID,
		CounterspellStatus: sql.NullString{String: "forfeited", Valid: true},
	}); err != nil {
		return CounterspellResult{}, fmt.Errorf("updating counterspell forfeited: %w", err)
	}

	return CounterspellResult{
		Outcome:        CounterspellForfeited,
		CasterName:     combatant.DisplayName,
		EnemySpellName: decl.CounterspellEnemySpell.String,
		DeclarationID:  declarationID,
	}, nil
}

// FormatCounterspellLog produces the combat log output for a Counterspell resolution.
func FormatCounterspellLog(result CounterspellResult) string {
	switch result.Outcome {
	case CounterspellCountered:
		return fmt.Sprintf("\u2728 %s counters %s!", result.CasterName, result.EnemySpellName)
	case CounterspellFailed:
		return fmt.Sprintf("\u274c %s failed to counter %s (DC %d). Slot expended.", result.CasterName, result.EnemySpellName, result.DC)
	case CounterspellNeedsCheck:
		return fmt.Sprintf("\u2753 %s attempts to counter %s (level %d) with a %s slot. Roll spellcasting ability check — DC %d.",
			result.CasterName, result.EnemySpellName, result.EnemyCastLevel, formatOrdinalSlotLevel(int32(result.SlotUsed)), result.DC)
	case CounterspellPassed:
		return fmt.Sprintf("\u23ed\ufe0f %s passes on Counterspell against %s.", result.CasterName, result.EnemySpellName)
	case CounterspellForfeited:
		return fmt.Sprintf("\u23f0 %s forfeited Counterspell against %s (timeout). Reaction consumed.", result.CasterName, result.EnemySpellName)
	default:
		return ""
	}
}

// AvailableCounterspellSlots returns sorted slot levels >= 3 that have remaining uses.
// Includes both regular spell slots and pact magic slots.
func AvailableCounterspellSlots(slots map[int]SlotInfo, pact PactMagicSlotState) []int {
	var levels []int
	for level, info := range slots {
		if level >= 3 && info.Current > 0 {
			levels = append(levels, level)
		}
	}
	if pact.Current > 0 && pact.SlotLevel >= 3 && !slices.Contains(levels, pact.SlotLevel) {
		levels = append(levels, pact.SlotLevel)
	}
	slices.Sort(levels)
	return levels
}

// refundCounterspelledSlot restores the spell slot of the enemy caster whose spell
// was successfully counterspelled. If the enemy caster is an NPC (no CharacterID),
// this is a no-op. Returns true if a slot was refunded. (SR-046)
func (s *Service) refundCounterspelledSlot(ctx context.Context, enemyCasterID uuid.NullUUID, slotLevel int) (bool, error) {
	if !enemyCasterID.Valid || slotLevel <= 0 {
		return false, nil
	}
	combatant, err := s.store.GetCombatant(ctx, enemyCasterID.UUID)
	if err != nil {
		return false, fmt.Errorf("getting enemy combatant for slot refund: %w", err)
	}
	if !combatant.CharacterID.Valid {
		return false, nil // NPC — no tracked slots to refund
	}
	char, err := s.store.GetCharacter(ctx, combatant.CharacterID.UUID)
	if err != nil {
		return false, fmt.Errorf("getting enemy character for slot refund: %w", err)
	}
	slots, err := parseIntKeyedSlots(char.SpellSlots.RawMessage)
	if err != nil {
		return false, fmt.Errorf("parsing enemy spell slots for refund: %w", err)
	}
	info, ok := slots[slotLevel]
	if !ok {
		return false, nil
	}
	if info.Current >= info.Max {
		return false, nil // already at max — nothing to refund
	}
	slots[slotLevel] = SlotInfo{Current: info.Current + 1, Max: info.Max}
	slotsJSON, err := json.Marshal(intToStringKeyedSlots(slots))
	if err != nil {
		return false, fmt.Errorf("marshaling refunded spell slots: %w", err)
	}
	if _, err := s.store.UpdateCharacterSpellSlots(ctx, refdata.UpdateCharacterSpellSlotsParams{
		ID:         char.ID,
		SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
	}); err != nil {
		return false, fmt.Errorf("persisting refunded spell slot: %w", err)
	}
	return true, nil
}
