package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// ErrEncounterNotActive is returned when EndCombat is called on a non-active encounter.
var ErrEncounterNotActive = errors.New("encounter must be active to end combat")

// ErrCharacterAlreadyInActiveEncounter is returned by AddCombatant when a
// character is already a combatant in another active encounter. Phase 105
// enforces the "one active encounter per character" rule at the service layer.
var ErrCharacterAlreadyInActiveEncounter = errors.New("character is already a combatant in another active encounter")

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
	GetActiveEncounterIDByCharacterID(ctx context.Context, characterID uuid.NullUUID) (uuid.UUID, error)
	UpdateEncounterStatus(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error)
	UpdateEncounterCurrentTurn(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error)
	UpdateEncounterRound(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error)
	UpdateEncounterDisplayName(ctx context.Context, arg refdata.UpdateEncounterDisplayNameParams) (refdata.Encounter, error)
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

	// Action Log. Phase 119: errors moved out of action_log into the
	// dedicated error_log table, so action_log columns turn_id/encounter_id/
	// actor_id are NOT NULL again and the sqlc signatures are uuid.UUID.
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
	DeleteConcentrationZonesByCombatant(ctx context.Context, sourceCombatantID uuid.UUID) (int64, error)
	DeleteExpiredZones(ctx context.Context, arg refdata.DeleteExpiredZonesParams) error
	UpdateEncounterZoneOrigin(ctx context.Context, arg refdata.UpdateEncounterZoneOriginParams) (refdata.EncounterZone, error)
	UpdateEncounterZoneTriggeredThisRound(ctx context.Context, arg refdata.UpdateEncounterZoneTriggeredThisRoundParams) (refdata.EncounterZone, error)
	ResetAllTriggeredThisRound(ctx context.Context, encounterID uuid.UUID) error

	// Reaction Declarations
	CreateReactionDeclaration(ctx context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error)
	CreateReadiedActionDeclaration(ctx context.Context, arg refdata.CreateReadiedActionDeclarationParams) (refdata.ReactionDeclaration, error)
	GetReactionDeclaration(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error)
	ListActiveReactionDeclarationsByEncounter(ctx context.Context, encounterID uuid.UUID) ([]refdata.ReactionDeclaration, error)
	ListReactionDeclarationsByEncounter(ctx context.Context, encounterID uuid.UUID) ([]refdata.ReactionDeclaration, error)
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
	ListPendingActionsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.PendingAction, error)
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

	// Impact Summary
	GetLastCompletedTurnByCombatant(ctx context.Context, arg refdata.GetLastCompletedTurnByCombatantParams) (refdata.Turn, error)
	ListActionLogSinceTurn(ctx context.Context, arg refdata.ListActionLogSinceTurnParams) ([]refdata.ActionLog, error)

	// Recap. Phase 119: encounter_id is uuid.UUID again (action_log is
	// combat-only; error rows live in error_log).
	ListActionLogWithRounds(ctx context.Context, encounterID uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error)
	GetMostRecentCompletedEncounter(ctx context.Context, campaignID uuid.UUID) (refdata.Encounter, error)

	// Player Characters
	GetPlayerCharacterByCharacter(ctx context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error)

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

	// Phase 118 — Concentration Cleanup Integration.
	SetCombatantConcentration(ctx context.Context, arg refdata.SetCombatantConcentrationParams) error
	ClearCombatantConcentration(ctx context.Context, id uuid.UUID) error
	GetCombatantConcentration(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error)
}

// EncounterPublisher fans out a fresh encounter snapshot over the dashboard
// WebSocket hub whenever combat state changes. It is injected (optionally)
// into Service so services can stay decoupled from the concrete
// dashboard.Publisher and so tests can use a fake.
type EncounterPublisher interface {
	PublishEncounterSnapshot(ctx context.Context, encounterID uuid.UUID) error
}

// DMNotifier is the minimal subset of dmqueue.Notifier the combat service
// uses for posting freeform actions and cancelling them. Defined locally to
// avoid an import-cycle hazard and to keep this file's surface narrow.
type DMNotifier interface {
	Post(ctx context.Context, e dmqueue.Event) (string, error)
	Cancel(ctx context.Context, itemID, reason string) error
}

// CardUpdater is the minimal callback the combat service fires after a
// successful HP / condition / concentration mutation so the persistent
// #character-cards message stays in sync with live combat state. Defined
// locally to avoid an import-cycle hazard with charactercard.
//
// charactercard.Service.OnCharacterUpdated satisfies this interface and is
// the production binding wired in cmd/dndnd/main.go. Errors are intentionally
// swallowed by the call site (see Service.notifyCardUpdate); a card-edit
// failure must never undo the underlying combat mutation.
type CardUpdater interface {
	OnCharacterUpdated(ctx context.Context, characterID uuid.UUID) error
}

// TurnStartNotifier is the minimal callback combat.Service fires when a
// new active turn is established by StartCombat (med-20 / Phase 26a).
// Without it, the very first PC's turn would not get a #your-turn ping
// until they complete it via /done. Production wiring in cmd/dndnd
// posts FormatTurnStartPrompt to the combatant's #your-turn channel.
type TurnStartNotifier interface {
	NotifyFirstTurn(ctx context.Context, encounterID uuid.UUID, turnInfo TurnInfo)
}

// InitiativeTrackerNotifier is the minimal callback combat.Service fires
// to keep #initiative-tracker in sync with the live encounter (med-18 /
// Phase 25). PostTracker creates the persistent tracker message after
// RollInitiative; UpdateTracker edits it after every AdvanceTurn;
// PostCompletedTracker posts the final summary after EndCombat. All three
// are no-ops in headless / test deploys (nil notifier or notifier-side
// errors must never roll back the underlying combat mutation).
type InitiativeTrackerNotifier interface {
	PostTracker(ctx context.Context, encounterID uuid.UUID, content string)
	UpdateTracker(ctx context.Context, encounterID uuid.UUID, content string)
	PostCompletedTracker(ctx context.Context, encounterID uuid.UUID, content string)
}

// Service manages combat encounters and their entities.
type Service struct {
	store                     Store
	summonedResources         *SummonedTurnResources
	publisher                 EncounterPublisher
	dmNotifier                DMNotifier
	cardUpdater               CardUpdater
	turnStartNotifier         TurnStartNotifier
	initiativeTrackerNotifier InitiativeTrackerNotifier
}

// NewService creates a new combat Service.
func NewService(store Store) *Service {
	return &Service{
		store:             store,
		summonedResources: NewSummonedTurnResources(),
	}
}

// SetDMNotifier wires the dm-queue notifier so freeform actions and cancels
// route through the unified Phase 106a notification framework. A nil
// notifier disables Notifier dispatch and the legacy DMQueueMessage /
// DMQueueEditMessage strings stay the only output.
func (s *Service) SetDMNotifier(n DMNotifier) {
	s.dmNotifier = n
}

// SetPublisher wires an EncounterPublisher onto the service. A nil publisher
// is tolerated and disables fan-out. Publish errors are logged but never
// surfaced to callers so that a dashboard hiccup cannot undo a committed DB
// mutation.
func (s *Service) SetPublisher(p EncounterPublisher) {
	s.publisher = p
}

// SetCardUpdater wires the character-card auto-update callback (Phase 17).
// A nil updater is tolerated and disables fan-out. Card-edit errors are
// logged but never surfaced to callers so that a Discord hiccup cannot undo
// a committed combat mutation.
func (s *Service) SetCardUpdater(u CardUpdater) {
	s.cardUpdater = u
}

// SetTurnStartNotifier wires the first-turn ping callback (med-20 / Phase
// 26a). A nil notifier is tolerated and disables the fan-out so legacy
// tests / dashboard-only deploys keep working.
func (s *Service) SetTurnStartNotifier(n TurnStartNotifier) {
	s.turnStartNotifier = n
}

// SetInitiativeTrackerNotifier wires the #initiative-tracker auto-post +
// auto-update callbacks (med-18 / Phase 25). A nil notifier disables the
// fan-out (legacy tests / dashboard-only deploys keep working).
func (s *Service) SetInitiativeTrackerNotifier(n InitiativeTrackerNotifier) {
	s.initiativeTrackerNotifier = n
}

// notifyCardUpdate fires the OnCharacterUpdated hook for the given combatant
// when (a) the updater is wired AND (b) the combatant is a player character
// (i.e. carries a non-NULL character_id). NPC / creature combatants have no
// character card to refresh and are silently skipped. Errors are swallowed
// — a stale card is preferable to surfacing a Discord-side failure as a
// combat-mutation rollback.
func (s *Service) notifyCardUpdate(ctx context.Context, c refdata.Combatant) {
	if s.cardUpdater == nil {
		return
	}
	if !c.CharacterID.Valid {
		return
	}
	if err := s.cardUpdater.OnCharacterUpdated(ctx, c.CharacterID.UUID); err != nil {
		log.Printf("character card auto-update failed for %s: %v", c.CharacterID.UUID, err)
	}
}

// notifyCardUpdateByCharacterID fires the OnCharacterUpdated hook directly
// when the caller already holds the character UUID (e.g. equip / level-up
// paths that operate on characters rather than combatants). Mirrors
// notifyCardUpdate's silent-on-error contract.
func (s *Service) notifyCardUpdateByCharacterID(ctx context.Context, characterID uuid.UUID) {
	if s.cardUpdater == nil {
		return
	}
	if characterID == uuid.Nil {
		return
	}
	if err := s.cardUpdater.OnCharacterUpdated(ctx, characterID); err != nil {
		log.Printf("character card auto-update failed for %s: %v", characterID, err)
	}
}

// publish fires the publisher with the given encounter ID, swallowing errors.
// Callers invoke this AFTER a successful DB mutation.
func (s *Service) publish(ctx context.Context, encounterID uuid.UUID) {
	if s.publisher == nil {
		return
	}
	if err := s.publisher.PublishEncounterSnapshot(ctx, encounterID); err != nil {
		log.Printf("encounter snapshot publish failed for %s: %v", encounterID, err)
	}
}

// SummonedResources returns the summoned creature turn resource tracker.
func (s *Service) SummonedResources() *SummonedTurnResources {
	return s.summonedResources
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

// GetTurn retrieves a turn by its ID.
// Phase 115: exposed on Service so the resume turn re-pinger can reconstruct
// the active turn without needing direct store access.
func (s *Service) GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return s.store.GetTurn(ctx, id)
}

// UpdateEncounterStatus changes the status of an encounter.
func (s *Service) UpdateEncounterStatus(ctx context.Context, id uuid.UUID, status string) (refdata.Encounter, error) {
	if !validStatuses[status] {
		return refdata.Encounter{}, fmt.Errorf("invalid status %q: must be preparing, active, or completed", status)
	}

	enc, err := s.store.UpdateEncounterStatus(ctx, refdata.UpdateEncounterStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return refdata.Encounter{}, err
	}
	s.publish(ctx, id)
	return enc, nil
}

// DeleteEncounter deletes an encounter by its ID.
func (s *Service) DeleteEncounter(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteEncounter(ctx, id)
}

// UpdateEncounterDisplayName sets the player-facing display name on an
// encounter. An empty string clears the column to NULL so the internal name
// is used instead. Phase 105 lets the DM swap vague names into the combat
// channels at any time during combat.
func (s *Service) UpdateEncounterDisplayName(ctx context.Context, id uuid.UUID, displayName string) (refdata.Encounter, error) {
	enc, err := s.store.UpdateEncounterDisplayName(ctx, refdata.UpdateEncounterDisplayNameParams{
		ID:          id,
		DisplayName: nullString(displayName),
	})
	if err != nil {
		return refdata.Encounter{}, err
	}
	s.publish(ctx, id)
	return enc, nil
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

		// Phase 105 — enforce the "one active encounter per character" rule.
		// If the character is already a live combatant in another active
		// encounter, refuse. A membership in the same target encounter is
		// fine because the DB row will be re-created idempotently.
		existingID, err := s.store.GetActiveEncounterIDByCharacterID(ctx, charID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return refdata.Combatant{}, fmt.Errorf("checking active encounters for character: %w", err)
		}
		if err == nil && existingID != uuid.Nil && existingID != encounterID {
			return refdata.Combatant{}, ErrCharacterAlreadyInActiveEncounter
		}
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
	c, err := s.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
		ID:        id,
		HpCurrent: hpCurrent,
		TempHp:    tempHP,
		IsAlive:   isAlive,
	})
	if err != nil {
		return refdata.Combatant{}, err
	}
	s.publish(ctx, c.EncounterID)
	return c, nil
}

// UpdateCombatantPosition updates a combatant's position.
func (s *Service) UpdateCombatantPosition(ctx context.Context, id uuid.UUID, col string, row, altitude int32) (refdata.Combatant, error) {
	c, err := s.store.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
		ID:          id,
		PositionCol: col,
		PositionRow: row,
		AltitudeFt:  altitude,
	})
	if err != nil {
		return refdata.Combatant{}, err
	}
	// Phase 118: if the moving combatant is a concentrating caster with a
	// V/S spell and the new tile is inside a Silence zone, break.
	if _, serr := s.CheckSilenceBreaksConcentration(ctx, id); serr != nil {
		return refdata.Combatant{}, fmt.Errorf("silence check on move: %w", serr)
	}
	s.publish(ctx, c.EncounterID)
	return c, nil
}

// UpdateCombatantConditions updates a combatant's conditions and exhaustion.
func (s *Service) UpdateCombatantConditions(ctx context.Context, id uuid.UUID, conditions json.RawMessage, exhaustion int32) (refdata.Combatant, error) {
	c, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              id,
		Conditions:      conditions,
		ExhaustionLevel: exhaustion,
	})
	if err != nil {
		return refdata.Combatant{}, err
	}
	s.publish(ctx, c.EncounterID)
	return c, nil
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
//
// Phase 119: action_log.{turn_id,encounter_id,actor_id} are NOT NULL again —
// errors moved to the dedicated error_log table — so callers must supply
// non-zero parents. The sqlc params take uuid.UUID directly.
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

	// med-20 / Phase 26a: ping the first combatant so they don't sit in
	// silence until someone runs /done. Best-effort: a nil notifier or a
	// notifier-side error must never roll back the encounter creation
	// (the encounter is already persisted at this point).
	if s.turnStartNotifier != nil {
		s.turnStartNotifier.NotifyFirstTurn(ctx, enc.ID, turnInfo)
	}

	tracker := FormatInitiativeTracker(enc, sortedCombatants, turnInfo.CombatantID)

	// med-18 / Phase 25: post the persistent #initiative-tracker message
	// once the first turn has been advanced to. The notifier persists the
	// returned message ID so subsequent AdvanceTurn calls can edit it.
	if s.initiativeTrackerNotifier != nil {
		s.initiativeTrackerNotifier.PostTracker(ctx, enc.ID, tracker)
	}

	return StartCombatResult{
		Encounter:         enc,
		Combatants:        sortedCombatants,
		InitiativeTracker: tracker,
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

	// Clean up summoned creature turn resources
	s.summonedResources.Clear()

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

	// med-19 / Phase 26b: end concentration on any lingering spells before
	// we tear down the encounter. Iterate on the pre-cleanup snapshot so
	// concentration columns are still authoritative. Failures are non-fatal:
	// log via best-effort (the surrounding tx isn't critical to combat
	// completion) — but a downstream error here should still bubble up so
	// callers see the cleanup hiccup.
	for _, c := range combatants {
		if !c.ConcentrationSpellID.Valid || !c.ConcentrationSpellName.Valid {
			continue
		}
		if _, err := s.BreakConcentrationFully(ctx, BreakConcentrationFullyInput{
			EncounterID: encounterID,
			CasterID:    c.ID,
			CasterName:  c.DisplayName,
			SpellID:     c.ConcentrationSpellID.String,
			SpellName:   c.ConcentrationSpellName.String,
			Reason:      "combat ended",
		}); err != nil {
			return EndCombatResult{}, fmt.Errorf("ending concentration for %s: %w", c.DisplayName, err)
		}
	}

	// med-19 / Phase 26b: pause all combat timers so any pending CON-save
	// timers / turn timeouts don't fire after the encounter is over. This
	// is the timer counterpart to clearing combat-only conditions below.
	if err := s.PauseCombatTimers(ctx, encounterID); err != nil {
		return EndCombatResult{}, fmt.Errorf("pausing combat timers: %w", err)
	}

	// NOTE: med-19 also calls for RecoverAmmunition to add back half of
	// each PC's spent arrows/bolts. That requires a per-encounter spent
	// counter (new column on combatants or turns) — deferred as a separate
	// schema migration. The helper exists at attack.go:212 ready to call.

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

	// Publish a final snapshot so dashboard subscribers see "completed".
	s.publish(ctx, encounterID)

	completedTracker := FormatCompletedInitiativeTracker(enc, cleaned)

	// med-18 / Phase 25: post the final completed tracker once. Best-effort:
	// a notifier-side error is silently swallowed so a Discord hiccup
	// cannot undo a successfully-ended combat.
	if s.initiativeTrackerNotifier != nil {
		s.initiativeTrackerNotifier.PostCompletedTracker(ctx, encounterID, completedTracker)
	}

	return EndCombatResult{
		Encounter:         enc,
		Combatants:        cleaned,
		Summary:           summary,
		Casualties:        casualties,
		RoundsElapsed:     roundsElapsed,
		InitiativeTracker: completedTracker,
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
