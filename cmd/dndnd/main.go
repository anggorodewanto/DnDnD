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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/bwmarrin/discordgo"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/asset"
	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/charactercard"
	"github.com/ab/dndnd/internal/characteroverview"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dashboard"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/ddbimport"
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
	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/magicitem"
	"github.com/ab/dndnd/internal/messageplayer"
	"github.com/ab/dndnd/internal/narration"
	"github.com/ab/dndnd/internal/open5e"
	"github.com/ab/dndnd/internal/portal"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/registration"
	"github.com/ab/dndnd/internal/rest"
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

// IsDM satisfies dashboard.DMVerifier (F-2) by reusing LookupActiveCampaign:
// a non-empty id means at least one non-archived campaign is owned by the
// user, which is the per-request DM verification spec line 65 calls for.
// Errors propagate so the middleware degrades to 403 — a transient DB blip
// must not silently grant access.
func (l dashboardCampaignLookup) IsDM(ctx context.Context, dmUserID string) (bool, error) {
	if dmUserID == "" {
		return false, nil
	}
	id, _, err := l.LookupActiveCampaign(ctx, dmUserID)
	if err != nil {
		return false, err
	}
	return id != "", nil
}

// approvalsCounter adapts dashboard.ApprovalStore.ListPendingApprovals to
// dashboard.PendingApprovalsCounter for the Campaign Home pending-approvals
// card (med-40 / Phase 15).
type approvalsCounter struct {
	store dashboard.ApprovalStore
}

func (a approvalsCounter) CountPendingApprovals(ctx context.Context, campaignID uuid.UUID) (int, error) {
	entries, err := a.store.ListPendingApprovals(ctx, campaignID)
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

// dmQueueCounter adapts dmqueue.PgStore.ListPendingForCampaign to
// dashboard.DMQueueCounter for the Campaign Home dm-queue card (med-40).
type dmQueueCounter struct {
	store *dmqueue.PgStore
}

func (d dmQueueCounter) CountPendingDMQueue(ctx context.Context, campaignID uuid.UUID) (int, error) {
	items, err := d.store.ListPendingForCampaign(ctx, campaignID)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

// charCreateRefData adapts portal.RefDataAdapter to the narrower
// dashboard.RefDataForCreate interface (which omits the per-campaign Open5e
// gating that the portal flow exposes via an extra campaignID arg).
type charCreateRefData struct {
	a *portal.RefDataAdapter
}

func (c charCreateRefData) ListRaces(ctx context.Context) ([]portal.RaceInfo, error) {
	return c.a.ListRaces(ctx)
}

func (c charCreateRefData) ListClasses(ctx context.Context) ([]portal.ClassInfo, error) {
	return c.a.ListClasses(ctx)
}

func (c charCreateRefData) ListEquipment(ctx context.Context) ([]portal.EquipmentItem, error) {
	return c.a.ListEquipment(ctx)
}

func (c charCreateRefData) ListSpellsByClass(ctx context.Context, class string) ([]portal.SpellInfo, error) {
	return c.a.ListSpellsByClass(ctx, class, "")
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

// combatDashboardWiring is the constructed router-mount surface for the combat
// dashboard route block (G-94a workspace + G-95/97a/97b DM dashboard). The
// poster is exposed so the wiring test can assert Phase 97b's CombatLogPoster
// was threaded through to NewDMDashboardHandlerWithDeps without poking at
// unexported handler fields.
type combatDashboardWiring struct {
	handler *combat.DMDashboardHandler
	poster  combat.CombatLogPoster
}

// workspaceStoreAdapter bridges *refdata.Queries to combat.WorkspaceStore.
// The interface names the combatant lookup GetCombatantByID; refdata calls it
// GetCombatant. Every other method is structurally compatible.
type workspaceStoreAdapter struct {
	*refdata.Queries
}

func (a workspaceStoreAdapter) GetCombatantByID(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return a.Queries.GetCombatant(ctx, id)
}

// mountCombatDashboardRoutes wires the workspace + DM dashboard HTTP surface
// onto router. workspaceStore may be nil to skip the WorkspaceHandler (test
// deploys); db may be nil to disable per-turn advisory locks in unit tests.
//
// WorkspaceHandler, DMDashboardHandler, and combat.Handler all open their
// own r.Route("/api/combat", ...) inside RegisterRoutes, which chi rejects as
// a duplicate mount. To keep all three reachable on the same router this
// helper bypasses the RegisterRoutes wrappers and binds each method directly
// onto the shared router (combat.Handler still owns its own /api/combat
// route group on line 612 because that path was wired first and stays the
// canonical mount for /start /end /reactions etc).
//
// Returns the constructed DMDashboardHandler so tests can verify the
// CombatLogPoster (Phase 97b) is threaded into NewDMDashboardHandlerWithDeps.
func mountCombatDashboardRoutes(
	router chi.Router,
	svc *combat.Service,
	workspaceStore combat.WorkspaceStore,
	db combat.TxBeginner,
	poster combat.CombatLogPoster,
) combatDashboardWiring {
	if workspaceStore != nil {
		wh := combat.NewWorkspaceHandler(workspaceStore)
		router.Get("/api/combat/workspace", wh.GetWorkspace)
		router.Patch("/api/combat/{encounterID}/combatants/{combatantID}/hp", wh.UpdateCombatantHP)
		router.Patch("/api/combat/{encounterID}/combatants/{combatantID}/conditions", wh.UpdateCombatantConditions)
		router.Patch("/api/combat/{encounterID}/combatants/{combatantID}/position", wh.UpdateCombatantPosition)
		router.Delete("/api/combat/{encounterID}/combatants/{combatantID}", wh.DeleteCombatant)
	}
	dm := combat.NewDMDashboardHandlerWithDeps(svc, db, poster)
	router.Post("/api/combat/{encounterID}/advance-turn", dm.AdvanceTurn)
	router.Get("/api/combat/{encounterID}/pending-actions", dm.ListPendingActions)
	router.Post("/api/combat/{encounterID}/pending-actions/{actionID}/resolve", dm.ResolvePendingAction)
	router.Get("/api/combat/{encounterID}/action-log", dm.ListActionLogViewer)
	router.Post("/api/combat/{encounterID}/undo-last-action", dm.UndoLastAction)
	router.Post("/api/combat/{encounterID}/override/combatant/{combatantID}/hp", dm.OverrideCombatantHP)
	router.Post("/api/combat/{encounterID}/override/combatant/{combatantID}/position", dm.OverrideCombatantPosition)
	router.Post("/api/combat/{encounterID}/override/combatant/{combatantID}/conditions", dm.OverrideCombatantConditions)
	router.Post("/api/combat/{encounterID}/override/combatant/{combatantID}/initiative", dm.OverrideCombatantInitiative)
	router.Post("/api/combat/{encounterID}/override/character/{characterID}/spell-slots", dm.OverrideCharacterSpellSlots)
	// C-35: per-attack DM advantage/disadvantage override.
	router.Post("/api/combat/{encounterID}/override/combatant/{combatantID}/advantage", dm.OverrideCombatantNextAttackAdvantage)
	router.Post("/api/combat/{encounterID}/combatants/{combatantID}/concentration/drop", dm.DropConcentration)
	return combatDashboardWiring{handler: dm, poster: poster}
}

// registrationDepsConfig collects the inputs that buildRegistrationDeps
// assembles into a discord.RegistrationDeps. Pulled into a struct so the
// test (TestBuildRegistrationDeps_CarriesDDBImporter) can target the
// DDBImporter wiring without setting up the full DB-backed inputs.
type registrationDepsConfig struct {
	regService    discord.RegistrationService
	campaignProv  discord.CampaignProvider
	charCreator   discord.CharacterCreator
	dmQueueFunc   func(string) string
	dmUserFunc    func(string) string
	tokenFunc     func(uuid.UUID, string) (string, error)
	nameResolver  discord.CharacterNameResolver
	ddbImporter   discord.DDBImporter
	portalBaseURL string
}

// buildRegistrationDeps assembles the RegistrationDeps used by the Phase
// 120a command router, threading the DDB importer through so /import lands
// on the real ddbimport service instead of handlePlaceholderImport (G-90).
// PortalBaseURL is forwarded so /create-character emits links rooted at the
// operator-configured BASE_URL instead of the hard-coded production host
// (A-14).
func buildRegistrationDeps(cfg registrationDepsConfig) *discord.RegistrationDeps {
	return &discord.RegistrationDeps{
		RegService:    cfg.regService,
		CampaignProv:  cfg.campaignProv,
		CharCreator:   cfg.charCreator,
		DMQueueFunc:   cfg.dmQueueFunc,
		DMUserFunc:    cfg.dmUserFunc,
		TokenFunc:     cfg.tokenFunc,
		NameResolver:  cfg.nameResolver,
		DDBImporter:   cfg.ddbImporter,
		PortalBaseURL: cfg.portalBaseURL,
	}
}

// authBundle bundles the session middleware and the OAuth2 service so the
// caller can both protect dashboard routes and mount the
// /portal/auth/{login,callback,logout} endpoints from a single construction.
// oauthSvc is nil when DISCORD_CLIENT_ID / DISCORD_CLIENT_SECRET are unset
// (local dev without OAuth) — middleware then falls back to passthrough.
type authBundle struct {
	middleware func(http.Handler) http.Handler
	oauthSvc   *auth.OAuthService
}

// buildAuth wires the session middleware and (when OAuth env vars are
// present) the *auth.OAuthService that backs /portal/auth/*. The redirect URL
// defaults to BASE_URL + /portal/auth/callback (BASE_URL itself defaults to
// http://localhost:8080) so a localhost playtest works without extra config.
// Override OAUTH_REDIRECT_URL directly if a reverse proxy fronts the bot.
func buildAuth(db *sql.DB, logger *slog.Logger) authBundle {
	clientID := os.Getenv("DISCORD_CLIENT_ID")
	clientSecret := os.Getenv("DISCORD_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		logger.Warn("DISCORD_CLIENT_ID or DISCORD_CLIENT_SECRET not set; dashboard auth disabled (passthrough)")
		return authBundle{middleware: passthroughMiddleware}
	}

	redirectURL := os.Getenv("OAUTH_REDIRECT_URL")
	if redirectURL == "" {
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		redirectURL = baseURL + "/portal/auth/callback"
	}

	sessionStore := auth.NewSessionStore(db)
	oauthCfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"identify"},
		Endpoint:     endpoints.Discord,
	}

	// secure=false locally so the session cookie works over plain HTTP.
	// Production should front the bot with TLS (BASE_URL=https://…) and set
	// COOKIE_SECURE=true.
	secure := os.Getenv("COOKIE_SECURE") == "true"
	userFetcher := &auth.DiscordUserInfoFetcher{}
	oauthSvc := auth.NewOAuthService(oauthCfg, sessionStore, userFetcher, logger, secure)
	return authBundle{
		middleware: auth.SessionMiddleware(sessionStore, oauthCfg, logger),
		oauthSvc:   oauthSvc,
	}
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

	// Phase 9b wiring (high-14): wrap the live discord session in a
	// MessageQueue-backed adapter so every ChannelMessageSend flows through
	// per-channel FIFO + 429 retry/backoff. Without this wrapper production
	// outbound traffic bypasses the queue and a single rate-limited channel
	// can starve unrelated channels. The queue is stopped during shutdown.
	var messageQueue *discord.MessageQueue
	if discordSession != nil {
		messageQueue = discord.NewMessageQueue(discordSession)
		defer messageQueue.Stop()
		discordSession = newQueueingSession(discordSession, messageQueue)
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

		// Seed SRD reference data (idempotent upserts) so the dashboard
		// character-create form, slash-command spell pickers, and stat-block
		// library have data to work with on a fresh database. ~900 row
		// upserts; set SKIP_SRD_SEED=true once the DB is known-seeded to
		// shave a second or two off boot.
		if os.Getenv("SKIP_SRD_SEED") == "true" {
			logger.Info("SRD reference data seed skipped (SKIP_SRD_SEED=true)")
		} else {
			if err := refdata.SeedAll(ctx, db); err != nil {
				logger.Error("SRD reference data seed failed", "error", err)
				return err
			}
			logger.Info("SRD reference data seeded")
		}

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
			// When deployed on Fly, the fly.toml volume mount is /data
			// (see [mounts] block). Default to /data/assets there so
			// uploaded assets survive machine restarts. Locally we fall
			// back to a relative ./data/assets directory.
			if os.Getenv("FLY_APP_NAME") != "" {
				assetDataDir = "/data/assets"
			} else {
				assetDataDir = "data/assets"
			}
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

		// Phase 22 wiring (high-10): the campaign-settings provider resolves
		// encounter-id -> channel_ids map for the DM combat-log poster
		// (G-97b) and is reused by the Discord slash-command handlers below.
		// Constructed here (before the Discord session block) so the DM
		// dashboard handler can post corrections even when the bot is offline.
		campaignSettingsProvider := discord.NewDefaultCampaignSettingsProvider(
			func(ctx context.Context, encounterID uuid.UUID) (refdata.Campaign, error) {
				return queries.GetCampaignByEncounterID(ctx, encounterID)
			},
		)

		// G-97b: DM correction poster. Best-effort — nil session means
		// corrections silently drop, matching the cardPoster pattern.
		var combatLogPoster combat.CombatLogPoster
		if discordSession != nil {
			combatLogPoster = discord.NewDMCorrectionPoster(discordSession, campaignSettingsProvider)
		}

		// G-94a + G-95: mount the combat workspace (G-94a) and DM dashboard
		// (G-95/97a) routes so /api/combat/workspace, /advance-turn,
		// /pending-actions, /action-log, undo, and the override family all
		// stop returning 404 in production.
		mountCombatDashboardRoutes(router, combatSvc, workspaceStoreAdapter{queries}, db, combatLogPoster)

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
		// dev without OAuth. The bundle also exposes the OAuthService for
		// /portal/auth/* mounting below; oauthSvc is nil in passthrough mode.
		authBundle := buildAuth(db, logger)
		authMw := authBundle.middleware

		// F-2: per-request DM verification (docs/dnd-async-discord-spec.md
		// line 65). dmAuthMw chains the session middleware with RequireDM so
		// every DM-only route rejects requests from authenticated
		// non-DM users (403 JSON) instead of merely rendering an empty UI.
		// dashboardCampaignLookup satisfies dashboard.DMVerifier via its IsDM
		// method. The verifier is nil in passthrough-auth mode so the local
		// dev path (DISCORD_CLIENT_ID unset) keeps working without OAuth.
		var dmVerifier dashboard.DMVerifier
		if authBundle.oauthSvc != nil {
			dmVerifier = dashboardCampaignLookup{queries: queries}
		}
		dmRequire := dashboard.RequireDM(dmVerifier)
		dmAuthMw := func(next http.Handler) http.Handler {
			return authMw(dmRequire(next))
		}

		dashboard.RegisterDMQueueRoutes(router, logger, dmQueueNotifier, dmAuthMw)

		// Phase 112: DM dashboard errors panel. Mount the /dashboard/errors
		// page behind dmAuthMw (F-2) and (when a top-level Handler is
		// present, via MountDashboard in future phases) wire the sidebar
		// 24h badge off the same errorlog.Reader.
		dashHandler := dashboard.MountDashboard(router, logger, hub, dmAuthMw)
		dashboard.MountErrorsRoutes(router, dashHandler, errorStore, time.Now, dmAuthMw)

		// Phase 115: drive the Pause/Resume button label + data-campaign-id
		// off the DM's current campaign.
		dashHandler.SetCampaignLookup(dashboardCampaignLookup{queries: queries})

		// med-39 / Phase 21a: GET /api/me returns the authenticated DM's
		// active campaign id so the Svelte SPA can replace its hard-coded
		// placeholder UUID with the real per-DM campaign id on boot. This
		// route stays on the session-only authMw (NOT dmAuthMw) because the
		// SPA fetches it before the user is known to be a DM — the response
		// itself carries the empty campaign_id that the UI uses to detect
		// the "not a DM" state.
		dashboard.RegisterMeRoute(router, dashboard.NewMeHandler(logger, dashboardCampaignLookup{queries: queries}), authMw)

		// Phase 110: exploration dashboard (Q4a). Mount behind dmAuthMw
		// (F-2) so non-DM authenticated users get 403 instead of an empty UI.
		explorationSvc := exploration.NewService(queries)
		explorationHandler := exploration.NewDashboardHandler(explorationSvc, queries)
		dashboard.RegisterExplorationRoutes(router, explorationHandler, dmAuthMw)

		// Phase 121: DM character-create form. charCreateRefData drops the
		// per-campaign Open5e gating arg because the DM-side form is
		// campaign-agnostic at construction time.
		charCreateRefAdapter := portal.NewRefDataAdapter(queries)
		charCreateStore := portal.NewBuilderStoreAdapter(queries, nil)
		charCreateSvc := dashboard.NewDMCharCreateService(charCreateStore)
		charCreateHandler := dashboard.NewCharCreateHandler(logger, charCreateSvc, charCreateRefData{a: charCreateRefAdapter})
		charCreateHandler.RegisterCharCreateRoutes(router.With(dmAuthMw))

		// Phase 121: character approval queue. SetCampaignLookup reuses the
		// same lookup the Pause/Resume button uses, so the handler serves
		// every DM from a single instance. cardPoster is nil when the bot
		// is offline (#character-cards posts then become silent no-ops).
		// PlayerNotifier wraps the existing direct-messenger when a Discord
		// session is available; otherwise it stays nil and the handler
		// silently skips the player DM (matches the cardPoster pattern).
		approvalStore := dashboard.NewDBApprovalStore(queries)
		// med-40 / Phase 15: Campaign Home cards now show live counts
		// instead of the historical 0 placeholders.
		dashHandler.SetCounters(
			approvalsCounter{store: approvalStore},
			dmQueueCounter{store: dmQueueStore},
		)
		var cardPoster dashboard.CharacterCardPoster
		if discordSession != nil {
			cardSvc := charactercard.NewService(discordSession, queries, logger)
			cardPoster = cardSvc
			// Phase 17 Cards: combat mutations (HP, conditions,
			// concentration, exhaustion) re-render the card via the
			// new CardUpdater hook.
			combatSvc.SetCardUpdater(cardSvc)
		}
		var approvalNotifier dashboard.PlayerNotifier
		if discordSession != nil {
			approvalNotifier = newPlayerNotifierAdapter(discord.NewDirectMessenger(discordSession))
		}
		approvalHandler := dashboard.NewApprovalHandler(logger, approvalStore, approvalNotifier, hub, uuid.Nil, cardPoster)
		approvalHandler.SetCampaignLookup(dashboardCampaignLookup{queries: queries})
		// F-2: approvals are DM-only — mount behind dmAuthMw so non-DM
		// authenticated users get 403 instead of an empty list.
		approvalHandler.RegisterApprovalRoutes(router.With(dmAuthMw))

		// Phase 91a: portal routes. The TokenService both issues
		// /create-character links (via newPortalTokenIssuer below) and
		// validates the token on /portal/create — sharing one TokenService
		// instance keeps the issue / redeem cycle on the same store.
		portalTokenSvc := portal.NewTokenService(portal.NewTokenStore(db))
		portalHandler := portal.NewHandler(logger, portalTokenSvc)
		var portalOpts []portal.RouteOption
		if authBundle.oauthSvc != nil {
			portalOpts = append(portalOpts, portal.WithOAuth(authBundle.oauthSvc))
		}
		// Phase 91b/91c/92 wiring (high-17): without WithAPI the Svelte
		// builder's POST /portal/api/characters returns 404; without
		// WithCharacterSheet the /character Discord embed link points to a
		// 404. buildPortalAPIAndSheetHandlers is the single source of truth
		// so both endpoints stay in sync with portalTokenSvc.
		portalAPIHandler, portalSheetHandler := buildPortalAPIAndSheetHandlers(queries, portalTokenSvc)
		if portalAPIHandler != nil {
			portalOpts = append(portalOpts, portal.WithAPI(portalAPIHandler))
		}
		if portalSheetHandler != nil {
			portalOpts = append(portalOpts, portal.WithCharacterSheet(portalSheetHandler))
		}
		portal.RegisterRoutes(router, portalHandler, authMw, portalOpts...)

		// Phase 104b: Publisher fan-out to non-combat services that can
		// mutate an active encounter's combatant state mid-combat. The
		// encounter lookup resolves "which active encounter (if any) is this
		// character currently in?" so each service can skip publishing when
		// the mutation doesn't touch live combat state.
		encLookup := encounterLookupAdapter{queries: queries}
		inventoryAPIHandler := inventory.NewAPIHandler(queries)
		inventoryAPIHandler.SetPublisher(publisher, encLookup)
		// F-2: /api/inventory/* are DM-only inventory mutations — apply
		// dmAuthMw so non-DM authenticated users get 403 instead of
		// reaching the handler.
		dashboard.RegisterInventoryAPI(router, inventoryAPIHandler, dmAuthMw)

		// Phase 83b/85/86/87 wiring (high-13): mount the loot pool, item
		// picker, shops, and party-rest dashboard endpoints. Without this
		// the Svelte UIs (`ItemPicker.svelte`, `ShopBuilder.svelte`, the
		// loot-pool widgets) call URLs that 404. Each handler is built
		// from the shared *refdata.Queries; the party-rest handler also
		// needs Discord-side adapters for player DMs and #roll-history
		// posts so we construct it inline.
		var partyRestHandler *rest.PartyRestHandler
		var partyDM playerDirectMessenger
		if discordSession != nil {
			partyDM = discord.NewDirectMessenger(discordSession)
		}
		partyLister := newPartyCharacterListerAdapter(queries)
		partyUpdater := newPartyCharacterUpdaterAdapter(queries)
		partyEncounterChecker := newPartyEncounterCheckerAdapter(queries)
		var partyNotifier rest.PartyPlayerNotifier = noopPartyPlayerNotifier{}
		if a := newPartyPlayerNotifierAdapter(partyDM); a != nil {
			partyNotifier = a
		}
		var partyPoster rest.PartySummaryPoster = noopPartySummaryPoster{}
		if a := newPartySummaryPosterAdapter(discordSession, queries); a != nil {
			partyPoster = a
		}
		if partyLister != nil && partyUpdater != nil && partyEncounterChecker != nil {
			partyRestHandler = rest.NewPartyRestHandler(
				rest.NewService(dice.NewRoller(nil)),
				partyLister,
				partyUpdater,
				partyEncounterChecker,
				partyNotifier,
				partyPoster,
			)
		}
		mountDashboardAPIs(router, dashboardAPIDeps{
			authMiddleware:   authMw,
			queries:          queries,
			partyRestHandler: partyRestHandler,
		})

		// Phase 104c / H-104c: Level-up handler. DB-backed store/class
		// adapters plus a DM-capable notifier with the public-channel
		// StoryPoster wired through narration.Poster so #the-story gets a
		// "🎉 X reached Level N!" announcement on every level-up. Both
		// the messenger and the story poster are optional — either being
		// nil silently no-ops the corresponding surface so headless
		// deploys keep working. SetPublisher runs before RegisterRoutes
		// so no mutation can land without fan-out.
		var levelUpDM levelup.DirectMessenger
		var levelUpStory levelup.StoryPoster
		if discordSession != nil {
			levelUpDM = discord.NewDirectMessenger(discordSession)
			levelUpStory = newLevelUpStoryPosterAdapter(queries, discord.NewNarrationPoster(discordSession))
		}
		levelUpSvc := levelup.NewService(
			levelup.NewCharacterStoreAdapter(queries),
			levelup.NewClassStoreAdapter(queries),
			levelup.NewNotifierAdapterWithStory(levelUpDM, levelUpStory),
		)
		levelUpSvc.SetPublisher(publisher, encLookup)
		levelup.NewHandler(levelUpSvc, hub).RegisterRoutes(router)

		// B-26b: lifecycle fan-outs on EndCombat. The three notifiers wire
		// the post-combat hooks the original phase doc called for: a
		// #combat-log announcement, a loot-pool auto-create, and a
		// DM-facing prompt when every hostile drops to 0 HP mid-combat.
		// Each adapter no-ops cleanly when Discord is offline so headless
		// e2e deploys keep working.
		combatSvc.SetCombatLogNotifier(newCombatLogNotifierAdapter(discordSession, campaignSettingsProvider))
		combatSvc.SetLootPoolCreator(newLootPoolCreatorAdapter(lootSvcForCombat(queries)))
		combatSvc.SetHostilesDefeatedNotifier(newHostilesDefeatedNotifierAdapter(dmQueueNotifier))

		// H-104b: publisher fan-out for the magic-item activation paths.
		// The Service surface is currently consulted by the /attune
		// production-wiring follow-up; this construction keeps the hook
		// reachable so a wired handler can call PublishForCharacter.
		magicItemSvc := magicitem.NewService()
		magicItemSvc.SetPublisher(publisher, encLookup)
		_ = magicItemSvc // reserved for /attune handler wiring (deferred)

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

			// Phase 120a: shared loot service drives both the /loot handler
			// and the dashboard loot endpoints in future phases.
			lootSvc := loot.NewService(queries)

			// Phase 22 wiring (high-10): campaignSettingsProvider is
			// constructed above (shared with the DM combat-log poster) so
			// /done, the rollHistoryLogger, and the discord slash-command
			// handlers all read channel ids from a single source.

			// med-20 / Phase 26a: post the first-combatant ping when
			// StartCombat creates the first turn so players don't sit in
			// silence until someone runs /done. Best-effort: a nil notifier
			// is tolerated and the StartCombat flow degrades silently.
			if firstTurnNotifier := newFirstTurnPingNotifier(discordSession, campaignSettingsProvider, queries); firstTurnNotifier != nil {
				combatSvc.SetTurnStartNotifier(firstTurnNotifier)
			}

			// med-18 / Phase 25: post + auto-update the persistent
			// #initiative-tracker message. The message ID lives in an
			// in-memory map for now; bot restart causes the next update to
			// post a fresh message (the user-visible behaviour stays correct,
			// just no edit-in-place across restarts).
			if trackerNotifier := newInitiativeTrackerNotifier(discordSession, campaignSettingsProvider); trackerNotifier != nil {
				combatSvc.SetInitiativeTrackerNotifier(trackerNotifier)
			}

			// Phase 22 wiring (high-10): the production map regenerator.
			// done_handler.PostCombatMap and enemy_turn_notifier both
			// consume this to render PNGs into #combat-map. Without this
			// wiring the field on discordHandlerDeps stayed nil and
			// Phase 22's "PNG generated from map JSON + combatant
			// positions" never reached production.
			mapRegen := newMapRegeneratorAdapter(queries)

			// AOE-CAST + D-48b/49/51 follow-up: build a single shared
			// reaction-prompt store so the cast handler's gold-fallback
			// confirmation and the attack handler's class-feature prompts
			// route their button clicks through one place. The same store
			// is registered on the CommandRouter below so rxprompt:* clicks
			// fan back to the per-prompt OnChoice closures.
			reactionPrompts := discord.NewReactionPromptStore(discordSession)

			// Phase 105b: Construct every Phase 105 slash-command handler
			// with the per-user encounter resolver injected, wire them into
			// a CommandRouter, and register the router as the discordgo
			// InteractionCreate callback so /move, /fly, /distance, /done,
			// /check, /save, /rest, /command (summon), /loot and /recap all
			// route to the invoker's own encounter when two simultaneous
			// encounters share a channel.
			discordHandlerSet := buildDiscordHandlers(discordHandlerDeps{
				session:                  discordSession,
				queries:                  queries,
				db:                       db, // Phase 27 turn-gate (combat.TxBeginner)
				combatService:            combatSvc,
				roller:                   dice.NewRoller(nil),
				resolver:                 newDiscordUserEncounterResolver(queries),
				campaignSettings:         campaignSettingsProvider,
				enemyTurnEncounterLookup: combatSvc,
				mapRegenerator:           mapRegen,
				// Phase 18 wiring (high-09): the rollHistoryLogger uses
				// the entry's Roller (character name) to resolve the
				// owning campaign and post to that campaign's
				// #roll-history channel. /check, /save, /rest all
				// populate Roller before calling LogRoll.
				rollHistoryLogger: newRollHistoryLoggerByRoller(discordSession, queries),
				lootService:       lootSvc,
				// crit-01c: plumb optional collaborators for /help, /inventory,
				// /equip, /give, /attune, /unattune, /character, ASI components,
				// /undo, /retire. Each handler is nil-safe; missing deps mean the
				// router falls back to the status-aware stub.
				levelUpService:  levelUpSvc,
				dmQueueFunc:     dmQueueChannel,
				notifier:        dmQueueNotifier,
				portalBaseURL:   os.Getenv("BASE_URL"),
				reactionPrompts: reactionPrompts,
			})

			// Phase 120a: wire RegistrationDeps so /register submits land in
			// the database (status=pending) and downstream stub commands
			// become status-aware. The router otherwise falls through to a
			// "not yet implemented" stub for /register.
			//
			// G-90: thread the ddbimport service through DDBImporter so
			// /import routes to the real DDB fetch/parse/diff/preview path
			// (handlePlaceholderImport remains as fallback for offline tests).
			regSvc := registration.NewService(queries)
			ddbImportSvc := ddbimport.NewService(ddbimport.NewDDBClient(), queries)
			regDeps := buildRegistrationDeps(registrationDepsConfig{
				regService:   regSvc,
				campaignProv: queries,
				charCreator:  regSvc,
				dmQueueFunc:  newDMQueueChannelResolver(discordSession),
				dmUserFunc: func(guildID string) string {
					camp, err := queries.GetCampaignByGuildID(context.Background(), guildID)
					if err != nil {
						return ""
					}
					return camp.DmUserID
				},
				// Phase 91a: mint real one-time portal tokens via the
				// shared TokenService. The same service instance is the
				// validator on portal.NewHandler so issue / redeem stays
				// on a single store. The closure captures the
				// application context so token persistence honours
				// graceful shutdown.
				tokenFunc: newPortalTokenIssuer(ctx, portalTokenSvc),
				nameResolver: func(ctx context.Context, characterID uuid.UUID) (string, error) {
					char, err := queries.GetCharacter(ctx, characterID)
					if err != nil {
						return "", err
					}
					return char.Name, nil
				},
				ddbImporter:   ddbImportSvc,
				portalBaseURL: os.Getenv("BASE_URL"),
			})
			// Phase 12: wire the /setup handler so the DM can create the
			// SYSTEM/NARRATION/COMBAT/REFERENCE channel structure for their
			// guild. Without this, /setup falls through to "Unknown command".
			setupHandler := discord.NewSetupHandler(bot, newSetupCampaignLookup(queries))
			cmdRouter := discord.NewCommandRouter(bot, setupHandler, regDeps)
			// Phase 112: wire panic recovery + error recorder so any handler
			// panic is caught, converted into a friendly ephemeral, logged at
			// ERROR level, and recorded for the DM dashboard badge / panel.
			cmdRouter.SetErrorRecorder(errorStore)
			// AOE-CAST + D-48b/49/51 follow-up: route rxprompt:* button
			// clicks (gold-fallback Buy & Cast, Stunning Strike Use Ki,
			// Divine Smite slot picker, Bardic Inspiration Use Die) through
			// the shared prompt store so OnChoice closures fire.
			cmdRouter.SetReactionPromptStore(reactionPrompts)
			attachPhase105Handlers(cmdRouter, discordHandlerSet)
			// H-105b: inject the Discord-backed enemy-turn notifier so
			// combat.Handler.ExecuteEnemyTurn posts the "⚔️ <display_name>
			// — Round N" label instead of silently no-oping in production.
			wireEnemyTurnNotifier(combatHandler, discordHandlerSet.enemyTurnNotifier)
			// Phase 106a: route /rest dm-queue posts through the notifier so
			// rest requests are persisted and resolvable from the dashboard.
			discordHandlerSet.rest.SetNotifier(dmQueueNotifier)
			// H-104b: fan out a dashboard snapshot when /rest mutates HP /
			// hit dice / spell slots for a character who is also a
			// combatant in a sibling active encounter. The lookup is the
			// same encLookup shared with inventory + levelup so a single
			// per-character resolver drives every Phase 104b fan-out.
			discordHandlerSet.rest.SetPublisher(publisher, encLookup)
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
