package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
)

// buildDiscordHandlersForWiring is a small constructor used by the wiring
// follow-up tests. It boots a real Postgres-backed refdata.Queries (via
// testutil) so handler construction takes the full production path, then
// returns the constructed handler set. The session is the lightweight
// testSession (defined in discord_handlers_test.go) — no Discord gateway
// is opened.
func buildDiscordHandlersForWiring(t *testing.T) discordHandlers {
	t.Helper()
	db := testutil.NewTestDB(t)
	require.NoError(t, database.MigrateUp(db, dbfs.Migrations))
	queries := refdata.New(db)
	combatSvc := combat.NewService(combat.NewStoreAdapter(queries))

	return buildDiscordHandlers(discordHandlerDeps{
		session:       &testSession{},
		queries:       queries,
		combatService: combatSvc,
		roller:        dice.NewRoller(nil),
		resolver:      &stubUserEncounterResolver{},
	})
}

// TestBuildDiscordHandlers_AttackHandlerHasMapProvider verifies the
// C-DISCORD follow-up: /attack must have a map provider wired so
// AttackCommand.Walls is populated and wall-based cover applies on hits.
func TestBuildDiscordHandlers_AttackHandlerHasMapProvider(t *testing.T) {
	set := buildDiscordHandlersForWiring(t)
	require.NotNil(t, set.attack)
	assert.True(t, set.attack.HasMapProvider(),
		"attack handler must have a non-nil AttackMapProvider wired in production")
}

// TestBuildDiscordHandlers_AttackHandlerHasClassFeaturePromptPoster verifies
// the D-48b/49/51 follow-up: /attack must have the post-hit prompt poster
// wired so Stunning Strike / Divine Smite / Bardic Inspiration prompts
// reach the player after a successful hit.
func TestBuildDiscordHandlers_AttackHandlerHasClassFeaturePromptPoster(t *testing.T) {
	set := buildDiscordHandlersForWiring(t)
	require.NotNil(t, set.attack)
	assert.True(t, set.attack.HasClassFeaturePromptPoster(),
		"attack handler must have a ClassFeaturePromptPoster wired in production")
}

// TestBuildDiscordHandlers_ActionHandlerHasStabilizeStore verifies the
// C-DISCORD follow-up: /action stabilize must have a death-save store
// wired so a successful Medicine check persists DeathSaves{Successes: 3}
// instead of reporting "not available".
func TestBuildDiscordHandlers_ActionHandlerHasStabilizeStore(t *testing.T) {
	set := buildDiscordHandlersForWiring(t)
	require.NotNil(t, set.action)
	assert.True(t, set.action.HasStabilizeStore(),
		"action handler must have an ActionStabilizeStore wired in production")
}

// TestBuildDiscordHandlers_CastHandlerHasMaterialPromptStore verifies the
// AOE-CAST follow-up: /cast must have a ReactionPromptStore wired so the
// gold-fallback Buy & Cast / Cancel prompt fires interactively rather than
// degrading to a plain ephemeral.
func TestBuildDiscordHandlers_CastHandlerHasMaterialPromptStore(t *testing.T) {
	set := buildDiscordHandlersForWiring(t)
	require.NotNil(t, set.cast)
	assert.True(t, set.cast.HasMaterialPromptStore(),
		"cast handler must have a *ReactionPromptStore wired in production")
}

// TestBuildDiscordHandlers_SaveHandlerHasAoESaveResolver verifies the
// AOE-CAST follow-up: /save must have an AoESaveResolver wired so per-
// player AoE saves resolve into the damage-application pipeline.
func TestBuildDiscordHandlers_SaveHandlerHasAoESaveResolver(t *testing.T) {
	set := buildDiscordHandlersForWiring(t)
	require.NotNil(t, set.save)
	assert.True(t, set.save.HasAoESaveResolver(),
		"save handler must have an AoESaveResolver wired in production")
}

// TestBuildDiscordHandlers_CheckHandlerHasZoneLookup verifies the
// COMBAT-MISC-followup: /check must have a CheckZoneLookup wired so
// obscurement-driven disadvantage on Perception fires inside heavily
// obscured zones (E-69 gating).
func TestBuildDiscordHandlers_CheckHandlerHasZoneLookup(t *testing.T) {
	set := buildDiscordHandlersForWiring(t)
	require.NotNil(t, set.check)
	assert.True(t, set.check.HasZoneLookup(),
		"check handler must have a CheckZoneLookup wired in production")
}

// TestBuildDiscordHandlers_ActionHandlerHasZoneLookup verifies the
// COMBAT-MISC-followup: /action must have an ActionZoneLookup wired so
// /action hide gates on obscurement (E-69 hiding rules).
func TestBuildDiscordHandlers_ActionHandlerHasZoneLookup(t *testing.T) {
	set := buildDiscordHandlersForWiring(t)
	require.NotNil(t, set.action)
	assert.True(t, set.action.HasZoneLookup(),
		"action handler must have an ActionZoneLookup wired in production")
}

// TestBuildDiscordHandlers_ActionHandlerHasSpeedLookup verifies the
// D-54-followup: /action stand must have a walk-speed lookup wired so
// halflings (25ft) and tabaxi (35ft) pay the half-movement-to-stand cost
// from their actual speed (12-13ft or 17-18ft), not the hardcoded 15ft.
func TestBuildDiscordHandlers_ActionHandlerHasSpeedLookup(t *testing.T) {
	set := buildDiscordHandlersForWiring(t)
	require.NotNil(t, set.action)
	assert.True(t, set.action.HasSpeedLookup(),
		"action handler must have an ActionSpeedLookup wired in production")
}

// TestBuildDiscordHandlers_ActionHandlerHasMedicineLookup verifies the
// C-43-stabilize-followup: /action stabilize must have a Medicine
// modifier lookup wired so the roll is d20 + WIS + proficiency instead
// of the historical flat d20.
func TestBuildDiscordHandlers_ActionHandlerHasMedicineLookup(t *testing.T) {
	set := buildDiscordHandlersForWiring(t)
	require.NotNil(t, set.action)
	assert.True(t, set.action.HasMedicineLookup(),
		"action handler must have an ActionMedicineLookup wired in production")
}
