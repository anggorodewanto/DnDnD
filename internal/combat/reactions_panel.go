package combat

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ReactionPanelEntry is an enriched reaction declaration for the DM dashboard panel.
type ReactionPanelEntry struct {
	ID                    uuid.UUID `json:"id"`
	EncounterID           uuid.UUID `json:"encounter_id"`
	CombatantID           uuid.UUID `json:"combatant_id"`
	CombatantDisplayName  string    `json:"combatant_display_name"`
	CombatantShortID      string    `json:"combatant_short_id"`
	Description           string    `json:"description"`
	Status                string    `json:"status"`
	IsReadiedAction       bool      `json:"is_readied_action"`
	ReactionUsedThisRound bool      `json:"reaction_used_this_round"`
	IsNpc                 bool      `json:"is_npc"`
}

type combatantInfo struct {
	DisplayName string
	ShortID     string
	IsNpc       bool
}

// ListReactionsForPanel returns all reaction declarations for an encounter,
// enriched with combatant display info and reaction-used status for the DM panel.
func (s *Service) ListReactionsForPanel(ctx context.Context, encounterID uuid.UUID) ([]ReactionPanelEntry, error) {
	decls, err := s.store.ListReactionDeclarationsByEncounter(ctx, encounterID)
	if err != nil {
		return nil, err
	}

	if len(decls) == 0 {
		return []ReactionPanelEntry{}, nil
	}

	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, err
	}

	infoByID := make(map[uuid.UUID]combatantInfo, len(combatants))
	for _, c := range combatants {
		infoByID[c.ID] = combatantInfo{DisplayName: c.DisplayName, ShortID: c.ShortID, IsNpc: c.IsNpc}
	}

	reactionUsedMap, err := s.buildReactionUsedMap(ctx, encounterID)
	if err != nil {
		return nil, err
	}

	result := make([]ReactionPanelEntry, len(decls))
	for i, d := range decls {
		info := infoByID[d.CombatantID]
		result[i] = ReactionPanelEntry{
			ID:                    d.ID,
			EncounterID:           d.EncounterID,
			CombatantID:           d.CombatantID,
			CombatantDisplayName:  info.DisplayName,
			CombatantShortID:      info.ShortID,
			Description:           d.Description,
			Status:                d.Status,
			IsReadiedAction:       d.IsReadiedAction,
			ReactionUsedThisRound: reactionUsedMap[d.CombatantID],
			IsNpc:                 info.IsNpc,
		}
	}

	return result, nil
}

// buildReactionUsedMap returns a map of combatant IDs that have used their reaction this round.
func (s *Service) buildReactionUsedMap(ctx context.Context, encounterID uuid.UUID) (map[uuid.UUID]bool, error) {
	activeTurn, err := s.store.GetActiveTurnByEncounterID(ctx, encounterID)
	if err == sql.ErrNoRows {
		return map[uuid.UUID]bool{}, nil
	}
	if err != nil {
		return nil, err
	}

	turns, err := s.store.ListTurnsByEncounterAndRound(ctx, refdata.ListTurnsByEncounterAndRoundParams{
		EncounterID: encounterID,
		RoundNumber: activeTurn.RoundNumber,
	})
	if err != nil {
		return nil, err
	}

	used := make(map[uuid.UUID]bool, len(turns))
	for _, t := range turns {
		if t.ReactionUsed {
			used[t.CombatantID] = true
		}
	}
	return used, nil
}
