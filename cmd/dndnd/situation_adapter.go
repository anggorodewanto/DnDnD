package main

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/situation"
)

// narrationTimelineLimit bounds the narration posts pulled for the DM Console
// timeline; the aggregator merges and re-truncates across all sources anyway.
const narrationTimelineLimit = 20

// narrationSummaryMax caps a narration body in the timeline so a long scene
// post stays a one-liner in the feed.
const narrationSummaryMax = 160

// situationProvider implements situation.Provider over refdata.Queries, mapping
// sqlc rows into the neutral shapes the aggregator consumes. It is a thin DB
// adapter (coverage-excluded like the other cmd/dndnd adapters); every piece of
// non-trivial aggregation logic — priority, sorting, timeline merge, next-step
// — lives in the unit-tested internal/situation package instead.
type situationProvider struct {
	queries *refdata.Queries
}

// newSituationProvider builds the refdata-backed Provider.
func newSituationProvider(queries *refdata.Queries) *situationProvider {
	return &situationProvider{queries: queries}
}

// parseUUID parses a string id; an invalid id yields a no-op (empty result)
// rather than an error, since the handler degrades to an empty Situation.
func parseUUID(id string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, false
	}
	return parsed, true
}

func (p *situationProvider) QueueItems(ctx context.Context, campaignID string) ([]situation.QueueRow, error) {
	id, ok := parseUUID(campaignID)
	if !ok {
		return nil, nil
	}
	items, err := p.queries.ListPendingDMQueueItems(ctx, id)
	if err != nil {
		return nil, err
	}
	rows := make([]situation.QueueRow, 0, len(items))
	for _, it := range items {
		rows = append(rows, situation.QueueRow{
			ID:         it.ID.String(),
			Kind:       it.Kind,
			Player:     it.PlayerName,
			Summary:    it.Summary,
			ResolveURL: it.ResolvePath,
			CreatedAt:  it.CreatedAt,
		})
	}
	return rows, nil
}

func (p *situationProvider) Approvals(ctx context.Context, campaignID string) ([]situation.ApprovalRow, error) {
	id, ok := parseUUID(campaignID)
	if !ok {
		return nil, nil
	}
	rows, err := p.queries.ListPlayerCharactersAwaitingApproval(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]situation.ApprovalRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, situation.ApprovalRow{
			ID:        r.ID.String(),
			Name:      r.CharacterName,
			Race:      r.Race,
			Level:     int(r.Level),
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (p *situationProvider) LevelUps(ctx context.Context, campaignID string) ([]situation.LevelUpRow, error) {
	id, ok := parseUUID(campaignID)
	if !ok {
		return nil, nil
	}
	rows, err := p.queries.ListPendingASIByCampaign(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]situation.LevelUpRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, situation.LevelUpRow{
			ID:        r.CharacterID.String(),
			Name:      r.CharacterName,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (p *situationProvider) Encounter(ctx context.Context, campaignID string) (*situation.EncounterRow, error) {
	id, ok := parseUUID(campaignID)
	if !ok {
		return nil, nil
	}
	encs, err := p.queries.ListEncountersByCampaignID(ctx, id)
	if err != nil {
		return nil, err
	}
	var active *refdata.Encounter
	for i := range encs {
		if encs[i].Status == "active" {
			active = &encs[i]
			break
		}
	}
	if active == nil {
		return nil, nil
	}

	combs, err := p.queries.ListCombatantsByEncounterID(ctx, active.ID)
	if err != nil {
		return nil, err
	}

	out := &situation.EncounterRow{
		ID:            active.ID.String(),
		Name:          combat.EncounterDisplayName(*active),
		Mode:          active.Mode,
		Status:        active.Status,
		Round:         int(active.RoundNumber),
		CurrentTurnID: p.currentTurnCombatantID(ctx, *active),
	}
	for _, c := range combs {
		out.Combatants = append(out.Combatants, situation.CombatantRow{
			ID:         c.ID.String(),
			Name:       c.DisplayName,
			ShortID:    c.ShortID,
			HPCurrent:  int(c.HpCurrent),
			HPMax:      int(c.HpMax),
			AC:         int(c.Ac),
			Col:        c.PositionCol,
			Row:        int(c.PositionRow),
			IsNPC:      c.IsNpc,
			IsAlive:    c.IsAlive,
			Conditions: parseConditionNames(c.Conditions),
		})
	}
	return out, nil
}

// currentTurnCombatantID resolves the encounter's current_turn_id (a turns row)
// to the combatant whose turn it is, so the aggregator can mark them. Best
// effort: any miss yields "" (no combatant flagged current).
func (p *situationProvider) currentTurnCombatantID(ctx context.Context, enc refdata.Encounter) string {
	if !enc.CurrentTurnID.Valid {
		return ""
	}
	turn, err := p.queries.GetTurn(ctx, enc.CurrentTurnID.UUID)
	if err != nil {
		return ""
	}
	return turn.CombatantID.String()
}

func (p *situationProvider) ActionEvents(ctx context.Context, encounterID string) ([]situation.TimelineRow, error) {
	id, ok := parseUUID(encounterID)
	if !ok {
		return nil, nil
	}
	logs, err := p.queries.ListActionLogByEncounterID(ctx, id)
	if err != nil {
		return nil, err
	}
	rows := make([]situation.TimelineRow, 0, len(logs))
	for _, l := range logs {
		summary := l.ActionType
		if l.Description.Valid && l.Description.String != "" {
			summary = l.Description.String
		}
		rows = append(rows, situation.TimelineRow{At: l.CreatedAt, Summary: summary})
	}
	return rows, nil
}

func (p *situationProvider) NarrationEvents(ctx context.Context, campaignID string) ([]situation.TimelineRow, error) {
	id, ok := parseUUID(campaignID)
	if !ok {
		return nil, nil
	}
	posts, err := p.queries.ListNarrationPostsByCampaign(ctx, refdata.ListNarrationPostsByCampaignParams{
		CampaignID: id,
		Limit:      narrationTimelineLimit,
		Offset:     0,
	})
	if err != nil {
		return nil, err
	}
	rows := make([]situation.TimelineRow, 0, len(posts))
	for _, post := range posts {
		rows = append(rows, situation.TimelineRow{
			At:      post.PostedAt,
			Actor:   "DM",
			Summary: truncateSummary(post.Body, narrationSummaryMax),
		})
	}
	return rows, nil
}

// ResolutionEvents is a no-op in v1: there is no campaign-scoped query for
// resolved dm-queue items yet, so DM resolutions are a future timeline source.
// The aggregator handles the empty slice as a no-op merge.
func (p *situationProvider) ResolutionEvents(_ context.Context, _ string) ([]situation.TimelineRow, error) {
	return nil, nil
}

// parseConditionNames extracts the condition names from a combatant's
// conditions JSON (an array of {"condition": "..."} objects). A nil/garbage
// blob yields no conditions rather than an error.
func parseConditionNames(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var conds []struct {
		Condition string `json:"condition"`
	}
	if err := json.Unmarshal(raw, &conds); err != nil {
		return nil
	}
	names := make([]string, 0, len(conds))
	for _, c := range conds {
		if c.Condition != "" {
			names = append(names, c.Condition)
		}
	}
	return names
}

// truncateSummary trims a body to max runes, appending an ellipsis when cut.
func truncateSummary(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
