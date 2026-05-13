package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/levelup"
	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// userEncounterResolver is the shared per-user encounter-lookup interface
// injected into every Phase 105 slash-command handler. A single concrete
// implementation (discordUserEncounterResolver) satisfies all per-handler
// provider interfaces structurally.
type userEncounterResolver interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

// enemyTurnNotifierSetter narrows combat.Handler to the SetEnemyTurnNotifier
// hook so cmd/dndnd can write a focused wiring test for H-105b without
// touching combat package internals.
type enemyTurnNotifierSetter interface {
	SetEnemyTurnNotifier(n combat.EnemyTurnNotifier)
}

// wireEnemyTurnNotifier injects the Discord-backed enemy-turn notifier into
// the combat HTTP handler so combat.Handler.ExecuteEnemyTurn posts the
// "⚔️ <display_name> — Round N" label instead of silently no-oping (H-105b).
// Nil-safe so test deploys without a session or notifier do not panic.
func wireEnemyTurnNotifier(h enemyTurnNotifierSetter, n combat.EnemyTurnNotifier) {
	if h == nil || n == nil {
		return
	}
	h.SetEnemyTurnNotifier(n)
}

// gatewayHandlerAdder narrows *discordgo.Session.AddHandler to the surface
// wireBotHandlers needs so the SR-003 wiring is unit-testable with a fake.
type gatewayHandlerAdder interface {
	AddHandler(handler any) func()
}

// gatewayIntentSetter narrows the OR-in-an-intent operation against a
// *discordgo.Session's Identify.Intents field. The production adapter below
// (wrapping *discordgo.Session) sets the bit directly; the test fake records
// the OR so the assertion can prove IntentsGuildMembers was requested.
type gatewayIntentSetter interface {
	OrIntent(i discordgo.Intent)
}

// sessionIntentSetter adapts *discordgo.Session to gatewayIntentSetter so
// wireBotHandlers can OR-in IntentsGuildMembers on the real session without
// every caller knowing the Identify field name.
type sessionIntentSetter struct {
	s *discordgo.Session
}

func (a *sessionIntentSetter) OrIntent(i discordgo.Intent) {
	a.s.Identify.Intents |= i
}

// wireBotHandlers registers the three gateway callbacks required for SR-003:
// HandleGuildCreate (dynamic guild-join command registration, spec line 179),
// HandleGuildMemberAdd (welcome DMs, spec lines 183-200), and the
// InteractionCreate router shim that dispatches slash commands through
// CommandRouter. It also OR-s IntentsGuildMembers into Identify.Intents —
// without that bit discordgo's default IntentsAllWithoutPrivileged excludes
// the GuildMembers privileged intent and member-join events never arrive.
//
// The adder + intents seam is what makes this unit-testable; in production
// the same *discordgo.Session satisfies both adder (via its AddHandler
// method) and intents (via sessionIntentSetter).
func wireBotHandlers(adder gatewayHandlerAdder, intents gatewayIntentSetter, bot *discord.Bot, router *discord.CommandRouter) {
	if adder == nil || bot == nil {
		return
	}
	adder.AddHandler(bot.HandleGuildCreate)
	adder.AddHandler(bot.HandleGuildMemberAdd)
	if router != nil {
		adder.AddHandler(func(_ *discordgo.Session, i *discordgo.InteractionCreate) {
			router.Handle(i.Interaction)
		})
	}
	if intents != nil {
		intents.OrIntent(discordgo.IntentsGuildMembers)
	}
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
	rollHistoryLogger        dice.RollHistoryLogger
	lootService              *loot.Service
	// crit-01c: optional collaborators for the inventory + ASI + character
	// + undo + retire wiring. Each field is nil-safe in buildDiscordHandlers.
	levelUpService *levelup.Service
	dmQueueFunc    func(guildID string) string
	notifier       dmqueue.Notifier
	portalBaseURL  string
	// reactionPrompts is the shared button-prompt store. When nil, the
	// constructor builds a fresh per-call store so unit-test wiring still
	// exercises the SetMaterialPromptStore / SetClassFeaturePromptPoster
	// branches. Production callers pass a single store that's also wired
	// onto the CommandRouter via SetReactionPromptStore so button clicks
	// route through one place.
	reactionPrompts *discord.ReactionPromptStore
	// SR-007: optional CardUpdater fan-out so non-combat mutators
	// (/equip /use /give /loot /attune /unattune /rest /prepare) refresh
	// the persistent #character-cards message on success. Nil-safe.
	cardUpdater discord.CardUpdater
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
	attack            *discord.AttackHandler
	bonus             *discord.BonusHandler
	shove             *discord.ShoveHandler
	interact          *discord.InteractHandler
	deathsave         *discord.DeathSaveHandler
	cast              *discord.CastHandler
	prepare           *discord.PrepareHandler
	help              *discord.HelpHandler
	inventory         *discord.InventoryHandler
	equip             *discord.EquipHandler
	give              *discord.GiveHandler
	attune            *discord.AttuneHandler
	unattune          *discord.UnattuneHandler
	character         *discord.CharacterHandler
	asi               *discord.ASIHandler
	undo              *discord.UndoHandler
	retire            *discord.RetireHandler
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
			deps.rollHistoryLogger,
		),
		save: discord.NewSaveHandler(
			deps.session,
			deps.roller,
			checkCampProv,
			characterLookup,
			deps.resolver,
			combatantLookup,
			deps.rollHistoryLogger,
		),
		rest: func() *discord.RestHandler {
			h := discord.NewRestHandler(
				deps.session,
				deps.roller,
				checkCampProv,
				characterLookup,
				deps.resolver,
				restCharUpdater,
				deps.rollHistoryLogger,
				deps.dmQueueFunc,
			)
			if deps.cardUpdater != nil {
				h.SetCardUpdater(deps.cardUpdater)
			}
			return h
		}(),
		summon:   discord.NewSummonCommandHandler(deps.session, summonSvc),
		recap:    discord.NewRecapHandler(deps.session, recapSvc, deps.resolver, newRecapPlayerLookupAdapter(combatantLookup)),
		reaction: discord.NewReactionHandler(deps.session, newReactionServiceAdapter(deps.combatService), deps.resolver, combatantLookup),
		use: func() *discord.UseHandler {
			h := discord.NewUseHandler(deps.session, checkCampProv, characterLookup, useStore, nil, newUseGiveTurnAdapter(deps.queries))
			if deps.cardUpdater != nil {
				h.SetCardUpdater(deps.cardUpdater)
			}
			return h
		}(),
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
		if deps.cardUpdater != nil {
			handlers.loot.SetCardUpdater(deps.cardUpdater)
		}
	}

	// Setter-based wiring for handlers that don't accept these via constructor.
	handlers.summon.SetEncounterProvider(deps.resolver)
	// D-50/D-54/D-57: /action subcommand dispatch needs a roller (Hide, Escape,
	// Turn Undead) and a channel-id provider (combat-log mirroring).
	if deps.roller != nil {
		handlers.action.SetRoller(deps.roller)
	}
	if deps.campaignSettings != nil {
		handlers.action.SetChannelIDProvider(deps.campaignSettings)
	}
	// C-DISCORD follow-up: wire /action stabilize persistence. *refdata.Queries
	// already exposes UpdateCombatantDeathSaves so it satisfies the
	// ActionStabilizeStore interface structurally. nil-safe.
	if deps.queries != nil {
		handlers.action.SetStabilizeStore(deps.queries)
	}
	// D-54-followup: wire walk-speed lookup so /action stand resolves the
	// actor's real max speed (Halfling 25ft / Tabaxi 35ft) instead of
	// hardcoding 30ft. moveSizeSpeedAdapter satisfies ActionSpeedLookup
	// via its LookupWalkSpeed helper. The same adapter doubles as the
	// ActionMedicineLookup for C-43-stabilize-followup (WIS + Medicine
	// proficiency).
	if deps.queries != nil {
		speedAndMedicine := newMoveSizeSpeedAdapter(deps.queries)
		handlers.action.SetSpeedLookup(speedAndMedicine)
		handlers.action.SetMedicineLookup(speedAndMedicine)
	}
	// AOE-CAST follow-up: wire /save AoE pending-save resolution so per-
	// player /save calls fan into Service.RecordAoEPendingSaveRoll +
	// ResolveAoEPendingSaves. Without this wiring the per-player rolls land
	// in pending_saves but the damage-application step never fires.
	if deps.combatService != nil && deps.roller != nil {
		handlers.save.SetAoESaveResolver(discord.NewAoESaveServiceAdapter(deps.combatService, deps.roller))
	}
	// Phase 110 it3: wire the character lookup so exploration /move can
	// disambiguate which PC combatant belongs to the invoking Discord user
	// (resolveExplorationMover falls back to pcs[0] when this is nil).
	if characterLookup != nil {
		handlers.move.SetCharacterLookup(characterLookup)
	}
	// med-21 / Phase 30: replace hardcoded Medium / 30 ft defaults in the
	// /move prone-stand path with a real character/creature size+speed
	// lookup. Skipped when no Queries are wired (test deploys).
	if deps.queries != nil {
		handlers.move.SetSizeSpeedLookup(newMoveSizeSpeedAdapter(deps.queries))
	}
	// med-24 / Phase 55: wire opportunity-attack hooks so /move fires
	// reaction prompts to #your-turn when a mover leaves a hostile's
	// reach. All four collaborators must be present; unset queries or
	// a missing campaignSettings provider degrades to no OA prompts.
	if deps.queries != nil && deps.campaignSettings != nil {
		handlers.move.SetOpportunityAttackHooks(
			newMoveOATurnsAdapter(deps.queries),
			newMoveOACreatureAdapter(deps.queries),
			newMoveOAPCReachAdapter(deps.queries),
			deps.campaignSettings,
		)
	}
	// D-56 / Phase 56: wire the drag lookup so /move applies the x2 drag
	// movement cost when the mover is currently grappling another combatant.
	if deps.combatService != nil {
		handlers.move.SetDragLookup(deps.combatService)
	}
	// med-31 / Phase 75b: wire armor lookup so /check stealth applies the
	// equipped armor's stealth_disadv flag. *refdata.Queries already
	// satisfies discord.CheckArmorLookup via GetArmor.
	if deps.queries != nil {
		handlers.check.SetArmorLookup(deps.queries)
	}
	// SR-022: wire the beast lookup on /check and /save so a Wild Shaped
	// druid actually rolls with the beast's STR/DEX/CON. *refdata.Queries
	// already exposes GetCreature so it satisfies CheckCreatureLookup
	// structurally. Nil-safe — handlers fall back to druid scores when
	// unwired.
	if deps.queries != nil {
		handlers.check.SetCreatureLookup(deps.queries)
		handlers.save.SetCreatureLookup(deps.queries)
	}
	// SR-024: wire the character-row lookup on /save so a paladin L6+
	// projects their Aura of Protection (CHA mod to saves within 10 ft;
	// 30 ft at L18) onto allies' /save FES. *refdata.Queries.GetCharacter
	// satisfies discord.SaveNearbyPaladinLookup structurally. Nil-safe —
	// /save silently skips the aura when unwired.
	if deps.queries != nil {
		handlers.save.SetNearbyPaladinLookup(deps.queries)
	}
	// COMBAT-MISC-followup / E-69: wire the encounter-zone lookup on both
	// /check (Perception disadvantage in obscurement) and /action (Hide
	// gating). *combat.Service already exposes ListZonesForEncounter so it
	// satisfies CheckZoneLookup / ActionZoneLookup structurally. nil-safe.
	if deps.combatService != nil {
		handlers.check.SetZoneLookup(deps.combatService)
		handlers.action.SetZoneLookup(deps.combatService)
	}
	if deps.enemyTurnEncounterLookup != nil {
		handlers.enemyTurnNotifier.SetEncounterLookup(deps.enemyTurnEncounterLookup)
	}

	// Phase 22 wiring (high-10): plumb the map regenerator + campaign-settings
	// provider into the /done handler so PostCombatMap actually fires PNGs to
	// #combat-map. PostCombatMap silently no-ops when either is nil; both
	// must be set in production for the channel to receive any image.
	if deps.mapRegenerator != nil {
		handlers.done.SetMapRegenerator(deps.mapRegenerator)
	}
	if deps.campaignSettings != nil {
		handlers.done.SetCampaignSettingsProvider(deps.campaignSettings)
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

	// crit-01a: wire the combat-action family of slash commands. Each
	// handler structurally relies on *combat.Service for the action
	// dispatch and *refdata.Queries for the lookup adapter. When either
	// is absent (test deploys without a database) the handlers stay nil
	// and the router falls back to the status-aware stub.
	attachCombatActionHandlers(&handlers, deps)

	// crit-01c: wire /help, /inventory, /equip, /give, /attune,
	// /unattune, /character, ASI components, /undo, /retire. /help has
	// no DB dependency so it's wired unconditionally; the rest are
	// guarded on their respective collaborators being non-nil.
	attachInventoryAndCharacterHandlers(&handlers, deps, characterLookup, checkCampProv, combatantLookup)

	return handlers
}

// attachInventoryAndCharacterHandlers builds /help, /inventory, /equip,
// /give, /attune, /unattune, /character, ASI components, /undo, and /retire
// when the necessary collaborators are available. /help is dependency-free
// so it always succeeds; the rest are guarded on deps.queries.
func attachInventoryAndCharacterHandlers(
	handlers *discordHandlers,
	deps discordHandlerDeps,
	characterLookup *characterByDiscordAdapter,
	checkCampProv *checkCampaignProviderAdapter,
	combatantLookup *combatantByDiscordAdapter,
) {
	if deps.session == nil {
		return
	}

	handlers.help = discord.NewHelpHandler(deps.session)

	if deps.queries == nil {
		return
	}

	handlers.inventory = discord.NewInventoryHandler(deps.session, checkCampProv, characterLookup)
	handlers.equip = discord.NewEquipHandler(deps.session, checkCampProv, characterLookup, deps.queries)
	// SR-004: route /equip through combat.Equip so 2H/shield validation,
	// in-combat armor block, AC recalc, and equipped_main_hand/off_hand/
	// armor column writes actually happen (downstream grapple, somatic,
	// stealth, attack all read those columns). When combat svc is nil
	// (test deploys without a combat service) the handler keeps the
	// legacy inventory-JSON-only fallback.
	if deps.combatService != nil {
		handlers.equip.SetCombatService(deps.combatService)
		handlers.equip.SetEncounterProvider(newCombatActionLookupAdapter(deps.combatService, deps.queries, deps.resolver))
	}
	if deps.cardUpdater != nil {
		// SR-007: legacy inventory-only fallback path also needs to refresh
		// the persistent #character-cards message. The SR-004 combat-routed
		// path is wired one level down via combat.Service.SetCardUpdater.
		handlers.equip.SetCardUpdater(deps.cardUpdater)
	}
	handlers.attune = discord.NewAttuneHandler(deps.session, checkCampProv, characterLookup, deps.queries)
	handlers.unattune = discord.NewUnattuneHandler(deps.session, checkCampProv, characterLookup, deps.queries)
	if deps.cardUpdater != nil {
		handlers.attune.SetCardUpdater(deps.cardUpdater)
		handlers.unattune.SetCardUpdater(deps.cardUpdater)
	}
	handlers.give = discord.NewGiveHandler(
		deps.session,
		checkCampProv,
		characterLookup,
		newGiveTargetResolverAdapter(deps.queries),
		deps.queries,
		nil, // GiveCombatProvider is currently unused by the handler.
	)
	// med-35: wire turn provider so /give in combat costs the per-turn
	// free object interaction. Out-of-combat /give carries no cost.
	handlers.give.SetTurnProvider(newUseGiveTurnAdapter(deps.queries))
	if deps.cardUpdater != nil {
		handlers.give.SetCardUpdater(deps.cardUpdater)
	}
	handlers.character = discord.NewCharacterHandler(deps.session, deps.queries, deps.queries, deps.portalBaseURL)

	if deps.levelUpService != nil {
		handlers.asi = discord.NewASIHandler(deps.session, newASIServiceAdapter(deps.levelUpService, deps.queries), deps.dmQueueFunc)
		// med-36 / Phase 89: wire feat lister so the "Choose a Feat"
		// button posts a real select-menu instead of the stub.
		if deps.queries != nil {
			handlers.asi.SetFeatLister(newASIFeatLister(deps.queries))
			// F-89d: persist pending ASI/Feat choices across restart.
			handlers.asi.SetPendingStore(newASIPendingStoreAdapter(deps.queries))
			_ = handlers.asi.HydratePending(context.Background())
		}
	}

	// /retire shares the campaign + character lookups with inventory. The
	// PC store flags the player's existing player_characters row with
	// created_via='retire' so the Phase 16 dashboard retire-approval branch
	// (internal/dashboard/approval_handler.go) becomes reachable.
	handlers.retire = discord.NewRetireHandler(deps.session, checkCampProv, characterLookup, deps.notifier)
	if deps.queries != nil {
		handlers.retire.SetPCStore(deps.queries)
	}

	// /undo needs the encounter resolver, the combatant lookup, and the
	// action_log reader. All of them are nil-safe in the handler.
	handlers.undo = discord.NewUndoHandler(
		deps.session,
		deps.resolver,
		combatantLookup,
		deps.queries,
		deps.notifier,
	)
	// SR-002: thread the campaign-by-guild lookup so /undo dm-queue posts
	// carry the campaign UUID that PgStore.Insert requires.
	handlers.undo.SetCampaignProvider(checkCampProv)

	// SR-002: same wiring for /reaction declarations.
	if handlers.reaction != nil {
		handlers.reaction.SetCampaignProvider(checkCampProv)
	}
}

// attachCombatActionHandlers builds the /attack, /bonus, /shove,
// /interact, and /deathsave handlers when the necessary collaborators
// are available, and wires the turn-gate + channel-id provider into each.
func attachCombatActionHandlers(handlers *discordHandlers, deps discordHandlerDeps) {
	if deps.session == nil || deps.queries == nil || deps.combatService == nil || deps.roller == nil {
		return
	}

	combatLookup := newCombatActionLookupAdapter(deps.combatService, deps.queries, deps.resolver)
	checkCampProv := newCheckCampaignProviderAdapter(deps.queries)
	characterLookup := newCharacterByDiscordAdapter(deps.queries)
	combatantLookup := newCombatantByDiscordAdapter(deps.queries)

	handlers.attack = discord.NewAttackHandler(deps.session, deps.combatService, combatLookup, deps.roller)
	handlers.bonus = discord.NewBonusHandler(deps.session, deps.combatService, combatLookup, deps.roller)
	handlers.shove = discord.NewShoveHandler(deps.session, deps.combatService, combatLookup, deps.roller)
	handlers.interact = discord.NewInteractHandler(deps.session, combatLookup, deps.queries)
	// SR-005: route /interact through combat.Interact so the second interact
	// on a turn falls back to the action (instead of being rejected outright)
	// and a pending_actions row is created for the DM queue / dashboard.
	handlers.interact.SetCombatService(deps.combatService)
	handlers.deathsave = discord.NewDeathSaveHandler(
		deps.session,
		deps.roller,
		deps.resolver,
		combatantLookup,
		deps.queries,
		checkCampProv,
		characterLookup,
	)

	// crit-01b: wire /cast and /prepare. Both share the combatLookup adapter
	// for the encounter/turn/combatant lookups; /cast also needs spell + map
	// catalog access via the dedicated castLookupAdapter. /prepare runs out
	// of combat too, so it tolerates a missing active encounter.
	castLookup := newCastLookupAdapter(deps.combatService, deps.queries, deps.resolver)
	handlers.cast = discord.NewCastHandler(deps.session, deps.combatService, castLookup, deps.roller)
	handlers.prepare = discord.NewPrepareHandler(
		deps.session,
		deps.combatService,
		newPrepareEncounterProviderAdapter(deps.combatService, deps.resolver),
		checkCampProv,
		characterLookup,
	)
	if deps.cardUpdater != nil {
		handlers.prepare.SetCardUpdater(deps.cardUpdater)
	}

	if deps.campaignSettings != nil {
		handlers.attack.SetChannelIDProvider(deps.campaignSettings)
		handlers.bonus.SetChannelIDProvider(deps.campaignSettings)
		handlers.shove.SetChannelIDProvider(deps.campaignSettings)
		handlers.interact.SetChannelIDProvider(deps.campaignSettings)
		handlers.deathsave.SetChannelIDProvider(deps.campaignSettings)
		handlers.cast.SetChannelIDProvider(deps.campaignSettings)
	}

	// C-DISCORD follow-up: wire the map provider for /attack so AttackCommand.Walls
	// is populated and wall-based cover applies (Phase 33).
	// *refdata.Queries.GetMapByID structurally satisfies AttackMapProvider.
	handlers.attack.SetMapProvider(deps.queries)

	// AOE-CAST follow-up: wire the shared ReactionPromptStore the cast
	// handler uses for the gold-fallback Buy & Cast prompt (E-63), and
	// build a ClassFeaturePromptPoster off the same store for the /attack
	// post-hit Stunning Strike / Divine Smite / Bardic Inspiration prompts
	// (D-48b/D-49/D-51 follow-up). A nil reactionPrompts in deps means the
	// test deploy didn't pass a shared store; we build a fresh one in-place
	// so the production wiring path is still exercised end-to-end.
	prompts := deps.reactionPrompts
	if prompts == nil {
		prompts = discord.NewReactionPromptStore(deps.session)
	}
	handlers.cast.SetMaterialPromptStore(prompts)
	// SR-025: wire the Empowered/Careful/Heightened Spell prompt poster so
	// /cast surfaces the interactive metamagic UI instead of leaving the
	// poster unreachable (the dead-code regression SR-025 closes).
	handlers.cast.SetMetamagicPromptPoster(discord.NewMetamagicPromptPoster(prompts))
	handlers.attack.SetClassFeaturePromptPoster(discord.NewClassFeaturePromptPoster(prompts))
	handlers.attack.SetClassFeatureService(deps.combatService)

	if deps.db != nil {
		gate := newTurnGateAdapter(deps.db, deps.queries)
		handlers.attack.SetTurnGate(gate)
		handlers.bonus.SetTurnGate(gate)
		handlers.shove.SetTurnGate(gate)
		handlers.interact.SetTurnGate(gate)
		handlers.cast.SetTurnGate(gate)
		// /deathsave is intentionally NOT gated: a dying PC rolls off-
		// turn, so the per-turn ownership check would always fail.
		// /prepare is intentionally NOT gated: it runs between sessions,
		// outside any turn — the handler itself rejects when the active
		// encounter is in `status="active"`.
	}
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
	if set.attack != nil {
		r.SetAttackHandler(set.attack)
	}
	if set.bonus != nil {
		r.SetBonusHandler(set.bonus)
	}
	if set.shove != nil {
		r.SetShoveHandler(set.shove)
	}
	if set.interact != nil {
		r.SetInteractHandler(set.interact)
	}
	if set.deathsave != nil {
		r.SetDeathSaveHandler(set.deathsave)
	}
	if set.cast != nil {
		r.SetCastHandler(set.cast)
	}
	if set.prepare != nil {
		r.SetPrepareHandler(set.prepare)
	}
	if set.help != nil {
		r.SetHelpHandler(set.help)
	}
	if set.inventory != nil {
		r.SetInventoryHandler(set.inventory)
	}
	if set.equip != nil {
		r.SetEquipHandler(set.equip)
	}
	if set.give != nil {
		r.SetGiveHandler(set.give)
	}
	if set.attune != nil {
		r.SetAttuneHandler(set.attune)
	}
	if set.unattune != nil {
		r.SetUnattuneHandler(set.unattune)
	}
	if set.character != nil {
		r.SetCharacterHandler(set.character)
	}
	if set.asi != nil {
		r.SetASIHandler(set.asi)
	}
	if set.undo != nil {
		r.SetUndoHandler(set.undo)
	}
	if set.retire != nil {
		r.SetRetireHandler(set.retire)
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

// moveOATurnsAdapter satisfies discord.MoveOATurnsLookup by enumerating the
// current round's turns for an encounter and re-keying them by combatant_id
// (the key DetectOpportunityAttacks expects). med-24 / Phase 55.
type moveOATurnsAdapter struct {
	queries *refdata.Queries
}

func newMoveOATurnsAdapter(q *refdata.Queries) *moveOATurnsAdapter {
	if q == nil {
		return nil
	}
	return &moveOATurnsAdapter{queries: q}
}

func (a *moveOATurnsAdapter) ListTurnsByEncounter(ctx context.Context, encounterID uuid.UUID) (map[uuid.UUID]refdata.Turn, error) {
	enc, err := a.queries.GetEncounter(ctx, encounterID)
	if err != nil {
		return nil, err
	}
	turns, err := a.queries.ListTurnsByEncounterAndRound(ctx, refdata.ListTurnsByEncounterAndRoundParams{
		EncounterID: encounterID,
		RoundNumber: enc.RoundNumber,
	})
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]refdata.Turn, len(turns))
	for _, t := range turns {
		out[t.CombatantID] = t
	}
	return out, nil
}

// moveOACreatureAdapter satisfies discord.MoveOACreatureLookup by fetching
// the creature row and parsing its Attacks JSONB column. med-24.
type moveOACreatureAdapter struct {
	queries *refdata.Queries
}

func newMoveOACreatureAdapter(q *refdata.Queries) *moveOACreatureAdapter {
	if q == nil {
		return nil
	}
	return &moveOACreatureAdapter{queries: q}
}

func (a *moveOACreatureAdapter) GetCreatureAttacks(ctx context.Context, refID string) ([]combat.CreatureAttackEntry, error) {
	creature, err := a.queries.GetCreature(ctx, refID)
	if err != nil {
		return nil, err
	}
	return combat.ParseCreatureAttacksWithSource(creature.Attacks, creature.Source)
}

// moveOAPCReachAdapter satisfies discord.MoveOAPCWeaponReach by reading the
// PC's equipped melee weapon and returning 10 if it has the "reach" property
// (5 otherwise, 0 when no equipped melee weapon). med-24.
type moveOAPCReachAdapter struct {
	queries *refdata.Queries
}

func newMoveOAPCReachAdapter(q *refdata.Queries) *moveOAPCReachAdapter {
	if q == nil {
		return nil
	}
	return &moveOAPCReachAdapter{queries: q}
}

func (a *moveOAPCReachAdapter) LookupPCReach(ctx context.Context, charID uuid.UUID) (int, error) {
	char, err := a.queries.GetCharacter(ctx, charID)
	if err != nil {
		return 0, err
	}
	if !char.EquippedMainHand.Valid || char.EquippedMainHand.String == "" {
		return 5, nil
	}
	weapon, err := a.queries.GetWeapon(ctx, char.EquippedMainHand.String)
	if err != nil {
		// Best-effort: weapon not found defaults to 5ft reach so we
		// still get an OA prompt at the standard distance.
		return 5, nil
	}
	if combat.IsRangedWeapon(weapon) {
		// Ranged weapons don't trigger melee OAs; signal "no PC
		// reach override" by returning 0 so the default 5ft is used.
		// The detector still treats hostile as melee-default, which
		// is acceptable degradation here.
		return 5, nil
	}
	if combat.HasProperty(weapon, "reach") {
		return 10, nil
	}
	return 5, nil
}

// useGiveTurnAdapter satisfies both discord.UseCombatProvider and
// discord.GiveTurnProvider by joining the invoking user's Discord ID through
// (campaign → character → active encounter → current turn). Returns
// inCombat=false when the character has no active encounter or the active
// encounter has no current turn (out of combat). med-35.
type useGiveTurnAdapter struct {
	queries *refdata.Queries
}

func newUseGiveTurnAdapter(q *refdata.Queries) *useGiveTurnAdapter {
	if q == nil {
		return nil
	}
	return &useGiveTurnAdapter{queries: q}
}

func (a *useGiveTurnAdapter) GetActiveTurnForCharacter(ctx context.Context, _ string, charID uuid.UUID) (refdata.Turn, bool, error) {
	encID, err := a.queries.GetActiveEncounterIDByCharacterID(ctx, uuid.NullUUID{UUID: charID, Valid: true})
	if err != nil {
		// sql.ErrNoRows = character is not in any active encounter
		// (out of combat). Surface as a clean (turn, false, nil) so the
		// caller treats it as out-of-combat instead of erroring.
		if errors.Is(err, sql.ErrNoRows) {
			return refdata.Turn{}, false, nil
		}
		return refdata.Turn{}, false, err
	}
	enc, err := a.queries.GetEncounter(ctx, encID)
	if err != nil {
		return refdata.Turn{}, false, err
	}
	if !enc.CurrentTurnID.Valid {
		return refdata.Turn{}, false, nil
	}
	turn, err := a.queries.GetTurn(ctx, enc.CurrentTurnID.UUID)
	if err != nil {
		return refdata.Turn{}, false, err
	}
	return turn, true, nil
}

func (a *useGiveTurnAdapter) UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	return a.queries.UpdateTurnActions(ctx, arg)
}

// asiFeatLister satisfies discord.FeatLister by enumerating all seeded feats
// (capped at 25, the Discord select-menu maximum). Prerequisite filtering is
// delegated to the approval flow per the chunk-7 recommendation; the picker
// surfaces every feat alphabetically for the simplest possible UX.
// med-36 / Phase 89.
type asiFeatLister struct {
	queries *refdata.Queries
}

func newASIFeatLister(q *refdata.Queries) *asiFeatLister {
	if q == nil {
		return nil
	}
	return &asiFeatLister{queries: q}
}

func (a *asiFeatLister) ListEligibleFeats(ctx context.Context, _ uuid.UUID) ([]discord.FeatOption, error) {
	feats, err := a.queries.ListFeats(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]discord.FeatOption, 0, len(feats))
	for _, f := range feats {
		if len(out) >= 25 {
			break
		}
		out = append(out, discord.FeatOption{
			ID:          f.ID,
			Name:        f.Name,
			Description: f.Description,
		})
	}
	return out, nil
}

// asiPendingStoreAdapter bridges the discord.ASIPendingStore contract onto
// refdata.Queries.UpsertPendingASI / GetPendingASI / DeletePendingASI / List
// PendingASI so pending ASI/Feat choices survive a bot restart (F-89d).
type asiPendingStoreAdapter struct {
	queries *refdata.Queries
}

func newASIPendingStoreAdapter(q *refdata.Queries) *asiPendingStoreAdapter {
	if q == nil {
		return nil
	}
	return &asiPendingStoreAdapter{queries: q}
}

func (a *asiPendingStoreAdapter) Save(ctx context.Context, c discord.PendingASIChoice) error {
	raw, err := discord.MarshalPendingASIChoice(c)
	if err != nil {
		return fmt.Errorf("marshalling pending ASI: %w", err)
	}
	return a.queries.UpsertPendingASI(ctx, refdata.UpsertPendingASIParams{
		CharacterID:  c.CharID,
		SnapshotJson: raw,
	})
}

func (a *asiPendingStoreAdapter) Delete(ctx context.Context, charID uuid.UUID) error {
	return a.queries.DeletePendingASI(ctx, charID)
}

func (a *asiPendingStoreAdapter) List(ctx context.Context) ([]discord.PendingASIChoice, error) {
	rows, err := a.queries.ListPendingASI(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]discord.PendingASIChoice, 0, len(rows))
	for _, r := range rows {
		c, err := discord.UnmarshalPendingASIChoice(r.SnapshotJson)
		if err != nil {
			continue
		}
		// Ensure we always populate CharID (durable PK).
		c.CharID = r.CharacterID
		out = append(out, c)
	}
	return out, nil
}

// moveSizeSpeedAdapter satisfies discord.MoveSizeSpeedLookup by joining
// the combatant to either a Character (PCs) or a Creature (NPCs) and
// extracting the size category + walk speed. PCs default to size Medium
// because the characters table doesn't carry a size column today; NPCs
// use Creature.Size and parse Creature.Speed JSON via combat.ParseWalkSpeed.
// med-21 / Phase 30 wires this so /move stops hardcoding Medium / 30 ft.
type moveSizeSpeedAdapter struct {
	queries *refdata.Queries
}

func newMoveSizeSpeedAdapter(q *refdata.Queries) *moveSizeSpeedAdapter {
	if q == nil {
		return nil
	}
	return &moveSizeSpeedAdapter{queries: q}
}

// LookupWalkSpeed satisfies discord.ActionSpeedLookup by returning just the
// walk-speed component of LookupSizeAndSpeed. D-54-followup wires this on
// the action handler so /action stand computes the half-movement stand
// cost from the actor's real walk speed (Halfling 25ft / Tabaxi 35ft).
func (a *moveSizeSpeedAdapter) LookupWalkSpeed(ctx context.Context, combatant refdata.Combatant) (int, error) {
	_, speed, err := a.LookupSizeAndSpeed(ctx, combatant)
	return speed, err
}

// LookupMedicineModifier satisfies discord.ActionMedicineLookup by reading
// the combatant's character / creature row and computing the full Medicine
// modifier (WIS mod + Medicine proficiency + expertise + jack-of-all-trades).
// NPCs use Creature.Skills' pre-calculated medicine if present, otherwise
// fall back to WIS modifier. Errors degrade to (0, err) so the handler's
// "always returns +0 on error" path keeps /action stabilize functioning.
// (C-43-stabilize-followup)
func (a *moveSizeSpeedAdapter) LookupMedicineModifier(ctx context.Context, combatant refdata.Combatant) (int, error) {
	if combatant.CharacterID.Valid {
		char, err := a.queries.GetCharacter(ctx, combatant.CharacterID.UUID)
		if err != nil {
			return 0, err
		}
		var scores character.AbilityScores
		if uerr := json.Unmarshal(char.AbilityScores, &scores); uerr != nil {
			return 0, uerr
		}
		profSkills, expertise, jack := parseProficienciesFromJSON(char.Proficiencies.RawMessage)
		return character.SkillModifier(scores, "medicine", profSkills, expertise, jack, int(char.ProficiencyBonus)), nil
	}
	if combatant.CreatureRefID.Valid && combatant.CreatureRefID.String != "" {
		creature, err := a.queries.GetCreature(ctx, combatant.CreatureRefID.String)
		if err != nil {
			return 0, err
		}
		if mod, ok := creatureMedicineMod(creature.Skills.RawMessage); ok {
			return mod, nil
		}
		var scores combat.AbilityScores
		if uerr := json.Unmarshal(creature.AbilityScores, &scores); uerr != nil {
			return 0, uerr
		}
		return combat.AbilityModifier(scores.Wis), nil
	}
	return 0, nil
}

// parseProficienciesFromJSON mirrors combat.parseProficiencies (unexported).
// Extracts skills / expertise / jack-of-all-trades from the proficiencies
// JSON column. Bad JSON returns the zero values rather than erroring out so
// /action stabilize degrades to "non-proficient" instead of failing.
func parseProficienciesFromJSON(raw json.RawMessage) (skills []string, expertise []string, jackOfAllTrades bool) {
	if len(raw) == 0 {
		return nil, nil, false
	}
	var data struct {
		Skills          []string `json:"skills"`
		Expertise       []string `json:"expertise"`
		JackOfAllTrades bool     `json:"jack_of_all_trades"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, nil, false
	}
	return data.Skills, data.Expertise, data.JackOfAllTrades
}

// creatureMedicineMod reads a creature's pre-calculated medicine skill
// modifier from the Skills JSON map. Mirrors combat.creatureSkillMod
// (unexported). Returns (0, false) when the field is missing or unparseable.
func creatureMedicineMod(skills []byte) (int, bool) {
	if len(skills) == 0 {
		return 0, false
	}
	var m map[string]int
	if err := json.Unmarshal(skills, &m); err != nil {
		return 0, false
	}
	v, ok := m["medicine"]
	return v, ok
}

func (a *moveSizeSpeedAdapter) LookupSizeAndSpeed(ctx context.Context, combatant refdata.Combatant) (int, int, error) {
	// PCs: Character.SpeedFt is authoritative; size defaults to Medium
	// (the characters table has no size column yet — this is a
	// known follow-up gap; see chunk2 cross-cutting risks).
	if combatant.CharacterID.Valid {
		char, err := a.queries.GetCharacter(ctx, combatant.CharacterID.UUID)
		if err != nil {
			return pathfinding.SizeMedium, 30, err
		}
		speed := int(char.SpeedFt)
		// med-31 / Phase 75b: heavy-armor STR-deficient -10 ft penalty.
		// CheckHeavyArmorPenalty returns 0 when the armor has no
		// strength_req or the wearer's STR meets it.
		if char.EquippedArmor.Valid && char.EquippedArmor.String != "" {
			armor, armorErr := a.queries.GetArmor(ctx, char.EquippedArmor.String)
			if armorErr == nil {
				if penalty := combat.CheckHeavyArmorPenalty(char, armor); penalty > 0 {
					speed -= int(penalty)
					if speed < 0 {
						speed = 0
					}
				}
			}
		}
		return pathfinding.SizeMedium, speed, nil
	}
	// NPCs: Creature.Size + parsed walk speed.
	if combatant.CreatureRefID.Valid {
		creature, err := a.queries.GetCreature(ctx, combatant.CreatureRefID.String)
		if err != nil {
			return pathfinding.SizeMedium, 30, err
		}
		size := pathfinding.ParseSizeCategory(creature.Size)
		speed := combat.ParseWalkSpeed(creature.Speed)
		return size, int(speed), nil
	}
	// Unknown combatant kind — return defaults rather than erroring so
	// /move keeps functioning.
	return pathfinding.SizeMedium, 30, nil
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

func (a *actionCombatServiceAdapter) ReadyAction(ctx context.Context, cmd combat.ReadyActionCommand) (combat.ReadyActionResult, error) {
	return a.svc.ReadyAction(ctx, cmd)
}

// D-47..D-57 dispatch passthroughs. The combat service has all of these
// implemented; we just forward so the discord adapter interface stays small.

func (a *actionCombatServiceAdapter) ActionSurge(ctx context.Context, cmd combat.ActionSurgeCommand) (combat.ActionSurgeResult, error) {
	return a.svc.ActionSurge(ctx, cmd)
}

func (a *actionCombatServiceAdapter) Dash(ctx context.Context, cmd combat.DashCommand) (combat.DashResult, error) {
	return a.svc.Dash(ctx, cmd)
}

func (a *actionCombatServiceAdapter) Disengage(ctx context.Context, cmd combat.DisengageCommand) (combat.DisengageResult, error) {
	return a.svc.Disengage(ctx, cmd)
}

func (a *actionCombatServiceAdapter) Dodge(ctx context.Context, cmd combat.DodgeCommand) (combat.DodgeResult, error) {
	return a.svc.Dodge(ctx, cmd)
}

func (a *actionCombatServiceAdapter) Help(ctx context.Context, cmd combat.HelpCommand) (combat.HelpResult, error) {
	return a.svc.Help(ctx, cmd)
}

func (a *actionCombatServiceAdapter) Hide(ctx context.Context, cmd combat.HideCommand, roller *dice.Roller) (combat.HideResult, error) {
	return a.svc.Hide(ctx, cmd, roller)
}

func (a *actionCombatServiceAdapter) Stand(ctx context.Context, cmd combat.StandCommand) (combat.StandResult, error) {
	return a.svc.Stand(ctx, cmd)
}

func (a *actionCombatServiceAdapter) DropProne(ctx context.Context, cmd combat.DropProneCommand) (combat.DropProneResult, error) {
	return a.svc.DropProne(ctx, cmd)
}

func (a *actionCombatServiceAdapter) Escape(ctx context.Context, cmd combat.EscapeCommand, roller *dice.Roller) (combat.EscapeResult, error) {
	return a.svc.Escape(ctx, cmd, roller)
}

func (a *actionCombatServiceAdapter) TurnUndead(ctx context.Context, cmd combat.TurnUndeadCommand, roller *dice.Roller) (combat.TurnUndeadResult, error) {
	return a.svc.TurnUndead(ctx, cmd, roller)
}

func (a *actionCombatServiceAdapter) PreserveLife(ctx context.Context, cmd combat.PreserveLifeCommand) (combat.PreserveLifeResult, error) {
	return a.svc.PreserveLife(ctx, cmd)
}

func (a *actionCombatServiceAdapter) SacredWeapon(ctx context.Context, cmd combat.SacredWeaponCommand) (combat.SacredWeaponResult, error) {
	return a.svc.SacredWeapon(ctx, cmd)
}

func (a *actionCombatServiceAdapter) VowOfEnmity(ctx context.Context, cmd combat.VowOfEnmityCommand) (combat.VowOfEnmityResult, error) {
	return a.svc.VowOfEnmity(ctx, cmd)
}

func (a *actionCombatServiceAdapter) ChannelDivinityDMQueue(ctx context.Context, cmd combat.ChannelDivinityDMQueueCommand) (combat.DMQueueResult, error) {
	return a.svc.ChannelDivinityDMQueue(ctx, cmd)
}

func (a *actionCombatServiceAdapter) LayOnHands(ctx context.Context, cmd combat.LayOnHandsCommand) (combat.LayOnHandsResult, error) {
	return a.svc.LayOnHands(ctx, cmd)
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

// combatActionLookupAdapter satisfies the AttackEncounterProvider /
// BonusEncounterProvider / ShoveEncounterProvider / InteractEncounterProvider
// interfaces with a single struct, since they all need the same five lookups
// (resolve encounter for user + GetEncounter + GetCombatant + List combatants
// + GetTurn). Per-user resolution is delegated to the shared resolver so we
// don't duplicate the player_character -> combatant chain.
type combatActionLookupAdapter struct {
	combat   *combat.Service
	queries  *refdata.Queries
	resolver userEncounterResolver
}

func newCombatActionLookupAdapter(svc *combat.Service, q *refdata.Queries, resolver userEncounterResolver) *combatActionLookupAdapter {
	if svc == nil || q == nil {
		return nil
	}
	return &combatActionLookupAdapter{combat: svc, queries: q, resolver: resolver}
}

func (a *combatActionLookupAdapter) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	if a.resolver == nil {
		return uuid.Nil, sqlNoRowsLike()
	}
	return a.resolver.ActiveEncounterForUser(ctx, guildID, discordUserID)
}

func (a *combatActionLookupAdapter) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return a.combat.GetEncounter(ctx, id)
}

func (a *combatActionLookupAdapter) GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return a.combat.GetCombatant(ctx, id)
}

func (a *combatActionLookupAdapter) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return a.combat.ListCombatantsByEncounterID(ctx, encounterID)
}

func (a *combatActionLookupAdapter) GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return a.queries.GetTurn(ctx, id)
}

// turnGateAdapter satisfies discord.TurnGate by delegating to
// combat.AcquireTurnLockWithValidation. AcquireAndRelease commits the
// validation tx immediately (read-only validator); AcquireAndRun (F-4) holds
// the tx across a caller-supplied write callback so concurrent handlers
// serialize on the advisory lock instead of interleaving writes against the
// same turn.
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

// AcquireAndRun validates ownership, acquires the per-turn advisory lock,
// runs fn with the lock still held, then commits (releasing the lock). If
// fn returns an error the tx is rolled back and fn's error propagates;
// validation/lock errors are returned BEFORE fn runs.
//
// fn's context carries the lock-holding *sql.Tx (via combat.ContextWithTx).
// Handlers that want their writes to share the lock-holding tx call
// combat.TxFromContext(ctx) and pass the result to refdata.Queries.WithTx.
// Handlers that don't opt in still get serialization: a peer's
// AcquireAndRun blocks at the pg_advisory_xact_lock acquire until our tx
// commits, so two concurrent handlers can never interleave their writes
// against the same turn.
func (a *turnGateAdapter) AcquireAndRun(ctx context.Context, encounterID uuid.UUID, discordUserID string, fn func(ctx context.Context) error) (combat.TurnOwnerInfo, error) {
	return combat.RunUnderTurnLock(ctx, a.db, a.queries, encounterID, discordUserID, fn)
}

// castLookupAdapter satisfies discord.CastEncounterProvider over
// (combat.Service + refdata.Queries + resolver). /cast needs the union of
// the combat-action lookup surface plus GetSpell + GetMapByID, so a
// dedicated adapter is the simplest fit; combatActionLookupAdapter does
// not expose those.
type castLookupAdapter struct {
	combat   *combat.Service
	queries  *refdata.Queries
	resolver userEncounterResolver
}

func newCastLookupAdapter(svc *combat.Service, q *refdata.Queries, resolver userEncounterResolver) *castLookupAdapter {
	if svc == nil || q == nil {
		return nil
	}
	return &castLookupAdapter{combat: svc, queries: q, resolver: resolver}
}

func (a *castLookupAdapter) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	if a.resolver == nil {
		return uuid.Nil, sqlNoRowsLike()
	}
	return a.resolver.ActiveEncounterForUser(ctx, guildID, discordUserID)
}

func (a *castLookupAdapter) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return a.combat.GetEncounter(ctx, id)
}

func (a *castLookupAdapter) GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return a.combat.GetCombatant(ctx, id)
}

func (a *castLookupAdapter) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return a.combat.ListCombatantsByEncounterID(ctx, encounterID)
}

func (a *castLookupAdapter) GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return a.queries.GetTurn(ctx, id)
}

func (a *castLookupAdapter) GetSpell(ctx context.Context, id string) (refdata.Spell, error) {
	return a.queries.GetSpell(ctx, id)
}

func (a *castLookupAdapter) GetMapByID(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
	return a.queries.GetMapByID(ctx, id)
}

// prepareEncounterProviderAdapter satisfies discord.PrepareEncounterProvider
// over (combat.Service + resolver). /prepare uses the encounter only to gate
// out-of-combat — failure to resolve simply skips the gate (player may
// /prepare between sessions).
type prepareEncounterProviderAdapter struct {
	combat   *combat.Service
	resolver userEncounterResolver
}

func newPrepareEncounterProviderAdapter(svc *combat.Service, resolver userEncounterResolver) *prepareEncounterProviderAdapter {
	if svc == nil {
		return nil
	}
	return &prepareEncounterProviderAdapter{combat: svc, resolver: resolver}
}

func (a *prepareEncounterProviderAdapter) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	if a.resolver == nil {
		return uuid.Nil, sqlNoRowsLike()
	}
	return a.resolver.ActiveEncounterForUser(ctx, guildID, discordUserID)
}

func (a *prepareEncounterProviderAdapter) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return a.combat.GetEncounter(ctx, id)
}

// giveTargetResolverAdapter satisfies discord.GiveTargetResolver by trying a
// UUID parse first (direct character ID lookup), then falling back to a
// case-insensitive name match against ListCharactersByCampaign. Returns the
// first match; when no character matches, returns sqlNoRowsLike().
type giveTargetResolverAdapter struct {
	queries *refdata.Queries
}

func newGiveTargetResolverAdapter(q *refdata.Queries) *giveTargetResolverAdapter {
	if q == nil {
		return nil
	}
	return &giveTargetResolverAdapter{queries: q}
}

func (a *giveTargetResolverAdapter) ResolveTarget(ctx context.Context, campaignID uuid.UUID, nameOrID string) (refdata.Character, error) {
	if id, err := uuid.Parse(nameOrID); err == nil {
		char, err := a.queries.GetCharacter(ctx, id)
		if err == nil && char.CampaignID == campaignID {
			return char, nil
		}
	}
	chars, err := a.queries.ListCharactersByCampaign(ctx, campaignID)
	if err != nil {
		return refdata.Character{}, err
	}
	needle := strings.ToLower(nameOrID)
	for _, c := range chars {
		if strings.ToLower(c.Name) == needle {
			return c, nil
		}
	}
	return refdata.Character{}, sqlNoRowsLike()
}

// asiServiceAdapter bridges *levelup.Service onto the discord.ASIService
// contract, translating between discord.ASIChoiceData (the handler's wire
// format) and levelup.ASIChoice (the service's domain type), and assembling
// an ASICharacterData snapshot from the levelup CharacterStore + queries.
type asiServiceAdapter struct {
	svc     *levelup.Service
	queries *refdata.Queries
}

func newASIServiceAdapter(svc *levelup.Service, q *refdata.Queries) discord.ASIService {
	if svc == nil || q == nil {
		return nil
	}
	return &asiServiceAdapter{svc: svc, queries: q}
}

func (a *asiServiceAdapter) ApproveASI(ctx context.Context, charID uuid.UUID, choice discord.ASIChoiceData) error {
	// med-36 / Phase 89: feat approvals route to ApplyFeat (which adds the
	// feat to the character's features and applies any baked-in ASI bonus).
	// Ability-score approvals continue through the existing ApproveASI
	// path. Empty FeatID for type=="feat" is rejected upstream.
	if choice.Type == "feat" && choice.FeatID != "" {
		feat, err := a.queries.GetFeat(ctx, choice.FeatID)
		if err != nil {
			return fmt.Errorf("loading feat %q: %w", choice.FeatID, err)
		}
		info := levelup.FeatInfo{ID: feat.ID, Name: feat.Name}
		if feat.AsiBonus.Valid {
			_ = json.Unmarshal(feat.AsiBonus.RawMessage, &info.ASIBonus)
		}
		if feat.MechanicalEffect.Valid {
			_ = json.Unmarshal(feat.MechanicalEffect.RawMessage, &info.MechanicalEffect)
		}
		return a.svc.ApplyFeat(ctx, charID, info)
	}
	return a.svc.ApproveASI(ctx, charID, levelup.ASIChoice{
		Type:     levelup.ASIType(choice.Type),
		Ability:  choice.Ability,
		Ability2: choice.Ability2,
		FeatID:   choice.FeatID,
	})
}

func (a *asiServiceAdapter) DenyASI(ctx context.Context, charID uuid.UUID, reason string) error {
	return a.svc.DenyASI(ctx, charID, reason)
}

func (a *asiServiceAdapter) GetCharacterForASI(ctx context.Context, charID uuid.UUID) (*discord.ASICharacterData, error) {
	row, err := a.queries.GetCharacterForLevelUp(ctx, charID)
	if err != nil {
		return nil, err
	}
	var scores character.AbilityScores
	if err := json.Unmarshal(row.AbilityScores, &scores); err != nil {
		return nil, fmt.Errorf("parsing ability scores: %w", err)
	}
	classInfo := ""
	if len(row.Classes) > 0 {
		var entries []character.ClassEntry
		if err := json.Unmarshal(row.Classes, &entries); err == nil {
			classInfo = character.FormatClassSummary(entries)
		}
	}
	return &discord.ASICharacterData{
		ID:            row.ID,
		Name:          row.Name,
		DiscordUserID: row.DiscordUserID,
		Scores:        scores,
		ClassInfo:     classInfo,
	}, nil
}
