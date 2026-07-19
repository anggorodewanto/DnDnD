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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 11: GET /api/combat/{encounterID}/enemy-turn/{combatantID}/plan ---

func TestGetEnemyTurnPlan_Success(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	pcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == npcID {
				return refdata.Combatant{
					ID:            npcID,
					EncounterID:   encounterID,
					DisplayName:   "Goblin",
					PositionCol:   "C",
					PositionRow:   3,
					IsNpc:         true,
					IsAlive:       true,
					HpCurrent:     10,
					CreatureRefID: sql.NullString{String: "goblin", Valid: true},
				}, nil
			}
			return refdata.Combatant{}, sql.ErrNoRows
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:            "goblin",
				Name:          "Goblin",
				Size:          "Small",
				Speed:         json.RawMessage(`{"walk":30}`),
				Attacks:       json.RawMessage(`[{"name":"Scimitar","to_hit":4,"damage":"1d6+2","damage_type":"slashing","reach_ft":5}]`),
				AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:          npcID,
					DisplayName: "Goblin",
					PositionCol: "C",
					PositionRow: 3,
					IsNpc:       true,
					IsAlive:     true,
					HpCurrent:   10,
				},
				{
					ID:          pcID,
					DisplayName: "Aragorn",
					PositionCol: "C",
					PositionRow: 5,
					IsNpc:       false,
					IsAlive:     true,
					HpCurrent:   45,
					Ac:          16,
					IsVisible:   true, // DB default: seen unless the Hide action set it false
				},
			}, nil
		},
		listActiveReactionDeclarationsByEncounterFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
			return nil, nil
		},
	}

	svc := NewService(store)
	roller := dice.NewRoller(func(max int) int { return 10 })
	handler := NewHandler(svc, roller)

	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/enemy-turn/"+npcID.String()+"/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp enemyTurnPlanResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, npcID.String(), resp.CombatantID)
	assert.Equal(t, "Goblin", resp.DisplayName)
	assert.GreaterOrEqual(t, len(resp.Steps), 1) // at least an attack step
}

func TestGetEnemyTurnPlan_NotNPC(t *testing.T) {
	encounterID := uuid.New()
	pcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          pcID,
				DisplayName: "Aragorn",
				IsNpc:       false,
			}, nil
		},
	}

	svc := NewService(store)
	roller := dice.NewRoller(nil)
	handler := NewHandler(svc, roller)

	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/enemy-turn/"+pcID.String()+"/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle 12: POST /api/combat/{encounterID}/enemy-turn ---

func TestExecuteEnemyTurn_Success(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	hpUpdated := false
	actionLogCreated := false
	turnActionsUpdated := false

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == npcID {
				return refdata.Combatant{
					ID:          npcID,
					DisplayName: "Goblin",
					IsNpc:       true,
					IsAlive:     true,
				}, nil
			}
			return refdata.Combatant{
				ID:          targetID,
				DisplayName: "Aragorn",
				IsNpc:       false,
				IsAlive:     true,
				HpCurrent:   45,
				Ac:          16,
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:          turnID,
				EncounterID: eid,
				CombatantID: npcID,
			}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			hpUpdated = true
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			actionLogCreated = true
			assert.Equal(t, "enemy_turn", arg.ActionType)
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			turnActionsUpdated = true
			return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
		},
	}

	svc := NewService(store)
	// Deterministic roller: d20=15, damage=4+3 (7 total)
	roller := newDeterministicRoller(15, 4, 3)
	handler := NewHandler(svc, roller)

	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	body := `{
		"combatant_id": "` + npcID.String() + `",
		"steps": [
			{
				"type": "attack",
				"attack": {
					"weapon_name": "Scimitar",
					"to_hit": 4,
					"damage_dice": "1d6+2",
					"damage_type": "slashing",
					"reach_ft": 5,
					"target_id": "` + targetID.String() + `",
					"target_name": "Aragorn"
				}
			}
		]
	}`

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/enemy-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp executeEnemyTurnResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.CombatLog)

	assert.True(t, hpUpdated, "HP should be updated")
	assert.True(t, actionLogCreated, "Action log should be created")
	assert.True(t, turnActionsUpdated, "Turn actions should be updated")
}

// --- Bug fix: enemy-turn action_log must populate before_state/after_state ---
//
// Regression (live play): ExecuteEnemyTurn created the action_log row without
// before_state or after_state. Because action_log.{before_state,after_state}
// are NOT NULL, Postgres rejected the INSERT ("null value in column
// \"before_state\" of relation \"action_log\" violates not-null constraint"),
// so the turn never advanced and combat got stuck on the enemy's turn even
// though damage had already been applied. The mock store below mimics the
// NOT NULL constraint so this unit test reproduces the live failure.
func TestExecuteEnemyTurn_PopulatesBeforeAndAfterState(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	var loggedBefore, loggedAfter json.RawMessage
	turnActionsUpdated := false

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == npcID {
				return refdata.Combatant{
					ID:          npcID,
					EncounterID: encounterID,
					DisplayName: "Goblin",
					PositionCol: "C",
					PositionRow: 3,
					IsNpc:       true,
					IsAlive:     true,
					HpCurrent:   10,
				}, nil
			}
			return refdata.Combatant{
				ID:          targetID,
				EncounterID: encounterID,
				DisplayName: "Aragorn",
				IsNpc:       false,
				IsAlive:     true,
				HpCurrent:   45,
				Ac:          16,
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
		// Mimic the Postgres NOT NULL constraint on action_log.before_state /
		// after_state so this unit test fails the same way live play did.
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			if len(arg.BeforeState) == 0 {
				return refdata.ActionLog{}, fmt.Errorf(`null value in column "before_state" of relation "action_log" violates not-null constraint`)
			}
			if len(arg.AfterState) == 0 {
				return refdata.ActionLog{}, fmt.Errorf(`null value in column "after_state" of relation "action_log" violates not-null constraint`)
			}
			loggedBefore = arg.BeforeState
			loggedAfter = arg.AfterState
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			turnActionsUpdated = true
			return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
		},
	}

	svc := NewService(store)
	// Deterministic roller: d20=15 (hit), damage=4+3 (7 total)
	roller := newDeterministicRoller(15, 4, 3)

	plan := TurnPlan{
		CombatantID: npcID,
		Steps: []TurnStep{
			{
				Type: StepTypeAttack,
				Attack: &AttackStep{
					WeaponName: "Scimitar",
					ToHit:      4,
					DamageDice: "1d6+2",
					DamageType: "slashing",
					ReachFt:    5,
					TargetID:   targetID,
					TargetName: "Aragorn",
				},
			},
		},
	}

	_, err := svc.ExecuteEnemyTurn(context.Background(), encounterID, plan, roller)
	require.NoError(t, err, "enemy turn must not fail on the action_log NOT NULL constraint")

	require.NotEmpty(t, loggedBefore, "action_log before_state must be populated")
	require.NotEmpty(t, loggedAfter, "action_log after_state must be populated")
	// before_state must be valid JSON the undo path can parse.
	assert.True(t, json.Valid(loggedBefore), "before_state must be valid JSON")
	assert.True(t, turnActionsUpdated, "turn must advance after the enemy action is logged")
}

// ISSUE-021: the enemy-turn combat log must name the acting NPC. The HTTP
// handler reconstructs the TurnPlan from the POST body (combatant_id + steps
// only, no display_name), so the service must backfill plan.DisplayName from
// the fetched combatant BEFORE formatting the log — otherwise the header
// renders blank as "**'s Turn**".
func TestExecuteEnemyTurn_CombatLogNamesActor(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == npcID {
				return refdata.Combatant{
					ID:          npcID,
					EncounterID: encounterID,
					DisplayName: "Ghoul",
					PositionCol: "C",
					PositionRow: 3,
					IsNpc:       true,
					IsAlive:     true,
					HpCurrent:   22,
				}, nil
			}
			return refdata.Combatant{
				ID:          targetID,
				EncounterID: encounterID,
				DisplayName: "Forge",
				IsNpc:       false,
				IsAlive:     true,
				HpCurrent:   32,
				Ac:          14,
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
		},
	}

	svc := NewService(store)
	roller := newDeterministicRoller(15, 4, 3)

	// Plan as the HTTP handler builds it: no DisplayName, only the steps.
	plan := TurnPlan{
		CombatantID: npcID,
		Steps: []TurnStep{
			{
				Type: StepTypeAttack,
				Attack: &AttackStep{
					WeaponName: "Bite",
					ToHit:      4,
					DamageDice: "2d6+2",
					DamageType: "piercing",
					ReachFt:    5,
					TargetID:   targetID,
					TargetName: "Forge",
				},
			},
		},
	}

	result, err := svc.ExecuteEnemyTurn(context.Background(), encounterID, plan, roller)
	require.NoError(t, err)
	assert.Contains(t, result.CombatLog, "**Ghoul's Turn**",
		"enemy-turn combat log header must name the acting NPC, not render blank")
}

// The enemy-turn combat log must report the damage actually dealt after the
// target's resistance — not the raw rolled total — so the DM/players see the
// halved number (e.g. a raging barbarian taking a bite). ExecuteEnemyTurn must
// thread ApplyDamage's FinalDamage back onto the attack step before formatting.
func TestExecuteEnemyTurn_LogShowsResistedDamage(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == npcID {
				return refdata.Combatant{
					ID: npcID, EncounterID: encounterID, DisplayName: "Ghoul",
					IsNpc: true, IsAlive: true, HpCurrent: 22,
				}, nil
			}
			// Resistant target: an NPC whose creature row resists piercing.
			return refdata.Combatant{
				ID: targetID, EncounterID: encounterID, DisplayName: "Bonecage",
				IsNpc: true, IsAlive: true, HpCurrent: 30, Ac: 14,
				CreatureRefID: sql.NullString{String: "bonecage", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{ID: "bonecage", DamageResistances: []string{"piercing"}}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
		},
	}

	svc := NewService(store)
	roller := newDeterministicRoller(15, 4, 3)

	// Pre-rolled hit for 8 raw piercing; resistance must halve it to 4.
	plan := TurnPlan{
		CombatantID: npcID,
		Steps: []TurnStep{
			{
				Type: StepTypeAttack,
				Attack: &AttackStep{
					WeaponName: "Bite", ToHit: 4, DamageDice: "2d6+2",
					DamageType: "piercing", ReachFt: 5,
					TargetID: targetID, TargetName: "Bonecage",
					RollResult: &AttackRollResult{ToHitTotal: 16, Hit: true, DamageTotal: 8},
				},
			},
		},
	}

	result, err := svc.ExecuteEnemyTurn(context.Background(), encounterID, plan, roller)
	require.NoError(t, err)
	assert.Contains(t, result.CombatLog, "4 piercing damage (resisted — halved from 8)",
		"combat log must show the halved (post-resistance) damage")
}

// COV-16: a Rogue 5+ hit by a visible attacker may declare Uncanny Dodge; the
// pre-rolled damage is halved BEFORE it is written to HP (no retroactive
// heal-back), the reaction is consumed, and the halved amount is what lands.
func TestExecuteEnemyTurn_UncannyDodgeHalvesDamageBeforeWrite(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	charID := uuid.New()
	turnID := uuid.New()

	declared, resolved := false, false
	var hpWritten int32 = -1

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == npcID {
			return refdata.Combatant{ID: npcID, EncounterID: encounterID, DisplayName: "Goblin", IsNpc: true, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
		}
		return refdata.Combatant{
			ID: targetID, EncounterID: encounterID, DisplayName: "Vex", IsNpc: false, IsAlive: true,
			HpCurrent: 30, Ac: 15, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(_ context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID, RoundNumber: 1}, nil
	}
	store.createReactionDeclarationFn = func(_ context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
		declared = true
		return refdata.ReactionDeclaration{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, Status: "active"}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(_ context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		resolved = true
		return refdata.ReactionDeclaration{ID: arg.ID, Status: "used"}, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpWritten = arg.HpCurrent
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}

	svc := NewService(store)

	// Pre-rolled hit for 8 raw slashing; Uncanny Dodge halves it to 4 before the HP write.
	plan := TurnPlan{
		CombatantID: npcID,
		Steps: []TurnStep{
			{
				Type: StepTypeAttack,
				Attack: &AttackStep{
					WeaponName: "Scimitar", ToHit: 4, DamageDice: "1d6+2",
					DamageType: "slashing", ReachFt: 5,
					TargetID: targetID, TargetName: "Vex",
					RollResult:     &AttackRollResult{ToHitRoll: 12, ToHitTotal: 16, Hit: true, DamageTotal: 8},
					ChosenReaction: &ReactionOption{ID: "uncanny-dodge", Label: "Uncanny Dodge (halve damage)", HalveDamage: true, Reason: "Uncanny Dodge"},
				},
			},
		},
	}

	result, err := svc.ExecuteEnemyTurn(context.Background(), encounterID, plan, newDeterministicRoller(12, 4))
	require.NoError(t, err)
	assert.Equal(t, int32(4), result.DamageApplied[targetID], "8 raw damage halved by Uncanny Dodge = 4 applied")
	assert.Equal(t, int32(26), hpWritten, "HP 30 − 4 (already halved) = 26 written; no full-damage-then-heal-back")
	assert.True(t, declared, "the Uncanny Dodge reaction must be declared")
	assert.True(t, resolved, "the reaction must be marked used so it can't fire twice this round")
	assert.Contains(t, result.CombatLog, "Uncanny Dodge", "combat log announces the reaction")
}

// COV-16: Uncanny Dodge triggers only "when an attacker hits you" — a declared
// halving reaction against an attack that MISSES must not be consumed (the
// reaction stays available) nor announced (no misleading log line). This is the
// post-hit reaction's key difference from a +AC reaction, which is spent
// regardless because it was applied to decide the hit.
func TestExecuteEnemyTurn_UncannyDodgeNotConsumedOnMiss(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	charID := uuid.New()

	declared, resolved := false, false

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == npcID {
			return refdata.Combatant{ID: npcID, EncounterID: encounterID, DisplayName: "Goblin", IsNpc: true, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
		}
		return refdata.Combatant{
			ID: targetID, EncounterID: encounterID, DisplayName: "Vex", IsNpc: false, IsAlive: true,
			HpCurrent: 30, Ac: 15, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(_ context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: eid, CombatantID: npcID, RoundNumber: 1}, nil
	}
	store.createReactionDeclarationFn = func(_ context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
		declared = true
		return refdata.ReactionDeclaration{ID: uuid.New(), Status: "active"}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(_ context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		resolved = true
		return refdata.ReactionDeclaration{ID: arg.ID, Status: "used"}, nil
	}

	svc := NewService(store)

	// Pre-rolled MISS (10 vs AC 15); the declared Uncanny Dodge must not fire.
	plan := TurnPlan{
		CombatantID: npcID,
		Steps: []TurnStep{
			{
				Type: StepTypeAttack,
				Attack: &AttackStep{
					WeaponName: "Scimitar", ToHit: 4, DamageDice: "1d6+2",
					DamageType: "slashing", ReachFt: 5,
					TargetID: targetID, TargetName: "Vex",
					RollResult:     &AttackRollResult{ToHitRoll: 6, ToHitTotal: 10, Hit: false, DamageTotal: 8},
					ChosenReaction: &ReactionOption{ID: "uncanny-dodge", Label: "Uncanny Dodge (halve damage)", HalveDamage: true, Reason: "Uncanny Dodge"},
				},
			},
		},
	}

	result, err := svc.ExecuteEnemyTurn(context.Background(), encounterID, plan, newDeterministicRoller(6, 4))
	require.NoError(t, err)
	assert.Zero(t, result.DamageApplied[targetID], "a missed attack deals no damage")
	assert.False(t, declared, "an untriggered post-hit reaction must not be declared")
	assert.False(t, resolved, "an untriggered post-hit reaction must not be consumed — it stays available")
	assert.NotContains(t, result.CombatLog, "Uncanny Dodge", "a missed attack must not announce a halving reaction")
}

// --- TDD Cycle 13: indexToColLabel ---

func TestIndexToColLabel(t *testing.T) {
	assert.Equal(t, "A", indexToColLabel(0))
	assert.Equal(t, "B", indexToColLabel(1))
	assert.Equal(t, "Z", indexToColLabel(25))
	assert.Equal(t, "AA", indexToColLabel(26))
}

// --- TDD Cycle 14: EnemyTurnNotifier called after execute ---

type mockEnemyTurnNotifier struct {
	called      bool
	encounterID uuid.UUID
	combatLog   string
	done        chan struct{}
}

func newMockEnemyTurnNotifier() *mockEnemyTurnNotifier {
	return &mockEnemyTurnNotifier{done: make(chan struct{})}
}

func (m *mockEnemyTurnNotifier) NotifyEnemyTurnExecuted(ctx context.Context, encounterID uuid.UUID, combatLog string) {
	m.called = true
	m.encounterID = encounterID
	m.combatLog = combatLog
	close(m.done)
}

func TestExecuteEnemyTurn_NotifiesOnSuccess(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == npcID {
				return refdata.Combatant{
					ID:          npcID,
					DisplayName: "Goblin",
					IsNpc:       true,
					IsAlive:     true,
				}, nil
			}
			return refdata.Combatant{
				ID:          targetID,
				DisplayName: "Aragorn",
				IsNpc:       false,
				IsAlive:     true,
				HpCurrent:   45,
				Ac:          16,
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
		},
	}

	svc := NewService(store)
	roller := newDeterministicRoller(15, 4, 3)
	handler := NewHandler(svc, roller)

	notifier := newMockEnemyTurnNotifier()
	handler.SetEnemyTurnNotifier(notifier)

	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	body := `{
		"combatant_id": "` + npcID.String() + `",
		"steps": [
			{
				"type": "attack",
				"attack": {
					"weapon_name": "Scimitar",
					"to_hit": 4,
					"damage_dice": "1d6+2",
					"damage_type": "slashing",
					"reach_ft": 5,
					"target_id": "` + targetID.String() + `",
					"target_name": "Aragorn"
				}
			}
		]
	}`

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/enemy-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Wait for async goroutine to complete
	select {
	case <-notifier.done:
	case <-time.After(2 * time.Second):
		t.Fatal("notifier was not called within timeout")
	}

	assert.True(t, notifier.called, "notifier should be called after successful execution")
	assert.Equal(t, encounterID, notifier.encounterID)
	assert.NotEmpty(t, notifier.combatLog)
}

func TestExecuteEnemyTurn_NoNotifierNoPanic(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          npcID,
				DisplayName: "Goblin",
				IsNpc:       true,
				IsAlive:     true,
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID}, nil
		},
	}

	svc := NewService(store)
	roller := newDeterministicRoller()
	handler := NewHandler(svc, roller)
	// No notifier set — should not panic

	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	body := `{"combatant_id": "` + npcID.String() + `", "steps": []}`

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/enemy-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- F-11: ExecuteEnemyTurn publishes WebSocket snapshot ---

func TestExecuteEnemyTurn_F11_PublishesSnapshot(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          npcID,
				EncounterID: encounterID,
				DisplayName: "Goblin",
				IsNpc:       true,
				IsAlive:     true,
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID}, nil
		},
		updateCombatantPositionFn: func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
		},
	}

	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	plan := TurnPlan{
		CombatantID: npcID,
		Steps: []TurnStep{
			{
				Type: StepTypeMovement,
				Movement: &MovementStep{
					Path: []pathfinding.Point{{Col: 3, Row: 4}},
				},
			},
		},
	}

	roller := dice.NewRoller(nil)
	_, err := svc.ExecuteEnemyTurn(context.Background(), encounterID, plan, roller)
	require.NoError(t, err)

	require.Equal(t, []uuid.UUID{encounterID}, pub.calls(),
		"ExecuteEnemyTurn must publish a WebSocket snapshot after mutations")
}

// --- F-12: GenerateEnemyTurnPlan uses encounter's actual map ---

func TestGenerateEnemyTurnPlan_F12_UsesEncounterMap(t *testing.T) {
	encounterID := uuid.New()
	mapID := uuid.New()
	npcID := uuid.New()
	pcID := uuid.New()

	// Build a 10x8 map with a wall blocking the direct path.
	// The tiled JSON has a terrain layer (10x8 open) and a walls layer with one wall.
	tiledJSON := json.RawMessage(`{
		"width": 10,
		"height": 8,
		"tilewidth": 48,
		"tileheight": 48,
		"tilesets": [],
		"layers": [
			{
				"name": "terrain",
				"type": "tilelayer",
				"width": 10,
				"height": 8,
				"data": [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
			},
			{
				"name": "walls",
				"type": "objectgroup",
				"objects": [
					{"x": 144, "y": 0, "width": 0, "height": 384}
				]
			}
		]
	}`)

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				EncounterID:   encounterID,
				DisplayName:   "Goblin",
				PositionCol:   "A",
				PositionRow:   1,
				IsNpc:         true,
				IsAlive:       true,
				HpCurrent:     10,
				CreatureRefID: sql.NullString{String: "goblin", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:            "goblin",
				Name:          "Goblin",
				Size:          "Small",
				Speed:         json.RawMessage(`{"walk":30}`),
				Attacks:       json.RawMessage(`[{"name":"Scimitar","to_hit":4,"damage":"1d6+2","damage_type":"slashing","reach_ft":5}]`),
				AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Goblin", PositionCol: "A", PositionRow: 1, IsNpc: true, IsAlive: true, HpCurrent: 10},
				{ID: pcID, DisplayName: "Aragorn", PositionCol: "E", PositionRow: 1, IsNpc: false, IsAlive: true, HpCurrent: 45, Ac: 16, IsVisible: true}, // not hidden — the map's wall, not a Hide action, must suppress the attack
			}, nil
		},
		listActiveReactionDeclarationsByEncounterFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
			return nil, nil
		},
		getEncounterFn: func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:     encounterID,
				MapID:  uuid.NullUUID{UUID: mapID, Valid: true},
				Status: "active",
			}, nil
		},
		getMapByIDUncheckedFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			assert.Equal(t, mapID, id, "should load the encounter's map")
			return refdata.Map{
				ID:            mapID,
				WidthSquares:  10,
				HeightSquares: 8,
				TiledJson:     tiledJSON,
			}, nil
		},
	}

	svc := NewService(store)
	plan, err := svc.GenerateEnemyTurnPlan(context.Background(), encounterID, npcID, dice.NewRoller(nil))
	require.NoError(t, err)
	require.NotNil(t, plan)

	// The encounter map has a full-height wall at the col2|col3 boundary that
	// fully divides the Goblin (A1) from the PC (E1). With the see-filter the
	// walled-off PC is unseeable, so the planner correctly refuses to plan an
	// attack against it and the turn is a hold (no attack step).
	//
	// This is exactly what proves the encounter map was loaded and honored: on
	// the 20x20 open FALLBACK grid (no wall) the Goblin WOULD see and attack the
	// PC 4 tiles away — so an attack-less plan can only mean the map's wall was
	// used for line-of-sight, not the default grid.
	attackSteps := filterStepsByType(plan.Steps, StepTypeAttack)
	assert.Empty(t, attackSteps, "walled-off, unseeable PC must not be attacked (proves the map's wall was loaded)")

	// Any movement that IS planned must stay within the 10x8 map bounds (guards
	// against the default 20x20 grid leaking in).
	for _, step := range plan.Steps {
		if step.Type == StepTypeMovement && step.Movement != nil {
			for _, pt := range step.Movement.Path {
				assert.Less(t, pt.Col, 10, "path col must be within map width 10")
				assert.Less(t, pt.Row, 8, "path row must be within map height 8")
			}
		}
	}
}

func TestGenerateEnemyTurnPlan_F12_FallsBackToDefaultGrid(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	pcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				EncounterID:   encounterID,
				DisplayName:   "Goblin",
				PositionCol:   "C",
				PositionRow:   3,
				IsNpc:         true,
				IsAlive:       true,
				HpCurrent:     10,
				CreatureRefID: sql.NullString{String: "goblin", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:            "goblin",
				Name:          "Goblin",
				Size:          "Small",
				Speed:         json.RawMessage(`{"walk":30}`),
				Attacks:       json.RawMessage(`[{"name":"Scimitar","to_hit":4,"damage":"1d6+2","damage_type":"slashing","reach_ft":5}]`),
				AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Goblin", PositionCol: "C", PositionRow: 3, IsNpc: true, IsAlive: true, HpCurrent: 10},
				{ID: pcID, DisplayName: "Aragorn", PositionCol: "C", PositionRow: 5, IsNpc: false, IsAlive: true, HpCurrent: 45, Ac: 16, IsVisible: true},
			}, nil
		},
		listActiveReactionDeclarationsByEncounterFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
			return nil, nil
		},
		// GetEncounter returns an encounter with no map
		getEncounterFn: func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active"}, nil
		},
	}

	svc := NewService(store)
	plan, err := svc.GenerateEnemyTurnPlan(context.Background(), encounterID, npcID, dice.NewRoller(nil))
	require.NoError(t, err)
	require.NotNil(t, plan)
	// Should still produce a valid plan using the 20x20 fallback
	assert.GreaterOrEqual(t, len(plan.Steps), 1)
}

// --- F-22: GenerateEnemyTurnPlan pre-rolls attacks for DM fudging ---

func TestGenerateEnemyTurnPlan_F22_AttackStepsHaveRollResult(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	pcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == npcID {
				return refdata.Combatant{
					ID:            npcID,
					EncounterID:   encounterID,
					DisplayName:   "Goblin",
					PositionCol:   "C",
					PositionRow:   3,
					IsNpc:         true,
					IsAlive:       true,
					HpCurrent:     10,
					CreatureRefID: sql.NullString{String: "goblin", Valid: true},
				}, nil
			}
			return refdata.Combatant{
				ID:          pcID,
				DisplayName: "Aragorn",
				PositionCol: "C",
				PositionRow: 5,
				IsNpc:       false,
				IsAlive:     true,
				HpCurrent:   45,
				Ac:          16,
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:            "goblin",
				Name:          "Goblin",
				Size:          "Small",
				Speed:         json.RawMessage(`{"walk":30}`),
				Attacks:       json.RawMessage(`[{"name":"Scimitar","to_hit":4,"damage":"1d6+2","damage_type":"slashing","reach_ft":5}]`),
				AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:          npcID,
					DisplayName: "Goblin",
					PositionCol: "C",
					PositionRow: 3,
					IsNpc:       true,
					IsAlive:     true,
					HpCurrent:   10,
				},
				{
					ID:          pcID,
					DisplayName: "Aragorn",
					PositionCol: "C",
					PositionRow: 5,
					IsNpc:       false,
					IsAlive:     true,
					HpCurrent:   45,
					Ac:          16,
					IsVisible:   true, // DB default: seen unless the Hide action set it false
				},
			}, nil
		},
		listActiveReactionDeclarationsByEncounterFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
			return nil, nil
		},
	}

	// Deterministic roller: d20 rolls 15, damage rolls 4
	roller := dice.NewRoller(func(max int) int { return 15 })

	svc := NewService(store)
	plan, err := svc.GenerateEnemyTurnPlan(context.Background(), encounterID, npcID, roller)
	require.NoError(t, err)
	require.NotNil(t, plan)

	// Find attack steps and verify they have pre-rolled results
	var attackSteps int
	for _, step := range plan.Steps {
		if step.Type == StepTypeAttack && step.Attack != nil {
			attackSteps++
			require.NotNil(t, step.Attack.RollResult, "attack step should have RollResult pre-populated for DM fudging")
			assert.Greater(t, step.Attack.RollResult.ToHitTotal, 0)
		}
	}
	assert.Greater(t, attackSteps, 0, "plan should contain at least one attack step")
}

// --- 5b: enemy-turn reaction window via Turn Builder ---

// GenerateEnemyTurnPlan surfaces a targeted PC's available reactions on the
// attack step so the DM can pick one before executing.
func TestGenerateEnemyTurnPlan_SurfacesReactionsForPCTarget(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	pcID := uuid.New()
	charID := uuid.New()

	feats, _ := json.Marshal([]CharacterFeature{{Name: "Defensive Duelist"}})

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == npcID {
			return refdata.Combatant{
				ID: npcID, EncounterID: encounterID, DisplayName: "Goblin",
				PositionCol: "C", PositionRow: 3, IsNpc: true, IsAlive: true, HpCurrent: 10,
				CreatureRefID: sql.NullString{String: "goblin", Valid: true},
				Conditions:    json.RawMessage(`[]`),
			}, nil
		}
		return refdata.Combatant{
			ID: pcID, EncounterID: encounterID, DisplayName: "Windreth",
			PositionCol: "C", PositionRow: 5, IsNpc: false, IsAlive: true, HpCurrent: 45, Ac: 16,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCreatureFn = func(_ context.Context, _ string) (refdata.Creature, error) {
		return refdata.Creature{
			ID: "goblin", Name: "Goblin", Size: "Small",
			Speed:         json.RawMessage(`{"walk":30}`),
			Attacks:       json.RawMessage(`[{"name":"Scimitar","to_hit":4,"damage":"1d6+2","damage_type":"slashing","reach_ft":5}]`),
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: npcID, DisplayName: "Goblin", PositionCol: "C", PositionRow: 3, IsNpc: true, IsAlive: true, HpCurrent: 10},
			{ID: pcID, DisplayName: "Windreth", PositionCol: "C", PositionRow: 5, IsNpc: false, IsAlive: true, HpCurrent: 45, Ac: 16, IsVisible: true, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		}, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID: charID, ProficiencyBonus: 3,
			Features:         pqtype.NullRawMessage{RawMessage: feats, Valid: true},
			EquippedMainHand: sql.NullString{String: "rapier", Valid: true},
		}, nil
	}
	store.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return finesseWeapon(), nil }
	// No active turn → the PC's reaction is free.
	store.getActiveTurnByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, sql.ErrNoRows
	}

	svc := NewService(store)
	roller := dice.NewRoller(func(int) int { return 10 })
	plan, err := svc.GenerateEnemyTurnPlan(context.Background(), encounterID, npcID, roller)
	require.NoError(t, err)

	var found bool
	for _, step := range plan.Steps {
		if step.Type == StepTypeAttack && step.Attack != nil && step.Attack.TargetID == pcID {
			require.Len(t, step.Attack.AvailableReactions, 1)
			assert.Equal(t, "defensive-duelist", step.Attack.AvailableReactions[0].ID)
			assert.Equal(t, 3, step.Attack.AvailableReactions[0].ACBonus)
			found = true
		}
	}
	assert.True(t, found, "the attack on the Defensive-Duelist PC should surface its reaction option")
}

// ExecuteEnemyTurn: a chosen reaction raises the target's AC so a pre-rolled hit
// becomes a miss, the reaction is marked used (declared+resolved), and the
// combat log announces it before the attack line.
func TestExecuteEnemyTurn_ChosenReactionFlipsHitAndMarksUsed(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	charID := uuid.New()
	turnID := uuid.New()

	declared, resolved := false, false
	declID := uuid.New()

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == npcID {
			return refdata.Combatant{ID: npcID, EncounterID: encounterID, DisplayName: "Goblin", IsNpc: true, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
		}
		return refdata.Combatant{
			ID: targetID, EncounterID: encounterID, DisplayName: "Windreth", IsNpc: false, IsAlive: true,
			HpCurrent: 30, Ac: 15, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(_ context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID, RoundNumber: 1}, nil
	}
	store.createReactionDeclarationFn = func(_ context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
		declared = true
		return refdata.ReactionDeclaration{ID: declID, EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, Status: "active"}, nil
	}
	store.getReactionDeclarationFn = func(_ context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: id, EncounterID: encounterID, CombatantID: targetID, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(_ context.Context, _ refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{{ID: uuid.New(), CombatantID: targetID, RoundNumber: 1}}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(_ context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		resolved = true
		return refdata.ReactionDeclaration{ID: arg.ID, Status: "used"}, nil
	}

	svc := NewService(store)
	handler := NewHandler(svc, newDeterministicRoller(12, 4, 3))
	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	// Pre-rolled at base AC 15: d20=12 +4 = 16 ≥ 15 → hit. Chosen DD +3 → AC 18 → miss.
	body := `{
		"combatant_id": "` + npcID.String() + `",
		"steps": [{
			"type": "attack",
			"attack": {
				"weapon_name": "Scimitar", "to_hit": 4, "damage_dice": "1d6+2",
				"damage_type": "slashing", "reach_ft": 5,
				"target_id": "` + targetID.String() + `", "target_name": "Windreth",
				"roll_result": {"to_hit_roll": 12, "to_hit_total": 16, "hit": true},
				"chosen_reaction": {"id": "defensive-duelist", "label": "Defensive Duelist (+3 AC)", "ac_bonus": 3, "reason": "Defensive Duelist"}
			}
		}]
	}`

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/enemy-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp executeEnemyTurnResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.Contains(t, resp.CombatLog, "Defensive Duelist", "combat log should announce the reaction")
	assert.Contains(t, resp.CombatLog, "Miss", "the +3 reaction should turn the hit into a miss")
	assert.True(t, declared, "reaction should be declared")
	assert.True(t, resolved, "reaction should be resolved (marked used) so the PC can't react twice")
}

// A second NPC's plan may carry a stale chosen_reaction the PC already spent
// against an earlier attacker this round. ExecuteEnemyTurn must drop it: no AC
// boost, no combat-log call-out, and no fresh declaration written.
func TestExecuteEnemyTurn_DropsChosenReactionAlreadySpentThisRound(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	charID := uuid.New()

	declaredAgain := false

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == npcID {
			return refdata.Combatant{ID: npcID, EncounterID: encounterID, DisplayName: "Goblin B", IsNpc: true, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
		}
		return refdata.Combatant{
			ID: targetID, EncounterID: encounterID, DisplayName: "Windreth", IsNpc: false, IsAlive: true,
			HpCurrent: 30, Ac: 15, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(_ context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: eid, CombatantID: npcID, RoundNumber: 1}, nil
	}
	// The PC already spent their reaction this round (against goblin A).
	store.listReactionDeclarationsByCombatantFn = func(_ context.Context, _ refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{{Status: "used", UsedOnRound: sql.NullInt32{Int32: 1, Valid: true}}}, nil
	}
	store.createReactionDeclarationFn = func(_ context.Context, _ refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
		declaredAgain = true
		return refdata.ReactionDeclaration{ID: uuid.New(), Status: "active"}, nil
	}

	svc := NewService(store)
	handler := NewHandler(svc, newDeterministicRoller(12, 4, 3))
	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	// Pre-rolled at base AC 15: d20=12 +4 = 16 ≥ 15 → hit. The stale +3 reaction
	// must NOT apply, so the hit stands.
	body := `{
		"combatant_id": "` + npcID.String() + `",
		"steps": [{
			"type": "attack",
			"attack": {
				"weapon_name": "Scimitar", "to_hit": 4, "damage_dice": "1d6+2",
				"damage_type": "slashing", "reach_ft": 5,
				"target_id": "` + targetID.String() + `", "target_name": "Windreth",
				"roll_result": {"to_hit_roll": 12, "to_hit_total": 16, "hit": true},
				"chosen_reaction": {"id": "defensive-duelist", "label": "Defensive Duelist (+3 AC)", "ac_bonus": 3, "reason": "Defensive Duelist"}
			}
		}]
	}`

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/enemy-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp executeEnemyTurnResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.NotContains(t, resp.CombatLog, "Defensive Duelist", "a spent reaction must not be re-announced")
	assert.Contains(t, resp.CombatLog, "Hit", "the hit stands because the stale reaction was dropped")
	assert.False(t, declaredAgain, "no second reaction declaration should be written for an already-spent reaction")
}
