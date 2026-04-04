package levelup

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/google/uuid"
)

// --- Mock dependencies ---

type mockCharacterStore struct {
	chars map[uuid.UUID]*StoredCharacter
}

func newMockCharacterStore() *mockCharacterStore {
	return &mockCharacterStore{chars: make(map[uuid.UUID]*StoredCharacter)}
}

func (m *mockCharacterStore) GetCharacterForLevelUp(ctx context.Context, id uuid.UUID) (*StoredCharacter, error) {
	c, ok := m.chars[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return c, nil
}

func (m *mockCharacterStore) UpdateCharacterStats(ctx context.Context, id uuid.UUID, update StatsUpdate) error {
	c, ok := m.chars[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	c.Level = int32(update.Level)
	c.HPMax = int32(update.HPMax)
	c.ProficiencyBonus = int32(update.ProficiencyBonus)
	c.Classes = update.Classes
	return nil
}

func (m *mockCharacterStore) UpdateAbilityScores(ctx context.Context, id uuid.UUID, scores json.RawMessage) error {
	c, ok := m.chars[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	c.AbilityScores = scores
	return nil
}

func (m *mockCharacterStore) UpdateFeatures(ctx context.Context, id uuid.UUID, features json.RawMessage) error {
	c, ok := m.chars[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	c.Features = features
	return nil
}

type mockClassStore struct {
	classes map[string]*ClassRefData
}

func newMockClassStore() *mockClassStore {
	return &mockClassStore{classes: make(map[string]*ClassRefData)}
}

func (m *mockClassStore) GetClassRefData(ctx context.Context, classID string) (*ClassRefData, error) {
	c, ok := m.classes[classID]
	if !ok {
		return nil, fmt.Errorf("class not found: %s", classID)
	}
	return c, nil
}

type mockNotifier struct {
	publicMessages  []string
	privateMessages []string
}

func (m *mockNotifier) SendPublicLevelUp(ctx context.Context, characterName string, newLevel int) error {
	m.publicMessages = append(m.publicMessages, fmt.Sprintf("%s reached Level %d", characterName, newLevel))
	return nil
}

func (m *mockNotifier) SendPrivateLevelUp(ctx context.Context, discordUserID string, details LevelUpDetails) error {
	m.privateMessages = append(m.privateMessages, fmt.Sprintf("Details for %s: HP+%d", details.CharacterName, details.HPGained))
	return nil
}

func (m *mockNotifier) SendASIPrompt(ctx context.Context, discordUserID string, characterID uuid.UUID, characterName string) error {
	m.privateMessages = append(m.privateMessages, fmt.Sprintf("ASI prompt for %s", characterName))
	return nil
}

func (m *mockNotifier) SendASIDenied(ctx context.Context, discordUserID string, characterName string, reason string) error {
	m.privateMessages = append(m.privateMessages, fmt.Sprintf("ASI denied for %s: %s", characterName, reason))
	return nil
}

// --- Tests ---

func TestService_ApplyLevelUp_BasicFighter(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	classes := []character.ClassEntry{{Class: "fighter", Level: 5}}
	classesJSON, _ := json.Marshal(classes)

	charStore.chars[charID] = &StoredCharacter{
		ID:               charID,
		Name:             "Aria",
		DiscordUserID:    "user123",
		Level:            5,
		HPMax:            44,
		HPCurrent:        44,
		ProficiencyBonus: 3,
		Classes:          classesJSON,
		AbilityScores:    mustJSON(t, character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}),
	}

	classStore.classes["fighter"] = &ClassRefData{
		HitDie:           "d10",
		AttacksPerAction: map[int]int{1: 1, 5: 2, 11: 3, 20: 4},
		SubclassLevel:    3,
	}

	svc := NewService(charStore, classStore, notifier)

	_, err := svc.ApplyLevelUp(context.Background(), charID, "fighter", 6)
	if err != nil {
		t.Fatalf("ApplyLevelUp error: %v", err)
	}

	// Verify character was updated
	updated := charStore.chars[charID]
	if updated.Level != 6 {
		t.Errorf("Level = %d, want 6", updated.Level)
	}
	if updated.HPMax <= 44 {
		t.Errorf("HPMax = %d, want > 44", updated.HPMax)
	}

	// Verify notifications sent
	if len(notifier.publicMessages) != 1 {
		t.Errorf("public messages = %d, want 1", len(notifier.publicMessages))
	}
	// Fighter level 6 is an extra ASI level, so we get level-up + ASI prompt = 2 private messages
	if len(notifier.privateMessages) != 2 {
		t.Errorf("private messages = %d, want 2 (level-up detail + ASI prompt for fighter 6)", len(notifier.privateMessages))
	}
}

func TestService_ApplyLevelUp_ASILevel(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	classes := []character.ClassEntry{{Class: "fighter", Level: 3}}
	classesJSON, _ := json.Marshal(classes)

	charStore.chars[charID] = &StoredCharacter{
		ID:               charID,
		Name:             "Brom",
		DiscordUserID:    "user456",
		Level:            3,
		HPMax:            28,
		HPCurrent:        28,
		ProficiencyBonus: 2,
		Classes:          classesJSON,
		AbilityScores:    mustJSON(t, character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}),
	}

	classStore.classes["fighter"] = &ClassRefData{
		HitDie:           "d10",
		AttacksPerAction: map[int]int{1: 1, 5: 2, 11: 3, 20: 4},
		SubclassLevel:    3,
	}

	svc := NewService(charStore, classStore, notifier)

	_, err := svc.ApplyLevelUp(context.Background(), charID, "fighter", 4)
	if err != nil {
		t.Fatalf("ApplyLevelUp error: %v", err)
	}

	// Level 4 is ASI level, should send ASI prompt
	hasASIPrompt := false
	for _, msg := range notifier.privateMessages {
		if msg == "ASI prompt for Brom" {
			hasASIPrompt = true
			break
		}
	}
	if !hasASIPrompt {
		t.Errorf("expected ASI prompt in private messages, got: %v", notifier.privateMessages)
	}
}

func TestService_ApplyLevelUp_NewMulticlass(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	classes := []character.ClassEntry{{Class: "fighter", Level: 5}}
	classesJSON, _ := json.Marshal(classes)

	charStore.chars[charID] = &StoredCharacter{
		ID:               charID,
		Name:             "Cira",
		DiscordUserID:    "user789",
		Level:            5,
		HPMax:            44,
		HPCurrent:        44,
		ProficiencyBonus: 3,
		Classes:          classesJSON,
		AbilityScores:    mustJSON(t, character.AbilityScores{STR: 14, DEX: 10, CON: 14, INT: 10, WIS: 13, CHA: 14}),
	}

	classStore.classes["fighter"] = &ClassRefData{
		HitDie:           "d10",
		AttacksPerAction: map[int]int{1: 1, 5: 2, 11: 3, 20: 4},
		SubclassLevel:    3,
	}
	classStore.classes["cleric"] = &ClassRefData{
		HitDie:           "d8",
		Spellcasting:     &character.ClassSpellcasting{SlotProgression: "full", SpellAbility: "wis"},
		AttacksPerAction: map[int]int{1: 1},
		SubclassLevel:    1,
	}

	svc := NewService(charStore, classStore, notifier)

	// Add cleric level 1 (multiclass)
	_, err := svc.ApplyLevelUp(context.Background(), charID, "cleric", 1)
	if err != nil {
		t.Fatalf("ApplyLevelUp error: %v", err)
	}

	updated := charStore.chars[charID]
	if updated.Level != 6 {
		t.Errorf("Level = %d, want 6", updated.Level)
	}

	// Verify classes JSON was updated
	var updatedClasses []character.ClassEntry
	json.Unmarshal(updated.Classes, &updatedClasses)
	if len(updatedClasses) != 2 {
		t.Fatalf("expected 2 class entries, got %d", len(updatedClasses))
	}
	if updatedClasses[1].Class != "cleric" || updatedClasses[1].Level != 1 {
		t.Errorf("new class entry = %+v, want cleric level 1", updatedClasses[1])
	}
}

func TestService_ApplyLevelUp_InvalidClass(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	classes := []character.ClassEntry{{Class: "fighter", Level: 5}}
	classesJSON, _ := json.Marshal(classes)

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Test",
		Level:         5,
		Classes:       classesJSON,
		AbilityScores: mustJSON(t, character.AbilityScores{}),
	}

	svc := NewService(charStore, classStore, notifier)

	_, err := svc.ApplyLevelUp(context.Background(), charID, "nonexistent", 6)
	if err == nil {
		t.Error("expected error for nonexistent class")
	}
}

func TestService_ApproveASI_Plus2(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	classes := []character.ClassEntry{{Class: "fighter", Level: 4}}
	classesJSON, _ := json.Marshal(classes)
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Aria",
		DiscordUserID: "user123",
		Level:         4,
		HPMax:         36,
		HPCurrent:     36,
		Classes:       classesJSON,
		AbilityScores: mustJSON(t, scores),
	}

	svc := NewService(charStore, classStore, notifier)

	choice := ASIChoice{Type: ASIPlus2, Ability: "str"}
	err := svc.ApproveASI(context.Background(), charID, choice)
	if err != nil {
		t.Fatalf("ApproveASI error: %v", err)
	}

	// Verify ability scores were updated
	var updatedScores character.AbilityScores
	json.Unmarshal(charStore.chars[charID].AbilityScores, &updatedScores)
	if updatedScores.STR != 18 {
		t.Errorf("STR = %d, want 18", updatedScores.STR)
	}
}

func TestService_ApproveASI_InvalidChoice(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	classes := []character.ClassEntry{{Class: "fighter", Level: 4}}
	classesJSON, _ := json.Marshal(classes)

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Aria",
		Level:         4,
		Classes:       classesJSON,
		AbilityScores: mustJSON(t, character.AbilityScores{STR: 20, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}),
	}

	svc := NewService(charStore, classStore, notifier)

	choice := ASIChoice{Type: ASIPlus2, Ability: "str"}
	err := svc.ApproveASI(context.Background(), charID, choice)
	if err == nil {
		t.Error("expected error for already-max ability score")
	}
}

func TestService_DenyASI(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Brom",
		DiscordUserID: "user456",
	}

	svc := NewService(charStore, classStore, notifier)

	err := svc.DenyASI(context.Background(), charID, "Choose a different ability")
	if err != nil {
		t.Fatalf("DenyASI error: %v", err)
	}

	// Should send notification about denial + re-prompt
	if len(notifier.privateMessages) < 1 {
		t.Error("expected at least one private message for denial")
	}
}

func TestService_ApplyFeat(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	classes := []character.ClassEntry{{Class: "fighter", Level: 4}}
	classesJSON, _ := json.Marshal(classes)
	features := []character.Feature{{Name: "Second Wind", Source: "fighter", Level: 1}}
	featuresJSON, _ := json.Marshal(features)
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Aria",
		DiscordUserID: "user123",
		Level:         4,
		Classes:       classesJSON,
		AbilityScores: mustJSON(t, scores),
		Features:      featuresJSON,
	}

	feat := FeatInfo{
		ID:   "alert",
		Name: "Alert",
		MechanicalEffect: []map[string]string{
			{"effect_type": "bonus_initiative", "value": "5"},
		},
	}

	svc := NewService(charStore, classStore, notifier)

	err := svc.ApplyFeat(context.Background(), charID, feat)
	if err != nil {
		t.Fatalf("ApplyFeat error: %v", err)
	}

	// Verify features were updated
	var updatedFeatures []character.Feature
	json.Unmarshal(charStore.chars[charID].Features, &updatedFeatures)
	if len(updatedFeatures) != 2 {
		t.Fatalf("features count = %d, want 2", len(updatedFeatures))
	}
	if updatedFeatures[1].Name != "Alert" {
		t.Errorf("new feature name = %s, want Alert", updatedFeatures[1].Name)
	}
	if updatedFeatures[1].MechanicalEffect == "" {
		t.Error("expected mechanical_effect to be set")
	}
}

func TestService_ApplyFeat_WithASIBonus(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	classes := []character.ClassEntry{{Class: "fighter", Level: 4}}
	classesJSON, _ := json.Marshal(classes)
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Brom",
		DiscordUserID: "user456",
		Level:         4,
		Classes:       classesJSON,
		AbilityScores: mustJSON(t, scores),
		Features:      mustJSON(t, []character.Feature{}),
	}

	feat := FeatInfo{
		ID:       "durable",
		Name:     "Durable",
		ASIBonus: map[string]any{"con": float64(1)},
	}

	svc := NewService(charStore, classStore, notifier)

	err := svc.ApplyFeat(context.Background(), charID, feat)
	if err != nil {
		t.Fatalf("ApplyFeat error: %v", err)
	}

	var updatedScores character.AbilityScores
	json.Unmarshal(charStore.chars[charID].AbilityScores, &updatedScores)
	if updatedScores.CON != 15 {
		t.Errorf("CON = %d, want 15", updatedScores.CON)
	}
}

func TestService_ApplyFeat_WithChooseAbilityASI(t *testing.T) {
	// Feats with choose_ability should skip the choose_ability/from keys in direct ASI
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Test",
		DiscordUserID: "user123",
		Level:         4,
		Classes:       mustJSON(t, []character.ClassEntry{{Class: "fighter", Level: 4}}),
		AbilityScores: mustJSON(t, scores),
		Features:      mustJSON(t, []character.Feature{}),
	}

	feat := FeatInfo{
		ID:       "athlete",
		Name:     "Athlete",
		ASIBonus: map[string]any{"choose_ability": float64(1), "from": []any{"str", "dex"}},
	}

	svc := NewService(charStore, classStore, notifier)

	err := svc.ApplyFeat(context.Background(), charID, feat)
	if err != nil {
		t.Fatalf("ApplyFeat error: %v", err)
	}

	// The choose_ability and from keys should be skipped, scores unchanged
	var updatedScores character.AbilityScores
	json.Unmarshal(charStore.chars[charID].AbilityScores, &updatedScores)
	if updatedScores.STR != 16 {
		t.Errorf("STR = %d, want 16 (unchanged)", updatedScores.STR)
	}
}

func TestService_ApplyLevelUp_WithSpellcasting(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}

	classes := []character.ClassEntry{{Class: "wizard", Level: 4}}
	scores := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Gandolf",
		DiscordUserID: "user999",
		Level:         4,
		HPMax:         22,
		HPCurrent:     22,
		Classes:       mustJSON(t, classes),
		AbilityScores: mustJSON(t, scores),
	}

	classStore.classes["wizard"] = &ClassRefData{
		HitDie:           "d6",
		Spellcasting:     &character.ClassSpellcasting{SlotProgression: "full", SpellAbility: "int"},
		AttacksPerAction: map[int]int{1: 1},
		SubclassLevel:    2,
	}

	svc := NewService(charStore, classStore, notifier)

	_, err := svc.ApplyLevelUp(context.Background(), charID, "wizard", 5)
	if err != nil {
		t.Fatalf("ApplyLevelUp error: %v", err)
	}

	updated := charStore.chars[charID]
	if updated.Level != 5 {
		t.Errorf("Level = %d, want 5", updated.Level)
	}
}

// mockErrorNotifier returns errors from all Send methods.
type mockErrorNotifier struct{}

func (m *mockErrorNotifier) SendPublicLevelUp(ctx context.Context, characterName string, newLevel int) error {
	return fmt.Errorf("public notification failed")
}
func (m *mockErrorNotifier) SendPrivateLevelUp(ctx context.Context, discordUserID string, details LevelUpDetails) error {
	return fmt.Errorf("private notification failed")
}
func (m *mockErrorNotifier) SendASIPrompt(ctx context.Context, discordUserID string, characterID uuid.UUID, characterName string) error {
	return fmt.Errorf("ASI prompt failed")
}
func (m *mockErrorNotifier) SendASIDenied(ctx context.Context, discordUserID string, characterName string, reason string) error {
	return fmt.Errorf("ASI denied notification failed")
}

func TestService_ApplyLevelUp_NotificationErrorsDoNotFail(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()

	classes := []character.ClassEntry{{Class: "fighter", Level: 3}}
	classesJSON, _ := json.Marshal(classes)

	charStore.chars[charID] = &StoredCharacter{
		ID:               charID,
		Name:             "Errata",
		DiscordUserID:    "user999",
		Level:            3,
		HPMax:            28,
		HPCurrent:        28,
		ProficiencyBonus: 2,
		Classes:          classesJSON,
		AbilityScores:    mustJSON(t, character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}),
	}

	classStore.classes["fighter"] = &ClassRefData{
		HitDie:           "d10",
		AttacksPerAction: map[int]int{1: 1, 5: 2},
		SubclassLevel:    3,
	}

	svc := NewService(charStore, classStore, &mockErrorNotifier{})

	// Level 4 triggers ASI prompt too, all 3 notifications will error but ApplyLevelUp should succeed
	details, err := svc.ApplyLevelUp(context.Background(), charID, "fighter", 4)
	if err != nil {
		t.Fatalf("ApplyLevelUp should not fail when notifications error: %v", err)
	}
	if details.NewLevel != 4 {
		t.Errorf("NewLevel = %d, want 4", details.NewLevel)
	}
}

func TestService_DenyASI_NotificationErrorsDoNotFail(t *testing.T) {
	charID := uuid.New()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Errata",
		DiscordUserID: "user999",
	}

	svc := NewService(charStore, classStore, &mockErrorNotifier{})

	err := svc.DenyASI(context.Background(), charID, "bad choice")
	if err != nil {
		t.Fatalf("DenyASI should not fail when notifications error: %v", err)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
