package combat

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// postStartCombat marshals body and POSTs it at the start-combat route.
func postStartCombat(t *testing.T, r chi.Router, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// --- Start-combat "initially hidden" (ambush openers) ---
//
// A PC who was hiding in the fiction before initiative was rolled must be seated
// unseen, so their first swing gets the attacker-hidden advantage (and therefore
// Sneak Attack) and the enemy planner won't target them. Before this, the only
// way to become hidden was to spend an in-combat Hide action, so round-1 ambushes
// always resolved flat.

// hiddenStartMockStore builds a store with one template creature ("G1") and one
// PC, wired for a full StartCombat run. visibility records the IsVisible flag the
// service asked for, keyed by short ID (create path), and hiddenByID records
// UpdateCombatantVisibility calls (post-create path).
func hiddenStartMockStore(t *testing.T, templateID, encounterID, charID uuid.UUID) (*mockStore, map[string]bool, map[uuid.UUID]bool) {
	t.Helper()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID: templateID, CampaignID: uuid.New(), Name: "Ambush",
			Creatures: json.RawMessage(`[{"creature_ref_id":"goblin","short_id":"G1","display_name":"Goblin","position_col":"A","position_row":1,"quantity":1}]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID: "goblin", Name: "Goblin", Ac: 15, HpAverage: 7,
			Speed:         json.RawMessage(`{"walk":30}`),
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID: charID, Name: "Windreth", HpMax: 30, HpCurrent: 30, Ac: 15, SpeedFt: 30,
			AbilityScores: json.RawMessage(`{"str":10,"dex":18,"con":12,"int":10,"wis":12,"cha":10}`),
		}, nil
	}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: encounterID, CampaignID: arg.CampaignID, Name: arg.Name, Status: arg.Status}, nil
	}

	visibility := map[string]bool{}
	createdIDs := map[string]uuid.UUID{}
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		cID := uuid.New()
		visibility[arg.ShortID] = arg.IsVisible
		createdIDs[arg.ShortID] = cID
		return refdata.Combatant{
			ID: cID, EncounterID: arg.EncounterID, ShortID: arg.ShortID, DisplayName: arg.DisplayName,
			HpMax: arg.HpMax, HpCurrent: arg.HpCurrent, Ac: arg.Ac, IsAlive: true, IsNpc: arg.IsNpc,
			IsVisible: arg.IsVisible, Conditions: json.RawMessage(`[]`), CharacterID: arg.CharacterID,
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		out := []refdata.Combatant{}
		if id, ok := createdIDs["G1"]; ok {
			out = append(out, refdata.Combatant{
				ID: id, EncounterID: encounterID, ShortID: "G1", DisplayName: "Goblin",
				IsAlive: true, IsNpc: true, IsVisible: visibility["G1"], HpMax: 7, HpCurrent: 7,
				Conditions: json.RawMessage(`[]`), CreatureRefID: sql.NullString{String: "goblin", Valid: true},
			})
		}
		if id, ok := createdIDs["WI"]; ok {
			out = append(out, refdata.Combatant{
				ID: id, EncounterID: encounterID, ShortID: "WI", DisplayName: "Windreth",
				IsAlive: true, IsNpc: false, IsVisible: visibility["WI"], HpMax: 30, HpCurrent: 30,
				Conditions: json.RawMessage(`[]`), CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			})
		}
		return out, nil
	}
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Name: "Ambush", Status: "active", RoundNumber: 1}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}

	hiddenByID := map[uuid.UUID]bool{}
	store.updateCombatantVisibilityFn = func(ctx context.Context, arg refdata.UpdateCombatantVisibilityParams) (refdata.Combatant, error) {
		hiddenByID[arg.ID] = arg.IsVisible
		return refdata.Combatant{ID: arg.ID, IsVisible: arg.IsVisible, Conditions: json.RawMessage(`[]`)}, nil
	}

	return store, visibility, hiddenByID
}

func TestService_StartCombat_HiddenCharacterIsSeatedUnseen(t *testing.T) {
	templateID, encounterID, charID := uuid.New(), uuid.New(), uuid.New()
	store, visibility, _ := hiddenStartMockStore(t, templateID, encounterID, charID)

	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:         templateID,
		CharacterIDs:       []uuid.UUID{charID},
		CharacterPositions: map[uuid.UUID]Position{charID: {Col: "D", Row: 5}},
		HiddenCharacterIDs: []uuid.UUID{charID},
	}, dice.NewRoller(func(max int) int { return 10 }))

	require.NoError(t, err)
	assert.False(t, visibility["WI"], "a PC flagged hidden must be created with is_visible=false")
	assert.True(t, visibility["G1"], "unflagged template creatures stay visible")
}

func TestService_StartCombat_UnflaggedCharacterStaysVisible(t *testing.T) {
	templateID, encounterID, charID := uuid.New(), uuid.New(), uuid.New()
	store, visibility, _ := hiddenStartMockStore(t, templateID, encounterID, charID)

	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:         templateID,
		CharacterIDs:       []uuid.UUID{charID},
		CharacterPositions: map[uuid.UUID]Position{charID: {Col: "D", Row: 5}},
	}, dice.NewRoller(func(max int) int { return 10 }))

	require.NoError(t, err)
	assert.True(t, visibility["WI"], "no hidden flag → the PC is seated visible, as before")
}

func TestService_StartCombat_HiddenShortIDHidesTemplateCreature(t *testing.T) {
	templateID, encounterID, charID := uuid.New(), uuid.New(), uuid.New()
	store, _, hiddenByID := hiddenStartMockStore(t, templateID, encounterID, charID)

	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:         templateID,
		CharacterIDs:       []uuid.UUID{charID},
		CharacterPositions: map[uuid.UUID]Position{charID: {Col: "D", Row: 5}},
		HiddenShortIDs:     []string{"G1"},
	}, dice.NewRoller(func(max int) int { return 10 }))

	require.NoError(t, err)
	require.Len(t, hiddenByID, 1, "exactly the flagged creature is hidden")
	for _, visible := range hiddenByID {
		assert.False(t, visible, "a short ID flagged hidden is set is_visible=false")
	}
}

// A short ID also resolves PCs, so the DM can flag a PC either way.
func TestService_StartCombat_HiddenShortIDAlsoHidesPC(t *testing.T) {
	templateID, encounterID, charID := uuid.New(), uuid.New(), uuid.New()
	store, _, hiddenByID := hiddenStartMockStore(t, templateID, encounterID, charID)

	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:         templateID,
		CharacterIDs:       []uuid.UUID{charID},
		CharacterPositions: map[uuid.UUID]Position{charID: {Col: "D", Row: 5}},
		HiddenShortIDs:     []string{"WI"},
	}, dice.NewRoller(func(max int) int { return 10 }))

	require.NoError(t, err)
	assert.Len(t, hiddenByID, 1, "the PC short ID resolves to the seated PC combatant")
}

func TestService_StartCombat_HiddenListCombatantsError(t *testing.T) {
	templateID, encounterID := uuid.New(), uuid.New()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{ID: templateID, CampaignID: uuid.New(), Name: "Test", Creatures: json.RawMessage(`[]`)}, nil
	}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: encounterID, Name: arg.Name, Status: arg.Status}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return nil, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:     templateID,
		HiddenShortIDs: []string{"G1"},
	}, dice.NewRoller(func(max int) int { return 10 }))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing combatants to hide")
}

func TestService_StartCombat_HiddenUpdateVisibilityError(t *testing.T) {
	templateID, encounterID, cID := uuid.New(), uuid.New(), uuid.New()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{ID: templateID, CampaignID: uuid.New(), Name: "Test", Creatures: json.RawMessage(`[]`)}, nil
	}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: encounterID, Name: arg.Name, Status: arg.Status}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{{ID: cID, ShortID: "G1", DisplayName: "Goblin", IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`)}}, nil
	}
	store.updateCombatantVisibilityFn = func(ctx context.Context, arg refdata.UpdateCombatantVisibilityParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:     templateID,
		HiddenShortIDs: []string{"G1"},
	}, dice.NewRoller(func(max int) int { return 10 }))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "hiding combatant G1")
}

// An unknown short ID is a DM typo, not a crash: the rest of combat still starts.
func TestService_StartCombat_UnknownHiddenShortIDIsIgnored(t *testing.T) {
	templateID, encounterID, charID := uuid.New(), uuid.New(), uuid.New()
	store, _, hiddenByID := hiddenStartMockStore(t, templateID, encounterID, charID)

	svc := NewService(store)
	_, err := svc.StartCombat(context.Background(), StartCombatInput{
		TemplateID:         templateID,
		CharacterIDs:       []uuid.UUID{charID},
		CharacterPositions: map[uuid.UUID]Position{charID: {Col: "D", Row: 5}},
		HiddenShortIDs:     []string{"ZZ"},
	}, dice.NewRoller(func(max int) int { return 10 }))

	require.NoError(t, err)
	assert.Empty(t, hiddenByID, "an unmatched short ID hides nobody")
}

// The whole point of seating a rogue unseen: their opener is made with the
// unseen-attacker advantage, so Sneak Attack fires on round 1 without needing an
// in-combat Hide action first. Guards the seam between the is_visible flag and
// the Sneak Attack effect condition (advantage_or_ally_within).
func TestServiceAttack_HiddenRogueOpenerFiresSneakAttack(t *testing.T) {
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()

	classes := []CharacterClass{{Class: "Rogue", Level: 5}}
	feats := []CharacterFeature{{Name: "Sneak Attack", MechanicalEffect: "sneak_attack"}}
	char := makeCharacterWithFeats(10, 16, 3, "rapier", feats, classes)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) { return makeRapier(), nil }
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5 // every damage die: 1d8 rapier + 3d6 sneak
	})

	// Hidden attacker, plain standing target — the ONLY advantage source here is
	// is_visible=false, so the sneak dice prove the hidden seat did the work.
	attacker := refdata.Combatant{
		ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Windreth", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: false, Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: targetID, DisplayName: "Counting-house Thug", PositionCol: "B", PositionRow: 1,
		Ac: 12, IsAlive: true, IsNpc: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: uuid.New(), CombatantID: attackerID, AttacksRemaining: 1}

	result, err := NewService(ms).Attack(context.Background(), AttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, roller)

	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Contains(t, result.AdvantageReasons, "attacker hidden")
	assert.Equal(t, dice.Advantage, result.RollMode)
	// 1d8(5) + DEX(+3) + 3d6 sneak(15) = 23
	assert.Equal(t, 23, result.DamageTotal, "Sneak Attack fires off the hidden opener")
	assert.True(t, result.AttackerRevealed, "attacking reveals the ambusher")
}

// --- Wire format ---

func TestHandler_StartCombat_HiddenFieldsReachTheStore(t *testing.T) {
	templateID, encounterID, charID := uuid.New(), uuid.New(), uuid.New()
	store, visibility, hiddenByID := hiddenStartMockStore(t, templateID, encounterID, charID)
	_, r := newTestCombatRouter(store)

	rec := postStartCombat(t, r, map[string]any{
		"template_id":                templateID.String(),
		"character_ids":              []string{charID.String()},
		"character_positions":        map[string]any{charID.String(): map[string]any{"col": "D", "row": 5}},
		"hidden_character_ids":       []string{charID.String()},
		"hidden_combatant_short_ids": []string{"G1"},
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.False(t, visibility["WI"], "hidden_character_ids seats the PC unseen")
	assert.Len(t, hiddenByID, 1, "hidden_combatant_short_ids hides the flagged creature")
}

func TestHandler_StartCombat_InvalidHiddenCharacterID(t *testing.T) {
	templateID, encounterID, charID := uuid.New(), uuid.New(), uuid.New()
	store, _, _ := hiddenStartMockStore(t, templateID, encounterID, charID)
	_, r := newTestCombatRouter(store)

	rec := postStartCombat(t, r, map[string]any{
		"template_id":          templateID.String(),
		"character_ids":        []string{charID.String()},
		"hidden_character_ids": []string{"not-a-uuid"},
	})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// The DM's live tracker must show who is still unseen, otherwise a hidden
// combatant is invisible to the DM too and the state silently stops being
// honoured at the table.
func TestBuildCombatStateResponse_CarriesIsVisible(t *testing.T) {
	enc := refdata.Encounter{ID: uuid.New(), Status: "active", RoundNumber: 2}
	resp := buildCombatStateResponse(enc, []refdata.Combatant{
		{ID: uuid.New(), ShortID: "WI", DisplayName: "Windreth", IsVisible: false, IsAlive: true},
		{ID: uuid.New(), ShortID: "FO", DisplayName: "Forge", IsVisible: true, IsAlive: true},
	}, uuid.Nil)

	require.Len(t, resp.Combatants, 2)
	assert.False(t, resp.Combatants[0].IsVisible, "the hidden PC reports is_visible=false")
	assert.True(t, resp.Combatants[1].IsVisible)
}

// The DM needs to see who is unseen in the start-combat response to confirm the
// ambush took; is_visible was previously dropped on the floor by the API.
func TestToCombatantResponses_CarriesIsVisible(t *testing.T) {
	out := toCombatantResponses([]refdata.Combatant{
		{ID: uuid.New(), ShortID: "WI", DisplayName: "Windreth", IsVisible: false},
		{ID: uuid.New(), ShortID: "FO", DisplayName: "Forge", IsVisible: true},
	})

	require.Len(t, out, 2)
	assert.False(t, out[0].IsVisible, "hidden combatant reports is_visible=false")
	assert.True(t, out[1].IsVisible)
}
