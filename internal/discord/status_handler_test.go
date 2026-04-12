package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

func pqtypeNull(raw json.RawMessage) pqtype.NullRawMessage {
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}

// --- mock types for StatusHandler dependencies ---

type mockStatusCampaignProvider struct {
	campaign refdata.Campaign
	err      error
}

func (m *mockStatusCampaignProvider) GetCampaignByGuildID(_ context.Context, _ string) (refdata.Campaign, error) {
	return m.campaign, m.err
}

type mockStatusCharacterLookup struct {
	char refdata.Character
	err  error
}

func (m *mockStatusCharacterLookup) GetCharacterByCampaignAndDiscord(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
	return m.char, m.err
}

type mockStatusEncounterProvider struct {
	encounterID uuid.UUID
	err         error
}

func (m *mockStatusEncounterProvider) ActiveEncounterForUser(_ context.Context, _, _ string) (uuid.UUID, error) {
	return m.encounterID, m.err
}

type mockStatusCombatantLookup struct {
	combatants []refdata.Combatant
	err        error
}

func (m *mockStatusCombatantLookup) ListCombatantsByEncounterID(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
	return m.combatants, m.err
}

type mockStatusConcentrationLookup struct {
	zones []refdata.EncounterZone
	err   error
}

func (m *mockStatusConcentrationLookup) ListConcentrationZonesByCombatant(_ context.Context, _ uuid.UUID) ([]refdata.EncounterZone, error) {
	return m.zones, m.err
}

type mockStatusReactionLookup struct {
	reactions []refdata.ReactionDeclaration
	err       error
}

func (m *mockStatusReactionLookup) ListActiveReactionDeclarationsByCombatant(_ context.Context, _ refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
	return m.reactions, m.err
}

// --- helpers ---

func statusInteraction(guildID, userID string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "status",
		},
	}
}

func runStatusHandler(t *testing.T, h *StatusHandler, guildID, userID string) (string, discordgo.MessageFlags) {
	t.Helper()
	var content string
	var flags discordgo.MessageFlags
	mock := h.session.(*MockSession)
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		content = resp.Data.Content
		flags = resp.Data.Flags
		return nil
	}
	h.Handle(statusInteraction(guildID, userID))
	return content, flags
}

// --- tests ---

func TestStatusHandler_NoCampaign(t *testing.T) {
	mock := newTestMock()
	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{err: errors.New("not found")},
		nil, nil, nil, nil, nil,
	)
	content, flags := runStatusHandler(t, h, "guild-1", "user-1")
	if flags != discordgo.MessageFlagsEphemeral {
		t.Errorf("expected ephemeral, got %d", flags)
	}
	if content != "No campaign found for this server." {
		t.Errorf("unexpected message: %q", content)
	}
}

func TestStatusHandler_NoCharacter(t *testing.T) {
	mock := newTestMock()
	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{err: errors.New("not found")},
		nil, nil, nil, nil,
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	if content != "Could not find your character. Use `/register` first." {
		t.Errorf("unexpected message: %q", content)
	}
}

func TestStatusHandler_NotInCombat_NoEffects(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()
	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:   charID,
			Name: "Aria",
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}},
		&mockStatusEncounterProvider{err: errors.New("no encounter")},
		nil, nil, nil,
	)
	content, flags := runStatusHandler(t, h, "guild-1", "user-1")
	if flags != discordgo.MessageFlagsEphemeral {
		t.Errorf("expected ephemeral, got %d", flags)
	}
	if content != "**Status — Aria**\n\nNo active effects." {
		t.Errorf("unexpected message: %q", content)
	}
}

func TestStatusHandler_InCombat_WithConditions(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	conditions := json.RawMessage(`[{"condition":"poisoned","duration_rounds":5,"started_round":1}]`)

	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Aria",
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}},
		&mockStatusEncounterProvider{encounterID: encounterID},
		&mockStatusCombatantLookup{combatants: []refdata.Combatant{
			{
				ID:          combatantID,
				ShortID:     "AR",
				DisplayName: "Aria",
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				Conditions:  conditions,
				TempHp:      8,
			},
		}},
		&mockStatusConcentrationLookup{zones: []refdata.EncounterZone{
			{SourceSpell: "Bless"},
		}},
		&mockStatusReactionLookup{reactions: []refdata.ReactionDeclaration{
			{Description: "Shield if hit by ranged attack", IsReadiedAction: false},
		}},
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	if content == "" {
		t.Fatal("expected non-empty response")
	}
	assertContains(t, content, "**Status — Aria (AR)**")
	assertContains(t, content, "Poisoned")
	assertContains(t, content, "**Concentration:** Bless")
	assertContains(t, content, "**Temp HP:** 8")
	assertContains(t, content, "Shield if hit by ranged attack")
}

func TestStatusHandler_InCombat_WithRage(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Grog",
			Classes: json.RawMessage(`[{"class":"Barbarian","level":5}]`),
		}},
		&mockStatusEncounterProvider{encounterID: encounterID},
		&mockStatusCombatantLookup{combatants: []refdata.Combatant{
			{
				ID:                  combatantID,
				ShortID:             "GR",
				DisplayName:         "Grog",
				CharacterID:         uuid.NullUUID{UUID: charID, Valid: true},
				IsRaging:            true,
				RageRoundsRemaining: sql.NullInt32{Int32: 6, Valid: true},
				Conditions:          json.RawMessage(`[]`),
			},
		}},
		&mockStatusConcentrationLookup{},
		&mockStatusReactionLookup{},
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	assertContains(t, content, "**Rage:** Active (6 rounds remaining)")
}

func TestStatusHandler_InCombat_WithBardicInspiration(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Aria",
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}},
		&mockStatusEncounterProvider{encounterID: encounterID},
		&mockStatusCombatantLookup{combatants: []refdata.Combatant{
			{
				ID:                      combatantID,
				ShortID:                 "AR",
				DisplayName:             "Aria",
				CharacterID:             uuid.NullUUID{UUID: charID, Valid: true},
				BardicInspirationDie:    sql.NullString{String: "d8", Valid: true},
				BardicInspirationSource: sql.NullString{String: "Melody", Valid: true},
				Conditions:              json.RawMessage(`[]`),
			},
		}},
		&mockStatusConcentrationLookup{},
		&mockStatusReactionLookup{},
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	assertContains(t, content, "**Bardic Inspiration:** d8 (from Melody)")
}

func TestStatusHandler_InCombat_WithWildShape(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Elara",
			Classes: json.RawMessage(`[{"class":"Druid","level":5}]`),
		}},
		&mockStatusEncounterProvider{encounterID: encounterID},
		&mockStatusCombatantLookup{combatants: []refdata.Combatant{
			{
				ID:                   combatantID,
				ShortID:              "EL",
				DisplayName:          "Elara",
				CharacterID:          uuid.NullUUID{UUID: charID, Valid: true},
				IsWildShaped:         true,
				WildShapeCreatureRef: sql.NullString{String: "Dire Wolf", Valid: true},
				Conditions:           json.RawMessage(`[]`),
			},
		}},
		&mockStatusConcentrationLookup{},
		&mockStatusReactionLookup{},
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	assertContains(t, content, "**Wild Shape:** Dire Wolf")
}

func TestStatusHandler_NotInCombat_WithKi(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()

	kiJSON := json.RawMessage(`{"ki":3}`)
	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Monk",
			Classes: json.RawMessage(`[{"class":"Monk","level":5}]`),
			FeatureUses: pqtypeNull(kiJSON),
		}},
		&mockStatusEncounterProvider{err: errors.New("no encounter")},
		nil, nil, nil,
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	assertContains(t, content, "**Ki Points:** 3/5")
}

func TestStatusHandler_NotInCombat_WithSorceryPoints(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()

	spJSON := json.RawMessage(`{"sorcery-points":4}`)
	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Sorc",
			Classes: json.RawMessage(`[{"class":"Sorcerer","level":7}]`),
			FeatureUses: pqtypeNull(spJSON),
		}},
		&mockStatusEncounterProvider{err: errors.New("no encounter")},
		nil, nil, nil,
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	assertContains(t, content, "**Sorcery Points:** 4/7")
}

func TestStatusHandler_InCombat_ReadiedActions(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Aria",
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}},
		&mockStatusEncounterProvider{encounterID: encounterID},
		&mockStatusCombatantLookup{combatants: []refdata.Combatant{
			{
				ID:          combatantID,
				ShortID:     "AR",
				DisplayName: "Aria",
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				Conditions:  json.RawMessage(`[]`),
			},
		}},
		&mockStatusConcentrationLookup{},
		&mockStatusReactionLookup{reactions: []refdata.ReactionDeclaration{
			{Description: "Shield if hit", IsReadiedAction: false},
			{Description: "Attack goblin if it moves", IsReadiedAction: true},
		}},
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	assertContains(t, content, "**Reaction Declarations:**")
	assertContains(t, content, "**Readied Actions:**")
	assertContains(t, content, "Shield if hit")
	assertContains(t, content, "Attack goblin if it moves")
}

func TestStatusHandler_InCombat_Exhaustion(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Aria",
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}},
		&mockStatusEncounterProvider{encounterID: encounterID},
		&mockStatusCombatantLookup{combatants: []refdata.Combatant{
			{
				ID:              combatantID,
				ShortID:         "AR",
				DisplayName:     "Aria",
				CharacterID:     uuid.NullUUID{UUID: charID, Valid: true},
				Conditions:      json.RawMessage(`[]`),
				ExhaustionLevel: 3,
			},
		}},
		&mockStatusConcentrationLookup{},
		&mockStatusReactionLookup{},
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	assertContains(t, content, "**Exhaustion:** Level 3")
}

func TestStatusHandler_NilEncounterProvider(t *testing.T) {
	mock := newTestMock()
	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      uuid.New(),
			Name:    "Aria",
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}},
		nil, nil, nil, nil,
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	assertContains(t, content, "**Status — Aria**")
	assertContains(t, content, "No active effects.")
}

func TestStatusHandler_CombatantLookupError(t *testing.T) {
	mock := newTestMock()
	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      uuid.New(),
			Name:    "Aria",
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}},
		&mockStatusEncounterProvider{encounterID: uuid.New()},
		&mockStatusCombatantLookup{err: errors.New("db error")},
		nil, nil,
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	// Should gracefully degrade to out-of-combat status.
	assertContains(t, content, "**Status — Aria**")
}

func TestStatusHandler_CharacterNotInCombatants(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()
	otherCharID := uuid.New()
	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Aria",
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}},
		&mockStatusEncounterProvider{encounterID: uuid.New()},
		&mockStatusCombatantLookup{combatants: []refdata.Combatant{
			{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: otherCharID, Valid: true}},
		}},
		nil, nil,
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	assertContains(t, content, "**Status — Aria**")
	assertContains(t, content, "No active effects.")
}

func TestStatusHandler_NilConcentrationAndReactionLookups(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Aria",
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}},
		&mockStatusEncounterProvider{encounterID: encounterID},
		&mockStatusCombatantLookup{combatants: []refdata.Combatant{
			{
				ID:          combatantID,
				ShortID:     "AR",
				DisplayName: "Aria",
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				Conditions:  json.RawMessage(`[]`),
			},
		}},
		nil, // nil concentration
		nil, // nil reaction
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	assertContains(t, content, "**Status — Aria (AR)**")
	assertContains(t, content, "No active effects.")
}

func TestStatusHandler_ConcentrationError(t *testing.T) {
	mock := newTestMock()
	charID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	h := NewStatusHandler(
		mock,
		&mockStatusCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockStatusCharacterLookup{char: refdata.Character{
			ID:      charID,
			Name:    "Aria",
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}},
		&mockStatusEncounterProvider{encounterID: encounterID},
		&mockStatusCombatantLookup{combatants: []refdata.Combatant{
			{
				ID:          combatantID,
				ShortID:     "AR",
				DisplayName: "Aria",
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				Conditions:  json.RawMessage(`[]`),
			},
		}},
		&mockStatusConcentrationLookup{err: errors.New("db error")},
		&mockStatusReactionLookup{err: errors.New("db error")},
	)
	content, _ := runStatusHandler(t, h, "guild-1", "user-1")
	// Errors should be handled gracefully, just showing what's available.
	assertContains(t, content, "**Status — Aria (AR)**")
}

func TestTitleCase_Empty(t *testing.T) {
	if got := titleCase(""); got != "" {
		t.Errorf("titleCase empty: got %q", got)
	}
}

func TestTitleCase_AlreadyUpper(t *testing.T) {
	if got := titleCase("Poisoned"); got != "Poisoned" {
		t.Errorf("titleCase Poisoned: got %q", got)
	}
}

func TestEqualFoldASCII_DifferentLengths(t *testing.T) {
	if equalFoldASCII("Monk", "Mo") {
		t.Error("should not match different lengths")
	}
}

func TestClassLevelFrom_NotFound(t *testing.T) {
	classes := []classEntry{{Class: "Fighter", Level: 5}}
	if got := classLevelFrom(classes, "Wizard"); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

// --- test helpers ---

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !containsSubstr(got, want) {
		t.Errorf("got %q, want to contain %q", got, want)
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
