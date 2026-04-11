package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// AbilityModifier returns the D&D 5e ability modifier for a given ability score.
// The formula is floor((score - 10) / 2). Go's integer division truncates toward
// zero, so we must adjust for negative odd differences.
func AbilityModifier(score int) int {
	diff := score - 10
	if diff >= 0 {
		return diff / 2
	}
	return (diff - 1) / 2
}

// AbilityScores represents the six core D&D ability scores.
type AbilityScores struct {
	Str int `json:"str"`
	Dex int `json:"dex"`
	Con int `json:"con"`
	Int int `json:"int"`
	Wis int `json:"wis"`
	Cha int `json:"cha"`
}

// ScoreByName returns the ability score for a given abbreviation (e.g. "str", "int").
// Returns 0 for unrecognized names.
func (s AbilityScores) ScoreByName(ability string) int {
	switch strings.ToLower(ability) {
	case "str":
		return s.Str
	case "dex":
		return s.Dex
	case "con":
		return s.Con
	case "int":
		return s.Int
	case "wis":
		return s.Wis
	case "cha":
		return s.Cha
	default:
		return 0
	}
}

// ParseAbilityScores parses a JSONB ability_scores column into an AbilityScores struct.
func ParseAbilityScores(raw json.RawMessage) (AbilityScores, error) {
	var scores AbilityScores
	if err := json.Unmarshal(raw, &scores); err != nil {
		return AbilityScores{}, err
	}
	return scores, nil
}

// InitiativeEntry holds a combatant's initiative data for sorting.
type InitiativeEntry struct {
	CombatantID uuid.UUID
	DisplayName string
	Roll        int
	DexMod      int
}

// CombatCondition represents a condition applied to a combatant.
type CombatCondition struct {
	Condition         string `json:"condition"`
	DurationRounds    int    `json:"duration_rounds"`
	StartedRound      int    `json:"started_round"`
	SourceCombatantID string `json:"source_combatant_id,omitempty"`
	ExpiresOn         string `json:"expires_on,omitempty"`
}

// SurprisedCondition returns the standard surprised condition struct.
func SurprisedCondition() CombatCondition {
	return CombatCondition{
		Condition:      "surprised",
		DurationRounds: 1,
		StartedRound:   0,
	}
}

// IsSurprised checks whether a combatant's conditions JSONB contains the "surprised" condition.
func IsSurprised(conditions json.RawMessage) bool {
	return HasCondition(conditions, "surprised")
}

// parseConditions unmarshals a conditions JSONB array, treating empty/nil as an empty slice.
func parseConditions(raw json.RawMessage) ([]CombatCondition, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var conds []CombatCondition
	if err := json.Unmarshal(raw, &conds); err != nil {
		return nil, err
	}
	return conds, nil
}

// AddSurprisedCondition adds the surprised condition to an existing conditions JSONB array.
func AddSurprisedCondition(conditions json.RawMessage) (json.RawMessage, error) {
	return AddCondition(conditions, SurprisedCondition())
}

// combatOnlyConditions is the set of conditions that are cleared when combat ends.
var combatOnlyConditions = map[string]bool{
	"stunned":       true,
	"frightened":    true,
	"charmed":       true,
	"restrained":    true,
	"grappled":      true,
	"prone":         true,
	"incapacitated": true,
	"paralyzed":     true,
	"blinded":       true,
	"deafened":      true,
	"surprised":     true,
	"dodge":         true,
}

// ClearCombatConditions removes combat-only conditions (stunned, frightened, charmed,
// restrained, grappled, prone, incapacitated, paralyzed, blinded, deafened, surprised)
// while preserving non-combat conditions like exhaustion, curse, disease.
func ClearCombatConditions(conditions json.RawMessage) (json.RawMessage, error) {
	conds, err := parseConditions(conditions)
	if err != nil {
		return nil, err
	}
	filtered := make([]CombatCondition, 0, len(conds))
	for _, c := range conds {
		if !combatOnlyConditions[c.Condition] {
			filtered = append(filtered, c)
		}
	}
	return json.Marshal(filtered)
}

// RemoveSurprisedCondition removes the surprised condition from a conditions JSONB array.
func RemoveSurprisedCondition(conditions json.RawMessage) (json.RawMessage, error) {
	return RemoveCondition(conditions, "surprised")
}

// SortByInitiative sorts entries by: roll DESC, DEX modifier DESC, display name ASC.
func SortByInitiative(entries []InitiativeEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Roll != entries[j].Roll {
			return entries[i].Roll > entries[j].Roll
		}
		if entries[i].DexMod != entries[j].DexMod {
			return entries[i].DexMod > entries[j].DexMod
		}
		return entries[i].DisplayName < entries[j].DisplayName
	})
}

// EncounterDisplayName returns the display name if set, otherwise the slug name.
func EncounterDisplayName(encounter refdata.Encounter) string {
	if encounter.DisplayName.Valid && encounter.DisplayName.String != "" {
		return encounter.DisplayName.String
	}
	return encounter.Name
}

// FormatEncounterLabel produces the Phase 105 prefix line used at the top of
// bot messages posted into shared combat channels, so players can tell which
// simultaneous encounter the message belongs to.
//
// Example output: "\u2694\ufe0f Rooftop Ambush \u2014 Round 3"
func FormatEncounterLabel(encounterDisplayName string, roundNumber int32) string {
	return fmt.Sprintf("\u2694\ufe0f %s \u2014 Round %d", encounterDisplayName, roundNumber)
}

// EncounterLabel is the convenience form of FormatEncounterLabel that pulls
// display name and round number straight off a refdata.Encounter.
func EncounterLabel(enc refdata.Encounter) string {
	return FormatEncounterLabel(EncounterDisplayName(enc), enc.RoundNumber)
}

// formatCombatantLine formats a single combatant line: NPCs show name only, PCs show HP.
func formatCombatantLine(c refdata.Combatant) string {
	if c.IsNpc {
		return fmt.Sprintf("  %s", c.DisplayName)
	}
	return fmt.Sprintf("  %s (%d/%d HP)", c.DisplayName, c.HpCurrent, c.HpMax)
}

// FormatInitiativeTracker produces the Discord message for the initiative tracker.
func FormatInitiativeTracker(encounter refdata.Encounter, combatants []refdata.Combatant, currentTurnCombatantID uuid.UUID) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\u2694\ufe0f %s \u2014 Round %d\n", EncounterDisplayName(encounter), encounter.RoundNumber)

	for _, c := range combatants {
		if c.ID == currentTurnCombatantID {
			fmt.Fprintf(&b, "\U0001f514 @%s \u2014 it's your turn!\n", c.DisplayName)
			continue
		}
		fmt.Fprintf(&b, "%s\n", formatCombatantLine(c))
	}

	return strings.TrimRight(b.String(), "\n")
}

// FormatCompletedInitiativeTracker produces the initiative tracker for a completed encounter.
// No active turn indicator is shown and a "--- Combat Complete ---" footer is appended.
func FormatCompletedInitiativeTracker(encounter refdata.Encounter, combatants []refdata.Combatant) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\u2694\ufe0f %s \u2014 Round %d\n", EncounterDisplayName(encounter), encounter.RoundNumber)

	for _, c := range combatants {
		fmt.Fprintf(&b, "%s\n", formatCombatantLine(c))
	}

	b.WriteString("--- Combat Complete ---")
	return b.String()
}

// dexModFromScores parses ability scores JSON and returns the DEX modifier.
func dexModFromScores(raw json.RawMessage, label string) (int, error) {
	scores, err := ParseAbilityScores(raw)
	if err != nil {
		return 0, fmt.Errorf("parsing %s ability scores: %w", label, err)
	}
	return AbilityModifier(scores.Dex), nil
}

// getDexModifier returns the DEX modifier for a combatant by looking up
// the character or creature ability scores.
func (s *Service) getDexModifier(ctx context.Context, c refdata.Combatant) (int, error) {
	if c.CharacterID.Valid {
		char, err := s.store.GetCharacter(ctx, c.CharacterID.UUID)
		if err != nil {
			return 0, fmt.Errorf("getting character %s: %w", c.CharacterID.UUID, err)
		}
		return dexModFromScores(char.AbilityScores, "character")
	}
	if c.CreatureRefID.Valid {
		creature, err := s.store.GetCreature(ctx, c.CreatureRefID.String)
		if err != nil {
			return 0, fmt.Errorf("getting creature %s: %w", c.CreatureRefID.String, err)
		}
		return dexModFromScores(creature.AbilityScores, "creature")
	}
	return 0, nil
}

// RollInitiative rolls initiative for all combatants in an encounter, sorts them,
// assigns initiative_order, sets round to 1 and status to "active".
func (s *Service) RollInitiative(ctx context.Context, encounterID uuid.UUID, roller *dice.Roller) ([]refdata.Combatant, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("listing combatants: %w", err)
	}
	if len(combatants) == 0 {
		return nil, errors.New("no combatants in encounter")
	}

	// Filter out summoned creatures — they share their summoner's turn.
	var rollable []refdata.Combatant
	for _, c := range combatants {
		if !IsSummonedCreature(c) {
			rollable = append(rollable, c)
		}
	}
	if len(rollable) == 0 {
		return nil, errors.New("no combatants in encounter")
	}

	entries := make([]InitiativeEntry, len(rollable))
	for i, c := range rollable {
		dexMod, err := s.getDexModifier(ctx, c)
		if err != nil {
			return nil, err
		}
		result, err := roller.RollD20(dexMod, dice.Normal)
		if err != nil {
			return nil, fmt.Errorf("rolling initiative for %s: %w", c.DisplayName, err)
		}
		entries[i] = InitiativeEntry{
			CombatantID: c.ID,
			DisplayName: c.DisplayName,
			Roll:        result.Total,
			DexMod:      dexMod,
		}
	}

	SortByInitiative(entries)

	result := make([]refdata.Combatant, len(entries))
	for i, e := range entries {
		updated, err := s.store.UpdateCombatantInitiative(ctx, refdata.UpdateCombatantInitiativeParams{
			ID:              e.CombatantID,
			InitiativeRoll:  int32(e.Roll),
			InitiativeOrder: int32(i + 1),
		})
		if err != nil {
			return nil, fmt.Errorf("updating initiative for %s: %w", e.DisplayName, err)
		}
		result[i] = updated
	}

	if _, err := s.store.UpdateEncounterRound(ctx, refdata.UpdateEncounterRoundParams{
		ID:          encounterID,
		RoundNumber: 1,
	}); err != nil {
		return nil, fmt.Errorf("setting round to 1: %w", err)
	}
	if _, err := s.store.UpdateEncounterStatus(ctx, refdata.UpdateEncounterStatusParams{
		ID:     encounterID,
		Status: "active",
	}); err != nil {
		return nil, fmt.Errorf("setting status to active: %w", err)
	}

	return result, nil
}

// SkippedInfo holds details about a combatant whose turn was auto-skipped.
type SkippedInfo struct {
	CombatantID   uuid.UUID
	DisplayName   string
	ConditionName string
}

// TurnInfo holds information about the current turn after advancing.
type TurnInfo struct {
	Turn               refdata.Turn
	CombatantID        uuid.UUID
	RoundNumber        int32
	Skipped            bool
	SkippedCombatants  []SkippedInfo
}

// MarkSurprised adds the surprised condition to a combatant.
func (s *Service) MarkSurprised(ctx context.Context, combatantID uuid.UUID) error {
	c, err := s.store.GetCombatant(ctx, combatantID)
	if err != nil {
		return fmt.Errorf("getting combatant: %w", err)
	}
	newConds, err := AddSurprisedCondition(c.Conditions)
	if err != nil {
		return fmt.Errorf("adding surprised condition: %w", err)
	}
	if _, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              combatantID,
		Conditions:      newConds,
		ExhaustionLevel: c.ExhaustionLevel,
	}); err != nil {
		return fmt.Errorf("updating conditions: %w", err)
	}
	return nil
}

// AdvanceTurn completes the current turn (if any), determines the next combatant,
// creates a new turn (skipping surprised combatants in round 1), and advances the
// round when all combatants have gone.
func (s *Service) AdvanceTurn(ctx context.Context, encounterID uuid.UUID) (TurnInfo, error) {
	enc, err := s.store.GetEncounter(ctx, encounterID)
	if err != nil {
		return TurnInfo{}, fmt.Errorf("getting encounter: %w", err)
	}

	// Complete current turn if there is one
	if enc.CurrentTurnID.Valid {
		// Process end-of-turn condition expiration for the current combatant
		currentTurn, err := s.store.GetTurn(ctx, enc.CurrentTurnID.UUID)
		if err != nil {
			return TurnInfo{}, fmt.Errorf("getting current turn: %w", err)
		}
		if _, err := s.ProcessTurnEndWithLog(ctx, encounterID, currentTurn.CombatantID, enc.RoundNumber, enc.CurrentTurnID.UUID); err != nil {
			return TurnInfo{}, fmt.Errorf("processing turn end conditions: %w", err)
		}

		if _, err := s.store.CompleteTurn(ctx, enc.CurrentTurnID.UUID); err != nil {
			return TurnInfo{}, fmt.Errorf("completing current turn: %w", err)
		}
	}

	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return TurnInfo{}, fmt.Errorf("listing combatants: %w", err)
	}
	if len(combatants) == 0 {
		return TurnInfo{}, errors.New("no combatants in encounter")
	}

	roundNumber := enc.RoundNumber

	// Get turns already taken this round
	turns, err := s.store.ListTurnsByEncounterAndRound(ctx, refdata.ListTurnsByEncounterAndRoundParams{
		EncounterID: encounterID,
		RoundNumber: roundNumber,
	})
	if err != nil {
		return TurnInfo{}, fmt.Errorf("listing turns: %w", err)
	}

	hadTurn := make(map[uuid.UUID]bool)
	for _, t := range turns {
		hadTurn[t.CombatantID] = true
	}

	// Build ordered list of candidates (alive combatants who haven't gone yet).
	// Summoned creatures share their summoner's turn — they never get their own.
	var candidates []refdata.Combatant
	for _, c := range combatants {
		if !c.IsAlive || hadTurn[c.ID] || IsSummonedCreature(c) {
			continue
		}
		candidates = append(candidates, c)
	}

	// If no candidates, advance to next round
	if len(candidates) == 0 {
		roundNumber++
		if _, err := s.store.UpdateEncounterRound(ctx, refdata.UpdateEncounterRoundParams{
			ID:          encounterID,
			RoundNumber: roundNumber,
		}); err != nil {
			return TurnInfo{}, fmt.Errorf("advancing round: %w", err)
		}
		// Reset candidates to all alive non-summoned combatants
		for _, c := range combatants {
			if c.IsAlive && !IsSummonedCreature(c) {
				candidates = append(candidates, c)
			}
		}
	}

	if len(candidates) == 0 {
		return TurnInfo{}, errors.New("no alive combatants")
	}

	// Iterate through candidates, skipping surprised and incapacitated ones
	allSurprised := roundNumber == 1
	allIncapacitated := true
	var skippedCombatants []SkippedInfo
	for _, candidate := range candidates {
		conds, _ := parseConditions(candidate.Conditions)

		if roundNumber == 1 && hasCondition(conds, "surprised") {
			if err := s.skipSurprisedTurn(ctx, encounterID, roundNumber, candidate); err != nil {
				return TurnInfo{}, err
			}
			continue
		}
		allSurprised = false

		info, skipped, err := s.skipOrActivate(ctx, encounterID, roundNumber, candidate, conds, skippedCombatants)
		if err != nil {
			return TurnInfo{}, err
		}
		if skipped != nil {
			skippedCombatants = append(skippedCombatants, *skipped)
			continue
		}
		allIncapacitated = false
		return info, nil
	}

	// All candidates were surprised or incapacitated — advance to next round
	if allSurprised || allIncapacitated {
		reason := "all surprised"
		if allIncapacitated {
			reason = "all incapacitated"
		}
		roundNumber++
		if _, err := s.store.UpdateEncounterRound(ctx, refdata.UpdateEncounterRoundParams{
			ID:          encounterID,
			RoundNumber: roundNumber,
		}); err != nil {
			return TurnInfo{}, fmt.Errorf("advancing round after %s: %w", reason, err)
		}

		return s.findFirstActiveCombatant(ctx, encounterID, roundNumber, combatants, skippedCombatants)
	}

	return TurnInfo{}, errors.New("no alive combatants")
}

// skipOrActivate checks if a combatant is incapacitated: if so, skips their turn and
// returns the SkippedInfo; otherwise creates an active turn. Returns (info, nil, nil)
// for an active turn or (zero, skipped, nil) for a skipped turn.
func (s *Service) skipOrActivate(ctx context.Context, encounterID uuid.UUID, roundNumber int32, combatant refdata.Combatant, conds []CombatCondition, priorSkipped []SkippedInfo) (TurnInfo, *SkippedInfo, error) {
	if IsIncapacitated(conds) {
		condName := GetIncapacitatingConditionName(conds)
		if err := s.skipCombatantTurn(ctx, encounterID, roundNumber, combatant, "incapacitated"); err != nil {
			return TurnInfo{}, nil, err
		}
		return TurnInfo{}, &SkippedInfo{
			CombatantID:   combatant.ID,
			DisplayName:   combatant.DisplayName,
			ConditionName: condName,
		}, nil
	}

	info, err := s.createActiveTurn(ctx, encounterID, roundNumber, combatant)
	if err != nil {
		return TurnInfo{}, nil, err
	}
	info.SkippedCombatants = priorSkipped
	return info, nil, nil
}

// findFirstActiveCombatant iterates alive combatants, skipping incapacitated ones,
// and returns a TurnInfo for the first combatant that can act.
func (s *Service) findFirstActiveCombatant(ctx context.Context, encounterID uuid.UUID, roundNumber int32, combatants []refdata.Combatant, skippedCombatants []SkippedInfo) (TurnInfo, error) {
	for _, c := range combatants {
		if !c.IsAlive || IsSummonedCreature(c) {
			continue
		}
		conds, _ := parseConditions(c.Conditions)
		info, skipped, err := s.skipOrActivate(ctx, encounterID, roundNumber, c, conds, skippedCombatants)
		if err != nil {
			return TurnInfo{}, err
		}
		if skipped != nil {
			skippedCombatants = append(skippedCombatants, *skipped)
			continue
		}
		return info, nil
	}
	return TurnInfo{}, errors.New("no alive combatants")
}

// skipSurprisedTurn skips a surprised combatant's turn and removes the surprised condition.
func (s *Service) skipSurprisedTurn(ctx context.Context, encounterID uuid.UUID, roundNumber int32, combatant refdata.Combatant) error {
	if err := s.skipCombatantTurn(ctx, encounterID, roundNumber, combatant, "surprised"); err != nil {
		return err
	}
	newConds, err := RemoveSurprisedCondition(combatant.Conditions)
	if err != nil {
		return fmt.Errorf("removing surprised condition: %w", err)
	}
	if _, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              combatant.ID,
		Conditions:      newConds,
		ExhaustionLevel: combatant.ExhaustionLevel,
	}); err != nil {
		return fmt.Errorf("updating conditions after surprise skip: %w", err)
	}
	return nil
}

// skipCombatantTurn creates and immediately skips a turn for the given reason.
func (s *Service) skipCombatantTurn(ctx context.Context, encounterID uuid.UUID, roundNumber int32, combatant refdata.Combatant, reason string) error {
	turn, err := s.store.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID: encounterID,
		CombatantID: combatant.ID,
		RoundNumber: roundNumber,
		Status:      "skipped",
	})
	if err != nil {
		return fmt.Errorf("creating skipped turn for %s: %w", reason, err)
	}
	if _, err := s.store.SkipTurn(ctx, turn.ID); err != nil {
		return fmt.Errorf("skipping %s turn: %w", reason, err)
	}
	return nil
}

// resolveTimerForTurn looks up the campaign's timeout setting and returns
// started_at (now) and timeout_at for a new turn. Returns zero values
// if the campaign lookup fails (graceful degradation).
func (s *Service) resolveTimerForTurn(ctx context.Context, encounterID uuid.UUID) (sql.NullTime, sql.NullTime) {
	camp, err := s.store.GetCampaignByEncounterID(ctx, encounterID)
	if err != nil {
		return sql.NullTime{}, sql.NullTime{}
	}

	var settings campaign.Settings
	if camp.Settings.Valid {
		if err := json.Unmarshal(camp.Settings.RawMessage, &settings); err != nil {
			return sql.NullTime{}, sql.NullTime{}
		}
	}

	hours := settings.TurnTimeoutHours
	if hours <= 0 {
		hours = 24
	}

	now := time.Now()
	return sql.NullTime{Time: now, Valid: true},
		sql.NullTime{Time: now.Add(time.Duration(hours) * time.Hour), Valid: true}
}

// hasCondition checks if a parsed condition slice contains a specific condition name.
func hasCondition(conds []CombatCondition, name string) bool {
	for _, c := range conds {
		if c.Condition == name {
			return true
		}
	}
	return false
}

// createActiveTurn creates an active turn and updates the encounter's current turn.
// It processes turn-start condition expiration before creating the turn.
// For PC combatants, it sets started_at and timeout_at based on campaign settings.
func (s *Service) createActiveTurn(ctx context.Context, encounterID uuid.UUID, roundNumber int32, combatant refdata.Combatant) (TurnInfo, error) {
	speedFt, attacksRemaining, err := s.ResolveTurnResources(ctx, combatant)
	if err != nil {
		return TurnInfo{}, fmt.Errorf("resolving turn resources: %w", err)
	}

	params := refdata.CreateTurnParams{
		EncounterID:         encounterID,
		CombatantID:         combatant.ID,
		RoundNumber:         roundNumber,
		Status:              "active",
		MovementRemainingFt: speedFt,
		AttacksRemaining:    attacksRemaining,
	}

	// Set timer for PC turns only
	if !combatant.IsNpc {
		startedAt, timeoutAt := s.resolveTimerForTurn(ctx, encounterID)
		params.StartedAt = startedAt
		params.TimeoutAt = timeoutAt
	}

	turn, err := s.store.CreateTurn(ctx, params)
	if err != nil {
		return TurnInfo{}, fmt.Errorf("creating turn: %w", err)
	}

	// Process turn-start condition expiration (after turn created so we have turnID for logging)
	if _, err := s.ProcessTurnStartWithLog(ctx, encounterID, combatant, roundNumber, turn.ID); err != nil {
		return TurnInfo{}, fmt.Errorf("processing turn start conditions: %w", err)
	}

	// Reset summoned creature turn resources for this summoner's creatures
	if err := s.resetSummonedCreatureResources(ctx, encounterID, combatant.ID); err != nil {
		return TurnInfo{}, fmt.Errorf("resetting summoned creature resources: %w", err)
	}

	if _, err := s.store.UpdateEncounterCurrentTurn(ctx, refdata.UpdateEncounterCurrentTurnParams{
		ID:            encounterID,
		CurrentTurnID: uuid.NullUUID{UUID: turn.ID, Valid: true},
	}); err != nil {
		return TurnInfo{}, fmt.Errorf("updating current turn: %w", err)
	}

	s.postEnemyTurnReady(ctx, encounterID, combatant)

	return TurnInfo{
		Turn:        turn,
		CombatantID: combatant.ID,
		RoundNumber: roundNumber,
	}, nil
}

// postEnemyTurnReady dispatches a KindEnemyTurnReady notification through
// the wired Notifier when the combatant is DM-controlled (IsNpc). Errors
// are intentionally swallowed: a notifier hiccup must not undo the
// successfully-persisted turn advance.
func (s *Service) postEnemyTurnReady(ctx context.Context, encounterID uuid.UUID, combatant refdata.Combatant) {
	if s.dmNotifier == nil {
		return
	}
	if !combatant.IsNpc {
		return
	}
	camp, err := s.store.GetCampaignByEncounterID(ctx, encounterID)
	if err != nil {
		return
	}
	_, _ = s.dmNotifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindEnemyTurnReady,
		PlayerName: combatant.DisplayName,
		Summary:    "is up",
		GuildID:    camp.GuildID,
		CampaignID: camp.ID.String(),
	})
}
