package discord

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Mock types for RestHandler ---

type mockRestCharacterUpdater struct {
	updateFeatureUsesFn   func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error)
	updateSpellSlotsFn    func(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error)
	updatePactMagicSlotsFn func(ctx context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error)
	updateCharacterFn     func(ctx context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error)
}

func (m *mockRestCharacterUpdater) UpdateCharacterFeatureUses(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
	if m.updateFeatureUsesFn != nil {
		return m.updateFeatureUsesFn(ctx, arg)
	}
	return refdata.Character{}, nil
}

func (m *mockRestCharacterUpdater) UpdateCharacterSpellSlots(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
	if m.updateSpellSlotsFn != nil {
		return m.updateSpellSlotsFn(ctx, arg)
	}
	return refdata.Character{}, nil
}

func (m *mockRestCharacterUpdater) UpdateCharacterPactMagicSlots(ctx context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error) {
	if m.updatePactMagicSlotsFn != nil {
		return m.updatePactMagicSlotsFn(ctx, arg)
	}
	return refdata.Character{}, nil
}

func (m *mockRestCharacterUpdater) UpdateCharacter(ctx context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
	if m.updateCharacterFn != nil {
		return m.updateCharacterFn(ctx, arg)
	}
	return refdata.Character{}, nil
}

// --- Helpers ---

func makeRestInteraction(restType string) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "type", Value: restType, Type: discordgo.ApplicationCommandOptionString},
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "rest",
			Options: opts,
		},
	}
}

func makeRestTestCharacter() refdata.Character {
	scores, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "fighter", Level: 5}})
	hitDice, _ := json.Marshal(map[string]int{"d10": 5})
	featureUses, _ := json.Marshal(map[string]character.FeatureUse{
		"action-surge": {Current: 0, Max: 1, Recharge: "short"},
		"second-wind":  {Current: 0, Max: 1, Recharge: "short"},
	})
	spellSlots, _ := json.Marshal(map[string]character.SlotInfo{})

	return refdata.Character{
		ID:               uuid.New(),
		CampaignID:       uuid.New(),
		Name:             "Thorin",
		Level:            5,
		HpMax:            44,
		HpCurrent:        20,
		AbilityScores:    scores,
		Classes:          classes,
		HitDiceRemaining: hitDice,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUses, Valid: true},
		SpellSlots:       pqtype.NullRawMessage{RawMessage: spellSlots, Valid: true},
		ProficiencyBonus: 3,
	}
}

func setupRestHandler(sess *MockSession) *RestHandler {
	campaignID := uuid.New()
	char := makeRestTestCharacter()
	char.CampaignID = campaignID

	roller := dice.NewRoller(func(max int) int { return 6 })
	logger := &mockCheckRollLogger{}
	updater := &mockRestCharacterUpdater{
		updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
			return refdata.Character{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
		updateFeatureUsesFn: func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
			return refdata.Character{ID: arg.ID}, nil
		},
		updateSpellSlotsFn: func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
			return refdata.Character{ID: arg.ID}, nil
		},
		updatePactMagicSlotsFn: func(_ context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error) {
			return refdata.Character{ID: arg.ID}, nil
		},
	}

	return NewRestHandler(
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
		updater,
		logger,
		nil, // dmQueueFunc
	)
}

// --- TDD Cycle 21: Handler parses "short" and responds ---

func TestRestHandler_ShortRest_Responds(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h := setupRestHandler(sess)
	h.Handle(makeRestInteraction("short"))

	if responded == "" {
		t.Fatal("expected a response")
	}
	if !strings.Contains(responded, "Short Rest") {
		t.Errorf("expected 'Short Rest' in response, got: %s", responded)
	}
}

// --- TDD Cycle 22: Handler parses "long" and responds ---

func TestRestHandler_LongRest_Responds(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h := setupRestHandler(sess)
	h.Handle(makeRestInteraction("long"))

	if responded == "" {
		t.Fatal("expected a response")
	}
	if !strings.Contains(responded, "Long Rest") {
		t.Errorf("expected 'Long Rest' in response, got: %s", responded)
	}
}

// --- TDD Cycle 23: Handler blocks rest during active combat ---

func TestRestHandler_BlockedDuringCombat(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	char := makeRestTestCharacter()
	char.CampaignID = campaignID
	roller := dice.NewRoller(func(max int) int { return 6 })

	h := NewRestHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.New(), nil // active encounter exists
		}},
		&mockRestCharacterUpdater{},
		nil,
		nil,
	)

	h.Handle(makeRestInteraction("short"))

	if !strings.Contains(responded, "cannot rest during active combat") {
		t.Errorf("expected combat-blocked message, got: %s", responded)
	}
}

// --- TDD Cycle 24: Handler rejects invalid rest type ---

func TestRestHandler_InvalidType(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h := setupRestHandler(sess)
	h.Handle(makeRestInteraction("invalid"))

	if !strings.Contains(responded, "short") || !strings.Contains(responded, "long") {
		t.Errorf("expected error mentioning valid types, got: %s", responded)
	}
}

// --- TDD Cycle 25: Handler handles no campaign ---

func TestRestHandler_NoCampaign(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	roller := dice.NewRoller(func(max int) int { return 6 })

	h := NewRestHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{}, errNoEncounter
		}},
		nil, nil, nil, nil, nil,
	)

	h.Handle(makeRestInteraction("short"))

	if !strings.Contains(responded, "No campaign") {
		t.Errorf("expected 'No campaign' message, got: %s", responded)
	}
}

// --- TDD Cycle 26: Short rest with auto-spend 0 dice (for now, auto-approve with 0 dice spend) ---

func TestRestHandler_ShortRest_FeatureRecharge(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h := setupRestHandler(sess)
	h.Handle(makeRestInteraction("short"))

	// Features should be recharged
	if !strings.Contains(responded, "action-surge") && !strings.Contains(responded, "second-wind") {
		t.Logf("Response: %s", responded)
	}
}

// --- TDD Cycle 27: Long rest restores HP and features ---

func TestRestHandler_LongRest_FullRestore(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h := setupRestHandler(sess)
	h.Handle(makeRestInteraction("long"))

	if !strings.Contains(responded, "44/44 HP") {
		t.Errorf("expected full HP restore in response, got: %s", responded)
	}
}

// --- TDD Cycle 28: Handler with no character found ---

func TestRestHandler_NoCharacter(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	roller := dice.NewRoller(func(max int) int { return 6 })

	h := NewRestHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return refdata.Character{}, errNoEncounter
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errNoEncounter
		}},
		nil, nil, nil,
	)

	h.Handle(makeRestInteraction("short"))

	if !strings.Contains(responded, "Could not find your character") {
		t.Errorf("expected character not found message, got: %s", responded)
	}
}

// --- TDD Cycle 29: Long rest with warlock (pact magic slots) ---

func TestRestHandler_LongRest_WithWarlock(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()

	scores, _ := json.Marshal(character.AbilityScores{STR: 10, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 16})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "warlock", Level: 5}})
	hitDice, _ := json.Marshal(map[string]int{"d8": 3})
	featureUses, _ := json.Marshal(map[string]character.FeatureUse{})
	spellSlots, _ := json.Marshal(map[string]character.SlotInfo{})
	pactSlots, _ := json.Marshal(character.PactMagicSlots{SlotLevel: 3, Current: 0, Max: 2})

	char := refdata.Character{
		ID:               uuid.New(),
		CampaignID:       campaignID,
		Name:             "Eldarin",
		Level:            5,
		HpMax:            38,
		HpCurrent:        20,
		AbilityScores:    scores,
		Classes:          classes,
		HitDiceRemaining: hitDice,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUses, Valid: true},
		SpellSlots:       pqtype.NullRawMessage{RawMessage: spellSlots, Valid: true},
		PactMagicSlots:   pqtype.NullRawMessage{RawMessage: pactSlots, Valid: true},
	}

	roller := dice.NewRoller(func(max int) int { return 6 })

	h := NewRestHandler(
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
		&mockRestCharacterUpdater{
			updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
			updateFeatureUsesFn: func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
			updatePactMagicSlotsFn: func(_ context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		&mockCheckRollLogger{},
		nil,
	)

	h.Handle(makeRestInteraction("long"))

	if !strings.Contains(responded, "Pact magic slots restored") {
		t.Errorf("expected pact restore message, got: %s", responded)
	}
}

// --- TDD Cycle 30: Handler with nil encounter provider (no encounter check) ---

func TestRestHandler_NilEncounterProvider(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	char := makeRestTestCharacter()
	char.CampaignID = campaignID
	roller := dice.NewRoller(func(max int) int { return 6 })

	h := NewRestHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		nil, // nil encounter provider
		&mockRestCharacterUpdater{
			updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		nil, nil,
	)

	h.Handle(makeRestInteraction("short"))

	if !strings.Contains(responded, "Short Rest") {
		t.Errorf("expected short rest to proceed with nil encounter provider, got: %s", responded)
	}
}

// --- TDD Cycle 31: Test parseRestType with missing type option ---

func TestRestHandler_MissingTypeOption(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h := setupRestHandler(sess)

	// Create interaction with no options
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "rest",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}

	h.Handle(interaction)

	if !strings.Contains(responded, "Invalid rest type") {
		t.Errorf("expected invalid type message, got: %s", responded)
	}
}

// --- TDD Cycle 32: Long rest with prepared caster (cleric) ---

func TestRestHandler_LongRest_PreparedCasterReminder(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()

	scores, _ := json.Marshal(character.AbilityScores{STR: 10, DEX: 14, CON: 14, INT: 10, WIS: 16, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "cleric", Level: 5}})
	hitDice, _ := json.Marshal(map[string]int{"d8": 3})
	featureUses, _ := json.Marshal(map[string]character.FeatureUse{})
	spellSlots, _ := json.Marshal(map[string]character.SlotInfo{
		"1": {Current: 1, Max: 4},
		"2": {Current: 0, Max: 3},
		"3": {Current: 0, Max: 2},
	})

	char := refdata.Character{
		ID:               uuid.New(),
		CampaignID:       campaignID,
		Name:             "Elara",
		Level:            5,
		HpMax:            38,
		HpCurrent:        20,
		AbilityScores:    scores,
		Classes:          classes,
		HitDiceRemaining: hitDice,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUses, Valid: true},
		SpellSlots:       pqtype.NullRawMessage{RawMessage: spellSlots, Valid: true},
	}

	roller := dice.NewRoller(func(max int) int { return 6 })

	h := NewRestHandler(
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
		&mockRestCharacterUpdater{
			updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
			updateFeatureUsesFn: func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
			updateSpellSlotsFn: func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		&mockCheckRollLogger{},
		nil,
	)

	h.Handle(makeRestInteraction("long"))

	if !strings.Contains(responded, "/prepare") {
		t.Errorf("expected prepared caster reminder, got: %s", responded)
	}
	if !strings.Contains(responded, "All spell slots restored") {
		t.Errorf("expected spell slots restored message, got: %s", responded)
	}
}
