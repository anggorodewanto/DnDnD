package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// ErrEncounterNotActive is returned when EndCombat is called on a non-active encounter.
var ErrEncounterNotActive = errors.New("encounter must be active to end combat")

// Valid encounter statuses.
var validStatuses = map[string]bool{
	"preparing": true,
	"active":    true,
	"completed": true,
}

// Store defines the database operations needed by the combat service.
type Store interface {
	// Encounters
	CreateEncounter(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error)
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	ListEncountersByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error)
	UpdateEncounterStatus(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error)
	UpdateEncounterCurrentTurn(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error)
	UpdateEncounterRound(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error)
	DeleteEncounter(ctx context.Context, id uuid.UUID) error

	// Combatants
	CreateCombatant(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	UpdateCombatantHP(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error)
	UpdateCombatantConditions(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error)
	UpdateCombatantPosition(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error)
	UpdateCombatantDeathSaves(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error)
	UpdateCombatantRage(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error)
	UpdateCombatantWildShape(ctx context.Context, arg refdata.UpdateCombatantWildShapeParams) (refdata.Combatant, error)
	UpdateCombatantBardicInspiration(ctx context.Context, arg refdata.UpdateCombatantBardicInspirationParams) (refdata.Combatant, error)
	UpdateCombatantVisibility(ctx context.Context, arg refdata.UpdateCombatantVisibilityParams) (refdata.Combatant, error)
	DeleteCombatant(ctx context.Context, id uuid.UUID) error

	// Turns
	CreateTurn(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error)
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	GetActiveTurnByEncounterID(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error)
	CompleteTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)

	// Action Log
	CreateActionLog(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error)
	ListActionLogByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.ActionLog, error)
	ListActionLogByTurnID(ctx context.Context, turnID uuid.UUID) ([]refdata.ActionLog, error)

	// Initiative
	UpdateCombatantInitiative(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error)
	SkipTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	ListTurnsByEncounterAndRound(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error)

	// Turn Resources
	UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error)

	// Reference data lookups
	GetEncounterTemplate(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error)
	GetCreature(ctx context.Context, id string) (refdata.Creature, error)
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetClass(ctx context.Context, id string) (refdata.Class, error)
	GetWeapon(ctx context.Context, id string) (refdata.Weapon, error)
	GetArmor(ctx context.Context, id string) (refdata.Armor, error)
	ListCharactersByCampaign(ctx context.Context, campaignID uuid.UUID) ([]refdata.Character, error)

	// Character inventory
	UpdateCharacterInventory(ctx context.Context, id uuid.UUID, inventory pqtype.NullRawMessage) error

	// Character gold
	UpdateCharacterGold(ctx context.Context, id uuid.UUID, gold int32) error

	// Character feature uses
	UpdateCharacterFeatureUses(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error)

	// Character spell slots
	UpdateCharacterSpellSlots(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error)

	// Character pact magic slots
	UpdateCharacterPactMagicSlots(ctx context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error)

	// Spells
	GetSpell(ctx context.Context, id string) (refdata.Spell, error)
	ListSpellsByClass(ctx context.Context, class string) ([]refdata.Spell, error)

	// Character equipment
	UpdateCharacterEquipment(ctx context.Context, arg refdata.UpdateCharacterEquipmentParams) (refdata.Character, error)

	// Character data
	UpdateCharacterData(ctx context.Context, arg refdata.UpdateCharacterDataParams) (refdata.Character, error)

	// Encounter Zones
	CreateEncounterZone(ctx context.Context, arg refdata.CreateEncounterZoneParams) (refdata.EncounterZone, error)
	GetEncounterZone(ctx context.Context, id uuid.UUID) (refdata.EncounterZone, error)
	ListEncounterZonesByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.EncounterZone, error)
	ListConcentrationZonesByCombatant(ctx context.Context, sourceCombatantID uuid.UUID) ([]refdata.EncounterZone, error)
	DeleteEncounterZone(ctx context.Context, id uuid.UUID) error
	DeleteEncounterZonesByEncounterID(ctx context.Context, encounterID uuid.UUID) error
	DeleteConcentrationZonesByCombatant(ctx context.Context, sourceCombatantID uuid.UUID) error
	DeleteExpiredZones(ctx context.Context, arg refdata.DeleteExpiredZonesParams) error
	UpdateEncounterZoneOrigin(ctx context.Context, arg refdata.UpdateEncounterZoneOriginParams) (refdata.EncounterZone, error)
	UpdateEncounterZoneTriggeredThisRound(ctx context.Context, arg refdata.UpdateEncounterZoneTriggeredThisRoundParams) (refdata.EncounterZone, error)
	ResetAllTriggeredThisRound(ctx context.Context, encounterID uuid.UUID) error

	// Reaction Declarations
	CreateReactionDeclaration(ctx context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error)
	CreateReadiedActionDeclaration(ctx context.Context, arg refdata.CreateReadiedActionDeclarationParams) (refdata.ReactionDeclaration, error)
	GetReactionDeclaration(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error)
	ListActiveReactionDeclarationsByEncounter(ctx context.Context, encounterID uuid.UUID) ([]refdata.ReactionDeclaration, error)
	ListReactionDeclarationsByCombatant(ctx context.Context, arg refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error)
	ListActiveReactionDeclarationsByCombatant(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error)
	UpdateReactionDeclarationStatusUsed(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error)
	CancelReactionDeclaration(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error)
	CancelAllReactionDeclarationsByCombatant(ctx context.Context, arg refdata.CancelAllReactionDeclarationsByCombatantParams) error
	DeleteReactionDeclarationsByEncounter(ctx context.Context, encounterID uuid.UUID) error

	// Counterspell
	UpdateReactionDeclarationCounterspellPrompt(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellPromptParams) (refdata.ReactionDeclaration, error)
	UpdateReactionDeclarationCounterspellResolved(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error)

	// Pending Actions
	CreatePendingAction(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error)
	GetPendingAction(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error)
	GetPendingActionByCombatant(ctx context.Context, combatantID uuid.UUID) (refdata.PendingAction, error)
	UpdatePendingActionStatus(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error)
	UpdatePendingActionDMQueueMessage(ctx context.Context, arg refdata.UpdatePendingActionDMQueueMessageParams) (refdata.PendingAction, error)
	CancelAllPendingActionsByCombatant(ctx context.Context, arg refdata.CancelAllPendingActionsByCombatantParams) error

	// Pending Saves
	CreatePendingSave(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error)
	GetPendingSave(ctx context.Context, id uuid.UUID) (refdata.PendingSafe, error)
	ListPendingSavesByCombatant(ctx context.Context, combatantID uuid.UUID) ([]refdata.PendingSafe, error)
	ListPendingSavesByEncounter(ctx context.Context, encounterID uuid.UUID) ([]refdata.PendingSafe, error)
	UpdatePendingSaveResult(ctx context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error)
	CancelAllPendingSavesByCombatant(ctx context.Context, arg refdata.CancelAllPendingSavesByCombatantParams) error

	// Turn Timer
	ListTurnsNeedingNudge(ctx context.Context) ([]refdata.Turn, error)
	ListTurnsNeedingWarning(ctx context.Context) ([]refdata.Turn, error)
	UpdateTurnNudgeSent(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	UpdateTurnWarningSent(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	UpdateTurnTimeout(ctx context.Context, arg refdata.UpdateTurnTimeoutParams) (refdata.Turn, error)
	ListActiveTurnsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Turn, error)
	ClearTurnTimeout(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	SetTurnTimeout(ctx context.Context, arg refdata.SetTurnTimeoutParams) (refdata.Turn, error)

	// Campaign lookup from encounter
	GetCampaignByEncounterID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)

	// Turn Timeout Resolution (Phase 76b)
	ListTurnsTimedOut(ctx context.Context) ([]refdata.Turn, error)
	UpdateTurnDMDecisionSent(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	ListTurnsNeedingDMAutoResolve(ctx context.Context) ([]refdata.Turn, error)
	UpdateTurnAutoResolved(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	UpdateTurnWaitExtended(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	ResetTurnNudgeAndWarning(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	UpdateCombatantAutoResolveCount(ctx context.Context, arg refdata.UpdateCombatantAutoResolveCountParams) (refdata.Combatant, error)
	ResetCombatantAutoResolveCount(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
}

// Service manages combat encounters and their entities.
type Service struct {
	store Store
}

// NewService creates a new combat Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateEncounterInput holds parameters for creating an encounter.
type CreateEncounterInput struct {
	CampaignID  uuid.UUID
	MapID       uuid.NullUUID
	Name        string
	DisplayName string
	TemplateID  uuid.NullUUID
}

// CreateEncounter validates input and creates a new encounter.
func (s *Service) CreateEncounter(ctx context.Context, input CreateEncounterInput) (refdata.Encounter, error) {
	if input.Name == "" {
		return refdata.Encounter{}, errors.New("name must not be empty")
	}

	enc, err := s.store.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID:  input.CampaignID,
		MapID:       input.MapID,
		Name:        input.Name,
		DisplayName: nullString(input.DisplayName),
		TemplateID:  input.TemplateID,
		Status:      "preparing",
		RoundNumber: 0,
	})
	if err != nil {
		return refdata.Encounter{}, fmt.Errorf("creating encounter: %w", err)
	}
	return enc, nil
}

// GetEncounter retrieves an encounter by its ID.
func (s *Service) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return s.store.GetEncounter(ctx, id)
}

// ListEncountersByCampaignID lists all encounters for a campaign.
func (s *Service) ListEncountersByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error) {
	return s.store.ListEncountersByCampaignID(ctx, campaignID)
}

// UpdateEncounterStatus changes the status of an encounter.
func (s *Service) UpdateEncounterStatus(ctx context.Context, id uuid.UUID, status string) (refdata.Encounter, error) {
	if !validStatuses[status] {
		return refdata.Encounter{}, fmt.Errorf("invalid status %q: must be preparing, active, or completed", status)
	}

	return s.store.UpdateEncounterStatus(ctx, refdata.UpdateEncounterStatusParams{
		ID:     id,
		Status: status,
	})
}

// DeleteEncounter deletes an encounter by its ID.
func (s *Service) DeleteEncounter(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteEncounter(ctx, id)
}

// AddCombatant creates a combatant in the given encounter from CombatantParams.
func (s *Service) AddCombatant(ctx context.Context, encounterID uuid.UUID, params CombatantParams) (refdata.Combatant, error) {
	charID := uuid.NullUUID{}
	if params.CharacterID != "" {
		parsed, err := uuid.Parse(params.CharacterID)
		if err != nil {
			return refdata.Combatant{}, fmt.Errorf("parsing character_id: %w", err)
		}
		charID = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	c, err := s.store.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID:     encounterID,
		CharacterID:     charID,
		CreatureRefID:   nullString(params.CreatureRefID),
		ShortID:         params.ShortID,
		DisplayName:     params.DisplayName,
		InitiativeRoll:  0,
		InitiativeOrder: 0,
		PositionCol:     params.PositionCol,
		PositionRow:     params.PositionRow,
		HpMax:           params.HPMax,
		HpCurrent:       params.HPCurrent,
		TempHp:          params.TempHP,
		Ac:              params.AC,
		Conditions:      json.RawMessage(`[]`),
		DeathSaves:      nullRawMessage(params.DeathSaves),
		IsVisible:       params.IsVisible,
		IsAlive:         params.IsAlive,
		IsNpc:           params.IsNPC,
	})
	if err != nil {
		return refdata.Combatant{}, fmt.Errorf("creating combatant: %w", err)
	}
	return c, nil
}

// GetCombatant retrieves a combatant by its ID.
func (s *Service) GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return s.store.GetCombatant(ctx, id)
}

// ListCombatantsByEncounterID lists all combatants for an encounter.
func (s *Service) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return s.store.ListCombatantsByEncounterID(ctx, encounterID)
}

// UpdateCombatantHP updates a combatant's hit points.
func (s *Service) UpdateCombatantHP(ctx context.Context, id uuid.UUID, hpCurrent, tempHP int32, isAlive bool) (refdata.Combatant, error) {
	return s.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
		ID:        id,
		HpCurrent: hpCurrent,
		TempHp:    tempHP,
		IsAlive:   isAlive,
	})
}

// UpdateCombatantPosition updates a combatant's position.
func (s *Service) UpdateCombatantPosition(ctx context.Context, id uuid.UUID, col string, row, altitude int32) (refdata.Combatant, error) {
	return s.store.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
		ID:          id,
		PositionCol: col,
		PositionRow: row,
		AltitudeFt:  altitude,
	})
}

// UpdateCombatantConditions updates a combatant's conditions and exhaustion.
func (s *Service) UpdateCombatantConditions(ctx context.Context, id uuid.UUID, conditions json.RawMessage, exhaustion int32) (refdata.Combatant, error) {
	return s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              id,
		Conditions:      conditions,
		ExhaustionLevel: exhaustion,
	})
}

// DeleteCombatant deletes a combatant by its ID.
func (s *Service) DeleteCombatant(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteCombatant(ctx, id)
}

// CreateEncounterFromTemplate creates a new encounter and combatants from a template.
func (s *Service) CreateEncounterFromTemplate(ctx context.Context, templateID uuid.UUID) (refdata.Encounter, []refdata.Combatant, error) {
	tmpl, err := s.store.GetEncounterTemplate(ctx, templateID)
	if err != nil {
		return refdata.Encounter{}, nil, fmt.Errorf("getting encounter template: %w", err)
	}

	templateCreatures, err := ParseTemplateCreatures(tmpl.Creatures)
	if err != nil {
		return refdata.Encounter{}, nil, fmt.Errorf("parsing template creatures: %w", err)
	}

	enc, err := s.store.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID:  tmpl.CampaignID,
		MapID:       tmpl.MapID,
		Name:        tmpl.Name,
		DisplayName: tmpl.DisplayName,
		TemplateID:  uuid.NullUUID{UUID: tmpl.ID, Valid: true},
		Status:      "preparing",
		RoundNumber: 0,
	})
	if err != nil {
		return refdata.Encounter{}, nil, fmt.Errorf("creating encounter: %w", err)
	}

	var combatants []refdata.Combatant
	for _, tc := range templateCreatures {
		creature, err := s.store.GetCreature(ctx, tc.CreatureRefID)
		if err != nil {
			return refdata.Encounter{}, nil, fmt.Errorf("getting creature %q: %w", tc.CreatureRefID, err)
		}

		for i := 0; i < tc.Quantity; i++ {
			shortID := tc.ShortID
			displayName := tc.DisplayName
			if tc.Quantity > 1 {
				shortID = fmt.Sprintf("%s%d", tc.ShortID, i+1)
				displayName = fmt.Sprintf("%s %d", tc.DisplayName, i+1)
			}

			params := CombatantFromCreature(creature, shortID, displayName, tc.PositionCol, int32(tc.PositionRow))
			c, err := s.AddCombatant(ctx, enc.ID, params)
			if err != nil {
				return refdata.Encounter{}, nil, fmt.Errorf("creating combatant %s: %w", shortID, err)
			}
			combatants = append(combatants, c)
		}
	}

	return enc, combatants, nil
}

// CreateActionLogInput holds parameters for creating an action log entry.
type CreateActionLogInput struct {
	TurnID      uuid.UUID
	EncounterID uuid.UUID
	ActionType  string
	ActorID     uuid.UUID
	TargetID    uuid.NullUUID
	Description string
	BeforeState json.RawMessage
	AfterState  json.RawMessage
	DiceRolls   json.RawMessage
}

// CreateActionLog validates input and creates an action log entry.
func (s *Service) CreateActionLog(ctx context.Context, input CreateActionLogInput) (refdata.ActionLog, error) {
	if input.ActionType == "" {
		return refdata.ActionLog{}, errors.New("action_type must not be empty")
	}

	return s.store.CreateActionLog(ctx, refdata.CreateActionLogParams{
		TurnID:      input.TurnID,
		EncounterID: input.EncounterID,
		ActionType:  input.ActionType,
		ActorID:     input.ActorID,
		TargetID:    input.TargetID,
		Description: nullString(input.Description),
		BeforeState: input.BeforeState,
		AfterState:  input.AfterState,
		DiceRolls:   nullRawMessage(input.DiceRolls),
	})
}

// ListActionLogByEncounterID lists all action log entries for an encounter.
func (s *Service) ListActionLogByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.ActionLog, error) {
	return s.store.ListActionLogByEncounterID(ctx, encounterID)
}

// ListActionLogByTurnID lists all action log entries for a turn.
func (s *Service) ListActionLogByTurnID(ctx context.Context, turnID uuid.UUID) ([]refdata.ActionLog, error) {
	return s.store.ListActionLogByTurnID(ctx, turnID)
}

// ShortIDFromName generates a short ID from a character name (first 2 letters uppercase).
func ShortIDFromName(name string) string {
	if len(name) < 2 {
		return strings.ToUpper(name)
	}
	return strings.ToUpper(name[:2])
}

// StartCombat orchestrates the full start-combat flow:
// create encounter from template, add PCs, mark surprised, roll initiative, advance to first turn.
func (s *Service) StartCombat(ctx context.Context, input StartCombatInput, roller *dice.Roller) (StartCombatResult, error) {
	// Step 1: Create encounter + creature combatants from template
	enc, _, err := s.CreateEncounterFromTemplate(ctx, input.TemplateID)
	if err != nil {
		return StartCombatResult{}, fmt.Errorf("creating encounter from template: %w", err)
	}

	// Step 2: Add PC combatants
	for _, charID := range input.CharacterIDs {
		char, err := s.store.GetCharacter(ctx, charID)
		if err != nil {
			return StartCombatResult{}, fmt.Errorf("getting character %s: %w", charID, err)
		}

		pos := input.CharacterPositions[charID]
		shortID := ShortIDFromName(char.Name)
		params := CombatantFromCharacter(char, shortID, pos.Col, pos.Row)

		if _, err := s.AddCombatant(ctx, enc.ID, params); err != nil {
			return StartCombatResult{}, fmt.Errorf("adding character combatant %s: %w", char.Name, err)
		}
	}

	// Step 3: Resolve surprised short IDs to combatant UUIDs and mark surprised
	if err := s.markSurprisedByShortIDs(ctx, enc.ID, input.SurprisedShortIDs); err != nil {
		return StartCombatResult{}, err
	}

	// Step 4: Roll initiative
	sortedCombatants, err := s.RollInitiative(ctx, enc.ID, roller)
	if err != nil {
		return StartCombatResult{}, fmt.Errorf("rolling initiative: %w", err)
	}

	// Step 5: Advance to first turn
	turnInfo, err := s.AdvanceTurn(ctx, enc.ID)
	if err != nil {
		return StartCombatResult{}, fmt.Errorf("advancing to first turn: %w", err)
	}

	// Re-fetch encounter to get updated state (round, status, current_turn)
	enc, err = s.store.GetEncounter(ctx, enc.ID)
	if err != nil {
		return StartCombatResult{}, fmt.Errorf("re-fetching encounter: %w", err)
	}

	return StartCombatResult{
		Encounter:         enc,
		Combatants:        sortedCombatants,
		InitiativeTracker: FormatInitiativeTracker(enc, sortedCombatants, turnInfo.CombatantID),
		FirstTurn:         turnInfo,
	}, nil
}

func (s *Service) markSurprisedByShortIDs(ctx context.Context, encounterID uuid.UUID, shortIDs []string) error {
	if len(shortIDs) == 0 {
		return nil
	}

	allCombatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return fmt.Errorf("listing combatants for surprise: %w", err)
	}

	shortIDSet := make(map[string]bool, len(shortIDs))
	for _, sid := range shortIDs {
		shortIDSet[sid] = true
	}

	for _, c := range allCombatants {
		if !shortIDSet[c.ShortID] {
			continue
		}
		if err := s.MarkSurprised(ctx, c.ID); err != nil {
			return fmt.Errorf("marking combatant %s surprised: %w", c.ShortID, err)
		}
	}
	return nil
}

// EndCombat validates the encounter is active, sets status to completed, clears combat-only
// conditions from all combatants, completes the active turn, and returns a summary.
func (s *Service) EndCombat(ctx context.Context, encounterID uuid.UUID) (EndCombatResult, error) {
	enc, err := s.store.GetEncounter(ctx, encounterID)
	if err != nil {
		return EndCombatResult{}, fmt.Errorf("getting encounter: %w", err)
	}
	if enc.Status != "active" {
		return EndCombatResult{}, fmt.Errorf("encounter is %q: %w", enc.Status, ErrEncounterNotActive)
	}

	// Complete active turn if any
	if enc.CurrentTurnID.Valid {
		if _, err := s.store.CompleteTurn(ctx, enc.CurrentTurnID.UUID); err != nil {
			return EndCombatResult{}, fmt.Errorf("completing active turn: %w", err)
		}
	}

	// Clean up all encounter zones
	if err := s.store.DeleteEncounterZonesByEncounterID(ctx, encounterID); err != nil {
		return EndCombatResult{}, fmt.Errorf("cleaning up encounter zones: %w", err)
	}

	// Clean up all reaction declarations
	if err := s.CleanupReactionsOnEncounterEnd(ctx, encounterID); err != nil {
		return EndCombatResult{}, fmt.Errorf("cleaning up reaction declarations: %w", err)
	}

	// Set status to completed
	enc, err = s.store.UpdateEncounterStatus(ctx, refdata.UpdateEncounterStatusParams{
		ID:     encounterID,
		Status: "completed",
	})
	if err != nil {
		return EndCombatResult{}, fmt.Errorf("setting status to completed: %w", err)
	}

	// List combatants and clear combat conditions
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return EndCombatResult{}, fmt.Errorf("listing combatants: %w", err)
	}

	casualties := 0
	cleaned := make([]refdata.Combatant, len(combatants))
	for i, c := range combatants {
		if !c.IsAlive {
			casualties++
		}
		newConds, err := ClearCombatConditions(c.Conditions)
		if err != nil {
			return EndCombatResult{}, fmt.Errorf("clearing conditions for %s: %w", c.DisplayName, err)
		}
		if string(newConds) != string(c.Conditions) {
			updated, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
				ID:              c.ID,
				Conditions:      newConds,
				ExhaustionLevel: c.ExhaustionLevel,
			})
			if err != nil {
				return EndCombatResult{}, fmt.Errorf("updating conditions for %s: %w", c.DisplayName, err)
			}
			cleaned[i] = updated
		} else {
			cleaned[i] = c
		}
	}

	roundsElapsed := enc.RoundNumber
	summary := fmt.Sprintf("%d rounds, %d casualties", roundsElapsed, casualties)

	return EndCombatResult{
		Encounter:         enc,
		Combatants:        cleaned,
		Summary:           summary,
		Casualties:        casualties,
		RoundsElapsed:     roundsElapsed,
		InitiativeTracker: FormatCompletedInitiativeTracker(enc, cleaned),
	}, nil
}

// AllHostilesDefeated checks if all NPC combatants in the encounter have 0 HP or are not alive.
func (s *Service) AllHostilesDefeated(ctx context.Context, encounterID uuid.UUID) (bool, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return false, fmt.Errorf("listing combatants: %w", err)
	}
	hostileCount := 0
	for _, c := range combatants {
		if !c.IsNpc {
			continue
		}
		hostileCount++
		if c.IsAlive && c.HpCurrent > 0 {
			return false, nil
		}
	}
	return hostileCount > 0, nil
}

// ListCharactersByCampaign returns all characters for a campaign.
func (s *Service) ListCharactersByCampaign(ctx context.Context, campaignID uuid.UUID) ([]refdata.Character, error) {
	return s.store.ListCharactersByCampaign(ctx, campaignID)
}

// nullString converts a string to sql.NullString, treating empty as null.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// nullRawMessage converts a json.RawMessage to pqtype.NullRawMessage, treating empty/nil as null.
func nullRawMessage(raw json.RawMessage) pqtype.NullRawMessage {
	return pqtype.NullRawMessage{RawMessage: raw, Valid: len(raw) > 0}
}
