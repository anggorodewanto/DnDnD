package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// COV-19 (per-turn re-save): Hold Person / Hold Monster grant the 2024
// end-of-turn repeat saving throw ("save ends"); a damage-or-negate spell like
// Ray of Sickness does not.
func TestSpellResavesAtEndOfTurn(t *testing.T) {
	assert.True(t, spellResavesAtEndOfTurn(makeHoldPerson()), "Hold Person is save-ends")
	assert.False(t, spellResavesAtEndOfTurn(makeRayOfSickness()), "Ray of Sickness is not save-ends")
}

// firstSaveEndsCondition finds the first condition carrying end-of-turn re-save
// metadata (SaveEndsAbility set) and ignores plain conditions.
func TestFirstSaveEndsCondition(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "prone"},
		{Condition: "paralyzed", SaveEndsAbility: "wis", SaveEndsDC: 14, SourceSpell: "hold-person"},
	}
	got, ok := firstSaveEndsCondition(conds)
	require.True(t, ok)
	assert.Equal(t, "paralyzed", got.Condition)
	assert.Equal(t, "wis", got.SaveEndsAbility)

	_, ok = firstSaveEndsCondition([]CombatCondition{{Condition: "prone"}})
	assert.False(t, ok, "no save-ends metadata → not found")
}

// A failed Hold Person save now stamps the applied paralyzed condition with the
// re-save ability + DC so the turn engine can re-roll it at end of turn.
func TestResolveAoEPendingSaves_HoldPerson_StampsSaveEnds(t *testing.T) {
	encounterID := uuid.New()
	casterID := uuid.New()
	failerID := uuid.New()
	spellID := "hold-person"
	source := AoEPendingSaveSource(spellID)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeHoldPerson(), nil }
	store.listSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: failerID, Source: source, Ability: "wis", Dc: 14, Status: "rolled", RollResult: sql.NullInt32{Int32: 8, Valid: true}, Success: sql.NullBool{Bool: false, Valid: true}},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: casterID, DisplayName: "Vale", ConcentrationSpellID: sql.NullString{String: spellID, Valid: true}},
		}, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "Grey Man", Conditions: json.RawMessage(`[]`)}, nil
	}
	var condUpdates []refdata.UpdateCombatantConditionsParams
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		condUpdates = append(condUpdates, arg)
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveAoEPendingSaves(context.Background(), encounterID, spellID, dice.NewRoller(func(_ int) int { return 4 }))
	require.NoError(t, err)

	require.Len(t, condUpdates, 1)
	conds, err := ListConditions(condUpdates[0].Conditions)
	require.NoError(t, err)
	require.Len(t, conds, 1)
	assert.Equal(t, "paralyzed", conds[0].Condition)
	assert.Equal(t, "wis", conds[0].SaveEndsAbility, "re-save ability stamped from the spell")
	assert.Equal(t, 14, conds[0].SaveEndsDC, "re-save DC frozen from the pending save")
}

// A non-save-ends spell (Ray of Sickness) applies its condition WITHOUT re-save
// metadata — it clears only via combat end / the DM editor, not a repeat save.
func TestResolveAoEPendingSaves_NonResaveSpell_NoStamp(t *testing.T) {
	encounterID := uuid.New()
	failerID := uuid.New()
	spellID := "ray-of-sickness"
	source := AoEPendingSaveSource(spellID)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeRayOfSickness(), nil }
	store.listSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: failerID, Source: source, Ability: "con", Dc: 13, Status: "rolled", RollResult: sql.NullInt32{Int32: 6, Valid: true}, Success: sql.NullBool{Bool: false, Valid: true}},
		}, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "Goblin", HpCurrent: 20, HpMax: 20, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
	}
	var condUpdates []refdata.UpdateCombatantConditionsParams
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		condUpdates = append(condUpdates, arg)
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveAoEPendingSaves(context.Background(), encounterID, spellID, dice.NewRoller(func(_ int) int { return 4 }))
	require.NoError(t, err)

	require.Len(t, condUpdates, 1)
	conds, err := ListConditions(condUpdates[0].Conditions)
	require.NoError(t, err)
	require.Len(t, conds, 1)
	assert.Equal(t, "poisoned", conds[0].Condition)
	assert.Empty(t, conds[0].SaveEndsAbility, "non-save-ends spell leaves no re-save metadata")
}

// makeParalyzedByHoldPerson returns a bearer combatant carrying a save-ends
// paralyzed condition scoped to caster (concentration teardown), plus the
// stubs needed by applySaveEndsOutcome / breakStoredConcentration.
func holdPersonResaveStore(t *testing.T, encounterID, casterID, bearerID uuid.UUID) (*mockStore, *int, *uuid.UUID) {
	t.Helper()
	paralyzed := json.RawMessage(`[{"condition":"paralyzed","source_combatant_id":"` + casterID.String() + `","source_spell":"hold-person","save_ends_ability":"wis","save_ends_dc":14}]`)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == bearerID {
			return refdata.Combatant{ID: bearerID, EncounterID: encounterID, DisplayName: "Grey Man", IsNpc: true, IsAlive: true, Conditions: paralyzed}, nil
		}
		return refdata.Combatant{ID: casterID, EncounterID: encounterID, DisplayName: "Vale", Conditions: json.RawMessage(`[]`), ConcentrationSpellID: sql.NullString{String: "hold-person", Valid: true}, ConcentrationSpellName: sql.NullString{String: "Hold Person", Valid: true}}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: casterID, EncounterID: encounterID, DisplayName: "Vale", Conditions: json.RawMessage(`[]`), ConcentrationSpellID: sql.NullString{String: "hold-person", Valid: true}, ConcentrationSpellName: sql.NullString{String: "Hold Person", Valid: true}},
			{ID: bearerID, EncounterID: encounterID, DisplayName: "Grey Man", Conditions: paralyzed},
		}, nil
	}
	store.getCombatantConcentrationFn = func(_ context.Context, _ uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
		return refdata.GetCombatantConcentrationRow{
			ConcentrationSpellID:   sql.NullString{String: "hold-person", Valid: true},
			ConcentrationSpellName: sql.NullString{String: "Hold Person", Valid: true},
		}, nil
	}
	clearCalls := 0
	clearedID := uuid.Nil
	store.clearCombatantConcentrationFn = func(_ context.Context, id uuid.UUID) error {
		clearCalls++
		clearedID = id
		return nil
	}
	return store, &clearCalls, &clearedID
}

// On a SUCCESSFUL end-of-turn re-save the paralyzed condition is removed and,
// as the sole held target, the caster's concentration drops (the spell ends).
func TestApplySaveEndsOutcome_Success_RemovesConditionAndDropsConcentration(t *testing.T) {
	encounterID, casterID, bearerID := uuid.New(), uuid.New(), uuid.New()
	store, clearCalls, clearedID := holdPersonResaveStore(t, encounterID, casterID, bearerID)

	var condUpdates []refdata.UpdateCombatantConditionsParams
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		condUpdates = append(condUpdates, arg)
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	bearer := refdata.Combatant{ID: bearerID, EncounterID: encounterID, DisplayName: "Grey Man", IsNpc: true}
	cond := CombatCondition{Condition: "paralyzed", SaveEndsAbility: "wis", SaveEndsDC: 14, SourceSpell: "hold-person", SourceCombatantID: casterID.String()}

	msgs, err := svc.applySaveEndsOutcome(context.Background(), encounterID, bearer, cond, true)
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	require.GreaterOrEqual(t, len(condUpdates), 1, "the paralyzed condition is removed from the bearer")
	assert.Equal(t, 1, *clearCalls, "the caster's now-purposeless concentration drops once")
	assert.Equal(t, casterID, *clearedID)
}

// On a FAILED re-save the condition persists — no removal, no concentration drop.
func TestApplySaveEndsOutcome_Fail_Persists(t *testing.T) {
	encounterID, casterID, bearerID := uuid.New(), uuid.New(), uuid.New()
	store, clearCalls, _ := holdPersonResaveStore(t, encounterID, casterID, bearerID)

	condUpdateCalls := 0
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		condUpdateCalls++
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	bearer := refdata.Combatant{ID: bearerID, EncounterID: encounterID, DisplayName: "Grey Man", IsNpc: true}
	cond := CombatCondition{Condition: "paralyzed", SaveEndsAbility: "wis", SaveEndsDC: 14, SourceSpell: "hold-person", SourceCombatantID: casterID.String()}

	msgs, err := svc.applySaveEndsOutcome(context.Background(), encounterID, bearer, cond, false)
	require.NoError(t, err)
	require.NotEmpty(t, msgs, "a 'still held' log line is returned")
	assert.Equal(t, 0, condUpdateCalls, "the condition is not removed on a failed save")
	assert.Equal(t, 0, *clearCalls, "concentration is not dropped on a failed save")
}

// A multi-target save-ends spell where OTHER targets remain held keeps the
// caster's concentration — only this bearer is freed.
func TestApplySaveEndsOutcome_Success_OtherTargetHeld_KeepsConcentration(t *testing.T) {
	encounterID, casterID, bearerID, otherID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	stillHeld := json.RawMessage(`[{"condition":"paralyzed","source_combatant_id":"` + casterID.String() + `","source_spell":"hold-person","save_ends_ability":"wis","save_ends_dc":14}]`)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, EncounterID: encounterID, DisplayName: "Bearer", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: casterID, EncounterID: encounterID, DisplayName: "Vale", ConcentrationSpellID: sql.NullString{String: "hold-person", Valid: true}},
			{ID: otherID, EncounterID: encounterID, DisplayName: "Other", Conditions: stillHeld},
		}, nil
	}
	clearCalls := 0
	store.clearCombatantConcentrationFn = func(_ context.Context, _ uuid.UUID) error { clearCalls++; return nil }
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	bearer := refdata.Combatant{ID: bearerID, EncounterID: encounterID, DisplayName: "Bearer"}
	cond := CombatCondition{Condition: "paralyzed", SaveEndsAbility: "wis", SaveEndsDC: 14, SourceSpell: "hold-person", SourceCombatantID: casterID.String()}

	_, err := svc.applySaveEndsOutcome(context.Background(), encounterID, bearer, cond, true)
	require.NoError(t, err)
	assert.Equal(t, 0, clearCalls, "another target is still held → concentration persists")
}

// The NPC path (ExecuteEnemyTurn hook): the service rolls the re-save honestly
// server-side. A high roll clears the paralysis and drops concentration.
func TestRollNPCEndOfTurnResave_Success(t *testing.T) {
	encounterID, casterID, bearerID := uuid.New(), uuid.New(), uuid.New()
	store, clearCalls, _ := holdPersonResaveStore(t, encounterID, casterID, bearerID)
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	svc.SetRoller(dice.NewRoller(func(_ int) int { return 19 })) // d20=19, NPC save bonus +0 → 19 >= 14

	bearer := refdata.Combatant{ID: bearerID, EncounterID: encounterID, DisplayName: "Grey Man", IsNpc: true,
		Conditions: json.RawMessage(`[{"condition":"paralyzed","source_combatant_id":"` + casterID.String() + `","source_spell":"hold-person","save_ends_ability":"wis","save_ends_dc":14}]`)}

	res, err := svc.rollNPCEndOfTurnResave(context.Background(), encounterID, bearer)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Resolved)
	assert.True(t, res.Success, "19 vs DC 14 succeeds")
	assert.Equal(t, 1, *clearCalls, "concentration drops on the NPC's successful re-save")
}

// A low roll leaves the NPC paralyzed and concentration intact.
func TestRollNPCEndOfTurnResave_Fail(t *testing.T) {
	encounterID, casterID, bearerID := uuid.New(), uuid.New(), uuid.New()
	store, clearCalls, _ := holdPersonResaveStore(t, encounterID, casterID, bearerID)

	svc := NewService(store)
	svc.SetRoller(dice.NewRoller(func(_ int) int { return 3 })) // d20=3 → 3 < 14

	bearer := refdata.Combatant{ID: bearerID, EncounterID: encounterID, DisplayName: "Grey Man", IsNpc: true,
		Conditions: json.RawMessage(`[{"condition":"paralyzed","source_combatant_id":"` + casterID.String() + `","source_spell":"hold-person","save_ends_ability":"wis","save_ends_dc":14}]`)}

	res, err := svc.rollNPCEndOfTurnResave(context.Background(), encounterID, bearer)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Resolved)
	assert.False(t, res.Success, "3 vs DC 14 fails")
	assert.Equal(t, 0, *clearCalls)
}

// A bearer with no save-ends condition is a no-op (nil result).
func TestRollNPCEndOfTurnResave_NoSaveEndsCondition_Noop(t *testing.T) {
	svc := NewService(defaultMockStore())
	bearer := refdata.Combatant{ID: uuid.New(), DisplayName: "Ogre", Conditions: json.RawMessage(`[{"condition":"prone"}]`)}
	res, err := svc.rollNPCEndOfTurnResave(context.Background(), uuid.New(), bearer)
	require.NoError(t, err)
	assert.Nil(t, res, "no save-ends condition → nothing to resolve")
}

// The PC path (/save): the player's rolled total is compared to the frozen DC;
// a total at/over the DC clears the condition.
func TestResolveEndOfTurnResaveForCombatant_PCSuccess(t *testing.T) {
	encounterID, casterID, bearerID := uuid.New(), uuid.New(), uuid.New()
	store, clearCalls, _ := holdPersonResaveStore(t, encounterID, casterID, bearerID)
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	res, err := svc.resolveEndOfTurnResaveForCombatant(context.Background(), encounterID, bearerID, "wis", 16, false)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Resolved)
	assert.True(t, res.Success, "16 vs DC 14 succeeds")
	assert.Equal(t, 1, *clearCalls)
}

// A player /save for an ability that no save-ends condition uses is a no-op.
func TestResolveEndOfTurnResaveForCombatant_AbilityMismatch_Noop(t *testing.T) {
	encounterID, casterID, bearerID := uuid.New(), uuid.New(), uuid.New()
	store, _, _ := holdPersonResaveStore(t, encounterID, casterID, bearerID)

	svc := NewService(store)
	res, err := svc.resolveEndOfTurnResaveForCombatant(context.Background(), encounterID, bearerID, "dex", 20, false)
	require.NoError(t, err)
	assert.Nil(t, res, "a DEX save does not resolve a WIS save-ends condition")
}

// skipOrActivate: a creature held by a save-ends condition is ACTIVATED with
// ResavePending (not silently skipped as incapacitated) so it gets its
// end-of-turn repeat save. A plainly-paralyzed creature is still skipped.
func TestSkipOrActivate_SaveEndsBearer_ActivatesResavePending(t *testing.T) {
	encounterID := uuid.New()
	bearerID := uuid.New()
	store := defaultMockStore()
	skipCalls := 0
	store.skipTurnFn = func(_ context.Context, id uuid.UUID) (refdata.Turn, error) {
		skipCalls++
		return refdata.Turn{ID: id, Status: "skipped"}, nil
	}

	svc := NewService(store)
	bearer := refdata.Combatant{ID: bearerID, EncounterID: encounterID, DisplayName: "Grey Man", IsNpc: true, IsAlive: true,
		Conditions: json.RawMessage(`[{"condition":"paralyzed","source_combatant_id":"x","source_spell":"hold-person","save_ends_ability":"wis","save_ends_dc":14}]`)}
	conds, _ := parseConditions(bearer.Conditions)

	info, skipped, err := svc.skipOrActivate(context.Background(), encounterID, 2, bearer, conds, nil)
	require.NoError(t, err)
	assert.Nil(t, skipped, "a save-ends bearer is not skipped")
	assert.True(t, info.ResavePending, "the active turn is flagged for the end-of-turn re-save")
	assert.Equal(t, "wis", info.ResaveAbility)
	assert.Equal(t, "paralyzed", info.ResaveConditionName)
	assert.Equal(t, 0, skipCalls, "no turn was skipped")
}

// Integration (the live grey-man path): running a paralyzed NPC's turn via
// ExecuteEnemyTurn rolls its end-of-turn WIS re-save server-side; a success
// clears the paralysis and drops the caster's concentration.
func TestExecuteEnemyTurn_ParalyzedNPC_RollsEndOfTurnResave(t *testing.T) {
	encounterID, casterID, npcID := uuid.New(), uuid.New(), uuid.New()
	paralyzed := json.RawMessage(`[{"condition":"paralyzed","source_combatant_id":"` + casterID.String() + `","source_spell":"hold-person","save_ends_ability":"wis","save_ends_dc":14}]`)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == npcID {
			return refdata.Combatant{ID: npcID, EncounterID: encounterID, DisplayName: "Grey Man", IsNpc: true, IsAlive: true, Conditions: paralyzed}, nil
		}
		return refdata.Combatant{ID: casterID, EncounterID: encounterID, DisplayName: "Vale", Conditions: json.RawMessage(`[]`), ConcentrationSpellID: sql.NullString{String: "hold-person", Valid: true}, ConcentrationSpellName: sql.NullString{String: "Hold Person", Valid: true}}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(_ context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: eid, CombatantID: npcID}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: casterID, EncounterID: encounterID, DisplayName: "Vale", Conditions: json.RawMessage(`[]`), ConcentrationSpellID: sql.NullString{String: "hold-person", Valid: true}, ConcentrationSpellName: sql.NullString{String: "Hold Person", Valid: true}},
			{ID: npcID, EncounterID: encounterID, DisplayName: "Grey Man", Conditions: paralyzed},
		}, nil
	}
	store.getCombatantConcentrationFn = func(_ context.Context, _ uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
		return refdata.GetCombatantConcentrationRow{ConcentrationSpellID: sql.NullString{String: "hold-person", Valid: true}, ConcentrationSpellName: sql.NullString{String: "Hold Person", Valid: true}}, nil
	}
	clearCalls := 0
	store.clearCombatantConcentrationFn = func(_ context.Context, _ uuid.UUID) error { clearCalls++; return nil }
	condCleared := false
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		conds, _ := ListConditions(arg.Conditions)
		if arg.ID == npcID && len(conds) == 0 {
			condCleared = true
		}
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	svc.SetRoller(dice.NewRoller(func(_ int) int { return 18 })) // d20=18, +0 → 18 >= 14 success

	res, err := svc.ExecuteEnemyTurn(context.Background(), encounterID, TurnPlan{CombatantID: npcID}, dice.NewRoller(func(_ int) int { return 1 }))
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, condCleared, "the paralyzed NPC's condition is removed on its successful end-of-turn re-save")
	assert.Equal(t, 1, clearCalls, "the caster's concentration drops when the sole held target breaks free")
	assert.Contains(t, res.CombatLog, "re-save", "the combat log surfaces the end-of-turn re-save")
}

func TestSkipOrActivate_PlainParalyzed_StillSkipped(t *testing.T) {
	encounterID := uuid.New()
	bearerID := uuid.New()
	store := defaultMockStore()
	skipCalls := 0
	store.skipTurnFn = func(_ context.Context, id uuid.UUID) (refdata.Turn, error) {
		skipCalls++
		return refdata.Turn{ID: id, Status: "skipped"}, nil
	}

	svc := NewService(store)
	bearer := refdata.Combatant{ID: bearerID, EncounterID: encounterID, DisplayName: "Statue", IsNpc: true, IsAlive: true,
		Conditions: json.RawMessage(`[{"condition":"paralyzed"}]`)}
	conds, _ := parseConditions(bearer.Conditions)

	_, skipped, err := svc.skipOrActivate(context.Background(), encounterID, 2, bearer, conds, nil)
	require.NoError(t, err)
	require.NotNil(t, skipped, "a plainly-paralyzed creature (no re-save) is still auto-skipped")
	assert.Equal(t, 1, skipCalls)
}
