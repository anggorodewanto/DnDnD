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
	"github.com/ab/dndnd/internal/exploration"
	"github.com/ab/dndnd/internal/gamemap"
	"github.com/ab/dndnd/internal/homebrew"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/levelup"
	"github.com/ab/dndnd/internal/messageplayer"
	"github.com/ab/dndnd/internal/narration"
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

// run starts the HTTP server and blocks until the context is cancelled.
// It returns nil on clean shutdown or an error if the server fails.
// Pass an empty addr to use the ADDR env var (defaulting to ":8080").
// If DATABASE_URL is set, it connects to PostgreSQL and runs migrations.
func run(ctx context.Context, logOutput io.Writer, addr string) error {
	if addr == "" {
		addr = os.Getenv("ADDR")
	}
	if addr == "" {
		addr = ":8080"
	}

	debug := os.Getenv("DEBUG") == "true"
	logger := server.NewLogger(logOutput, debug)

	router, _ := server.NewRouter(logger)

	// Phase 104: Construct (but do NOT open) the Discord session up-front so
	// the wiring below can inject it into narration.Service,
	// messageplayer.Service, and the turn-timer notifier. Discord is optional
	// — if DISCORD_BOT_TOKEN is unset, session stays nil and the dependent
	// services fall back to their placeholder behavior. Per spec lines
	// 116-121, we must complete stale-state recovery BEFORE the bot starts
	// receiving gateway events, so session.Open() is deferred until after
	// the crash-recovery scan runs below.
	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	discordSession, rawDG, err := buildDiscordSession(discordToken)
	if err != nil {
		logger.Error("discord session construction failed", "error", err)
		return err
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

		// Wire Stat Block Library API handler (Phase 98)
		statBlockSvc := statblocklibrary.NewService(queries)
		statBlockHandler := statblocklibrary.NewHandler(statBlockSvc)
		statBlockHandler.RegisterRoutes(router)

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
		campaignSvc := campaign.NewService(queries, nil)
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
		if err := timer.PollOnce(ctx); err != nil {
			logger.Error("startup stale-turn scan failed", "error", err)
		} else {
			logger.Info("startup stale-turn scan complete")
		}

		// Step 4 — Open the Discord gateway. Only after recovery.
		if rawDG != nil {
			if err := rawDG.Open(); err != nil {
				logger.Error("discord session open failed", "error", err)
				return fmt.Errorf("discord session open: %w", err)
			}
			defer rawDG.Close()
			logger.Info("discord session opened")

			// Step 5 — Re-register slash commands for every guild the
			// session is currently in. Per spec lines 178-181, the bot
			// must always reconcile its command set on startup.
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
