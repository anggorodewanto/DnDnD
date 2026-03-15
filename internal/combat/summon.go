package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ErrNotSummoner is returned when a player tries to command a creature they didn't summon.
var ErrNotSummoner = errors.New("you can only command creatures you summoned")

// ErrNotSummoned is returned when trying to command a combatant that is not a summoned creature.
var ErrNotSummoned = errors.New("this combatant is not a summoned creature")

// SummonCreatureInput holds parameters for summoning a creature into combat.
type SummonCreatureInput struct {
	EncounterID   uuid.UUID
	SummonerID    uuid.UUID
	CreatureRefID string
	ShortID       string
	DisplayName   string
	PositionCol   string
	PositionRow   int32
}

// SummonCreature creates a summoned creature combatant linked to the summoner.
func (s *Service) SummonCreature(ctx context.Context, input SummonCreatureInput) (refdata.Combatant, error) {
	creature, err := s.store.GetCreature(ctx, input.CreatureRefID)
	if err != nil {
		return refdata.Combatant{}, fmt.Errorf("looking up creature %q: %w", input.CreatureRefID, err)
	}

	combatant, err := s.store.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: input.EncounterID,
		CreatureRefID: sql.NullString{String: input.CreatureRefID, Valid: true},
		ShortID:     input.ShortID,
		DisplayName: input.DisplayName,
		PositionCol: input.PositionCol,
		PositionRow: input.PositionRow,
		HpMax:       creature.HpAverage,
		HpCurrent:   creature.HpAverage,
		Ac:          creature.Ac,
		Conditions:  json.RawMessage(`[]`),
		IsNpc:       true,
		IsAlive:     true,
		IsVisible:   true,
		SummonerID:  uuid.NullUUID{UUID: input.SummonerID, Valid: true},
	})
	if err != nil {
		return refdata.Combatant{}, fmt.Errorf("creating summoned combatant: %w", err)
	}

	return combatant, nil
}

// DismissResult holds the outcome of dismissing a summoned creature.
type DismissResult struct {
	ShortID     string
	DisplayName string
}

// DismissSummon removes a summoned creature from the encounter.
// Validates that the caller is the summoner.
func (s *Service) DismissSummon(ctx context.Context, creatureID, summonerID uuid.UUID) (DismissResult, error) {
	creature, err := s.store.GetCombatant(ctx, creatureID)
	if err != nil {
		return DismissResult{}, fmt.Errorf("getting summoned creature: %w", err)
	}

	if err := ValidateCommandOwnership(creature, summonerID); err != nil {
		return DismissResult{}, err
	}

	if err := s.store.DeleteCombatant(ctx, creatureID); err != nil {
		return DismissResult{}, fmt.Errorf("removing summoned creature: %w", err)
	}

	return DismissResult{
		ShortID:     creature.ShortID,
		DisplayName: creature.DisplayName,
	}, nil
}

// SummonMultipleInput holds parameters for summoning multiple creatures at once.
type SummonMultipleInput struct {
	EncounterID     uuid.UUID
	SummonerID      uuid.UUID
	CreatureRefID   string
	BaseShortID     string
	BaseDisplayName string
	Quantity        int
	PositionCol     string
	PositionRow     int32
}

// SummonMultipleCreatures summons multiple instances of a creature (e.g., Conjure Animals).
// Each gets a numbered short ID (WF1, WF2, ...) and display name.
func (s *Service) SummonMultipleCreatures(ctx context.Context, input SummonMultipleInput) ([]refdata.Combatant, error) {
	results := make([]refdata.Combatant, 0, input.Quantity)
	for i := 1; i <= input.Quantity; i++ {
		shortID := fmt.Sprintf("%s%d", input.BaseShortID, i)
		displayName := fmt.Sprintf("%s #%d", input.BaseDisplayName, i)

		c, err := s.SummonCreature(ctx, SummonCreatureInput{
			EncounterID:   input.EncounterID,
			SummonerID:    input.SummonerID,
			CreatureRefID: input.CreatureRefID,
			ShortID:       shortID,
			DisplayName:   displayName,
			PositionCol:   input.PositionCol,
			PositionRow:   input.PositionRow,
		})
		if err != nil {
			return nil, fmt.Errorf("summoning creature %d: %w", i, err)
		}
		results = append(results, c)
	}
	return results, nil
}

// DismissSummonsByConcentration removes all summoned creatures belonging to the
// given summoner from the encounter. Called when the summoner's concentration breaks.
// Returns the number of creatures removed.
func (s *Service) DismissSummonsByConcentration(ctx context.Context, encounterID, summonerID uuid.UUID) (int, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return 0, fmt.Errorf("listing combatants for concentration dismissal: %w", err)
	}

	removed := 0
	for _, c := range combatants {
		if !c.SummonerID.Valid || c.SummonerID.UUID != summonerID {
			continue
		}
		if err := s.store.DeleteCombatant(ctx, c.ID); err != nil {
			return removed, fmt.Errorf("removing summoned creature %s: %w", c.ShortID, err)
		}
		removed++
	}
	return removed, nil
}

// HandleSummonDeath checks if a combatant is a summoned creature at 0 HP and removes it.
// Summoned creatures are removed immediately on death (no death saves).
// Returns true if the creature was removed, false if not applicable.
func (s *Service) HandleSummonDeath(ctx context.Context, creature refdata.Combatant) (bool, error) {
	if !creature.SummonerID.Valid {
		return false, nil
	}
	if creature.IsAlive || creature.HpCurrent > 0 {
		return false, nil
	}

	if err := s.store.DeleteCombatant(ctx, creature.ID); err != nil {
		return false, fmt.Errorf("removing dead summoned creature: %w", err)
	}
	return true, nil
}

// CommandCreatureInput holds the inputs for /command.
type CommandCreatureInput struct {
	EncounterID     uuid.UUID
	SummonerID      uuid.UUID // the combatant ID of the summoning player
	SummonerName    string    // display name of the summoner
	CreatureShortID string
	Action          string
	Args            []string
}

// CommandCreatureResult holds the outcome of a /command action.
type CommandCreatureResult struct {
	Action    string
	CombatLog string
}

// CommandCreature executes a /command action on a summoned creature.
func (s *Service) CommandCreature(ctx context.Context, input CommandCreatureInput) (CommandCreatureResult, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, input.EncounterID)
	if err != nil {
		return CommandCreatureResult{}, fmt.Errorf("listing combatants: %w", err)
	}

	creature, err := FindCombatantByShortID(combatants, input.CreatureShortID)
	if err != nil {
		return CommandCreatureResult{}, err
	}

	if err := ValidateCommandOwnership(creature, input.SummonerID); err != nil {
		return CommandCreatureResult{}, err
	}

	switch input.Action {
	case "dismiss":
		if err := s.store.DeleteCombatant(ctx, creature.ID); err != nil {
			return CommandCreatureResult{}, fmt.Errorf("dismissing creature: %w", err)
		}
		s.summonedResources.Remove(creature.ID)
		return CommandCreatureResult{
			Action:    "dismiss",
			CombatLog: FormatSummonDismissLog(input.SummonerName, creature.DisplayName, creature.ShortID),
		}, nil

	case "done":
		s.summonedResources.MarkDone(creature.ID)
		return CommandCreatureResult{
			Action:    "done",
			CombatLog: fmt.Sprintf("%s (%s) ends their turn", creature.DisplayName, creature.ShortID),
		}, nil

	case "move":
		s.summonedResources.UseMovement(creature.ID)
		return CommandCreatureResult{
			Action:    input.Action,
			CombatLog: fmt.Sprintf("%s (%s) moves", creature.DisplayName, creature.ShortID),
		}, nil

	default:
		s.summonedResources.UseAction(creature.ID)
		return CommandCreatureResult{
			Action:    input.Action,
			CombatLog: fmt.Sprintf("%s (%s) uses %s", creature.DisplayName, creature.ShortID, input.Action),
		}, nil
	}
}

// IsSummonedCreature returns true if the combatant is a summoned creature.
func IsSummonedCreature(c refdata.Combatant) bool {
	return c.SummonerID.Valid
}

// ListSummonedCreatures returns all combatants summoned by the given summoner.
func ListSummonedCreatures(combatants []refdata.Combatant, summonerID uuid.UUID) []refdata.Combatant {
	var result []refdata.Combatant
	for _, c := range combatants {
		if c.SummonerID.Valid && c.SummonerID.UUID == summonerID {
			result = append(result, c)
		}
	}
	return result
}

// FindCombatantByShortID finds a combatant by short ID from a list.
func FindCombatantByShortID(combatants []refdata.Combatant, shortID string) (refdata.Combatant, error) {
	upper := strings.ToUpper(shortID)
	for _, c := range combatants {
		if strings.ToUpper(c.ShortID) == upper {
			return c, nil
		}
	}
	return refdata.Combatant{}, fmt.Errorf("no creature found with ID %q", shortID)
}

// CommandArgs holds parsed /command arguments.
type CommandArgs struct {
	CreatureShortID string
	Action          string
	Args            []string
}

// ParseCommandArgs parses the raw argument string from /command.
// Expected format: "[creature-id] [action] [args...]"
func ParseCommandArgs(raw string) (CommandArgs, error) {
	parts := strings.Fields(raw)
	if len(parts) < 2 {
		return CommandArgs{}, errors.New("usage: /command [creature-id] [action] [target?]")
	}

	cmd := CommandArgs{
		CreatureShortID: parts[0],
		Action:          strings.ToLower(parts[1]),
	}
	if len(parts) > 2 {
		cmd.Args = parts[2:]
	}
	return cmd, nil
}

// FormatSummonTurnNotification produces the ping message for a summoned creature's turn.
func FormatSummonTurnNotification(playerName, creatureName, shortID string) string {
	return fmt.Sprintf("\U0001f514 @%s \u2014 your %s (%s)'s turn!", playerName, creatureName, shortID)
}

// FormatSummonDismissLog produces the combat log for voluntary dismissal.
func FormatSummonDismissLog(summonerName, creatureName, shortID string) string {
	return fmt.Sprintf("\U0001f4a8 %s dismisses %s (%s)", summonerName, creatureName, shortID)
}

// FormatSummonDeathLog produces the combat log for a summoned creature dying.
func FormatSummonDeathLog(creatureName, shortID string) string {
	return fmt.Sprintf("\U0001f480 %s (%s) is destroyed", creatureName, shortID)
}

// FormatSummonLog produces the combat log for a creature being summoned.
func FormatSummonLog(summonerName, creatureName, shortID, position string) string {
	return fmt.Sprintf("\U0001f43e %s summons %s (%s) at %s", summonerName, creatureName, shortID, position)
}

// ValidateCommandOwnership checks that the given combatant is a summoned creature
// and that the caller is the summoner.
func ValidateCommandOwnership(creature refdata.Combatant, callerCombatantID uuid.UUID) error {
	if !creature.SummonerID.Valid {
		return ErrNotSummoned
	}
	if creature.SummonerID.UUID != callerCombatantID {
		return ErrNotSummoner
	}
	return nil
}

// resetSummonedCreatureResources resets turn resources for all summoned creatures
// belonging to the given summoner. Called at the start of the summoner's turn.
func (s *Service) resetSummonedCreatureResources(ctx context.Context, encounterID, summonerID uuid.UUID) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return // best-effort
	}
	for _, c := range combatants {
		if c.SummonerID.Valid && c.SummonerID.UUID == summonerID {
			s.summonedResources.Reset(c.ID)
		}
	}
}

// --- Summoned Creature Turn Resources ---

// summonedCreatureResources tracks action/movement/bonus for one summoned creature.
type summonedCreatureResources struct {
	ActionUsed      bool
	MovementUsed    bool
	BonusActionUsed bool
	Done            bool
}

// SummonedTurnResources tracks per-creature turn resources for all summoned creatures.
// Keyed by creature combatant ID.
type SummonedTurnResources struct {
	creatures map[uuid.UUID]*summonedCreatureResources
}

// NewSummonedTurnResources creates a new SummonedTurnResources tracker.
func NewSummonedTurnResources() *SummonedTurnResources {
	return &SummonedTurnResources{
		creatures: make(map[uuid.UUID]*summonedCreatureResources),
	}
}

// getOrInit returns existing resources or creates fresh ones.
func (s *SummonedTurnResources) getOrInit(creatureID uuid.UUID) *summonedCreatureResources {
	if r, ok := s.creatures[creatureID]; ok {
		return r
	}
	r := &summonedCreatureResources{}
	s.creatures[creatureID] = r
	return r
}

// Reset resets all turn resources for a creature (called at start of summoner's turn).
func (s *SummonedTurnResources) Reset(creatureID uuid.UUID) {
	s.creatures[creatureID] = &summonedCreatureResources{}
}

// Remove removes a creature's resources (called on dismissal/death).
func (s *SummonedTurnResources) Remove(creatureID uuid.UUID) {
	delete(s.creatures, creatureID)
}

// HasAction returns true if the creature's action is still available.
func (s *SummonedTurnResources) HasAction(creatureID uuid.UUID) bool {
	return !s.getOrInit(creatureID).ActionUsed
}

// HasMovement returns true if the creature's movement is still available.
func (s *SummonedTurnResources) HasMovement(creatureID uuid.UUID) bool {
	return !s.getOrInit(creatureID).MovementUsed
}

// HasBonusAction returns true if the creature's bonus action is still available.
func (s *SummonedTurnResources) HasBonusAction(creatureID uuid.UUID) bool {
	return !s.getOrInit(creatureID).BonusActionUsed
}

// IsDone returns true if the creature has been marked done for this turn.
func (s *SummonedTurnResources) IsDone(creatureID uuid.UUID) bool {
	return s.getOrInit(creatureID).Done
}

// UseAction marks the creature's action as used.
func (s *SummonedTurnResources) UseAction(creatureID uuid.UUID) {
	s.getOrInit(creatureID).ActionUsed = true
}

// UseMovement marks the creature's movement as used.
func (s *SummonedTurnResources) UseMovement(creatureID uuid.UUID) {
	s.getOrInit(creatureID).MovementUsed = true
}

// UseBonusAction marks the creature's bonus action as used.
func (s *SummonedTurnResources) UseBonusAction(creatureID uuid.UUID) {
	s.getOrInit(creatureID).BonusActionUsed = true
}

// MarkDone marks the creature as done for this turn.
func (s *SummonedTurnResources) MarkDone(creatureID uuid.UUID) {
	s.getOrInit(creatureID).Done = true
}

// FormatCreatureResources returns a display string of available resources for a creature.
func (s *SummonedTurnResources) FormatCreatureResources(creatureID uuid.UUID, displayName, shortID string) string {
	r := s.getOrInit(creatureID)
	if r.Done {
		return fmt.Sprintf("%s (%s): done", displayName, shortID)
	}

	var parts []string
	if !r.ActionUsed {
		parts = append(parts, "action")
	}
	if !r.MovementUsed {
		parts = append(parts, "movement")
	}
	if !r.BonusActionUsed {
		parts = append(parts, "bonus")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%s (%s): all resources spent", displayName, shortID)
	}
	return fmt.Sprintf("%s (%s): %s", displayName, shortID, strings.Join(parts, ", "))
}

// FormatSummonedCreaturesPrompt returns a prompt section listing summoned creatures
// and their available resources. Included in the summoner's turn start notification.
func FormatSummonedCreaturesPrompt(creatures []refdata.Combatant, resources *SummonedTurnResources) string {
	if len(creatures) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\U0001f43e **Summoned Creatures** (use /command):\n")
	for _, c := range creatures {
		b.WriteString("  \u2022 ")
		b.WriteString(resources.FormatCreatureResources(c.ID, c.DisplayName, c.ShortID))
		b.WriteString("\n")
	}
	return b.String()
}

// --- HTTP Handlers ---

// commandCreatureRequest is the JSON request body for /command.
type commandCreatureRequest struct {
	SummonerID      string `json:"summoner_id"`
	SummonerName    string `json:"summoner_name"`
	CreatureShortID string `json:"creature_short_id"`
	Action          string `json:"action"`
	Args            []string `json:"args,omitempty"`
}

// commandCreatureResponse is the JSON response for /command.
type commandCreatureResponse struct {
	Action    string `json:"action"`
	CombatLog string `json:"combat_log"`
}

// CommandCreatureHandler handles POST /api/combat/{encounterID}/command.
func (h *Handler) CommandCreatureHandler(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	var req commandCreatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	summonerID, err := uuid.Parse(req.SummonerID)
	if err != nil {
		http.Error(w, "invalid summoner_id", http.StatusBadRequest)
		return
	}

	result, err := h.svc.CommandCreature(r.Context(), CommandCreatureInput{
		EncounterID:     encounterID,
		SummonerID:      summonerID,
		SummonerName:    req.SummonerName,
		CreatureShortID: req.CreatureShortID,
		Action:          req.Action,
		Args:            req.Args,
	})
	if err != nil {
		if errors.Is(err, ErrNotSummoner) || errors.Is(err, ErrNotSummoned) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, commandCreatureResponse{
		Action:    result.Action,
		CombatLog: result.CombatLog,
	})
}

// summonCreatureRequest is the JSON request body for /summon.
type summonCreatureRequest struct {
	SummonerID    string `json:"summoner_id"`
	SummonerName  string `json:"summoner_name"`
	CreatureRefID string `json:"creature_ref_id"`
	ShortID       string `json:"short_id"`
	DisplayName   string `json:"display_name"`
	PositionCol   string `json:"position_col"`
	PositionRow   int32  `json:"position_row"`
}

// SummonCreatureHandler handles POST /api/combat/{encounterID}/summon.
func (h *Handler) SummonCreatureHandler(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	var req summonCreatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	summonerID, err := uuid.Parse(req.SummonerID)
	if err != nil {
		http.Error(w, "invalid summoner_id", http.StatusBadRequest)
		return
	}

	combatant, err := h.svc.SummonCreature(r.Context(), SummonCreatureInput{
		EncounterID:   encounterID,
		SummonerID:    summonerID,
		CreatureRefID: req.CreatureRefID,
		ShortID:       req.ShortID,
		DisplayName:   req.DisplayName,
		PositionCol:   req.PositionCol,
		PositionRow:   req.PositionRow,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, combatantResponse{
		ID:          combatant.ID.String(),
		ShortID:     combatant.ShortID,
		DisplayName: combatant.DisplayName,
		HpMax:       combatant.HpMax,
		HpCurrent:   combatant.HpCurrent,
		Ac:          combatant.Ac,
		IsNpc:       combatant.IsNpc,
		IsAlive:     combatant.IsAlive,
	})
}
