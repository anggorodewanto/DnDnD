package combat

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// attackStatsKind tags an action_log.dice_rolls payload as the structured
// attack-outcome record aggregated into post-combat "fun stats". Rows whose
// dice_rolls is absent or carries a different kind are ignored, so the feature
// is backward-compatible with encounters logged before it shipped.
const attackStatsKind = "attack"

// attackSwing is one resolved attack's outcome — the minimal shape needed to
// aggregate combat stats without re-parsing the human-readable description.
// A single /attack writes one swing; an NPC multiattack (or a Cleave) writes
// several. Field names are kept short because this is serialized on every
// attack row.
type attackSwing struct {
	Target string `json:"t,omitempty"` // target combatant UUID; "" when unattributed
	Hit    bool   `json:"h"`
	Crit   bool   `json:"c,omitempty"`
	Damage int    `json:"d,omitempty"` // damage dealt on a hit (player-visible amount)
}

// attackStatsPayload is the action_log.dice_rolls JSON shape for attack and
// enemy_turn rows.
type attackStatsPayload struct {
	Kind   string        `json:"kind"`
	Swings []attackSwing `json:"swings"`
}

// encodeAttackSwings marshals swings into the dice_rolls payload, or returns
// nil for an empty set so callers persist SQL NULL (no stats) rather than an
// empty object.
func encodeAttackSwings(swings []attackSwing) json.RawMessage {
	if len(swings) == 0 {
		return nil
	}
	raw, err := json.Marshal(attackStatsPayload{Kind: attackStatsKind, Swings: swings})
	if err != nil {
		return nil
	}
	return raw
}

// swingFromResult renders one AttackResult into a swing. A miss carries the
// target (so the defender is credited with an evade) but no damage.
func swingFromResult(r AttackResult, targetID uuid.UUID) attackSwing {
	sw := attackSwing{Hit: r.Hit, Crit: r.Hit && (r.CriticalHit || r.AutoCrit)}
	if targetID != uuid.Nil {
		sw.Target = targetID.String()
	}
	if r.Hit {
		sw.Damage = r.DamageTotal
	}
	return sw
}

// attackResultSwings turns a player-driven attack (with its optional Cleave
// secondary) into the swings persisted on the action_log row. The Cleave hits
// a different creature whose combatant ID is not threaded through the result,
// so it is recorded unattributed — it still counts toward the attacker's totals
// and the overall hit/crit rates, just not toward a specific defender's evades.
func attackResultSwings(r AttackResult, targetID uuid.UUID) []attackSwing {
	swings := []attackSwing{swingFromResult(r, targetID)}
	if r.CleaveAttack != nil {
		swings = append(swings, swingFromResult(*r.CleaveAttack, uuid.Nil))
	}
	return swings
}

// enemyTurnSwings extracts one swing per resolved attack step of an executed
// NPC turn plan. Steps that were never rolled (no RollResult) are skipped. The
// post-resistance FinalDamage is preferred once damage has landed so the stats
// match the amount the combat log reported.
func enemyTurnSwings(plan TurnPlan) []attackSwing {
	var swings []attackSwing
	for i := range plan.Steps {
		step := plan.Steps[i]
		if step.Type != StepTypeAttack || step.Attack == nil || step.Attack.RollResult == nil {
			continue
		}
		rr := step.Attack.RollResult
		sw := attackSwing{Hit: rr.Hit, Crit: rr.Hit && rr.Critical}
		if step.Attack.TargetID != uuid.Nil {
			sw.Target = step.Attack.TargetID.String()
		}
		if rr.Hit {
			dmg := rr.DamageTotal
			if rr.DamageResolved {
				dmg = rr.FinalDamage
			}
			sw.Damage = dmg
		}
		swings = append(swings, sw)
	}
	return swings
}

// CombatantStat is one combatant's aggregated attack record over an encounter.
type CombatantStat struct {
	Name        string
	Attacks     int
	Hits        int
	Misses      int
	Crits       int
	TotalDamage int
	MaxHit      int
	Evaded      int // attacks that missed this combatant while it was the target
	DamageTaken int // damage this combatant soaked as the target of landed hits
}

// CombatStats is the whole-encounter aggregate plus per-combatant breakdowns.
type CombatStats struct {
	ByID   map[uuid.UUID]*CombatantStat
	Total  int
	Hits   int
	Misses int
	Crits  int
}

// AggregateCombatStats walks an encounter's action_log rows and tallies attack
// outcomes from the structured dice_rolls payloads. Rows without a stats
// payload (older rows, non-attack actions, malformed JSON) are ignored, so the
// result degrades gracefully rather than erroring.
func AggregateCombatStats(rows []refdata.ActionLog, combatants []refdata.Combatant) CombatStats {
	nameByID := make(map[uuid.UUID]string, len(combatants))
	for _, c := range combatants {
		nameByID[c.ID] = c.DisplayName
	}

	cs := CombatStats{ByID: make(map[uuid.UUID]*CombatantStat)}
	ensure := func(id uuid.UUID) *CombatantStat {
		st, ok := cs.ByID[id]
		if ok {
			return st
		}
		st = &CombatantStat{Name: nameByID[id]}
		cs.ByID[id] = st
		return st
	}

	for _, row := range rows {
		if !row.DiceRolls.Valid {
			continue
		}
		var p attackStatsPayload
		if err := json.Unmarshal(row.DiceRolls.RawMessage, &p); err != nil || p.Kind != attackStatsKind {
			continue
		}
		attacker := ensure(row.ActorID)
		for _, sw := range p.Swings {
			cs.Total++
			attacker.Attacks++
			if !sw.Hit {
				cs.Misses++
				attacker.Misses++
				if tid, err := uuid.Parse(sw.Target); err == nil && tid != uuid.Nil {
					ensure(tid).Evaded++
				}
				continue
			}
			cs.Hits++
			attacker.Hits++
			attacker.TotalDamage += sw.Damage
			if tid, err := uuid.Parse(sw.Target); err == nil && tid != uuid.Nil {
				ensure(tid).DamageTaken += sw.Damage
			}
			if sw.Damage > attacker.MaxHit {
				attacker.MaxHit = sw.Damage
			}
			if sw.Crit {
				cs.Crits++
				attacker.Crits++
			}
		}
	}
	return cs
}

// encounterDisplayName returns the player-facing encounter label, preferring the
// display name over the internal name so combat-log posts stay consistent with
// the running initiative tracker.
func encounterDisplayName(enc refdata.Encounter) string {
	if enc.DisplayName.Valid && enc.DisplayName.String != "" {
		return enc.DisplayName.String
	}
	return enc.Name
}

// pct returns n/total as an integer percentage, guarding division by zero.
func pct(n, total int) int {
	if total <= 0 {
		return 0
	}
	return n * 100 / total
}

// leader returns the combatant name and value with the highest val(stat),
// scanning in a deterministic (name, then ID) order so ties break stably. It
// returns ("", 0) when no combatant scores above zero.
func leader(cs CombatStats, val func(*CombatantStat) int) (string, int) {
	ids := make([]uuid.UUID, 0, len(cs.ByID))
	for id := range cs.ByID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		ni, nj := cs.ByID[ids[i]].Name, cs.ByID[ids[j]].Name
		if ni != nj {
			return ni < nj
		}
		return ids[i].String() < ids[j].String()
	})

	bestName, bestVal := "", 0
	for _, id := range ids {
		if v := val(cs.ByID[id]); v > bestVal {
			bestName, bestVal = cs.ByID[id].Name, v
		}
	}
	return bestName, bestVal
}

// FormatCombatStats renders the post-combat "fun stats" message for #combat-log.
// It returns "" when no attacks were recorded so the caller can skip posting an
// empty summary. Award lines only appear when there is a non-zero leader.
func FormatCombatStats(cs CombatStats, encName string) string {
	if cs.Total == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📊 **Combat Stats — %s**\n", encName)
	fmt.Fprintf(&b, "🎲 %d attacks · %d%% hit · %d%% miss · %d%% crit\n",
		cs.Total, pct(cs.Hits, cs.Total), pct(cs.Misses, cs.Total), pct(cs.Crits, cs.Total))

	if name, val := leader(cs, func(s *CombatantStat) int { return s.MaxHit }); val > 0 {
		fmt.Fprintf(&b, "💥 Biggest hit — %s (%d dmg)\n", name, val)
	}
	if name, val := leader(cs, func(s *CombatantStat) int { return s.TotalDamage }); val > 0 {
		fmt.Fprintf(&b, "🗡️ Most damage — %s (%d total)\n", name, val)
	}
	if name, val := leader(cs, func(s *CombatantStat) int { return s.Crits }); val > 0 {
		fmt.Fprintf(&b, "🍀 Most crits — %s (%d)\n", name, val)
	}
	if name, val := leader(cs, func(s *CombatantStat) int { return s.DamageTaken }); val > 0 {
		fmt.Fprintf(&b, "🛡️ Most damage tanked — %s (%d taken)\n", name, val)
	}
	if name, val := leader(cs, func(s *CombatantStat) int { return s.Evaded }); val > 0 {
		fmt.Fprintf(&b, "🌫️ Most evasive — %s (%d dodged)\n", name, val)
	}

	return strings.TrimRight(b.String(), "\n")
}
