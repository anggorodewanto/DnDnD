package combat_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// ISSUE-044 — AoE save-for-half spells never applied their damage in
// production once every target's save resolved. These tests drive the REAL
// service ↔ store ↔ Postgres wiring (not a mock) so they exercise the actual
// ListSavesByEncounter SQL. A mock that returns whatever rows the test injects
// hid the bug: the apply gate listed rows with a WHERE status='pending' filter,
// so the just-rolled row was invisible and damage never landed.

// insertFireballSpell seeds a damaging, save-for-half Fireball into the spells
// reference table so Service.ResolveAoEPendingSaves can look it up.
func insertFireballSpell(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO spells
		(id, name, level, school, casting_time, range_type, components, duration, description, classes, damage, save_ability, save_effect, resolution_mode)
		VALUES ('fireball','Fireball',3,'evocation','1 action','ranged','{V,S,M}','instantaneous','', '{wizard,sorcerer}',
		        '{"dice":"8d6","damage_type":"fire"}','dex','half_damage','auto')
		ON CONFLICT (id) DO NOTHING`)
	require.NoError(t, err)
}

// addBlastMonster adds an NPC combatant (no character/creature ref → passes the
// monster-vs-player guard) with the given HP.
func addBlastMonster(t *testing.T, svc *combat.Service, encID uuid.UUID, short, name string, hp int32, col string) refdata.Combatant {
	t.Helper()
	c, err := svc.AddCombatant(context.Background(), encID, combat.CombatantParams{
		ShortID: short, DisplayName: name,
		HPMax: hp, HPCurrent: hp, AC: 12, SpeedFt: 30,
		PositionCol: col, PositionRow: 1, IsAlive: true, IsNPC: true, IsVisible: true,
	})
	require.NoError(t, err)
	return c
}

func readHP(t *testing.T, db *sql.DB, id uuid.UUID) int {
	t.Helper()
	var hp int
	require.NoError(t, db.QueryRow(`SELECT hp_current FROM combatants WHERE id=$1`, id).Scan(&hp))
	return hp
}

func readSaveStatus(t *testing.T, db *sql.DB, id uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM pending_saves WHERE id=$1`, id).Scan(&status))
	return status
}

// createRolledSave creates a pending AoE save and resolves it to 'rolled' with
// the given success, returning the row id. This is the exact state the
// production /save and DM resolver paths leave a row in right before the apply
// gate fires.
func createRolledSave(t *testing.T, queries *refdata.Queries, encID, combatantID uuid.UUID, success bool, total int) uuid.UUID {
	t.Helper()
	created, err := queries.CreatePendingSave(context.Background(), refdata.CreatePendingSaveParams{
		EncounterID: encID, CombatantID: combatantID, Ability: "dex", Dc: 15,
		Source: combat.AoEPendingSaveSource("fireball"), CoverBonus: 0,
	})
	require.NoError(t, err)
	_, err = queries.UpdatePendingSaveResult(context.Background(), refdata.UpdatePendingSaveResultParams{
		ID:         created.ID,
		RollResult: sql.NullInt32{Int32: int32(total), Valid: true},
		Success:    sql.NullBool{Bool: success, Valid: true},
	})
	require.NoError(t, err)
	return created.ID
}

// fixedDamageRoller rolls `face` for every die (so 8d6 → 8*face).
func fixedDamageRoller(face int) *dice.Roller {
	return dice.NewRoller(func(int) int { return face })
}

// TestIntegration_ISSUE044_LastSaveAppliesAoEDamage is the core regression: once
// the only target's save is recorded 'rolled', ResolveAoEPendingSaves must roll
// + apply damage and mark the row 'applied'. Before the fix this returned
// (nil,nil) and HP never changed because the apply gate listed pending-only.
func TestIntegration_ISSUE044_LastSaveAppliesAoEDamage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	insertFireballSpell(t, db)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{CampaignID: campaignID, Name: "issue044"})
	require.NoError(t, err)
	mob := addBlastMonster(t, svc, enc.ID, "G1", "Goblin", 100, "A")

	saveID := createRolledSave(t, queries, enc.ID, mob.ID, false, 4) // failed save

	res, err := svc.ResolveAoEPendingSaves(context.Background(), enc.ID, "fireball", fixedDamageRoller(4))
	require.NoError(t, err)
	require.NotNil(t, res, "all rows resolved → damage must apply")
	require.Len(t, res.Targets, 1)
	assert.Equal(t, 32, res.Targets[0].DamageDealt, "failed save takes full 8d6=32")

	assert.Equal(t, 68, readHP(t, db, mob.ID), "100 HP - 32 = 68")
	assert.Equal(t, "applied", readSaveStatus(t, db, saveID), "row lifecycle pending→rolled→applied")
}

// TestIntegration_ISSUE044_ApplyIsIdempotent proves the apply runs exactly once:
// a second drive after the row is 'applied' is a no-op and does not re-damage.
func TestIntegration_ISSUE044_ApplyIsIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	insertFireballSpell(t, db)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{CampaignID: campaignID, Name: "issue044-idem"})
	require.NoError(t, err)
	mob := addBlastMonster(t, svc, enc.ID, "G1", "Goblin", 100, "A")
	saveID := createRolledSave(t, queries, enc.ID, mob.ID, false, 4)

	_, err = svc.ResolveAoEPendingSaves(context.Background(), enc.ID, "fireball", fixedDamageRoller(4))
	require.NoError(t, err)
	assert.Equal(t, 68, readHP(t, db, mob.ID))

	res2, err := svc.ResolveAoEPendingSaves(context.Background(), enc.ID, "fireball", fixedDamageRoller(4))
	require.NoError(t, err)
	assert.Nil(t, res2, "second drive is an idempotent no-op")
	assert.Equal(t, 68, readHP(t, db, mob.ID), "HP must not be reduced twice")
	assert.Equal(t, "applied", readSaveStatus(t, db, saveID))
}

// TestIntegration_ISSUE044_MultiTargetAppliesOnceAfterLast proves a multi-target
// blast waits for every save then applies once. Damage lands only after the
// last target resolves, and all rows end 'applied'.
func TestIntegration_ISSUE044_MultiTargetAppliesOnceAfterLast(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	insertFireballSpell(t, db)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{CampaignID: campaignID, Name: "issue044-multi"})
	require.NoError(t, err)
	m1 := addBlastMonster(t, svc, enc.ID, "G1", "Goblin 1", 100, "A")
	m2 := addBlastMonster(t, svc, enc.ID, "G2", "Goblin 2", 100, "B")

	// Only the first target has rolled; the second is still pending.
	save1 := createRolledSave(t, queries, enc.ID, m1.ID, false, 4)
	created2, err := queries.CreatePendingSave(context.Background(), refdata.CreatePendingSaveParams{
		EncounterID: enc.ID, CombatantID: m2.ID, Ability: "dex", Dc: 15,
		Source: combat.AoEPendingSaveSource("fireball"),
	})
	require.NoError(t, err)

	res, err := svc.ResolveAoEPendingSaves(context.Background(), enc.ID, "fireball", fixedDamageRoller(4))
	require.NoError(t, err)
	assert.Nil(t, res, "must wait while a target's save is still pending")
	assert.Equal(t, 100, readHP(t, db, m1.ID), "no damage before the last save resolves")

	// Resolve the second target, then drive again.
	_, err = queries.UpdatePendingSaveResult(context.Background(), refdata.UpdatePendingSaveResultParams{
		ID:         created2.ID,
		RollResult: sql.NullInt32{Int32: 20, Valid: true},
		Success:    sql.NullBool{Bool: true, Valid: true}, // success → half
	})
	require.NoError(t, err)

	res2, err := svc.ResolveAoEPendingSaves(context.Background(), enc.ID, "fireball", fixedDamageRoller(4))
	require.NoError(t, err)
	require.NotNil(t, res2)
	require.Len(t, res2.Targets, 2)
	assert.Equal(t, 68, readHP(t, db, m1.ID), "failed save: 100-32")
	assert.Equal(t, 84, readHP(t, db, m2.ID), "successful save: 100-16 (half of 32)")
	assert.Equal(t, "applied", readSaveStatus(t, db, save1))
	assert.Equal(t, "applied", readSaveStatus(t, db, created2.ID))
}

// TestIntegration_ISSUE044_MonsterSaveRecoversRolledRow proves the live recovery
// path: a row stuck at 'rolled' (its save already failed but damage never
// applied) can be re-POSTed to ResolveMonsterPendingSave, which applies the
// damage using the STORED roll — no re-roll, success preserved — and marks the
// row 'applied'. A second call returns ErrSaveAlreadyResolved (409).
func TestIntegration_ISSUE044_MonsterSaveRecoversRolledRow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	insertFireballSpell(t, db)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{CampaignID: campaignID, Name: "issue044-recover"})
	require.NoError(t, err)
	mob := addBlastMonster(t, svc, enc.ID, "G1", "Goblin", 100, "A")
	saveID := createRolledSave(t, queries, enc.ID, mob.ID, false, 4) // failed, total 4

	// A roller that records any d20 request and would FLIP the save to a
	// success if it were (wrongly) re-rolled.
	d20Rolled := false
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			d20Rolled = true
			return 20
		}
		return 4
	})
	svc.SetRoller(roller)

	resolution, err := svc.ResolveMonsterPendingSave(context.Background(), enc.ID, saveID)
	require.NoError(t, err)
	assert.False(t, d20Rolled, "recovery must NOT re-roll the d20")
	assert.False(t, resolution.Success, "stored failed save is preserved")
	assert.Equal(t, 0, resolution.NaturalRoll, "no fresh d20 → no natural roll recoverable")
	assert.Equal(t, 4, resolution.Total, "reports the stored roll total")
	require.NotNil(t, resolution.Damage)
	require.Len(t, resolution.Damage.Targets, 1)
	assert.Equal(t, 32, resolution.Damage.Targets[0].DamageDealt, "failed save → full 8d6=32")
	assert.Equal(t, 68, readHP(t, db, mob.ID))
	assert.Equal(t, "applied", readSaveStatus(t, db, saveID))

	// Second call: row is now 'applied' → 409.
	_, err = svc.ResolveMonsterPendingSave(context.Background(), enc.ID, saveID)
	require.ErrorIs(t, err, combat.ErrSaveAlreadyResolved)
}
