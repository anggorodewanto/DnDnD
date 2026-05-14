package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/inventory"
)

const characterDataExhaustionKey = "exhaustion_level"

// EncounterPublisher fans out a fresh encounter snapshot over the dashboard
// WebSocket hub whenever a /rest mutation (HP / hit dice / spell slots /
// dawn-recharge) touches a character that is also a combatant in an active
// encounter (H-104b). The interface is injected (optionally) onto Service
// so the package stays decoupled from the concrete dashboard.Publisher.
type EncounterPublisher interface {
	PublishEncounterSnapshot(ctx context.Context, encounterID uuid.UUID) error
}

// EncounterLookup resolves the active encounter (if any) that currently
// contains the given character. Returns (encID, true, nil) when the
// character is a combatant in an active encounter; (uuid.Nil, false, nil)
// when not in combat; or a non-nil error on store failure.
type EncounterLookup interface {
	ActiveEncounterIDForCharacter(ctx context.Context, characterID uuid.UUID) (uuid.UUID, bool, error)
}

type CombatantExhaustionState struct {
	ID              uuid.UUID
	Conditions      []byte
	ExhaustionLevel int
}

type CombatantExhaustionStore interface {
	ActiveCombatantExhaustionForCharacter(ctx context.Context, characterID uuid.UUID) (CombatantExhaustionState, bool, error)
	UpdateCombatantExhaustion(ctx context.Context, combatantID uuid.UUID, conditions []byte, exhaustionLevel int) error
}

// Service handles rest logic (short and long rests).
type Service struct {
	roller          *dice.Roller
	publisher       EncounterPublisher
	lookup          EncounterLookup
	exhaustionStore CombatantExhaustionStore
}

// NewService creates a new rest Service.
func NewService(roller *dice.Roller) *Service {
	return &Service{roller: roller}
}

// SetPublisher wires the optional dashboard publisher and encounter lookup
// (H-104b). A nil publisher is tolerated and disables fan-out. Publish
// errors are logged but never surfaced to callers so a dashboard hiccup
// cannot undo a committed rest write.
func (s *Service) SetPublisher(p EncounterPublisher, lookup EncounterLookup) {
	s.publisher = p
	s.lookup = lookup
}

func (s *Service) SetCombatantExhaustionStore(store CombatantExhaustionStore) {
	s.exhaustionStore = store
}

func (s *Service) ExhaustionLevelForCharacter(ctx context.Context, characterID uuid.UUID) int {
	if s.exhaustionStore == nil {
		return 0
	}
	state, ok, err := s.exhaustionStore.ActiveCombatantExhaustionForCharacter(ctx, characterID)
	if err != nil {
		log.Printf("rest: active combatant exhaustion lookup failed for %s: %v", characterID, err)
		return 0
	}
	if !ok {
		return 0
	}
	return state.ExhaustionLevel
}

func (s *Service) PersistLongRestExhaustion(ctx context.Context, characterID uuid.UUID, result LongRestResult) {
	if s.exhaustionStore == nil {
		return
	}
	if !result.ExhaustionDecreased {
		return
	}
	state, ok, err := s.exhaustionStore.ActiveCombatantExhaustionForCharacter(ctx, characterID)
	if err != nil {
		log.Printf("rest: active combatant exhaustion lookup failed for %s: %v", characterID, err)
		return
	}
	if !ok {
		return
	}
	if err := s.exhaustionStore.UpdateCombatantExhaustion(ctx, state.ID, state.Conditions, result.ExhaustionLevelAfter); err != nil {
		log.Printf("rest: combatant exhaustion update failed for %s: %v", state.ID, err)
	}
}

// PublishForCharacter looks up the character's active encounter (if any)
// and fires the publisher. Silently no-ops when the character is not in
// combat, when the publisher is unset, or when the lookup/publish fails.
// Callers (the /rest Discord handler) invoke this AFTER persisting rest
// changes so dashboard subscribers see the refreshed HP / hit-dice /
// spell-slot state.
func (s *Service) PublishForCharacter(ctx context.Context, characterID uuid.UUID) {
	if s.publisher == nil || s.lookup == nil {
		return
	}
	encID, ok, err := s.lookup.ActiveEncounterIDForCharacter(ctx, characterID)
	if err != nil {
		log.Printf("rest: active encounter lookup failed for %s: %v", characterID, err)
		return
	}
	if !ok {
		return
	}
	if err := s.publisher.PublishEncounterSnapshot(ctx, encID); err != nil {
		log.Printf("rest: encounter snapshot publish failed for %s: %v", encID, err)
	}
}

// ShortRestInput holds parameters for a short rest.
type ShortRestInput struct {
	HPCurrent        int
	HPMax            int
	CONModifier      int
	HitDiceRemaining map[string]int // e.g. {"d10": 5, "d8": 2}
	HitDiceSpend     map[string]int // e.g. {"d10": 2} — how many to spend per die type
	FeatureUses      map[string]character.FeatureUse
	PactMagicSlots   *character.PactMagicSlots // nil if not a warlock
	Classes          []character.ClassEntry
	Inventory        []character.InventoryItem // optional: for item study
	StudyItemID      string                    // optional: item to study during rest
}

// HitDieRoll records a single hit die roll result.
type HitDieRoll struct {
	DieType string // e.g. "d10"
	Rolled  int    // the die result
	CONMod  int    // CON modifier added
	Healed  int    // actual HP healed (may be less if capped)
}

// ShortRestResult holds the results of a short rest.
type ShortRestResult struct {
	HPBefore          int
	HPAfter           int
	HPMax             int
	HPHealed          int
	HitDieRolls       []HitDieRoll
	HitDiceRemaining  map[string]int
	FeaturesRecharged []string
	PactSlotsRestored bool
	PactSlotsCurrent  int
	ItemStudied       bool
	StudiedItemName   string
	UpdatedInventory  []character.InventoryItem
}

// ShortRest applies a short rest to a character.
func (s *Service) ShortRest(input ShortRestInput) (ShortRestResult, error) {
	result := ShortRestResult{
		HPBefore:         input.HPCurrent,
		HPMax:            input.HPMax,
		HitDiceRemaining: maps.Clone(input.HitDiceRemaining),
	}

	hp := input.HPCurrent

	// Spend hit dice
	for dieType, count := range input.HitDiceSpend {
		remaining, ok := result.HitDiceRemaining[dieType]
		if !ok {
			return ShortRestResult{}, fmt.Errorf("no hit dice of type %s available", dieType)
		}
		if count > remaining {
			return ShortRestResult{}, fmt.Errorf("cannot spend %d %s hit dice, only %d remaining", count, dieType, remaining)
		}

		dieSize := character.HitDieValue(dieType)
		if dieSize == 0 {
			return ShortRestResult{}, fmt.Errorf("invalid hit die type: %s", dieType)
		}

		for i := 0; i < count; i++ {
			roll, err := s.roller.Roll(fmt.Sprintf("1%s", dieType))
			if err != nil {
				return ShortRestResult{}, fmt.Errorf("rolling hit die: %w", err)
			}

			healing := roll.Total + input.CONModifier
			if healing < 0 {
				healing = 0
			}

			actualHealed := healing
			if hp+actualHealed > input.HPMax {
				actualHealed = input.HPMax - hp
			}

			hp += actualHealed

			result.HitDieRolls = append(result.HitDieRolls, HitDieRoll{
				DieType: dieType,
				Rolled:  roll.Total,
				CONMod:  input.CONModifier,
				Healed:  actualHealed,
			})
		}

		result.HitDiceRemaining[dieType] = remaining - count
	}

	// Recharge short rest features
	for name, fu := range input.FeatureUses {
		if fu.Recharge == "short" {
			if fu.Current < fu.Max {
				fu.Current = fu.Max
				input.FeatureUses[name] = fu
				result.FeaturesRecharged = append(result.FeaturesRecharged, name)
			}
		}
	}

	// Restore pact magic slots
	if input.PactMagicSlots != nil && input.PactMagicSlots.Max > 0 {
		if input.PactMagicSlots.Current < input.PactMagicSlots.Max {
			input.PactMagicSlots.Current = input.PactMagicSlots.Max
			result.PactSlotsRestored = true
		}
		result.PactSlotsCurrent = input.PactMagicSlots.Current
	}

	// Study a magic item during the rest (optional)
	if input.StudyItemID != "" {
		identResult, err := inventory.StudyItemDuringRest(inventory.IdentifyInput{
			Items:  input.Inventory,
			ItemID: input.StudyItemID,
		})
		if err != nil {
			return ShortRestResult{}, err
		}
		result.ItemStudied = true
		result.StudiedItemName = identResult.ItemName
		result.UpdatedInventory = identResult.UpdatedItems
	}

	result.HPAfter = hp
	result.HPHealed = hp - input.HPCurrent

	return result, nil
}

// LongRestInput holds parameters for a long rest.
type LongRestInput struct {
	HPCurrent          int
	HPMax              int
	TempHP             int
	HitDiceRemaining   map[string]int
	Classes            []character.ClassEntry
	FeatureUses        map[string]character.FeatureUse
	SpellSlots         map[string]character.SlotInfo
	PactMagicSlots     *character.PactMagicSlots
	DeathSaveSuccesses int
	DeathSaveFailures  int

	// Phase 88b dawn recharge: when both Inventory and RechargeInfo are
	// non-empty, the rest service rolls recharge dice for every magic
	// item with charges, capped at MaxCharges. Items with DestroyOnZero
	// = true that ran out roll d20 — on 1 they are destroyed. The
	// recharged inventory replaces the input slice in the result.
	Inventory    []character.InventoryItem
	RechargeInfo map[string]inventory.RechargeInfo

	// SR-019: current exhaustion level [0-6]. A long rest decrements by 1
	// (floor 0). SR-042 will resolve where this value lives on the
	// character + persist the decrement back; today the LongRest result
	// surfaces ExhaustionLevelAfter so callers (the /rest handler and any
	// future combatant-side persistence) can act on it.
	ExhaustionLevel int
}

// LongRestResult holds the results of a long rest.
type LongRestResult struct {
	HPBefore               int
	HPAfter                int
	HPMax                  int
	HPHealed               int
	TempHPCleared          bool
	HitDiceRemaining       map[string]int
	HitDiceRestored        int
	FeaturesRecharged      []string
	SpellSlots             map[string]character.SlotInfo
	PactSlotsRestored      bool
	DeathSavesReset        bool
	PreparedCasterReminder bool

	// Dawn recharge outputs (populated when input carries Inventory + RechargeInfo).
	UpdatedInventory []character.InventoryItem
	RechargedItems   []inventory.RechargedItem

	// SR-019: long-rest exhaustion decrement. ExhaustionLevelAfter is
	// max(0, input.ExhaustionLevel-1). ExhaustionDecreased is true only
	// when the level actually changed (i.e. input was > 0).
	ExhaustionLevelAfter int
	ExhaustionDecreased  bool
}

// LongRestExhaustionLevel returns the exhaustion level after a long rest:
// max(0, current-1). Per spec line 1365, a long rest (with food/water)
// decreases exhaustion by 1 level. Negative inputs are treated as 0 so the
// helper is total — callers do not need to validate. SR-042 will wire this
// into combatant persistence; today it is exposed via LongRest's result and
// the DM-dashboard exhaustion override.
func LongRestExhaustionLevel(current int) int {
	if current <= 0 {
		return 0
	}
	return current - 1
}

func ExhaustionLevelFromCharacterData(raw []byte) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return 0, false
	}
	value, ok := data[characterDataExhaustionKey]
	if !ok {
		return 0, false
	}
	var level int
	if err := json.Unmarshal(value, &level); err != nil {
		return 0, false
	}
	if level < 0 {
		return 0, true
	}
	return level, true
}

func CharacterDataWithExhaustion(raw []byte, level int) []byte {
	data := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data)
	}
	if level < 0 {
		level = 0
	}
	data[characterDataExhaustionKey] = level
	out, err := json.Marshal(data)
	if err != nil {
		return []byte(fmt.Sprintf(`{"%s":%d}`, characterDataExhaustionKey, level))
	}
	return out
}

// preparedCasterClasses are classes that prepare spells and should be reminded.
var preparedCasterClasses = map[string]bool{
	"cleric":  true,
	"druid":   true,
	"paladin": true,
}

// LongRest applies a long rest to a character.
func (s *Service) LongRest(input LongRestInput) LongRestResult {
	result := LongRestResult{
		HPBefore:         input.HPCurrent,
		HPAfter:          input.HPMax,
		HPMax:            input.HPMax,
		HPHealed:         input.HPMax - input.HPCurrent,
		HitDiceRemaining: maps.Clone(input.HitDiceRemaining),
	}

	// Restore spell slots
	result.SpellSlots = make(map[string]character.SlotInfo, len(input.SpellSlots))
	for level, slot := range input.SpellSlots {
		result.SpellSlots[level] = character.SlotInfo{Current: slot.Max, Max: slot.Max}
	}

	// Restore pact magic slots
	if input.PactMagicSlots != nil && input.PactMagicSlots.Max > 0 {
		if input.PactMagicSlots.Current < input.PactMagicSlots.Max {
			input.PactMagicSlots.Current = input.PactMagicSlots.Max
			result.PactSlotsRestored = true
		}
	}

	// Recharge all short and long rest features
	for name, fu := range input.FeatureUses {
		if (fu.Recharge == "short" || fu.Recharge == "long") && fu.Current < fu.Max {
			fu.Current = fu.Max
			input.FeatureUses[name] = fu
			result.FeaturesRecharged = append(result.FeaturesRecharged, name)
		}
	}

	// Restore hit dice: regain half total level (minimum 1)
	totalLevel := character.TotalLevel(input.Classes)
	toRestore := totalLevel / 2
	if toRestore < 1 {
		toRestore = 1
	}

	// Build max hit dice from classes
	maxHitDice := make(map[string]int)
	for _, c := range input.Classes {
		die := classHitDie(c.Class)
		maxHitDice[die] += c.Level
	}

	// Distribute restoration proportionally across die types
	restored := 0
	for die, maxCount := range maxHitDice {
		current := result.HitDiceRemaining[die]
		canRestore := maxCount - current
		if canRestore <= 0 {
			continue
		}
		restoreCount := toRestore - restored
		if restoreCount <= 0 {
			break
		}
		if restoreCount > canRestore {
			restoreCount = canRestore
		}
		result.HitDiceRemaining[die] = current + restoreCount
		restored += restoreCount
	}
	result.HitDiceRestored = restored

	// Death saves reset
	if input.DeathSaveSuccesses > 0 || input.DeathSaveFailures > 0 {
		result.DeathSavesReset = true
	}

	// SR-019: long rest decreases exhaustion by 1 (floor 0). Decreased
	// flag drives the format/Discord message — already-at-zero rests
	// stay silent on the exhaustion line.
	result.ExhaustionLevelAfter = LongRestExhaustionLevel(input.ExhaustionLevel)
	result.ExhaustionDecreased = input.ExhaustionLevel > 0

	// Prepared caster reminder
	for _, c := range input.Classes {
		if preparedCasterClasses[c.Class] {
			result.PreparedCasterReminder = true
			break
		}
	}

	// SR-053: long rest clears temp HP (spec line 1346).
	if input.TempHP > 0 {
		result.TempHPCleared = true
	}

	// Phase 88b: dawn recharge for magic items with charges.
	if len(input.Inventory) > 0 && len(input.RechargeInfo) > 0 {
		// Reuse a fresh inventory service per call — DawnRecharge is
		// stateless (only consults its random source). Wiring a shared
		// inventory.Service into rest.Service is a separate refactor.
		dawnRes, err := inventory.NewService(nil).DawnRecharge(inventory.DawnRechargeInput{
			Items:        input.Inventory,
			RechargeInfo: input.RechargeInfo,
		})
		if err == nil {
			result.UpdatedInventory = dawnRes.UpdatedItems
			result.RechargedItems = dawnRes.Recharged
		}
	}

	return result
}

// classHitDie returns the hit die string for a class.
func classHitDie(class string) string {
	switch class {
	case "barbarian":
		return "d12"
	case "fighter", "paladin", "ranger":
		return "d10"
	case "bard", "cleric", "druid", "monk", "rogue", "warlock":
		return "d8"
	case "sorcerer", "wizard":
		return "d6"
	default:
		return "d8"
	}
}
