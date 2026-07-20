package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// ddFixture wires a mock store for the Defensive Duelist duration tests: one NPC
// attacker, one PC defender (AC 15, PB 3) whose conditions JSON round-trips
// through UpdateCombatantConditions so a marker written during one attack step is
// visible to the next.
type ddFixture struct {
	store       *mockStore
	encounterID uuid.UUID
	npcID       uuid.UUID
	targetID    uuid.UUID
	charID      uuid.UUID

	declarations int          // how many reaction declarations were written
	targetConds  *json.RawMessage
}

func newDDFixture(t *testing.T) *ddFixture {
	t.Helper()

	conds := json.RawMessage(`[]`)
	f := &ddFixture{
		store:       defaultMockStore(),
		encounterID: uuid.New(),
		npcID:       uuid.New(),
		targetID:    uuid.New(),
		charID:      uuid.New(),
		targetConds: &conds,
	}

	f.store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == f.npcID {
			return refdata.Combatant{
				ID: f.npcID, EncounterID: f.encounterID, DisplayName: "Goblin",
				IsNpc: true, IsAlive: true, Conditions: json.RawMessage(`[]`),
			}, nil
		}
		return refdata.Combatant{
			ID: f.targetID, EncounterID: f.encounterID, DisplayName: "Windreth",
			IsNpc: false, IsAlive: true, HpCurrent: 30, Ac: 15,
			CharacterID: uuid.NullUUID{UUID: f.charID, Valid: true},
			Conditions:  *f.targetConds,
		}, nil
	}
	f.store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		if arg.ID == f.targetID {
			*f.targetConds = arg.Conditions
		}
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	f.store.getActiveTurnByEncounterIDFn = func(_ context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: eid, CombatantID: f.npcID, RoundNumber: 1}, nil
	}
	f.store.createReactionDeclarationFn = func(_ context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
		f.declarations++
		return refdata.ReactionDeclaration{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, Status: "active"}, nil
	}
	f.store.getReactionDeclarationFn = func(_ context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: id, EncounterID: f.encounterID, CombatantID: f.targetID, Status: "active"}, nil
	}
	f.store.listTurnsByEncounterAndRoundFn = func(_ context.Context, _ refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{{ID: uuid.New(), CombatantID: f.targetID, RoundNumber: 1}}, nil
	}
	f.store.updateReactionDeclarationStatusUsedFn = func(_ context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: arg.ID, Status: "used"}, nil
	}
	return f
}

// execEnemyTurn POSTs a raw steps array through the enemy-turn route.
func (f *ddFixture) execEnemyTurn(t *testing.T, steps string) executeEnemyTurnResponse {
	t.Helper()

	svc := NewService(f.store)
	handler := NewHandler(svc, newDeterministicRoller(12, 4, 3))
	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	body := `{"combatant_id": "` + f.npcID.String() + `", "steps": ` + steps + `}`
	req := httptest.NewRequest("POST", "/api/combat/"+f.encounterID.String()+"/enemy-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp executeEnemyTurnResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	return resp
}

// ddAttackStep renders one attack step. Pre-rolled totals are quoted at the
// defender's BASE AC (15), mirroring how GenerateEnemyTurnPlan pre-rolls.
func ddAttackStep(targetID uuid.UUID, name string, roll, total, reachFt int, chosenReaction string) string {
	step := `{
		"type": "attack",
		"attack": {
			"weapon_name": "` + name + `", "to_hit": 4, "damage_dice": "1d6+2",
			"damage_type": "slashing", "reach_ft": ` + strconv.Itoa(reachFt) + `,
			"target_id": "` + targetID.String() + `", "target_name": "Windreth",
			"roll_result": {"to_hit_roll": ` + strconv.Itoa(roll) + `, "to_hit_total": ` + strconv.Itoa(total) + `, "hit": true}`
	if chosenReaction != "" {
		step += `, "chosen_reaction": ` + chosenReaction
	}
	return step + `}}`
}

const ddChosenReactionJSON = `{"id": "defensive-duelist", "label": "Defensive Duelist (+3 AC)", "ac_bonus": 3, "reason": "Defensive Duelist"}`

// --- The regression: 2024 PHB p.203 "You gain this bonus to your AC against
// melee attacks until the start of your next turn." ---

// One Reaction spent on the first swing of a multiattack must keep the +PB AC up
// for the SECOND swing too. Both swings are pre-rolled to beat base AC 15 but
// fall short of the boosted AC 18, so today the second swing wrongly lands.
func TestExecuteEnemyTurn_DefensiveDuelistACPersistsAcrossMultiattack(t *testing.T) {
	f := newDDFixture(t)

	steps := `[` +
		ddAttackStep(f.targetID, "Scimitar", 12, 16, 5, ddChosenReactionJSON) + `,` +
		ddAttackStep(f.targetID, "Shortsword", 13, 17, 5, "") +
		`]`

	resp := f.execEnemyTurn(t, steps)

	assert.Contains(t, resp.CombatLog, "Defensive Duelist", "the reaction should be announced once")
	assert.NotContains(t, resp.CombatLog, "Hit!",
		"both swings must resolve against AC 18 — the lingering +3 lasts until the start of the defender's next turn")
	assert.Equal(t, 2, strings.Count(resp.CombatLog, "Miss"), "both swings should miss")
}

// The lingering AC costs exactly ONE reaction across the whole multiattack.
func TestExecuteEnemyTurn_DefensiveDuelistSpendsExactlyOneReaction(t *testing.T) {
	f := newDDFixture(t)

	steps := `[` +
		ddAttackStep(f.targetID, "Scimitar", 12, 16, 5, ddChosenReactionJSON) + `,` +
		ddAttackStep(f.targetID, "Shortsword", 13, 17, 5, "") +
		`]`

	f.execEnemyTurn(t, steps)

	assert.Equal(t, 1, f.declarations, "the lingering AC must not cost a second reaction")
}

// A melee attack from a DIFFERENT creature later in the same round still gets the
// bonus, and still costs no second reaction (the PC's reaction is already spent).
func TestExecuteEnemyTurn_DefensiveDuelistACAppliesToLaterAttackerSameRound(t *testing.T) {
	f := newDDFixture(t)

	// Goblin A spends the reaction.
	f.execEnemyTurn(t, `[`+ddAttackStep(f.targetID, "Scimitar", 12, 16, 5, ddChosenReactionJSON)+`]`)
	require.Equal(t, 1, f.declarations)

	// Goblin B swings later in the same round. Its plan carries no reaction (the
	// PC has none left), but the lingering AC is still up.
	f.store.listReactionDeclarationsByCombatantFn = func(_ context.Context, _ refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{{Status: "used", UsedOnRound: sql.NullInt32{Int32: 1, Valid: true}}}, nil
	}
	resp := f.execEnemyTurn(t, `[`+ddAttackStep(f.targetID, "Greatclub", 13, 17, 5, "")+`]`)

	assert.Contains(t, resp.CombatLog, "Miss", "the lingering +3 AC applies to a later attacker in the same round")
	assert.Equal(t, 1, f.declarations, "no second reaction may be spent")
}

// The bonus is gone once the defender's next turn starts — the marker expires
// through the generic start_of_turn condition-expiry sweep.
func TestDefensiveDuelistAC_ExpiresAtStartOfDefendersNextTurn(t *testing.T) {
	f := newDDFixture(t)

	f.execEnemyTurn(t, `[`+ddAttackStep(f.targetID, "Scimitar", 12, 16, 5, ddChosenReactionJSON)+`]`)
	require.True(t, HasCondition(*f.targetConds, defensiveDuelistACCondition),
		"spending the reaction should stamp the lingering-AC marker")

	// Start of the defender's next turn (round 2): the sweep clears it.
	remaining, expired, err := CheckExpiredConditions(*f.targetConds, 2, f.targetID.String(), "start_of_turn")
	require.NoError(t, err)
	assert.False(t, HasCondition(remaining, defensiveDuelistACCondition),
		"the Defensive Duelist AC must not survive the start of the defender's next turn")
	assert.NotEmpty(t, expired)

	// And with the marker gone, an identical swing lands again.
	*f.targetConds = remaining
	f.store.listReactionDeclarationsByCombatantFn = func(_ context.Context, _ refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{{Status: "used", UsedOnRound: sql.NullInt32{Int32: 1, Valid: true}}}, nil
	}
	resp := f.execEnemyTurn(t, `[`+ddAttackStep(f.targetID, "Scimitar", 13, 17, 5, "")+`]`)
	assert.Contains(t, resp.CombatLog, "Hit!", "with the marker expired the swing beats base AC 15 again")
}

// RAW scopes the lingering bonus to MELEE attacks. A ranged swing must not get it.
func TestExecuteEnemyTurn_DefensiveDuelistACDoesNotApplyToRangedAttacks(t *testing.T) {
	f := newDDFixture(t)

	f.execEnemyTurn(t, `[`+ddAttackStep(f.targetID, "Scimitar", 12, 16, 5, ddChosenReactionJSON)+`]`)
	require.True(t, HasCondition(*f.targetConds, defensiveDuelistACCondition))

	f.store.listReactionDeclarationsByCombatantFn = func(_ context.Context, _ refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{{Status: "used", UsedOnRound: sql.NullInt32{Int32: 1, Valid: true}}}, nil
	}
	// reach_ft 150 = a longbow shot in the seeded creature data.
	resp := f.execEnemyTurn(t, `[`+ddAttackStep(f.targetID, "Longbow", 13, 17, 150, "")+`]`)

	assert.Contains(t, resp.CombatLog, "Hit!", "Defensive Duelist's lingering AC is melee-only")
}

// Uncanny Dodge is a per-hit damage-halving reaction, not a lasting AC bonus: it
// must never stamp the lingering-AC marker.
func TestExecuteEnemyTurn_UncannyDodgeLeavesNoLingeringACMarker(t *testing.T) {
	f := newDDFixture(t)

	ud := `{"id": "uncanny-dodge", "label": "Uncanny Dodge (halve damage)", "halve_damage": true, "reason": "Uncanny Dodge"}`
	resp := f.execEnemyTurn(t, `[`+ddAttackStep(f.targetID, "Scimitar", 12, 16, 5, ud)+`]`)

	assert.Contains(t, resp.CombatLog, "Uncanny Dodge", "Uncanny Dodge still resolves")
	assert.Contains(t, resp.CombatLog, "Hit!", "Uncanny Dodge does not change hit/miss")
	assert.False(t, HasCondition(*f.targetConds, defensiveDuelistACCondition),
		"a damage-halving reaction must not grant a lingering AC bonus")
}

// isMeleeAttackStep classifies the overloaded reach_ft field seeded on creature
// attacks: true melee reaches are 5-20ft (0 = unspecified swarm bites), while
// 25ft+ entries are ranged distances (javelin 30, longbow 150).
func TestIsMeleeAttackStep(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reachFt int
		want    bool
	}{
		{"unspecified swarm bite", 0, true},
		{"standard melee reach", 5, true},
		{"halberd / large creature", 10, true},
		{"huge creature reach", 15, true},
		{"gargantuan tail", 20, true},
		{"thrown javelin", 30, false},
		{"longbow", 150, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isMeleeAttackStep(&AttackStep{ReachFt: tc.reachFt}))
		})
	}

	assert.False(t, isMeleeAttackStep(nil), "a nil step is not a melee attack")
}

// lingeringDefensiveDuelistAC reads the marker off the defender's conditions.
func TestLingeringDefensiveDuelistAC(t *testing.T) {
	marked := refdata.Combatant{Conditions: json.RawMessage(
		`[{"condition":"defensive_duelist_ac","duration_rounds":1,"expires_on":"start_of_turn","ac_bonus":3}]`)}
	bare := refdata.Combatant{Conditions: json.RawMessage(`[]`)}

	assert.Equal(t, 3, lingeringDefensiveDuelistAC(marked, &AttackStep{ReachFt: 5}))
	assert.Equal(t, 0, lingeringDefensiveDuelistAC(marked, &AttackStep{ReachFt: 150}), "melee-only")
	assert.Equal(t, 0, lingeringDefensiveDuelistAC(bare, &AttackStep{ReachFt: 5}), "no marker, no bonus")
}

// applyLingeringDefensiveDuelistAC: a non-positive bonus is a no-op, the marker
// is never duplicated, and store failures surface as wrapped errors.
func TestApplyLingeringDefensiveDuelistAC(t *testing.T) {
	defenderID := uuid.New()

	t.Run("non-positive bonus writes nothing", func(t *testing.T) {
		store := defaultMockStore()
		wrote := false
		store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			wrote = true
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		}
		require.NoError(t, NewService(store).applyLingeringDefensiveDuelistAC(context.Background(), defenderID, 0))
		assert.False(t, wrote)
	})

	t.Run("already marked is a no-op", func(t *testing.T) {
		store := defaultMockStore()
		wrote := false
		store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, Conditions: json.RawMessage(
				`[{"condition":"defensive_duelist_ac","duration_rounds":1,"expires_on":"start_of_turn","ac_bonus":3}]`)}, nil
		}
		store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			wrote = true
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		}
		require.NoError(t, NewService(store).applyLingeringDefensiveDuelistAC(context.Background(), defenderID, 3))
		assert.False(t, wrote, "the marker must not be stamped twice")
	})

	t.Run("lookup failure is wrapped", func(t *testing.T) {
		store := defaultMockStore()
		store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, sql.ErrNoRows
		}
		err := NewService(store).applyLingeringDefensiveDuelistAC(context.Background(), defenderID, 3)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "getting defender")
	})

	t.Run("persist failure is wrapped", func(t *testing.T) {
		store := defaultMockStore()
		store.updateCombatantConditionsFn = func(_ context.Context, _ refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{}, sql.ErrConnDone
		}
		err := NewService(store).applyLingeringDefensiveDuelistAC(context.Background(), defenderID, 3)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "applying Defensive Duelist AC marker")
	})
}
