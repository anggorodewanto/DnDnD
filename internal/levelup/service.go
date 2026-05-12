package levelup

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ab/dndnd/internal/character"
	"github.com/google/uuid"
)

// StoredCharacter holds the character data needed for level-up operations.
type StoredCharacter struct {
	ID               uuid.UUID       `json:"id"`
	Name             string          `json:"name"`
	DiscordUserID    string          `json:"discord_user_id"`
	Level            int32           `json:"level"`
	HPMax            int32           `json:"hp_max"`
	HPCurrent        int32           `json:"hp_current"`
	ProficiencyBonus int32           `json:"proficiency_bonus"`
	Classes          json.RawMessage `json:"classes"`
	AbilityScores    json.RawMessage `json:"ability_scores"`
	SpellSlots       json.RawMessage `json:"spell_slots"`
	PactMagicSlots   json.RawMessage `json:"pact_magic_slots"`
	Features         json.RawMessage `json:"features"`
}

// StatsUpdate holds the fields to update after a level-up.
type StatsUpdate struct {
	Level            int
	HPMax            int
	HPCurrent        int
	ProficiencyBonus int
	Classes          json.RawMessage
	SpellSlots       json.RawMessage
	PactMagicSlots   json.RawMessage
	AttacksPerAction int
	Features         json.RawMessage
}

// ClassRefData holds the reference data for a class needed for level-up.
type ClassRefData struct {
	HitDie           string
	Spellcasting     *character.ClassSpellcasting
	AttacksPerAction map[int]int
	SubclassLevel    int
	Subclasses       map[string]any
	FeaturesByLevel  map[string][]character.Feature
}

// LevelUpDetails holds information about the level-up for notifications and response.
type LevelUpDetails struct {
	CharacterName       string
	CharacterID         uuid.UUID
	OldLevel            int
	NewLevel            int
	HPGained            int
	NewHPMax            int
	NewProficiencyBonus int
	NewAttacksPerAction int
	LeveledClass        string
	LeveledClassLevel   int
	GrantsASI           bool
	NeedsSubclass       bool
}

// CharacterStore is the interface for character data access.
type CharacterStore interface {
	GetCharacterForLevelUp(ctx context.Context, id uuid.UUID) (*StoredCharacter, error)
	UpdateCharacterStats(ctx context.Context, id uuid.UUID, update StatsUpdate) error
	UpdateAbilityScores(ctx context.Context, id uuid.UUID, scores json.RawMessage) error
	UpdateFeatures(ctx context.Context, id uuid.UUID, features json.RawMessage) error
}

// ClassStore is the interface for class reference data access.
type ClassStore interface {
	GetClassRefData(ctx context.Context, classID string) (*ClassRefData, error)
}

// Notifier sends level-up notifications. SendPublicLevelUp receives the
// character ID so adapters can resolve the owning campaign / guild and
// route through the campaign's #the-story channel.
type Notifier interface {
	SendPublicLevelUp(ctx context.Context, characterID uuid.UUID, characterName string, newLevel int) error
	SendPrivateLevelUp(ctx context.Context, discordUserID string, details LevelUpDetails) error
	SendASIPrompt(ctx context.Context, discordUserID string, characterID uuid.UUID, characterName string) error
	SendASIDenied(ctx context.Context, discordUserID string, characterName string, reason string) error
}

// EncounterPublisher fans out a fresh encounter snapshot when a level-up
// commits stats for a character who is currently in an active encounter.
type EncounterPublisher interface {
	PublishEncounterSnapshot(ctx context.Context, encounterID uuid.UUID) error
}

// EncounterLookup resolves the active encounter (if any) containing the
// given character. Returns (encID, true, nil) when in combat;
// (uuid.Nil, false, nil) otherwise.
type EncounterLookup interface {
	ActiveEncounterIDForCharacter(ctx context.Context, characterID uuid.UUID) (uuid.UUID, bool, error)
}

// Service orchestrates the level-up workflow.
type Service struct {
	charStore  CharacterStore
	classStore ClassStore
	notifier   Notifier
	publisher  EncounterPublisher
	lookup     EncounterLookup
}

// NewService creates a new level-up Service.
func NewService(charStore CharacterStore, classStore ClassStore, notifier Notifier) *Service {
	return &Service{
		charStore:  charStore,
		classStore: classStore,
		notifier:   notifier,
	}
}

// SetPublisher wires an EncounterPublisher and encounter lookup onto the
// service. A nil publisher is tolerated and disables fan-out. Publish errors
// are logged but never surfaced to callers.
func (s *Service) SetPublisher(p EncounterPublisher, lookup EncounterLookup) {
	s.publisher = p
	s.lookup = lookup
}

// publishForCharacter fires the publisher with the character's active
// encounter ID, swallowing errors. Callers invoke this AFTER a successful DB
// mutation. Silently no-ops when the publisher is unset, the lookup is
// unset, the character is not in combat, or lookup/publish fails.
func (s *Service) publishForCharacter(ctx context.Context, charID uuid.UUID) {
	if s.publisher == nil || s.lookup == nil {
		return
	}
	encID, ok, err := s.lookup.ActiveEncounterIDForCharacter(ctx, charID)
	if err != nil {
		slog.Error("levelup: active encounter lookup failed", "error", err, "character_id", charID)
		return
	}
	if !ok {
		return
	}
	if err := s.publisher.PublishEncounterSnapshot(ctx, encID); err != nil {
		slog.Error("levelup: encounter snapshot publish failed", "error", err, "encounter_id", encID)
	}
}

// ApplyLevelUp applies a level-up for a specific class entry.
// classID is the class being leveled, newClassLevel is the new level for that class.
// Returns the LevelUpDetails for the caller to use (e.g. handler response, notifications).
// NOTE: DDB-imported characters should re-import via Phase 90 instead of using this flow.
func (s *Service) ApplyLevelUp(ctx context.Context, characterID uuid.UUID, classID string, newClassLevel int) (*LevelUpDetails, error) {
	// Load character
	char, err := s.charStore.GetCharacterForLevelUp(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("loading character: %w", err)
	}

	// Load class ref data
	classRef, err := s.classStore.GetClassRefData(ctx, classID)
	if err != nil {
		return nil, fmt.Errorf("loading class data: %w", err)
	}

	// Parse current classes
	var oldClasses []character.ClassEntry
	if err := json.Unmarshal(char.Classes, &oldClasses); err != nil {
		return nil, fmt.Errorf("parsing classes: %w", err)
	}

	// Parse ability scores
	var scores character.AbilityScores
	if err := json.Unmarshal(char.AbilityScores, &scores); err != nil {
		return nil, fmt.Errorf("parsing ability scores: %w", err)
	}

	// Build new classes
	newClasses := buildNewClasses(oldClasses, classID, newClassLevel)

	// Build lookup maps for calculations
	hitDice, spellcasting, attacksMap, err := s.buildRefMaps(ctx, newClasses, classRef, classID)
	if err != nil {
		return nil, err
	}

	// Calculate level-up result
	result := CalculateLevelUp(oldClasses, newClasses, hitDice, spellcasting, attacksMap, scores)

	// Build stats update
	newClassesJSON, err := json.Marshal(newClasses)
	if err != nil {
		return nil, fmt.Errorf("marshaling new classes: %w", err)
	}

	update := StatsUpdate{
		Level:            result.NewLevel,
		HPMax:            result.NewHPMax,
		HPCurrent:        int(char.HPCurrent) + result.HPGained,
		ProficiencyBonus: result.NewProficiencyBonus,
		Classes:          newClassesJSON,
		AttacksPerAction: result.NewAttacksPerAction,
	}

	// Spell slots
	if result.NewSpellSlots != nil {
		slotsJSON, err := json.Marshal(result.NewSpellSlots)
		if err != nil {
			return nil, fmt.Errorf("marshaling spell slots: %w", err)
		}
		update.SpellSlots = slotsJSON
	}

	// Pact magic slots
	if result.NewPactSlots.Max > 0 {
		pactJSON, err := json.Marshal(result.NewPactSlots)
		if err != nil {
			return nil, fmt.Errorf("marshaling pact slots: %w", err)
		}
		update.PactMagicSlots = pactJSON
	}

	// Apply update
	if err := s.charStore.UpdateCharacterStats(ctx, characterID, update); err != nil {
		return nil, fmt.Errorf("updating character: %w", err)
	}
	s.publishForCharacter(ctx, characterID)

	// Determine if this level grants ASI
	grantsASI := IsASILevel(classID, newClassLevel)

	// Determine if subclass selection is needed
	hasSubclass := classHasSubclass(oldClasses, classID)
	needsSubclass := NeedsSubclassSelection(newClassLevel, classRef.SubclassLevel, hasSubclass)

	details := &LevelUpDetails{
		CharacterName:       char.Name,
		CharacterID:         characterID,
		OldLevel:            result.OldLevel,
		NewLevel:            result.NewLevel,
		HPGained:            result.HPGained,
		NewHPMax:            result.NewHPMax,
		NewProficiencyBonus: result.NewProficiencyBonus,
		NewAttacksPerAction: result.NewAttacksPerAction,
		LeveledClass:        classID,
		LeveledClassLevel:   newClassLevel,
		GrantsASI:           grantsASI,
		NeedsSubclass:       needsSubclass,
	}

	// Send notifications (log errors rather than silently discarding)
	if s.notifier != nil {
		if err := s.notifier.SendPublicLevelUp(ctx, char.ID, char.Name, result.NewLevel); err != nil {
			slog.Error("failed to send public level-up notification", "error", err, "character", char.Name)
		}
		if err := s.notifier.SendPrivateLevelUp(ctx, char.DiscordUserID, *details); err != nil {
			slog.Error("failed to send private level-up notification", "error", err, "character", char.Name)
		}

		if grantsASI {
			if err := s.notifier.SendASIPrompt(ctx, char.DiscordUserID, characterID, char.Name); err != nil {
				slog.Error("failed to send ASI prompt", "error", err, "character", char.Name)
			}
		}
	}

	return details, nil
}

// ApproveASI applies an approved ASI choice to a character's ability scores.
func (s *Service) ApproveASI(ctx context.Context, characterID uuid.UUID, choice ASIChoice) error {
	char, err := s.charStore.GetCharacterForLevelUp(ctx, characterID)
	if err != nil {
		return fmt.Errorf("loading character: %w", err)
	}

	var scores character.AbilityScores
	if err := json.Unmarshal(char.AbilityScores, &scores); err != nil {
		return fmt.Errorf("parsing ability scores: %w", err)
	}

	newScores, err := ApplyASI(scores, choice)
	if err != nil {
		return fmt.Errorf("applying ASI: %w", err)
	}

	scoresJSON, err := json.Marshal(newScores)
	if err != nil {
		return fmt.Errorf("marshaling scores: %w", err)
	}

	if err := s.charStore.UpdateAbilityScores(ctx, characterID, scoresJSON); err != nil {
		return err
	}
	s.publishForCharacter(ctx, characterID)
	return nil
}

// DenyASI denies a player's ASI choice and re-prompts them.
func (s *Service) DenyASI(ctx context.Context, characterID uuid.UUID, reason string) error {
	char, err := s.charStore.GetCharacterForLevelUp(ctx, characterID)
	if err != nil {
		return fmt.Errorf("loading character: %w", err)
	}

	if s.notifier != nil {
		if err := s.notifier.SendASIDenied(ctx, char.DiscordUserID, char.Name, reason); err != nil {
			slog.Error("failed to send ASI denied notification", "error", err, "character", char.Name)
		}
		if err := s.notifier.SendASIPrompt(ctx, char.DiscordUserID, characterID, char.Name); err != nil {
			slog.Error("failed to send ASI prompt after denial", "error", err, "character", char.Name)
		}
	}

	return nil
}

// ApplyFeat adds a feat to a character's features and applies any ASI bonuses.
func (s *Service) ApplyFeat(ctx context.Context, characterID uuid.UUID, feat FeatInfo) error {
	char, err := s.charStore.GetCharacterForLevelUp(ctx, characterID)
	if err != nil {
		return fmt.Errorf("loading character: %w", err)
	}

	// Parse existing features
	var features []character.Feature
	if len(char.Features) > 0 {
		if err := json.Unmarshal(char.Features, &features); err != nil {
			return fmt.Errorf("parsing features: %w", err)
		}
	}

	// Add the feat as a feature
	feature := character.Feature{
		Name:        feat.Name,
		Source:      "feat",
		Description: fmt.Sprintf("Feat: %s", feat.Name),
	}
	if len(feat.MechanicalEffect) > 0 {
		effectJSON, _ := json.Marshal(feat.MechanicalEffect)
		feature.MechanicalEffect = string(effectJSON)
	}
	features = append(features, feature)

	featuresJSON, err := json.Marshal(features)
	if err != nil {
		return fmt.Errorf("marshaling features: %w", err)
	}

	if err := s.charStore.UpdateFeatures(ctx, characterID, featuresJSON); err != nil {
		return fmt.Errorf("updating features: %w", err)
	}
	s.publishForCharacter(ctx, characterID)

	// Apply ASI bonus from feat if present
	if len(feat.ASIBonus) > 0 {
		if err := s.applyFeatASI(ctx, char, feat.ASIBonus); err != nil {
			return fmt.Errorf("applying feat ASI: %w", err)
		}
	}

	return nil
}

// applyFeatASI applies the ASI bonus from a feat (e.g. {"con": 1} or {"choose_ability": 1, "from": [...]}).
func (s *Service) applyFeatASI(ctx context.Context, char *StoredCharacter, asiBonus map[string]any) error {
	var scores character.AbilityScores
	if err := json.Unmarshal(char.AbilityScores, &scores); err != nil {
		return fmt.Errorf("parsing ability scores: %w", err)
	}

	// Direct ability bonuses (e.g. {"con": 1})
	for ability, val := range asiBonus {
		if ability == "choose_ability" || ability == "from" {
			continue // handled by interactive prompt
		}
		bonus, ok := toInt(val)
		if !ok {
			continue
		}
		current := scores.Get(ability)
		setScore(&scores, ability, current+bonus)
	}

	scoresJSON, err := json.Marshal(scores)
	if err != nil {
		return fmt.Errorf("marshaling scores: %w", err)
	}

	if err := s.charStore.UpdateAbilityScores(ctx, char.ID, scoresJSON); err != nil {
		return err
	}
	s.publishForCharacter(ctx, char.ID)
	return nil
}

// toInt converts a JSON number value to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	}
	return 0, false
}

// buildNewClasses creates the updated class list after a level change.
func buildNewClasses(oldClasses []character.ClassEntry, classID string, newLevel int) []character.ClassEntry {
	newClasses := make([]character.ClassEntry, len(oldClasses))
	copy(newClasses, oldClasses)

	found := false
	for i, c := range newClasses {
		if c.Class == classID {
			newClasses[i].Level = newLevel
			found = true
			break
		}
	}
	if !found {
		newClasses = append(newClasses, character.ClassEntry{
			Class: classID,
			Level: newLevel,
		})
	}
	return newClasses
}

// buildRefMaps builds the lookup maps needed for stat calculations.
func (s *Service) buildRefMaps(
	ctx context.Context,
	classes []character.ClassEntry,
	knownRef *ClassRefData,
	knownClassID string,
) (
	hitDice map[string]string,
	spellcasting map[string]character.ClassSpellcasting,
	attacksMap map[string]map[int]int,
	err error,
) {
	hitDice = make(map[string]string)
	spellcasting = make(map[string]character.ClassSpellcasting)
	attacksMap = make(map[string]map[int]int)

	for _, c := range classes {
		var ref *ClassRefData
		if c.Class == knownClassID {
			ref = knownRef
		} else {
			ref, err = s.classStore.GetClassRefData(ctx, c.Class)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("loading class %s: %w", c.Class, err)
			}
		}
		hitDice[c.Class] = ref.HitDie
		if ref.Spellcasting != nil {
			spellcasting[c.Class] = *ref.Spellcasting
		}
		attacksMap[c.Class] = ref.AttacksPerAction
	}
	return
}

// classHasSubclass checks whether the given class already has a subclass selected.
func classHasSubclass(classes []character.ClassEntry, classID string) bool {
	for _, c := range classes {
		if c.Class == classID {
			return c.Subclass != ""
		}
	}
	return false
}
