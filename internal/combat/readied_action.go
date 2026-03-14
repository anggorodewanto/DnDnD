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
	for _, decl := range active {
		if !decl.IsReadiedAction {
			continue
		}

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

	var readied []refdata.ReactionDeclaration
	for _, decl := range active {
		if decl.IsReadiedAction {
			readied = append(readied, decl)
		}
	}
	return readied, nil
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
