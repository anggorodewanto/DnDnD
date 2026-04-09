package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/asset"
	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/characteroverview"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dashboard"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/encounter"
	"github.com/ab/dndnd/internal/gamemap"
	"github.com/ab/dndnd/internal/homebrew"
	"github.com/ab/dndnd/internal/messageplayer"
	"github.com/ab/dndnd/internal/narration"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/server"
	"github.com/ab/dndnd/internal/statblocklibrary"
)

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
		combatStore := &combatStoreAdapter{queries}
		combatSvc := combat.NewService(combatStore)
		combatSvc.SetPublisher(publisher)
		combatHandler := combat.NewHandler(combatSvc, dice.NewRoller(nil))
		combatHandler.RegisterRoutes(router)

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
