package situation

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// timelineLimit caps the merged recent-event feed so a long encounter doesn't
// flood the DM Console; the newest events are the ones that matter.
const timelineLimit = 20

// Provider supplies the raw, lightly-typed rows the Service aggregates. The
// production implementation (cmd/dndnd) maps refdata/sqlc rows into these
// neutral shapes; tests supply a fake. Every method is campaign-scoped and
// best-effort: returning an error for one source must not blank the others, so
// the Service skips a failed source and joins the errors for the caller to log.
type Provider interface {
	QueueItems(ctx context.Context, campaignID string) ([]QueueRow, error)
	Approvals(ctx context.Context, campaignID string) ([]ApprovalRow, error)
	LevelUps(ctx context.Context, campaignID string) ([]LevelUpRow, error)
	// Encounter returns the campaign's single active encounter, or nil when
	// there is none (out of combat / between encounters).
	Encounter(ctx context.Context, campaignID string) (*EncounterRow, error)
	// ActionEvents / NarrationEvents / ResolutionEvents feed the timeline.
	// ActionEvents is encounter-scoped; encounterID is "" when no encounter.
	ActionEvents(ctx context.Context, encounterID string) ([]TimelineRow, error)
	NarrationEvents(ctx context.Context, campaignID string) ([]TimelineRow, error)
	ResolutionEvents(ctx context.Context, campaignID string) ([]TimelineRow, error)
}

// QueueRow is a pending dm_queue_items row in neutral form.
type QueueRow struct {
	ID         string
	Kind       string
	Player     string
	Summary    string
	ResolveURL string
	CreatedAt  time.Time
}

// ApprovalRow is a player character awaiting DM approval.
type ApprovalRow struct {
	ID        string
	Name      string
	Race      string
	Level     int
	CreatedAt time.Time
}

// LevelUpRow is a pending level-up / ASI awaiting DM approval.
type LevelUpRow struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// EncounterRow is the live encounter plus its combatants.
type EncounterRow struct {
	ID            string
	Name          string
	Mode          string
	Status        string
	Round         int
	CurrentTurnID string // combatant ID whose turn it is ("" if none)
	Combatants    []CombatantRow
}

// CombatantRow is one participant in neutral form.
type CombatantRow struct {
	ID         string
	Name       string
	ShortID    string
	HPCurrent  int
	HPMax      int
	AC         int
	Col        string
	Row        int
	IsNPC      bool
	IsAlive    bool
	Conditions []string
}

// TimelineRow is one timeline event in neutral form (the Service stamps Source).
type TimelineRow struct {
	At      time.Time
	Actor   string
	Summary string
}

// Service builds a Situation from a Provider. It owns all the aggregation
// logic — priority assignment, sorting, timeline merge/truncate, and next-step
// derivation — so that logic is unit-tested independent of the database.
type Service struct {
	provider Provider
}

// NewService constructs a Service over the given Provider.
func NewService(provider Provider) *Service {
	return &Service{provider: provider}
}

// Build assembles the Situation for one campaign. It is best-effort: a failure
// in any single source contributes nothing from that source but never aborts
// the whole view; the returned error joins all source failures so the caller
// (the dashboard handler) can log them while still rendering the partial view.
func (s *Service) Build(ctx context.Context, campaignID string) (Situation, error) {
	var errs []error
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	pending := s.buildPending(ctx, campaignID, collect)

	encounter, err := s.provider.Encounter(ctx, campaignID)
	collect(err)
	state := buildState(encounter)

	timeline := s.buildTimeline(ctx, campaignID, state.EncounterID, collect)

	return Situation{
		Pending:  pending,
		State:    state,
		Timeline: timeline,
		NextStep: deriveNextStep(pending, encounter),
	}, errors.Join(errs...)
}

// buildPending unifies the three pending sources into one priority-sorted list.
func (s *Service) buildPending(ctx context.Context, campaignID string, collect func(error)) []PendingItem {
	items := []PendingItem{}

	queue, err := s.provider.QueueItems(ctx, campaignID)
	collect(err)
	for _, q := range queue {
		items = append(items, PendingItem{
			ID:         q.ID,
			Source:     SourceQueue,
			Kind:       q.Kind,
			Label:      queueKindLabel(q.Kind),
			Player:     q.Player,
			Summary:    q.Summary,
			ResolveURL: q.ResolveURL,
			Priority:   queueKindPriority(q.Kind),
			CreatedAt:  q.CreatedAt,
		})
	}

	approvals, err := s.provider.Approvals(ctx, campaignID)
	collect(err)
	for _, a := range approvals {
		items = append(items, PendingItem{
			ID:        a.ID,
			Source:    SourceApproval,
			Kind:      "character_approval",
			Label:     "Character Approval",
			Player:    a.Name,
			Summary:   approvalSummary(a),
			Priority:  priorityApproval,
			CreatedAt: a.CreatedAt,
		})
	}

	levelUps, err := s.provider.LevelUps(ctx, campaignID)
	collect(err)
	for _, l := range levelUps {
		items = append(items, PendingItem{
			ID:        l.ID,
			Source:    SourceLevelUp,
			Kind:      "level_up",
			Label:     "Level-Up Approval",
			Player:    l.Name,
			Summary:   "Level-up awaiting approval",
			Priority:  priorityLevelUp,
			CreatedAt: l.CreatedAt,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}

// buildState maps the active encounter (if any) into the StateView, marking the
// current-turn combatant and formatting grid positions.
func buildState(enc *EncounterRow) StateView {
	if enc == nil {
		return StateView{}
	}
	state := StateView{
		HasEncounter: true,
		EncounterID:  enc.ID,
		Name:         enc.Name,
		Mode:         enc.Mode,
		Status:       enc.Status,
		Round:        enc.Round,
	}
	for _, c := range enc.Combatants {
		isCurrent := c.ID != "" && c.ID == enc.CurrentTurnID
		if isCurrent {
			state.CurrentTurn = c.Name
		}
		state.Combatants = append(state.Combatants, CombatantView{
			Name:       c.Name,
			ShortID:    c.ShortID,
			HPCurrent:  c.HPCurrent,
			HPMax:      c.HPMax,
			AC:         c.AC,
			Position:   formatPosition(c.Col, c.Row),
			IsNPC:      c.IsNPC,
			IsAlive:    c.IsAlive,
			IsCurrent:  isCurrent,
			Conditions: c.Conditions,
		})
	}
	return state
}

// buildTimeline merges the three event sources, tags each with its origin,
// sorts newest-first, and truncates to timelineLimit.
func (s *Service) buildTimeline(ctx context.Context, campaignID, encounterID string, collect func(error)) []TimelineEvent {
	events := []TimelineEvent{}
	add := func(rows []TimelineRow, source string) {
		for _, r := range rows {
			events = append(events, TimelineEvent{At: r.At, Source: source, Actor: r.Actor, Summary: r.Summary})
		}
	}

	if encounterID != "" {
		actions, err := s.provider.ActionEvents(ctx, encounterID)
		collect(err)
		add(actions, "action")
	}

	narration, err := s.provider.NarrationEvents(ctx, campaignID)
	collect(err)
	add(narration, "narration")

	resolutions, err := s.provider.ResolutionEvents(ctx, campaignID)
	collect(err)
	add(resolutions, "resolution")

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].At.After(events[j].At)
	})
	if len(events) > timelineLimit {
		events = events[:timelineLimit]
	}
	return events
}

// deriveNextStep produces the single "what should the DM do now" hint. An NPC's
// live combat turn outranks everything (it blocks the round); otherwise the
// top-priority pending item; otherwise nothing.
func deriveNextStep(pending []PendingItem, enc *EncounterRow) string {
	if enc != nil && enc.CurrentTurnID != "" {
		for _, c := range enc.Combatants {
			if c.ID == enc.CurrentTurnID && c.IsNPC && c.IsAlive {
				return fmt.Sprintf("Resolve %s's turn (NPC enemy).", c.Name)
			}
		}
	}
	if len(pending) > 0 {
		top := pending[0]
		if s := strings.TrimSpace(top.Summary); s != "" {
			return fmt.Sprintf("Resolve %s from %s: %s", top.Label, top.Player, s)
		}
		return fmt.Sprintf("Resolve %s from %s.", top.Label, top.Player)
	}
	return ""
}

func formatPosition(col string, row int) string {
	if col == "" && row == 0 {
		return ""
	}
	return col + strconv.Itoa(row)
}

func approvalSummary(a ApprovalRow) string {
	parts := make([]string, 0, 2)
	if a.Race != "" {
		parts = append(parts, a.Race)
	}
	if a.Level > 0 {
		parts = append(parts, fmt.Sprintf("level %d", a.Level))
	}
	if len(parts) == 0 {
		return "Character awaiting approval"
	}
	return strings.Join(parts, " ") + " awaiting approval"
}

// Pending priorities. Lower is more urgent. Queue kinds are split into bands so
// combat-blocking work (an NPC turn, an opportunity attack) surfaces above
// player requests, which surface above housekeeping (rests, retires) and
// non-urgent approvals/level-ups.
const (
	priorityCombatBlocking = 0
	prioritySystemAlert    = 1
	priorityReaction       = 2
	priorityPlayerAction   = 3
	priorityNarration      = 4
	priorityHousekeeping   = 5
	priorityApproval       = 6
	priorityLevelUp        = 6
	priorityDefault        = 7
)

func queueKindPriority(kind string) int {
	switch kind {
	case "enemy_turn_ready", "opportunity_attack":
		return priorityCombatBlocking
	case "map_render_failure":
		return prioritySystemAlert
	case "reaction_declaration":
		return priorityReaction
	case "freeform_action", "player_whisper", "consumable", "narrative_teleport", "channel_divinity":
		return priorityPlayerAction
	case "skill_check_narration":
		return priorityNarration
	case "rest_request", "undo_request", "retire_request":
		return priorityHousekeeping
	default:
		return priorityDefault
	}
}

func queueKindLabel(kind string) string {
	switch kind {
	case "freeform_action":
		return "Freeform Action"
	case "reaction_declaration":
		return "Reaction Declaration"
	case "rest_request":
		return "Rest Request"
	case "skill_check_narration":
		return "Skill Check Narration"
	case "consumable":
		return "Consumable Usage"
	case "enemy_turn_ready":
		return "Enemy Turn Ready"
	case "narrative_teleport":
		return "Narrative Teleport"
	case "player_whisper":
		return "Player Whisper"
	case "undo_request":
		return "Undo Request"
	case "retire_request":
		return "Retire Request"
	case "opportunity_attack":
		return "Opportunity Attack"
	case "channel_divinity":
		return "Channel Divinity"
	case "map_render_failure":
		return "Map Render Failed"
	default:
		return "Notification"
	}
}
