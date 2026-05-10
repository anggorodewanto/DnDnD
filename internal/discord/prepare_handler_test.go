package discord

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Mocks for /prepare ---

type mockPrepareService struct {
	infoCalls    []prepareInfoCall
	infoResult   combat.PreparationInfo
	infoErr      error
	prepareCalls []combat.PrepareSpellsInput
	prepResult   combat.PrepareSpellsResult
	prepErr      error
}

type prepareInfoCall struct {
	charID    uuid.UUID
	className string
	subclass  string
}

func (m *mockPrepareService) GetPreparationInfo(_ context.Context, charID uuid.UUID, className, subclass string) (combat.PreparationInfo, error) {
	m.infoCalls = append(m.infoCalls, prepareInfoCall{charID, className, subclass})
	return m.infoResult, m.infoErr
}

func (m *mockPrepareService) PrepareSpells(_ context.Context, in combat.PrepareSpellsInput) (combat.PrepareSpellsResult, error) {
	m.prepareCalls = append(m.prepareCalls, in)
	return m.prepResult, m.prepErr
}

type mockPrepareEncProvider struct {
	encID      uuid.UUID
	resolveErr error
	enc        refdata.Encounter
	getEncErr  error
}

func (m *mockPrepareEncProvider) ActiveEncounterForUser(_ context.Context, _, _ string) (uuid.UUID, error) {
	if m.resolveErr != nil {
		return uuid.Nil, m.resolveErr
	}
	return m.encID, nil
}

func (m *mockPrepareEncProvider) GetEncounter(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
	if m.getEncErr != nil {
		return refdata.Encounter{}, m.getEncErr
	}
	return m.enc, nil
}

type mockPrepareCampProvider struct {
	camp refdata.Campaign
	err  error
}

func (m *mockPrepareCampProvider) GetCampaignByGuildID(_ context.Context, _ string) (refdata.Campaign, error) {
	return m.camp, m.err
}

type mockPrepareCharLookup struct {
	char refdata.Character
	err  error
}

func (m *mockPrepareCharLookup) GetCharacterByCampaignAndDiscord(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
	return m.char, m.err
}

// --- Helpers ---

func makePrepareInteraction(opts map[string]string) *discordgo.Interaction {
	cmdOpts := []*discordgo.ApplicationCommandInteractionDataOption{}
	for name, val := range opts {
		cmdOpts = append(cmdOpts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: name, Value: val, Type: discordgo.ApplicationCommandOptionString,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "prepare",
			Options: cmdOpts,
		},
	}
}

func setupPrepareHandler() (*PrepareHandler, *mockMoveSession, *mockPrepareService, *mockPrepareEncProvider, *mockPrepareCharLookup) {
	encID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	classesJSON, _ := json.Marshal([]map[string]any{
		{"class": "cleric", "subclass": "life", "level": 5},
	})

	provider := &mockPrepareEncProvider{
		encID: encID,
		enc: refdata.Encounter{
			ID:         encID,
			CampaignID: campID,
			Status:     "preparing",
		},
	}

	charLookup := &mockPrepareCharLookup{
		char: refdata.Character{
			ID:      charID,
			Name:    "Aria",
			Classes: classesJSON,
		},
	}

	prepSvc := &mockPrepareService{
		infoResult: combat.PreparationInfo{
			MaxPrepared:     5,
			CurrentPrepared: []string{"bless"},
			ClassSpells:     []refdata.Spell{{ID: "bless", Name: "Bless", Level: 1, School: "enchantment"}},
		},
		prepResult: combat.PrepareSpellsResult{
			PreparedCount: 1,
			MaxPrepared:   5,
		},
	}

	sess := &mockMoveSession{}
	h := NewPrepareHandler(
		sess,
		prepSvc,
		provider,
		&mockPrepareCampProvider{camp: refdata.Campaign{ID: campID}},
		charLookup,
	)
	return h, sess, prepSvc, provider, charLookup
}

// --- Tests ---

func TestPrepareHandler_PreviewListsSpellsWhenSpellsArgEmpty(t *testing.T) {
	h, sess, svc, _, _ := setupPrepareHandler()

	h.Handle(makePrepareInteraction(map[string]string{}))

	if len(svc.infoCalls) != 1 {
		t.Fatalf("expected 1 GetPreparationInfo call, got %d", len(svc.infoCalls))
	}
	if len(svc.prepareCalls) != 0 {
		t.Errorf("expected no PrepareSpells call in preview mode")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Spell Preparation") {
		t.Errorf("expected preparation message, got %q", sess.lastResponse.Data.Content)
	}
}

func TestPrepareHandler_CommitsSpellsWhenSpellsArgProvided(t *testing.T) {
	h, sess, svc, _, _ := setupPrepareHandler()

	h.Handle(makePrepareInteraction(map[string]string{
		"spells": "bless, cure-wounds",
	}))

	if len(svc.prepareCalls) != 1 {
		t.Fatalf("expected 1 PrepareSpells call, got %d", len(svc.prepareCalls))
	}
	got := svc.prepareCalls[0].Selected
	if len(got) != 2 || got[0] != "bless" || got[1] != "cure-wounds" {
		t.Errorf("expected [bless, cure-wounds], got %v", got)
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Prepared") {
		t.Errorf("expected prepared confirmation, got %q", sess.lastResponse.Data.Content)
	}
}

func TestPrepareHandler_RejectsActiveCombat(t *testing.T) {
	h, sess, svc, provider, _ := setupPrepareHandler()
	provider.enc.Status = "active"

	h.Handle(makePrepareInteraction(map[string]string{
		"spells": "bless",
	}))

	if len(svc.prepareCalls) != 0 {
		t.Error("expected no PrepareSpells call during active combat")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "out of combat") {
		t.Errorf("expected out-of-combat rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestPrepareHandler_NoEncounter_StillUsesCharacterFromCampaign(t *testing.T) {
	h, sess, svc, provider, _ := setupPrepareHandler()
	provider.resolveErr = errors.New("no encounter")

	h.Handle(makePrepareInteraction(map[string]string{}))

	if len(svc.infoCalls) != 1 {
		t.Fatalf("expected 1 GetPreparationInfo call even when no encounter, got %d", len(svc.infoCalls))
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Spell Preparation") {
		t.Errorf("expected preparation message, got %q", sess.lastResponse.Data.Content)
	}
}

func TestPrepareHandler_NoPreparedCasterClass(t *testing.T) {
	h, sess, _, _, charLookup := setupPrepareHandler()
	classesJSON, _ := json.Marshal([]map[string]any{
		{"class": "fighter", "subclass": "champion", "level": 5},
	})
	charLookup.char.Classes = classesJSON

	h.Handle(makePrepareInteraction(map[string]string{}))

	if !strings.Contains(sess.lastResponse.Data.Content, "not a prepared caster") {
		t.Errorf("expected non-prepared-caster message, got %q", sess.lastResponse.Data.Content)
	}
}

func TestPrepareHandler_CharacterLookupError(t *testing.T) {
	h, sess, _, _, charLookup := setupPrepareHandler()
	charLookup.err = errors.New("not registered")

	h.Handle(makePrepareInteraction(map[string]string{}))

	if !strings.Contains(sess.lastResponse.Data.Content, "character") {
		t.Errorf("expected character-lookup error, got %q", sess.lastResponse.Data.Content)
	}
}

func TestPrepareHandler_PrepareServiceError(t *testing.T) {
	h, sess, svc, _, _ := setupPrepareHandler()
	svc.prepErr = errors.New("validation failed")

	h.Handle(makePrepareInteraction(map[string]string{
		"spells": "bless",
	}))

	if !strings.Contains(sess.lastResponse.Data.Content, "validation failed") {
		t.Errorf("expected service error in response, got %q", sess.lastResponse.Data.Content)
	}
}

func TestPrepareHandler_ExplicitClassOverride(t *testing.T) {
	h, _, svc, _, charLookup := setupPrepareHandler()
	classesJSON, _ := json.Marshal([]map[string]any{
		{"class": "cleric", "subclass": "life", "level": 3},
		{"class": "wizard", "subclass": "evocation", "level": 2},
	})
	charLookup.char.Classes = classesJSON

	h.Handle(makePrepareInteraction(map[string]string{
		"spells": "bless",
		"class":  "cleric",
	}))

	if len(svc.prepareCalls) != 1 {
		t.Fatalf("expected 1 PrepareSpells call, got %d", len(svc.prepareCalls))
	}
	if svc.prepareCalls[0].ClassName != "cleric" {
		t.Errorf("ClassName = %q want cleric", svc.prepareCalls[0].ClassName)
	}
}
