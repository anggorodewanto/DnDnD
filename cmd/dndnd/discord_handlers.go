package main

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/refdata"
)

// userEncounterResolver is the shared per-user encounter-lookup interface
// injected into every Phase 105 slash-command handler. A single concrete
// implementation (discordUserEncounterResolver) satisfies all per-handler
// provider interfaces structurally.
type userEncounterResolver interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

// discordHandlerDeps bundles the collaborators needed to construct every
// Phase 105 slash-command handler in one place. Optional fields may be nil —
// constructors tolerate it and Set* wiring is applied later in run().
type discordHandlerDeps struct {
	session                  discord.Session
	queries                  *refdata.Queries
	db                       combat.TxBeginner // Phase 27 turn-gate; nil disables /move /fly ownership checks
	combatService            *combat.Service
	roller                   *dice.Roller
	resolver                 userEncounterResolver
	campaignSettings         discord.CampaignSettingsProvider
	enemyTurnEncounterLookup discord.EnemyTurnEncounterLookup
	mapRegenerator           discord.MapRegenerator
	lootService              *loot.Service
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
	reaction          *discord.ReactionHandler
	use               *discord.UseHandler
	status            *discord.StatusHandler
	whisper           *discord.WhisperHandler
	action            *discord.ActionHandler
	loot              *discord.LootHandler
	enemyTurnNotifier *discord.DiscordEnemyTurnNotifier
}

// buildDiscordHandlers constructs every Phase 105 handler with the shared
// user-aware encounter resolver injected. Optional collaborators (turn
// advancer, DM notifiers, map regenerator, character lookups) are either
// attached via Set* helpers or left nil — each handler is defensive about nil.
//
// The returned enemyTurnNotifier already has SetEncounterLookup called so
// NotifyEnemyTurnExecuted produces the Phase 105 "⚔️ <display_name> — Round N"
// label in production instead of an empty fallback.
func buildDiscordHandlers(deps discordHandlerDeps) discordHandlers {
	// Keep the `if deps.queries != nil` guard: assigning a typed-nil
	// *refdata.Queries into these interface variables would produce a
	// non-nil interface holding a nil pointer, which handlers cannot detect.
	var (
		moveSvc     discord.MoveService
		turnSvc     discord.MoveTurnProvider
		mapSvc      discord.MoveMapProvider
		campaignSvc discord.CampaignProvider
	)
	if deps.queries != nil {
		moveSvc = newMoveServiceAdapter(deps.queries)
		mapSvc = newMapProviderAdapter(deps.queries)
		turnSvc = deps.queries
		campaignSvc = deps.queries
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

	var useStore discord.UseCharacterStore
	if deps.queries != nil {
		useStore = deps.queries
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
		summon:   discord.NewSummonCommandHandler(deps.session, summonSvc),
		recap:    discord.NewRecapHandler(deps.session, recapSvc, deps.resolver, newRecapPlayerLookupAdapter(combatantLookup)),
		reaction: discord.NewReactionHandler(deps.session, newReactionServiceAdapter(deps.combatService), deps.resolver, combatantLookup),
		use:      discord.NewUseHandler(deps.session, checkCampProv, characterLookup, useStore, nil, nil),
		status: discord.NewStatusHandler(
			deps.session,
			checkCampProv,
			characterLookup,
			deps.resolver,
			combatantLookup,
			newConcentrationLookupAdapter(deps.queries),
			newReactionLookupAdapter(deps.queries),
		),
		whisper: discord.NewWhisperHandler(deps.session, checkCampProv, characterLookup),
		action: discord.NewActionHandler(
			deps.session,
			deps.resolver,
			newActionCombatServiceAdapter(deps.combatService),
			turnSvc,
			checkCampProv,
			characterLookup,
			newActionPendingStoreAdapter(deps.queries),
		),
		enemyTurnNotifier: discord.NewDiscordEnemyTurnNotifier(deps.session, deps.campaignSettings, deps.mapRegenerator),
	}

	// Phase 120a: wire /loot. Requires the loot service plus a
	// LootEncounterProvider — *refdata.Queries already exposes
	// GetMostRecentCompletedEncounter so it satisfies the interface
	// structurally. Skip when either dependency is nil so handler
	// construction is safe in test deploys. checkCampProv and
	// characterLookup are derived from deps.queries above, so the
	// queries check covers them too.
	if deps.lootService != nil && deps.queries != nil {
		handlers.loot = discord.NewLootHandler(
			deps.session,
			checkCampProv,
			characterLookup,
			deps.queries,
			deps.lootService,
		)
	}

	// Setter-based wiring for handlers that don't accept these via constructor.
	handlers.summon.SetEncounterProvider(deps.resolver)
	// Phase 110 it3: wire the character lookup so exploration /move can
	// disambiguate which PC combatant belongs to the invoking Discord user
	// (resolveExplorationMover falls back to pcs[0] when this is nil).
	if characterLookup != nil {
		handlers.move.SetCharacterLookup(characterLookup)
	}
	if deps.enemyTurnEncounterLookup != nil {
		handlers.enemyTurnNotifier.SetEncounterLookup(deps.enemyTurnEncounterLookup)
	}

	// Phase 27 turn-gate: wire the advisory-lock + ownership-validation
	// gate into the state-mutating /move and /fly handlers. /distance is
	// intentionally skipped (combat.IsExemptCommand("distance") is true);
	// SetTurnGate on the distance handler is a no-op stored field today.
	// Both deps.db and deps.queries must be present — nil-safe so test
	// deploys without a database can still construct handlers.
	if deps.db != nil && deps.queries != nil {
		gate := newTurnGateAdapter(deps.db, deps.queries)
		handlers.move.SetTurnGate(gate)
		handlers.fly.SetTurnGate(gate)
		// /distance is exempt; setter is wired anyway so the production
		// graph is symmetric with /move and /fly.
		handlers.distance.SetTurnGate(gate)
	}

	return handlers
}

// attachPhase105Handlers registers every Phase 105 slash-command handler from
// the given set on the router. Kept as a standalone helper so main.go doesn't
// grow a long setter chain each time a new handler is wired.
func attachPhase105Handlers(r *discord.CommandRouter, set discordHandlers) {
	r.SetMoveHandler(set.move)
	r.SetFlyHandler(set.fly)
	r.SetDistanceHandler(set.distance)
	r.SetDoneHandler(set.done)
	r.SetCheckHandler(set.check)
	r.SetSaveHandler(set.save)
	r.SetRestHandler(set.rest)
	r.SetSummonCommandHandler(set.summon)
	r.SetRecapHandler(set.recap)
	r.SetUseHandler(set.use)
	r.SetReactionHandler(set.reaction)
	r.SetStatusHandler(set.status)
	r.SetWhisperHandler(set.whisper)
	r.SetActionHandler(set.action)
	if set.loot != nil {
		r.SetLootHandler(set.loot)
	}
}

// --- Thin adapters bridging refdata.Queries / combat.Service to the handler
// interfaces. Each new* constructor returns nil when its backing queries /
// service are nil so buildDiscordHandlers can skip typed-nil interface traps. ---

// moveServiceAdapter satisfies discord.MoveService over *refdata.Queries,
// unpacking the positional UpdateCombatantPosition signature into the
// sqlc-generated params struct.
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

// mapProviderAdapter satisfies discord.MoveMapProvider. Refdata exposes the
// lookup as GetMapByID; we rename it to the GetByID shape the interface wants.
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
// specific encounter by chaining player_character and combatants, and also
// satisfies CheckCombatantLookup via ListCombatantsByEncounterID. One struct
// covers both /check and /summon lookups.
type combatantByDiscordAdapter struct {
	queries *refdata.Queries
}

func newCombatantByDiscordAdapter(q *refdata.Queries) *combatantByDiscordAdapter {
	if q == nil {
		return nil
	}
	return &combatantByDiscordAdapter{queries: q}
}

func (a *combatantByDiscordAdapter) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return a.queries.ListCombatantsByEncounterID(ctx, encounterID)
}

// GetCombatantIDByDiscordUser returns (combatantID, displayName, error) for
// the SummonCommandPlayerLookup contract.
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
// interface, unpacking the positional GetLastCompletedTurnByCombatant call
// into the sqlc params struct.
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

// newReactionServiceAdapter returns the combat.Service as a
// discord.ReactionService, or nil if combatService is nil so the handler
// constructor can short-circuit safely. combat.Service structurally
// satisfies the interface (CanDeclareReaction, DeclareReaction,
// CancelReactionByDescription, CancelAllReactions, ListReactionsByCombatant).
func newReactionServiceAdapter(svc *combat.Service) discord.ReactionService {
	if svc == nil {
		return nil
	}
	return svc
}

// restCharUpdaterAdapter satisfies RestCharacterUpdater over *refdata.Queries.
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
// *refdata.Queries. Separate from CampaignProvider only so the constructor
// can return a nil *struct (not a typed-nil interface) when queries is nil.
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

// concentrationLookupAdapter satisfies discord.StatusConcentrationLookup over *refdata.Queries.
type concentrationLookupAdapter struct {
	queries *refdata.Queries
}

func newConcentrationLookupAdapter(q *refdata.Queries) *concentrationLookupAdapter {
	if q == nil {
		return nil
	}
	return &concentrationLookupAdapter{queries: q}
}

func (a *concentrationLookupAdapter) ListConcentrationZonesByCombatant(ctx context.Context, sourceCombatantID uuid.UUID) ([]refdata.EncounterZone, error) {
	return a.queries.ListConcentrationZonesByCombatant(ctx, sourceCombatantID)
}

// reactionLookupAdapter satisfies discord.StatusReactionLookup over *refdata.Queries.
type reactionLookupAdapter struct {
	queries *refdata.Queries
}

func newReactionLookupAdapter(q *refdata.Queries) *reactionLookupAdapter {
	if q == nil {
		return nil
	}
	return &reactionLookupAdapter{queries: q}
}

func (a *reactionLookupAdapter) ListActiveReactionDeclarationsByCombatant(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
	return a.queries.ListActiveReactionDeclarationsByCombatant(ctx, arg)
}

// sqlNoRowsLike returns a sentinel error for "combatant not found" lookups
// that didn't hit a driver-level ErrNoRows but semantically represent a miss.
// Handlers treat any error as "not in combat / not registered" anyway.
func sqlNoRowsLike() error {
	return &combatantNotFoundError{}
}

type combatantNotFoundError struct{}

func (e *combatantNotFoundError) Error() string { return "combatant not found for discord user" }

// actionCombatServiceAdapter exposes the narrow slice of *combat.Service that
// the /action handler needs (freeform post/cancel + a small set of lookups).
// Returning a dedicated adapter avoids a typed-nil interface trap when the
// combat service is absent in test deploys.
type actionCombatServiceAdapter struct {
	svc *combat.Service
}

func newActionCombatServiceAdapter(svc *combat.Service) discord.ActionCombatService {
	if svc == nil {
		return nil
	}
	return &actionCombatServiceAdapter{svc: svc}
}

func (a *actionCombatServiceAdapter) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return a.svc.GetEncounter(ctx, id)
}

func (a *actionCombatServiceAdapter) GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return a.svc.GetCombatant(ctx, id)
}

func (a *actionCombatServiceAdapter) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return a.svc.ListCombatantsByEncounterID(ctx, encounterID)
}

func (a *actionCombatServiceAdapter) FreeformAction(ctx context.Context, cmd combat.FreeformActionCommand) (combat.FreeformActionResult, error) {
	return a.svc.FreeformAction(ctx, cmd)
}

func (a *actionCombatServiceAdapter) CancelFreeformAction(ctx context.Context, cmd combat.CancelFreeformActionCommand) (combat.CancelFreeformActionResult, error) {
	return a.svc.CancelFreeformAction(ctx, cmd)
}

func (a *actionCombatServiceAdapter) CancelExplorationFreeformAction(ctx context.Context, combatantID uuid.UUID) (combat.CancelFreeformActionResult, error) {
	return a.svc.CancelExplorationFreeformAction(ctx, combatantID)
}

// actionPendingStoreAdapter satisfies discord.ActionPendingStore over
// *refdata.Queries. Used by the exploration /action path, which must persist
// a pending_actions row without going through combat.Service (no Turn).
type actionPendingStoreAdapter struct {
	queries *refdata.Queries
}

func newActionPendingStoreAdapter(q *refdata.Queries) discord.ActionPendingStore {
	if q == nil {
		return nil
	}
	return &actionPendingStoreAdapter{queries: q}
}

func (a *actionPendingStoreAdapter) CreatePendingAction(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
	return a.queries.CreatePendingAction(ctx, arg)
}

// turnGateAdapter satisfies discord.TurnGate by delegating to
// combat.AcquireTurnLockWithValidation. After validation succeeds we commit
// the held tx immediately to release the advisory lock — this is intentional:
// today's /move and /fly handlers do their persistence outside the lock-held
// tx, so the gate's job is to fire the wrong-owner check (and serialize peers
// at the validation point). A future patch can extend the adapter to thread
// the tx through the persistence layer for true serialized writes.
type turnGateAdapter struct {
	db      combat.TxBeginner
	queries *refdata.Queries
}

func newTurnGateAdapter(db combat.TxBeginner, queries *refdata.Queries) *turnGateAdapter {
	return &turnGateAdapter{db: db, queries: queries}
}

// AcquireAndRelease validates ownership, acquires the per-turn advisory
// lock, and commits the tx so the lock releases. Errors propagate verbatim
// so the discord handler can branch on combat.ErrNotYourTurn / ErrLockTimeout.
func (a *turnGateAdapter) AcquireAndRelease(ctx context.Context, encounterID uuid.UUID, discordUserID string) (combat.TurnOwnerInfo, error) {
	tx, info, err := combat.AcquireTurnLockWithValidation(ctx, a.db, a.queries, encounterID, discordUserID)
	if err != nil {
		return combat.TurnOwnerInfo{}, err
	}
	if commitErr := tx.Commit(); commitErr != nil {
		return combat.TurnOwnerInfo{}, commitErr
	}
	return info, nil
}
