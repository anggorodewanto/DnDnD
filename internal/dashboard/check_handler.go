package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/check"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// CheckParticipantStore is the narrow surface the dashboard check handler
// needs to materialise group-check / DM-prompted-check participants. Wrap
// refdata.Queries in production; tests provide a stub so the handler is
// unit-testable. (F-81-group-check-handler / F-81-dm-prompted-checks)
type CheckParticipantStore interface {
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	UpsertPendingCheck(ctx context.Context, arg refdata.UpsertPendingCheckParams) (refdata.PendingCheck, error)
}

// CheckHandler exposes DM-initiated check flows from the dashboard:
//   - POST /api/encounters/{encounterID}/group-check — roll a /group/ check
//     against a shared DC across every alive PC combatant in the encounter
//     and return the aggregated outcome (F-81-group-check-handler).
//   - POST /api/encounters/{encounterID}/prompt-check — persist a
//     pending_check row asking a specific PC to roll a skill/ability check.
//     Player resolution still flows through /check at the bot side; this
//     route is the DM-prompt persistence layer analogous to pending_saves
//     for /save (F-81-dm-prompted-checks).
type CheckHandler struct {
	store  CheckParticipantStore
	roller *dice.Roller
}

// NewCheckHandler creates a dashboard CheckHandler. The roller may be nil
// at construction; tests typically inject a deterministic roller.
func NewCheckHandler(store CheckParticipantStore, roller *dice.Roller) *CheckHandler {
	if roller == nil {
		// Default to the standard random-source roller. Tests pass a
		// deterministic roller to assert d20 outputs.
		roller = dice.NewRoller(nil)
	}
	return &CheckHandler{store: store, roller: roller}
}

// GroupCheckRequest is the JSON body for POST .../group-check.
type GroupCheckRequest struct {
	Skill string `json:"skill"`
	DC    int    `json:"dc"`
}

// GroupCheckParticipantResult mirrors check.GroupParticipantResult in JSON form.
type GroupCheckParticipantResult struct {
	Name    string `json:"name"`
	Total   int    `json:"total"`
	Natural int    `json:"natural"`
	Passed  bool   `json:"passed"`
}

// GroupCheckResponse is the JSON response for the group-check endpoint.
type GroupCheckResponse struct {
	Skill   string                        `json:"skill"`
	DC      int                           `json:"dc"`
	Passed  int                           `json:"passed"`
	Failed  int                           `json:"failed"`
	Success bool                          `json:"success"`
	Results []GroupCheckParticipantResult `json:"results"`
}

// HandleGroupCheck dispatches a group check across all alive PC
// combatants in the encounter and returns the aggregated pass/fail
// outcome (F-81-group-check-handler). Mirrors the bot-side single-check
// resolver but for the DM-driven multi-participant case the spec calls
// out.
func (h *CheckHandler) HandleGroupCheck(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		writeCheckError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}
	var req GroupCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCheckError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Skill == "" {
		writeCheckError(w, "skill is required", http.StatusBadRequest)
		return
	}

	participants, err := h.collectGroupParticipants(r.Context(), encounterID, req.Skill)
	if err != nil {
		writeCheckError(w, fmt.Sprintf("failed to list combatants: %v", err), http.StatusInternalServerError)
		return
	}
	if len(participants) == 0 {
		writeCheckError(w, "no eligible participants in encounter", http.StatusBadRequest)
		return
	}

	svc := check.NewService(h.roller)
	res := svc.GroupCheck(check.GroupCheckInput{DC: req.DC, Participants: participants})

	resp := GroupCheckResponse{
		Skill:   req.Skill,
		DC:      res.DC,
		Passed:  res.Passed,
		Failed:  res.Failed,
		Success: res.Success,
	}
	for _, p := range res.Results {
		resp.Results = append(resp.Results, GroupCheckParticipantResult{
			Name:    p.Name,
			Total:   p.D20.Total,
			Natural: p.D20.Chosen,
			Passed:  p.Passed,
		})
	}
	writeCheckJSON(w, resp)
}

// collectGroupParticipants materialises check.GroupParticipant entries from
// every alive PC combatant in the encounter, computing the appropriate
// skill modifier per character.
func (h *CheckHandler) collectGroupParticipants(ctx context.Context, encounterID uuid.UUID, skill string) ([]check.GroupParticipant, error) {
	combs, err := h.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, err
	}
	out := make([]check.GroupParticipant, 0, len(combs))
	for _, c := range combs {
		if c.IsNpc || !c.IsAlive || !c.CharacterID.Valid {
			continue
		}
		char, err := h.store.GetCharacter(ctx, c.CharacterID.UUID)
		if err != nil {
			continue
		}
		mod, ok := participantSkillModifier(char, skill)
		if !ok {
			continue
		}
		name := c.DisplayName
		if name == "" {
			name = char.Name
		}
		out = append(out, check.GroupParticipant{Name: name, Modifier: mod})
	}
	return out, nil
}

// PromptCheckRequest is the JSON body for POST .../prompt-check.
type PromptCheckRequest struct {
	CombatantID uuid.UUID `json:"combatant_id"`
	Skill       string    `json:"skill"`
	DC          int       `json:"dc"`
	Reason      string    `json:"reason,omitempty"`
}

// PromptCheckResponse is the JSON returned to the DM after a successful
// prompt-check creation.
type PromptCheckResponse struct {
	ID          uuid.UUID `json:"id"`
	CombatantID uuid.UUID `json:"combatant_id"`
	Skill       string    `json:"skill"`
	DC          int       `json:"dc"`
	Status      string    `json:"status"`
}

// HandlePromptCheck persists a pending_check row asking the named
// combatant for a skill/ability check. The player resolves the prompt
// through the normal /check slash command at the bot side; this route is
// the DM-prompt persistence layer (F-81-dm-prompted-checks).
func (h *CheckHandler) HandlePromptCheck(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		writeCheckError(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}
	var req PromptCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCheckError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Skill == "" {
		writeCheckError(w, "skill is required", http.StatusBadRequest)
		return
	}
	if req.CombatantID == uuid.Nil {
		writeCheckError(w, "combatant_id is required", http.StatusBadRequest)
		return
	}
	saved, err := h.store.UpsertPendingCheck(r.Context(), refdata.UpsertPendingCheckParams{
		EncounterID: encounterID,
		CombatantID: req.CombatantID,
		Skill:       req.Skill,
		Dc:          int32(req.DC),
		Reason:      nullString(req.Reason),
	})
	if err != nil {
		writeCheckError(w, fmt.Sprintf("failed to persist pending check: %v", err), http.StatusInternalServerError)
		return
	}
	writeCheckJSON(w, PromptCheckResponse{
		ID:          saved.ID,
		CombatantID: saved.CombatantID,
		Skill:       saved.Skill,
		DC:          int(saved.Dc),
		Status:      saved.Status,
	})
}

// RegisterCheckRoutes mounts the dashboard check endpoints on the given
// router. F-81 group + DM-prompt routes share one handler.
func RegisterCheckRoutes(r chi.Router, h *CheckHandler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/encounters/{encounterID}", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Post("/group-check", h.HandleGroupCheck)
		r.Post("/prompt-check", h.HandlePromptCheck)
	})
}

func writeCheckError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeCheckJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// participantSkillModifier returns the skill modifier for a PC. ok=false
// when the character's stat blob can't be parsed (defensive: a malformed
// row drops the participant rather than crashing the rollup).
func participantSkillModifier(char refdata.Character, skill string) (int, bool) {
	var scores character.AbilityScores
	if err := json.Unmarshal(char.AbilityScores, &scores); err != nil {
		return 0, false
	}
	var profs struct {
		Skills          []string `json:"skills"`
		Expertise       []string `json:"expertise"`
		JackOfAllTrades bool     `json:"jack_of_all_trades"`
	}
	if char.Proficiencies.Valid && len(char.Proficiencies.RawMessage) > 0 {
		_ = json.Unmarshal(char.Proficiencies.RawMessage, &profs)
	}
	// Raw ability check (e.g. "str", "wis"): use the ability modifier.
	if _, isSkill := character.SkillAbilityMap[skill]; !isSkill {
		// when key is one of the 6 abilities, AbilityScores.Get handles it
		// and AbilityModifier returns the modifier.
		return character.AbilityModifier(scores.Get(skill)), true
	}
	return character.SkillModifier(scores, skill, profs.Skills, profs.Expertise, profs.JackOfAllTrades, int(char.ProficiencyBonus)), true
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
