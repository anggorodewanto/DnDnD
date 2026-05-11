package discord

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// --- C-30 occupant size: buildOccupants resolves each occupant's actual size ---

func TestBuildOccupants_UsesSizeLookup_PerCombatant(t *testing.T) {
	moverID := uuid.New()
	smallID := uuid.New()
	largeID := uuid.New()

	mover := refdata.Combatant{ID: moverID, PositionCol: "A", PositionRow: 1, IsAlive: true}
	all := []refdata.Combatant{
		mover,
		{ID: smallID, PositionCol: "B", PositionRow: 1, IsAlive: true},
		{ID: largeID, PositionCol: "C", PositionRow: 1, IsAlive: true},
	}

	sizeByID := map[uuid.UUID]int{
		smallID: pathfinding.SizeTiny,
		largeID: pathfinding.SizeLarge,
	}
	occupants := buildOccupants(all, mover, func(c refdata.Combatant) int {
		return sizeByID[c.ID]
	})
	require.Len(t, occupants, 2)
	gotByCol := map[int]int{}
	for _, o := range occupants {
		gotByCol[o.Col] = o.SizeCategory
	}
	assert.Equal(t, pathfinding.SizeTiny, gotByCol[1], "Tiny occupant in column B")
	assert.Equal(t, pathfinding.SizeLarge, gotByCol[2], "Large occupant in column C")
}

func TestBuildOccupants_NilSizeFn_FallsBackToMedium(t *testing.T) {
	mover := refdata.Combatant{ID: uuid.New(), PositionCol: "A", PositionRow: 1, IsAlive: true}
	all := []refdata.Combatant{
		mover,
		{ID: uuid.New(), PositionCol: "B", PositionRow: 1, IsAlive: true},
	}
	occupants := buildOccupants(all, mover, nil)
	require.Len(t, occupants, 1)
	assert.Equal(t, pathfinding.SizeMedium, occupants[0].SizeCategory)
}

func TestMoveHandler_BuildOccupants_RoutesThroughWiredSizeLookup(t *testing.T) {
	// Wire a size lookup that returns SizeTiny for one specific NPC. After
	// /move runs, the lookup should be invoked once per occupant
	// (excluding the mover and dead combatants).
	sess := &mockMoveSession{}
	handler, encounterID, turnID, moverID := setupMoveHandler(sess)

	tinyID := uuid.New()
	mover := refdata.Combatant{ID: moverID, PositionCol: "A", PositionRow: 1, IsAlive: true, IsNpc: false, HpCurrent: 10}
	tiny := refdata.Combatant{ID: tinyID, PositionCol: "B", PositionRow: 1, IsAlive: true, IsNpc: true, HpCurrent: 5}

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
				MapID:         uuid.NullUUID{UUID: uuid.New(), Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == tinyID {
				return tiny, nil
			}
			return mover, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{mover, tiny}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	var seen []uuid.UUID
	handler.SetSizeSpeedLookup(&recordingSizeLookup{
		fn: func(c refdata.Combatant) (int, int, error) {
			seen = append(seen, c.ID)
			if c.ID == tinyID {
				return pathfinding.SizeTiny, 30, nil
			}
			return pathfinding.SizeMedium, 30, nil
		},
	})

	handler.Handle(makeMoveInteraction("D1"))

	// The handler resolves the mover's size, then iterates the occupants
	// (tiny) — both must hit the lookup at least once.
	require.NotEmpty(t, seen, "expected sizeSpeedLookup to be invoked")
	found := false
	for _, id := range seen {
		if id == tinyID {
			found = true
		}
	}
	assert.True(t, found, "expected the tiny NPC occupant to be resolved through the size lookup")
}

type recordingSizeLookup struct {
	fn func(refdata.Combatant) (int, int, error)
}

func (r *recordingSizeLookup) LookupSizeAndSpeed(_ context.Context, c refdata.Combatant) (int, int, error) {
	return r.fn(c)
}

// --- C-32 range-rejection format ---

func TestAttackHandler_OutOfRange_UsesFormatRangeRejection(t *testing.T) {
	h, sess, svc, _ := setupAttackHandler()
	svc.attackErr = errors.New("out of range: 65ft away (max 60ft)")

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	got := sess.lastResponse.Data.Content
	want := combat.FormatRangeRejection(65, 60)
	if got != want {
		t.Errorf("expected formatted range-rejection helper output, got %q want %q", got, want)
	}
}

func TestAttackHandler_OffhandOutOfRange_UsesFormatRangeRejection(t *testing.T) {
	h, sess, svc, _ := setupAttackHandler()
	svc.offhandErr = errors.New("out of range: 30ft away (max 20ft)")

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS", "offhand": true}))

	got := sess.lastResponse.Data.Content
	want := combat.FormatRangeRejection(30, 20)
	if got != want {
		t.Errorf("expected formatted range-rejection helper output, got %q want %q", got, want)
	}
}

func TestAttackHandler_OtherErrors_KeepLegacyWording(t *testing.T) {
	h, sess, svc, _ := setupAttackHandler()
	svc.attackErr = errors.New("out of attacks")

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	if !strings.Contains(sess.lastResponse.Data.Content, "Attack failed") {
		t.Errorf("expected legacy 'Attack failed' wording, got %q", sess.lastResponse.Data.Content)
	}
}

// --- C-33-followup: AttackCommand.Walls populated from map provider ---

type stubAttackMapProvider struct {
	mapData refdata.Map
	err     error
}

func (s *stubAttackMapProvider) GetMapByID(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
	return s.mapData, s.err
}

func TestAttackHandler_PopulatesWallsFromMap(t *testing.T) {
	h, _, svc, provider := setupAttackHandler()
	mapID := uuid.New()
	provider.enc.MapID = uuid.NullUUID{UUID: mapID, Valid: true}

	// A 3x3 tiled map with one wall segment between B1 and C1.
	tiled := json.RawMessage(`{
		"width": 3, "height": 3, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 3, "height": 3,
			 "data": [1,1,1, 1,1,1, 1,1,1]},
			{"name": "walls", "type": "objectgroup",
			 "objects": [{"x": 96, "y": 0, "width": 0, "height": 48}]}
		],
		"tilesets": [{"firstgid": 1, "name": "base", "tiles": [{"id": 0, "type": "open_ground"}]}]
	}`)
	h.SetMapProvider(&stubAttackMapProvider{mapData: refdata.Map{ID: mapID, TiledJson: tiled}})

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	require.Len(t, svc.attackCalls, 1, "expected attack to dispatch")
	assert.NotEmpty(t, svc.attackCalls[0].Walls, "expected AttackCommand.Walls populated from map")
}

func TestAttackHandler_NoMapProvider_WallsRemainNil(t *testing.T) {
	h, _, svc, _ := setupAttackHandler()
	// Deliberately no SetMapProvider call.
	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))
	require.Len(t, svc.attackCalls, 1)
	assert.Nil(t, svc.attackCalls[0].Walls, "expected nil Walls when no map provider wired")
}

func TestAttackHandler_OffhandPopulatesWallsFromMap(t *testing.T) {
	h, _, svc, provider := setupAttackHandler()
	mapID := uuid.New()
	provider.enc.MapID = uuid.NullUUID{UUID: mapID, Valid: true}

	tiled := json.RawMessage(`{
		"width": 3, "height": 3, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 3, "height": 3,
			 "data": [1,1,1, 1,1,1, 1,1,1]},
			{"name": "walls", "type": "objectgroup",
			 "objects": [{"x": 96, "y": 0, "width": 0, "height": 48}]}
		],
		"tilesets": [{"firstgid": 1, "name": "base", "tiles": [{"id": 0, "type": "open_ground"}]}]
	}`)
	h.SetMapProvider(&stubAttackMapProvider{mapData: refdata.Map{ID: mapID, TiledJson: tiled}})

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS", "offhand": true}))

	require.Len(t, svc.offhandCalls, 1)
	assert.NotEmpty(t, svc.offhandCalls[0].Walls)
}

// --- C-40 frightened-move rejects approaching the fear source ---

func frightenedConditionsRaw(t *testing.T, sourceID uuid.UUID) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal([]map[string]interface{}{
		{
			"condition":           "frightened",
			"source_combatant_id": sourceID.String(),
		},
	})
	require.NoError(t, err)
	return raw
}

func TestMoveHandler_Frightened_BlocksApproachToSource(t *testing.T) {
	sess := &mockMoveSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	moverID := uuid.New()
	scaryID := uuid.New()
	mapID := uuid.New()

	mover := refdata.Combatant{
		ID:          moverID,
		PositionCol: "C", PositionRow: 3,
		IsAlive: true, IsNpc: false, HpCurrent: 10,
		Conditions: frightenedConditionsRaw(t, scaryID),
	}
	scary := refdata.Combatant{
		ID:          scaryID,
		PositionCol: "E", PositionRow: 3,
		IsAlive: true, IsNpc: true, HpCurrent: 30,
	}

	combatSvc := &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
				MapID:         uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return mover, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{mover, scary}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}
	mapProv := &mockMoveMapProvider{getByID: func(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
		return refdata.Map{ID: mapID, WidthSquares: 5, HeightSquares: 5, TiledJson: tiledJSON5x5()}, nil
	}}
	turnProv := &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: moverID, MovementRemainingFt: 30}, nil
		},
	}
	encProv := &mockMoveEncounterProvider{
		activeEncounterForUser: func(_ context.Context, _, _ string) (uuid.UUID, error) { return encounterID, nil },
	}
	handler := NewMoveHandler(sess, combatSvc, mapProv, turnProv, encProv, nil)

	// Move from C3 to D3 — closer to E3 → must reject.
	handler.Handle(makeMoveInteraction("D3"))

	require.NotNil(t, sess.lastResponse)
	got := sess.lastResponse.Data.Content
	assert.Contains(t, got, "source of your fear", "expected frightened rejection, got: %s", got)
}

func TestMoveHandler_Frightened_AllowsMoveAwayFromSource(t *testing.T) {
	sess := &mockMoveSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	moverID := uuid.New()
	scaryID := uuid.New()
	mapID := uuid.New()

	mover := refdata.Combatant{
		ID:          moverID,
		PositionCol: "C", PositionRow: 3,
		IsAlive: true, IsNpc: false, HpCurrent: 10,
		Conditions: frightenedConditionsRaw(t, scaryID),
	}
	scary := refdata.Combatant{
		ID:          scaryID,
		PositionCol: "E", PositionRow: 3,
		IsAlive: true, IsNpc: true, HpCurrent: 30,
	}

	combatSvc := &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
				MapID:         uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return mover, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{mover, scary}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}
	mapProv := &mockMoveMapProvider{getByID: func(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
		return refdata.Map{ID: mapID, WidthSquares: 5, HeightSquares: 5, TiledJson: tiledJSON5x5()}, nil
	}}
	turnProv := &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: moverID, MovementRemainingFt: 30}, nil
		},
	}
	encProv := &mockMoveEncounterProvider{
		activeEncounterForUser: func(_ context.Context, _, _ string) (uuid.UUID, error) { return encounterID, nil },
	}
	handler := NewMoveHandler(sess, combatSvc, mapProv, turnProv, encProv, nil)

	// Move from C3 to B3 — farther from E3 → must allow.
	handler.Handle(makeMoveInteraction("B3"))

	require.NotNil(t, sess.lastResponse)
	got := sess.lastResponse.Data.Content
	assert.NotContains(t, got, "source of your fear", "should allow move away, got: %s", got)
	assert.Contains(t, got, "Move to B3")
}

func TestMoveHandler_NotFrightened_NoRejection(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	handler.Handle(makeMoveInteraction("D1"))

	require.NotNil(t, sess.lastResponse)
	got := sess.lastResponse.Data.Content
	assert.NotContains(t, got, "source of your fear")
}

// --- C-43-block-commands: dying / unconscious actors are blocked ---

func dyingCombatant(id uuid.UUID) refdata.Combatant {
	dsBytes, _ := json.Marshal(map[string]int{"successes": 0, "failures": 1})
	return refdata.Combatant{
		ID:          id,
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		HpCurrent:   0,
		DeathSaves:  pqtype.NullRawMessage{RawMessage: dsBytes, Valid: true},
	}
}

func unconsciousCombatant(id uuid.UUID) refdata.Combatant {
	condsRaw, _ := json.Marshal([]map[string]interface{}{{"condition": "unconscious"}})
	return refdata.Combatant{
		ID:          id,
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		HpCurrent:   5,
		Conditions:  condsRaw,
	}
}

func TestMoveHandler_DyingCombatant_Blocked(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID := setupMoveHandler(sess)
	dying := dyingCombatant(combatantID)
	handler.combatService = &mockMoveService{
		getEncounter:   handler.combatService.(*mockMoveService).getEncounter,
		getCombatant:   func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return dying, nil },
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return []refdata.Combatant{dying}, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	handler.Handle(makeMoveInteraction("D1"))
	require.NotNil(t, sess.lastResponse)
	assert.Contains(t, sess.lastResponse.Data.Content, "dying")
}

func TestMoveHandler_UnconsciousCombatant_Blocked(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID := setupMoveHandler(sess)
	uncon := unconsciousCombatant(combatantID)
	handler.combatService = &mockMoveService{
		getEncounter:   handler.combatService.(*mockMoveService).getEncounter,
		getCombatant:   func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return uncon, nil },
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return []refdata.Combatant{uncon}, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	handler.Handle(makeMoveInteraction("D1"))
	require.NotNil(t, sess.lastResponse)
	assert.Contains(t, sess.lastResponse.Data.Content, "incapacitated")
}

func TestAttackHandler_DyingCombatant_Blocked(t *testing.T) {
	h, sess, svc, provider := setupAttackHandler()
	provider.attacker = dyingCombatant(provider.attacker.ID)

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	if len(svc.attackCalls) != 0 {
		t.Errorf("expected no attack dispatched when attacker is dying, got %d", len(svc.attackCalls))
	}
	assert.Contains(t, sess.lastResponse.Data.Content, "dying")
}

func TestActionHandler_DyingCombatant_BlocksDispatch(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	dsBytes, _ := json.Marshal(map[string]int{"successes": 0, "failures": 1})
	dying := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive:     true,
		HpCurrent:   0,
		DeathSaves:  pqtype.NullRawMessage{RawMessage: dsBytes, Valid: true},
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		Status:        "active",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters: map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants: map[uuid.UUID]refdata.Combatant{combatantID: dying},
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}
	camp := &fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}}
	chars := &fakeActionCharacterLookup{char: refdata.Character{ID: charID}}

	h := NewActionHandler(sess, &fakeActionEncounterResolver{encounterID: encounterID}, svc, turnProv, camp, chars, &fakeActionPendingStore{})
	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "dash", ""))

	if len(svc.dashCalls) != 0 {
		t.Errorf("expected dash NOT to dispatch while actor is dying")
	}
	assert.Contains(t, resp, "dying")
}

// --- C-43 stabilize: /action stabilize <target> wires StabilizeTarget ---

type fakeStabilizeStore struct {
	calls []refdata.UpdateCombatantDeathSavesParams
	err   error
}

func (f *fakeStabilizeStore) UpdateCombatantDeathSaves(_ context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
	f.calls = append(f.calls, arg)
	if f.err != nil {
		return refdata.Combatant{}, f.err
	}
	return refdata.Combatant{}, nil
}

func setupStabilizeActionHandler(t *testing.T, rollerVal int, target refdata.Combatant) (*ActionHandler, *MockSession, *fakeStabilizeStore, *fakeActionCombatService) {
	t.Helper()
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	actorID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	actor := refdata.Combatant{
		ID:          actorID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive:     true,
		HpCurrent:   18,
		ShortID:     "AR",
		PositionCol: "A",
		PositionRow: 1,
	}
	target.EncounterID = encounterID
	if target.PositionCol == "" {
		target.PositionCol = "A"
		target.PositionRow = 2 // adjacent (5ft)
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: actorID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		Status:        "active",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{actorID: actor, target.ID: target},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {actor, target}},
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}
	camp := &fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}}
	chars := &fakeActionCharacterLookup{char: refdata.Character{ID: charID}}

	h := NewActionHandler(sess, &fakeActionEncounterResolver{encounterID: encounterID}, svc, turnProv, camp, chars, &fakeActionPendingStore{})
	h.SetRoller(dice.NewRoller(func(_ int) int { return rollerVal }))
	store := &fakeStabilizeStore{}
	h.SetStabilizeStore(store)
	return h, sess, store, svc
}

func TestActionHandler_Stabilize_SuccessPersistsThreeSuccesses(t *testing.T) {
	dyingID := uuid.New()
	dsBytes, _ := json.Marshal(map[string]int{"successes": 0, "failures": 1})
	target := refdata.Combatant{
		ID:          dyingID,
		ShortID:     "DY",
		DisplayName: "Fallen",
		IsAlive:     true,
		HpCurrent:   0,
		DeathSaves:  pqtype.NullRawMessage{RawMessage: dsBytes, Valid: true},
	}
	h, _, store, _ := setupStabilizeActionHandler(t, 18, target) // 18 ≥ DC 10

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "stabilize", "DY"))

	require.Len(t, store.calls, 1, "expected exactly one death-save update on success, got: %s", resp)
	var ds combat.DeathSaves
	require.NoError(t, json.Unmarshal(store.calls[0].DeathSaves.RawMessage, &ds))
	assert.Equal(t, 3, ds.Successes, "stabilize must persist 3 successes")
	assert.Contains(t, resp, "stabilized")
}

func TestActionHandler_Stabilize_FailureDoesNotPersist(t *testing.T) {
	dyingID := uuid.New()
	dsBytes, _ := json.Marshal(map[string]int{"successes": 0, "failures": 1})
	target := refdata.Combatant{
		ID:          dyingID,
		ShortID:     "DY",
		DisplayName: "Fallen",
		IsAlive:     true,
		HpCurrent:   0,
		DeathSaves:  pqtype.NullRawMessage{RawMessage: dsBytes, Valid: true},
	}
	h, _, store, _ := setupStabilizeActionHandler(t, 5, target) // 5 < DC 10

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "stabilize", "DY"))

	assert.Len(t, store.calls, 0, "failed stabilize must not persist death saves")
	assert.Contains(t, resp, "fails")
}

func TestActionHandler_Stabilize_TargetNotDying_Rejected(t *testing.T) {
	healthy := refdata.Combatant{
		ID:          uuid.New(),
		ShortID:     "OK",
		DisplayName: "Healthy",
		IsAlive:     true,
		HpCurrent:   20,
	}
	h, _, store, _ := setupStabilizeActionHandler(t, 18, healthy)
	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "stabilize", "OK"))
	assert.Len(t, store.calls, 0)
	assert.Contains(t, resp, "not dying")
}

func TestActionHandler_Stabilize_OutOfReach_Rejected(t *testing.T) {
	dyingID := uuid.New()
	dsBytes, _ := json.Marshal(map[string]int{"successes": 0, "failures": 1})
	target := refdata.Combatant{
		ID:          dyingID,
		ShortID:     "DY",
		DisplayName: "Fallen",
		IsAlive:     true,
		HpCurrent:   0,
		DeathSaves:  pqtype.NullRawMessage{RawMessage: dsBytes, Valid: true},
		PositionCol: "E", // 4 cols + 1 row = 4 squares = 20ft from A1
		PositionRow: 2,
	}
	h, _, store, _ := setupStabilizeActionHandler(t, 18, target)
	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "stabilize", "DY"))
	assert.Len(t, store.calls, 0)
	assert.Contains(t, resp, "5ft")
}

func TestActionHandler_Stabilize_NoStore_ReportsUnavailable(t *testing.T) {
	dyingID := uuid.New()
	target := refdata.Combatant{
		ID:          dyingID,
		ShortID:     "DY",
		DisplayName: "Fallen",
		IsAlive:     true,
		HpCurrent:   0,
	}
	h, _, _, _ := setupStabilizeActionHandler(t, 18, target)
	h.stabilizeStore = nil

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "stabilize", "DY"))
	assert.Contains(t, resp, "not available")
}

// --- C-32 helper unit test ---

func TestRangeRejectionMessage_ParsesAttackError(t *testing.T) {
	msg, ok := rangeRejectionMessage(errors.New("out of range: 25ft away (max 20ft)"))
	require.True(t, ok)
	assert.Equal(t, combat.FormatRangeRejection(25, 20), msg)
}

func TestRangeRejectionMessage_NonRangeError(t *testing.T) {
	_, ok := rangeRejectionMessage(errors.New("some other failure"))
	assert.False(t, ok)
}

// --- ensure compile-time uses of imports stay live ---

var _ pathfinding.Occupant
var _ renderer.WallSegment
var _ = strings.HasPrefix
var _ = (*discordgo.Interaction)(nil)
