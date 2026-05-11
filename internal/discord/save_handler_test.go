package discord

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/sqlc-dev/pqtype"
)

func makeSaveInteraction(ability string, adv, disadv bool) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "ability", Value: ability, Type: discordgo.ApplicationCommandOptionString},
	}
	if adv {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "adv", Value: true, Type: discordgo.ApplicationCommandOptionBoolean,
		})
	}
	if disadv {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "disadv", Value: true, Type: discordgo.ApplicationCommandOptionBoolean,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "save",
			Options: opts,
		},
	}
}

func makeTestCharacterWithSaves() refdata.Character {
	scores, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 18, CHA: 8})
	profs, _ := json.Marshal(map[string]interface{}{
		"saves":  []string{"wis", "cha"},
		"skills": []string{"perception"},
	})
	return refdata.Character{
		ID:               uuid.New(),
		CampaignID:       uuid.New(),
		Name:             "Aria",
		Level:            5,
		ProficiencyBonus: 3,
		AbilityScores:    scores,
		Proficiencies:    pqtype.NullRawMessage{RawMessage: profs, Valid: true},
	}
}

func setupSaveHandler(sess *MockSession) (*SaveHandler, *mockCheckRollLogger) {
	campaignID := uuid.New()
	char := makeTestCharacterWithSaves()
	char.CampaignID = campaignID

	roller := dice.NewRoller(func(max int) int { return 12 })
	logger := &mockCheckRollLogger{}

	h := NewSaveHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errNoEncounter
		}},
		nil,
		logger,
	)
	return h, logger
}

// --- E-59 AoE pending-save resolver tests ---

type mockAoESaveResolver struct {
	recordCombatantID uuid.UUID
	recordAbility     string
	recordTotal       int
	recordSuccess     bool
	recordCalls       int
	recordSpellID     string
	recordResolved    bool
	recordErr         error

	resolveCalls   int
	resolveSpellID string
	resolveResult  *struct{ totalDamage int }
	resolveErr     error
}

func (m *mockAoESaveResolver) RecordAoEPendingSaveRoll(_ context.Context, combatantID uuid.UUID, ability string, total int, success bool) (string, bool, error) {
	m.recordCalls++
	m.recordCombatantID = combatantID
	m.recordAbility = ability
	m.recordTotal = total
	m.recordSuccess = success
	return m.recordSpellID, m.recordResolved, m.recordErr
}

func (m *mockAoESaveResolver) ResolveAoEPendingSavesForSpell(_ context.Context, _ uuid.UUID, spellID string) error {
	m.resolveCalls++
	m.resolveSpellID = spellID
	return m.resolveErr
}

// TestSaveHandler_RecordsAndResolvesAoEPendingSaves verifies that when a
// player /saves with an AoE pending row outstanding, the handler resolves
// the row and (if it was the last one) drives the AoE damage application.
func TestSaveHandler_RecordsAndResolvesAoEPendingSaves(t *testing.T) {
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error { return nil }

	combatantID := uuid.New()
	campaignID := uuid.New()
	encounterID := uuid.New()
	char := makeTestCharacterWithSaves()
	char.CampaignID = campaignID

	resolver := &mockAoESaveResolver{
		recordSpellID:  "fireball",
		recordResolved: true,
	}

	roller := dice.NewRoller(func(_ int) int { return 12 })
	h := NewSaveHandler(
		sess, roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		}},
		&mockCheckCombatantLookup{listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combatantID, CharacterID: uuid.NullUUID{UUID: char.ID, Valid: true}, Conditions: json.RawMessage(`[]`)},
			}, nil
		}},
		&mockCheckRollLogger{},
	)
	h.SetAoESaveResolver(resolver)
	h.Handle(makeSaveInteraction("dex", false, false))

	if resolver.recordCalls != 1 {
		t.Fatalf("expected RecordAoEPendingSaveRoll called once, got %d", resolver.recordCalls)
	}
	if resolver.recordCombatantID != combatantID {
		t.Errorf("expected resolver to be called with combatant %s, got %s", combatantID, resolver.recordCombatantID)
	}
	if resolver.recordAbility != "dex" {
		t.Errorf("expected ability=dex, got %q", resolver.recordAbility)
	}
	if resolver.resolveCalls != 1 {
		t.Errorf("expected ResolveAoEPendingSavesForSpell called once after a save was recorded, got %d", resolver.resolveCalls)
	}
	if resolver.resolveSpellID != "fireball" {
		t.Errorf("expected resolver to drive damage on fireball, got %q", resolver.resolveSpellID)
	}
}

// TestSaveHandler_NoAoEPendingSave_SkipsResolution verifies the resolver is
// not invoked if no AoE row matched (player rolled save without a pending
// row, e.g. proactive defensive check).
func TestSaveHandler_NoAoEPendingSave_SkipsResolution(t *testing.T) {
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error { return nil }

	combatantID := uuid.New()
	campaignID := uuid.New()
	encounterID := uuid.New()
	char := makeTestCharacterWithSaves()
	char.CampaignID = campaignID

	resolver := &mockAoESaveResolver{recordResolved: false}

	roller := dice.NewRoller(func(_ int) int { return 12 })
	h := NewSaveHandler(
		sess, roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		}},
		&mockCheckCombatantLookup{listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combatantID, CharacterID: uuid.NullUUID{UUID: char.ID, Valid: true}, Conditions: json.RawMessage(`[]`)},
			}, nil
		}},
		&mockCheckRollLogger{},
	)
	h.SetAoESaveResolver(resolver)
	h.Handle(makeSaveInteraction("dex", false, false))

	if resolver.recordCalls != 1 {
		t.Errorf("expected record-call attempted once, got %d", resolver.recordCalls)
	}
	if resolver.resolveCalls != 0 {
		t.Errorf("expected no damage-resolution when no row matched, got %d", resolver.resolveCalls)
	}
}

// TestAoESaveServiceAdapter_Forwards verifies that the adapter passes the
// record/resolve calls straight through and injects the roller on the
// damage-resolution path.
func TestAoESaveServiceAdapter_Forwards(t *testing.T) {
	mock := &mockAoESaveAdapterBackend{
		recordSpellID:  "fireball",
		recordResolved: true,
	}
	adapter := NewAoESaveServiceAdapter(mock, dice.NewRoller(func(_ int) int { return 4 }))

	id := uuid.New()
	spellID, resolved, err := adapter.RecordAoEPendingSaveRoll(context.Background(), id, "dex", 18, false)
	if err != nil {
		t.Fatalf("RecordAoEPendingSaveRoll: %v", err)
	}
	if spellID != "fireball" || !resolved {
		t.Errorf("forwarding broken: got (%q, %v)", spellID, resolved)
	}
	if mock.recordCalls != 1 {
		t.Errorf("expected backend RecordAoEPendingSaveRoll called once, got %d", mock.recordCalls)
	}

	if err := adapter.ResolveAoEPendingSavesForSpell(context.Background(), uuid.New(), "fireball"); err != nil {
		t.Fatalf("ResolveAoEPendingSavesForSpell: %v", err)
	}
	if mock.resolveCalls != 1 {
		t.Errorf("expected backend ResolveAoEPendingSaves called once, got %d", mock.resolveCalls)
	}
	if mock.lastRoller == nil {
		t.Errorf("expected adapter to inject its roller, got nil")
	}
}

type mockAoESaveAdapterBackend struct {
	recordCalls    int
	recordSpellID  string
	recordResolved bool

	resolveCalls int
	lastRoller   *dice.Roller
}

func (m *mockAoESaveAdapterBackend) RecordAoEPendingSaveRoll(_ context.Context, _ uuid.UUID, _ string, _ int, _ bool) (string, bool, error) {
	m.recordCalls++
	return m.recordSpellID, m.recordResolved, nil
}

func (m *mockAoESaveAdapterBackend) ResolveAoEPendingSaves(_ context.Context, _ uuid.UUID, _ string, r *dice.Roller) (*combat.AoEDamageResult, error) {
	m.resolveCalls++
	m.lastRoller = r
	return nil, nil
}

func TestSaveHandler_BasicSave(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, logger := setupSaveHandler(sess)
	h.Handle(makeSaveInteraction("wis", false, false))

	if responded == "" {
		t.Fatal("expected a response")
	}
	if !strings.Contains(responded, "Aria") {
		t.Errorf("expected Aria in response, got: %s", responded)
	}
	if !strings.Contains(responded, "WIS Save") {
		t.Errorf("expected WIS Save in response, got: %s", responded)
	}
	// Roll logged
	if len(logger.logged) != 1 {
		t.Errorf("expected 1 roll logged, got %d", len(logger.logged))
	}
	if !strings.Contains(logger.logged[0].Purpose, "save") {
		t.Errorf("expected save in purpose, got: %s", logger.logged[0].Purpose)
	}
}

func TestSaveHandler_AdvantageFlag(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupSaveHandler(sess)
	h.Handle(makeSaveInteraction("dex", true, false))

	if !strings.Contains(responded, "advantage") {
		t.Errorf("expected advantage in response, got: %s", responded)
	}
}

func TestSaveHandler_NoAbility(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupSaveHandler(sess)
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "save",
			Options: nil,
		},
	}
	h.Handle(interaction)

	if !strings.Contains(responded, "specify") {
		t.Errorf("expected specify prompt, got: %s", responded)
	}
}

func TestSaveHandler_NoCampaign(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewSaveHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("no campaign")
		}},
		nil, nil, nil, nil,
	)
	h.Handle(makeSaveInteraction("dex", false, false))

	if !strings.Contains(responded, "No campaign") {
		t.Errorf("expected no campaign message, got: %s", responded)
	}
}

func TestSaveHandler_NoCharacter(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewSaveHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: uuid.New()}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return refdata.Character{}, errors.New("not found")
		}},
		nil, nil, nil,
	)
	h.Handle(makeSaveInteraction("dex", false, false))

	if !strings.Contains(responded, "register") || !strings.Contains(responded, "character") {
		t.Errorf("expected register prompt, got: %s", responded)
	}
}

func TestSaveHandler_WithCombatConditions(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	charID := uuid.New()
	encounterID := uuid.New()
	char := makeTestCharacterWithSaves()
	char.ID = charID
	char.CampaignID = campaignID

	condJSON, _ := json.Marshal([]map[string]interface{}{
		{"condition": "paralyzed"},
	})

	roller := dice.NewRoller(func(max int) int { return 10 })
	logger := &mockCheckRollLogger{}

	h := NewSaveHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		}},
		&mockCheckCombatantLookup{listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
					Conditions:  condJSON,
				},
			}, nil
		}},
		logger,
	)

	h.Handle(makeSaveInteraction("dex", false, false))

	if !strings.Contains(responded, "Auto-fail") {
		t.Errorf("expected Auto-fail for paralyzed DEX save, got: %s", responded)
	}
	// Auto-fail should not log a roll
	if len(logger.logged) != 0 {
		t.Errorf("expected 0 rolls logged for auto-fail, got %d", len(logger.logged))
	}
}

func TestSetSaveHandler_RoutesCommand(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	bot := &Bot{session: sess}
	router := NewCommandRouter(bot, nil)

	h, _ := setupSaveHandler(sess)
	router.SetSaveHandler(h)

	router.Handle(makeSaveInteraction("wis", false, false))

	if !strings.Contains(responded, "WIS Save") {
		t.Errorf("expected WIS Save routed through save handler, got: %s", responded)
	}
}

func TestSaveHandler_InvalidAbility(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupSaveHandler(sess)
	h.Handle(makeSaveInteraction("luck", false, false))

	if !strings.Contains(responded, "failed") || !strings.Contains(responded, "unknown") {
		t.Errorf("expected error about unknown ability, got: %s", responded)
	}
}

func TestSaveHandler_BadProficienciesJSON(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	scores, _ := json.Marshal(character.AbilityScores{WIS: 16})
	char := refdata.Character{
		ID:               uuid.New(),
		CampaignID:       campaignID,
		Name:             "Bad",
		Level:            5,
		ProficiencyBonus: 3,
		AbilityScores:    scores,
		Proficiencies:    pqtype.NullRawMessage{RawMessage: []byte(`{bad json`), Valid: true},
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewSaveHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		nil, nil, nil,
	)

	h.Handle(makeSaveInteraction("wis", false, false))

	if !strings.Contains(responded, "Error") {
		t.Errorf("expected Error for bad JSON, got: %s", responded)
	}
}

// --- med-33 / Phase 82: FeatureEffects populated from char.Classes + char.Features ---

func TestBuildSaveFeatureEffects_PopulatesFromCharacterFeatures(t *testing.T) {
	classes, err := json.Marshal([]map[string]any{{"class": "Rogue", "level": 7}})
	if err != nil {
		t.Fatal(err)
	}
	feats, err := json.Marshal([]map[string]any{
		{"name": "Evasion", "mechanical_effect": "evasion"},
	})
	if err != nil {
		t.Fatal(err)
	}
	char := refdata.Character{
		Classes:  classes,
		Features: pqtype.NullRawMessage{RawMessage: feats, Valid: true},
	}

	defs := buildSaveFeatureEffects(char)

	if len(defs) == 0 {
		t.Fatalf("expected feature definitions to be populated, got 0")
	}
	found := false
	for _, d := range defs {
		if d.Name == "Evasion" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Evasion in feature definitions, got: %+v", defs)
	}
}

func TestBuildSaveFeatureEffects_EmptyClassesAndFeatures_ReturnsNil(t *testing.T) {
	char := refdata.Character{}
	if defs := buildSaveFeatureEffects(char); defs != nil {
		t.Errorf("expected nil for empty classes/features, got %+v", defs)
	}
}

func TestBuildSaveFeatureEffects_BadJSON_DegradesToNil(t *testing.T) {
	char := refdata.Character{
		Classes: []byte("not json"),
	}
	// Should degrade gracefully — no panic, no error, just no extra effects.
	_ = buildSaveFeatureEffects(char)
}
