package discord

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// SR-024 — Paladin Aura of Protection.
//
// L6+ paladin grants their CHA mod to saves of self and allies within 10 ft
// (30 ft at L18). These tests drive `nearbyPaladinAuras` against a small
// encounter fixture: a paladin combatant + an ally combatant. The ally's
// FES is built via the registry (BuildFeatureDefinitions +
// ResolvePaladinAura) — NO test constructs a `FeatureDefinition{Name: "Aura
// of Protection", ...}` literal.

// mockSaveNearbyPaladinLookup is a minimal stand-in for the SaveNearbyPaladinLookup
// interface used by /save to fetch other PCs' character rows by ID.
type mockSaveNearbyPaladinLookup struct {
	byID map[uuid.UUID]refdata.Character
}

func (m *mockSaveNearbyPaladinLookup) GetCharacter(_ context.Context, id uuid.UUID) (refdata.Character, error) {
	c, ok := m.byID[id]
	if !ok {
		return refdata.Character{}, errNoEncounter
	}
	return c, nil
}

// makePaladinChar fabricates a Paladin character with the given level + CHA.
func makePaladinChar(t *testing.T, level int, cha int) refdata.Character {
	t.Helper()
	classes, err := json.Marshal([]combat.CharacterClass{{Class: "Paladin", Level: level}})
	if err != nil {
		t.Fatal(err)
	}
	scores, err := json.Marshal(character.AbilityScores{STR: 16, DEX: 10, CON: 12, INT: 8, WIS: 10, CHA: cha})
	if err != nil {
		t.Fatal(err)
	}
	return refdata.Character{
		ID:            uuid.New(),
		Name:          "Sir Aura",
		Classes:       classes,
		AbilityScores: scores,
	}
}

// makeCombatantAt places a combatant at the given grid coordinate.
func makeCombatantAt(charID uuid.UUID, col string, row int32) refdata.Combatant {
	return refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		PositionCol: col,
		PositionRow: row,
		IsAlive:     true,
	}
}

// SR-024 — Paladin L6 (CHA 16 / +3), ally within 10 ft → ally's FES gains
// the paladin's +3 aura via the registry.
func TestNearbyPaladinAuras_WithinRadius_L6_AddsCHAMod(t *testing.T) {
	encounterID := uuid.New()
	allyID := uuid.New()

	paladin := makePaladinChar(t, 6, 16) // +3
	palComb := makeCombatantAt(paladin.ID, "A", 1)
	allyComb := makeCombatantAt(allyID, "B", 1) // 5 ft Chebyshev

	combLookup := &mockCheckCombatantLookup{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{palComb, allyComb}, nil
		},
	}
	palLookup := &mockSaveNearbyPaladinLookup{byID: map[uuid.UUID]refdata.Character{paladin.ID: paladin}}

	defs := nearbyPaladinAuras(context.Background(), combLookup, palLookup, encounterID, allyID)

	if len(defs) != 1 {
		t.Fatalf("expected 1 aura def, got %d (%+v)", len(defs), defs)
	}
	if defs[0].Name != "Aura of Protection" {
		t.Errorf("Name = %q, want Aura of Protection", defs[0].Name)
	}
	if defs[0].Effects[0].Modifier != 3 {
		t.Errorf("Modifier = %d, want 3", defs[0].Effects[0].Modifier)
	}
}

// SR-024 — Paladin L6, ally 15 ft away (outside 10-ft aura) → no aura.
func TestNearbyPaladinAuras_OutsideRadius_L6_NoBonus(t *testing.T) {
	encounterID := uuid.New()
	allyID := uuid.New()

	paladin := makePaladinChar(t, 6, 16)
	palComb := makeCombatantAt(paladin.ID, "A", 1)
	allyComb := makeCombatantAt(allyID, "D", 1) // 15 ft Chebyshev

	combLookup := &mockCheckCombatantLookup{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{palComb, allyComb}, nil
		},
	}
	palLookup := &mockSaveNearbyPaladinLookup{byID: map[uuid.UUID]refdata.Character{paladin.ID: paladin}}

	defs := nearbyPaladinAuras(context.Background(), combLookup, palLookup, encounterID, allyID)

	if len(defs) != 0 {
		t.Errorf("expected no aura defs outside 10 ft, got %d (%+v)", len(defs), defs)
	}
}

// SR-024 — Paladin L18 extends radius to 30 ft. Same ally 25 ft away → aura
// applies (would NOT apply at L6).
func TestNearbyPaladinAuras_L18ExtendedRadius_AppliesAt25Ft(t *testing.T) {
	encounterID := uuid.New()
	allyID := uuid.New()

	paladin := makePaladinChar(t, 18, 16) // +3
	palComb := makeCombatantAt(paladin.ID, "A", 1)
	allyComb := makeCombatantAt(allyID, "F", 1) // 25 ft Chebyshev

	combLookup := &mockCheckCombatantLookup{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{palComb, allyComb}, nil
		},
	}
	palLookup := &mockSaveNearbyPaladinLookup{byID: map[uuid.UUID]refdata.Character{paladin.ID: paladin}}

	defs := nearbyPaladinAuras(context.Background(), combLookup, palLookup, encounterID, allyID)
	if len(defs) != 1 {
		t.Fatalf("expected aura def at 25 ft for L18 paladin, got %d (%+v)", len(defs), defs)
	}
	if defs[0].Effects[0].Modifier != 3 {
		t.Errorf("Modifier = %d, want 3", defs[0].Effects[0].Modifier)
	}

	// Sanity: same setup with the paladin at L6 → no aura at 25 ft.
	paladin6 := makePaladinChar(t, 6, 16)
	paladin6.ID = paladin.ID
	palLookup6 := &mockSaveNearbyPaladinLookup{byID: map[uuid.UUID]refdata.Character{paladin6.ID: paladin6}}
	if defs6 := nearbyPaladinAuras(context.Background(), combLookup, palLookup6, encounterID, allyID); len(defs6) != 0 {
		t.Errorf("L6 paladin at 25 ft must not project aura, got %+v", defs6)
	}
}

// --- SR-024: end-to-end /save Handle()-level integration ---
//
// The three tests below mirror SR-022's WildShaped/NotWildShaped/degradation
// triad. They drive `SaveHandler.Handle()` (not the bare helper) so the
// reviewer-blocking "production /save still grants no aura" failure mode is
// covered: the ally's response message must include the paladin's +3 CHA
// mod when (and only when) the aura is in range and the lookups are wired.
// d20 is pinned to 10; ally is DEX 14 (+2), no DEX-save proficiency → base
// total 12. With aura → 15; without → 12.

// allyCampaignID is the campaign for the ally invoking /save.
type auraIntegrationFixture struct {
	campaignID  uuid.UUID
	encounterID uuid.UUID
	ally        refdata.Character
	paladin     refdata.Character
	combatants  []refdata.Combatant
}

// buildAuraIntegrationFixture wires an ally + L6 paladin (CHA 16 → +3) with
// the two combatants placed `colDistFt` feet apart on the same row.
func buildAuraIntegrationFixture(t *testing.T, colDistFt int) auraIntegrationFixture {
	t.Helper()
	campaignID := uuid.New()
	encounterID := uuid.New()

	allyScores, err := json.Marshal(character.AbilityScores{STR: 10, DEX: 14, CON: 12, INT: 10, WIS: 10, CHA: 10})
	if err != nil {
		t.Fatal(err)
	}
	ally := refdata.Character{
		ID:               uuid.New(),
		CampaignID:       campaignID,
		Name:             "Ally",
		Level:            5,
		ProficiencyBonus: 3,
		AbilityScores:    allyScores,
	}
	paladin := makePaladinChar(t, 6, 16) // +3 CHA mod
	paladin.CampaignID = campaignID

	// Place ally at col "A" and paladin at the column matching colDistFt
	// (5 ft per grid cell, Chebyshev). col "A" + N cells → rune('A'+N).
	cells := colDistFt / 5
	palCol := string(rune('A' + cells))
	allyComb := makeCombatantAt(ally.ID, "A", 1)
	palComb := makeCombatantAt(paladin.ID, palCol, 1)

	return auraIntegrationFixture{
		campaignID:  campaignID,
		encounterID: encounterID,
		ally:        ally,
		paladin:     paladin,
		combatants:  []refdata.Combatant{allyComb, palComb},
	}
}

// newAuraSaveHandler wires a SaveHandler for the ally's /save command using
// the fixture. wirePaladinLookup=false simulates the degradation case (e.g.
// before SR-024 was wired) — the aura must not apply.
func newAuraSaveHandler(t *testing.T, sess *MockSession, f auraIntegrationFixture, wirePaladinLookup bool) *SaveHandler {
	t.Helper()
	roller := dice.NewRoller(func(_ int) int { return 10 })
	h := NewSaveHandler(
		sess, roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: f.campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return f.ally, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return f.encounterID, nil
		}},
		&mockCheckCombatantLookup{listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return f.combatants, nil
		}},
		&mockCheckRollLogger{},
	)
	if wirePaladinLookup {
		h.SetNearbyPaladinLookup(&mockSaveNearbyPaladinLookup{byID: map[uuid.UUID]refdata.Character{f.paladin.ID: f.paladin}})
	}
	return h
}

// TestSaveHandler_NearbyPaladin_WithinRadius_AddsCHAMod — paladin L6 (CHA 16)
// + ally 5 ft away → ally's /save dex total = 10 + 2 + 3 = 15.
func TestSaveHandler_NearbyPaladin_WithinRadius_AddsCHAMod(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	f := buildAuraIntegrationFixture(t, 5)
	h := newAuraSaveHandler(t, sess, f, true)

	h.Handle(makeSaveInteraction("dex", false, false))

	if !strings.Contains(responded, "**15**") {
		t.Fatalf("expected aura-boosted DEX save total 15 (10 + 2 DEX + 3 CHA aura), got: %s", responded)
	}
}

// TestSaveHandler_NearbyPaladin_OutsideRadius_NoBonus — paladin L6 + ally
// 15 ft away (outside 10-ft aura) → no bonus. Total = 12.
func TestSaveHandler_NearbyPaladin_OutsideRadius_NoBonus(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	f := buildAuraIntegrationFixture(t, 15)
	h := newAuraSaveHandler(t, sess, f, true)

	h.Handle(makeSaveInteraction("dex", false, false))

	if !strings.Contains(responded, "**12**") {
		t.Fatalf("expected unboosted DEX save total 12 (10 + 2 DEX, no aura at 15 ft), got: %s", responded)
	}
	if strings.Contains(responded, "**15**") {
		t.Fatalf("aura must NOT apply at 15 ft, but total looks boosted: %s", responded)
	}
}

// TestSaveHandler_NearbyPaladin_LookupUnwired_NoBonus — paladin L6 + ally
// in range, but `SetNearbyPaladinLookup` was never called → handler degrades
// silently and the aura does not apply. Mirrors SR-022's degradation case
// (creatureLookup nil → druid scores) and SR-006's silent-skip convention.
func TestSaveHandler_NearbyPaladin_LookupUnwired_NoBonus(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	f := buildAuraIntegrationFixture(t, 5)
	h := newAuraSaveHandler(t, sess, f, false) // paladinLookup nil

	h.Handle(makeSaveInteraction("dex", false, false))

	if !strings.Contains(responded, "**12**") {
		t.Fatalf("expected unboosted DEX save total 12 when paladinLookup is unwired, got: %s", responded)
	}
}
