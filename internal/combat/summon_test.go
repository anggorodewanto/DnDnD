package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func TestSummonCreature_CreatesWithSummonerID(t *testing.T) {
	summonerID := uuid.New()
	encounterID := uuid.New()
	creatureRefID := "owl"

	var capturedParams refdata.CreateCombatantParams

	ms := &mockStore{
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:        "owl",
				Name:      "Owl",
				Ac:        11,
				HpAverage: 1,
				Speed:     json.RawMessage(`{"walk": 5, "fly": 60}`),
				AbilityScores: json.RawMessage(`{"str":3,"dex":13,"con":8,"int":2,"wis":12,"cha":7}`),
				Attacks:   json.RawMessage(`[{"name":"Talons","to_hit":3,"damage":"1","damage_type":"slashing"}]`),
			}, nil
		},
		createCombatantFn: func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
			capturedParams = arg
			return refdata.Combatant{
				ID:          uuid.New(),
				EncounterID: arg.EncounterID,
				ShortID:     arg.ShortID,
				DisplayName: arg.DisplayName,
				HpMax:       arg.HpMax,
				HpCurrent:   arg.HpCurrent,
				Ac:          arg.Ac,
				IsNpc:       arg.IsNpc,
				IsAlive:     arg.IsAlive,
				IsVisible:   arg.IsVisible,
				SummonerID:  arg.SummonerID,
				CreatureRefID: arg.CreatureRefID,
				Conditions:  json.RawMessage(`[]`),
			}, nil
		},
	}

	svc := NewService(ms)
	result, err := svc.SummonCreature(context.Background(), SummonCreatureInput{
		EncounterID:   encounterID,
		SummonerID:    summonerID,
		CreatureRefID: creatureRefID,
		ShortID:       "FAM",
		DisplayName:   "Aria's Owl",
		PositionCol:   "C",
		PositionRow:   5,
	})

	require.NoError(t, err)
	assert.Equal(t, "FAM", result.ShortID)
	assert.Equal(t, "Aria's Owl", result.DisplayName)
	assert.True(t, capturedParams.SummonerID.Valid)
	assert.Equal(t, summonerID, capturedParams.SummonerID.UUID)
	assert.True(t, capturedParams.IsNpc)
	assert.True(t, capturedParams.IsAlive)
	assert.True(t, capturedParams.IsVisible)
	assert.Equal(t, sql.NullString{String: "owl", Valid: true}, capturedParams.CreatureRefID)
}

func TestValidateCommandOwnership_MatchingSummoner(t *testing.T) {
	summonerID := uuid.New()
	creature := refdata.Combatant{
		ID:         uuid.New(),
		SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true},
	}
	err := ValidateCommandOwnership(creature, summonerID)
	assert.NoError(t, err)
}

func TestValidateCommandOwnership_WrongSummoner(t *testing.T) {
	creature := refdata.Combatant{
		ID:         uuid.New(),
		SummonerID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
	}
	err := ValidateCommandOwnership(creature, uuid.New())
	assert.ErrorIs(t, err, ErrNotSummoner)
}

func TestValidateCommandOwnership_NotSummoned(t *testing.T) {
	creature := refdata.Combatant{
		ID:         uuid.New(),
		SummonerID: uuid.NullUUID{},
	}
	err := ValidateCommandOwnership(creature, uuid.New())
	assert.ErrorIs(t, err, ErrNotSummoned)
}

func TestDismissSummon_RemovesCombatant(t *testing.T) {
	creatureID := uuid.New()
	summonerID := uuid.New()
	var deletedID uuid.UUID

	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          creatureID,
				ShortID:     "FAM",
				DisplayName: "Aria's Owl",
				SummonerID:  uuid.NullUUID{UUID: summonerID, Valid: true},
				IsAlive:     true,
				Conditions:  json.RawMessage(`[]`),
			}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, IsAlive: arg.IsAlive}, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
	}

	svc := NewService(ms)
	result, err := svc.DismissSummon(context.Background(), creatureID, summonerID)
	require.NoError(t, err)
	assert.Equal(t, creatureID, deletedID)
	assert.Equal(t, "FAM", result.ShortID)
	assert.Equal(t, "Aria's Owl", result.DisplayName)
}

func TestDismissSummon_WrongOwner(t *testing.T) {
	creatureID := uuid.New()
	wrongSummoner := uuid.New()

	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:         creatureID,
				SummonerID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			}, nil
		},
	}

	svc := NewService(ms)
	_, err := svc.DismissSummon(context.Background(), creatureID, wrongSummoner)
	assert.ErrorIs(t, err, ErrNotSummoner)
}

func TestHandleSummonDeath_RemovesAtZeroHP(t *testing.T) {
	creatureID := uuid.New()
	var deletedID uuid.UUID

	creature := refdata.Combatant{
		ID:          creatureID,
		ShortID:     "WF1",
		DisplayName: "Aria's Wolf #1",
		SummonerID:  uuid.NullUUID{UUID: uuid.New(), Valid: true},
		HpCurrent:   0,
		IsAlive:     false,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := &mockStore{
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
	}

	svc := NewService(ms)
	removed, err := svc.HandleSummonDeath(context.Background(), creature)
	require.NoError(t, err)
	assert.True(t, removed)
	assert.Equal(t, creatureID, deletedID)
}

func TestHandleSummonDeath_IgnoresNonSummoned(t *testing.T) {
	creature := refdata.Combatant{
		ID:         uuid.New(),
		SummonerID: uuid.NullUUID{},
		HpCurrent:  0,
		IsAlive:    false,
	}

	svc := NewService(&mockStore{})
	removed, err := svc.HandleSummonDeath(context.Background(), creature)
	require.NoError(t, err)
	assert.False(t, removed)
}

func TestDismissSummonsByConcentration_RemovesLinkedCreatures(t *testing.T) {
	summonerID := uuid.New()
	encounterID := uuid.New()
	wolf1ID := uuid.New()
	wolf2ID := uuid.New()

	var deletedIDs []uuid.UUID

	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: wolf1ID, SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}, ShortID: "WF1", DisplayName: "Wolf #1", IsAlive: true},
				{ID: wolf2ID, SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}, ShortID: "WF2", DisplayName: "Wolf #2", IsAlive: true},
				{ID: uuid.New(), SummonerID: uuid.NullUUID{}, ShortID: "G1", DisplayName: "Goblin", IsAlive: true},
			}, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			deletedIDs = append(deletedIDs, id)
			return nil
		},
	}

	svc := NewService(ms)
	removed, err := svc.DismissSummonsByConcentration(context.Background(), encounterID, summonerID)
	require.NoError(t, err)
	assert.Equal(t, 2, removed)
	assert.Contains(t, deletedIDs, wolf1ID)
	assert.Contains(t, deletedIDs, wolf2ID)
}

func TestDismissSummonsByConcentration_IgnoresOtherSummoners(t *testing.T) {
	otherSummoner := uuid.New()
	encounterID := uuid.New()

	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), SummonerID: uuid.NullUUID{UUID: otherSummoner, Valid: true}, ShortID: "FAM", IsAlive: true},
			}, nil
		},
	}

	svc := NewService(ms)
	removed, err := svc.DismissSummonsByConcentration(context.Background(), encounterID, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

func TestFormatSummonTurnNotification(t *testing.T) {
	msg := FormatSummonTurnNotification("PlayerName", "Wolf #1", "WF1")
	assert.Contains(t, msg, "@PlayerName")
	assert.Contains(t, msg, "Wolf #1")
	assert.Contains(t, msg, "WF1")
}

func TestFormatSummonDismissLog(t *testing.T) {
	msg := FormatSummonDismissLog("Aria", "Owl", "FAM")
	assert.Contains(t, msg, "Aria")
	assert.Contains(t, msg, "Owl")
	assert.Contains(t, msg, "FAM")
}

func TestFormatSummonDeathLog(t *testing.T) {
	msg := FormatSummonDeathLog("Wolf #1", "WF1")
	assert.Contains(t, msg, "Wolf #1")
	assert.Contains(t, msg, "WF1")
}

func TestFormatSummonLog(t *testing.T) {
	msg := FormatSummonLog("Aria", "Owl", "FAM", "C5")
	assert.Contains(t, msg, "Aria")
	assert.Contains(t, msg, "Owl")
	assert.Contains(t, msg, "FAM")
	assert.Contains(t, msg, "C5")
}

func TestParseCommandArgs_ValidActions(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantID   string
		wantAction string
		wantArgs []string
	}{
		{"attack", "FAM attack G1", "FAM", "attack", []string{"G1"}},
		{"move", "SW move C5", "SW", "move", []string{"C5"}},
		{"done", "FAM done", "FAM", "done", nil},
		{"dismiss", "FAM dismiss", "FAM", "dismiss", nil},
		{"help with target", "FAM help Thorn G1", "FAM", "help", []string{"Thorn", "G1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseCommandArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, cmd.CreatureShortID)
			assert.Equal(t, tt.wantAction, cmd.Action)
			assert.Equal(t, tt.wantArgs, cmd.Args)
		})
	}
}

func TestParseCommandArgs_TooFew(t *testing.T) {
	_, err := ParseCommandArgs("FAM")
	assert.Error(t, err)
}

func TestParseCommandArgs_Empty(t *testing.T) {
	_, err := ParseCommandArgs("")
	assert.Error(t, err)
}

func TestCommandCreature_Dismiss(t *testing.T) {
	summonerID := uuid.New()
	encounterID := uuid.New()
	creatureID := uuid.New()
	var deletedID uuid.UUID

	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: creatureID, ShortID: "FAM", DisplayName: "Aria's Owl", SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}, IsAlive: true, Conditions: json.RawMessage(`[]`)},
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: creatureID, ShortID: "FAM", DisplayName: "Aria's Owl", SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
	}

	svc := NewService(ms)
	result, err := svc.CommandCreature(context.Background(), CommandCreatureInput{
		EncounterID:     encounterID,
		SummonerID:      summonerID,
		CreatureShortID: "FAM",
		Action:          "dismiss",
	})
	require.NoError(t, err)
	assert.Equal(t, "dismiss", result.Action)
	assert.Equal(t, creatureID, deletedID)
	assert.Contains(t, result.CombatLog, "dismisses")
}

func TestCommandCreature_WrongOwner(t *testing.T) {
	summonerID := uuid.New()
	encounterID := uuid.New()

	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), ShortID: "FAM", SummonerID: uuid.NullUUID{UUID: uuid.New(), Valid: true}, IsAlive: true},
			}, nil
		},
	}

	svc := NewService(ms)
	_, err := svc.CommandCreature(context.Background(), CommandCreatureInput{
		EncounterID:     encounterID,
		SummonerID:      summonerID,
		CreatureShortID: "FAM",
		Action:          "dismiss",
	})
	assert.ErrorIs(t, err, ErrNotSummoner)
}

func TestCommandCreature_Done(t *testing.T) {
	summonerID := uuid.New()
	encounterID := uuid.New()
	creatureID := uuid.New()

	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: creatureID, ShortID: "WF1", DisplayName: "Wolf #1", SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}, IsAlive: true, Conditions: json.RawMessage(`[]`)},
			}, nil
		},
	}

	svc := NewService(ms)
	result, err := svc.CommandCreature(context.Background(), CommandCreatureInput{
		EncounterID:     encounterID,
		SummonerID:      summonerID,
		CreatureShortID: "WF1",
		Action:          "done",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", result.Action)
	assert.Contains(t, result.CombatLog, "ends their turn")
}

func TestCommandCreature_GenericAction(t *testing.T) {
	summonerID := uuid.New()
	encounterID := uuid.New()
	creatureID := uuid.New()

	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: creatureID, ShortID: "FAM", DisplayName: "Aria's Owl", SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}, IsAlive: true, Conditions: json.RawMessage(`[]`)},
			}, nil
		},
	}

	svc := NewService(ms)
	result, err := svc.CommandCreature(context.Background(), CommandCreatureInput{
		EncounterID:     encounterID,
		SummonerID:      summonerID,
		CreatureShortID: "FAM",
		Action:          "help",
		Args:            []string{"Thorn", "G1"},
	})
	require.NoError(t, err)
	assert.Equal(t, "help", result.Action)
}

func TestCommandCreature_NotFound(t *testing.T) {
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{}, nil
		},
	}

	svc := NewService(ms)
	_, err := svc.CommandCreature(context.Background(), CommandCreatureInput{
		EncounterID:     uuid.New(),
		SummonerID:      uuid.New(),
		CreatureShortID: "NOPE",
		Action:          "dismiss",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOPE")
}

func TestSummonMultipleCreatures(t *testing.T) {
	summonerID := uuid.New()
	encounterID := uuid.New()
	var capturedShortIDs []string

	ms := &mockStore{
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:        "wolf",
				Name:      "Wolf",
				Ac:        13,
				HpAverage: 11,
				Speed:     json.RawMessage(`{"walk": 40}`),
				AbilityScores: json.RawMessage(`{"str":12,"dex":15,"con":12,"int":3,"wis":12,"cha":6}`),
				Attacks:   json.RawMessage(`[{"name":"Bite","to_hit":4,"damage":"2d4+2","damage_type":"piercing"}]`),
			}, nil
		},
		createCombatantFn: func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
			capturedShortIDs = append(capturedShortIDs, arg.ShortID)
			return refdata.Combatant{
				ID:          uuid.New(),
				ShortID:     arg.ShortID,
				DisplayName: arg.DisplayName,
				SummonerID:  arg.SummonerID,
			}, nil
		},
	}

	svc := NewService(ms)
	results, err := svc.SummonMultipleCreatures(context.Background(), SummonMultipleInput{
		EncounterID:   encounterID,
		SummonerID:    summonerID,
		CreatureRefID: "wolf",
		BaseShortID:   "WF",
		BaseDisplayName: "Wolf",
		Quantity:      3,
		PositionCol:   "D",
		PositionRow:   5,
	})
	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, []string{"WF1", "WF2", "WF3"}, capturedShortIDs)
}

func TestCommandCreatureHandler_BadJSON(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/combat/%s/command", uuid.New()), strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCommandCreatureHandler_BadEncounterID(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/not-a-uuid/command", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCommandCreatureHandler_BadSummonerID(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	body := `{"summoner_id":"not-a-uuid","creature_short_id":"FAM","action":"dismiss"}`
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/combat/%s/command", uuid.New()), strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCommandCreatureHandler_Forbidden(t *testing.T) {
	wrongOwner := uuid.New()
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), ShortID: "FAM", SummonerID: uuid.NullUUID{UUID: uuid.New(), Valid: true}},
			}, nil
		},
	}

	svc := NewService(ms)
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	body := fmt.Sprintf(`{"summoner_id":"%s","creature_short_id":"FAM","action":"dismiss"}`, wrongOwner)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/combat/%s/command", uuid.New()), strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCommandCreatureHandler_ServiceError(t *testing.T) {
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	svc := NewService(ms)
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	body := fmt.Sprintf(`{"summoner_id":"%s","creature_short_id":"FAM","action":"dismiss"}`, uuid.New())
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/combat/%s/command", uuid.New()), strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSummonCreatureHandler_BadJSON(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/combat/%s/summon", uuid.New()), strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSummonCreatureHandler_BadSummonerID(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	body := `{"summoner_id":"bad","creature_ref_id":"owl","short_id":"FAM","display_name":"Owl","position_col":"A","position_row":1}`
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/combat/%s/summon", uuid.New()), strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSummonCreatureHandler_BadEncounterID(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/not-uuid/summon", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSummonCreatureHandler_CreatureNotFound(t *testing.T) {
	ms := &mockStore{
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{}, fmt.Errorf("creature not found")
		},
	}

	svc := NewService(ms)
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	body := fmt.Sprintf(`{"summoner_id":"%s","creature_ref_id":"unknown","short_id":"FAM","display_name":"Unknown","position_col":"A","position_row":1}`, uuid.New())
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/combat/%s/summon", uuid.New()), strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSummonCreature_CreatureNotFound(t *testing.T) {
	ms := &mockStore{
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{}, fmt.Errorf("not found")
		},
	}

	svc := NewService(ms)
	_, err := svc.SummonCreature(context.Background(), SummonCreatureInput{
		EncounterID: uuid.New(), SummonerID: uuid.New(), CreatureRefID: "x",
		ShortID: "X", DisplayName: "X", PositionCol: "A", PositionRow: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "looking up creature")
}

func TestSummonCreature_CreateFails(t *testing.T) {
	ms := &mockStore{
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{ID: "owl", HpAverage: 1, Ac: 11, Speed: json.RawMessage(`{"walk":5}`)}, nil
		},
		createCombatantFn: func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
			return refdata.Combatant{}, fmt.Errorf("db error")
		},
	}

	svc := NewService(ms)
	_, err := svc.SummonCreature(context.Background(), SummonCreatureInput{
		EncounterID: uuid.New(), SummonerID: uuid.New(), CreatureRefID: "owl",
		ShortID: "FAM", DisplayName: "Owl", PositionCol: "A", PositionRow: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating summoned combatant")
}

func TestDismissSummon_GetFails(t *testing.T) {
	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, fmt.Errorf("not found")
		},
	}
	svc := NewService(ms)
	_, err := svc.DismissSummon(context.Background(), uuid.New(), uuid.New())
	assert.Error(t, err)
}

func TestDismissSummon_DeleteFails(t *testing.T) {
	summonerID := uuid.New()
	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}}, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			return fmt.Errorf("db error")
		},
	}
	svc := NewService(ms)
	_, err := svc.DismissSummon(context.Background(), uuid.New(), summonerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "removing summoned creature")
}

func TestHandleSummonDeath_DeleteFails(t *testing.T) {
	ms := &mockStore{
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			return fmt.Errorf("db error")
		},
	}
	svc := NewService(ms)
	_, err := svc.HandleSummonDeath(context.Background(), refdata.Combatant{
		ID: uuid.New(), SummonerID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		HpCurrent: 0, IsAlive: false,
	})
	assert.Error(t, err)
}

func TestDismissSummonsByConcentration_ListFails(t *testing.T) {
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	svc := NewService(ms)
	_, err := svc.DismissSummonsByConcentration(context.Background(), uuid.New(), uuid.New())
	assert.Error(t, err)
}

func TestDismissSummonsByConcentration_DeleteFails(t *testing.T) {
	summonerID := uuid.New()
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}},
			}, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			return fmt.Errorf("db error")
		},
	}
	svc := NewService(ms)
	_, err := svc.DismissSummonsByConcentration(context.Background(), uuid.New(), summonerID)
	assert.Error(t, err)
}

func TestCommandCreature_ListFails(t *testing.T) {
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	svc := NewService(ms)
	_, err := svc.CommandCreature(context.Background(), CommandCreatureInput{
		EncounterID: uuid.New(), SummonerID: uuid.New(), CreatureShortID: "FAM", Action: "done",
	})
	assert.Error(t, err)
}

func TestCommandCreature_DismissDeleteFails(t *testing.T) {
	summonerID := uuid.New()
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), ShortID: "FAM", SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}},
			}, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			return fmt.Errorf("db error")
		},
	}
	svc := NewService(ms)
	_, err := svc.CommandCreature(context.Background(), CommandCreatureInput{
		EncounterID: uuid.New(), SummonerID: summonerID, CreatureShortID: "FAM", Action: "dismiss",
	})
	assert.Error(t, err)
}

func TestSummonMultipleCreatures_Failure(t *testing.T) {
	callCount := 0
	ms := &mockStore{
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			callCount++
			if callCount > 1 {
				return refdata.Creature{}, fmt.Errorf("db error")
			}
			return refdata.Creature{ID: "wolf", HpAverage: 11, Ac: 13, Speed: json.RawMessage(`{"walk":40}`)}, nil
		},
		createCombatantFn: func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: uuid.New(), ShortID: arg.ShortID}, nil
		},
	}
	svc := NewService(ms)
	_, err := svc.SummonMultipleCreatures(context.Background(), SummonMultipleInput{
		EncounterID: uuid.New(), SummonerID: uuid.New(), CreatureRefID: "wolf",
		BaseShortID: "WF", BaseDisplayName: "Wolf", Quantity: 3,
		PositionCol: "D", PositionRow: 5,
	})
	assert.Error(t, err)
}

func TestIsSummonedCreature(t *testing.T) {
	summoned := refdata.Combatant{SummonerID: uuid.NullUUID{UUID: uuid.New(), Valid: true}}
	assert.True(t, IsSummonedCreature(summoned))

	notSummoned := refdata.Combatant{SummonerID: uuid.NullUUID{}}
	assert.False(t, IsSummonedCreature(notSummoned))
}

func TestListSummonedCreatures(t *testing.T) {
	summonerID := uuid.New()
	combatants := []refdata.Combatant{
		{ID: uuid.New(), ShortID: "AR", SummonerID: uuid.NullUUID{}},
		{ID: uuid.New(), ShortID: "FAM", SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}},
		{ID: uuid.New(), ShortID: "WF1", SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}},
		{ID: uuid.New(), ShortID: "G1", SummonerID: uuid.NullUUID{UUID: uuid.New(), Valid: true}},
	}

	mine := ListSummonedCreatures(combatants, summonerID)
	assert.Len(t, mine, 2)
	assert.Equal(t, "FAM", mine[0].ShortID)
	assert.Equal(t, "WF1", mine[1].ShortID)
}

func TestCommandCreatureHandler_Dismiss(t *testing.T) {
	summonerID := uuid.New()
	encounterID := uuid.New()
	creatureID := uuid.New()

	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: creatureID, ShortID: "FAM", DisplayName: "Aria's Owl", SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}, IsAlive: true, Conditions: json.RawMessage(`[]`)},
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: creatureID, ShortID: "FAM", DisplayName: "Aria's Owl", SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}}, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}

	svc := NewService(ms)
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	body := fmt.Sprintf(`{"summoner_id":"%s","creature_short_id":"FAM","action":"dismiss"}`, summonerID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/combat/%s/command", encounterID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp commandCreatureResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "dismiss", resp.Action)
	assert.Contains(t, resp.CombatLog, "dismisses")
}

func TestSummonCreatureHandler(t *testing.T) {
	encounterID := uuid.New()
	summonerID := uuid.New()

	ms := &mockStore{
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID: "owl", Name: "Owl", Ac: 11, HpAverage: 1,
				Speed: json.RawMessage(`{"walk": 5, "fly": 60}`),
				AbilityScores: json.RawMessage(`{"str":3,"dex":13,"con":8,"int":2,"wis":12,"cha":7}`),
				Attacks: json.RawMessage(`[]`),
			}, nil
		},
		createCombatantFn: func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID: uuid.New(), ShortID: arg.ShortID, DisplayName: arg.DisplayName,
				SummonerID: arg.SummonerID, IsAlive: true, Conditions: json.RawMessage(`[]`),
			}, nil
		},
	}

	svc := NewService(ms)
	handler := NewHandler(svc, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	body := fmt.Sprintf(`{"summoner_id":"%s","creature_ref_id":"owl","short_id":"FAM","display_name":"Aria's Owl","position_col":"C","position_row":5}`, summonerID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/combat/%s/summon", encounterID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestFindCombatantByShortID(t *testing.T) {
	combatants := []refdata.Combatant{
		{ID: uuid.New(), ShortID: "AR"},
		{ID: uuid.New(), ShortID: "FAM"},
		{ID: uuid.New(), ShortID: "G1"},
	}

	c, err := FindCombatantByShortID(combatants, "FAM")
	require.NoError(t, err)
	assert.Equal(t, "FAM", c.ShortID)

	_, err = FindCombatantByShortID(combatants, "NOTFOUND")
	assert.Error(t, err)
}

func TestHandleSummonDeath_IgnoresAlive(t *testing.T) {
	creature := refdata.Combatant{
		ID:         uuid.New(),
		SummonerID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		HpCurrent:  5,
		IsAlive:    true,
	}

	svc := NewService(&mockStore{})
	removed, err := svc.HandleSummonDeath(context.Background(), creature)
	require.NoError(t, err)
	assert.False(t, removed)
}
