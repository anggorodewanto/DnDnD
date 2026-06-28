package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

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
	// Memoize parsed creature summaries by ref id so two combatants of the same
	// creature (e.g. a pack of ghouls) cost one GetCreature, not N.
	summaryCache := map[string]*situation.CreatureSummary{}
	for _, c := range combs {
		out.Combatants = append(out.Combatants, situation.CombatantRow{
			ID:                  c.ID.String(),
			Name:                c.DisplayName,
			ShortID:             c.ShortID,
			InitiativeOrder:     int(c.InitiativeOrder),
			Initiative:          int(c.InitiativeRoll),
			HPCurrent:           int(c.HpCurrent),
			HPMax:               int(c.HpMax),
			TempHP:              int(c.TempHp),
			AC:                  int(c.Ac),
			Col:                 c.PositionCol,
			Row:                 int(c.PositionRow),
			IsNPC:               c.IsNpc,
			IsAlive:             c.IsAlive,
			Exhaustion:          int(c.ExhaustionLevel),
			IsRaging:            c.IsRaging,
			RageRoundsRemaining: nullInt32To(c.RageRoundsRemaining),
			Concentration:       nullStringTo(c.ConcentrationSpellName),
			DeathSaves:          parseDeathSaves(c.DeathSaves),
			Conditions:          parseConditions(c.Conditions),
			CreatureSummary:     p.creatureSummary(ctx, c, summaryCache),
		})
	}
	return out, nil
}

// creatureSummary loads an NPC's moveset (attacks + recharge/legendary/lair
// availability) so the DM can run its turn from the Console (ISSUE-027). It is
// best-effort: PCs, NPCs with no creature ref, a GetCreature miss, or a creature
// with no parsable moveset all yield nil (the field is then omitted). Results
// are memoized per ref id in cache to avoid refetching shared creatures.
func (p *situationProvider) creatureSummary(ctx context.Context, c refdata.Combatant, cache map[string]*situation.CreatureSummary) *situation.CreatureSummary {
	if !c.IsNpc || !c.CreatureRefID.Valid {
		return nil
	}
	ref := c.CreatureRefID.String
	if cached, ok := cache[ref]; ok {
		return cached
	}
	cache[ref] = nil // memoize the miss/empty case by default
	creature, err := p.queries.GetCreature(ctx, ref)
	if err != nil {
		return nil
	}
	summary := combat.BuildCreatureTurnSummary(creature)
	if summary.IsEmpty() {
		return nil
	}
	view := mapCreatureSummary(summary)
	cache[ref] = view
	return view
}

// mapCreatureSummary converts the combat-domain turn summary into the neutral
// situation view shape served in the payload.
func mapCreatureSummary(s combat.CreatureTurnSummary) *situation.CreatureSummary {
	view := &situation.CreatureSummary{
		HasLegendary:    s.HasLegendary,
		LegendaryBudget: s.LegendaryBudget,
		HasLair:         s.HasLair,
	}
	for _, a := range s.Attacks {
		view.Attacks = append(view.Attacks, situation.AttackSummary{
			Name:       a.Name,
			ToHit:      a.ToHit,
			Damage:     a.Damage,
			DamageType: a.DamageType,
			ReachFt:    a.ReachFt,
			RangeFt:    a.RangeFt,
		})
	}
	for _, r := range s.RechargeAbilities {
		view.RechargeAbilities = append(view.RechargeAbilities, situation.RechargeSummary{
			Name:        r.Name,
			RechargeMin: r.RechargeMin,
		})
	}
	return view
}

func nullInt32To(v sql.NullInt32) int {
	if !v.Valid {
		return 0
	}
	return int(v.Int32)
}

func nullStringTo(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
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

// parseConditions maps a combatant's conditions JSON (an array of objects keyed
// by "condition", with optional duration/source/expiry metadata — see
// combat.CombatCondition) into the situation view's ConditionInfo, so the DM
// Console shows whether each condition is one-shot or ongoing, who applied it,
// and when it ends. A nil/garbage blob yields no conditions rather than an error.
func parseConditions(raw json.RawMessage) []situation.ConditionInfo {
	if len(raw) == 0 {
		return nil
	}
	var conds []struct {
		Condition         string `json:"condition"`
		DurationRounds    int    `json:"duration_rounds"`
		SourceCombatantID string `json:"source_combatant_id"`
		ExpiresOn         string `json:"expires_on"`
		SourceSpell       string `json:"source_spell"`
	}
	if err := json.Unmarshal(raw, &conds); err != nil {
		return nil
	}
	out := make([]situation.ConditionInfo, 0, len(conds))
	for _, c := range conds {
		if c.Condition == "" {
			continue
		}
		out = append(out, situation.ConditionInfo{
			Name:           c.Condition,
			DurationRounds: c.DurationRounds,
			SourceSpell:    c.SourceSpell,
			SourceID:       c.SourceCombatantID,
			ExpiresOn:      c.ExpiresOn,
		})
	}
	return out
}

// parseDeathSaves maps a combatant's death_saves JSON ({"successes","failures"})
// into the situation view. It returns nil when the column is null/empty OR an
// all-zero tally — PCs are seeded with {0,0}, so a zero tally means "not actually
// rolling death saves"; surfacing it would put a noisy "✓0 ✗0" badge on every
// healthy combatant. Once a real save lands (or fails) the badge appears.
func parseDeathSaves(raw pqtype.NullRawMessage) *situation.DeathSaves {
	if !raw.Valid || len(raw.RawMessage) == 0 {
		return nil
	}
	var ds situation.DeathSaves
	if err := json.Unmarshal(raw.RawMessage, &ds); err != nil {
		return nil
	}
	if ds.Successes == 0 && ds.Failures == 0 {
		return nil
	}
	return &ds
}

// truncateSummary trims a body to max runes, appending an ellipsis when cut.
func truncateSummary(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
