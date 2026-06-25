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

// CombatantView is one participant in the live encounter, with the fields a DM
// reads off the initiative tracker plus an IsCurrent flag for "whose turn".
type CombatantView struct {
	Name       string   `json:"name"`
	ShortID    string   `json:"short_id"`
	HPCurrent  int      `json:"hp_current"`
	HPMax      int      `json:"hp_max"`
	AC         int      `json:"ac"`
	Position   string   `json:"position"` // e.g. "E7"
	IsNPC      bool     `json:"is_npc"`
	IsAlive    bool     `json:"is_alive"`
	IsCurrent  bool     `json:"is_current"`
	Conditions []string `json:"conditions"`
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
