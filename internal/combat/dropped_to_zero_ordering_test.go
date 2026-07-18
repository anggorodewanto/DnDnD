package combat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// FormatAttackLog must fold the drop-to-0 line into the SAME message as the
// hit/damage narrative, at the tail — so #combat-log shows the "defeated" line
// AFTER the attack that caused it, never before. The line is carried on the
// result (DownLogLine); the service defers its own immediate post for attack
// paths (see TestApplyHitDamage_Defer_* below).
func TestFormatAttackLog_AppendsDownLogLineAfterDamage(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria", TargetName: "Ghoul", WeaponName: "scimitar",
		IsMelee: true, DistanceFt: 5,
		Hit: true, DamageTotal: 12, DamageType: "slashing", DamageDice: "1d6+3",
		DownLogLine: "\U0001f480  Ghoul drops to 0 HP — defeated",
	}
	out := FormatAttackLog(result)
	require.Contains(t, out, "defeated")
	assert.Greater(t, strings.Index(out, "defeated"), strings.Index(out, "Damage:"),
		"drop-to-0 line must come after the damage line")
}

// A hit that leaves the target standing carries no DownLogLine, so the narrative
// shows no drop-to-0 tail.
func TestFormatAttackLog_NoDownLogLine_Omitted(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria", TargetName: "Ghoul", WeaponName: "scimitar",
		IsMelee: true, DistanceFt: 5,
		Hit: true, DamageTotal: 4, DamageType: "slashing", DamageDice: "1d6+1",
	}
	assert.NotContains(t, FormatAttackLog(result), "drops to 0")
}

// ApplyDamage with DeferDownLog skips its own #combat-log post (the attack
// handler posts the line as the tail of FormatAttackLog instead) but still
// records the DM-console downed row and returns the formatted line for the
// caller to carry.
func TestApplyDamage_DeferDownLog_SkipsPost_ReturnsLine(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	ms, _ := applyDamageMockStore()
	validTurnEncounter(ms, turnID)
	logged := captureActionLog(ms)
	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	target := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID, DisplayName: "Ghoul",
		HpMax: 22, HpCurrent: 6, IsAlive: true, IsNpc: true,
		Conditions: json.RawMessage(`[]`),
	}
	out, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 30, DamageType: "slashing",
		DeferDownLog: true,
	})
	require.NoError(t, err)

	assert.Empty(t, cl.all(), "deferred: ApplyDamage must NOT post the down-line itself")
	assert.Contains(t, out.DownLogLine, "defeated", "the formatted line is returned for the caller to post")
	require.Len(t, downedRows(*logged), 1, "DM-console downed row is still recorded")
}

// applyHitDamage is the shared attack-damage chokepoint. With defer=true it
// suppresses the immediate #combat-log down-post and stamps the line onto the
// caller's AttackResult so FormatAttackLog renders it after the hit.
func TestApplyHitDamage_Defer_CarriesDownLineNoPost(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	ms, _ := applyDamageMockStore()
	validTurnEncounter(ms, turnID)
	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Ghoul",
		HpMax: 22, HpCurrent: 6, IsAlive: true, IsNpc: true,
		Conditions: json.RawMessage(`[]`),
	}
	result := AttackResult{Hit: true, DamageTotal: 30, DamageType: "slashing"}
	_, _, err := svc.applyHitDamage(context.Background(), encounterID, target, &result, true)
	require.NoError(t, err)

	assert.Empty(t, cl.all(), "defer: no immediate down-post from the service")
	assert.Contains(t, result.DownLogLine, "defeated", "down-line stamped onto the result")
}

// Flurry of Blows keeps the immediate post (defer=false): its aggregated log
// never runs through FormatAttackLog, so the service must still surface the kill.
func TestApplyHitDamage_NoDefer_PostsImmediately(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	ms, _ := applyDamageMockStore()
	validTurnEncounter(ms, turnID)
	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Ghoul",
		HpMax: 22, HpCurrent: 6, IsAlive: true, IsNpc: true,
		Conditions: json.RawMessage(`[]`),
	}
	result := AttackResult{Hit: true, DamageTotal: 30, DamageType: "slashing"}
	_, _, err := svc.applyHitDamage(context.Background(), encounterID, target, &result, false)
	require.NoError(t, err)

	posts := cl.all()
	require.Len(t, posts, 1, "no-defer: service posts the down-line immediately")
	assert.Contains(t, posts[0].content, "defeated")
	assert.Empty(t, result.DownLogLine, "no-defer path leaves DownLogLine empty (immediate post owns it)")
}
