package combat

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ReadyActionCommand holds the inputs for readying an action.
type ReadyActionCommand struct {
	Combatant      refdata.Combatant
	Turn           refdata.Turn
	Description    string
	SpellName      string // optional: set if readying a spell
	SpellSlotLevel int    // optional: spell slot level expended (0 = not a spell)
}

// ReadyActionResult holds the outputs of readying an action.
type ReadyActionResult struct {
	Turn        refdata.Turn
	Declaration refdata.ReactionDeclaration
	CombatLog   string
}

// ReadyAction handles the /action ready command.
// Costs an action. Creates a readied action (reaction declaration with is_readied_action=true).
//
// med-28 / Phase 71: when the readied action carries a SpellName + non-zero
// SpellSlotLevel, the matching spell slot is expended at ready-time and
// concentration on the readied spell is established. Both per spec line
// 1103: "spell slot is expended when readying; caster must hold
// concentration on the readied spell until trigger fires".
func (s *Service) ReadyAction(ctx context.Context, cmd ReadyActionCommand) (ReadyActionResult, error) {
	description := strings.TrimSpace(cmd.Description)
	if description == "" {
		return ReadyActionResult{}, fmt.Errorf("description must not be empty")
	}

	if ok, reason := CanActRaw(cmd.Combatant.Conditions); !ok {
		return ReadyActionResult{}, fmt.Errorf("%s", reason)
	}
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return ReadyActionResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return ReadyActionResult{}, err
	}

	spellName := sql.NullString{}
	spellSlotLevel := sql.NullInt32{}
	if cmd.SpellName != "" {
		spellName = sql.NullString{String: cmd.SpellName, Valid: true}
		spellSlotLevel = sql.NullInt32{Int32: int32(cmd.SpellSlotLevel), Valid: true}
	}

	// med-28: deduct the spell slot AND set concentration before creating
	// the declaration so a slot deduction error doesn't leave a phantom
	// readied action with no slot expended. Only fires for PCs (NPCs
	// readying spells aren't supported via this path) and only when a
	// real (non-zero) slot level is supplied.
	if cmd.SpellName != "" && cmd.SpellSlotLevel > 0 && cmd.Combatant.CharacterID.Valid {
		if err := s.expendReadiedSpellSlot(ctx, cmd); err != nil {
			return ReadyActionResult{}, err
		}
		if err := s.setReadiedSpellConcentration(ctx, cmd); err != nil {
			return ReadyActionResult{}, err
		}
	}

	decl, err := s.store.CreateReadiedActionDeclaration(ctx, refdata.CreateReadiedActionDeclarationParams{
		EncounterID:    cmd.Combatant.EncounterID,
		CombatantID:    cmd.Combatant.ID,
		Description:    description,
		SpellName:      spellName,
		SpellSlotLevel: spellSlotLevel,
	})
	if err != nil {
		return ReadyActionResult{}, fmt.Errorf("creating readied action declaration: %w", err)
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return ReadyActionResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	log := fmt.Sprintf("\u23f3 %s readies an action: \"%s\"", cmd.Combatant.DisplayName, description)

	return ReadyActionResult{
		Turn:        updatedTurn,
		Declaration: decl,
		CombatLog:   log,
	}, nil
}

// expendReadiedSpellSlot deducts the spell slot used to ready the spell.
// Prefers a Pact Magic slot when the character has one matching the level,
// otherwise consumes a regular spell slot. (med-28)
func (s *Service) expendReadiedSpellSlot(ctx context.Context, cmd ReadyActionCommand) error {
	char, err := s.store.GetCharacter(ctx, cmd.Combatant.CharacterID.UUID)
	if err != nil {
		return fmt.Errorf("loading character for readied spell slot: %w", err)
	}
	pact, _ := parsePactMagicSlots(char.PactMagicSlots.RawMessage)
	if pact.Current > 0 && cmd.SpellSlotLevel <= pact.SlotLevel {
		if _, err := s.deductAndPersistPactSlot(ctx, char.ID, pact); err != nil {
			return fmt.Errorf("deducting pact slot for readied spell: %w", err)
		}
		return nil
	}
	slots, err := parseIntKeyedSlots(char.SpellSlots.RawMessage)
	if err != nil {
		return fmt.Errorf("parsing spell slots: %w", err)
	}
	if _, err := s.deductAndPersistSlot(ctx, char.ID, slots, cmd.SpellSlotLevel); err != nil {
		return fmt.Errorf("deducting spell slot for readied spell: %w", err)
	}
	return nil
}

// setReadiedSpellConcentration writes the readied spell into the caster's
// concentration columns so the combatant is treated as concentrating until
// the trigger fires (or the readied action is cancelled / expired). The
// SpellID is empty because ReadyActionCommand only carries SpellName today
// \u2014 the cleanup paths key off SpellName, so this is sufficient for the
// pending CON-save / Silence-break pipelines. (med-28)
func (s *Service) setReadiedSpellConcentration(ctx context.Context, cmd ReadyActionCommand) error {
	if err := s.store.SetCombatantConcentration(ctx, refdata.SetCombatantConcentrationParams{
		ID:                     cmd.Combatant.ID,
		ConcentrationSpellID:   sql.NullString{},
		ConcentrationSpellName: sql.NullString{String: cmd.SpellName, Valid: true},
	}); err != nil {
		return fmt.Errorf("setting concentration on readied spell: %w", err)
	}
	return nil
}

// ExpireReadiedActions checks for active readied actions belonging to the
// given combatant and cancels them, returning expiry notice strings for the
// turn-start prompt.
func (s *Service) ExpireReadiedActions(ctx context.Context, combatantID, encounterID uuid.UUID) ([]string, error) {
	active, err := s.store.ListActiveReactionDeclarationsByCombatant(ctx, refdata.ListActiveReactionDeclarationsByCombatantParams{
		CombatantID: combatantID,
		EncounterID: encounterID,
	})
	if err != nil {
		return nil, fmt.Errorf("listing active reactions for expiry: %w", err)
	}

	var notices []string
	for _, decl := range filterReadiedActions(active) {
		if _, err := s.store.CancelReactionDeclaration(ctx, decl.ID); err != nil {
			return nil, fmt.Errorf("cancelling expired readied action: %w", err)
		}

		notice := fmt.Sprintf("\u23f3 Your readied action expired unused: \"%s\"", decl.Description)
		if decl.SpellName.Valid {
			notice += fmt.Sprintf("\n   \u2192 Concentration on %s ended. %s spell slot lost.",
				decl.SpellName.String, formatOrdinalSlotLevel(decl.SpellSlotLevel.Int32))
		}
		notices = append(notices, notice)
	}
	return notices, nil
}

// ListReadiedActions returns only the active readied action declarations for a combatant.
func (s *Service) ListReadiedActions(ctx context.Context, combatantID, encounterID uuid.UUID) ([]refdata.ReactionDeclaration, error) {
	active, err := s.store.ListActiveReactionDeclarationsByCombatant(ctx, refdata.ListActiveReactionDeclarationsByCombatantParams{
		CombatantID: combatantID,
		EncounterID: encounterID,
	})
	if err != nil {
		return nil, fmt.Errorf("listing active reactions: %w", err)
	}

	return filterReadiedActions(active), nil
}

// filterReadiedActions returns only the declarations that are readied actions.
func filterReadiedActions(decls []refdata.ReactionDeclaration) []refdata.ReactionDeclaration {
	var readied []refdata.ReactionDeclaration
	for _, d := range decls {
		if d.IsReadiedAction {
			readied = append(readied, d)
		}
	}
	return readied
}

// FormatReadiedActionsStatus produces a status display string listing active readied actions.
func FormatReadiedActionsStatus(readied []refdata.ReactionDeclaration) string {
	if len(readied) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\u23f3 Readied Actions:\n")
	for _, d := range readied {
		if d.SpellName.Valid {
			fmt.Fprintf(&b, "  - \"%s\" (spell: %s, %s slot)\n", d.Description, d.SpellName.String, formatOrdinalSlotLevel(d.SpellSlotLevel.Int32))
		} else {
			fmt.Fprintf(&b, "  - \"%s\"\n", d.Description)
		}
	}
	return b.String()
}

// formatOrdinalSlotLevel returns "1st-level", "2nd-level", etc.
func formatOrdinalSlotLevel(level int32) string {
	switch level {
	case 1:
		return "1st-level"
	case 2:
		return "2nd-level"
	case 3:
		return "3rd-level"
	default:
		return fmt.Sprintf("%dth-level", level)
	}
}
