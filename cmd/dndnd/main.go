package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/bwmarrin/discordgo"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/asset"
	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/characteroverview"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dashboard"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/encounter"
	"github.com/ab/dndnd/internal/errorlog"
	"github.com/ab/dndnd/internal/exploration"
	"github.com/ab/dndnd/internal/gamemap"
	"github.com/ab/dndnd/internal/homebrew"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/levelup"
	"github.com/ab/dndnd/internal/messageplayer"
	"github.com/ab/dndnd/internal/narration"
	"github.com/ab/dndnd/internal/open5e"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/server"
	"github.com/ab/dndnd/internal/statblocklibrary"
)

// encounterLookupAdapter resolves the active encounter (if any) containing
// the given character by asking refdata.Queries. It satisfies the
// inventory.EncounterLookup contract (and levelup.EncounterLookup, which is
// structurally identical) so Phase 104b's publisher fan-out can skip
// publishing whenever the mutation does not touch live combat state.
type encounterLookupAdapter struct {
	queries *refdata.Queries
}

func (a encounterLookupAdapter) ActiveEncounterIDForCharacter(ctx context.Context, characterID uuid.UUID) (uuid.UUID, bool, error) {
	encID, err := a.queries.GetActiveEncounterIDByCharacterID(ctx, uuid.NullUUID{UUID: characterID, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		// Expected whenever the character is not a combatant in an active
		// encounter — treat as "not in combat" so callers silently skip.
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return encID, true, nil
}

// resumePlayerLookupAdapter bridges refdata.Queries to the
// discord.ResumePlayerLookup contract so the Phase 115 resume re-pinger can
// @mention the current-turn player by their Discord user id.
type resumePlayerLookupAdapter struct {
	queries *refdata.Queries
}

func (a resumePlayerLookupAdapter) GetPlayerCharacterByCharacter(ctx context.Context, campaignID, characterID uuid.UUID) (refdata.PlayerCharacter, error) {
	return a.queries.GetPlayerCharacterByCharacter(ctx, refdata.GetPlayerCharacterByCharacterParams{
		CampaignID:  campaignID,
		CharacterID: characterID,
	})
}

// dashboardCampaignLookup resolves the DM's first active/paused campaign by
// discord user id so the Pause/Resume button can carry a data-campaign-id and
// show the correct label. Returns ("","",nil) when no campaign exists; errors
// propagate so the caller can log + degrade.
type dashboardCampaignLookup struct {
	queries *refdata.Queries
}

func (l dashboardCampaignLookup) LookupActiveCampaign(ctx context.Context, dmUserID string) (string, string, error) {
	campaigns, err := l.queries.ListCampaigns(ctx)
	if err != nil {
		return "", "", err
	}
	for _, c := range campaigns {
		if c.DmUserID != dmUserID {
			continue
		}
		if c.Status == "archived" {
			continue
		}
		return c.ID.String(), c.Status, nil
	}
	return "", "", nil
}

// newDMQueueChannelResolver returns a closure that resolves a guild ID to
// the channel ID of its #dm-queue text channel by scanning the live
// discordgo session state. Phase 106a uses this to drive
// dmqueue.Notifier.Post — guilds without a #dm-queue channel return "" and
// notifier posts become silent no-ops.
func newDMQueueChannelResolver(session discord.Session) func(string) string {
	return func(guildID string) string {
		channels, err := session.GuildChannels(guildID)
		if err != nil {
			return ""
		}
		for _, ch := range channels {
			if ch.Name == "dm-queue" {
				return ch.ID
			}
		}
		return ""
	}
}

// passthroughMiddleware is a no-op HTTP middleware used as a fallback when
// Discord OAuth2 env vars are not configured (local dev without OAuth).
func passthroughMiddleware(next http.Handler) http.Handler { return next }

// buildAuthMiddleware returns a real auth.SessionMiddleware when
// DISCORD_CLIENT_ID and DISCORD_CLIENT_SECRET are set. Otherwise it falls
// back to passthroughMiddleware with a warning log for local dev.
func buildAuthMiddleware(db *sql.DB, logger *slog.Logger) func(http.Handler) http.Handler {
	clientID := os.Getenv("DISCORD_CLIENT_ID")
	clientSecret := os.Getenv("DISCORD_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		logger.Warn("DISCORD_CLIENT_ID or DISCORD_CLIENT_SECRET not set; dashboard auth disabled (passthrough)")
		return passthroughMiddleware
	}

	sessionStore := auth.NewSessionStore(db)
	oauthCfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://discord.com/api/oauth2/token",
		},
	}
	return auth.SessionMiddleware(sessionStore, oauthCfg, logger)
}

// buildDiscordSession constructs (but does NOT open) a Discord session using
// the given bot token. An empty token is treated as "Discord is optional for
// this deploy" and returns (nil, nil, nil). The raw *discordgo.Session is
// returned alongside the Session interface wrapper so run() can call Open()
// and Close() on it directly after crash recovery completes.
func buildDiscordSession(token string) (discord.Session, *discordgo.Session, error) {
	if token == "" {
		return nil, nil, nil
	}
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, nil, fmt.Errorf("discordgo.New: %w", err)
	}
	return &discord.DiscordgoSession{S: dg}, dg, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Stdout, ""); err != nil {
		os.Exit(1)
	}
}

// runConfig collects optional knobs that the e2e harness uses to substitute a
// fake Discord session and observe the constructed CommandRouter. Production
// startup leaves every field nil and run() falls back to env-derived defaults.
type runConfig struct {
	// session, when non-nil, replaces the discordgo session that run() would
	// otherwise build from DISCORD_BOT_TOKEN. The associated rawDG is kept nil
	// so run() does not try to Open() / Close() / register a real handler.
	session discord.Session
	// onRouterReady, when non-nil, is invoked exactly once after the
	// CommandRouter has been fully wired with every Phase 105 handler. The
	// e2e harness uses it to capture router.Handle and re-deliver injected
	// interactions through the fake session.
	onRouterReady func(*discord.CommandRouter)
}

// runOption mutates a runConfig. Defined as a function-typed option to keep
// the seam open for future flags (custom roller, custom clock, etc.).
type runOption func(*runConfig)

// withDiscordSession wires an externally-supplied discord.Session into run()
// in place of the env-derived one. Used by the Phase 120 e2e harness.
func withDiscordSession(s discord.Session) runOption {
	return func(c *runConfig) { c.session = s }
}

// withCommandRouterReady installs a callback that fires once the Phase 105
// CommandRouter is fully wired. The e2e harness uses it to capture
// router.Handle so the fake session can deliver injected interactions.
func withCommandRouterReady(cb func(*discord.CommandRouter)) runOption {
	return func(c *runConfig) { c.onRouterReady = cb }
}

// run starts the HTTP server and blocks until the context is cancelled.
// It returns nil on clean shutdown or an error if the server fails.
// Pass an empty addr to use the ADDR env var (defaulting to ":8080").
// If DATABASE_URL is set, it connects to PostgreSQL and runs migrations.
func run(ctx context.Context, logOutput io.Writer, addr string) error {
	return runWithOptions(ctx, logOutput, addr)
}

// runWithOptions is the option-aware variant of run. Production callers go
// through run(); the e2e harness reaches in here to substitute a fake Discord
// session (cfg.session) and capture the constructed CommandRouter
// (cfg.onRouterReady). All other behaviour matches run() so existing callers
// and tests see the same wiring.
func runWithOptions(ctx context.Context, logOutput io.Writer, addr string, opts ...runOption) error {
	var cfg runConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	if addr == "" {
		addr = os.Getenv("ADDR")
	}
	if addr == "" {
		addr = ":8080"
	}

	debug := os.Getenv("DEBUG") == "true"
	logger := server.NewLogger(logOutput, debug)

	router, health := server.NewRouter(logger)

	// Phase 112: error recorder + reader. Starts as an in-memory store so
	// panic recovery always has somewhere to land; upgraded to a PgStore
	// backed by error_log once DATABASE_URL is configured below.
	var errorStore errorlog.Store = errorlog.NewMemoryStore(nil)

	// Phase 104: Construct (but do NOT open) the Discord session up-front so
	// the wiring below can inject it into narration.Service,
	// messageplayer.Service, and the turn-timer notifier. Discord is optional
	// — if DISCORD_BOT_TOKEN is unset, session stays nil and the dependent
	// services fall back to their placeholder behavior. Per spec lines
	// 116-121, we must complete stale-state recovery BEFORE the bot starts
	// receiving gateway events, so session.Open() is deferred until after
	// the crash-recovery scan runs below.
	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	var (
		discordSession discord.Session
		rawDG          *discordgo.Session
		err            error
	)
	if cfg.session != nil {
		// Phase 120 e2e harness: an externally-supplied session bypasses
		// discordgo entirely. rawDG stays nil so the gateway open/close
		// path below is skipped, but every other wiring runs as in production.
		discordSession = cfg.session
		logger.Info("discord session injected (e2e harness)")
	} else {
		discordSession, rawDG, err = buildDiscordSession(discordToken)
		if err != nil {
			logger.Error("discord session construction failed", "error", err)
			return err
		}
	}
	if discordSession != nil {
		logger.Info("discord session constructed (open deferred until after recovery)")
	} else {
		logger.Info("discord session skipped (DISCORD_BOT_TOKEN not set)")
	}

	// Phase 104: Construct the dashboard hub up-front so the publisher can
	// be wired into combat.Service inside the DB block below. The hub has
	// its own goroutine; Stop() is called during shutdown.
	hub := dashboard.NewHub()
	go hub.Run()
	defer hub.Stop()

	// Optional database connection. Per spec lines 116-121, the correct
	// startup order is:
	//   1. Connect to PostgreSQL
	//   2. Run migrations
	//   3. Scan for stale state (TurnTimer.PollOnce)
	//   4. Open Discord gateway (dg.Open)
	//   5. Re-register slash commands for every guild
	//   6. Start the periodic timer ticker
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		db, err := database.Connect(databaseURL)
		if err != nil {
			logger.Error("database connection failed", "error", err)
			return err
		}
		defer db.Close()

		if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
			logger.Error("database migration failed", "error", err)
			return err
		}
		logger.Info("database connected and migrated")

		// Phase 119: upgrade the error store to a PgStore backed by the
		// dedicated error_log table. The PgStore both records errors and
		// drives the dashboard badge + panel.
		if pg := errorlog.NewPgStore(db); pg != nil {
			errorStore = pg
		}

		// Phase 112: wire the DB ping into the health endpoint.
		health.Register("db", server.NewDBChecker(db))

		// Wire map API handler
		queries := refdata.New(db)
		mapSvc := gamemap.NewService(queries)
		mapHandler := gamemap.NewHandler(mapSvc)
		mapHandler.RegisterRoutes(router)

		// Wire encounter API handler
		encounterSvc := encounter.NewService(queries)
		encounterHandler := encounter.NewHandler(encounterSvc)
		encounterHandler.RegisterRoutes(router)

		// Wire asset API handler
		assetDataDir := os.Getenv("ASSET_DATA_DIR")
		if assetDataDir == "" {
			assetDataDir = "data/assets"
		}
		assetStore := asset.NewLocalStore(assetDataDir)
		assetSvc := asset.NewService(queries, assetStore)
		assetHandler := asset.NewHandler(assetSvc)
		assetHandler.RegisterRoutes(router)

		// Wire Stat Block Library API handler (Phase 98).
		// Phase 111: inject an Open5e campaign-lookup so the library
		// applies per-campaign open5e_sources gating when a campaign_id
		// accompanies the request.
		statBlockSvc := statblocklibrary.NewService(queries)
		open5eCampaignLookup := open5e.NewCampaignSourceLookup(queries)
		statBlockHandler := statblocklibrary.NewHandlerWithCampaignLookup(statBlockSvc, open5eCampaignLookup)
		statBlockHandler.RegisterRoutes(router)

		// Phase 111: Open5e extended-content integration. Live search
		// proxy + on-demand cache into creatures/spells tables. Sources
		// are gated per campaign via the statblocklibrary filter above.
		open5eClient := open5e.NewClient("", nil)
		open5eCache := open5e.NewCache(queries)
		open5eSvc := open5e.NewService(open5eClient, open5eCache)
		open5eHandler := open5e.NewHandler(open5eSvc)
		open5eHandler.RegisterRoutes(router)

		// Wire Homebrew Content API handler (Phase 99)
		homebrewSvc := homebrew.NewService(queries)
		homebrewHandler := homebrew.NewHandler(homebrewSvc)
		homebrewHandler.RegisterRoutes(router)

		// Wire Narration API handler (Phase 100a). Phase 104: inject the
		// Discord narration poster if a session is available; otherwise
		// fall back to nil so narration.Service surfaces
		// ErrPosterUnavailable at runtime (matches "Discord optional" mode).
		var narrationPoster narration.Poster
		if discordSession != nil {
			narrationPoster = discord.NewNarrationPoster(discordSession)
		}
		// Phase 115: inject the Discord-backed campaign announcer so
		// pause/resume post to #the-story; falls back to nil when the bot
		// is offline (messages are then silently skipped per service
		// "best-effort" contract).
		var campaignAnnouncer campaign.Announcer
		if discordSession != nil {
			campaignAnnouncer = discord.NewCampaignAnnouncer(discordSession)
		}
		campaignSvc := campaign.NewService(queries, campaignAnnouncer)
		campaignHandler := campaign.NewHandler(campaignSvc)
		campaignHandler.RegisterRoutes(router)
		narrationStore := narration.NewDBStore(queries)
		narrationAssets := narration.NewAssetAttachmentResolver(assetSvc)
		narrationCampaigns := narration.NewCampaignResolverAdapter(campaignSvc)
		narrationSvc := narration.NewService(narrationStore, narrationPoster, narrationAssets, narrationCampaigns)
		narrationHandler := narration.NewHandler(narrationSvc)
		narrationHandler.RegisterRoutes(router)

		// Wire Narration Template API handler (Phase 100b).
		narrationTemplateStore := narration.NewTemplateDBStore(queries)
		narrationTemplateSvc := narration.NewTemplateService(narrationTemplateStore)
		narrationTemplateHandler := narration.NewTemplateHandler(narrationTemplateSvc)
		narrationTemplateHandler.RegisterRoutes(router)

		// Wire Character Overview API handler (Phase 101).
		characterOverviewStore := characteroverview.NewDBStore(queries)
		characterOverviewSvc := characteroverview.NewService(characterOverviewStore)
		characterOverviewHandler := characteroverview.NewHandler(characterOverviewSvc)
		characterOverviewHandler.RegisterRoutes(router)

		// Wire Message Player API handler (Phase 101). Phase 104: inject a
		// real Discord direct messenger when a session is available.
		var directMessenger messageplayer.Messenger
		if discordSession != nil {
			directMessenger = discord.NewDirectMessenger(discordSession)
		}
		messagePlayerStore := messageplayer.NewDBStore(queries)
		messagePlayerLookup := messageplayer.NewPlayerLookupAdapter(queries)
		messagePlayerSvc := messageplayer.NewService(messagePlayerStore, messagePlayerLookup, directMessenger)
		messagePlayerHandler := messageplayer.NewHandler(messagePlayerSvc)
		messagePlayerHandler.RegisterRoutes(router)

		// Phase 104: Dashboard publisher + combat service wiring.
		// The publisher is injected into combat.Service so every HP /
		// condition / status mutation fans a fresh snapshot out to
		// WebSocket subscribers. The publisher is attached BEFORE the
		// combat handler is mounted so no mutation can land without the
		// publisher wired in.
		snapshotBuilder := dashboard.NewSnapshotBuilder(queries, time.Now)
		publisher := dashboard.NewPublisher(hub, snapshotBuilder)
		combatStore := combat.NewStoreAdapter(queries)
		combatSvc := combat.NewService(combatStore)
		combatSvc.SetPublisher(publisher)
		combatHandler := combat.NewHandler(combatSvc, dice.NewRoller(nil))
		combatHandler.RegisterRoutes(router)

		// Phase 115: wire the resume-time turn re-pinger so /api/campaigns/:id/resume
		// automatically @mentions the current-turn player in #your-turn when
		// the paused campaign had mid-combat state. No-op without a Discord
		// session (unit tests cover the per-branch decisions).
		if discordSession != nil {
			playerLookup := resumePlayerLookupAdapter{queries: queries}
			resumePinger := discord.NewResumeTurnPinger(discordSession, combatSvc, playerLookup)
			campaignSvc.SetTurnPinger(resumePinger)
		}

		// Phase 106a: DM Notification System wiring. Construct a Notifier
		// backed by the dm_queue_items PgStore and (when a Discord session is
		// available) a SessionSender that posts directly via discordgo. The
		// channel resolver looks up #dm-queue per guild from cached session
		// state. The notifier is shared by combat freeform actions, the rest
		// handler, and any future event producer. The dashboard resolver
		// page is mounted regardless of Discord availability so DMs can still
		// inspect persisted items in headless deploys.
		dmQueueStore := dmqueue.NewPgStore(queries)
		var dmQueueSender dmqueue.Sender
		dmQueueChannel := func(string) string { return "" }
		if discordSession != nil {
			dmQueueSender = dmqueue.NewSessionSender(discordSession)
			dmQueueChannel = newDMQueueChannelResolver(discordSession)
		}
		dmQueueNotifier := dmqueue.NewNotifierWithStore(
			dmQueueSender,
			dmQueueChannel,
			func(itemID string) string { return "/dashboard/queue/" + itemID },
			dmQueueStore,
		)
		combatSvc.SetDMNotifier(dmQueueNotifier)
		if discordSession != nil {
			dmQueueNotifier.SetWhisperDeliverer(discord.NewDirectMessenger(discordSession))
			// Phase 106d: deliver /check skill-check narrations as
			// non-ephemeral follow-ups in the originating channel.
			dmQueueNotifier.SetSkillCheckNarrationDeliverer(discord.NewSkillCheckNarrationDeliverer(discordSession))
		}

		// Phase 106f: Build real auth middleware when Discord OAuth2 env
		// vars are available; otherwise fall back to passthrough for local
		// dev without OAuth.
		authMw := buildAuthMiddleware(db, logger)
		dashboard.RegisterDMQueueRoutes(router, logger, dmQueueNotifier, authMw)

		// Phase 112: DM dashboard errors panel. Mount the /dashboard/errors
		// page behind authMw and (when a top-level Handler is present, via
		// MountDashboard in future phases) wire the sidebar 24h badge off
		// the same errorlog.Reader.
		dashHandler := dashboard.MountDashboard(router, logger, hub, authMw)
		dashboard.MountErrorsRoutes(router, dashHandler, errorStore, time.Now, authMw)

		// Phase 115: drive the Pause/Resume button label + data-campaign-id
		// off the DM's current campaign.
		dashHandler.SetCampaignLookup(dashboardCampaignLookup{queries: queries})

		// Phase 110: exploration dashboard (Q4a). Mount behind authMw so the
		// page is only reachable to authenticated DMs. Queries directly
		// satisfy exploration.Store and exploration.MapLister.
		explorationSvc := exploration.NewService(queries)
		explorationHandler := exploration.NewDashboardHandler(explorationSvc, queries)
		dashboard.RegisterExplorationRoutes(router, explorationHandler, authMw)

		// Phase 104b: Publisher fan-out to non-combat services that can
		// mutate an active encounter's combatant state mid-combat. The
		// encounter lookup resolves "which active encounter (if any) is this
		// character currently in?" so each service can skip publishing when
		// the mutation doesn't touch live combat state.
		encLookup := encounterLookupAdapter{queries: queries}
		inventoryAPIHandler := inventory.NewAPIHandler(queries)
		inventoryAPIHandler.SetPublisher(publisher, encLookup)
		dashboard.RegisterInventoryAPI(router, inventoryAPIHandler, authMw)

		// Phase 104c: Level-up handler. DB-backed store/class adapters plus
		// a DM-only notifier (public announcements deferred) share the
		// publisher/lookup used by combat and inventory. SetPublisher runs
		// before RegisterRoutes so no mutation can land without fan-out.
		var levelUpDM levelup.DirectMessenger
		if discordSession != nil {
			levelUpDM = discord.NewDirectMessenger(discordSession)
		}
		levelUpSvc := levelup.NewService(
			levelup.NewCharacterStoreAdapter(queries),
			levelup.NewClassStoreAdapter(queries),
			levelup.NewNotifierAdapter(levelUpDM),
		)
		levelUpSvc.SetPublisher(publisher, encLookup)
		levelup.NewHandler(levelUpSvc, hub).RegisterRoutes(router)

		// Phase 104: Startup recovery per spec lines 116-121.
		//
		// Step 3 — Scan for stale state. PollOnce runs synchronously BEFORE
		// the Discord gateway is opened so any overdue turns (nudge,
		// warning, DM prompt, auto-resolve) are processed in deadline order
		// while the bot is still "dark" — no new interactions can race with
		// recovery. Notifier is the Discord adapter when a session is
		// available; otherwise a no-op notifier.
		var timerNotifier combat.Notifier
		if discordSession != nil {
			timerNotifier = discord.NewTurnTimerNotifier(discordSession)
		} else {
			timerNotifier = noopNotifier{}
		}
		timer := combat.NewTurnTimer(combatStore, timerNotifier, 30*time.Second)
		// Phase 118: wire the concentration save resolver so failed CON
		// saves rolled by AutoResolveTurn fire the cleanup pipeline.
		timer.SetConcentrationResolver(func(ctx context.Context, ps refdata.PendingSafe) error {
			_, err := combatSvc.ResolveConcentrationSave(ctx, ps)
			return err
		})
		if err := timer.PollOnce(ctx); err != nil {
			logger.Error("startup stale-turn scan failed", "error", err)
		} else {
			logger.Info("startup stale-turn scan complete")
		}

		// Step 4 — Open the Discord gateway. Only after recovery. The fake
		// session injected by the e2e harness leaves rawDG nil so this
		// block is skipped while still letting the slash-command router
		// wiring below run unchanged.
		if rawDG != nil {
			if err := rawDG.Open(); err != nil {
				logger.Error("discord session open failed", "error", err)
				return fmt.Errorf("discord session open: %w", err)
			}
			defer rawDG.Close()
			logger.Info("discord session opened")

			// Phase 112: wire the Discord gateway into the health endpoint.
			// DataReady is toggled by discordgo on every Ready / Resumed
			// event and is the gateway-level liveness signal discordgo
			// itself publishes.
			health.Register("discord", server.NewDiscordChecker(func() bool { return rawDG.DataReady }))
		}

		// Step 5 — Re-register slash commands for every guild the
		// session is currently in. Per spec lines 178-181, the bot
		// must always reconcile its command set on startup. The same
		// CommandRouter wiring runs whether the session is the real
		// discordgo bot or the Phase 120 e2e fake; only the AddHandler /
		// RegisterAllGuilds gateway hooks are skipped for the fake.
		if discordSession != nil {
			appID := os.Getenv("DISCORD_APPLICATION_ID")
			bot := discord.NewBot(discordSession, appID, logger)

			// Phase 105b: Construct every Phase 105 slash-command handler
			// with the per-user encounter resolver injected, wire them into
			// a CommandRouter, and register the router as the discordgo
			// InteractionCreate callback so /move, /fly, /distance, /done,
			// /check, /save, /rest, /command (summon), and /recap all route
			// to the invoker's own encounter when two simultaneous
			// encounters share a channel.
			discordHandlerSet := buildDiscordHandlers(discordHandlerDeps{
				session:                  discordSession,
				queries:                  queries,
				combatService:            combatSvc,
				roller:                   dice.NewRoller(nil),
				resolver:                 newDiscordUserEncounterResolver(queries),
				enemyTurnEncounterLookup: combatSvc,
			})
			cmdRouter := discord.NewCommandRouter(bot, nil)
			// Phase 112: wire panic recovery + error recorder so any handler
			// panic is caught, converted into a friendly ephemeral, logged at
			// ERROR level, and recorded for the DM dashboard badge / panel.
			cmdRouter.SetErrorRecorder(errorStore)
			attachPhase105Handlers(cmdRouter, discordHandlerSet)
			// Phase 106a: route /rest dm-queue posts through the notifier so
			// rest requests are persisted and resolvable from the dashboard.
			discordHandlerSet.rest.SetNotifier(dmQueueNotifier)
			// Phase 106d: gate non-trivial /check rolls through #dm-queue.
			discordHandlerSet.check.SetNotifier(dmQueueNotifier)
			// Phase 106c: route /reaction declarations through the dm-queue
			// notifier so each declaration is posted to #dm-queue and the
			// player can cancel it before the trigger fires.
			discordHandlerSet.reaction.SetNotifier(dmQueueNotifier)
			// Phase 106e: route /use consumable posts through the dm-queue
			// notifier so consumable usage is persisted and resolvable from
			// the dashboard.
			discordHandlerSet.use.SetNotifier(dmQueueNotifier)
			// Phase 109: route /whisper messages through the dm-queue
			// notifier so whispers are posted to #dm-queue for DM resolution.
			discordHandlerSet.whisper.SetNotifier(dmQueueNotifier)
			// Phase 110a: route /action freeform posts and exploration
			// cancels through the dm-queue notifier so every freeform
			// action (combat or exploration) lands in #dm-queue and the
			// player can cancel it before the DM resolves.
			discordHandlerSet.action.SetNotifier(dmQueueNotifier)
			if rawDG != nil {
				rawDG.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
					cmdRouter.Handle(i.Interaction)
				})

				if state := discordSession.GetState(); state != nil {
					guildIDs := make([]string, 0, len(state.Guilds))
					for _, g := range state.Guilds {
						guildIDs = append(guildIDs, g.ID)
					}
					if errs := bot.RegisterAllGuilds(guildIDs); len(errs) > 0 {
						logger.Warn("some guild command registrations failed", "count", len(errs))
					}
				}
			}

			// Phase 120 seam: hand the constructed router to the e2e harness
			// (or any other test caller) so it can deliver injected
			// interactions through cmdRouter.Handle. No-op in production.
			if cfg.onRouterReady != nil {
				cfg.onRouterReady(cmdRouter)
			}
		}

		// Step 6 — Start the periodic timer ticker. This runs LAST so that
		// the ticker-driven poll loop cannot fire while we are still
		// reconciling slash commands.
		timer.Start()
		defer timer.Stop()
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server listen error", "error", err)
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
		return err
	}

	// Wait for ListenAndServe goroutine to finish
	<-errCh
	return nil
}
