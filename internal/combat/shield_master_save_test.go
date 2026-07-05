package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// COV-9 — Shield Master's third half: add the equipped shield's AC bonus to a
// DEX saving throw against an effect that "targets only you" (a single-target
// save spell). The bonus rides the single-target pending save's CoverBonus.

func TestEquippedShieldACBonus(t *testing.T) {
	tests := []struct {
		name     string
		offHand  sql.NullString
		armor    refdata.Armor
		armorErr error
		want     int
	}{
		{"plain shield +2", sql.NullString{String: "shield", Valid: true}, refdata.Armor{ID: "shield", ArmorType: "shield", AcBase: 2}, nil, 2},
		{"magic shield +3", sql.NullString{String: "shield-1", Valid: true}, refdata.Armor{ID: "shield-1", ArmorType: "shield", AcBase: 3}, nil, 3},
		{"off-hand not a shield", sql.NullString{String: "torch", Valid: true}, refdata.Armor{ID: "torch", ArmorType: "light"}, nil, 0},
		{"empty off-hand", sql.NullString{}, refdata.Armor{}, nil, 0},
		{"armor lookup error", sql.NullString{String: "shield", Valid: true}, refdata.Armor{}, sql.ErrNoRows, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := defaultMockStore()
			ms.getArmorFn = func(_ context.Context, _ string) (refdata.Armor, error) { return tc.armor, tc.armorErr }
			char := refdata.Character{EquippedOffHand: tc.offHand}
			svc := NewService(ms)
			if got := svc.equippedShieldACBonus(context.Background(), char); got != tc.want {
				t.Errorf("equippedShieldACBonus() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestShieldMasterDexSaveBonus(t *testing.T) {
	shieldChar := shieldMasterChar(uuid.New())     // Shield Master feat + shield off-hand
	plainChar := makeWizardCharacter(uuid.New())   // no feat
	noShield := shieldMasterChar(uuid.New())       // feat but no shield
	noShield.EquippedOffHand = sql.NullString{}

	stunned := pcTargetWithChar(shieldChar.ID) // RAW: no bonus while incapacitated
	stunned.Conditions = json.RawMessage(`[{"condition":"stunned"}]`)

	tests := []struct {
		name    string
		ability string
		target  refdata.Combatant
		char    refdata.Character
		want    int
	}{
		{"dex + feat + shield", "dex", pcTargetWithChar(shieldChar.ID), shieldChar, 2},
		{"case-insensitive DEX", "DEX", pcTargetWithChar(shieldChar.ID), shieldChar, 2},
		{"non-dex save (wis)", "wis", pcTargetWithChar(shieldChar.ID), shieldChar, 0},
		{"feat but no shield", "dex", pcTargetWithChar(noShield.ID), noShield, 0},
		{"no feat", "dex", pcTargetWithChar(plainChar.ID), plainChar, 0},
		{"incapacitated saver", "dex", stunned, shieldChar, 0},
		{"monster target (no character)", "dex", makeSpellTarget(), shieldChar, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := defaultMockStore()
			ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return tc.char, nil }
			ms.getArmorFn = func(_ context.Context, _ string) (refdata.Armor, error) {
				return refdata.Armor{ID: "shield", ArmorType: "shield", AcBase: 2}, nil
			}
			svc := NewService(ms)
			if got := svc.shieldMasterDexSaveBonus(context.Background(), tc.target, tc.ability); got != tc.want {
				t.Errorf("shieldMasterDexSaveBonus() = %d, want %d", got, tc.want)
			}
		})
	}
}

// pcTargetWithChar builds a PC target combatant that carries the given character ID.
func pcTargetWithChar(charID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		EncounterID: uuid.New(),
		DisplayName: "Bran",
		PositionCol: "E",
		PositionRow: 6,
		Ac:          18,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}
}

// End-to-end through Cast: a PC casting a single-target DEX-save cantrip at a PC
// with Shield Master + shield enqueues the pending save with the shield bonus
// folded into CoverBonus, so /save resolution adds it to the target's roll.
func TestCast_SingleTargetDexSave_ShieldMasterTarget_AddsShieldBonus(t *testing.T) {
	cover := castDexSaveAtTarget(t, shieldMasterChar(uuid.New()))
	assert.Equal(t, int32(2), cover, "Shield Master + shield → +2 shield AC on the DEX save (CoverBonus)")
}

// Control: a PC target WITHOUT the Shield Master feat gets no shield-save bonus.
func TestCast_SingleTargetDexSave_PlainTarget_NoBonus(t *testing.T) {
	cover := castDexSaveAtTarget(t, makeWizardCharacter(uuid.New()))
	assert.Equal(t, int32(0), cover, "no Shield Master feat → no shield bonus on the save")
}

// castDexSaveAtTarget runs Cast for the DEX-save cantrip Sacred Flame aimed at a
// PC carrying targetChar, and returns the CoverBonus recorded on the single
// enqueued pending save. getCharacterFn returns targetChar for its own ID and a
// wizard caster otherwise.
func castDexSaveAtTarget(t *testing.T, targetChar refdata.Character) int32 {
	t.Helper()
	casterChar := makeWizardCharacter(uuid.New())
	caster := makeSpellCaster(casterChar.ID)
	target := pcTargetWithChar(targetChar.ID)

	var pendingCalls []refdata.CreatePendingSaveParams
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeSacredFlame(), nil }
	store.getCharacterFn = func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
		if id == targetChar.ID {
			return targetChar, nil
		}
		return casterChar, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	store.getArmorFn = func(_ context.Context, _ string) (refdata.Armor, error) {
		return refdata.Armor{ID: "shield", ArmorType: "shield", AcBase: 2}, nil
	}
	store.createPendingSaveFn = func(_ context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
		pendingCalls = append(pendingCalls, arg)
		return refdata.PendingSafe{ID: uuid.New(), CombatantID: arg.CombatantID, Ability: arg.Ability, Dc: arg.Dc, Source: arg.Source, Status: "pending"}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "sacred-flame",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}
	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.Len(t, pendingCalls, 1, "single-target save spell must enqueue exactly one pending save")
	return pendingCalls[0].CoverBonus
}
