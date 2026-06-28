package combat

import "github.com/ab/dndnd/internal/refdata"

// CreatureTurnSummary is the read-only moveset a DM needs to run an NPC's turn
// straight from the DM Console, without opening the full stat block (ISSUE-027):
// the creature's attacks plus whether it has recharge / legendary / lair
// options. It reports *availability* (what the creature can do) — not live
// per-turn resource state; the enemy-turn executor still resolves the chosen
// action.
type CreatureTurnSummary struct {
	Attacks           []CreatureAttackEntry
	RechargeAbilities []RechargeAbility
	HasLegendary      bool
	LegendaryBudget   int
	HasLair           bool
}

// RechargeAbility is one recharge-gated ability (e.g. "Fire Breath (Recharge
// 5-6)") and the minimum d6 it recharges on.
type RechargeAbility struct {
	Name        string
	RechargeMin int
}

// BuildCreatureTurnSummary parses a creature's stat block into the DM-facing
// turn summary, reusing the same parsers the Turn Builder uses so the Console
// and the executor agree on the moveset. It is best-effort: malformed attack
// JSON (or an open5e prose row) yields no structured attacks rather than an
// error, so a partial creature still surfaces what it can — matching the rest
// of the situation adapter's tolerant decoding.
func BuildCreatureTurnSummary(creature refdata.Creature) CreatureTurnSummary {
	summary := CreatureTurnSummary{}

	if attacks, err := ParseCreatureAttacksWithSource(creature.Attacks, creature.Source); err == nil {
		summary.Attacks = attacks
	}

	abilities := parseCreatureAbilitiesFromCreature(creature)
	for _, a := range abilities {
		if !isRechargeAbility(a.Name) {
			continue
		}
		summary.RechargeAbilities = append(summary.RechargeAbilities, RechargeAbility{
			Name:        a.Name,
			RechargeMin: parseRechargeMin(a.Name),
		})
	}

	// ParseLegendaryInfo returns non-nil exactly when the "Legendary Actions"
	// header is present (same detection as HasLegendaryActions), so a single
	// parse yields both the flag and the budget.
	if info := ParseLegendaryInfo(abilities); info != nil {
		summary.HasLegendary = true
		summary.LegendaryBudget = info.Budget
	}
	summary.HasLair = HasLairActions(abilities)

	return summary
}

// IsEmpty reports whether the summary carries nothing worth surfacing, so the
// adapter can omit creature_summary for a creature with no parsable moveset
// (e.g. a commoner, or an open5e prose row with no structured attacks).
func (s CreatureTurnSummary) IsEmpty() bool {
	return len(s.Attacks) == 0 &&
		len(s.RechargeAbilities) == 0 &&
		!s.HasLegendary &&
		!s.HasLair
}
