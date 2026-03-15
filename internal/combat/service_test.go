package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// mockStore implements Store for unit tests.
type mockStore struct {
	createEncounterFn             func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error)
	getEncounterFn                func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	listEncountersByCampaignIDFn  func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error)
	updateEncounterStatusFn       func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error)
	updateEncounterCurrentTurnFn  func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error)
	updateEncounterRoundFn        func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error)
	deleteEncounterFn             func(ctx context.Context, id uuid.UUID) error
	createCombatantFn             func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error)
	getCombatantFn                func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	listCombatantsByEncounterIDFn func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	updateCombatantHPFn           func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error)
	updateCombatantConditionsFn   func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error)
	updateCombatantPositionFn     func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error)
	updateCombatantDeathSavesFn   func(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error)
	updateCombatantVisibilityFn   func(ctx context.Context, arg refdata.UpdateCombatantVisibilityParams) (refdata.Combatant, error)
	deleteCombatantFn             func(ctx context.Context, id uuid.UUID) error
	createTurnFn                  func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error)
	getTurnFn                     func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	getActiveTurnByEncounterIDFn  func(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error)
	completeTurnFn                func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	createActionLogFn             func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error)
	listActionLogByEncounterIDFn  func(ctx context.Context, encounterID uuid.UUID) ([]refdata.ActionLog, error)
	listActionLogByTurnIDFn       func(ctx context.Context, turnID uuid.UUID) ([]refdata.ActionLog, error)
	getEncounterTemplateFn            func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error)
	getCreatureFn                     func(ctx context.Context, id string) (refdata.Creature, error)
	updateCombatantInitiativeFn       func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error)
	skipTurnFn                        func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	listTurnsByEncounterAndRoundFn    func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error)
	getCharacterFn                    func(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	getClassFn                        func(ctx context.Context, id string) (refdata.Class, error)
	listCharactersByCampaignFn        func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Character, error)
	updateTurnActionsFn               func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error)
	getWeaponFn                       func(ctx context.Context, id string) (refdata.Weapon, error)
	updateCharacterInventoryFn        func(ctx context.Context, id uuid.UUID, inventory pqtype.NullRawMessage) error
	updateCombatantRageFn             func(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error)
	updateCombatantWildShapeFn             func(ctx context.Context, arg refdata.UpdateCombatantWildShapeParams) (refdata.Combatant, error)
	updateCombatantBardicInspirationFn     func(ctx context.Context, arg refdata.UpdateCombatantBardicInspirationParams) (refdata.Combatant, error)
	getArmorFn                             func(ctx context.Context, id string) (refdata.Armor, error)
	updateCharacterFeatureUsesFn      func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error)
	updateCharacterSpellSlotsFn       func(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error)
	updateCharacterPactMagicSlotsFn   func(ctx context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error)
	getSpellFn                        func(ctx context.Context, id string) (refdata.Spell, error)
	updateCharacterGoldFn             func(ctx context.Context, id uuid.UUID, gold int32) error
	listSpellsByClassFn               func(ctx context.Context, class string) ([]refdata.Spell, error)
	updateCharacterEquipmentFn        func(ctx context.Context, arg refdata.UpdateCharacterEquipmentParams) (refdata.Character, error)
	updateCharacterDataFn             func(ctx context.Context, arg refdata.UpdateCharacterDataParams) (refdata.Character, error)

	// Encounter Zones
	createEncounterZoneFn                   func(ctx context.Context, arg refdata.CreateEncounterZoneParams) (refdata.EncounterZone, error)
	getEncounterZoneFn                      func(ctx context.Context, id uuid.UUID) (refdata.EncounterZone, error)
	listEncounterZonesByEncounterIDFn       func(ctx context.Context, encounterID uuid.UUID) ([]refdata.EncounterZone, error)
	listConcentrationZonesByCombatantFn     func(ctx context.Context, sourceCombatantID uuid.UUID) ([]refdata.EncounterZone, error)
	deleteEncounterZoneFn                   func(ctx context.Context, id uuid.UUID) error
	deleteEncounterZonesByEncounterIDFn     func(ctx context.Context, encounterID uuid.UUID) error
	deleteConcentrationZonesByCombatantFn   func(ctx context.Context, sourceCombatantID uuid.UUID) error
	deleteExpiredZonesFn                    func(ctx context.Context, arg refdata.DeleteExpiredZonesParams) error
	updateEncounterZoneOriginFn             func(ctx context.Context, arg refdata.UpdateEncounterZoneOriginParams) (refdata.EncounterZone, error)
	updateEncounterZoneTriggeredThisRoundFn func(ctx context.Context, arg refdata.UpdateEncounterZoneTriggeredThisRoundParams) (refdata.EncounterZone, error)
	resetAllTriggeredThisRoundFn            func(ctx context.Context, encounterID uuid.UUID) error

	// Reaction Declarations
	createReactionDeclarationFn                func(ctx context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error)
	createReadiedActionDeclarationFn           func(ctx context.Context, arg refdata.CreateReadiedActionDeclarationParams) (refdata.ReactionDeclaration, error)
	getReactionDeclarationFn                   func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error)
	listActiveReactionDeclarationsByEncounterFn func(ctx context.Context, encounterID uuid.UUID) ([]refdata.ReactionDeclaration, error)
	listReactionDeclarationsByCombatantFn      func(ctx context.Context, arg refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error)
	listActiveReactionDeclarationsByCombatantFn func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error)
	updateReactionDeclarationStatusUsedFn      func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error)
	cancelReactionDeclarationFn                func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error)
	cancelAllReactionDeclarationsByCombatantFn func(ctx context.Context, arg refdata.CancelAllReactionDeclarationsByCombatantParams) error
	deleteReactionDeclarationsByEncounterFn    func(ctx context.Context, encounterID uuid.UUID) error

	// Counterspell
	updateReactionDeclarationCounterspellPromptFn   func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellPromptParams) (refdata.ReactionDeclaration, error)
	updateReactionDeclarationCounterspellResolvedFn func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error)

	// Pending Actions
	createPendingActionFn                  func(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error)
	getPendingActionFn                     func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error)
	cancelAllPendingActionsByCombatantFn   func(ctx context.Context, arg refdata.CancelAllPendingActionsByCombatantParams) error

	// Pending Saves
	createPendingSaveFn                    func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error)
	getPendingSaveFn                       func(ctx context.Context, id uuid.UUID) (refdata.PendingSafe, error)
	listPendingSavesByCombatantFn          func(ctx context.Context, combatantID uuid.UUID) ([]refdata.PendingSafe, error)
	listPendingSavesByEncounterFn          func(ctx context.Context, encounterID uuid.UUID) ([]refdata.PendingSafe, error)
	updatePendingSaveResultFn              func(ctx context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error)
	cancelAllPendingSavesByCombatantFn     func(ctx context.Context, arg refdata.CancelAllPendingSavesByCombatantParams) error

	// Turn Timer
	listTurnsNeedingNudgeFn          func(ctx context.Context) ([]refdata.Turn, error)
	listTurnsNeedingWarningFn        func(ctx context.Context) ([]refdata.Turn, error)
	updateTurnNudgeSentFn            func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	updateTurnWarningSentFn          func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	updateTurnTimeoutFn              func(ctx context.Context, arg refdata.UpdateTurnTimeoutParams) (refdata.Turn, error)
	listActiveTurnsByEncounterIDFn   func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Turn, error)
	clearTurnTimeoutFn               func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	setTurnTimeoutFn                 func(ctx context.Context, arg refdata.SetTurnTimeoutParams) (refdata.Turn, error)
	getCampaignByEncounterIDFn       func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
	getPendingActionByCombatantFn     func(ctx context.Context, combatantID uuid.UUID) (refdata.PendingAction, error)
	updatePendingActionStatusFn       func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error)
	updatePendingActionDMQueueMessageFn func(ctx context.Context, arg refdata.UpdatePendingActionDMQueueMessageParams) (refdata.PendingAction, error)

	// Turn Timeout Resolution (Phase 76b)
	listTurnsTimedOutFn                func(ctx context.Context) ([]refdata.Turn, error)
	updateTurnDMDecisionSentFn         func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	listTurnsNeedingDMAutoResolveFn    func(ctx context.Context) ([]refdata.Turn, error)
	updateTurnAutoResolvedFn           func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	updateTurnWaitExtendedFn           func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	resetTurnNudgeAndWarningFn         func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	updateCombatantAutoResolveCountFn  func(ctx context.Context, arg refdata.UpdateCombatantAutoResolveCountParams) (refdata.Combatant, error)
	resetCombatantAutoResolveCountFn   func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
}

func (m *mockStore) CreateEncounter(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
	return m.createEncounterFn(ctx, arg)
}
func (m *mockStore) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return m.getEncounterFn(ctx, id)
}
func (m *mockStore) ListEncountersByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error) {
	return m.listEncountersByCampaignIDFn(ctx, campaignID)
}
func (m *mockStore) UpdateEncounterStatus(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
	return m.updateEncounterStatusFn(ctx, arg)
}
func (m *mockStore) UpdateEncounterCurrentTurn(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
	return m.updateEncounterCurrentTurnFn(ctx, arg)
}
func (m *mockStore) UpdateEncounterRound(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
	return m.updateEncounterRoundFn(ctx, arg)
}
func (m *mockStore) DeleteEncounter(ctx context.Context, id uuid.UUID) error {
	return m.deleteEncounterFn(ctx, id)
}
func (m *mockStore) CreateCombatant(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
	return m.createCombatantFn(ctx, arg)
}
func (m *mockStore) GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return m.getCombatantFn(ctx, id)
}
func (m *mockStore) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return m.listCombatantsByEncounterIDFn(ctx, encounterID)
}
func (m *mockStore) UpdateCombatantHP(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
	return m.updateCombatantHPFn(ctx, arg)
}
func (m *mockStore) UpdateCombatantConditions(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
	return m.updateCombatantConditionsFn(ctx, arg)
}
func (m *mockStore) UpdateCombatantPosition(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
	return m.updateCombatantPositionFn(ctx, arg)
}
func (m *mockStore) UpdateCombatantDeathSaves(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
	return m.updateCombatantDeathSavesFn(ctx, arg)
}
func (m *mockStore) UpdateCombatantVisibility(ctx context.Context, arg refdata.UpdateCombatantVisibilityParams) (refdata.Combatant, error) {
	return m.updateCombatantVisibilityFn(ctx, arg)
}
func (m *mockStore) DeleteCombatant(ctx context.Context, id uuid.UUID) error {
	return m.deleteCombatantFn(ctx, id)
}
func (m *mockStore) CreateTurn(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
	return m.createTurnFn(ctx, arg)
}
func (m *mockStore) GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return m.getTurnFn(ctx, id)
}
func (m *mockStore) GetActiveTurnByEncounterID(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error) {
	return m.getActiveTurnByEncounterIDFn(ctx, encounterID)
}
func (m *mockStore) CompleteTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return m.completeTurnFn(ctx, id)
}
func (m *mockStore) CreateActionLog(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
	return m.createActionLogFn(ctx, arg)
}
func (m *mockStore) ListActionLogByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.ActionLog, error) {
	return m.listActionLogByEncounterIDFn(ctx, encounterID)
}
func (m *mockStore) ListActionLogByTurnID(ctx context.Context, turnID uuid.UUID) ([]refdata.ActionLog, error) {
	return m.listActionLogByTurnIDFn(ctx, turnID)
}
func (m *mockStore) GetEncounterTemplate(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
	return m.getEncounterTemplateFn(ctx, id)
}
func (m *mockStore) GetCreature(ctx context.Context, id string) (refdata.Creature, error) {
	return m.getCreatureFn(ctx, id)
}
func (m *mockStore) UpdateCombatantInitiative(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
	return m.updateCombatantInitiativeFn(ctx, arg)
}
func (m *mockStore) SkipTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return m.skipTurnFn(ctx, id)
}
func (m *mockStore) ListTurnsByEncounterAndRound(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
	return m.listTurnsByEncounterAndRoundFn(ctx, arg)
}
func (m *mockStore) GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
	return m.getCharacterFn(ctx, id)
}
func (m *mockStore) ListCharactersByCampaign(ctx context.Context, campaignID uuid.UUID) ([]refdata.Character, error) {
	return m.listCharactersByCampaignFn(ctx, campaignID)
}
func (m *mockStore) GetClass(ctx context.Context, id string) (refdata.Class, error) {
	return m.getClassFn(ctx, id)
}
func (m *mockStore) UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	return m.updateTurnActionsFn(ctx, arg)
}
func (m *mockStore) GetWeapon(ctx context.Context, id string) (refdata.Weapon, error) {
	return m.getWeaponFn(ctx, id)
}
func (m *mockStore) UpdateCharacterInventory(ctx context.Context, id uuid.UUID, inventory pqtype.NullRawMessage) error {
	return m.updateCharacterInventoryFn(ctx, id, inventory)
}
func (m *mockStore) UpdateCombatantRage(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
	return m.updateCombatantRageFn(ctx, arg)
}
func (m *mockStore) UpdateCombatantWildShape(ctx context.Context, arg refdata.UpdateCombatantWildShapeParams) (refdata.Combatant, error) {
	return m.updateCombatantWildShapeFn(ctx, arg)
}
func (m *mockStore) UpdateCombatantBardicInspiration(ctx context.Context, arg refdata.UpdateCombatantBardicInspirationParams) (refdata.Combatant, error) {
	return m.updateCombatantBardicInspirationFn(ctx, arg)
}
func (m *mockStore) GetArmor(ctx context.Context, id string) (refdata.Armor, error) {
	return m.getArmorFn(ctx, id)
}
func (m *mockStore) UpdateCharacterFeatureUses(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
	return m.updateCharacterFeatureUsesFn(ctx, arg)
}
func (m *mockStore) UpdateCharacterSpellSlots(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
	if m.updateCharacterSpellSlotsFn != nil {
		return m.updateCharacterSpellSlotsFn(ctx, arg)
	}
	return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
}
func (m *mockStore) UpdateCharacterPactMagicSlots(ctx context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error) {
	if m.updateCharacterPactMagicSlotsFn != nil {
		return m.updateCharacterPactMagicSlotsFn(ctx, arg)
	}
	return refdata.Character{ID: arg.ID, PactMagicSlots: arg.PactMagicSlots}, nil
}
func (m *mockStore) GetSpell(ctx context.Context, id string) (refdata.Spell, error) {
	if m.getSpellFn != nil {
		return m.getSpellFn(ctx, id)
	}
	return refdata.Spell{}, fmt.Errorf("spell %q not found", id)
}
func (m *mockStore) UpdateCharacterGold(ctx context.Context, id uuid.UUID, gold int32) error {
	if m.updateCharacterGoldFn != nil {
		return m.updateCharacterGoldFn(ctx, id, gold)
	}
	return nil
}
func (m *mockStore) ListSpellsByClass(ctx context.Context, class string) ([]refdata.Spell, error) {
	if m.listSpellsByClassFn != nil {
		return m.listSpellsByClassFn(ctx, class)
	}
	return nil, nil
}
func (m *mockStore) UpdateCharacterEquipment(ctx context.Context, arg refdata.UpdateCharacterEquipmentParams) (refdata.Character, error) {
	if m.updateCharacterEquipmentFn != nil {
		return m.updateCharacterEquipmentFn(ctx, arg)
	}
	return refdata.Character{ID: arg.ID, EquippedMainHand: arg.EquippedMainHand, EquippedOffHand: arg.EquippedOffHand, EquippedArmor: arg.EquippedArmor, Ac: arg.Ac}, nil
}
func (m *mockStore) UpdateCharacterData(ctx context.Context, arg refdata.UpdateCharacterDataParams) (refdata.Character, error) {
	if m.updateCharacterDataFn != nil {
		return m.updateCharacterDataFn(ctx, arg)
	}
	return refdata.Character{ID: arg.ID, CharacterData: arg.CharacterData}, nil
}
func (m *mockStore) CreateEncounterZone(ctx context.Context, arg refdata.CreateEncounterZoneParams) (refdata.EncounterZone, error) {
	if m.createEncounterZoneFn != nil {
		return m.createEncounterZoneFn(ctx, arg)
	}
	return refdata.EncounterZone{}, nil
}
func (m *mockStore) GetEncounterZone(ctx context.Context, id uuid.UUID) (refdata.EncounterZone, error) {
	if m.getEncounterZoneFn != nil {
		return m.getEncounterZoneFn(ctx, id)
	}
	return refdata.EncounterZone{}, nil
}
func (m *mockStore) ListEncounterZonesByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.EncounterZone, error) {
	if m.listEncounterZonesByEncounterIDFn != nil {
		return m.listEncounterZonesByEncounterIDFn(ctx, encounterID)
	}
	return []refdata.EncounterZone{}, nil
}
func (m *mockStore) ListConcentrationZonesByCombatant(ctx context.Context, sourceCombatantID uuid.UUID) ([]refdata.EncounterZone, error) {
	if m.listConcentrationZonesByCombatantFn != nil {
		return m.listConcentrationZonesByCombatantFn(ctx, sourceCombatantID)
	}
	return []refdata.EncounterZone{}, nil
}
func (m *mockStore) DeleteEncounterZone(ctx context.Context, id uuid.UUID) error {
	if m.deleteEncounterZoneFn != nil {
		return m.deleteEncounterZoneFn(ctx, id)
	}
	return nil
}
func (m *mockStore) DeleteEncounterZonesByEncounterID(ctx context.Context, encounterID uuid.UUID) error {
	if m.deleteEncounterZonesByEncounterIDFn != nil {
		return m.deleteEncounterZonesByEncounterIDFn(ctx, encounterID)
	}
	return nil
}
func (m *mockStore) DeleteConcentrationZonesByCombatant(ctx context.Context, sourceCombatantID uuid.UUID) error {
	if m.deleteConcentrationZonesByCombatantFn != nil {
		return m.deleteConcentrationZonesByCombatantFn(ctx, sourceCombatantID)
	}
	return nil
}
func (m *mockStore) DeleteExpiredZones(ctx context.Context, arg refdata.DeleteExpiredZonesParams) error {
	if m.deleteExpiredZonesFn != nil {
		return m.deleteExpiredZonesFn(ctx, arg)
	}
	return nil
}
func (m *mockStore) UpdateEncounterZoneOrigin(ctx context.Context, arg refdata.UpdateEncounterZoneOriginParams) (refdata.EncounterZone, error) {
	if m.updateEncounterZoneOriginFn != nil {
		return m.updateEncounterZoneOriginFn(ctx, arg)
	}
	return refdata.EncounterZone{}, nil
}
func (m *mockStore) UpdateEncounterZoneTriggeredThisRound(ctx context.Context, arg refdata.UpdateEncounterZoneTriggeredThisRoundParams) (refdata.EncounterZone, error) {
	if m.updateEncounterZoneTriggeredThisRoundFn != nil {
		return m.updateEncounterZoneTriggeredThisRoundFn(ctx, arg)
	}
	return refdata.EncounterZone{}, nil
}
func (m *mockStore) ResetAllTriggeredThisRound(ctx context.Context, encounterID uuid.UUID) error {
	if m.resetAllTriggeredThisRoundFn != nil {
		return m.resetAllTriggeredThisRoundFn(ctx, encounterID)
	}
	return nil
}
func (m *mockStore) CreateReactionDeclaration(ctx context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
	if m.createReactionDeclarationFn != nil {
		return m.createReactionDeclarationFn(ctx, arg)
	}
	return refdata.ReactionDeclaration{}, nil
}
func (m *mockStore) CreateReadiedActionDeclaration(ctx context.Context, arg refdata.CreateReadiedActionDeclarationParams) (refdata.ReactionDeclaration, error) {
	if m.createReadiedActionDeclarationFn != nil {
		return m.createReadiedActionDeclarationFn(ctx, arg)
	}
	return refdata.ReactionDeclaration{}, nil
}
func (m *mockStore) GetReactionDeclaration(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
	if m.getReactionDeclarationFn != nil {
		return m.getReactionDeclarationFn(ctx, id)
	}
	return refdata.ReactionDeclaration{}, nil
}
func (m *mockStore) ListActiveReactionDeclarationsByEncounter(ctx context.Context, encounterID uuid.UUID) ([]refdata.ReactionDeclaration, error) {
	if m.listActiveReactionDeclarationsByEncounterFn != nil {
		return m.listActiveReactionDeclarationsByEncounterFn(ctx, encounterID)
	}
	return []refdata.ReactionDeclaration{}, nil
}
func (m *mockStore) ListReactionDeclarationsByCombatant(ctx context.Context, arg refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
	if m.listReactionDeclarationsByCombatantFn != nil {
		return m.listReactionDeclarationsByCombatantFn(ctx, arg)
	}
	return []refdata.ReactionDeclaration{}, nil
}
func (m *mockStore) ListActiveReactionDeclarationsByCombatant(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
	if m.listActiveReactionDeclarationsByCombatantFn != nil {
		return m.listActiveReactionDeclarationsByCombatantFn(ctx, arg)
	}
	return []refdata.ReactionDeclaration{}, nil
}
func (m *mockStore) UpdateReactionDeclarationStatusUsed(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
	if m.updateReactionDeclarationStatusUsedFn != nil {
		return m.updateReactionDeclarationStatusUsedFn(ctx, arg)
	}
	return refdata.ReactionDeclaration{}, nil
}
func (m *mockStore) CancelReactionDeclaration(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
	if m.cancelReactionDeclarationFn != nil {
		return m.cancelReactionDeclarationFn(ctx, id)
	}
	return refdata.ReactionDeclaration{}, nil
}
func (m *mockStore) CancelAllReactionDeclarationsByCombatant(ctx context.Context, arg refdata.CancelAllReactionDeclarationsByCombatantParams) error {
	if m.cancelAllReactionDeclarationsByCombatantFn != nil {
		return m.cancelAllReactionDeclarationsByCombatantFn(ctx, arg)
	}
	return nil
}
func (m *mockStore) DeleteReactionDeclarationsByEncounter(ctx context.Context, encounterID uuid.UUID) error {
	if m.deleteReactionDeclarationsByEncounterFn != nil {
		return m.deleteReactionDeclarationsByEncounterFn(ctx, encounterID)
	}
	return nil
}
func (m *mockStore) UpdateReactionDeclarationCounterspellPrompt(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellPromptParams) (refdata.ReactionDeclaration, error) {
	if m.updateReactionDeclarationCounterspellPromptFn != nil {
		return m.updateReactionDeclarationCounterspellPromptFn(ctx, arg)
	}
	return refdata.ReactionDeclaration{}, nil
}
func (m *mockStore) UpdateReactionDeclarationCounterspellResolved(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
	if m.updateReactionDeclarationCounterspellResolvedFn != nil {
		return m.updateReactionDeclarationCounterspellResolvedFn(ctx, arg)
	}
	return refdata.ReactionDeclaration{}, nil
}
func (m *mockStore) CreatePendingAction(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
	if m.createPendingActionFn != nil {
		return m.createPendingActionFn(ctx, arg)
	}
	return refdata.PendingAction{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, ActionText: arg.ActionText, Status: "pending"}, nil
}
func (m *mockStore) GetPendingAction(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
	if m.getPendingActionFn != nil {
		return m.getPendingActionFn(ctx, id)
	}
	return refdata.PendingAction{}, fmt.Errorf("not found")
}
func (m *mockStore) GetPendingActionByCombatant(ctx context.Context, combatantID uuid.UUID) (refdata.PendingAction, error) {
	if m.getPendingActionByCombatantFn != nil {
		return m.getPendingActionByCombatantFn(ctx, combatantID)
	}
	return refdata.PendingAction{}, fmt.Errorf("no pending action")
}
func (m *mockStore) UpdatePendingActionStatus(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
	if m.updatePendingActionStatusFn != nil {
		return m.updatePendingActionStatusFn(ctx, arg)
	}
	return refdata.PendingAction{ID: arg.ID, Status: arg.Status}, nil
}
func (m *mockStore) UpdatePendingActionDMQueueMessage(ctx context.Context, arg refdata.UpdatePendingActionDMQueueMessageParams) (refdata.PendingAction, error) {
	if m.updatePendingActionDMQueueMessageFn != nil {
		return m.updatePendingActionDMQueueMessageFn(ctx, arg)
	}
	return refdata.PendingAction{ID: arg.ID, DmQueueMessageID: arg.DmQueueMessageID, DmQueueChannelID: arg.DmQueueChannelID}, nil
}
func (m *mockStore) CancelAllPendingActionsByCombatant(ctx context.Context, arg refdata.CancelAllPendingActionsByCombatantParams) error {
	if m.cancelAllPendingActionsByCombatantFn != nil {
		return m.cancelAllPendingActionsByCombatantFn(ctx, arg)
	}
	return nil
}
func (m *mockStore) CreatePendingSave(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
	if m.createPendingSaveFn != nil {
		return m.createPendingSaveFn(ctx, arg)
	}
	return refdata.PendingSafe{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, Ability: arg.Ability, Dc: arg.Dc, Source: arg.Source, Status: "pending"}, nil
}
func (m *mockStore) GetPendingSave(ctx context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
	if m.getPendingSaveFn != nil {
		return m.getPendingSaveFn(ctx, id)
	}
	return refdata.PendingSafe{}, fmt.Errorf("not found")
}
func (m *mockStore) ListPendingSavesByCombatant(ctx context.Context, combatantID uuid.UUID) ([]refdata.PendingSafe, error) {
	if m.listPendingSavesByCombatantFn != nil {
		return m.listPendingSavesByCombatantFn(ctx, combatantID)
	}
	return []refdata.PendingSafe{}, nil
}
func (m *mockStore) ListPendingSavesByEncounter(ctx context.Context, encounterID uuid.UUID) ([]refdata.PendingSafe, error) {
	if m.listPendingSavesByEncounterFn != nil {
		return m.listPendingSavesByEncounterFn(ctx, encounterID)
	}
	return []refdata.PendingSafe{}, nil
}
func (m *mockStore) UpdatePendingSaveResult(ctx context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
	if m.updatePendingSaveResultFn != nil {
		return m.updatePendingSaveResultFn(ctx, arg)
	}
	return refdata.PendingSafe{ID: arg.ID, Status: "rolled", RollResult: arg.RollResult, Success: arg.Success}, nil
}
func (m *mockStore) CancelAllPendingSavesByCombatant(ctx context.Context, arg refdata.CancelAllPendingSavesByCombatantParams) error {
	if m.cancelAllPendingSavesByCombatantFn != nil {
		return m.cancelAllPendingSavesByCombatantFn(ctx, arg)
	}
	return nil
}
func (m *mockStore) ListTurnsNeedingNudge(ctx context.Context) ([]refdata.Turn, error) {
	if m.listTurnsNeedingNudgeFn != nil {
		return m.listTurnsNeedingNudgeFn(ctx)
	}
	return []refdata.Turn{}, nil
}
func (m *mockStore) ListTurnsNeedingWarning(ctx context.Context) ([]refdata.Turn, error) {
	if m.listTurnsNeedingWarningFn != nil {
		return m.listTurnsNeedingWarningFn(ctx)
	}
	return []refdata.Turn{}, nil
}
func (m *mockStore) UpdateTurnNudgeSent(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	if m.updateTurnNudgeSentFn != nil {
		return m.updateTurnNudgeSentFn(ctx, id)
	}
	return refdata.Turn{ID: id}, nil
}
func (m *mockStore) UpdateTurnWarningSent(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	if m.updateTurnWarningSentFn != nil {
		return m.updateTurnWarningSentFn(ctx, id)
	}
	return refdata.Turn{ID: id}, nil
}
func (m *mockStore) UpdateTurnTimeout(ctx context.Context, arg refdata.UpdateTurnTimeoutParams) (refdata.Turn, error) {
	if m.updateTurnTimeoutFn != nil {
		return m.updateTurnTimeoutFn(ctx, arg)
	}
	return refdata.Turn{ID: arg.ID, TimeoutAt: arg.TimeoutAt}, nil
}
func (m *mockStore) ListActiveTurnsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Turn, error) {
	if m.listActiveTurnsByEncounterIDFn != nil {
		return m.listActiveTurnsByEncounterIDFn(ctx, encounterID)
	}
	return []refdata.Turn{}, nil
}
func (m *mockStore) ClearTurnTimeout(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	if m.clearTurnTimeoutFn != nil {
		return m.clearTurnTimeoutFn(ctx, id)
	}
	return refdata.Turn{ID: id}, nil
}
func (m *mockStore) SetTurnTimeout(ctx context.Context, arg refdata.SetTurnTimeoutParams) (refdata.Turn, error) {
	if m.setTurnTimeoutFn != nil {
		return m.setTurnTimeoutFn(ctx, arg)
	}
	return refdata.Turn{ID: arg.ID, TimeoutAt: arg.TimeoutAt}, nil
}
func (m *mockStore) GetCampaignByEncounterID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	if m.getCampaignByEncounterIDFn != nil {
		return m.getCampaignByEncounterIDFn(ctx, id)
	}
	return refdata.Campaign{}, nil
}
func (m *mockStore) ListTurnsTimedOut(ctx context.Context) ([]refdata.Turn, error) {
	if m.listTurnsTimedOutFn != nil {
		return m.listTurnsTimedOutFn(ctx)
	}
	return []refdata.Turn{}, nil
}
func (m *mockStore) UpdateTurnDMDecisionSent(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	if m.updateTurnDMDecisionSentFn != nil {
		return m.updateTurnDMDecisionSentFn(ctx, id)
	}
	return refdata.Turn{ID: id}, nil
}
func (m *mockStore) ListTurnsNeedingDMAutoResolve(ctx context.Context) ([]refdata.Turn, error) {
	if m.listTurnsNeedingDMAutoResolveFn != nil {
		return m.listTurnsNeedingDMAutoResolveFn(ctx)
	}
	return []refdata.Turn{}, nil
}
func (m *mockStore) UpdateTurnAutoResolved(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	if m.updateTurnAutoResolvedFn != nil {
		return m.updateTurnAutoResolvedFn(ctx, id)
	}
	return refdata.Turn{ID: id, AutoResolved: true, Status: "completed"}, nil
}
func (m *mockStore) UpdateTurnWaitExtended(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	if m.updateTurnWaitExtendedFn != nil {
		return m.updateTurnWaitExtendedFn(ctx, id)
	}
	return refdata.Turn{ID: id, WaitExtended: true}, nil
}
func (m *mockStore) ResetTurnNudgeAndWarning(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	if m.resetTurnNudgeAndWarningFn != nil {
		return m.resetTurnNudgeAndWarningFn(ctx, id)
	}
	return refdata.Turn{ID: id}, nil
}
func (m *mockStore) UpdateCombatantAutoResolveCount(ctx context.Context, arg refdata.UpdateCombatantAutoResolveCountParams) (refdata.Combatant, error) {
	if m.updateCombatantAutoResolveCountFn != nil {
		return m.updateCombatantAutoResolveCountFn(ctx, arg)
	}
	return refdata.Combatant{ID: arg.ID, ConsecutiveAutoResolves: arg.ConsecutiveAutoResolves, IsAbsent: arg.IsAbsent}, nil
}
func (m *mockStore) ResetCombatantAutoResolveCount(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	if m.resetCombatantAutoResolveCountFn != nil {
		return m.resetCombatantAutoResolveCountFn(ctx, id)
	}
	return refdata.Combatant{ID: id}, nil
}

func defaultMockStore() *mockStore {
	encounterID := uuid.New()
	combatantID := uuid.New()
	return &mockStore{
		createEncounterFn: func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:         encounterID,
				CampaignID: arg.CampaignID,
				Name:       arg.Name,
				Status:     arg.Status,
			}, nil
		},
		getEncounterFn: func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: id, Name: "Test", Status: "preparing"}, nil
		},
		listEncountersByCampaignIDFn: func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{}, nil
		},
		updateEncounterStatusFn: func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: arg.ID, Status: arg.Status}, nil
		},
		updateEncounterCurrentTurnFn: func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: arg.ID, CurrentTurnID: arg.CurrentTurnID}, nil
		},
		updateEncounterRoundFn: func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: arg.ID, RoundNumber: arg.RoundNumber}, nil
		},
		deleteEncounterFn: func(ctx context.Context, id uuid.UUID) error { return nil },
		createCombatantFn: func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				EncounterID: arg.EncounterID,
				ShortID:     arg.ShortID,
				DisplayName: arg.DisplayName,
				HpMax:       arg.HpMax,
				HpCurrent:   arg.HpCurrent,
				Ac:          arg.Ac,
				IsAlive:     true,
				IsVisible:   true,
				Conditions:  json.RawMessage(`[]`),
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Test", Conditions: json.RawMessage(`[]`)}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		},
		updateCombatantPositionFn: func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, Conditions: json.RawMessage(`[]`)}, nil
		},
		updateCombatantDeathSavesFn: func(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, DeathSaves: arg.DeathSaves, Conditions: json.RawMessage(`[]`)}, nil
		},
		updateCombatantVisibilityFn: func(ctx context.Context, arg refdata.UpdateCombatantVisibilityParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, IsVisible: arg.IsVisible, Conditions: json.RawMessage(`[]`)}, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error { return nil },
		createTurnFn: func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New(), EncounterID: arg.EncounterID, Status: arg.Status}, nil
		},
		getTurnFn: func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: id, Status: "active"}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New(), EncounterID: encounterID, Status: "active"}, nil
		},
		completeTurnFn: func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: id, Status: "completed"}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New(), EncounterID: arg.EncounterID}, nil
		},
		listActionLogByEncounterIDFn: func(ctx context.Context, encounterID uuid.UUID) ([]refdata.ActionLog, error) {
			return []refdata.ActionLog{}, nil
		},
		listActionLogByTurnIDFn: func(ctx context.Context, turnID uuid.UUID) ([]refdata.ActionLog, error) {
			return []refdata.ActionLog{}, nil
		},
		getEncounterTemplateFn: func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, sql.ErrNoRows
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{}, sql.ErrNoRows
		},
		updateCombatantInitiativeFn: func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder}, nil
		},
		skipTurnFn: func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: id, Status: "skipped"}, nil
		},
		listTurnsByEncounterAndRoundFn: func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
			return []refdata.Turn{}, nil
		},
		getCharacterFn: func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
			return refdata.Character{}, sql.ErrNoRows
		},
		listCharactersByCampaignFn: func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Character, error) {
			return []refdata.Character{}, nil
		},
		getClassFn: func(ctx context.Context, id string) (refdata.Class, error) {
			return refdata.Class{}, sql.ErrNoRows
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID}, nil
		},
		getWeaponFn: func(ctx context.Context, id string) (refdata.Weapon, error) {
			return refdata.Weapon{}, sql.ErrNoRows
		},
		updateCharacterInventoryFn: func(ctx context.Context, id uuid.UUID, inventory pqtype.NullRawMessage) error {
			return nil
		},
		updateCombatantRageFn: func(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:                      arg.ID,
				IsRaging:                arg.IsRaging,
				RageRoundsRemaining:     arg.RageRoundsRemaining,
				RageAttackedThisRound:   arg.RageAttackedThisRound,
				RageTookDamageThisRound: arg.RageTookDamageThisRound,
				Conditions:              json.RawMessage(`[]`),
			}, nil
		},
		updateCombatantWildShapeFn: func(ctx context.Context, arg refdata.UpdateCombatantWildShapeParams) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:                   arg.ID,
				IsWildShaped:         arg.IsWildShaped,
				WildShapeCreatureRef: arg.WildShapeCreatureRef,
				WildShapeOriginal:    arg.WildShapeOriginal,
				HpMax:                arg.HpMax,
				HpCurrent:            arg.HpCurrent,
				Ac:                   arg.Ac,
				Conditions:           json.RawMessage(`[]`),
			}, nil
		},
		getArmorFn: func(ctx context.Context, id string) (refdata.Armor, error) {
			return refdata.Armor{}, sql.ErrNoRows
		},
		updateCharacterFeatureUsesFn: func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
			return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
		},
	}
}

// --- TDD Cycle 6: NewService returns non-nil ---

func TestNewCombatService(t *testing.T) {
	svc := NewService(defaultMockStore())
	assert.NotNil(t, svc)
}

// --- TDD Cycle 7: CreateEncounter validates name ---

func TestService_CreateEncounter_RejectsEmptyName(t *testing.T) {
	svc := NewService(defaultMockStore())
	_, err := svc.CreateEncounter(context.Background(), CreateEncounterInput{
		CampaignID: uuid.New(),
		Name:       "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

// --- TDD Cycle 8: CreateEncounter success ---

func TestService_CreateEncounter_Success(t *testing.T) {
	svc := NewService(defaultMockStore())
	enc, err := svc.CreateEncounter(context.Background(), CreateEncounterInput{
		CampaignID: uuid.New(),
		Name:       "Goblin Ambush",
	})
	require.NoError(t, err)
	assert.Equal(t, "Goblin Ambush", enc.Name)
	assert.Equal(t, "preparing", enc.Status)
}

// --- TDD Cycle 9: CreateEncounter store error ---

func TestService_CreateEncounter_StoreError(t *testing.T) {
	store := defaultMockStore()
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.CreateEncounter(context.Background(), CreateEncounterInput{
		CampaignID: uuid.New(),
		Name:       "Test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating encounter")
}

// --- TDD Cycle 10: GetEncounter ---

func TestService_GetEncounter(t *testing.T) {
	id := uuid.New()
	svc := NewService(defaultMockStore())
	enc, err := svc.GetEncounter(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, enc.ID)
}

// --- TDD Cycle 11: ListEncountersByCampaign ---

func TestService_ListEncounters(t *testing.T) {
	svc := NewService(defaultMockStore())
	list, err := svc.ListEncountersByCampaignID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, list)
}

// --- TDD Cycle 12: UpdateEncounterStatus ---

func TestService_UpdateEncounterStatus(t *testing.T) {
	svc := NewService(defaultMockStore())
	enc, err := svc.UpdateEncounterStatus(context.Background(), uuid.New(), "active")
	require.NoError(t, err)
	assert.Equal(t, "active", enc.Status)
}

func TestService_UpdateEncounterStatus_InvalidStatus(t *testing.T) {
	svc := NewService(defaultMockStore())
	_, err := svc.UpdateEncounterStatus(context.Background(), uuid.New(), "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

// --- TDD Cycle 13: AddCombatant ---

func TestService_AddCombatant(t *testing.T) {
	svc := NewService(defaultMockStore())
	encID := uuid.New()
	params := CombatantParams{
		CreatureRefID: "goblin",
		ShortID:       "G1",
		DisplayName:   "Goblin 1",
		HPMax:         7,
		HPCurrent:     7,
		AC:            15,
		SpeedFt:       30,
		PositionCol:   "A",
		PositionRow:   1,
		IsNPC:         true,
		IsAlive:       true,
		IsVisible:     true,
	}
	c, err := svc.AddCombatant(context.Background(), encID, params)
	require.NoError(t, err)
	assert.Equal(t, "G1", c.ShortID)
}

func TestService_AddCombatant_StoreError(t *testing.T) {
	store := defaultMockStore()
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AddCombatant(context.Background(), uuid.New(), CombatantParams{
		ShortID:     "G1",
		DisplayName: "Goblin",
		HPMax:       7,
		HPCurrent:   7,
		AC:          15,
		IsAlive:     true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating combatant")
}

// --- TDD Cycle 14: CreateEncounterFromTemplate ---

func TestService_CreateEncounterFromTemplate_Success(t *testing.T) {
	templateID := uuid.New()
	campaignID := uuid.New()
	mapID := uuid.New()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
			Name:       "Goblin Ambush",
			DisplayName: sql.NullString{String: "The Dark Forest", Valid: true},
			Creatures: json.RawMessage(`[
				{"creature_ref_id":"goblin","short_id":"G1","display_name":"Goblin 1","position_col":"A","position_row":1,"quantity":1}
			]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:        "goblin",
			Name:      "Goblin",
			Ac:        15,
			HpAverage: 7,
			Speed:     json.RawMessage(`{"walk":30}`),
		}, nil
	}

	var createdCombatants []refdata.CreateCombatantParams
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		createdCombatants = append(createdCombatants, arg)
		return refdata.Combatant{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			ShortID:     arg.ShortID,
			DisplayName: arg.DisplayName,
			HpMax:       arg.HpMax,
			HpCurrent:   arg.HpCurrent,
			Ac:          arg.Ac,
			IsAlive:     true,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}

	svc := NewService(store)
	enc, combatants, err := svc.CreateEncounterFromTemplate(context.Background(), templateID)
	require.NoError(t, err)
	assert.Equal(t, "Goblin Ambush", enc.Name)
	assert.Equal(t, "preparing", enc.Status)
	require.Len(t, combatants, 1)
	assert.Equal(t, "G1", createdCombatants[0].ShortID)
	assert.Equal(t, int32(7), createdCombatants[0].HpMax)
	assert.Equal(t, int32(15), createdCombatants[0].Ac)
}

func TestService_CreateEncounterFromTemplate_TemplateNotFound(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{}, sql.ErrNoRows
	}
	svc := NewService(store)
	_, _, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting encounter template")
}

func TestService_CreateEncounterFromTemplate_CreatureNotFound(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         uuid.New(),
			CampaignID: uuid.New(),
			Name:       "Test",
			Creatures:  json.RawMessage(`[{"creature_ref_id":"missing","short_id":"M1","display_name":"Missing","position_col":"A","position_row":1,"quantity":1}]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{}, sql.ErrNoRows
	}
	svc := NewService(store)
	_, _, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting creature")
}

func TestService_CreateEncounterFromTemplate_MultipleQuantity(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         uuid.New(),
			CampaignID: uuid.New(),
			Name:       "Test",
			Creatures:  json.RawMessage(`[{"creature_ref_id":"goblin","short_id":"G","display_name":"Goblin","position_col":"A","position_row":1,"quantity":3}]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Ac: 15, HpAverage: 7, Speed: json.RawMessage(`{"walk":30}`)}, nil
	}
	var count int
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		count++
		return refdata.Combatant{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			ShortID:     arg.ShortID,
			DisplayName: arg.DisplayName,
			HpMax:       arg.HpMax,
			Ac:          arg.Ac,
			IsAlive:     true,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}

	svc := NewService(store)
	_, combatants, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, combatants, 3)
	assert.Equal(t, 3, count)
}

// --- TDD Cycle 15: DeleteEncounter ---

func TestService_DeleteEncounter(t *testing.T) {
	svc := NewService(defaultMockStore())
	err := svc.DeleteEncounter(context.Background(), uuid.New())
	require.NoError(t, err)
}

// --- TDD Cycle 16: GetCombatant ---

func TestService_GetCombatant(t *testing.T) {
	id := uuid.New()
	svc := NewService(defaultMockStore())
	c, err := svc.GetCombatant(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, c.ID)
}

// --- TDD Cycle 17: ListCombatants ---

func TestService_ListCombatants(t *testing.T) {
	svc := NewService(defaultMockStore())
	list, err := svc.ListCombatantsByEncounterID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, list)
}

// --- TDD Cycle 18: UpdateCombatantHP ---

func TestService_UpdateCombatantHP(t *testing.T) {
	svc := NewService(defaultMockStore())
	c, err := svc.UpdateCombatantHP(context.Background(), uuid.New(), 10, 0, true)
	require.NoError(t, err)
	assert.Equal(t, int32(10), c.HpCurrent)
}

// --- TDD Cycle 19: DeleteCombatant ---

func TestService_DeleteCombatant(t *testing.T) {
	svc := NewService(defaultMockStore())
	err := svc.DeleteCombatant(context.Background(), uuid.New())
	require.NoError(t, err)
}

// --- TDD Cycle 20: CreateEncounterFromTemplate with invalid creatures JSON ---

func TestService_CreateEncounterFromTemplate_InvalidCreaturesJSON(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         uuid.New(),
			CampaignID: uuid.New(),
			Name:       "Bad",
			Creatures:  json.RawMessage(`invalid`),
		}, nil
	}
	svc := NewService(store)
	_, _, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing template creatures")
}

// --- TDD Cycle 21: ListActionLog ---

func TestService_ListActionLogByEncounterID(t *testing.T) {
	svc := NewService(defaultMockStore())
	logs, err := svc.ListActionLogByEncounterID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, logs)
}

func TestService_ListActionLogByTurnID(t *testing.T) {
	svc := NewService(defaultMockStore())
	logs, err := svc.ListActionLogByTurnID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, logs)
}

// --- TDD Cycle 22: CreateActionLog ---

func TestService_CreateActionLog(t *testing.T) {
	svc := NewService(defaultMockStore())
	log, err := svc.CreateActionLog(context.Background(), CreateActionLogInput{
		TurnID:      uuid.New(),
		EncounterID: uuid.New(),
		ActionType:  "attack",
		ActorID:     uuid.New(),
		BeforeState: json.RawMessage(`{}`),
		AfterState:  json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, log.ID)
}

func TestService_CreateActionLog_EmptyActionType(t *testing.T) {
	svc := NewService(defaultMockStore())
	_, err := svc.CreateActionLog(context.Background(), CreateActionLogInput{
		TurnID:      uuid.New(),
		EncounterID: uuid.New(),
		ActionType:  "",
		ActorID:     uuid.New(),
		BeforeState: json.RawMessage(`{}`),
		AfterState:  json.RawMessage(`{}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action_type must not be empty")
}

// --- TDD Cycle 23: UpdateCombatantPosition ---

func TestService_UpdateCombatantPosition(t *testing.T) {
	svc := NewService(defaultMockStore())
	c, err := svc.UpdateCombatantPosition(context.Background(), uuid.New(), "C", 5, 0)
	require.NoError(t, err)
	assert.Equal(t, "C", c.PositionCol)
	assert.Equal(t, int32(5), c.PositionRow)
}

// --- TDD Cycle 24: UpdateCombatantConditions ---

func TestService_UpdateCombatantConditions(t *testing.T) {
	svc := NewService(defaultMockStore())
	conds := json.RawMessage(`["poisoned"]`)
	c, err := svc.UpdateCombatantConditions(context.Background(), uuid.New(), conds, 1)
	require.NoError(t, err)
	assert.Equal(t, conds, c.Conditions)
}

// --- TDD Cycle 25: CreateEncounterFromTemplate encounter create error ---

func TestService_CreateEncounterFromTemplate_CreateEncounterError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         uuid.New(),
			CampaignID: uuid.New(),
			Name:       "Test",
			Creatures:  json.RawMessage(`[]`),
		}, nil
	}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	svc := NewService(store)
	_, _, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating encounter")
}

// --- TDD Cycle 26: CreateEncounterFromTemplate combatant create error ---

func TestService_CreateEncounterFromTemplate_CombatantCreateError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         uuid.New(),
			CampaignID: uuid.New(),
			Name:       "Test",
			Creatures:  json.RawMessage(`[{"creature_ref_id":"goblin","short_id":"G1","display_name":"Goblin","position_col":"A","position_row":1,"quantity":1}]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Ac: 15, HpAverage: 7, Speed: json.RawMessage(`{"walk":30}`)}, nil
	}
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db error")
	}
	svc := NewService(store)
	_, _, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating combatant")
}

// --- TDD Cycle 27: AddCombatant with character ID ---

func TestService_AddCombatant_WithCharacterID(t *testing.T) {
	charID := uuid.New()
	svc := NewService(defaultMockStore())
	c, err := svc.AddCombatant(context.Background(), uuid.New(), CombatantParams{
		CharacterID: charID.String(),
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HPMax:       45,
		HPCurrent:   45,
		AC:          18,
		IsAlive:     true,
		IsVisible:   true,
		DeathSaves:  json.RawMessage(`{"successes":0,"failures":0}`),
		PositionCol: "D",
		PositionRow: 5,
	})
	require.NoError(t, err)
	assert.Equal(t, "AR", c.ShortID)
}

func TestService_AddCombatant_InvalidCharacterID(t *testing.T) {
	svc := NewService(defaultMockStore())
	_, err := svc.AddCombatant(context.Background(), uuid.New(), CombatantParams{
		CharacterID: "not-a-uuid",
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HPMax:       45,
		HPCurrent:   45,
		AC:          18,
		IsAlive:     true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing character_id")
}

// --- TDD Cycle 28: CreateActionLog with description and dice rolls ---

func TestService_CreateActionLog_WithOptionalFields(t *testing.T) {
	svc := NewService(defaultMockStore())
	log, err := svc.CreateActionLog(context.Background(), CreateActionLogInput{
		TurnID:      uuid.New(),
		EncounterID: uuid.New(),
		ActionType:  "attack",
		ActorID:     uuid.New(),
		TargetID:    uuid.NullUUID{UUID: uuid.New(), Valid: true},
		Description: "Goblin attacks Aragorn with a scimitar",
		BeforeState: json.RawMessage(`{"hp":45}`),
		AfterState:  json.RawMessage(`{"hp":39}`),
		DiceRolls:   json.RawMessage(`[{"type":"d20","result":15}]`),
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, log.ID)
}

// --- TDD Cycle 29: StartCombat happy path ---

func TestService_StartCombat_Success(t *testing.T) {
	templateID := uuid.New()
	campaignID := uuid.New()
	mapID := uuid.New()
	encounterID := uuid.New()
	charID := uuid.New()

	store := defaultMockStore()

	// Template lookup
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
			Name:       "Goblin Ambush",
			Creatures: json.RawMessage(`[
				{"creature_ref_id":"goblin","short_id":"G1","display_name":"Goblin","position_col":"A","position_row":1,"quantity":1}
			]`),
		}, nil
	}

	// Creature lookup
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "goblin",
			Name:          "Goblin",
			Ac:            15,
			HpAverage:     7,
			Speed:         json.RawMessage(`{"walk":30}`),
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
		}, nil
	}

	// Character lookup
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:            charID,
			Name:          "Aragorn",
			HpMax:         45,
			HpCurrent:     45,
			Ac:            18,
			SpeedFt:       30,
			AbilityScores: json.RawMessage(`{"str":16,"dex":14,"con":14,"int":10,"wis":12,"cha":14}`),
		}, nil
	}

	// Track created combatants
	createdCombatantIDs := []uuid.UUID{}
	combatantCounter := 0
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:          encounterID,
			CampaignID:  campaignID,
			MapID:       uuid.NullUUID{UUID: mapID, Valid: true},
			Name:        arg.Name,
			Status:      arg.Status,
			RoundNumber: arg.RoundNumber,
		}, nil
	}
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		cID := uuid.New()
		createdCombatantIDs = append(createdCombatantIDs, cID)
		combatantCounter++
		return refdata.Combatant{
			ID:          cID,
			EncounterID: arg.EncounterID,
			CharacterID: arg.CharacterID,
			ShortID:     arg.ShortID,
			DisplayName: arg.DisplayName,
			HpMax:       arg.HpMax,
			HpCurrent:   arg.HpCurrent,
			Ac:          arg.Ac,
			IsAlive:     true,
			IsNpc:       arg.IsNpc,
			IsVisible:   true,
			PositionCol: arg.PositionCol,
			PositionRow: arg.PositionRow,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}

	// Initiative updates: return combatants sorted
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		// Return the 2 combatants (goblin + aragorn) after creation
		return []refdata.Combatant{
			{ID: createdCombatantIDs[0], EncounterID: encounterID, DisplayName: "Goblin", ShortID: "G1", IsAlive: true, IsNpc: true, HpMax: 7, HpCurrent: 7, Conditions: json.RawMessage(`[]`), CreatureRefID: sql.NullString{String: "goblin", Valid: true}},
			{ID: createdCombatantIDs[1], EncounterID: encounterID, DisplayName: "Aragorn", ShortID: "AR", IsAlive: true, IsNpc: false, HpMax: 45, HpCurrent: 45, Conditions: json.RawMessage(`[]`), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		}, nil
	}

	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:              arg.ID,
			EncounterID:     encounterID,
			InitiativeRoll:  arg.InitiativeRoll,
			InitiativeOrder: arg.InitiativeOrder,
			IsAlive:         true,
			Conditions:      json.RawMessage(`[]`),
		}, nil
	}

	store.updateEncounterRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, RoundNumber: arg.RoundNumber, Name: "Goblin Ambush", Status: "active"}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, Status: arg.Status, Name: "Goblin Ambush", RoundNumber: 1}, nil
	}

	// AdvanceTurn needs encounter + turns
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Name: "Goblin Ambush", Status: "active", RoundNumber: 1}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}

	turnID := uuid.New()
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}
	store.updateEncounterCurrentTurnFn = func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, CurrentTurnID: arg.CurrentTurnID, RoundNumber: 1, Name: "Goblin Ambush"}, nil
	}

	roller := dice.NewRoller(func(max int) int { return 15 })
	svc := NewService(store)

	result, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:   templateID,
		CharacterIDs: []uuid.UUID{charID},
		CharacterPositions: map[uuid.UUID]Position{
			charID: {Col: "D", Row: 5},
		},
	}, roller)

	require.NoError(t, err)
	assert.Equal(t, encounterID, result.Encounter.ID)
	assert.NotEmpty(t, result.Combatants)
	assert.NotEmpty(t, result.InitiativeTracker)
	assert.NotEqual(t, uuid.Nil, result.FirstTurn.Turn.ID)
}

// --- TDD Cycle 30: StartCombat template error ---

func TestService_StartCombat_TemplateError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{}, errors.New("not found")
	}
	roller := dice.NewRoller(func(max int) int { return 10 })
	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID: uuid.New(),
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating encounter from template")
}

// --- TDD Cycle 31: StartCombat character not found ---

func TestService_StartCombat_CharacterNotFound(t *testing.T) {
	templateID := uuid.New()
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         templateID,
			CampaignID: uuid.New(),
			Name:       "Test",
			Creatures:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, errors.New("not found")
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:   templateID,
		CharacterIDs: []uuid.UUID{uuid.New()},
		CharacterPositions: map[uuid.UUID]Position{},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

// --- TDD Cycle 32: StartCombat with surprised combatants ---

func TestService_StartCombat_WithSurprisedCombatants(t *testing.T) {
	templateID := uuid.New()
	encounterID := uuid.New()
	charID := uuid.New()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         templateID,
			CampaignID: uuid.New(),
			Name:       "Surprise Attack",
			Creatures:  json.RawMessage(`[{"creature_ref_id":"goblin","short_id":"G1","display_name":"Goblin","position_col":"A","position_row":1,"quantity":1}]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Ac: 15, HpAverage: 7, Speed: json.RawMessage(`{"walk":30}`), AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, Name: "Aragorn", HpMax: 45, HpCurrent: 45, Ac: 18, SpeedFt: 30, AbilityScores: json.RawMessage(`{"str":16,"dex":14,"con":14,"int":10,"wis":12,"cha":14}`)}, nil
	}

	createdCombatantIDs := []uuid.UUID{}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: encounterID, Name: arg.Name, Status: arg.Status}, nil
	}
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		cID := uuid.New()
		createdCombatantIDs = append(createdCombatantIDs, cID)
		return refdata.Combatant{ID: cID, EncounterID: arg.EncounterID, ShortID: arg.ShortID, DisplayName: arg.DisplayName, HpMax: arg.HpMax, HpCurrent: arg.HpCurrent, Ac: arg.Ac, IsAlive: true, IsNpc: arg.IsNpc, Conditions: json.RawMessage(`[]`)}, nil
	}

	// Track MarkSurprised calls — getCombatant is called by MarkSurprised and skipSurprisedTurn
	markSurprisedCalled := false
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		markSurprisedCalled = true
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	// Initiative + turn support
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		if len(createdCombatantIDs) < 2 {
			return []refdata.Combatant{}, nil
		}
		return []refdata.Combatant{
			{ID: createdCombatantIDs[0], EncounterID: encounterID, ShortID: "G1", DisplayName: "Goblin", IsAlive: true, IsNpc: true, HpMax: 7, HpCurrent: 7, Conditions: json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0}]`), CreatureRefID: sql.NullString{String: "goblin", Valid: true}},
			{ID: createdCombatantIDs[1], EncounterID: encounterID, ShortID: "AR", DisplayName: "Aragorn", IsAlive: true, IsNpc: false, HpMax: 45, HpCurrent: 45, Conditions: json.RawMessage(`[]`), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		}, nil
	}
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Name: "Surprise Attack", Status: "active", RoundNumber: 1}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}

	roller := dice.NewRoller(func(max int) int { return 15 })
	svc := NewService(store)

	result, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:         templateID,
		CharacterIDs:       []uuid.UUID{charID},
		CharacterPositions: map[uuid.UUID]Position{charID: {Col: "D", Row: 5}},
		SurprisedShortIDs:  []string{"G1"},
	}, roller)

	require.NoError(t, err)
	assert.NotEmpty(t, result.Combatants)
	// Verify MarkSurprised was called
	assert.True(t, markSurprisedCalled, "MarkSurprised should have been called")
}

// --- TDD Cycle 33a: StartCombat add combatant error ---

func TestService_StartCombat_AddCombatantError(t *testing.T) {
	templateID := uuid.New()
	charID := uuid.New()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID: templateID, CampaignID: uuid.New(), Name: "Test",
			Creatures: json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, Name: "Aragorn", HpMax: 45, HpCurrent: 45, Ac: 18, SpeedFt: 30}, nil
	}
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db error")
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:         templateID,
		CharacterIDs:       []uuid.UUID{charID},
		CharacterPositions: map[uuid.UUID]Position{charID: {Col: "A", Row: 1}},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adding character combatant")
}

// --- TDD Cycle 33b: StartCombat mark surprised error ---

func TestService_StartCombat_MarkSurprisedError(t *testing.T) {
	templateID := uuid.New()
	encounterID := uuid.New()
	cID := uuid.New()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID: templateID, CampaignID: uuid.New(), Name: "Test",
			Creatures: json.RawMessage(`[]`),
		}, nil
	}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: encounterID, Name: arg.Name, Status: arg.Status}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: cID, ShortID: "G1", DisplayName: "Goblin", IsAlive: true, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("not found")
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:        templateID,
		SurprisedShortIDs: []string{"G1"},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marking combatant")
}

// --- TDD Cycle 33b2: StartCombat listing combatants for surprise error ---

func TestService_StartCombat_ListCombatantsForSurpriseError(t *testing.T) {
	templateID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID: templateID, CampaignID: uuid.New(), Name: "Test",
			Creatures: json.RawMessage(`[]`),
		}, nil
	}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: encounterID, Name: arg.Name, Status: arg.Status}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return nil, errors.New("db error")
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:        templateID,
		SurprisedShortIDs: []string{"G1"},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing combatants for surprise")
}

// --- TDD Cycle 33b3: StartCombat ListCharactersByCampaign ---

func TestService_ListCharactersByCampaign(t *testing.T) {
	charID := uuid.New()
	campaignID := uuid.New()
	store := defaultMockStore()
	store.listCharactersByCampaignFn = func(ctx context.Context, cID uuid.UUID) ([]refdata.Character, error) {
		return []refdata.Character{{ID: charID, CampaignID: campaignID, Name: "Aragorn"}}, nil
	}
	svc := NewService(store)
	chars, err := svc.ListCharactersByCampaign(context.Background(), campaignID)
	require.NoError(t, err)
	require.Len(t, chars, 1)
	assert.Equal(t, "Aragorn", chars[0].Name)
}

// --- TDD Cycle 33c: StartCombat roll initiative error ---

func TestService_StartCombat_RollInitiativeError(t *testing.T) {
	templateID := uuid.New()
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID: templateID, CampaignID: uuid.New(), Name: "Test",
			Creatures: json.RawMessage(`[]`),
		}, nil
	}
	// ListCombatantsByEncounterID returns empty -> RollInitiative fails
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{}, nil
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID: templateID,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolling initiative")
}

// --- TDD Cycle 33d: StartCombat advance turn error ---

func TestService_StartCombat_AdvanceTurnError(t *testing.T) {
	templateID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID: templateID, CampaignID: uuid.New(), Name: "Test",
			Creatures: json.RawMessage(`[]`),
		}, nil
	}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: encounterID, Name: arg.Name, Status: arg.Status}, nil
	}
	cID := uuid.New()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: cID, EncounterID: encounterID, DisplayName: "Goblin", IsAlive: true, Conditions: json.RawMessage(`[]`), CreatureRefID: sql.NullString{String: "goblin", Valid: true}},
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
	}
	// GetEncounter fails for AdvanceTurn
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID: templateID,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "advancing to first turn")
}

// --- TDD Cycle 33e: StartCombat re-fetch encounter error ---

func TestService_StartCombat_RefetchEncounterError(t *testing.T) {
	templateID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID: templateID, CampaignID: uuid.New(), Name: "Test",
			Creatures: json.RawMessage(`[]`),
		}, nil
	}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: encounterID, Name: arg.Name, Status: arg.Status}, nil
	}
	cID := uuid.New()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: cID, EncounterID: encounterID, DisplayName: "Goblin", IsAlive: true, Conditions: json.RawMessage(`[]`), CreatureRefID: sql.NullString{String: "goblin", Valid: true}},
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}

	getEncounterCalls := 0
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		getEncounterCalls++
		// First call (from AdvanceTurn) succeeds, second call (re-fetch) fails
		if getEncounterCalls == 1 {
			return refdata.Encounter{ID: id, Name: "Test", Status: "active", RoundNumber: 1}, nil
		}
		return refdata.Encounter{}, errors.New("db error")
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID: templateID,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "re-fetching encounter")
}

// --- TDD Cycle 33: ShortIDFromName ---

func TestShortIDFromName(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"Aragorn", "AR"},
		{"Bo", "BO"},
		{"X", "X"},
		{"legolas", "LE"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ShortIDFromName(tt.name))
		})
	}
}

// --- TDD Cycle 44: ClearCombatConditions removes combat-only conditions ---

func TestClearCombatConditions_RemovesCombatConditions(t *testing.T) {
	input := json.RawMessage(`[
		{"condition":"stunned","duration_rounds":3,"started_round":1},
		{"condition":"frightened","duration_rounds":2,"started_round":1},
		{"condition":"exhaustion","duration_rounds":0,"started_round":0}
	]`)
	result, err := ClearCombatConditions(input)
	require.NoError(t, err)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(result, &conds))
	require.Len(t, conds, 1)
	assert.Equal(t, "exhaustion", conds[0].Condition)
}

// --- TDD Cycle 45: ClearCombatConditions handles empty and nil input ---

func TestClearCombatConditions_EmptyInput(t *testing.T) {
	result, err := ClearCombatConditions(json.RawMessage(`[]`))
	require.NoError(t, err)
	assert.Equal(t, "[]", string(result))

	result2, err := ClearCombatConditions(nil)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(result2))
}

// --- TDD Cycle 46: ClearCombatConditions removes all 12 combat conditions ---

func TestClearCombatConditions_AllCombatConditions(t *testing.T) {
	allCombat := []CombatCondition{
		{Condition: "stunned"}, {Condition: "frightened"}, {Condition: "charmed"},
		{Condition: "restrained"}, {Condition: "grappled"}, {Condition: "prone"},
		{Condition: "incapacitated"}, {Condition: "paralyzed"}, {Condition: "blinded"},
		{Condition: "deafened"}, {Condition: "surprised"}, {Condition: "dodge"},
	}
	input, _ := json.Marshal(allCombat)
	result, err := ClearCombatConditions(input)
	require.NoError(t, err)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(result, &conds))
	assert.Empty(t, conds)
}

// --- TDD Cycle 47: ClearCombatConditions preserves non-combat conditions ---

func TestClearCombatConditions_PreservesNonCombat(t *testing.T) {
	input := json.RawMessage(`[
		{"condition":"stunned","duration_rounds":1,"started_round":1},
		{"condition":"curse","duration_rounds":0,"started_round":0},
		{"condition":"disease","duration_rounds":0,"started_round":0},
		{"condition":"blinded","duration_rounds":2,"started_round":1}
	]`)
	result, err := ClearCombatConditions(input)
	require.NoError(t, err)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(result, &conds))
	require.Len(t, conds, 2)
	assert.Equal(t, "curse", conds[0].Condition)
	assert.Equal(t, "disease", conds[1].Condition)
}

// --- TDD Cycle 48: AllHostilesDefeated returns true when all NPCs dead ---

func TestAllHostilesDefeated_AllDead(t *testing.T) {
	encounterID := uuid.New()
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), IsNpc: false, IsAlive: true, HpCurrent: 20, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	svc := NewService(store)

	result, err := svc.AllHostilesDefeated(context.Background(), encounterID)
	require.NoError(t, err)
	assert.True(t, result)
}

// --- TDD Cycle 49: AllHostilesDefeated returns false when some NPCs alive ---

func TestAllHostilesDefeated_SomeAlive(t *testing.T) {
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), IsNpc: true, IsAlive: true, HpCurrent: 5, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	svc := NewService(store)

	result, err := svc.AllHostilesDefeated(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.False(t, result)
}

// --- TDD Cycle 50: AllHostilesDefeated returns false with no NPCs ---

func TestAllHostilesDefeated_NoNPCs(t *testing.T) {
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: false, IsAlive: true, HpCurrent: 20, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	svc := NewService(store)

	result, err := svc.AllHostilesDefeated(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.False(t, result)
}

// --- TDD Cycle 51: AllHostilesDefeated store error ---

func TestAllHostilesDefeated_StoreError(t *testing.T) {
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return nil, errors.New("db error")
	}
	svc := NewService(store)

	_, err := svc.AllHostilesDefeated(context.Background(), uuid.New())
	assert.Error(t, err)
}

// --- TDD Cycle 63: EndCombat skips condition update when no combat conditions present ---

func TestEndCombat_SkipsUpdateWhenNoCombatConditions(t *testing.T) {
	encounterID := uuid.New()
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 2}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, Status: "completed", RoundNumber: 2}, nil
	}
	conditionUpdateCalled := false
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		conditionUpdateCalled = true
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: false, IsAlive: true, HpCurrent: 20, DisplayName: "Frodo", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	svc := NewService(store)

	_, err := svc.EndCombat(context.Background(), encounterID)
	require.NoError(t, err)
	assert.False(t, conditionUpdateCalled, "should not update conditions when no combat conditions to clear")
}

// --- TDD Cycle 64: EndCombat completed encounter error ---

func TestEndCombat_CompletedEncounter(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "completed"}, nil
	}
	svc := NewService(store)

	_, err := svc.EndCombat(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be active")
}

// --- TDD Cycle 65: ClearCombatConditions with invalid JSON ---

func TestClearCombatConditions_InvalidJSON(t *testing.T) {
	_, err := ClearCombatConditions(json.RawMessage(`not json`))
	assert.Error(t, err)
}

// --- TDD Cycle 66: EndCombat update status error ---

func TestEndCombat_UpdateStatusError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	svc := NewService(store)

	_, err := svc.EndCombat(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setting status")
}

// --- TDD Cycle 67: EndCombat complete turn error ---

func TestEndCombat_CompleteTurnError(t *testing.T) {
	turnID := uuid.New()
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1, CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true}}, nil
	}
	store.completeTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("turn error")
	}
	svc := NewService(store)

	_, err := svc.EndCombat(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "completing active turn")
}

// --- TDD Cycle 68: EndCombat list combatants error ---

func TestEndCombat_ListCombatantsError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, Status: "completed", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return nil, errors.New("db error")
	}
	svc := NewService(store)

	_, err := svc.EndCombat(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing combatants")
}

// --- TDD Cycle 69: EndCombat update conditions error ---

func TestEndCombat_UpdateConditionsError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, Status: "completed", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), DisplayName: "Test", Conditions: json.RawMessage(`[{"condition":"stunned","duration_rounds":1,"started_round":1}]`)},
		}, nil
	}
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("update error")
	}
	svc := NewService(store)

	_, err := svc.EndCombat(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating conditions")
}

// --- TDD Cycle 52: EndCombat success ---

func TestEndCombat_Success(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Name:          "Goblin Ambush",
			DisplayName:   sql.NullString{String: "The Goblin Ambush", Valid: true},
			Status:        "active",
			RoundNumber:   3,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		}, nil
	}
	statusUpdated := false
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		assert.Equal(t, "completed", arg.Status)
		statusUpdated = true
		return refdata.Encounter{ID: arg.ID, Status: "completed", RoundNumber: 3, Name: "Goblin Ambush"}, nil
	}
	turnCompleted := false
	store.completeTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		assert.Equal(t, turnID, id)
		turnCompleted = true
		return refdata.Turn{ID: id, Status: "completed"}, nil
	}
	conditionsCleared := map[uuid.UUID]bool{}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantIDs[0], IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin 1", Conditions: json.RawMessage(`[{"condition":"stunned","duration_rounds":1,"started_round":2}]`)},
			{ID: combatantIDs[1], IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin 2", Conditions: json.RawMessage(`[]`)},
			{ID: combatantIDs[2], IsNpc: false, IsAlive: true, HpCurrent: 30, DisplayName: "Aragorn", Conditions: json.RawMessage(`[{"condition":"frightened","duration_rounds":2,"started_round":2},{"condition":"exhaustion","duration_rounds":0,"started_round":0}]`)},
		}, nil
	}
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		conditionsCleared[arg.ID] = true
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	result, err := svc.EndCombat(context.Background(), encounterID)
	require.NoError(t, err)

	assert.True(t, statusUpdated, "encounter status should be set to completed")
	assert.True(t, turnCompleted, "active turn should be completed")
	assert.Equal(t, int32(3), result.RoundsElapsed)
	assert.Equal(t, 2, result.Casualties)
	assert.Contains(t, result.Summary, "3 rounds")
	assert.Contains(t, result.Summary, "2 casualties")
	assert.True(t, conditionsCleared[combatantIDs[0]], "goblin 1 conditions should be cleared")
	assert.True(t, conditionsCleared[combatantIDs[2]], "aragorn conditions should be cleared")
}

// --- TDD Cycle 53: EndCombat rejects non-active encounter ---

func TestEndCombat_NotActive(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "preparing"}, nil
	}
	svc := NewService(store)

	_, err := svc.EndCombat(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be active")
}

// --- TDD Cycle 54: EndCombat with no active turn ---

func TestEndCombat_NoActiveTurn(t *testing.T) {
	encounterID := uuid.New()
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, Status: "completed", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: false, IsAlive: true, HpCurrent: 20, DisplayName: "Frodo", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	svc := NewService(store)

	result, err := svc.EndCombat(context.Background(), encounterID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), result.RoundsElapsed)
	assert.Equal(t, 0, result.Casualties)
}

// --- TDD Cycle 55: EndCombat encounter not found ---

func TestEndCombat_EncounterNotFound(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{}, sql.ErrNoRows
	}
	svc := NewService(store)

	_, err := svc.EndCombat(context.Background(), uuid.New())
	assert.Error(t, err)
}

// --- TDD Cycle 71: EndCombat includes frozen initiative tracker ---

func TestEndCombat_IncludesInitiativeTracker(t *testing.T) {
	encounterID := uuid.New()
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID: id, Name: "Goblin Ambush", Status: "active", RoundNumber: 3,
			DisplayName: sql.NullString{String: "The Goblin Ambush", Valid: true},
		}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID: arg.ID, Status: "completed", RoundNumber: 3, Name: "Goblin Ambush",
			DisplayName: sql.NullString{String: "The Goblin Ambush", Valid: true},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), IsNpc: false, IsAlive: true, HpCurrent: 30, HpMax: 45, DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
		}, nil
	}

	svc := NewService(store)
	result, err := svc.EndCombat(context.Background(), encounterID)
	require.NoError(t, err)

	assert.Contains(t, result.InitiativeTracker, "The Goblin Ambush")
	assert.Contains(t, result.InitiativeTracker, "Round 3")
	assert.Contains(t, result.InitiativeTracker, "--- Combat Complete ---")
	assert.NotContains(t, result.InitiativeTracker, "\U0001f514")
}

// --- TDD Cycle 72: EndCombat returns ErrEncounterNotActive for non-active encounter ---

func TestEndCombat_ReturnsErrEncounterNotActive(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "preparing"}, nil
	}
	svc := NewService(store)

	_, err := svc.EndCombat(context.Background(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEncounterNotActive)
}

