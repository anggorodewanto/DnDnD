// Package situation aggregates the scattered DM-facing data sources (pending
// dm-queue items, character approvals, level-up requests, live encounter
// state, and a recent event timeline) into a single "DM Situation" view.
//
// It exists because a DM — human or AI — otherwise has to look in 6+ places
// (Discord #dm-queue, dashboard approval/level-up tabs, #initiative-tracker,
// #combat-log, action_log, narration) to answer three questions: what needs my
// action, where are we, and what just happened. The Service composes a Provider
// (backed by the DB in production, a fake in tests) into one Situation that
// both the dashboard DM Console and the AI DM read as a single source of truth.
package situation

import "time"

// Situation is the aggregated DM view for one campaign: everything awaiting
// action, the live state, the recent timeline, and a single derived next step.
type Situation struct {
	Pending  []PendingItem   `json:"pending"`
	State    StateView       `json:"state"`
	Timeline []TimelineEvent `json:"timeline"`
	NextStep string          `json:"next_step"`
}

// PendingItem is one thing awaiting DM action, unified across every source
// (dm-queue items, character approvals, level-up requests). Priority orders the
// list so combat-blocking items surface above non-urgent requests; lower is
// more urgent.
type PendingItem struct {
	ID         string    `json:"id"`
	Source     string    `json:"source"` // SourceQueue | SourceApproval | SourceLevelUp
	Kind       string    `json:"kind"`   // queue kind, or "character_approval" / "level_up"
	Label      string    `json:"label"`  // human-readable kind label
	Player     string    `json:"player"` // player / character name
	Summary    string    `json:"summary"`
	ResolveURL string    `json:"resolve_url"` // dashboard deep-link when available
	Priority   int       `json:"priority"`    // lower = more urgent
	CreatedAt  time.Time `json:"created_at"`
}

// StateView is the live game-state snapshot the DM needs at a glance.
type StateView struct {
	HasEncounter bool            `json:"has_encounter"`
	EncounterID  string          `json:"encounter_id"`
	Name         string          `json:"name"`
	Mode         string          `json:"mode"` // "combat" | "exploration"
	Status       string          `json:"status"`
	Round        int             `json:"round"`
	CurrentTurn  string          `json:"current_turn"` // name of the combatant whose turn it is
	Combatants   []CombatantView `json:"combatants"`
}

// CombatantView is one participant in the live encounter, with everything a DM
// needs to adjudicate its turn without tracking anything by hand: position,
// HP/temp-HP, initiative, the IsCurrent "whose turn" flag, and the resource
// state that used to live only in the DB (concentration, rage, exhaustion,
// death saves) plus condition metadata (duration / source / expiry).
type CombatantView struct {
	Name                string          `json:"name"`
	ShortID             string          `json:"short_id"`
	Initiative          int             `json:"initiative"` // initiative roll (display); combatants are returned in turn order
	HPCurrent           int             `json:"hp_current"`
	HPMax               int             `json:"hp_max"`
	TempHP              int             `json:"temp_hp"`
	AC                  int             `json:"ac"`
	Position            string          `json:"position"` // e.g. "E7"
	IsNPC               bool            `json:"is_npc"`
	IsAlive             bool            `json:"is_alive"`
	IsCurrent           bool            `json:"is_current"`
	Exhaustion          int             `json:"exhaustion"`
	IsRaging            bool            `json:"is_raging"`
	RageRoundsRemaining int             `json:"rage_rounds_remaining,omitempty"`
	Concentration       string          `json:"concentration,omitempty"` // spell being concentrated on, "" if none
	DeathSaves          *DeathSaves     `json:"death_saves,omitempty"`   // non-nil only for a downed combatant rolling saves
	Conditions          []ConditionInfo `json:"conditions"`
	// CreatureSummary is an NPC's moveset for running its turn from the Console
	// (attacks + recharge/legendary/lair availability). Nil for PCs and for
	// NPCs whose creature has no parsable moveset, so the field is omitted.
	CreatureSummary *CreatureSummary `json:"creature_summary,omitempty"`
}

// CreatureSummary is an NPC's read-only moveset, surfaced so a DM can run the
// enemy's turn straight from the DM Console without opening the stat block
// (ISSUE-027). It reports *availability* (what the creature can do) — not live
// per-turn resource state; the executor still resolves the chosen action.
type CreatureSummary struct {
	Attacks           []AttackSummary   `json:"attacks,omitempty"`
	RechargeAbilities []RechargeSummary `json:"recharge_abilities,omitempty"`
	HasLegendary      bool              `json:"has_legendary,omitempty"`
	LegendaryBudget   int               `json:"legendary_budget,omitempty"`
	HasLair           bool              `json:"has_lair,omitempty"`
}

// AttackSummary is one NPC attack: name, to-hit, damage dice, and reach/range —
// everything the DM needs to narrate and adjudicate the swing.
type AttackSummary struct {
	Name       string `json:"name"`
	ToHit      int    `json:"to_hit"`
	Damage     string `json:"damage"`
	DamageType string `json:"damage_type,omitempty"`
	ReachFt    int    `json:"reach_ft,omitempty"`
	RangeFt    int    `json:"range_ft,omitempty"`
}

// RechargeSummary is one recharge-gated ability and the minimum d6 it recharges
// on (e.g. "Fire Breath (Recharge 5-6)" → 5).
type RechargeSummary struct {
	Name        string `json:"name"`
	RechargeMin int    `json:"recharge_min,omitempty"`
}

// DeathSaves is a downed combatant's running death-save tally — the DM needs it
// to know whether the next failure is fatal.
type DeathSaves struct {
	Successes int `json:"successes"`
	Failures  int `json:"failures"`
}

// ConditionInfo is one active condition plus the metadata that tells a DM
// whether it is one-shot or ongoing, who applied it, and when it ends — so a
// rider like a spell's lingering effect doesn't have to be tracked by hand.
type ConditionInfo struct {
	Name           string `json:"name"`
	DurationRounds int    `json:"duration_rounds,omitempty"` // 0 = indefinite / unknown
	SourceSpell    string `json:"source_spell,omitempty"`
	SourceID       string `json:"source_combatant_id,omitempty"`
	ExpiresOn      string `json:"expires_on,omitempty"` // "start_of_turn" | "end_of_turn" | ""
}

// TimelineEvent is one entry in the merged "what just happened" feed (combat
// actions, narration posts, and dm-queue resolutions), newest first.
type TimelineEvent struct {
	At      time.Time `json:"at"`
	Source  string    `json:"source"` // "action" | "narration" | "resolution"
	Actor   string    `json:"actor"`
	Summary string    `json:"summary"`
}

// Pending source discriminators (the Source field on PendingItem).
const (
	SourceQueue    = "queue"
	SourceApproval = "approval"
	SourceLevelUp  = "levelup"
)
