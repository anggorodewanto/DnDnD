package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/rest"
)

// --- Mock types for RestHandler ---

type mockRestCharacterUpdater struct {
	updateCharacterFn func(ctx context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error)
}

func (m *mockRestCharacterUpdater) UpdateCharacter(ctx context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
	if m.updateCharacterFn != nil {
		return m.updateCharacterFn(ctx, arg)
	}
	return refdata.Character{}, nil
}

type fakeRestExhaustionStore struct {
	combatantID     uuid.UUID
	conditions      []byte
	exhaustionLevel int
	updated         bool
	updatedLevel    int
}

func (f *fakeRestExhaustionStore) ActiveCombatantExhaustionForCharacter(_ context.Context, _ uuid.UUID) (rest.CombatantExhaustionState, bool, error) {
	return rest.CombatantExhaustionState{
		ID:              f.combatantID,
		Conditions:      f.conditions,
		ExhaustionLevel: f.exhaustionLevel,
	}, true, nil
}

func (f *fakeRestExhaustionStore) UpdateCombatantExhaustion(_ context.Context, _ uuid.UUID, _ []byte, level int) error {
	f.updated = true
	f.updatedLevel = level
	f.exhaustionLevel = level
	return nil
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

// TestRestHandler_PostsToDMQueueViaNotifier verifies that /rest posts a
// rest-request notification to the dmqueue Notifier when one is wired.
// SR-002: also pins the CampaignID payload so PgStore.Insert can persist
// the row instead of failing with "parse campaign id" after the Discord
// message is sent.
func TestRestHandler_PostsToDMQueueViaNotifier(t *testing.T) {
	sess := newTestMock()
	sess.InteractionRespondFunc = func(*discordgo.Interaction, *discordgo.InteractionResponse) error {
		return nil
	}

	campaignID := uuid.New()
	char := makeRestTestCharacter()
	char.CampaignID = campaignID
	roller := dice.NewRoller(func(max int) int { return 6 })
	updater := &mockRestCharacterUpdater{
		updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
			return refdata.Character{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
	}
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
		updater,
		&mockCheckRollLogger{},
		nil,
	)
	rec := &recordingNotifier{}
	h.SetNotifier(rec)
	h.Handle(makeRestInteraction("short"))

	if len(rec.posted) != 1 {
		t.Fatalf("expected 1 notifier post, got %d", len(rec.posted))
	}
	ev := rec.posted[0]
	if ev.Kind != dmqueue.KindRestRequest {
		t.Errorf("kind = %q want rest_request", ev.Kind)
	}
	if !strings.Contains(ev.Summary, "short") {
		t.Errorf("summary missing 'short': %q", ev.Summary)
	}
	if ev.GuildID != "guild1" {
		t.Errorf("guild = %q want guild1", ev.GuildID)
	}
	// SR-002: CampaignID must be set so PgStore.Insert succeeds.
	if ev.CampaignID != campaignID.String() {
		t.Errorf("CampaignID = %q want %q (SR-002)", ev.CampaignID, campaignID.String())
	}
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
	var editContent string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		return nil
	}
	sess.InteractionResponseEditFunc = func(_ *discordgo.Interaction, edit *discordgo.WebhookEdit) (*discordgo.Message, error) {
		if edit.Content != nil {
			editContent = *edit.Content
		}
		return &discordgo.Message{}, nil
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
			return uuid.Nil, errNoEncounter
		}},
		&mockRestCharacterUpdater{
			updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		&mockCheckRollLogger{},
		nil,
	)

	// Simulate button click: spend 0 dice (skip) to trigger features
	componentInteraction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "rest_hitdice:" + char.ID.String() + ":d10:0",
		},
	}

	h.HandleHitDiceComponent(componentInteraction)

	// Features should be recharged in the edit response
	if !strings.Contains(editContent, "action-surge") {
		t.Errorf("expected 'action-surge' in response, got: %s", editContent)
	}
	if !strings.Contains(editContent, "second-wind") {
		t.Errorf("expected 'second-wind' in response, got: %s", editContent)
	}
}

// --- TDD Cycle 35: Long rest persistLongRest uses correct new values in single UpdateCharacter call ---

func TestRestHandler_LongRest_PersistCorrectValues(t *testing.T) {
	var responded string
	var capturedParams refdata.UpdateCharacterParams
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()

	scores, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "fighter", Level: 5}})
	hitDice, _ := json.Marshal(map[string]int{"d10": 2})
	featureUses, _ := json.Marshal(map[string]character.FeatureUse{
		"action-surge": {Current: 0, Max: 1, Recharge: "short"},
	})
	spellSlots, _ := json.Marshal(map[string]character.SlotInfo{
		"1": {Current: 0, Max: 4},
	})
	pactSlots, _ := json.Marshal(character.PactMagicSlots{SlotLevel: 3, Current: 0, Max: 2})

	char := refdata.Character{
		ID:               uuid.New(),
		CampaignID:       campaignID,
		Name:             "Thorin",
		Level:            5,
		HpMax:            44,
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
				capturedParams = arg
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		&mockCheckRollLogger{},
		nil,
	)

	h.Handle(makeRestInteraction("long"))

	if responded == "" {
		t.Fatal("expected a response")
	}

	// HP should be set to HPMax
	if capturedParams.HpCurrent != 44 {
		t.Errorf("HpCurrent = %d, want 44", capturedParams.HpCurrent)
	}

	// Feature uses should be recharged (marshaled new values)
	var persistedFeatures map[string]character.FeatureUse
	if capturedParams.FeatureUses.Valid {
		if err := json.Unmarshal(capturedParams.FeatureUses.RawMessage, &persistedFeatures); err != nil {
			t.Fatalf("unmarshal feature uses: %v", err)
		}
		as, ok := persistedFeatures["action-surge"]
		if !ok {
			t.Error("action-surge not in persisted features")
		} else if as.Current != 1 {
			t.Errorf("action-surge.Current = %d, want 1 (recharged)", as.Current)
		}
	} else {
		t.Error("FeatureUses not valid in UpdateCharacterParams")
	}

	// Spell slots should be restored
	var persistedSlots map[string]character.SlotInfo
	if capturedParams.SpellSlots.Valid {
		if err := json.Unmarshal(capturedParams.SpellSlots.RawMessage, &persistedSlots); err != nil {
			t.Fatalf("unmarshal spell slots: %v", err)
		}
		slot1, ok := persistedSlots["1"]
		if !ok {
			t.Error("spell slot level 1 not in persisted slots")
		} else if slot1.Current != 4 {
			t.Errorf("slot1.Current = %d, want 4 (restored)", slot1.Current)
		}
	} else {
		t.Error("SpellSlots not valid in UpdateCharacterParams")
	}

	// Pact magic slots should be restored
	if capturedParams.PactMagicSlots.Valid {
		var persistedPact character.PactMagicSlots
		if err := json.Unmarshal(capturedParams.PactMagicSlots.RawMessage, &persistedPact); err != nil {
			t.Fatalf("unmarshal pact slots: %v", err)
		}
		if persistedPact.Current != 2 {
			t.Errorf("pact.Current = %d, want 2 (restored)", persistedPact.Current)
		}
	} else {
		t.Error("PactMagicSlots not valid in UpdateCharacterParams")
	}

	// Hit dice should be restored (fighter 5, had 2, restore 2 = 4)
	var persistedHitDice map[string]int
	if err := json.Unmarshal(capturedParams.HitDiceRemaining, &persistedHitDice); err != nil {
		t.Fatalf("unmarshal hit dice: %v", err)
	}
	if persistedHitDice["d10"] != 4 {
		t.Errorf("hit dice d10 = %d, want 4", persistedHitDice["d10"])
	}
}

// --- TDD Cycle 36: Short rest sends hit dice button prompt ---

func TestRestHandler_ShortRest_HitDicePrompt(t *testing.T) {
	var responded *discordgo.InteractionResponse
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp
		return nil
	}

	h := setupRestHandler(sess)
	h.Handle(makeRestInteraction("short"))

	if responded == nil {
		t.Fatal("expected a response")
	}

	// Should have action row components for hit dice selection
	if len(responded.Data.Components) == 0 {
		t.Error("expected action row components for hit dice buttons")
	}

	// Should contain the hit dice info text
	if !strings.Contains(responded.Data.Content, "hit dice") {
		t.Errorf("expected 'hit dice' in prompt message, got: %s", responded.Data.Content)
	}
}

// --- TDD Cycle 37: Short rest hit dice component handler applies healing ---

func TestRestHandler_HitDiceComponent_AppliesHealing(t *testing.T) {
	var updatedParams refdata.UpdateCharacterParams
	var finalResponse *discordgo.WebhookEdit
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		return nil
	}
	sess.InteractionResponseEditFunc = func(_ *discordgo.Interaction, edit *discordgo.WebhookEdit) (*discordgo.Message, error) {
		finalResponse = edit
		return &discordgo.Message{}, nil
	}

	campaignID := uuid.New()
	charID := uuid.New()
	scores, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "fighter", Level: 5}})
	hitDice, _ := json.Marshal(map[string]int{"d10": 5})
	featureUses, _ := json.Marshal(map[string]character.FeatureUse{
		"action-surge": {Current: 0, Max: 1, Recharge: "short"},
	})
	spellSlots, _ := json.Marshal(map[string]character.SlotInfo{})

	char := refdata.Character{
		ID:               charID,
		CampaignID:       campaignID,
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

	roller := dice.NewRoller(func(max int) int { return 6 }) // always rolls 6

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
				updatedParams = arg
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		&mockCheckRollLogger{},
		nil,
	)

	// Simulate button click: spend 2 d10 hit dice
	componentInteraction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "rest_hitdice:" + charID.String() + ":d10:2",
		},
	}

	h.HandleHitDiceComponent(componentInteraction)

	// Should have updated HP: 20 + 2*(6+2) = 36
	if updatedParams.HpCurrent != 36 {
		t.Errorf("HpCurrent = %d, want 36", updatedParams.HpCurrent)
	}

	// Should have response edit showing healing results
	if finalResponse == nil {
		t.Fatal("expected edit response for hit dice result")
	}
	if !strings.Contains(*finalResponse.Content, "Short Rest") {
		t.Errorf("expected 'Short Rest' in final message, got: %s", *finalResponse.Content)
	}
}

// --- TDD Cycle 38: Short rest skip button (spend 0) ---

func TestRestHandler_HitDiceComponent_SpendZero(t *testing.T) {
	var finalResponse *discordgo.WebhookEdit
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		return nil
	}
	sess.InteractionResponseEditFunc = func(_ *discordgo.Interaction, edit *discordgo.WebhookEdit) (*discordgo.Message, error) {
		finalResponse = edit
		return &discordgo.Message{}, nil
	}

	campaignID := uuid.New()
	charID := uuid.New()
	scores, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "fighter", Level: 5}})
	hitDice, _ := json.Marshal(map[string]int{"d10": 5})
	featureUses, _ := json.Marshal(map[string]character.FeatureUse{
		"action-surge": {Current: 0, Max: 1, Recharge: "short"},
	})
	spellSlots, _ := json.Marshal(map[string]character.SlotInfo{})

	char := refdata.Character{
		ID:               charID,
		CampaignID:       campaignID,
		Name:             "Thorin",
		Level:            5,
		HpMax:            44,
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
		},
		&mockCheckRollLogger{},
		nil,
	)

	componentInteraction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "rest_hitdice:" + charID.String() + ":d10:0",
		},
	}

	h.HandleHitDiceComponent(componentInteraction)

	if finalResponse == nil {
		t.Fatal("expected edit response")
	}
	if !strings.Contains(*finalResponse.Content, "Short Rest") {
		t.Errorf("expected 'Short Rest' in message, got: %s", *finalResponse.Content)
	}
	if !strings.Contains(*finalResponse.Content, "action-surge") {
		t.Errorf("expected feature recharge in message, got: %s", *finalResponse.Content)
	}
}

// --- TDD Cycle 39: BuildHitDiceButtons creates proper button layout ---

func TestBuildHitDiceButtons_SingleClass(t *testing.T) {
	hitDice := map[string]int{"d10": 5}
	charID := uuid.New()
	components := BuildHitDiceButtons(charID, hitDice)

	if len(components) == 0 {
		t.Fatal("expected at least one action row")
	}

	// Should have buttons from 0 to 5
	row := components[0]
	actionsRow, ok := row.(*discordgo.ActionsRow)
	if !ok {
		t.Fatal("expected ActionsRow component")
	}
	if len(actionsRow.Components) < 2 {
		t.Errorf("expected at least 2 buttons (0 and more), got %d", len(actionsRow.Components))
	}
}

func TestBuildHitDiceButtons_Multiclass(t *testing.T) {
	hitDice := map[string]int{"d10": 3, "d8": 2}
	charID := uuid.New()
	components := BuildHitDiceButtons(charID, hitDice)

	// Should have separate rows per die type
	if len(components) < 2 {
		t.Errorf("expected at least 2 action rows for multiclass, got %d", len(components))
	}
}

// --- TDD Cycle 40: ParseHitDiceCustomID error cases ---

func TestParseHitDiceCustomID_InvalidFormat(t *testing.T) {
	_, _, _, err := ParseHitDiceCustomID("invalid")
	if err == nil {
		t.Error("expected error for invalid custom ID")
	}
}

func TestParseHitDiceCustomID_InvalidCharID(t *testing.T) {
	_, _, _, err := ParseHitDiceCustomID("rest_hitdice:bad-id:d10:2")
	if err == nil {
		t.Error("expected error for invalid character ID")
	}
}

func TestParseHitDiceCustomID_InvalidCount(t *testing.T) {
	_, _, _, err := ParseHitDiceCustomID("rest_hitdice:" + uuid.New().String() + ":d10:abc")
	if err == nil {
		t.Error("expected error for invalid count")
	}
}

func TestParseHitDiceCustomID_Valid(t *testing.T) {
	id := uuid.New()
	charID, dieType, count, err := ParseHitDiceCustomID("rest_hitdice:" + id.String() + ":d10:3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if charID != id {
		t.Errorf("charID = %s, want %s", charID, id)
	}
	if dieType != "d10" {
		t.Errorf("dieType = %s, want d10", dieType)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

// --- TDD Cycle 41: HandleHitDiceComponent wrong character ---

func TestRestHandler_HitDiceComponent_WrongCharacter(t *testing.T) {
	var editContent string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		return nil
	}
	sess.InteractionResponseEditFunc = func(_ *discordgo.Interaction, edit *discordgo.WebhookEdit) (*discordgo.Message, error) {
		if edit.Content != nil {
			editContent = *edit.Content
		}
		return &discordgo.Message{}, nil
	}

	h := setupRestHandler(sess)

	// Use a different char ID than what the handler finds
	otherID := uuid.New()
	componentInteraction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "rest_hitdice:" + otherID.String() + ":d10:1",
		},
	}

	h.HandleHitDiceComponent(componentInteraction)

	if !strings.Contains(editContent, "not for your character") {
		t.Errorf("expected 'not for your character' message, got: %s", editContent)
	}
}

// --- Multiclass multi-step hit dice: first click shows updated buttons for remaining types ---

func TestRestHandler_HitDiceComponent_MulticlassMultiStep(t *testing.T) {
	var editComponents *[]discordgo.MessageComponent
	var editContent string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		return nil
	}
	sess.InteractionResponseEditFunc = func(_ *discordgo.Interaction, edit *discordgo.WebhookEdit) (*discordgo.Message, error) {
		if edit.Content != nil {
			editContent = *edit.Content
		}
		if edit.Components != nil {
			editComponents = edit.Components
		}
		return &discordgo.Message{}, nil
	}

	campaignID := uuid.New()
	charID := uuid.New()
	scores, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{
		{Class: "fighter", Level: 3},
		{Class: "rogue", Level: 2},
	})
	hitDice, _ := json.Marshal(map[string]int{"d10": 3, "d8": 2})
	featureUses, _ := json.Marshal(map[string]character.FeatureUse{})
	spellSlots, _ := json.Marshal(map[string]character.SlotInfo{})

	char := refdata.Character{
		ID:               charID,
		CampaignID:       campaignID,
		Name:             "Kael",
		Level:            5,
		HpMax:            44,
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
		},
		&mockCheckRollLogger{},
		nil,
	)

	// First click: spend 1 d10. Since d8 dice remain, should show updated buttons
	componentInteraction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "rest_hitdice:" + charID.String() + ":d10:1",
		},
	}

	h.HandleHitDiceComponent(componentInteraction)

	// Should show updated prompt with "Spend more" text and remaining buttons
	if !strings.Contains(editContent, "Spend more hit dice") {
		t.Errorf("expected multi-step prompt, got: %s", editContent)
	}
	// Should have components (buttons for remaining d8 dice + Done button)
	if editComponents == nil || len(*editComponents) < 2 {
		t.Errorf("expected at least 2 component rows (d8 buttons + Done), got %v", editComponents)
	}
}

// --- Done button finalizes without spending more dice ---

func TestRestHandler_HitDiceComponent_DoneButton(t *testing.T) {
	var editContent string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		return nil
	}
	sess.InteractionResponseEditFunc = func(_ *discordgo.Interaction, edit *discordgo.WebhookEdit) (*discordgo.Message, error) {
		if edit.Content != nil {
			editContent = *edit.Content
		}
		return &discordgo.Message{}, nil
	}

	campaignID := uuid.New()
	charID := uuid.New()
	scores, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "fighter", Level: 5}})
	hitDice, _ := json.Marshal(map[string]int{"d10": 5})
	featureUses, _ := json.Marshal(map[string]character.FeatureUse{})
	spellSlots, _ := json.Marshal(map[string]character.SlotInfo{})

	char := refdata.Character{
		ID:               charID,
		CampaignID:       campaignID,
		Name:             "Thorin",
		Level:            5,
		HpMax:            44,
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
		},
		&mockCheckRollLogger{},
		nil,
	)

	componentInteraction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "rest_hitdice:" + charID.String() + ":done:0",
		},
	}

	h.HandleHitDiceComponent(componentInteraction)

	// Done button should finalize the rest
	if !strings.Contains(editContent, "Short Rest Complete") {
		t.Errorf("expected 'Short Rest Complete' after Done button, got: %s", editContent)
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

func TestRestHandler_LongRest_DecrementsCombatantExhaustion(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	store := &fakeRestExhaustionStore{
		combatantID:     uuid.New(),
		conditions:      []byte(`[]`),
		exhaustionLevel: 2,
	}
	h := setupRestHandler(sess)
	h.restService.SetCombatantExhaustionStore(store)

	h.Handle(makeRestInteraction("long"))

	if !store.updated {
		t.Fatal("expected combatant exhaustion to be persisted")
	}
	if store.updatedLevel != 1 {
		t.Errorf("updated exhaustion level = %d, want 1", store.updatedLevel)
	}
	if !strings.Contains(responded, "Exhaustion: level 1") {
		t.Errorf("expected exhaustion result line, got: %s", responded)
	}
}

func TestRestHandler_LongRest_ExhaustionZeroStaysZero(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	store := &fakeRestExhaustionStore{
		combatantID:     uuid.New(),
		conditions:      []byte(`[]`),
		exhaustionLevel: 0,
	}
	h := setupRestHandler(sess)
	h.restService.SetCombatantExhaustionStore(store)

	h.Handle(makeRestInteraction("long"))

	if store.exhaustionLevel != 0 {
		t.Errorf("exhaustion level = %d, want 0", store.exhaustionLevel)
	}
	if store.updated {
		t.Error("expected no persistence write when exhaustion is already 0")
	}
	if strings.Contains(responded, "Exhaustion:") {
		t.Errorf("did not expect exhaustion result line, got: %s", responded)
	}
}

func TestRestHandler_LongRest_DecrementsDurableExhaustionWithoutActiveEncounter(t *testing.T) {
	var responded string
	var capturedParams refdata.UpdateCharacterParams
	var updated bool
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	char := makeRestTestCharacter()
	char.CampaignID = campaignID
	char.CharacterData = pqtype.NullRawMessage{
		RawMessage: []byte(`{"exhaustion_level":2,"notes":"durable"}`),
		Valid:      true,
	}

	h := NewRestHandler(
		sess,
		dice.NewRoller(func(max int) int { return 6 }),
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
				updated = true
				capturedParams = arg
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		&mockCheckRollLogger{},
		nil,
	)

	h.Handle(makeRestInteraction("long"))

	if !updated {
		t.Fatal("expected durable character update")
	}
	exhaustion, ok := rest.ExhaustionLevelFromCharacterData(capturedParams.CharacterData.RawMessage)
	if !ok {
		t.Fatalf("persisted character_data missing exhaustion_level: %s", capturedParams.CharacterData.RawMessage)
	}
	if exhaustion != 1 {
		t.Errorf("persisted exhaustion level = %d, want 1", exhaustion)
	}
	if !strings.Contains(responded, "Exhaustion: level 1") {
		t.Errorf("expected exhaustion result line, got: %s", responded)
	}
}

func TestRestHandler_LongRest_DurableExhaustionZeroStaysZeroWithoutActiveEncounter(t *testing.T) {
	var responded string
	var capturedParams refdata.UpdateCharacterParams
	var updated bool
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	char := makeRestTestCharacter()
	char.CampaignID = campaignID
	char.CharacterData = pqtype.NullRawMessage{
		RawMessage: []byte(`{"exhaustion_level":0}`),
		Valid:      true,
	}

	h := NewRestHandler(
		sess,
		dice.NewRoller(func(max int) int { return 6 }),
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
				updated = true
				capturedParams = arg
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		&mockCheckRollLogger{},
		nil,
	)

	h.Handle(makeRestInteraction("long"))

	if !updated {
		t.Fatal("expected durable character update")
	}
	exhaustion, ok := rest.ExhaustionLevelFromCharacterData(capturedParams.CharacterData.RawMessage)
	if !ok {
		t.Fatalf("persisted character_data missing exhaustion_level: %s", capturedParams.CharacterData.RawMessage)
	}
	if exhaustion != 0 {
		t.Errorf("persisted exhaustion level = %d, want 0", exhaustion)
	}
	if strings.Contains(responded, "Exhaustion:") {
		t.Errorf("did not expect exhaustion result line, got: %s", responded)
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

// --- med-34 / Phase 83a: rest gated on DM approval via auto_approve_rest setting ---

func TestRestHandler_AutoApproveRest_False_ShortCircuitsToWaiting(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	// Build a campaign whose settings explicitly disable auto-approval.
	autoFalse := false
	settings := struct {
		TurnTimeoutHours int    `json:"turn_timeout_hours"`
		DiagonalRule     string `json:"diagonal_rule"`
		AutoApproveRest  *bool  `json:"auto_approve_rest"`
	}{TurnTimeoutHours: 24, DiagonalRule: "standard", AutoApproveRest: &autoFalse}
	raw, _ := json.Marshal(settings)

	campaignID := uuid.New()
	char := makeRestTestCharacter()
	char.CampaignID = campaignID

	roller := dice.NewRoller(func(max int) int { return 6 })
	updaterCalls := 0
	updater := &mockRestCharacterUpdater{
		updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
			updaterCalls++
			return refdata.Character{}, nil
		},
	}
	h := NewRestHandler(
		sess, roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{
				ID:       campaignID,
				Settings: pqtype.NullRawMessage{RawMessage: raw, Valid: true},
			}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errNoEncounter
		}},
		updater, &mockCheckRollLogger{}, nil,
	)
	h.SetNotifier(&recordingNotifier{})
	h.Handle(makeRestInteraction("short"))

	if !strings.Contains(responded, "DM") {
		t.Errorf("expected DM-approval message, got: %s", responded)
	}
	if !strings.Contains(strings.ToLower(responded), "approve") {
		t.Errorf("expected approval-pending text, got: %s", responded)
	}
	if updaterCalls != 0 {
		t.Errorf("rest must NOT apply when auto-approval is disabled (saw %d UpdateCharacter calls)", updaterCalls)
	}
}

func TestRestHandler_AutoApproveRest_DefaultIsTrue(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	// No settings configured at all — should default to auto-approve and run
	// the rest immediately (matches historical behaviour).
	h := setupRestHandler(sess)
	h.Handle(makeRestInteraction("short"))

	if !strings.Contains(responded, "Short Rest") {
		t.Errorf("expected the actual rest prompt when auto-approval defaults to on, got: %s", responded)
	}
}

// --- Finding 9 test: dawn recharge supplied and persisted during long rest ---

// mockRestMagicItemLookup returns magic items with charges info.
type mockRestMagicItemLookup struct {
	items map[string]refdata.MagicItem
}

func (m *mockRestMagicItemLookup) GetMagicItem(_ context.Context, id string) (refdata.MagicItem, error) {
	if mi, ok := m.items[id]; ok {
		return mi, nil
	}
	return refdata.MagicItem{}, fmt.Errorf("not found: %s", id)
}

func TestRestHandler_LongRest_DawnRechargeSuppliedAndPersisted(t *testing.T) {
	var capturedParams refdata.UpdateCharacterParams
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		return nil
	}

	campaignID := uuid.New()
	charID := uuid.New()

	scores, _ := json.Marshal(character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "wizard", Level: 5}})
	hitDice, _ := json.Marshal(map[string]int{"d6": 5})

	// Inventory with a wand that has 3/7 charges
	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 3, MaxCharges: 7},
	}
	itemsJSON, _ := json.Marshal(items)

	char := refdata.Character{
		ID:               charID,
		CampaignID:       campaignID,
		Name:             "Gandalf",
		Level:            5,
		HpMax:            30,
		HpCurrent:        15,
		AbilityScores:    scores,
		Classes:          classes,
		HitDiceRemaining: hitDice,
		Inventory:        pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
	}

	// Magic item ref data with dawn recharge
	chargesJSON, _ := json.Marshal(map[string]any{"max": 7, "recharge": "dawn", "recharge_dice": "1d6+1", "destroy_on_zero": true})
	magicItemLookup := &mockRestMagicItemLookup{
		items: map[string]refdata.MagicItem{
			"wand-of-fireballs": {
				ID:      "wand-of-fireballs",
				Charges: pqtype.NullRawMessage{RawMessage: chargesJSON, Valid: true},
			},
		},
	}

	roller := dice.NewRoller(func(max int) int { return 4 })

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
				capturedParams = arg
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		nil,
		nil,
	)
	h.SetMagicItemLookup(magicItemLookup)

	h.Handle(makeRestInteraction("long"))

	// Finding 9: verify inventory was persisted with recharged charges.
	// The key assertion is that Inventory IS persisted (was previously missing).
	// The exact charge value depends on the crypto-rand roll inside
	// inventory.NewService(nil).DawnRecharge, so we assert the charges
	// increased from the starting value of 3 (any recharge adds at least 2
	// from 1d6+1 minimum roll of 1+1=2).
	if !capturedParams.Inventory.Valid {
		t.Fatal("expected Inventory to be persisted after dawn recharge")
	}
	var updatedItems []character.InventoryItem
	if err := json.Unmarshal(capturedParams.Inventory.RawMessage, &updatedItems); err != nil {
		t.Fatalf("unmarshal inventory: %v", err)
	}
	if len(updatedItems) != 1 {
		t.Fatalf("expected 1 item, got %d", len(updatedItems))
	}
	if updatedItems[0].Charges <= 3 {
		t.Errorf("expected charges to increase from 3 after dawn recharge, got %d", updatedItems[0].Charges)
	}
	if updatedItems[0].Charges > 7 {
		t.Errorf("expected charges to be capped at max 7, got %d", updatedItems[0].Charges)
	}
}
