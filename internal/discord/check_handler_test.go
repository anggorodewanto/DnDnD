package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"errors"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/sqlc-dev/pqtype"
)

// --- Mock types for CheckHandler ---

type mockCheckCharacterLookup struct {
	fn func(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error)
}

func (m *mockCheckCharacterLookup) GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error) {
	return m.fn(ctx, campaignID, discordUserID)
}

type mockCheckCampaignProvider struct {
	fn func(ctx context.Context, guildID string) (refdata.Campaign, error)
}

func (m *mockCheckCampaignProvider) GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	return m.fn(ctx, guildID)
}

type mockCheckEncounterProvider struct {
	// Phase 105: legacy guild-only func retained so existing tests keep
	// working. New disambiguation tests can set fnUser instead.
	fn     func(ctx context.Context, guildID string) (uuid.UUID, error)
	fnUser func(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

func (m *mockCheckEncounterProvider) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	if m.fnUser != nil {
		return m.fnUser(ctx, guildID, discordUserID)
	}
	return m.fn(ctx, guildID)
}

type mockCheckCombatantLookup struct {
	listFn func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
}

func (m *mockCheckCombatantLookup) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return m.listFn(ctx, encounterID)
}

type mockCheckRollLogger struct {
	logged []dice.RollLogEntry
}

func (m *mockCheckRollLogger) LogRoll(entry dice.RollLogEntry) error {
	m.logged = append(m.logged, entry)
	return nil
}

// --- Helper to build a /check interaction ---

func makeCheckInteraction(skill string, adv, disadv bool, target ...string) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "skill", Value: skill, Type: discordgo.ApplicationCommandOptionString},
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
	if len(target) > 0 {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "target", Value: target[0], Type: discordgo.ApplicationCommandOptionString,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "check",
			Options: opts,
		},
	}
}

func makeTestCharacter() refdata.Character {
	scores, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 18, CHA: 8})
	profs, _ := json.Marshal(map[string]interface{}{
		"skills": []string{"perception", "insight", "medicine"},
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

func setupCheckHandler(sess *MockSession) (*CheckHandler, *mockCheckRollLogger) {
	campaignID := uuid.New()
	char := makeTestCharacter()
	char.CampaignID = campaignID

	roller := dice.NewRoller(func(max int) int { return 12 })

	logger := &mockCheckRollLogger{}

	h := NewCheckHandler(
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
		nil, // combatant lookup not needed for basic tests
		logger,
	)
	return h, logger
}

func TestCheckHandler_BasicSkillCheck(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, logger := setupCheckHandler(sess)
	h.Handle(makeCheckInteraction("perception", false, false))

	if responded == "" {
		t.Fatal("expected a response")
	}
	if !strings.Contains(responded, "Aria") {
		t.Errorf("expected Aria in response, got: %s", responded)
	}
	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected Perception in response, got: %s", responded)
	}
	// Roll logged
	if len(logger.logged) != 1 {
		t.Errorf("expected 1 roll logged, got %d", len(logger.logged))
	}
}

func TestCheckHandler_ExpertiseAndJackOfAllTrades(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	scores, _ := json.Marshal(character.AbilityScores{DEX: 14, CHA: 12}) // DEX +2, CHA +1
	profs, _ := json.Marshal(map[string]interface{}{
		"skills":             []string{"stealth", "perception"},
		"expertise":          []string{"stealth"},
		"jack_of_all_trades": true,
	})
	char := refdata.Character{
		ID:               uuid.New(),
		CampaignID:       campaignID,
		Name:             "Bard",
		Level:            5,
		ProficiencyBonus: 3,
		AbilityScores:    scores,
		Proficiencies:    pqtype.NullRawMessage{RawMessage: profs, Valid: true},
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	logger := &mockCheckRollLogger{}

	h := NewCheckHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		nil, nil,
		logger,
	)

	// Stealth with expertise: d20(10) + DEX(2) + expertise(3*2=6) = 18
	h.Handle(makeCheckInteraction("stealth", false, false))
	if !strings.Contains(responded, "18") {
		t.Errorf("expected total 18 with expertise, got: %s", responded)
	}

	// Persuasion (not proficient) with Jack of All Trades: d20(10) + CHA(1) + JoAT(3/2=1) = 12
	h.Handle(makeCheckInteraction("persuasion", false, false))
	if !strings.Contains(responded, "12") {
		t.Errorf("expected total 12 with jack of all trades, got: %s", responded)
	}
}

func TestCheckHandler_AdvantageFlag(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	h.Handle(makeCheckInteraction("athletics", true, false))

	if !strings.Contains(responded, "advantage") {
		t.Errorf("expected advantage in response, got: %s", responded)
	}
}

func TestCheckHandler_DisadvantageFlag(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	h.Handle(makeCheckInteraction("stealth", false, true))

	if !strings.Contains(responded, "disadvantage") {
		t.Errorf("expected disadvantage in response, got: %s", responded)
	}
}

func TestCheckHandler_RawAbilityCheck(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	h.Handle(makeCheckInteraction("str", false, false))

	if !strings.Contains(responded, "Str") {
		t.Errorf("expected Str in response, got: %s", responded)
	}
}

func TestCheckHandler_InvalidSkill(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	h.Handle(makeCheckInteraction("bogus", false, false))

	if !strings.Contains(responded, "unknown") && !strings.Contains(responded, "Unknown") {
		t.Errorf("expected error about unknown skill, got: %s", responded)
	}
}

func TestCheckHandler_BothAdvDisadv(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	h.Handle(makeCheckInteraction("perception", true, true))

	// Both adv and disadv cancel out
	if responded == "" {
		t.Fatal("expected a response")
	}
	// Should succeed, just with normal roll
	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected Perception in response, got: %s", responded)
	}
}

func TestCheckHandler_NoCampaign(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewCheckHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("no campaign")
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return refdata.Character{}, errors.New("no char")
		}},
		nil, nil, nil,
	)
	h.Handle(makeCheckInteraction("perception", false, false))

	if !strings.Contains(responded, "No campaign") {
		t.Errorf("expected no campaign message, got: %s", responded)
	}
}

func TestCheckHandler_NoCharacter(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewCheckHandler(
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
	h.Handle(makeCheckInteraction("perception", false, false))

	if !strings.Contains(responded, "register") || !strings.Contains(responded, "character") {
		t.Errorf("expected register prompt, got: %s", responded)
	}
}

func TestCheckHandler_WithCombatConditions(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	charID := uuid.New()
	encounterID := uuid.New()
	char := makeTestCharacter()
	char.ID = charID
	char.CampaignID = campaignID

	condJSON, _ := json.Marshal([]map[string]interface{}{
		{"condition": "poisoned"},
	})

	roller := dice.NewRoller(func(max int) int { return 10 })
	logger := &mockCheckRollLogger{}

	h := NewCheckHandler(
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

	h.Handle(makeCheckInteraction("athletics", false, false))

	if !strings.Contains(responded, "poisoned") {
		t.Errorf("expected poisoned in response, got: %s", responded)
	}
}

func TestSetCheckHandler_RoutesCommand(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	bot := &Bot{session: sess}
	router := NewCommandRouter(bot, nil)

	h, _ := setupCheckHandler(sess)
	router.SetCheckHandler(h)

	router.Handle(makeCheckInteraction("perception", false, false))

	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected Perception routed through check handler, got: %s", responded)
	}
}

func TestCheckHandler_CombatConditions_NoCombatant(t *testing.T) {
	// Character is not in combat (no matching combatant)
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	char := makeTestCharacter()
	char.CampaignID = campaignID
	encounterID := uuid.New()

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewCheckHandler(
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
			return []refdata.Combatant{}, nil // no combatants match
		}},
		nil,
	)

	h.Handle(makeCheckInteraction("perception", false, false))

	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected Perception in response (no combat conditions), got: %s", responded)
	}
}

func TestCheckHandler_BadProficienciesJSON(t *testing.T) {
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
	h := NewCheckHandler(
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

	h.Handle(makeCheckInteraction("perception", false, false))

	if !strings.Contains(responded, "Error") {
		t.Errorf("expected Error for bad JSON, got: %s", responded)
	}
}

func TestCheckHandler_CombatantLookupError(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	char := makeTestCharacter()
	char.CampaignID = campaignID
	encounterID := uuid.New()

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewCheckHandler(
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
			return nil, errors.New("db error")
		}},
		nil,
	)

	h.Handle(makeCheckInteraction("perception", false, false))

	// Should still succeed without conditions
	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected Perception (graceful fallback), got: %s", responded)
	}
}

func TestCheckHandler_NilEncounterProvider(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	char := makeTestCharacter()
	char.CampaignID = campaignID

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewCheckHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		nil, // no encounter provider
		nil, // no combatant lookup
		nil,
	)

	h.Handle(makeCheckInteraction("perception", false, false))

	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected Perception (no combat), got: %s", responded)
	}
}

func TestCheckHandler_BadAbilityScoresJSON(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	char := refdata.Character{
		ID:               uuid.New(),
		CampaignID:       campaignID,
		Name:             "Bad",
		Level:            5,
		ProficiencyBonus: 3,
		AbilityScores:    []byte(`{bad`),
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewCheckHandler(
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

	h.Handle(makeCheckInteraction("perception", false, false))

	if !strings.Contains(responded, "Error") {
		t.Errorf("expected Error for bad ability scores, got: %s", responded)
	}
}

func TestCheckHandler_NilCombatantLookup(t *testing.T) {
	// encounterProvider exists but combatantLookup is nil
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	char := makeTestCharacter()
	char.CampaignID = campaignID

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewCheckHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.New(), nil
		}},
		nil, // combatant lookup is nil
		nil,
	)

	h.Handle(makeCheckInteraction("perception", false, false))

	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected successful check, got: %s", responded)
	}
}

func TestCheckHandler_NoOptions(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "check",
			Options: nil,
		},
	}
	h.Handle(interaction)

	if responded == "" {
		t.Fatal("expected a response for no options")
	}
}

// --- med-32 / Phase 81: targeted contested check ---

type stubOpponentResolver struct {
	name      string
	modifier  int
	ok        bool
	seenSkill string
	seenID    string
}

func (s *stubOpponentResolver) ResolveContestedOpponent(_ context.Context, _ uuid.UUID, targetShortID, skill string) (string, int, bool) {
	s.seenSkill = skill
	s.seenID = targetShortID
	return s.name, s.modifier, s.ok
}

func TestCheckHandler_Target_RoutesToContestedCheck(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	// Ensure the active-encounter resolver succeeds so the contested path
	// engages.
	encID := uuid.New()
	h.encounterProvider = &mockCheckEncounterProvider{fnUser: func(_ context.Context, _, _ string) (uuid.UUID, error) {
		return encID, nil
	}}
	resolver := &stubOpponentResolver{name: "Goblin Boss", modifier: 2, ok: true}
	h.SetOpponentResolver(resolver)

	h.Handle(makeCheckInteraction("athletics", false, false, "G1"))

	if responded == "" {
		t.Fatal("expected a contested-check response")
	}
	if !strings.Contains(responded, "Contested Athletics") {
		t.Errorf("expected contested message, got: %s", responded)
	}
	if resolver.seenID != "G1" {
		t.Errorf("opponent resolver should see target id G1, got %q", resolver.seenID)
	}
	if resolver.seenSkill != "athletics" {
		t.Errorf("opponent resolver should see skill athletics, got %q", resolver.seenSkill)
	}
}

func TestCheckHandler_Target_NoOpponentResolver_FallsBackToSingleCheck(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	// Deliberately no SetOpponentResolver — should NOT engage contested path.

	h.Handle(makeCheckInteraction("perception", false, false, "G1"))

	if strings.Contains(responded, "Contested") {
		t.Errorf("expected single-check fallback, got contested: %s", responded)
	}
	if !strings.Contains(responded, "Aria") {
		t.Errorf("expected single-check response with character name, got: %s", responded)
	}
}

func TestCheckHandler_Target_OpponentNotResolved_FallsBackToSingleCheck(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	encID := uuid.New()
	h.encounterProvider = &mockCheckEncounterProvider{fnUser: func(_ context.Context, _, _ string) (uuid.UUID, error) {
		return encID, nil
	}}
	h.SetOpponentResolver(&stubOpponentResolver{ok: false})

	h.Handle(makeCheckInteraction("perception", false, false, "ZZ"))

	if strings.Contains(responded, "Contested") {
		t.Errorf("expected single-check fallback when opponent unresolved, got: %s", responded)
	}
}

// med-31 / Phase 75b: stealth checks made by a character wearing armor with
// stealth_disadv = true must be rolled at disadvantage. The handler resolves
// the equipped armor via CheckArmorLookup and applies dice.Disadvantage to
// the SingleCheck input automatically.
type stubArmorLookup struct {
	armor refdata.Armor
	err   error
	calls int
}

func (s *stubArmorLookup) GetArmor(_ context.Context, _ string) (refdata.Armor, error) {
	s.calls++
	return s.armor, s.err
}

func TestCheckHandler_Stealth_AppliesArmorDisadvantage(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	// Override the character lookup to return armor-equipped Aria.
	campaignID := uuid.New()
	char := makeTestCharacter()
	char.CampaignID = campaignID
	char.EquippedArmor = sql.NullString{String: "plate", Valid: true}
	h.campaignProvider = &mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
		return refdata.Campaign{ID: campaignID}, nil
	}}
	h.characterLookup = &mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
		return char, nil
	}}
	armor := &stubArmorLookup{armor: refdata.Armor{
		ID:            "plate",
		Name:          "Plate",
		StealthDisadv: sql.NullBool{Bool: true, Valid: true},
		ArmorType:     "heavy",
	}}
	h.SetArmorLookup(armor)

	h.Handle(makeCheckInteraction("stealth", false, false))

	if armor.calls != 1 {
		t.Fatalf("expected armor lookup once, got %d", armor.calls)
	}
	if !strings.Contains(strings.ToLower(responded), "disadvantage") {
		t.Errorf("expected disadvantage to be reflected in response, got: %s", responded)
	}
}

func TestCheckHandler_Stealth_NoArmor_NoLookup(t *testing.T) {
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
		return nil
	}

	h, _ := setupCheckHandler(sess)
	armor := &stubArmorLookup{}
	h.SetArmorLookup(armor)

	// Character returned by setupCheckHandler has no EquippedArmor — lookup
	// must not be called.
	h.Handle(makeCheckInteraction("stealth", false, false))

	if armor.calls != 0 {
		t.Fatalf("expected zero armor lookups when no armor equipped, got %d", armor.calls)
	}
}

func TestCheckHandler_Stealth_ArmorWithoutDisadv_NoEffect(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	campaignID := uuid.New()
	char := makeTestCharacter()
	char.CampaignID = campaignID
	char.EquippedArmor = sql.NullString{String: "leather", Valid: true}
	h.campaignProvider = &mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
		return refdata.Campaign{ID: campaignID}, nil
	}}
	h.characterLookup = &mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
		return char, nil
	}}
	h.SetArmorLookup(&stubArmorLookup{armor: refdata.Armor{
		ID:            "leather",
		Name:          "Leather",
		StealthDisadv: sql.NullBool{Bool: false, Valid: true},
		ArmorType:     "light",
	}})

	h.Handle(makeCheckInteraction("stealth", false, false))

	if strings.Contains(strings.ToLower(responded), "disadvantage") {
		t.Errorf("expected NO disadvantage for non-stealth-disadv armor, got: %s", responded)
	}
}

// --- F-81: targeted (non-contested) /check adjacency + action cost ---

type stubCheckTargetResolver struct {
	caster   refdata.Combatant
	target   refdata.Combatant
	ok       bool
	seenID   string
}

func (s *stubCheckTargetResolver) ResolveTargetCombatant(_ context.Context, _ uuid.UUID, _, targetShortID string) (refdata.Combatant, refdata.Combatant, bool) {
	s.seenID = targetShortID
	return s.caster, s.target, s.ok
}

type stubCheckTurnProvider struct {
	turn       refdata.Turn
	inCombat   bool
	updated    refdata.Turn
	updateErr  error
	getErr     error
	updateCalls int
}

func (s *stubCheckTurnProvider) GetActiveTurnForCharacter(_ context.Context, _ string, _ uuid.UUID) (refdata.Turn, bool, error) {
	if s.getErr != nil {
		return refdata.Turn{}, false, s.getErr
	}
	return s.turn, s.inCombat, nil
}

func (s *stubCheckTurnProvider) UpdateTurnActions(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	s.updateCalls++
	s.updated = refdata.Turn{
		ID:                  arg.ID,
		ActionUsed:          arg.ActionUsed,
		MovementRemainingFt: arg.MovementRemainingFt,
		BonusActionUsed:     arg.BonusActionUsed,
		ReactionUsed:        arg.ReactionUsed,
		FreeInteractUsed:    arg.FreeInteractUsed,
		AttacksRemaining:    arg.AttacksRemaining,
	}
	return s.updated, s.updateErr
}

func TestCheckHandler_TargetedCheck_RejectsNonAdjacentTarget(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	encID := uuid.New()
	h.encounterProvider = &mockCheckEncounterProvider{fnUser: func(_ context.Context, _, _ string) (uuid.UUID, error) {
		return encID, nil
	}}
	// Contested resolver returns ok=false so the targeted-non-contested path fires.
	h.SetOpponentResolver(&stubOpponentResolver{ok: false})
	h.SetTargetResolver(&stubCheckTargetResolver{
		caster: refdata.Combatant{PositionCol: "A", PositionRow: 1},
		target: refdata.Combatant{PositionCol: "F", PositionRow: 6, DisplayName: "Bjorn"},
		ok:     true,
	})

	h.Handle(makeCheckInteraction("medicine", false, false, "BJ"))

	if !strings.Contains(responded, "not adjacent") {
		t.Errorf("expected adjacency rejection, got: %s", responded)
	}
}

func TestCheckHandler_TargetedCheck_AdjacentInCombat_DeductsAction(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	encID := uuid.New()
	h.encounterProvider = &mockCheckEncounterProvider{fnUser: func(_ context.Context, _, _ string) (uuid.UUID, error) {
		return encID, nil
	}}
	h.SetOpponentResolver(&stubOpponentResolver{ok: false})
	h.SetTargetResolver(&stubCheckTargetResolver{
		caster: refdata.Combatant{PositionCol: "B", PositionRow: 2},
		target: refdata.Combatant{PositionCol: "B", PositionRow: 3, DisplayName: "Bjorn"},
		ok:     true,
	})
	turnProv := &stubCheckTurnProvider{
		turn:     refdata.Turn{ID: uuid.New(), ActionUsed: false},
		inCombat: true,
	}
	h.SetTurnProvider(turnProv)

	h.Handle(makeCheckInteraction("medicine", false, false, "BJ"))

	if !strings.Contains(responded, "Medicine") {
		t.Errorf("expected successful single-check response, got: %s", responded)
	}
	if turnProv.updateCalls != 1 {
		t.Fatalf("expected 1 UpdateTurnActions call, got %d", turnProv.updateCalls)
	}
	if !turnProv.updated.ActionUsed {
		t.Errorf("expected action_used=true after deduction")
	}
}

func TestCheckHandler_TargetedCheck_OutOfCombat_NoDeduction(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	encID := uuid.New()
	h.encounterProvider = &mockCheckEncounterProvider{fnUser: func(_ context.Context, _, _ string) (uuid.UUID, error) {
		return encID, nil
	}}
	h.SetOpponentResolver(&stubOpponentResolver{ok: false})
	h.SetTargetResolver(&stubCheckTargetResolver{
		caster: refdata.Combatant{PositionCol: "B", PositionRow: 2},
		target: refdata.Combatant{PositionCol: "B", PositionRow: 3, DisplayName: "Bjorn"},
		ok:     true,
	})
	// Turn provider reports not-in-combat.
	turnProv := &stubCheckTurnProvider{inCombat: false}
	h.SetTurnProvider(turnProv)

	h.Handle(makeCheckInteraction("medicine", false, false, "BJ"))

	if turnProv.updateCalls != 0 {
		t.Errorf("expected no UpdateTurnActions call out of combat, got %d", turnProv.updateCalls)
	}
	if !strings.Contains(responded, "Medicine") {
		t.Errorf("expected single-check response, got: %s", responded)
	}
}

func TestCheckHandler_TargetedCheck_ActionAlreadyUsed_Rejects(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	encID := uuid.New()
	h.encounterProvider = &mockCheckEncounterProvider{fnUser: func(_ context.Context, _, _ string) (uuid.UUID, error) {
		return encID, nil
	}}
	h.SetOpponentResolver(&stubOpponentResolver{ok: false})
	h.SetTargetResolver(&stubCheckTargetResolver{
		caster: refdata.Combatant{PositionCol: "B", PositionRow: 2},
		target: refdata.Combatant{PositionCol: "B", PositionRow: 3, DisplayName: "Bjorn"},
		ok:     true,
	})
	turnProv := &stubCheckTurnProvider{
		turn:     refdata.Turn{ID: uuid.New(), ActionUsed: true},
		inCombat: true,
	}
	h.SetTurnProvider(turnProv)

	h.Handle(makeCheckInteraction("medicine", false, false, "BJ"))

	if !strings.Contains(strings.ToLower(responded), "already used your action") {
		t.Errorf("expected action-spent rejection, got: %s", responded)
	}
}

func TestCheckHandler_TargetedCheck_NoResolverWired_FallsThrough(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	// no targetResolver, no opponentResolver — fall through to regular check
	h.Handle(makeCheckInteraction("perception", false, false, "ZZ"))
	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected fall-through single-check, got: %s", responded)
	}
}

// --- E-69: Obscurement-aware /check ---

type mockCheckZoneLookup struct {
	fn func(ctx context.Context, encounterID uuid.UUID) ([]combat.ZoneInfo, error)
}

func (m *mockCheckZoneLookup) ListZonesForEncounter(ctx context.Context, encounterID uuid.UUID) ([]combat.ZoneInfo, error) {
	return m.fn(ctx, encounterID)
}

// E-69: /check perception inside a heavily-obscured zone applies disadvantage
// from ObscurementCheckEffect and surfaces the lighting reason in the response.
func TestCheckHandler_Perception_InObscuredZone_AppliesDisadvantageAndReason(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	encounterID := uuid.New()
	char := makeTestCharacter()
	char.CampaignID = campaignID

	// Caster combatant at A1 (col index 0, row 0).
	casterCombatant := refdata.Combatant{
		ID:          uuid.New(),
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: char.ID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
		IsAlive:     true,
	}

	// Heavy-obscurement zone covering a 5ft square at A1.
	dims, _ := json.Marshal(map[string]int{"side_ft": 5})
	zone := combat.ZoneInfo{
		ID:          uuid.New(),
		EncounterID: encounterID,
		Shape:       "square",
		OriginCol:   "A",
		OriginRow:   1,
		Dimensions:  dims,
		ZoneType:    "heavy_obscurement",
	}

	roller := dice.NewRoller(func(max int) int { return 12 })
	logger := &mockCheckRollLogger{}

	h := NewCheckHandler(
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
			return []refdata.Combatant{casterCombatant}, nil
		}},
		logger,
	)

	// Wire the zone lookup so the obscurement check can compute the level.
	h.SetZoneLookup(&mockCheckZoneLookup{fn: func(_ context.Context, _ uuid.UUID) ([]combat.ZoneInfo, error) {
		return []combat.ZoneInfo{zone}, nil
	}})

	h.Handle(makeCheckInteraction("perception", false, false))

	if !strings.Contains(strings.ToLower(responded), "disadvantage") {
		t.Errorf("expected disadvantage from heavy obscurement, got: %s", responded)
	}
	if !strings.Contains(strings.ToLower(responded), "obscured") {
		t.Errorf("expected obscurement reason in response, got: %s", responded)
	}
}

// --- SR-022: /check uses beast STR/DEX/CON while Wild Shaped ---

type mockCheckCreatureLookup struct {
	fn func(ctx context.Context, id string) (refdata.Creature, error)
}

func (m *mockCheckCreatureLookup) GetCreature(ctx context.Context, id string) (refdata.Creature, error) {
	return m.fn(ctx, id)
}

// brownBearCreatureRow returns a refdata.Creature row matching the brown
// bear used by the SR-022 wildshape_test cases (STR 19, DEX 10, CON 16).
func brownBearCreatureRow() refdata.Creature {
	return refdata.Creature{
		ID:            "brown-bear",
		Name:          "Brown Bear",
		Type:          "beast",
		AbilityScores: json.RawMessage(`{"str":19,"dex":10,"con":16,"int":2,"wis":13,"cha":7}`),
	}
}

// TestCheckHandler_WildShaped_AthleticsUsesBeastStr — a STR-8 druid Wild
// Shaped into a brown bear (STR 19, +4 mod) must roll /check athletics with
// bear STR, not druid STR. d20 is pinned to 10 so the only variable in the
// printed total is the STR modifier. Expected: 10 + 4 = 14, not 10 - 1 = 9.
func TestCheckHandler_WildShaped_AthleticsUsesBeastStr(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	encounterID := uuid.New()
	charID := uuid.New()

	// Druid scores: STR 8 (-1) but the player should roll with bear STR.
	scores, _ := json.Marshal(character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 16, WIS: 18, CHA: 10})
	char := refdata.Character{
		ID:               charID,
		CampaignID:       campaignID,
		Name:             "Elara",
		Level:            4,
		ProficiencyBonus: 2,
		AbilityScores:    scores,
	}

	combatant := refdata.Combatant{
		ID:                   uuid.New(),
		EncounterID:          encounterID,
		CharacterID:          uuid.NullUUID{UUID: charID, Valid: true},
		IsWildShaped:         true,
		WildShapeCreatureRef: sql.NullString{String: "brown-bear", Valid: true},
		Conditions:           json.RawMessage(`[]`),
	}

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 10
		}
		return 1
	})

	h := NewCheckHandler(
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
			return []refdata.Combatant{combatant}, nil
		}},
		&mockCheckRollLogger{},
	)
	h.SetCreatureLookup(&mockCheckCreatureLookup{fn: func(_ context.Context, id string) (refdata.Creature, error) {
		if id == "brown-bear" {
			return brownBearCreatureRow(), nil
		}
		return refdata.Creature{}, sql.ErrNoRows
	}})

	h.Handle(makeCheckInteraction("athletics", false, false))

	// 10 + 4 (bear STR mod) = 14. Druid STR alone would yield 10 - 1 = 9.
	if !strings.Contains(responded, "14") {
		t.Fatalf("expected athletics total 14 from bear STR, got: %s", responded)
	}
	if strings.Contains(responded, " 9 ") {
		t.Fatalf("druid STR (-1) leaked into the roll: %s", responded)
	}
}

// TestCheckHandler_NotWildShaped_UsesDruidStr — sanity check: with the
// creature lookup wired but the player not Wild Shaped, the druid's own
// STR is used (10 + (-1) = 9).
func TestCheckHandler_NotWildShaped_UsesDruidStr(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	encounterID := uuid.New()
	charID := uuid.New()

	scores, _ := json.Marshal(character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 16, WIS: 18, CHA: 10})
	char := refdata.Character{
		ID:               charID,
		CampaignID:       campaignID,
		Name:             "Elara",
		Level:            4,
		ProficiencyBonus: 2,
		AbilityScores:    scores,
	}

	// IsWildShaped: false — the lookup should never be consulted.
	combatant := refdata.Combatant{
		ID:          uuid.New(),
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	creatureCalled := false
	h := NewCheckHandler(
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
			return []refdata.Combatant{combatant}, nil
		}},
		&mockCheckRollLogger{},
	)
	h.SetCreatureLookup(&mockCheckCreatureLookup{fn: func(_ context.Context, _ string) (refdata.Creature, error) {
		creatureCalled = true
		return brownBearCreatureRow(), nil
	}})

	h.Handle(makeCheckInteraction("athletics", false, false))

	if creatureCalled {
		t.Fatalf("creature lookup must not be consulted when player is not Wild Shaped")
	}
	// 10 - 1 (druid STR) = 9.
	if !strings.Contains(responded, " 9") {
		t.Fatalf("expected druid athletics total 9, got: %s", responded)
	}
}

// TestCheckHandler_WildShaped_BeastLookupFails_FallsBackToDruid — when the
// beast can't be resolved (e.g. database hiccup), /check must degrade to
// druid scores rather than rejecting the roll (SR-006 silent-fallback).
func TestCheckHandler_WildShaped_BeastLookupFails_FallsBackToDruid(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	encounterID := uuid.New()
	charID := uuid.New()

	scores, _ := json.Marshal(character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 16, WIS: 18, CHA: 10})
	char := refdata.Character{
		ID: charID, CampaignID: campaignID, Name: "Elara",
		Level: 4, ProficiencyBonus: 2, AbilityScores: scores,
	}

	combatant := refdata.Combatant{
		ID:                   uuid.New(),
		EncounterID:          encounterID,
		CharacterID:          uuid.NullUUID{UUID: charID, Valid: true},
		IsWildShaped:         true,
		WildShapeCreatureRef: sql.NullString{String: "ghost-beast", Valid: true},
		Conditions:           json.RawMessage(`[]`),
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	h := NewCheckHandler(
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
			return []refdata.Combatant{combatant}, nil
		}},
		&mockCheckRollLogger{},
	)
	h.SetCreatureLookup(&mockCheckCreatureLookup{fn: func(_ context.Context, _ string) (refdata.Creature, error) {
		return refdata.Creature{}, errors.New("not found")
	}})

	h.Handle(makeCheckInteraction("athletics", false, false))

	// Beast lookup failed, fall back to druid STR (-1): 10 - 1 = 9.
	if !strings.Contains(responded, " 9") {
		t.Fatalf("expected druid fallback total 9 on lookup error, got: %s", responded)
	}
}

// --- G-H04: medicine check validates dying state and auto-stabilizes ---

type stubCheckStabilizeStore struct {
	called bool
	arg    refdata.UpdateCombatantDeathSavesParams
}

func (s *stubCheckStabilizeStore) UpdateCombatantDeathSaves(_ context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
	s.called = true
	s.arg = arg
	return refdata.Combatant{}, nil
}

func TestCheckHandler_MedicineTarget_DyingTarget_Stabilizes(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	// Roller returns 12 → total = 12 + WIS(4) + prof(3) = 19 >= 10 → success
	h, _ := setupCheckHandler(sess)
	encID := uuid.New()
	h.encounterProvider = &mockCheckEncounterProvider{fnUser: func(_ context.Context, _, _ string) (uuid.UUID, error) {
		return encID, nil
	}}
	h.SetOpponentResolver(&stubOpponentResolver{ok: false})

	targetID := uuid.New()
	deathSaves, _ := json.Marshal(combat.DeathSaves{Successes: 0, Failures: 1})
	h.SetTargetResolver(&stubCheckTargetResolver{
		caster: refdata.Combatant{PositionCol: "B", PositionRow: 2},
		target: refdata.Combatant{
			ID:          targetID,
			PositionCol: "B", PositionRow: 3,
			DisplayName: "Bjorn",
			HpCurrent:   0,
			IsAlive:     true,
			DeathSaves:  pqtype.NullRawMessage{RawMessage: deathSaves, Valid: true},
		},
		ok: true,
	})

	store := &stubCheckStabilizeStore{}
	h.SetStabilizeStore(store)

	h.Handle(makeCheckInteraction("medicine", false, false, "BJ"))

	if !store.called {
		t.Fatal("expected stabilize store to be called on successful medicine check against dying target")
	}
	if store.arg.ID != targetID {
		t.Errorf("expected stabilize for target %v, got %v", targetID, store.arg.ID)
	}
	if !strings.Contains(responded, "stabilize") && !strings.Contains(responded, "stabilized") {
		t.Errorf("expected stabilization message in response, got: %s", responded)
	}
}

func TestCheckHandler_MedicineTarget_NotDying_Rejects(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandler(sess)
	encID := uuid.New()
	h.encounterProvider = &mockCheckEncounterProvider{fnUser: func(_ context.Context, _, _ string) (uuid.UUID, error) {
		return encID, nil
	}}
	h.SetOpponentResolver(&stubOpponentResolver{ok: false})
	h.SetTargetResolver(&stubCheckTargetResolver{
		caster: refdata.Combatant{PositionCol: "B", PositionRow: 2},
		target: refdata.Combatant{
			PositionCol: "B", PositionRow: 3,
			DisplayName: "Bjorn",
			HpCurrent:   15, // not dying
			IsAlive:     true,
		},
		ok: true,
	})

	store := &stubCheckStabilizeStore{}
	h.SetStabilizeStore(store)

	h.Handle(makeCheckInteraction("medicine", false, false, "BJ"))

	if store.called {
		t.Fatal("stabilize store should NOT be called for non-dying target")
	}
	if !strings.Contains(responded, "not dying") {
		t.Errorf("expected 'not dying' rejection, got: %s", responded)
	}
}
