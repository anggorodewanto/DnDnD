package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ActionLogFilter defines optional filters for the Action Log viewer.
type ActionLogFilter struct {
	ActionTypes []string    // include only entries whose action_type is in this set (empty = all)
	ActorID     uuid.UUID   // zero = no filter
	TargetID    uuid.UUID   // zero = no filter
	Round       int32       // 0 = no filter
	TurnID      uuid.UUID   // zero = no filter
	SortAsc     bool        // default false = newest-first
}

// ActionLogViewerEntry is an enriched action log row for the DM dashboard viewer.
type ActionLogViewerEntry struct {
	ID                uuid.UUID       `json:"id"`
	TurnID            uuid.UUID       `json:"turn_id"`
	EncounterID       uuid.UUID       `json:"encounter_id"`
	ActionType        string          `json:"action_type"`
	ActorID           uuid.UUID       `json:"actor_id"`
	ActorDisplayName  string          `json:"actor_display_name"`
	ActorShortID      string          `json:"actor_short_id"`
	TargetID          *uuid.UUID      `json:"target_id,omitempty"`
	TargetDisplayName string          `json:"target_display_name,omitempty"`
	TargetShortID     string          `json:"target_short_id,omitempty"`
	Description       string          `json:"description"`
	BeforeState       json.RawMessage `json:"before_state"`
	AfterState        json.RawMessage `json:"after_state"`
	DiceRolls         json.RawMessage `json:"dice_rolls,omitempty"`
	CreatedAt         string          `json:"created_at"`
	RoundNumber       int32           `json:"round_number"`
	IsOverride        bool            `json:"is_override"`
}

// ListActionLogForViewer returns filtered+sorted enriched action log entries for an encounter.
func (s *Service) ListActionLogForViewer(ctx context.Context, encounterID uuid.UUID, filter ActionLogFilter) ([]ActionLogViewerEntry, error) {
	rows, err := s.store.ListActionLogWithRounds(ctx, uuid.NullUUID{UUID: encounterID, Valid: true})
	if err != nil {
		return nil, err
	}

	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, err
	}

	infoByID := make(map[uuid.UUID]combatantInfo, len(combatants))
	for _, c := range combatants {
		infoByID[c.ID] = combatantInfo{DisplayName: c.DisplayName, ShortID: c.ShortID, IsNpc: c.IsNpc}
	}

	typeSet := buildTypeSet(filter.ActionTypes)

	filtered := make([]refdata.ListActionLogWithRoundsRow, 0, len(rows))
	for _, row := range rows {
		if !matchesFilter(row, filter, typeSet) {
			continue
		}
		filtered = append(filtered, row)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filter.SortAsc {
			return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	entries := make([]ActionLogViewerEntry, len(filtered))
	for i, row := range filtered {
		entries[i] = enrichActionLogRow(row, infoByID)
	}
	return entries, nil
}

// buildTypeSet returns a set of action types; empty set means "no filter".
func buildTypeSet(types []string) map[string]struct{} {
	if len(types) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(types))
	for _, t := range types {
		if t == "" {
			continue
		}
		set[t] = struct{}{}
	}
	return set
}

// matchesFilter returns true if the row passes all active filter predicates.
func matchesFilter(row refdata.ListActionLogWithRoundsRow, f ActionLogFilter, typeSet map[string]struct{}) bool {
	if typeSet != nil {
		if _, ok := typeSet[row.ActionType]; !ok {
			return false
		}
	}
	if f.ActorID != uuid.Nil {
		if !row.ActorID.Valid || row.ActorID.UUID != f.ActorID {
			return false
		}
	}
	if f.TargetID != uuid.Nil {
		if !row.TargetID.Valid || row.TargetID.UUID != f.TargetID {
			return false
		}
	}
	if f.Round != 0 && row.RoundNumber != f.Round {
		return false
	}
	if f.TurnID != uuid.Nil {
		if !row.TurnID.Valid || row.TurnID.UUID != f.TurnID {
			return false
		}
	}
	return true
}

// enrichActionLogRow converts a row into a viewer entry with combatant display info.
// Phase 118c: turn_id/encounter_id/actor_id are nullable on action_log so error
// rows can be inserted without a parent. We surface them as zero UUIDs in the
// viewer entry — JSON consumers already tolerate uuid.Nil and the viewer
// renders error rows separately.
func enrichActionLogRow(row refdata.ListActionLogWithRoundsRow, infoByID map[uuid.UUID]combatantInfo) ActionLogViewerEntry {
	actorID := row.ActorID.UUID // zero if invalid
	actorInfo := infoByID[actorID]
	entry := ActionLogViewerEntry{
		ID:               row.ID,
		TurnID:           row.TurnID.UUID,
		EncounterID:      row.EncounterID.UUID,
		ActionType:       row.ActionType,
		ActorID:          actorID,
		ActorDisplayName: actorInfo.DisplayName,
		ActorShortID:     actorInfo.ShortID,
		BeforeState:      row.BeforeState,
		AfterState:       row.AfterState,
		CreatedAt:        row.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		RoundNumber:      row.RoundNumber,
		IsOverride:       row.ActionType == "dm_override",
	}
	if row.TargetID.Valid {
		tid := row.TargetID.UUID
		entry.TargetID = &tid
		targetInfo := infoByID[tid]
		entry.TargetDisplayName = targetInfo.DisplayName
		entry.TargetShortID = targetInfo.ShortID
	}
	if row.Description.Valid {
		entry.Description = row.Description.String
	}
	if row.DiceRolls.Valid {
		entry.DiceRolls = row.DiceRolls.RawMessage
	}
	return entry
}

// ListActionLogViewer handles GET /api/combat/{encounterID}/action-log.
func (h *DMDashboardHandler) ListActionLogViewer(w http.ResponseWriter, r *http.Request) {
	encounterID, err := parseEncounterID(r)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	filter, err := parseActionLogFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entries, err := h.svc.ListActionLogForViewer(r.Context(), encounterID, filter)
	if err != nil {
		http.Error(w, "failed to list action log", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, entries)
}

// parseActionLogFilter extracts filter parameters from query string.
func parseActionLogFilter(r *http.Request) (ActionLogFilter, error) {
	q := r.URL.Query()
	filter := ActionLogFilter{
		ActionTypes: parseActionTypes(q["action_type"]),
	}

	actorID, err := parseUUIDQuery(q, "actor_id")
	if err != nil {
		return filter, err
	}
	filter.ActorID = actorID

	targetID, err := parseUUIDQuery(q, "target_id")
	if err != nil {
		return filter, err
	}
	filter.TargetID = targetID

	turnID, err := parseUUIDQuery(q, "turn_id")
	if err != nil {
		return filter, err
	}
	filter.TurnID = turnID

	if v := q.Get("round"); v != "" {
		round, err := strconv.Atoi(v)
		if err != nil {
			return filter, fmt.Errorf("invalid round")
		}
		filter.Round = int32(round)
	}

	filter.SortAsc = q.Get("sort") == "asc"
	return filter, nil
}

// parseUUIDQuery returns the parsed UUID for the given query field, or uuid.Nil if absent.
func parseUUIDQuery(q url.Values, field string) (uuid.UUID, error) {
	v := q.Get(field)
	if v == "" {
		return uuid.Nil, nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s", field)
	}
	return id, nil
}

// parseActionTypes flattens repeated + comma-separated action_type query values.
func parseActionTypes(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			out = append(out, p)
		}
	}
	return out
}

