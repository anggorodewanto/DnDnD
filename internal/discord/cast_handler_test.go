package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Mocks for /cast ---

type mockCastCombatService struct {
	castCalls    []combat.CastCommand
	castResult   combat.CastResult
	castErr      error
	aoeCalls     []combat.AoECastCommand
	aoeResult    combat.AoECastResult
	aoeErr       error
	concID       string
	concErr      error
}

func (m *mockCastCombatService) Cast(_ context.Context, cmd combat.CastCommand, _ *dice.Roller) (combat.CastResult, error) {
	m.castCalls = append(m.castCalls, cmd)
	return m.castResult, m.castErr
}

func (m *mockCastCombatService) CastAoE(_ context.Context, cmd combat.AoECastCommand) (combat.AoECastResult, error) {
	m.aoeCalls = append(m.aoeCalls, cmd)
	return m.aoeResult, m.aoeErr
}

func (m *mockCastCombatService) GetCasterConcentrationName(_ context.Context, _ uuid.UUID) (string, error) {
	return m.concID, m.concErr
}

type mockCastProvider struct {
	encID      uuid.UUID
	turn       refdata.Turn
	caster     refdata.Combatant
	target     refdata.Combatant
	enc        refdata.Encounter
	spells     map[string]refdata.Spell
	mapData    refdata.Map
	resolveErr error
	getEncErr  error
	getTurnErr error
	getCombErr error
	getSpellErr error
	listErr    error
	getMapErr  error
	// listCombatantsOverride, when non-nil, replaces the default
	// `[caster, target]` slice ListCombatantsByEncounterID returns. SR-025
	// tests use this to seed extra combatants (twin target, AoE protectees).
	listCombatantsOverride []refdata.Combatant
}

func (m *mockCastProvider) ActiveEncounterForUser(_ context.Context, _, _ string) (uuid.UUID, error) {
	if m.resolveErr != nil {
		return uuid.Nil, m.resolveErr
	}
	return m.encID, nil
}

func (m *mockCastProvider) GetEncounter(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
	if m.getEncErr != nil {
		return refdata.Encounter{}, m.getEncErr
	}
	return m.enc, nil
}

func (m *mockCastProvider) GetCombatant(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
	if m.getCombErr != nil {
		return refdata.Combatant{}, m.getCombErr
	}
	if id == m.caster.ID {
		return m.caster, nil
	}
	if id == m.target.ID {
		return m.target, nil
	}
	for _, c := range m.listCombatantsOverride {
		if c.ID == id {
			return c, nil
		}
	}
	return m.target, nil
}

func (m *mockCastProvider) ListCombatantsByEncounterID(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	if m.listCombatantsOverride != nil {
		return m.listCombatantsOverride, nil
	}
	return []refdata.Combatant{m.caster, m.target}, nil
}

func (m *mockCastProvider) GetTurn(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
	if m.getTurnErr != nil {
		return refdata.Turn{}, m.getTurnErr
	}
	return m.turn, nil
}

func (m *mockCastProvider) GetSpell(_ context.Context, id string) (refdata.Spell, error) {
	if m.getSpellErr != nil {
		return refdata.Spell{}, m.getSpellErr
	}
	sp, ok := m.spells[id]
	if !ok {
		return refdata.Spell{}, errors.New("spell not found")
	}
	return sp, nil
}

func (m *mockCastProvider) GetMapByID(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
	if m.getMapErr != nil {
		return refdata.Map{}, m.getMapErr
	}
	return m.mapData, nil
}

// --- Helpers ---

func makeCastInteraction(opts map[string]any) *discordgo.Interaction {
	cmdOpts := []*discordgo.ApplicationCommandInteractionDataOption{}
	for name, val := range opts {
		switch v := val.(type) {
		case string:
			cmdOpts = append(cmdOpts, &discordgo.ApplicationCommandInteractionDataOption{
				Name: name, Value: v, Type: discordgo.ApplicationCommandOptionString,
			})
		case bool:
			cmdOpts = append(cmdOpts, &discordgo.ApplicationCommandInteractionDataOption{
				Name: name, Value: v, Type: discordgo.ApplicationCommandOptionBoolean,
			})
		case int:
			cmdOpts = append(cmdOpts, &discordgo.ApplicationCommandInteractionDataOption{
				Name: name, Value: float64(v), Type: discordgo.ApplicationCommandOptionInteger,
			})
		}
	}
	return &discordgo.Interaction{
		Type:      discordgo.InteractionApplicationCommand,
		GuildID:   "g1",
		ChannelID: "ch-cast",
		Member:    &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "cast",
			Options: cmdOpts,
		},
	}
}

// minimalTiledJSON returns a tiny 5x5 Tiled JSON suitable for AoE walls parsing.
const minimalTiledJSON = `{"width":5,"height":5,"tilewidth":32,"tileheight":32,"layers":[{"type":"tilelayer","name":"floor","width":5,"height":5,"data":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1]}],"tilesets":[]}`

func setupCastHandler() (*CastHandler, *mockMoveSession, *mockCastCombatService, *mockCastProvider) {
	encID := uuid.New()
	turnID := uuid.New()
	casterID := uuid.New()
	targetID := uuid.New()
	mapID := uuid.New()

	provider := &mockCastProvider{
		encID: encID,
		enc: refdata.Encounter{
			ID:            encID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			MapID:         uuid.NullUUID{UUID: mapID, Valid: true},
			Status:        "active",
		},
		turn: refdata.Turn{
			ID:          turnID,
			CombatantID: casterID,
			ActionUsed:  false,
		},
		caster: refdata.Combatant{
			ID: casterID, ShortID: "AR", DisplayName: "Aria",
			PositionCol: "B", PositionRow: 2,
		},
		target: refdata.Combatant{
			ID: targetID, ShortID: "OS", DisplayName: "Orc",
			PositionCol: "D", PositionRow: 4,
		},
		spells: map[string]refdata.Spell{
			"fire-bolt": {
				ID: "fire-bolt", Name: "Fire Bolt", Level: 0,
			},
			"fireball": {
				ID: "fireball", Name: "Fireball", Level: 3,
				AreaOfEffect: pqtype.NullRawMessage{
					RawMessage: []byte(`{"shape":"sphere","radius_ft":20}`),
					Valid:      true,
				},
			},
		},
		mapData: refdata.Map{
			TiledJson: []byte(minimalTiledJSON),
		},
	}

	combatSvc := &mockCastCombatService{
		castResult: combat.CastResult{
			CasterName: "Aria",
			SpellName:  "Fire Bolt",
			SpellLevel: 0,
			TargetName: "Orc",
		},
		aoeResult: combat.AoECastResult{
			CasterName:    "Aria",
			SpellName:     "Fireball",
			SpellLevel:    3,
			AffectedNames: []string{"Orc"},
		},
	}
	sess := &mockMoveSession{}
	h := NewCastHandler(sess, combatSvc, provider, dice.NewRoller(func(_ int) int { return 10 }))
	return h, sess, combatSvc, provider
}

// --- Tests ---

func TestCastHandler_DispatchesSingleTargetCast(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fire-bolt",
		"target": "OS",
	}))

	if len(svc.castCalls) != 1 {
		t.Fatalf("expected 1 cast call, got %d", len(svc.castCalls))
	}
	if len(svc.aoeCalls) != 0 {
		t.Errorf("expected no AoE call for non-AoE spell")
	}
	got := svc.castCalls[0]
	if got.SpellID != "fire-bolt" {
		t.Errorf("SpellID = %q want fire-bolt", got.SpellID)
	}
	if got.TargetID == uuid.Nil {
		t.Errorf("expected non-nil TargetID")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Aria") {
		t.Errorf("expected cast log, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_DispatchesAoECastForAreaSpell(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fireball",
		"target": "D4",
	}))

	if len(svc.aoeCalls) != 1 {
		t.Fatalf("expected 1 AoE cast call, got %d", len(svc.aoeCalls))
	}
	if len(svc.castCalls) != 0 {
		t.Errorf("expected no single-target call for AoE spell")
	}
	got := svc.aoeCalls[0]
	if got.SpellID != "fireball" {
		t.Errorf("SpellID = %q want fireball", got.SpellID)
	}
	if got.TargetCol != "D" || got.TargetRow != 4 {
		t.Errorf("Target = %s%d want D4", got.TargetCol, got.TargetRow)
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Fireball") {
		t.Errorf("expected AoE log, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_PassesSlotLevelAndMetamagic(t *testing.T) {
	h, _, svc, _ := setupCastHandler()

	h.Handle(makeCastInteraction(map[string]any{
		"spell":     "fire-bolt",
		"target":    "OS",
		"level":     2,
		"empowered": true,
		"distant":   true,
	}))

	if len(svc.castCalls) != 1 {
		t.Fatalf("expected 1 cast call, got %d", len(svc.castCalls))
	}
	got := svc.castCalls[0]
	if got.SlotLevel != 2 {
		t.Errorf("SlotLevel = %d want 2", got.SlotLevel)
	}
	if !containsString(got.Metamagic, "empowered") {
		t.Errorf("expected metamagic to include 'empowered', got %v", got.Metamagic)
	}
	if !containsString(got.Metamagic, "distant") {
		t.Errorf("expected metamagic to include 'distant', got %v", got.Metamagic)
	}
}

// E-66b: /cast extended:true forwards "extended" to the combat service so
// Extended Spell metamagic is reachable from Discord.
func TestCastHandler_ForwardsExtendedMetamagic(t *testing.T) {
	h, _, svc, _ := setupCastHandler()

	h.Handle(makeCastInteraction(map[string]any{
		"spell":    "fire-bolt",
		"target":   "OS",
		"extended": true,
	}))

	if len(svc.castCalls) != 1 {
		t.Fatalf("expected 1 cast call, got %d", len(svc.castCalls))
	}
	got := svc.castCalls[0]
	if !containsString(got.Metamagic, "extended") {
		t.Errorf("expected metamagic to include 'extended', got %v", got.Metamagic)
	}
}

func TestCastHandler_NoSpell(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()

	h.Handle(makeCastInteraction(map[string]any{}))

	if len(svc.castCalls) != 0 || len(svc.aoeCalls) != 0 {
		t.Error("expected no service call when spell missing")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "spell") {
		t.Errorf("expected spell-missing prompt, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_NoActiveEncounter(t *testing.T) {
	h, sess, _, provider := setupCastHandler()
	provider.resolveErr = errors.New("no encounter")

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fire-bolt",
		"target": "OS",
	}))

	if !strings.Contains(sess.lastResponse.Data.Content, "not in an active encounter") {
		t.Errorf("expected encounter rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_TurnGate_RejectsWrongOwner(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()
	h.SetTurnGate(&stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Bob",
		CurrentDiscordUserID: "u-bob",
	}})

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fire-bolt",
		"target": "OS",
	}))

	if len(svc.castCalls) != 0 {
		t.Error("expected no service call when gate rejects")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Bob") {
		t.Errorf("expected wrong-owner rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_PostsToCombatLog(t *testing.T) {
	h, _, _, _ := setupCastHandler()
	captured := []string{}
	h.session = &capturingSession{
		mockMoveSession: &mockMoveSession{},
		sendFn: func(channelID, content string) {
			captured = append(captured, channelID+":"+content)
		},
	}
	h.SetChannelIDProvider(&mockDeathSaveCSP{
		fn: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "ch-cl"}, nil
		},
	})

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fire-bolt",
		"target": "OS",
	}))

	if len(captured) != 1 || !strings.HasPrefix(captured[0], "ch-cl:") {
		t.Errorf("expected combat-log post to ch-cl, got %v", captured)
	}
}

func TestCastHandler_TargetNotFound_SingleTarget(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fire-bolt",
		"target": "ZZ",
	}))

	if len(svc.castCalls) != 0 {
		t.Error("expected no service call when target missing")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "not found") {
		t.Errorf("expected target-missing rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_AoENoTarget(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()

	h.Handle(makeCastInteraction(map[string]any{
		"spell": "fireball",
	}))

	if len(svc.aoeCalls) != 0 {
		t.Error("expected no AoE call when target missing")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "target") {
		t.Errorf("expected target-prompt, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_ServiceError(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()
	svc.castErr = errors.New("not enough slots")

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fire-bolt",
		"target": "OS",
	}))

	if !strings.Contains(sess.lastResponse.Data.Content, "Cast failed") {
		t.Errorf("expected service-error rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_AoEServiceError(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()
	svc.aoeErr = errors.New("invalid aoe")

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fireball",
		"target": "D4",
	}))

	if !strings.Contains(sess.lastResponse.Data.Content, "Cast failed") {
		t.Errorf("expected service-error rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_RitualFlag(t *testing.T) {
	h, _, svc, _ := setupCastHandler()

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fire-bolt",
		"target": "OS",
		"ritual": true,
	}))

	if len(svc.castCalls) != 1 {
		t.Fatalf("expected 1 cast call, got %d", len(svc.castCalls))
	}
	if !svc.castCalls[0].IsRitual {
		t.Errorf("expected IsRitual=true")
	}
}

// --- /cast identify and /cast detect-magic short-circuit tests ---

// mockCastInventoryAdapter is a test double for CastInventoryAdapter.
type mockCastInventoryAdapter struct {
	char            refdata.Character
	charErr         error
	updateInvCalls  []refdata.UpdateCharacterInventoryParams
	updateSlotCalls []updateSpellSlotsCall
	updateInvErr    error
	updateSlotErr   error
}

type updateSpellSlotsCall struct {
	CharID uuid.UUID
	Slots  pqtype.NullRawMessage
}

func (m *mockCastInventoryAdapter) GetCharacterByGuildAndDiscord(_ context.Context, _, _ string) (refdata.Character, error) {
	return m.char, m.charErr
}

func (m *mockCastInventoryAdapter) UpdateCharacterInventory(_ context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
	m.updateInvCalls = append(m.updateInvCalls, arg)
	return m.char, m.updateInvErr
}

func (m *mockCastInventoryAdapter) UpdateCharacterSpellSlots(_ context.Context, charID uuid.UUID, slots pqtype.NullRawMessage) error {
	m.updateSlotCalls = append(m.updateSlotCalls, updateSpellSlotsCall{CharID: charID, Slots: slots})
	return m.updateSlotErr
}

func makeIdentifyTestCharacter(charID uuid.UUID, items []byte, slots []byte) refdata.Character {
	// CharacterData stores spells_known via a "spells" key.
	// Provide a minimal shape that the identify path can introspect.
	const charData = `{"spells_known":["identify"],"spells_prepared":["identify"]}`
	return refdata.Character{
		ID:            charID,
		Name:          "Aria",
		Inventory:     pqtype.NullRawMessage{RawMessage: items, Valid: true},
		SpellSlots:    pqtype.NullRawMessage{RawMessage: slots, Valid: true},
		CharacterData: pqtype.NullRawMessage{RawMessage: []byte(charData), Valid: true},
	}
}

func TestCastHandler_IdentifyShortCircuits(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()

	charID := uuid.New()
	unidentified := false
	items, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "mystery-ring", Name: "Ring of Mystery", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	})
	slots, _ := json.Marshal(map[string]character.SlotInfo{"1": {Current: 2, Max: 2}})

	adapter := &mockCastInventoryAdapter{char: makeIdentifyTestCharacter(charID, items, slots)}
	h.SetInventoryAdapter(adapter)

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "identify",
		"target": "mystery-ring",
		"level":  1,
	}))

	if len(svc.castCalls) != 0 {
		t.Errorf("expected /cast identify to short-circuit (no Cast call), got %d Cast calls", len(svc.castCalls))
	}
	if len(adapter.updateInvCalls) != 1 {
		t.Fatalf("expected 1 inventory update, got %d", len(adapter.updateInvCalls))
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Identify") && !strings.Contains(sess.lastResponse.Data.Content, "Ring of Mystery") {
		t.Errorf("expected identify confirmation, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_DetectMagicShortCircuits(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()

	charID := uuid.New()
	items, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	})
	slots, _ := json.Marshal(map[string]character.SlotInfo{"1": {Current: 2, Max: 2}})

	adapter := &mockCastInventoryAdapter{char: makeIdentifyTestCharacter(charID, items, slots)}
	h.SetInventoryAdapter(adapter)

	h.Handle(makeCastInteraction(map[string]any{
		"spell": "detect-magic",
	}))

	if len(svc.castCalls) != 0 || len(svc.aoeCalls) != 0 {
		t.Errorf("expected /cast detect-magic to short-circuit; got %d cast / %d aoe calls", len(svc.castCalls), len(svc.aoeCalls))
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Ring of Protection") {
		t.Errorf("expected detect-magic to list magic items, got %q", content)
	}
	if strings.Contains(content, "Longsword") {
		t.Errorf("detect-magic should not list non-magic items, got %q", content)
	}
}

// --- F-88c: detect-magic environment scan ---

type stubNearbyScanner struct {
	groups []NearbyInventory
	err    error
	called bool
}

func (s *stubNearbyScanner) ScanNearby(_ context.Context, _, _ string, _ int) ([]NearbyInventory, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	return s.groups, nil
}

func TestCastHandler_DetectMagic_ScansNearbyInventories(t *testing.T) {
	h, sess, _, _ := setupCastHandler()

	charID := uuid.New()
	items, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	})
	slots, _ := json.Marshal(map[string]character.SlotInfo{"1": {Current: 2, Max: 2}})
	adapter := &mockCastInventoryAdapter{char: makeIdentifyTestCharacter(charID, items, slots)}
	h.SetInventoryAdapter(adapter)

	scanner := &stubNearbyScanner{
		groups: []NearbyInventory{
			{
				SourceName: "Goblin Lair Chest",
				Items: []character.InventoryItem{
					{ItemID: "wand", Name: "Wand of Magic Detection", IsMagic: true, Quantity: 1},
					{ItemID: "stone", Name: "Plain Stone", IsMagic: false, Quantity: 5},
				},
			},
			{
				SourceName: "Bjorn (PC)",
				Items: []character.InventoryItem{
					{ItemID: "ring2", Name: "Ring of Spell Storing", IsMagic: true, Quantity: 1},
				},
			},
		},
	}
	h.SetNearbyScanner(scanner)

	h.Handle(makeCastInteraction(map[string]any{
		"spell": "detect-magic",
	}))

	if !scanner.called {
		t.Fatalf("expected nearby scanner to be invoked")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Ring of Protection") {
		t.Errorf("expected caster's own magic item, got %q", content)
	}
	if !strings.Contains(content, "Wand of Magic Detection") {
		t.Errorf("expected nearby loot magic item, got %q", content)
	}
	if !strings.Contains(content, "Ring of Spell Storing") {
		t.Errorf("expected nearby PC magic item, got %q", content)
	}
	if strings.Contains(content, "Plain Stone") {
		t.Errorf("non-magic nearby items must be filtered out, got %q", content)
	}
}

func TestCastHandler_DetectMagic_NoNearbyMagic_StillReportsSelf(t *testing.T) {
	h, sess, _, _ := setupCastHandler()

	charID := uuid.New()
	items, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", IsMagic: true, Quantity: 1, Type: "magic_item"},
	})
	slots, _ := json.Marshal(map[string]character.SlotInfo{"1": {Current: 2, Max: 2}})
	adapter := &mockCastInventoryAdapter{char: makeIdentifyTestCharacter(charID, items, slots)}
	h.SetInventoryAdapter(adapter)
	h.SetNearbyScanner(&stubNearbyScanner{})

	h.Handle(makeCastInteraction(map[string]any{"spell": "detect-magic"}))
	if !strings.Contains(sess.lastResponse.Data.Content, "Ring of Protection") {
		t.Errorf("expected self-only magic to still surface, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_DetectMagic_ScannerErrorDegradesGracefully(t *testing.T) {
	h, sess, _, _ := setupCastHandler()

	charID := uuid.New()
	items, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", IsMagic: true, Quantity: 1, Type: "magic_item"},
	})
	slots, _ := json.Marshal(map[string]character.SlotInfo{"1": {Current: 2, Max: 2}})
	adapter := &mockCastInventoryAdapter{char: makeIdentifyTestCharacter(charID, items, slots)}
	h.SetInventoryAdapter(adapter)
	h.SetNearbyScanner(&stubNearbyScanner{err: errors.New("scanner offline")})

	h.Handle(makeCastInteraction(map[string]any{"spell": "detect-magic"}))
	if !strings.Contains(sess.lastResponse.Data.Content, "Ring of Protection") {
		t.Errorf("scanner error should not block self-inventory result, got %q", sess.lastResponse.Data.Content)
	}
}

func TestCastHandler_DetectMagic_NoSelfNoNearby_ReportsEmpty(t *testing.T) {
	h, sess, _, _ := setupCastHandler()

	charID := uuid.New()
	items, _ := json.Marshal([]character.InventoryItem{})
	slots, _ := json.Marshal(map[string]character.SlotInfo{"1": {Current: 2, Max: 2}})
	adapter := &mockCastInventoryAdapter{char: makeIdentifyTestCharacter(charID, items, slots)}
	h.SetInventoryAdapter(adapter)
	h.SetNearbyScanner(&stubNearbyScanner{})

	h.Handle(makeCastInteraction(map[string]any{"spell": "detect-magic"}))
	if !strings.Contains(sess.lastResponse.Data.Content, "no magical auras") {
		t.Errorf("expected empty-result fallback message, got %q", sess.lastResponse.Data.Content)
	}
}

// containsString reports whether needle is in haystack.
func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// --- E-59 AoE save-prompt tests ---

// TestCastHandler_AoEDispatch_PromptsAffectedPlayersToSave verifies that
// after CastAoE returns, the handler posts a /save prompt naming each
// player-controlled affected combatant in the combat-log channel.
func TestCastHandler_AoEDispatch_PromptsAffectedPlayersToSave(t *testing.T) {
	h, _, svc, provider := setupCastHandler()
	captured := []string{}
	h.session = &capturingSession{
		mockMoveSession: &mockMoveSession{},
		sendFn: func(channelID, content string) {
			captured = append(captured, channelID+":"+content)
		},
	}
	h.SetChannelIDProvider(&mockDeathSaveCSP{
		fn: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "ch-cl"}, nil
		},
	})

	// Use known combatant IDs so the prompt loop can resolve display names.
	playerCombatantID := provider.caster.ID
	npcCombatantID := provider.target.ID
	svc.aoeResult = combat.AoECastResult{
		CasterName:    "Aria",
		SpellName:     "Fireball",
		SpellLevel:    3,
		SaveDC:        15,
		SaveAbility:   "dex",
		AffectedNames: []string{"Aria", "Orc"},
		PendingSaves: []combat.PendingSave{
			{CombatantID: playerCombatantID, SaveAbility: "dex", DC: 15, IsNPC: false},
			{CombatantID: npcCombatantID, SaveAbility: "dex", DC: 15, IsNPC: true},
		},
	}

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fireball",
		"target": "D4",
	}))

	hasSavePrompt := false
	npcPromptCount := 0
	for _, m := range captured {
		if strings.Contains(m, "/save dex") && strings.Contains(m, "Aria") && strings.Contains(m, "15") {
			hasSavePrompt = true
		}
		if strings.Contains(m, "/save") && strings.Contains(m, "Orc") {
			npcPromptCount++
		}
	}
	if !hasSavePrompt {
		t.Errorf("expected per-player /save prompt mentioning Aria & DC 15, got: %v", captured)
	}
	if npcPromptCount != 0 {
		t.Errorf("NPC saves should not produce player /save pings, got %d Orc pings", npcPromptCount)
	}
}

// --- E-63 material-component prompt tests ---

// setupCastHandlerWithPrompts wires the handler with a MockSession + prompt
// store so the gold-fallback Buy & Cast / Cancel buttons can be inspected and
// clicked back through HandleComponent.
func setupCastHandlerWithPrompts(t *testing.T) (*CastHandler, *MockSession, *mockCastCombatService, *[]*discordgo.MessageSend) {
	t.Helper()
	mock, sent := captureSentComplex()
	encID := uuid.New()
	turnID := uuid.New()
	casterID := uuid.New()
	targetID := uuid.New()

	provider := &mockCastProvider{
		encID: encID,
		enc: refdata.Encounter{
			ID:            encID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			Status:        "active",
		},
		turn: refdata.Turn{
			ID:          turnID,
			CombatantID: casterID,
		},
		caster: refdata.Combatant{
			ID: casterID, ShortID: "AR", DisplayName: "Aria",
			PositionCol: "B", PositionRow: 2,
		},
		target: refdata.Combatant{
			ID: targetID, ShortID: "OS", DisplayName: "Orc",
			PositionCol: "D", PositionRow: 4,
		},
		spells: map[string]refdata.Spell{
			"identify": {ID: "identify", Name: "Identify", Level: 1},
		},
	}

	combatSvc := &mockCastCombatService{}
	h := NewCastHandler(mock, combatSvc, provider, dice.NewRoller(func(_ int) int { return 10 }))
	prompts := NewReactionPromptStoreWithTTL(mock, time.Hour)
	h.SetMaterialPromptStore(prompts)
	return h, mock, combatSvc, sent
}

// TestCastHandler_MaterialComponent_PromptsGoldFallback verifies that when
// CastResult.MaterialComponent.NeedsGoldConfirmation is true, the handler
// posts a Buy & Cast / Cancel confirmation rather than treating the cast as
// completed.
func TestCastHandler_MaterialComponent_PromptsGoldFallback(t *testing.T) {
	h, _, svc, sent := setupCastHandlerWithPrompts(t)
	svc.castResult = combat.CastResult{
		CasterName: "Aria",
		SpellName:  "Identify",
		SpellLevel: 1,
		MaterialComponent: &combat.CastMaterialComponentInfo{
			NeedsGoldConfirmation: true,
			ComponentName:         "pearl worth 100gp",
			CostGp:                100,
			CurrentGold:           500,
		},
	}

	h.Handle(makeCastInteraction(map[string]any{
		"spell": "identify",
	}))

	if len(svc.castCalls) != 1 {
		t.Fatalf("expected initial cast call, got %d", len(svc.castCalls))
	}
	if svc.castCalls[0].GoldFallback {
		t.Errorf("initial cast should not set GoldFallback=true")
	}
	if len(*sent) != 1 {
		t.Fatalf("expected confirmation prompt to be posted, got %d sends", len(*sent))
	}
	content := (*sent)[0].Content
	if !strings.Contains(content, "pearl") {
		t.Errorf("expected prompt to mention component name, got %q", content)
	}
	row, ok := (*sent)[0].Components[0].(discordgo.ActionsRow)
	if !ok {
		t.Fatalf("expected ActionsRow with buttons, got %+v", (*sent)[0].Components)
	}
	if len(row.Components) != 2 {
		t.Fatalf("expected 2 buttons (Buy & Cast, Cancel), got %d", len(row.Components))
	}
}

// TestCastHandler_MaterialComponent_BuyAndCastRetriesWithGoldFallback verifies
// that clicking "Buy & Cast" re-invokes Cast with GoldFallback=true.
func TestCastHandler_MaterialComponent_BuyAndCastRetriesWithGoldFallback(t *testing.T) {
	h, _, svc, sent := setupCastHandlerWithPrompts(t)
	svc.castResult = combat.CastResult{
		CasterName: "Aria",
		SpellName:  "Identify",
		SpellLevel: 1,
		MaterialComponent: &combat.CastMaterialComponentInfo{
			NeedsGoldConfirmation: true,
			ComponentName:         "pearl worth 100gp",
			CostGp:                100,
			CurrentGold:           500,
		},
	}

	h.Handle(makeCastInteraction(map[string]any{
		"spell": "identify",
	}))
	if len(*sent) != 1 {
		t.Fatalf("expected initial prompt; got %d sends", len(*sent))
	}

	// Now switch the service to return a clean success and click "Buy & Cast".
	svc.castResult = combat.CastResult{
		CasterName: "Aria",
		SpellName:  "Identify",
		SpellLevel: 1,
	}

	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	buyBtn := row.Components[0].(discordgo.Button)
	clickInteraction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: buyBtn.CustomID,
		},
	}
	if !h.HandleComponent(clickInteraction) {
		t.Fatalf("HandleComponent should claim the material-prompt button")
	}

	if len(svc.castCalls) != 2 {
		t.Fatalf("expected two cast calls (initial + buy retry), got %d", len(svc.castCalls))
	}
	if !svc.castCalls[1].GoldFallback {
		t.Errorf("retry cast should set GoldFallback=true, got %+v", svc.castCalls[1])
	}
}

// TestCastHandler_MaterialComponent_NoPromptStoreFallsBackToEphemeral
// verifies that when no prompt store is wired the handler still surfaces
// the gold-fallback notice as an ephemeral so the caster knows the cast
// didn't go through.
func TestCastHandler_MaterialComponent_NoPromptStoreFallsBackToEphemeral(t *testing.T) {
	h, sess, svc, _ := setupCastHandler()
	// Re-use the fire-bolt spell registered in setupCastHandler; what
	// matters is the handler's reaction to MaterialComponent on the result.
	svc.castResult = combat.CastResult{
		CasterName: "Aria",
		SpellName:  "Fire Bolt",
		SpellLevel: 0,
		MaterialComponent: &combat.CastMaterialComponentInfo{
			NeedsGoldConfirmation: true,
			ComponentName:         "pearl worth 100gp",
			CostGp:                100,
			CurrentGold:           500,
		},
	}

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fire-bolt",
		"target": "OS",
	}))

	if !strings.Contains(sess.lastResponse.Data.Content, "pearl") {
		t.Errorf("expected component name in ephemeral fallback, got %q", sess.lastResponse.Data.Content)
	}
}

// TestCastHandler_HandleComponent_NoPromptStoreReturnsFalse verifies the
// router contract: without a prompt store, button clicks aren't claimed.
func TestCastHandler_HandleComponent_NoPromptStoreReturnsFalse(t *testing.T) {
	h, _, _, _ := setupCastHandler()
	got := h.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: "rxprompt:abc:buy"},
	})
	if got {
		t.Errorf("expected HandleComponent to return false when no prompt store wired")
	}
}

// TestCastHandler_MaterialComponent_CancelDoesNotRetry verifies that clicking
// "Cancel" does not produce a second cast call.
func TestCastHandler_MaterialComponent_CancelDoesNotRetry(t *testing.T) {
	h, _, svc, sent := setupCastHandlerWithPrompts(t)
	svc.castResult = combat.CastResult{
		CasterName: "Aria",
		SpellName:  "Identify",
		SpellLevel: 1,
		MaterialComponent: &combat.CastMaterialComponentInfo{
			NeedsGoldConfirmation: true,
			ComponentName:         "pearl worth 100gp",
			CostGp:                100,
			CurrentGold:           500,
		},
	}

	h.Handle(makeCastInteraction(map[string]any{
		"spell": "identify",
	}))

	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	cancelBtn := row.Components[1].(discordgo.Button)
	clickInteraction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: cancelBtn.CustomID,
		},
	}
	if !h.HandleComponent(clickInteraction) {
		t.Fatalf("HandleComponent should claim the cancel button")
	}

	if len(svc.castCalls) != 1 {
		t.Errorf("cancel should not invoke a second Cast call, got %d", len(svc.castCalls))
	}
}

// --- SR-025: Metamagic interactive prompts + twin-target wiring ---

// setupCastHandlerForMetamagic wires a cast handler with a MockSession that
// captures complex message sends (so prompt buttons can be inspected), plus
// a ReactionPromptStore + MetamagicPromptPoster. Three combatants are seeded
// so AoE casts have multiple targets and the second-target lookup for
// Twinned has someone other than the caster + primary target.
func setupCastHandlerForMetamagic(t *testing.T) (*CastHandler, *MockSession, *mockCastCombatService, *mockCastProvider, *[]*discordgo.MessageSend) {
	t.Helper()
	mock, sent := captureSentComplex()

	encID := uuid.New()
	turnID := uuid.New()
	casterID := uuid.New()
	targetID := uuid.New()
	twinID := uuid.New()
	mapID := uuid.New()

	provider := &mockCastProvider{
		encID: encID,
		enc: refdata.Encounter{
			ID:            encID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			MapID:         uuid.NullUUID{UUID: mapID, Valid: true},
			Status:        "active",
		},
		turn: refdata.Turn{
			ID:          turnID,
			CombatantID: casterID,
		},
		caster: refdata.Combatant{
			ID: casterID, ShortID: "AR", DisplayName: "Aria",
			PositionCol: "B", PositionRow: 2,
		},
		target: refdata.Combatant{
			ID: targetID, ShortID: "OS", DisplayName: "Orc",
			PositionCol: "D", PositionRow: 4,
		},
		spells: map[string]refdata.Spell{
			"fire-bolt": {ID: "fire-bolt", Name: "Fire Bolt", Level: 0},
			"hold-person": {ID: "hold-person", Name: "Hold Person", Level: 2,
				SaveAbility: sql.NullString{String: "wis", Valid: true}},
			"fireball": {ID: "fireball", Name: "Fireball", Level: 3,
				AreaOfEffect: pqtype.NullRawMessage{
					RawMessage: []byte(`{"shape":"sphere","radius_ft":20}`),
					Valid:      true,
				},
				SaveAbility: sql.NullString{String: "dex", Valid: true},
			},
		},
		mapData: refdata.Map{TiledJson: []byte(minimalTiledJSON)},
	}
	// Inject a third combatant for AoE-prompt + twin-target tests via the
	// ListCombatantsByEncounterID seam. We override the list path by hand
	// because mockCastProvider returns only caster+target by default.
	provider.listCombatantsOverride = []refdata.Combatant{
		provider.caster,
		provider.target,
		{ID: twinID, ShortID: "GB", DisplayName: "Goblin", PositionCol: "C", PositionRow: 3},
	}

	combatSvc := &mockCastCombatService{
		castResult: combat.CastResult{
			CasterName: "Aria", SpellName: "Hold Person", SpellLevel: 2, TargetName: "Orc",
		},
		aoeResult: combat.AoECastResult{
			CasterName: "Aria", SpellName: "Fireball", SpellLevel: 3,
			AffectedNames: []string{"Orc", "Goblin"},
		},
	}

	h := NewCastHandler(mock, combatSvc, provider, dice.NewRoller(func(_ int) int { return 10 }))
	prompts := NewReactionPromptStoreWithTTL(mock, time.Hour)
	h.SetMaterialPromptStore(prompts)
	h.SetMetamagicPromptPoster(NewMetamagicPromptPoster(prompts))
	// Pipe combat-log posts to a known channel so prompt postings have somewhere
	// to land.
	h.SetChannelIDProvider(&mockDeathSaveCSP{
		fn: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "ch-cl"}, nil
		},
	})

	return h, mock, combatSvc, provider, sent
}

// SR-025: /cast twin-target:<short-id> resolves the second target and threads
// CastCommand.TwinTargetID into the combat service call.
func TestCastHandler_TwinTarget_ThreadsTwinTargetID(t *testing.T) {
	h, _, svc, provider, _ := setupCastHandlerForMetamagic(t)

	h.Handle(makeCastInteraction(map[string]any{
		"spell":       "hold-person",
		"target":      "OS",
		"twin-target": "GB", // Goblin
		"twin":        true,
	}))

	require.Len(t, svc.castCalls, 1, "expected one Cast call")
	got := svc.castCalls[0]
	assert.NotEqual(t, uuid.Nil, got.TwinTargetID, "TwinTargetID must be set when twin-target option is provided")
	// Verify it resolved to Goblin, not the primary Orc target.
	goblinID := provider.listCombatantsOverride[2].ID
	assert.Equal(t, goblinID, got.TwinTargetID, "TwinTargetID must point at the resolved second target")
}

// SR-025: /cast twin-target:<unknown> short-circuits with an error rather
// than silently dropping the option.
func TestCastHandler_TwinTarget_UnknownRejects(t *testing.T) {
	h, mock, svc, _, _ := setupCastHandlerForMetamagic(t)
	var lastInteractionContent string
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			lastInteractionContent = resp.Data.Content
		}
		return nil
	}

	h.Handle(makeCastInteraction(map[string]any{
		"spell":       "hold-person",
		"target":      "OS",
		"twin-target": "ZZZ",
		"twin":        true,
	}))

	assert.Len(t, svc.castCalls, 0, "should not invoke Cast when twin target unresolvable")
	assert.Contains(t, strings.ToLower(lastInteractionContent), "twin")
}

// SR-025: single-target cast with empowered metamagic posts the
// PromptEmpowered button row before invoking Cast — the cast proceeds when
// the player picks a die (or the forfeit timer fires).
func TestCastHandler_Empowered_PostsPromptAndResumesCast(t *testing.T) {
	h, _, svc, _, sent := setupCastHandlerForMetamagic(t)

	h.Handle(makeCastInteraction(map[string]any{
		"spell":     "hold-person",
		"target":    "OS",
		"empowered": true,
	}))

	require.NotEmpty(t, *sent, "expected at least one prompt message")
	last := (*sent)[len(*sent)-1]
	assert.Contains(t, last.Content, "Empowered", "prompt content should mention Empowered")
	row, ok := last.Components[0].(discordgo.ActionsRow)
	require.True(t, ok, "expected ActionsRow")
	require.NotEmpty(t, row.Components, "expected at least one button")

	// Click the first die button to resume the cast.
	btn := row.Components[0].(discordgo.Button)
	h.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: btn.CustomID},
	})

	require.Len(t, svc.castCalls, 1, "Cast should fire once the prompt resolves")
	assert.Contains(t, svc.castCalls[0].Metamagic, "empowered")
}

// SR-025: AoE cast with careful metamagic posts the PromptCareful button
// row; clicking a target threads that combatant's ID into
// AoECastCommand.CarefulTargetIDs.
func TestCastHandler_Careful_AoEPostsPromptAndThreadsTargets(t *testing.T) {
	h, _, svc, provider, sent := setupCastHandlerForMetamagic(t)

	h.Handle(makeCastInteraction(map[string]any{
		"spell":   "fireball",
		"target":  "D4",
		"careful": true,
	}))

	require.NotEmpty(t, *sent, "expected at least one prompt message")
	last := (*sent)[len(*sent)-1]
	assert.Contains(t, last.Content, "Careful")
	row, ok := last.Components[0].(discordgo.ActionsRow)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(row.Components), 2, "should list at least the two affected non-caster combatants")

	// Click the first target (Orc) to resume.
	btn := row.Components[0].(discordgo.Button)
	h.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: btn.CustomID},
	})

	require.Len(t, svc.aoeCalls, 1, "CastAoE should fire after the prompt resolves")
	got := svc.aoeCalls[0]
	require.Len(t, got.CarefulTargetIDs, 1, "expected one careful target threaded")
	orcID := provider.listCombatantsOverride[1].ID
	assert.Equal(t, orcID, got.CarefulTargetIDs[0])
}

// SR-025: AoE cast with heightened metamagic posts the PromptHeightened
// button row; clicking a target threads HeightenedTargetID into the AoE
// cast command.
func TestCastHandler_Heightened_AoEPostsPromptAndThreadsTarget(t *testing.T) {
	h, _, svc, provider, sent := setupCastHandlerForMetamagic(t)

	h.Handle(makeCastInteraction(map[string]any{
		"spell":      "fireball",
		"target":     "D4",
		"heightened": true,
	}))

	require.NotEmpty(t, *sent, "expected at least one prompt message")
	last := (*sent)[len(*sent)-1]
	assert.Contains(t, last.Content, "Heightened")
	row, ok := last.Components[0].(discordgo.ActionsRow)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(row.Components), 1)

	btn := row.Components[0].(discordgo.Button)
	h.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: btn.CustomID},
	})

	require.Len(t, svc.aoeCalls, 1)
	orcID := provider.listCombatantsOverride[1].ID
	assert.Equal(t, orcID, svc.aoeCalls[0].HeightenedTargetID)
}

// SR-025: HandleComponent must claim metamagic-poster buttons so the router
// stops fan-out. (Mirrors the SR-024 cast-handler material-prompt contract.)
func TestCastHandler_HandleComponent_ClaimsMetamagicPromptClicks(t *testing.T) {
	h, _, _, _, sent := setupCastHandlerForMetamagic(t)

	h.Handle(makeCastInteraction(map[string]any{
		"spell":     "hold-person",
		"target":    "OS",
		"empowered": true,
	}))
	require.NotEmpty(t, *sent)
	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	btn := row.Components[0].(discordgo.Button)

	claimed := h.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: btn.CustomID},
	})
	assert.True(t, claimed, "HandleComponent must claim metamagic-prompt clicks")
}

// SR-026 ------------------------------------------------------------------
//
// dm_required spells and high-level (narrative) teleports must create a
// real `dm_queue_items` row so the dashboard sees the cast. Insert-then-send
// ordering is delivered by the shared Notifier (post-SR-002); the cast
// handler only needs to invoke Notifier.Post on the success path.

// recordingCastNotifier captures dmqueue.Event posts for SR-026 assertions
// and lets a test simulate an insert failure by setting postErr.
type recordingCastNotifier struct {
	posted  []dmqueue.Event
	postErr error
}

func (r *recordingCastNotifier) Post(_ context.Context, e dmqueue.Event) (string, error) {
	r.posted = append(r.posted, e)
	if r.postErr != nil {
		return "", r.postErr
	}
	return "item-1", nil
}
func (r *recordingCastNotifier) Cancel(_ context.Context, _, _ string) error  { return nil }
func (r *recordingCastNotifier) Resolve(_ context.Context, _, _ string) error { return nil }
func (r *recordingCastNotifier) ResolveWhisper(_ context.Context, _, _ string) error {
	return nil
}
func (r *recordingCastNotifier) ResolveSkillCheckNarration(_ context.Context, _, _ string) error {
	return nil
}
func (r *recordingCastNotifier) Get(string) (dmqueue.Item, bool) { return dmqueue.Item{}, false }
func (r *recordingCastNotifier) ListPending() []dmqueue.Item    { return nil }

// setupCastHandlerWithDMQueue extends setupCastHandler with a wired Notifier
// and CampaignProvider, plus seeds an extra dm_required spell ("charm-person")
// and a high-level narrative teleport ("teleport") so SR-026 tests can drive
// both code paths without redefining the whole mock.
func setupCastHandlerWithDMQueue(t *testing.T) (*CastHandler, *mockCastCombatService, *mockCastProvider, *recordingCastNotifier, uuid.UUID) {
	t.Helper()
	h, _, svc, provider := setupCastHandler()

	// Seed: dm_required single-target spell (no AoE, no teleport).
	provider.spells["charm-person"] = refdata.Spell{
		ID:             "charm-person",
		Name:           "Charm Person",
		Level:          1,
		ResolutionMode: "dm_required",
	}
	// Seed: high-level teleport with target=group (IsDMQueueTeleport=true).
	provider.spells["teleport"] = refdata.Spell{
		ID:             "teleport",
		Name:           "Teleport",
		Level:          7,
		ResolutionMode: "dm_required",
		Teleport: pqtype.NullRawMessage{
			RawMessage: []byte(`{"target":"group","range_ft":0,"requires_sight":false}`),
			Valid:      true,
		},
	}
	// Seed: dm_required AoE spell (e.g. fog-cloud).
	provider.spells["fog-cloud"] = refdata.Spell{
		ID:             "fog-cloud",
		Name:           "Fog Cloud",
		Level:          1,
		ResolutionMode: "dm_required",
		AreaOfEffect: pqtype.NullRawMessage{
			RawMessage: []byte(`{"shape":"sphere","radius_ft":20}`),
			Valid:      true,
		},
	}

	rec := &recordingCastNotifier{}
	campID := uuid.New()
	h.SetNotifier(rec)
	h.SetCampaignProvider(&mockCheckCampaignProvider{
		fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campID}, nil
		},
	})

	// CastResult mirrors what the service would return for a dm_required spell:
	// ResolutionMode propagates from spell.ResolutionMode at spellcasting.go:565.
	svc.castResult = combat.CastResult{
		CasterName:     "Aria",
		SpellName:      "Charm Person",
		SpellLevel:     1,
		ResolutionMode: "dm_required",
	}
	svc.aoeResult = combat.AoECastResult{
		CasterName: "Aria",
		SpellName:  "Fog Cloud",
		SpellLevel: 1,
	}
	return h, svc, provider, rec, campID
}

func TestCastHandler_DMRequired_SinglePostsToDMQueue(t *testing.T) {
	h, svc, _, rec, campID := setupCastHandlerWithDMQueue(t)

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "charm-person",
		"target": "OS",
	}))

	require.Len(t, svc.castCalls, 1, "Cast must still run")
	require.Len(t, rec.posted, 1, "SR-026: dm_required cast must post one dm_queue_items row")
	ev := rec.posted[0]
	assert.Equal(t, "Aria", ev.PlayerName)
	assert.Equal(t, "g1", ev.GuildID)
	assert.Equal(t, campID.String(), ev.CampaignID,
		"SR-026: CampaignID must be threaded so PgStore.Insert can persist")
	assert.Contains(t, strings.ToLower(ev.Summary), "charm person",
		"Summary should name the cast spell")
}

func TestCastHandler_HighLevelTeleport_PostsKindNarrativeTeleport(t *testing.T) {
	h, svc, _, rec, _ := setupCastHandlerWithDMQueue(t)
	// Teleport's CastResult: Cast service flips ResolutionMode to "dm_required"
	// when teleResult.DMQueueRouted is true (spellcasting.go:646-648).
	svc.castResult = combat.CastResult{
		CasterName:     "Aria",
		SpellName:      "Teleport",
		SpellLevel:     7,
		ResolutionMode: "dm_required",
		Teleport: &combat.TeleportResult{
			DMQueueRouted: true,
		},
	}

	h.Handle(makeCastInteraction(map[string]any{
		"spell": "teleport",
	}))

	require.Len(t, rec.posted, 1, "high-level teleport must post a dm_queue_items row")
	ev := rec.posted[0]
	assert.Equal(t, dmqueue.KindNarrativeTeleport, ev.Kind,
		"narrative teleport casts must use KindNarrativeTeleport")
	assert.Equal(t, "Aria", ev.PlayerName)
}

func TestCastHandler_AutoresolveSpell_NoDMQueuePost(t *testing.T) {
	h, _, _, rec, _ := setupCastHandlerWithDMQueue(t)
	// Override to an autoresolve cast (ResolutionMode != "dm_required").
	h.combatService.(*mockCastCombatService).castResult = combat.CastResult{
		CasterName:     "Aria",
		SpellName:      "Fire Bolt",
		SpellLevel:     0,
		ResolutionMode: "auto",
	}

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fire-bolt",
		"target": "OS",
	}))

	assert.Empty(t, rec.posted, "SR-026: autoresolve spells must NOT create dm_queue rows")
}

func TestCastHandler_DMRequired_AoEPostsToQueue(t *testing.T) {
	h, _, _, rec, campID := setupCastHandlerWithDMQueue(t)

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "fog-cloud",
		"target": "D4",
	}))

	require.Len(t, rec.posted, 1, "dm_required AoE casts must post one dm_queue row")
	ev := rec.posted[0]
	assert.Equal(t, "Aria", ev.PlayerName)
	assert.Equal(t, campID.String(), ev.CampaignID)
	assert.Contains(t, strings.ToLower(ev.Summary), "fog cloud")
}

func TestCastHandler_DMRequired_CastErrorSkipsPost(t *testing.T) {
	h, svc, _, rec, _ := setupCastHandlerWithDMQueue(t)
	svc.castErr = errors.New("not enough slots")

	h.Handle(makeCastInteraction(map[string]any{
		"spell":  "charm-person",
		"target": "OS",
	}))

	assert.Empty(t, rec.posted,
		"SR-026: a failed cast must not create a dm_queue row — only successful casts post")
}

