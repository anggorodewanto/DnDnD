package main

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/refdata"
)

// userEncounterResolver is the interface the Phase 105 handlers share for
// resolving the active encounter a Discord user is currently a combatant in.
// A single concrete implementation (discordUserEncounterResolver) satisfies
// all per-handler provider interfaces structurally.
type userEncounterResolver interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

// discordHandlerDeps bundles the collaborators needed to construct every
// Phase 105 slash-command handler in one place. Fields that are optional in
// any given handler can be left nil — the constructors tolerate it and the
// matching Set* wiring is applied later in run().
type discordHandlerDeps struct {
	session                  discord.Session
	queries                  *refdata.Queries
	combatService            *combat.Service
	roller                   *dice.Roller
	resolver                 userEncounterResolver
	campaignSettings         discord.CampaignSettingsProvider
	enemyTurnEncounterLookup discord.EnemyTurnEncounterLookup
	mapRegenerator           discord.MapRegenerator
}

// discordHandlers holds the constructed slash-command handlers so main.go can
// register them with a CommandRouter and so tests can assert every handler is
// non-nil without tampering with router internals.
type discordHandlers struct {
	move              *discord.MoveHandler
	fly               *discord.FlyHandler
	distance          *discord.DistanceHandler
	done              *discord.DoneHandler
	check             *discord.CheckHandler
	save              *discord.SaveHandler
	rest              *discord.RestHandler
	summon            *discord.SummonCommandHandler
	recap             *discord.RecapHandler
	enemyTurnNotifier *discord.DiscordEnemyTurnNotifier
}

// buildDiscordHandlers constructs every Phase 105 handler with the shared
// user-aware encounter resolver injected. Optional collaborators that depend
// on adapters not yet wired in main.go (turn advancer, DM notifiers,
// map regenerator, character lookups) are attached via Set* helpers or left
// nil — each handler is defensive about nil.
//
// The returned enemyTurnNotifier already has SetEncounterLookup called so
// NotifyEnemyTurnExecuted produces the Phase 105 "⚔️ <display_name> — Round N"
// label in production instead of an empty fallback.
func buildDiscordHandlers(deps discordHandlerDeps) discordHandlers {
	var (
		moveSvc     discord.MoveService
		turnSvc     discord.MoveTurnProvider
		mapSvc      discord.MoveMapProvider
		campaignSvc discord.CampaignProvider
	)
	if deps.queries != nil {
		turnSvc = deps.queries
		mapSvc = newMapProviderAdapter(deps.queries)
		campaignSvc = deps.queries
		moveSvc = newMoveServiceAdapter(deps.queries)
	}

	characterLookup := newCharacterByDiscordAdapter(deps.queries)
	combatantLookup := newCombatantByDiscordAdapter(deps.queries)
	recapSvc := newRecapServiceAdapter(deps.queries, deps.combatService)
	restCharUpdater := newRestCharUpdaterAdapter(deps.queries)
	checkCampProv := newCheckCampaignProviderAdapter(deps.queries)

	var summonSvc discord.SummonCommandService
	if deps.combatService != nil {
		summonSvc = deps.combatService
	}

	handlers := discordHandlers{
		move:     discord.NewMoveHandler(deps.session, moveSvc, mapSvc, turnSvc, deps.resolver, campaignSvc),
		fly:      discord.NewFlyHandler(deps.session, moveSvc, turnSvc, deps.resolver),
		distance: discord.NewDistanceHandler(deps.session, moveSvc, turnSvc, deps.resolver),
		done:     discord.NewDoneHandler(deps.session, moveSvc, turnSvc, deps.resolver),
		check: discord.NewCheckHandler(
			deps.session,
			deps.roller,
			checkCampProv,
			characterLookup,
			deps.resolver,
			combatantLookup,
			nil, // rollLogger: no production adapter yet (tests only).
		),
		save: discord.NewSaveHandler(
			deps.session,
			deps.roller,
			checkCampProv,
			characterLookup,
			deps.resolver,
			combatantLookup,
			nil,
		),
		rest: discord.NewRestHandler(
			deps.session,
			deps.roller,
			checkCampProv,
			characterLookup,
			deps.resolver,
			restCharUpdater,
			nil,
			nil,
		),
		summon: discord.NewSummonCommandHandler(deps.session, summonSvc),
		recap:  discord.NewRecapHandler(deps.session, recapSvc, deps.resolver, newRecapPlayerLookupAdapter(combatantLookup)),
		enemyTurnNotifier: discord.NewDiscordEnemyTurnNotifier(
			deps.session,
			deps.campaignSettings,
			deps.mapRegenerator,
		),
	}

	// Phase 105b: inject the per-user encounter provider into handlers that
	// use setter-based wiring rather than constructor injection.
	handlers.summon.SetEncounterProvider(deps.resolver)

	// Phase 105b: wire the encounter lookup so NotifyEnemyTurnExecuted
	// produces the "⚔️ <display_name> — Round N" label in production instead
	// of the empty fallback left by Phase 105.
	if deps.enemyTurnEncounterLookup != nil {
		handlers.enemyTurnNotifier.SetEncounterLookup(deps.enemyTurnEncounterLookup)
	}

	return handlers
}

// --- Thin adapters bridging refdata.Queries / combat.Service to the handler
// interfaces ---

// moveServiceAdapter satisfies discord.MoveService over *refdata.Queries.
// The sqlc-generated UpdateCombatantPosition takes a params struct while the
// handler interface takes positional args, so we unpack here. Same story for
// UpdateCombatantConditions.
type moveServiceAdapter struct {
	queries *refdata.Queries
}

func newMoveServiceAdapter(q *refdata.Queries) *moveServiceAdapter {
	if q == nil {
		return nil
	}
	return &moveServiceAdapter{queries: q}
}

func (a *moveServiceAdapter) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return a.queries.GetEncounter(ctx, id)
}

func (a *moveServiceAdapter) GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return a.queries.GetCombatant(ctx, id)
}

func (a *moveServiceAdapter) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return a.queries.ListCombatantsByEncounterID(ctx, encounterID)
}

func (a *moveServiceAdapter) UpdateCombatantPosition(ctx context.Context, id uuid.UUID, col string, row, altitude int32) (refdata.Combatant, error) {
	return a.queries.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
		ID:          id,
		PositionCol: col,
		PositionRow: row,
		AltitudeFt:  altitude,
	})
}

func (a *moveServiceAdapter) UpdateCombatantConditions(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
	return a.queries.UpdateCombatantConditions(ctx, arg)
}

// mapProviderAdapter satisfies discord.MoveMapProvider. refdata.Queries
// doesn't expose a GetByID(uuid) map lookup directly; we delegate to
// GetMap which returns a refdata.Map for a given ID.
type mapProviderAdapter struct {
	queries *refdata.Queries
}

func newMapProviderAdapter(q *refdata.Queries) *mapProviderAdapter {
	if q == nil {
		return nil
	}
	return &mapProviderAdapter{queries: q}
}

func (a *mapProviderAdapter) GetByID(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
	return a.queries.GetMapByID(ctx, id)
}


// characterByDiscordAdapter chains GetPlayerCharacterByDiscordUser with
// GetCharacter so handlers can look up a character by (campaignID, discordUserID)
// without a dedicated sqlc query.
type characterByDiscordAdapter struct {
	queries *refdata.Queries
}

func newCharacterByDiscordAdapter(q *refdata.Queries) *characterByDiscordAdapter {
	if q == nil {
		return nil
	}
	return &characterByDiscordAdapter{queries: q}
}

func (a *characterByDiscordAdapter) GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error) {
	pc, err := a.queries.GetPlayerCharacterByDiscordUser(ctx, refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    campaignID,
		DiscordUserID: discordUserID,
	})
	if err != nil {
		return refdata.Character{}, err
	}
	return a.queries.GetCharacter(ctx, pc.CharacterID)
}

// combatantByDiscordAdapter resolves a Discord user to their combatant in a
// specific encounter by chaining the player_character and combatants tables.
type combatantByDiscordAdapter struct {
	queries *refdata.Queries
}

func newCombatantByDiscordAdapter(q *refdata.Queries) *combatantByDiscordAdapter {
	if q == nil {
		return nil
	}
	return &combatantByDiscordAdapter{queries: q}
}

// GetCombatantIDByDiscordUser returns (combatantID, displayName, error) for
// the SummonCommandPlayerLookup contract.
// ListCombatantsByEncounterID satisfies CheckCombatantLookup by delegating to
// the underlying queries. We attach it to this adapter so the same struct
// can cover both /check and /summon lookups in Phase 105b wiring.
func (a *combatantByDiscordAdapter) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return a.queries.ListCombatantsByEncounterID(ctx, encounterID)
}

func (a *combatantByDiscordAdapter) GetCombatantIDByDiscordUser(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, string, error) {
	enc, err := a.queries.GetEncounter(ctx, encounterID)
	if err != nil {
		return uuid.Nil, "", err
	}
	pc, err := a.queries.GetPlayerCharacterByDiscordUser(ctx, refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    enc.CampaignID,
		DiscordUserID: discordUserID,
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	combatants, err := a.queries.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return uuid.Nil, "", err
	}
	for _, c := range combatants {
		if c.CharacterID.Valid && c.CharacterID.UUID == pc.CharacterID {
			return c.ID, c.DisplayName, nil
		}
	}
	return uuid.Nil, "", sqlNoRowsLike()
}

// recapPlayerLookupAdapter wraps combatantByDiscordAdapter to satisfy the
// RecapPlayerLookup interface, which returns (combatantID, error) without
// the display name.
type recapPlayerLookupAdapter struct {
	inner *combatantByDiscordAdapter
}

func newRecapPlayerLookupAdapter(inner *combatantByDiscordAdapter) *recapPlayerLookupAdapter {
	if inner == nil {
		return nil
	}
	return &recapPlayerLookupAdapter{inner: inner}
}

func (a *recapPlayerLookupAdapter) GetCombatantIDByDiscordUser(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, error) {
	id, _, err := a.inner.GetCombatantIDByDiscordUser(ctx, encounterID, discordUserID)
	return id, err
}

// recapServiceAdapter bridges refdata + combat.Service to the RecapService
// interface. The sqlc-generated GetLastCompletedTurnByCombatant takes a
// params struct, so we adapt to the positional (encounterID, combatantID)
// signature the handler expects.
type recapServiceAdapter struct {
	queries *refdata.Queries
	combat  *combat.Service
}

func newRecapServiceAdapter(q *refdata.Queries, svc *combat.Service) *recapServiceAdapter {
	if q == nil || svc == nil {
		return nil
	}
	return &recapServiceAdapter{queries: q, combat: svc}
}

func (a *recapServiceAdapter) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return a.combat.GetEncounter(ctx, id)
}

func (a *recapServiceAdapter) ListActionLogWithRounds(ctx context.Context, encounterID uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
	return a.queries.ListActionLogWithRounds(ctx, encounterID)
}

func (a *recapServiceAdapter) GetMostRecentCompletedEncounter(ctx context.Context, campaignID uuid.UUID) (refdata.Encounter, error) {
	return a.queries.GetMostRecentCompletedEncounter(ctx, campaignID)
}

func (a *recapServiceAdapter) GetLastCompletedTurnByCombatant(ctx context.Context, encounterID, combatantID uuid.UUID) (refdata.Turn, error) {
	return a.queries.GetLastCompletedTurnByCombatant(ctx, refdata.GetLastCompletedTurnByCombatantParams{
		EncounterID: encounterID,
		CombatantID: combatantID,
	})
}

// restCharUpdaterAdapter satisfies RestCharacterUpdater by delegating to
// *refdata.Queries.UpdateCharacter.
type restCharUpdaterAdapter struct {
	queries *refdata.Queries
}

func newRestCharUpdaterAdapter(q *refdata.Queries) *restCharUpdaterAdapter {
	if q == nil {
		return nil
	}
	return &restCharUpdaterAdapter{queries: q}
}

func (a *restCharUpdaterAdapter) UpdateCharacter(ctx context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
	return a.queries.UpdateCharacter(ctx, arg)
}

// checkCampaignProviderAdapter implements CheckCampaignProvider over
// *refdata.Queries. Separate from CampaignProvider only so we can return nil
// when queries is nil, preventing typed-nil interface traps in constructors.
type checkCampaignProviderAdapter struct {
	queries *refdata.Queries
}

func newCheckCampaignProviderAdapter(q *refdata.Queries) *checkCampaignProviderAdapter {
	if q == nil {
		return nil
	}
	return &checkCampaignProviderAdapter{queries: q}
}

func (a *checkCampaignProviderAdapter) GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	return a.queries.GetCampaignByGuildID(ctx, guildID)
}

// sqlNoRowsLike returns a sentinel error for "combatant not found" lookups
// that didn't hit a driver-level ErrNoRows but semantically represent a miss.
// We construct a fresh error so callers can use errors.Is sparingly — the
// handlers treat any error as "not in combat / not registered" anyway.
func sqlNoRowsLike() error {
	return &combatantNotFoundError{}
}

type combatantNotFoundError struct{}

func (e *combatantNotFoundError) Error() string { return "combatant not found for discord user" }
