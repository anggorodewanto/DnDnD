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

// moveMarkerFixture wires a mock store for the move-spell-marker path: a caster
// concentrating on `spellID`, an old marked target, and a fresh target. The
// returned captures let each test assert what was persisted.
type moveMarkerFixture struct {
	ms          *mockStore
	caster      refdata.Combatant
	oldTarget   refdata.Combatant
	newTarget   refdata.Combatant
	turn        refdata.Turn
	condWrites  map[uuid.UUID]json.RawMessage // combatantID -> last conditions written
	turnWritten *refdata.UpdateTurnActionsParams
}

func newMoveMarkerFixture(t *testing.T, condName, spellID string, oldTargetHP int32, concentrating bool) *moveMarkerFixture {
	t.Helper()
	encID := uuid.New()
	casterID, oldID, newID := uuid.New(), uuid.New(), uuid.New()

	marker := `[{"condition":"` + condName + `","source_spell":"` + spellID + `","source_combatant_id":"` + casterID.String() + `"}]`

	caster := refdata.Combatant{ID: casterID, EncounterID: encID, DisplayName: "Windreth", IsAlive: true, Conditions: json.RawMessage(`[]`)}
	oldTarget := refdata.Combatant{ID: oldID, EncounterID: encID, DisplayName: "Goblin", HpCurrent: oldTargetHP, IsAlive: oldTargetHP > 0, Conditions: json.RawMessage(marker)}
	newTarget := refdata.Combatant{ID: newID, EncounterID: encID, DisplayName: "Ogre", HpCurrent: 30, IsAlive: true, Conditions: json.RawMessage(`[]`)}

	fx := &moveMarkerFixture{
		caster: caster, oldTarget: oldTarget, newTarget: newTarget,
		turn:       refdata.Turn{ID: uuid.New(), CombatantID: casterID},
		condWrites: map[uuid.UUID]json.RawMessage{},
	}

	ms := defaultMockStore()
	ms.getCombatantConcentrationFn = func(_ context.Context, _ uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
		if !concentrating {
			return refdata.GetCombatantConcentrationRow{}, nil
		}
		return refdata.GetCombatantConcentrationRow{
			ConcentrationSpellID:   sql.NullString{String: spellID, Valid: true},
			ConcentrationSpellName: sql.NullString{String: "Spell", Valid: true},
		}, nil
	}
	ms.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		switch id {
		case oldID:
			return fx.oldTarget, nil
		case newID:
			return fx.newTarget, nil
		default:
			return caster, nil
		}
	}
	ms.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{fx.caster, fx.oldTarget, fx.newTarget}, nil
	}
	ms.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		fx.condWrites[arg.ID] = arg.Conditions
		// keep fixture state coherent so a re-read reflects the write
		if arg.ID == oldID {
			fx.oldTarget.Conditions = arg.Conditions
		}
		if arg.ID == newID {
			fx.newTarget.Conditions = arg.Conditions
		}
		return refdata.Combatant{ID: arg.ID, EncounterID: encID, DisplayName: fx.nameFor(arg.ID), Conditions: arg.Conditions}, nil
	}
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		fx.turnWritten = &arg
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}
	fx.ms = ms
	return fx
}

func (fx *moveMarkerFixture) nameFor(id uuid.UUID) string {
	switch id {
	case fx.oldTarget.ID:
		return fx.oldTarget.DisplayName
	case fx.newTarget.ID:
		return fx.newTarget.DisplayName
	default:
		return fx.caster.DisplayName
	}
}

// Happy path: caster concentrating on Hex, old target downed (0 HP). Move
// strips the marker from the old target, stamps it on the new target, spends
// the bonus action, and reports both names.
func TestMoveHex_OldTargetDowned_MovesMarkerAndSpendsBonus(t *testing.T) {
	fx := newMoveMarkerFixture(t, hexConditionName, hexSpellID, 0, true)
	svc := NewService(fx.ms)

	res, err := svc.MoveHex(context.Background(), fx.caster, fx.turn, fx.newTarget.ID)
	require.NoError(t, err)

	// old target's hex marker stripped
	require.Contains(t, fx.condWrites, fx.oldTarget.ID)
	assert.NotContains(t, string(fx.condWrites[fx.oldTarget.ID]), hexSpellID, "old target should lose the hex marker")
	// new target stamped with a source-tagged marker
	require.Contains(t, fx.condWrites, fx.newTarget.ID)
	assert.True(t, targetMarkedBySpell(fx.condWrites[fx.newTarget.ID], fx.caster.ID, hexConditionName, hexSpellID),
		"new target must carry the caster's source-tagged hex marker")
	// bonus action spent
	require.NotNil(t, fx.turnWritten)
	assert.True(t, fx.turnWritten.BonusActionUsed)
	assert.True(t, res.Turn.BonusActionUsed)
	assert.Equal(t, "Goblin", res.OldTargetName)
	assert.Contains(t, res.CombatLog, "Ogre")
	assert.Contains(t, res.CombatLog, "Goblin")
}

// Not concentrating on Hex → rejected, no bonus action spent, no writes.
func TestMoveHex_NotConcentrating_Rejected(t *testing.T) {
	fx := newMoveMarkerFixture(t, hexConditionName, hexSpellID, 0, false)
	svc := NewService(fx.ms)

	_, err := svc.MoveHex(context.Background(), fx.caster, fx.turn, fx.newTarget.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not concentrating")
	assert.Nil(t, fx.turnWritten, "no bonus action should be spent on rejection")
	assert.Empty(t, fx.condWrites, "no condition writes on rejection")
}

// Current hex target still standing (>0 HP) → rejected per RAW (curse only
// moves off a target that dropped to 0 HP).
func TestMoveHex_TargetStillStanding_Rejected(t *testing.T) {
	fx := newMoveMarkerFixture(t, hexConditionName, hexSpellID, 7, true)
	svc := NewService(fx.ms)

	_, err := svc.MoveHex(context.Background(), fx.caster, fx.turn, fx.newTarget.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0 HP")
	assert.Nil(t, fx.turnWritten)
	assert.Empty(t, fx.condWrites)
}

// No bonus action available → rejected before any mutation.
func TestMoveHex_NoBonusAction_Rejected(t *testing.T) {
	fx := newMoveMarkerFixture(t, hexConditionName, hexSpellID, 0, true)
	fx.turn.BonusActionUsed = true
	svc := NewService(fx.ms)

	_, err := svc.MoveHex(context.Background(), fx.caster, fx.turn, fx.newTarget.ID)
	require.Error(t, err)
	assert.Nil(t, fx.turnWritten)
	assert.Empty(t, fx.condWrites)
}

// Moving onto yourself is rejected.
func TestMoveHex_OntoSelf_Rejected(t *testing.T) {
	fx := newMoveMarkerFixture(t, hexConditionName, hexSpellID, 0, true)
	svc := NewService(fx.ms)

	_, err := svc.MoveHex(context.Background(), fx.caster, fx.turn, fx.caster.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yourself")
	assert.Nil(t, fx.turnWritten)
}

// No existing marker anywhere (old target already left the encounter) → the
// move is still allowed and simply stamps the new target.
func TestMoveHex_NoExistingMarker_StampsNewTarget(t *testing.T) {
	fx := newMoveMarkerFixture(t, hexConditionName, hexSpellID, 0, true)
	// strip the marker from the old target so no marker exists in the encounter
	fx.oldTarget.Conditions = json.RawMessage(`[]`)
	svc := NewService(fx.ms)

	res, err := svc.MoveHex(context.Background(), fx.caster, fx.turn, fx.newTarget.ID)
	require.NoError(t, err)
	assert.Equal(t, "", res.OldTargetName)
	assert.True(t, targetMarkedBySpell(fx.condWrites[fx.newTarget.ID], fx.caster.ID, hexConditionName, hexSpellID))
	require.NotNil(t, fx.turnWritten)
	assert.True(t, fx.turnWritten.BonusActionUsed)
}

// Hunter's Mark parity: the same move works with the ranger's marker.
func TestMoveHuntersMark_OldTargetDowned_MovesMarker(t *testing.T) {
	fx := newMoveMarkerFixture(t, huntersMarkConditionName, huntersMarkSpellID, 0, true)
	svc := NewService(fx.ms)

	res, err := svc.MoveHuntersMark(context.Background(), fx.caster, fx.turn, fx.newTarget.ID)
	require.NoError(t, err)
	assert.True(t, targetMarkedBySpell(fx.condWrites[fx.newTarget.ID], fx.caster.ID, huntersMarkConditionName, huntersMarkSpellID))
	assert.Contains(t, res.CombatLog, "Hunter's Mark")
	require.NotNil(t, fx.turnWritten)
	assert.True(t, fx.turnWritten.BonusActionUsed)
}
