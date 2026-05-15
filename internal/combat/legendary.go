package combat

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// LegendaryAction represents one legendary action option with its cost.
type LegendaryAction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Cost        int    `json:"cost"`
}

// LegendaryInfo holds the legendary action configuration for a creature.
type LegendaryInfo struct {
	Budget  int               `json:"budget"`
	Actions []LegendaryAction `json:"actions"`
}

// LairAction represents one lair action option.
type LairAction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// LairInfo holds lair action configuration for a creature.
type LairInfo struct {
	Actions []LairAction `json:"actions"`
}

// legendaryBudgetRegexp matches "can take N legendary actions" from the description.
var legendaryBudgetRegexp = regexp.MustCompile(`can take (\d+) legendary action`)

// legendaryCostRegexp matches "(Costs N Actions)" from a legendary action name.
var legendaryCostRegexp = regexp.MustCompile(`\(Costs? (\d+) Actions?\)`)

// ParseLegendaryInfo extracts legendary action info from a creature's abilities list.
// Returns nil if the creature has no legendary actions.
func ParseLegendaryInfo(abilities []CreatureAbilityEntry) *LegendaryInfo {
	// Find the "Legendary Actions" header entry to determine budget
	budget := 0
	headerIdx := -1
	for i, a := range abilities {
		if strings.EqualFold(a.Name, "Legendary Actions") {
			headerIdx = i
			if m := legendaryBudgetRegexp.FindStringSubmatch(a.Description); len(m) >= 2 {
				n, err := strconv.Atoi(m[1])
				if err == nil {
					budget = n
				}
			}
			break
		}
	}

	if headerIdx < 0 {
		return nil
	}

	if budget == 0 {
		budget = 3 // default per 5e rules
	}

	// Collect actions that follow the header
	var actions []LegendaryAction
	for i := headerIdx + 1; i < len(abilities); i++ {
		a := abilities[i]
		// Stop if we hit another section header (e.g., "Lair Actions")
		if isAbilitySectionHeader(a.Name) {
			break
		}
		// Skip "Legendary Resistance" entries
		if strings.HasPrefix(a.Name, "Legendary Resistance") {
			continue
		}

		cost := 1
		cleanName := a.Name
		if m := legendaryCostRegexp.FindStringSubmatch(a.Name); len(m) >= 2 {
			n, err := strconv.Atoi(m[1])
			if err == nil {
				cost = n
			}
			// Strip cost annotation from name
			cleanName = strings.TrimSpace(legendaryCostRegexp.ReplaceAllString(a.Name, ""))
		}

		actions = append(actions, LegendaryAction{
			Name:        cleanName,
			Description: a.Description,
			Cost:        cost,
		})
	}

	if len(actions) == 0 {
		return nil
	}

	return &LegendaryInfo{
		Budget:  budget,
		Actions: actions,
	}
}

// isAbilitySectionHeader returns true if the ability name represents a
// section header like "Legendary Actions", "Lair Actions", etc.
func isAbilitySectionHeader(name string) bool {
	lower := strings.ToLower(name)
	return lower == "legendary actions" ||
		lower == "lair actions" ||
		lower == "mythic actions" ||
		lower == "reactions"
}

// LegendaryActionBudget tracks remaining legendary action budget within a round.
type LegendaryActionBudget struct {
	Total     int `json:"total"`
	Remaining int `json:"remaining"`
}

// NewLegendaryActionBudget creates a fresh budget for a round.
func NewLegendaryActionBudget(total int) LegendaryActionBudget {
	return LegendaryActionBudget{Total: total, Remaining: total}
}

// CanAfford returns true if the budget can cover the given cost.
func (b LegendaryActionBudget) CanAfford(cost int) bool {
	return b.Remaining >= cost
}

// Spend deducts the cost and returns an updated budget and an error if insufficient.
func (b LegendaryActionBudget) Spend(cost int) (LegendaryActionBudget, error) {
	if b.Remaining < cost {
		return b, fmt.Errorf("insufficient legendary actions: need %d, have %d", cost, b.Remaining)
	}
	b.Remaining -= cost
	return b, nil
}

// Reset restores the budget to its full value (called at creature's turn start).
func (b LegendaryActionBudget) Reset() LegendaryActionBudget {
	b.Remaining = b.Total
	return b
}

// ParseLairInfo extracts lair action info from a creature's abilities list.
// Returns nil if the creature has no lair actions.
func ParseLairInfo(abilities []CreatureAbilityEntry) *LairInfo {
	headerIdx := -1
	for i, a := range abilities {
		if strings.EqualFold(a.Name, "Lair Actions") {
			headerIdx = i
			break
		}
	}

	if headerIdx < 0 {
		return nil
	}

	var actions []LairAction
	for i := headerIdx + 1; i < len(abilities); i++ {
		a := abilities[i]
		if isAbilitySectionHeader(a.Name) {
			break
		}
		actions = append(actions, LairAction{
			Name:        a.Name,
			Description: a.Description,
		})
	}

	if len(actions) == 0 {
		return nil
	}

	return &LairInfo{Actions: actions}
}

// FormatLegendaryActionLog formats the combat log for a legendary action.
func FormatLegendaryActionLog(creatureName string, action LegendaryAction, budgetSpent, budgetTotal int) string {
	return fmt.Sprintf("\U0001f451  %s uses Legendary Action: %s (%d/%d)",
		creatureName, action.Name, budgetSpent, budgetTotal)
}

// FormatLairActionLog formats the combat log for a lair action.
func FormatLairActionLog(action LairAction) string {
	return fmt.Sprintf("\U0001f3f0  Lair Action (Initiative 20): %s", action.Name)
}

// LairActionTracker tracks which lair action was last used to enforce no-repeat.
type LairActionTracker struct {
	LastUsedName string `json:"last_used_name"`
}

// CanUse returns true if the lair action is available (not the same as last used).
func (t LairActionTracker) CanUse(name string) bool {
	return t.LastUsedName != name
}

// Use records the action as used and returns the updated tracker.
func (t LairActionTracker) Use(name string) LairActionTracker {
	t.LastUsedName = name
	return t
}

// LegendaryActionOption is an action with its affordability status for the plan.
type LegendaryActionOption struct {
	LegendaryAction
	Affordable bool `json:"affordable"`
}

// LegendaryActionPlan is the plan presented to the DM for a legendary action prompt.
type LegendaryActionPlan struct {
	CreatureName     string                  `json:"creature_name"`
	BudgetTotal      int                     `json:"budget_total"`
	BudgetRemaining  int                     `json:"budget_remaining"`
	AvailableActions []LegendaryActionOption `json:"available_actions"`
}

// BuildLegendaryActionPlan builds a legendary action plan for the DM prompt.
func BuildLegendaryActionPlan(creatureName string, info *LegendaryInfo, budget LegendaryActionBudget) LegendaryActionPlan {
	plan := LegendaryActionPlan{
		CreatureName:    creatureName,
		BudgetTotal:     budget.Total,
		BudgetRemaining: budget.Remaining,
	}

	for _, a := range info.Actions {
		plan.AvailableActions = append(plan.AvailableActions, LegendaryActionOption{
			LegendaryAction: a,
			Affordable:      budget.CanAfford(a.Cost),
		})
	}

	return plan
}

// LairActionPlan is the plan presented to the DM for a lair action turn.
type LairActionPlan struct {
	AvailableActions []LairAction `json:"available_actions"`
	DisabledActions  []LairAction `json:"disabled_actions"`
}

// BuildLairActionPlan builds a lair action plan, enforcing no-repeat rule.
func BuildLairActionPlan(info *LairInfo, tracker LairActionTracker) LairActionPlan {
	plan := LairActionPlan{}
	for _, a := range info.Actions {
		if tracker.CanUse(a.Name) {
			plan.AvailableActions = append(plan.AvailableActions, a)
		} else {
			plan.DisabledActions = append(plan.DisabledActions, a)
		}
	}
	return plan
}

// HasLegendaryActions returns true if the abilities list contains a "Legendary Actions" header.
func HasLegendaryActions(abilities []CreatureAbilityEntry) bool {
	return hasAbilityHeader(abilities, "Legendary Actions")
}

// HasLairActions returns true if the abilities list contains a "Lair Actions" header.
func HasLairActions(abilities []CreatureAbilityEntry) bool {
	return hasAbilityHeader(abilities, "Lair Actions")
}

// hasAbilityHeader returns true if any ability entry matches the given header name (case-insensitive).
func hasAbilityHeader(abilities []CreatureAbilityEntry, header string) bool {
	for _, a := range abilities {
		if strings.EqualFold(a.Name, header) {
			return true
		}
	}
	return false
}

// TurnQueueEntryType represents the type of entry in the turn queue.
const (
	TurnQueueCombatant  = "combatant"
	TurnQueueLegendary  = "legendary"
	TurnQueueLairAction = "lair_action"
)

// TurnQueueEntry represents one entry in the turn queue display.
type TurnQueueEntry struct {
	Type         string    `json:"type"`
	CombatantID  uuid.UUID `json:"combatant_id,omitempty"`
	DisplayName  string    `json:"display_name"`
	Initiative   int32     `json:"initiative"`
	IsNPC        bool      `json:"is_npc"`
	IsLairAction bool      `json:"is_lair_action,omitempty"`
}

// BuildTurnQueueEntries builds the full turn queue including legendary and lair action entries.
// legendaryCreatures maps combatant ID -> creature display name for creatures with legendary actions.
// lairCreatures maps combatant ID -> creature display name for creatures with lair actions.
func BuildTurnQueueEntries(combatants []refdata.Combatant, legendaryCreatures map[uuid.UUID]string, lairCreatures map[uuid.UUID]string) []TurnQueueEntry {
	var entries []TurnQueueEntry

	// Add lair action at initiative 20 (losing ties) if any creature has lair actions
	if len(lairCreatures) > 0 {
		// Pick the first lair creature for display name
		var name string
		for _, n := range lairCreatures {
			name = n
			break
		}
		entries = append(entries, TurnQueueEntry{
			Type:         TurnQueueLairAction,
			DisplayName:  name + " (Lair)",
			Initiative:   20,
			IsLairAction: true,
		})
	}

	// Add regular combatants and legendary action markers
	for _, c := range combatants {
		entries = append(entries, TurnQueueEntry{
			Type:        TurnQueueCombatant,
			CombatantID: c.ID,
			DisplayName: c.DisplayName,
			Initiative:  c.InitiativeRoll,
			IsNPC:       c.IsNpc,
		})

		// If this combatant is NOT a legendary creature, add legendary action marker after them
		// (Legendary actions can be taken after any other creature's turn)
		if _, isLegendary := legendaryCreatures[c.ID]; !isLegendary {
			for lid, lname := range legendaryCreatures {
				entries = append(entries, TurnQueueEntry{
					Type:        TurnQueueLegendary,
					CombatantID: lid,
					DisplayName: lname,
					Initiative:  c.InitiativeRoll, // same init as the preceding combatant
					IsNPC:       true,
				})
			}
		}
	}

	// Sort: higher initiative first; lair actions lose ties (go last at same initiative)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Initiative != entries[j].Initiative {
			return entries[i].Initiative > entries[j].Initiative
		}
		// Same initiative: lair actions go after non-lair entries
		if entries[i].IsLairAction != entries[j].IsLairAction {
			return !entries[i].IsLairAction
		}
		return false
	})

	return entries
}

// AvailableLairActions returns lair actions filtering out the last-used one.
func AvailableLairActions(info *LairInfo, tracker LairActionTracker) []LairAction {
	if info == nil {
		return nil
	}
	return BuildLairActionPlan(info, tracker).AvailableActions
}
