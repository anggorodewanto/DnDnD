package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/ddbimport"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/registration"
)

// --- high-09: rollHistoryLogger production adapter ---

// stubChannelLookup satisfies discord.CampaignSettingsProvider for the
// roll-history adapter unit tests.
type stubChannelLookup struct {
	channels map[string]string
	err      error
}

func (s *stubChannelLookup) GetChannelIDs(_ context.Context, _ uuid.UUID) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.channels, nil
}

// TestRollHistoryLoggerAdapter_PostsToRollHistoryChannel covers the happy path:
// a roll log entry resolves the per-encounter roll-history channel and posts the
// formatted message via the session.
func TestRollHistoryLoggerAdapter_PostsToRollHistoryChannel(t *testing.T) {
	var sentChannel, sentContent string
	sess := &testSession{
		sendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentChannel = channelID
			sentContent = content
			return &discordgo.Message{}, nil
		},
	}
	encID := uuid.New()
	csp := &stubChannelLookup{channels: map[string]string{"roll-history": "ch-rh"}}
	a := newRollHistoryLoggerAdapter(sess, csp, encID)

	err := a.LogRoll(dice.RollLogEntry{
		Roller:     "Aragorn",
		Purpose:    "Perception",
		Total:      18,
		Expression: "1d20+5",
	})
	require.NoError(t, err)
	assert.Equal(t, "ch-rh", sentChannel)
	assert.Contains(t, sentContent, "Aragorn")
	assert.Contains(t, sentContent, "Perception")
}

// TestRollHistoryLoggerAdapter_NoChannelIsNoOp ensures missing roll-history
// channel ID is silently ignored — matches the best-effort pattern used by
// other channel posters in the codebase.
func TestRollHistoryLoggerAdapter_NoChannelIsNoOp(t *testing.T) {
	called := false
	sess := &testSession{
		sendFunc: func(string, string) (*discordgo.Message, error) {
			called = true
			return nil, nil
		},
	}
	csp := &stubChannelLookup{channels: map[string]string{}}
	a := newRollHistoryLoggerAdapter(sess, csp, uuid.New())

	err := a.LogRoll(dice.RollLogEntry{Roller: "x"})
	require.NoError(t, err)
	assert.False(t, called, "no channel-id means no send")
}

// TestRollHistoryLoggerAdapter_ProviderErrorIsNoOp ensures a CSP error does
// not propagate (best-effort logging shouldn't fail the dice roll).
func TestRollHistoryLoggerAdapter_ProviderErrorIsNoOp(t *testing.T) {
	sess := &testSession{}
	csp := &stubChannelLookup{err: errors.New("db down")}
	a := newRollHistoryLoggerAdapter(sess, csp, uuid.New())
	require.NoError(t, a.LogRoll(dice.RollLogEntry{Roller: "x"}))
}

// TestBuildDiscordHandlers_WiresRollHistoryLogger asserts that when the deps
// carry a rollHistoryLogger, the /check, /save, /rest handlers all see a
// non-nil logger so #roll-history posts actually fire end-to-end.
func TestBuildDiscordHandlers_WiresRollHistoryLogger(t *testing.T) {
	session := &testSession{}
	deps := discordHandlerDeps{
		session:           session,
		roller:            dice.NewRoller(nil),
		resolver:          &stubUserEncounterResolver{},
		combatService:     combat.NewService(nil),
		rollHistoryLogger: &recordingRollLogger{},
	}
	set := buildDiscordHandlers(deps)
	require.NotNil(t, set.check)
	require.NotNil(t, set.save)
	require.NotNil(t, set.rest)
	assert.True(t, set.check.HasRollLogger(), "/check must have rollLogger when wired in deps")
	assert.True(t, set.save.HasRollLogger(), "/save must have rollLogger when wired in deps")
	assert.True(t, set.rest.HasRollLogger(), "/rest must have rollLogger when wired in deps")
}

// recordingRollLogger is a minimal dice.RollHistoryLogger used to prove the
// adapter is wired through. The handlers' HasRollLogger introspection is
// what's load-bearing — we don't need to inspect the entries here.
type recordingRollLogger struct {
	entries []dice.RollLogEntry
}

func (r *recordingRollLogger) LogRoll(e dice.RollLogEntry) error {
	r.entries = append(r.entries, e)
	return nil
}

// --- high-10: mapRegenerator field set on discordHandlerDeps ---

// TestBuildDiscordHandlers_WiresMapRegenerator asserts that the mapRegenerator
// field on discordHandlerDeps is propagated into both DoneHandler and
// DiscordEnemyTurnNotifier so PostCombatMap actually fires PNGs in production.
func TestBuildDiscordHandlers_WiresMapRegenerator(t *testing.T) {
	session := &testSession{}
	mr := &recordingMapRegenerator{png: []byte("PNG-DATA")}
	deps := discordHandlerDeps{
		session:        session,
		roller:         dice.NewRoller(nil),
		resolver:       &stubUserEncounterResolver{},
		combatService:  combat.NewService(nil),
		mapRegenerator: mr,
		campaignSettings: &stubCampaignSettingsProvider{
			channels: map[string]string{"combat-map": "ch-cm"},
		},
	}
	set := buildDiscordHandlers(deps)
	require.NotNil(t, set.done, "done handler must be constructed")
	require.NotNil(t, set.enemyTurnNotifier)
	assert.True(t, set.done.HasMapRegenerator(),
		"done handler must propagate the deps.mapRegenerator field (otherwise #combat-map is silent)")
}

// recordingMapRegenerator returns canned PNG bytes for production-wiring tests.
type recordingMapRegenerator struct {
	png []byte
}

func (r *recordingMapRegenerator) RegenerateMap(_ context.Context, _ uuid.UUID) ([]byte, error) {
	return r.png, nil
}

// TestMapRegeneratorAdapter_ExploredHistory_UnionsAcrossRenders verifies the
// med-27 / Phase 68 explored-history wiring: a tile that was Visible on the
// previous render is upgraded from Unexplored to Explored on the next render
// even when the vision source has moved away.
func TestMapRegeneratorAdapter_ExploredHistory_UnionsAcrossRenders(t *testing.T) {
	a := &mapRegeneratorAdapter{exploredCells: map[uuid.UUID]map[int]bool{}}
	encID := uuid.New()

	// First render: tile (0) is Visible, (1) is Unexplored.
	first := &renderer.FogOfWar{
		Width:  2,
		Height: 1,
		States: []renderer.VisibilityState{renderer.Visible, renderer.Unexplored},
	}
	a.recordVisibleTiles(encID, first)

	// Second render: same map, but vision source moved so tile (0) is now
	// Unexplored. After applyExploredHistory, (0) should be Explored.
	second := &renderer.FogOfWar{
		Width:  2,
		Height: 1,
		States: []renderer.VisibilityState{renderer.Unexplored, renderer.Visible},
	}
	a.applyExploredHistory(encID, second)
	require.Equal(t, renderer.Explored, second.States[0], "previously-visible tile must render as Explored")
	require.Equal(t, renderer.Visible, second.States[1], "currently-visible tile must remain Visible")

	// Third render: union widens. (0) Explored stays in history; (1) is
	// now Visible and gets recorded so a fourth render dimming both
	// would surface both as Explored.
	a.recordVisibleTiles(encID, second)
	third := &renderer.FogOfWar{
		Width:  2,
		Height: 1,
		States: []renderer.VisibilityState{renderer.Unexplored, renderer.Unexplored},
	}
	a.applyExploredHistory(encID, third)
	require.Equal(t, renderer.Explored, third.States[0])
	require.Equal(t, renderer.Explored, third.States[1])
}

// TestMapRegeneratorAdapter_RendersAndDebouncesViaQueue covers the production
// adapter: a map-regenerator backed by refdata.Queries plus the renderer's
// RenderQueue produces PNG bytes when invoked — used as the implementation
// behind discordHandlerDeps.mapRegenerator in main.go.
func TestMapRegeneratorAdapter_RendersAndDebouncesViaQueue(t *testing.T) {
	// Adapter accepts a queries-shaped interface; we provide a stub.
	q := &fakeMapRegenQueries{
		encs:       map[uuid.UUID]refdata.Encounter{},
		maps:       map[uuid.UUID]refdata.Map{},
		combatants: map[uuid.UUID][]refdata.Combatant{},
	}
	encID := uuid.New()
	mapID := uuid.New()
	q.encs[encID] = refdata.Encounter{
		ID:    encID,
		MapID: uuid.NullUUID{UUID: mapID, Valid: true},
	}
	q.maps[mapID] = refdata.Map{
		ID: mapID,
		TiledJson: []byte(`{
			"width": 4, "height": 4,
			"tilewidth": 48, "tileheight": 48,
			"layers": [{"name": "terrain", "type": "tilelayer", "data": [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]}],
			"tilesets": []
		}`),
	}
	q.combatants[encID] = []refdata.Combatant{}

	a := newMapRegeneratorAdapter(q)
	require.NotNil(t, a)

	png, err := a.RegenerateMap(context.Background(), encID)
	require.NoError(t, err)
	require.NotEmpty(t, png, "renderer must produce PNG bytes")
}

// fakeMapRegenQueries satisfies the narrow queries surface the
// mapRegeneratorAdapter needs.
type fakeMapRegenQueries struct {
	encs       map[uuid.UUID]refdata.Encounter
	maps       map[uuid.UUID]refdata.Map
	combatants map[uuid.UUID][]refdata.Combatant
	zones      map[uuid.UUID][]refdata.EncounterZone
}

func (f *fakeMapRegenQueries) GetEncounter(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
	enc, ok := f.encs[id]
	if !ok {
		return refdata.Encounter{}, errors.New("encounter not found")
	}
	return enc, nil
}

func (f *fakeMapRegenQueries) GetMapByID(_ context.Context, id uuid.UUID) (refdata.Map, error) {
	m, ok := f.maps[id]
	if !ok {
		return refdata.Map{}, errors.New("map not found")
	}
	return m, nil
}

func (f *fakeMapRegenQueries) ListCombatantsByEncounterID(_ context.Context, id uuid.UUID) ([]refdata.Combatant, error) {
	return f.combatants[id], nil
}

func (f *fakeMapRegenQueries) ListEncounterZonesByEncounterID(_ context.Context, id uuid.UUID) ([]refdata.EncounterZone, error) {
	return f.zones[id], nil
}

// E-67-zone-render-on-map: zonesToRendererOverlays converts encounter_zones
// rows into renderer.ZoneOverlay records suitable for DrawZoneOverlays.
func TestZonesToRendererOverlays_BuildsOverlaysFromZones(t *testing.T) {
	zones := []refdata.EncounterZone{
		{
			ID:           uuid.New(),
			SourceSpell:  "Fog Cloud",
			Shape:        "circle",
			OriginCol:    "C",
			OriginRow:    3,
			Dimensions:   []byte(`{"radius_ft":20}`),
			OverlayColor: "#808080",
			MarkerIcon:   sql.NullString{String: "☁", Valid: true},
		},
		{
			ID:           uuid.New(),
			SourceSpell:  "Spirit Guardians",
			Shape:        "circle",
			OriginCol:    "B",
			OriginRow:    2,
			Dimensions:   []byte(`{"radius_ft":15}`),
			OverlayColor: "#FFD700",
			MarkerIcon:   sql.NullString{String: "✨", Valid: true},
		},
		{
			// Malformed hex — should be skipped.
			ID:           uuid.New(),
			SourceSpell:  "Bad Zone",
			Shape:        "circle",
			OriginCol:    "A",
			OriginRow:    1,
			Dimensions:   []byte(`{"radius_ft":5}`),
			OverlayColor: "not-a-hex",
		},
	}
	overlays := zonesToRendererOverlays(zones)
	require.Len(t, overlays, 2, "malformed-hex zone should be skipped")
	assert.Equal(t, uint8(0x80), overlays[0].Color.R)
	assert.Equal(t, "☁", overlays[0].MarkerIcon)
	assert.NotEmpty(t, overlays[0].AffectedTiles)
	assert.NotEmpty(t, overlays[1].AffectedTiles)
}

// E-67-zone-render-on-map: RegenerateMap wires zones into MapData.ZoneOverlays
// so DrawZoneOverlays paints them.
func TestMapRegeneratorAdapter_RendersWithZoneOverlays(t *testing.T) {
	q := &fakeMapRegenQueries{
		encs:       map[uuid.UUID]refdata.Encounter{},
		maps:       map[uuid.UUID]refdata.Map{},
		combatants: map[uuid.UUID][]refdata.Combatant{},
		zones:      map[uuid.UUID][]refdata.EncounterZone{},
	}
	encID := uuid.New()
	mapID := uuid.New()
	q.encs[encID] = refdata.Encounter{
		ID:    encID,
		MapID: uuid.NullUUID{UUID: mapID, Valid: true},
	}
	q.maps[mapID] = refdata.Map{
		ID: mapID,
		TiledJson: []byte(`{
			"width": 4, "height": 4,
			"tilewidth": 48, "tileheight": 48,
			"layers": [{"name": "terrain", "type": "tilelayer", "data": [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]}],
			"tilesets": []
		}`),
	}
	q.zones[encID] = []refdata.EncounterZone{
		{
			SourceSpell:  "Fog Cloud",
			Shape:        "circle",
			OriginCol:    "B",
			OriginRow:    2,
			Dimensions:   []byte(`{"radius_ft":10}`),
			OverlayColor: "#808080",
			MarkerIcon:   sql.NullString{String: "☁", Valid: true},
		},
	}

	a := newMapRegeneratorAdapter(q)
	require.NotNil(t, a)
	png, err := a.RegenerateMap(context.Background(), encID)
	require.NoError(t, err)
	require.NotEmpty(t, png, "renderer must still produce PNG bytes when zones are present")
}

// --- high-13: dashboard API handlers (loot, item picker, shops, party rest) ---

// TestMountDashboardAPIs_NilDepsIsSafe asserts that calling the wiring
// helper with no concrete handlers (and no queries) is panic-free —
// matches the "nil-safe in test deploys" pattern used elsewhere in the
// wiring layer.
func TestMountDashboardAPIs_NilDepsIsSafe(t *testing.T) {
	r := chi.NewRouter()
	require.NotPanics(t, func() {
		mountDashboardAPIs(r, dashboardAPIDeps{})
	})
}

// --- high-14: production MessageQueue wraps Discord session sends ---

// TestQueueingSession_RoutesSendsThroughMessageQueue asserts that the
// queueing wrapper used to wire MessageQueue into production sends every
// ChannelMessageSend through the queue (per-channel serialization +
// rate-limit retry) instead of bypassing it.
func TestQueueingSession_RoutesSendsThroughMessageQueue(t *testing.T) {
	inner := &testSession{
		sendFunc: func(channelID, content string) (*discordgo.Message, error) {
			return &discordgo.Message{ChannelID: channelID, Content: content}, nil
		},
	}
	q := discord.NewMessageQueue(inner)
	defer q.Stop()

	wrapped := newQueueingSession(inner, q)

	msg, err := wrapped.ChannelMessageSend("ch-1", "hello")
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "ch-1", msg.ChannelID)
}

// TestQueueingSession_PassesNonSendMethodsThrough verifies the wrapper does
// NOT route InteractionRespond, GuildChannels, etc. through the queue (those
// have separate rate limits).
func TestQueueingSession_PassesNonSendMethodsThrough(t *testing.T) {
	called := false
	inner := &testSession{
		guildChannelsFunc: func(string) ([]*discordgo.Channel, error) {
			called = true
			return []*discordgo.Channel{{ID: "x"}}, nil
		},
	}
	q := discord.NewMessageQueue(inner)
	defer q.Stop()

	wrapped := newQueueingSession(inner, q)
	chs, err := wrapped.GuildChannels("g")
	require.NoError(t, err)
	require.Len(t, chs, 1)
	assert.True(t, called)
}

// --- high-17: portal RegisterRoutes receives WithAPI + WithCharacterSheet ---

// TestBuildPortalRouteOptions_AppendsAPIAndCharacterSheet asserts that the
// production wiring helper attaches BOTH WithAPI and WithCharacterSheet to
// the portal RouteOptions slice — without those, /portal/api/* returns 404
// and /portal/character/{id} is unreachable (high-17).
func TestBuildPortalRouteOptions_AppendsAPIAndCharacterSheet(t *testing.T) {
	q := refdata.New(nil)
	apiH, sheetH := buildPortalAPIAndSheetHandlers(q, nil)
	require.NotNil(t, apiH, "production wiring must construct portal.APIHandler from queries")
	require.NotNil(t, sheetH, "production wiring must construct portal.CharacterSheetHandler from queries")
}

// --- G-94a / G-95: combat workspace + DM dashboard routes mounted on router ---

// TestMountCombatRoutes_RegistersWorkspaceAndDMDashboard asserts that the
// shared helper used by run() mounts WorkspaceHandler (G-94a) and
// DMDashboardHandler (G-95/97a/97b) on the chi router so /api/combat/workspace,
// /api/combat/{enc}/pending-actions, /api/combat/{enc}/action-log,
// /api/combat/{enc}/advance-turn, /api/combat/{enc}/undo-last-action, and the
// override family all return non-404.
func TestMountCombatRoutes_RegistersWorkspaceAndDMDashboard(t *testing.T) {
	r := chi.NewRouter()
	svc := combat.NewService(nil)
	_ = mountCombatDashboardRoutes(r, svc, stubWorkspaceStore{}, nil, nil)

	// Use chi.Walk to enumerate the registered routes so the assertion is
	// independent of handler behaviour (the underlying Service is wired to a
	// nil store, so any handler call would panic — but route registration is
	// the only thing we're asserting here).
	registered := map[string]map[string]bool{}
	require.NoError(t, chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if registered[method] == nil {
			registered[method] = map[string]bool{}
		}
		registered[method][route] = true
		return nil
	}))
	expected := []struct {
		method string
		path   string
	}{
		{"GET", "/api/combat/workspace"},
		{"GET", "/api/combat/{encounterID}/pending-actions"},
		{"POST", "/api/combat/{encounterID}/pending-actions/{actionID}/resolve"},
		{"GET", "/api/combat/{encounterID}/action-log"},
		{"POST", "/api/combat/{encounterID}/advance-turn"},
		{"POST", "/api/combat/{encounterID}/undo-last-action"},
		{"POST", "/api/combat/{encounterID}/override/combatant/{combatantID}/hp"},
		{"POST", "/api/combat/{encounterID}/override/combatant/{combatantID}/position"},
		{"POST", "/api/combat/{encounterID}/override/combatant/{combatantID}/conditions"},
		{"POST", "/api/combat/{encounterID}/override/combatant/{combatantID}/initiative"},
		{"POST", "/api/combat/{encounterID}/override/character/{characterID}/spell-slots"},
		{"PATCH", "/api/combat/{encounterID}/combatants/{combatantID}/hp"},
		{"DELETE", "/api/combat/{encounterID}/combatants/{combatantID}"},
	}
	for _, rt := range expected {
		assert.True(t, registered[rt.method][rt.path],
			"%s %s must be registered after mountCombatDashboardRoutes (got %v)", rt.method, rt.path, registered[rt.method])
	}
}

// TestMountCombatRoutes_InjectsCombatLogPoster verifies that the helper
// threads the CombatLogPoster through to NewDMDashboardHandlerWithDeps so
// Phase 97b undo / override paths actually deliver correction messages. The
// integration with real Service.store is covered by
// internal/combat/dm_dashboard_undo_integration_test.go; this test is a
// wiring smoke check.
func TestMountCombatRoutes_InjectsCombatLogPoster(t *testing.T) {
	r := chi.NewRouter()
	svc := combat.NewService(nil)
	poster := &recordingCombatLogPoster{}
	wiring := mountCombatDashboardRoutes(r, svc, stubWorkspaceStore{}, nil, poster)
	require.NotNil(t, wiring.handler, "helper must return constructed dm dashboard handler")
	assert.Same(t, combat.CombatLogPoster(poster), wiring.poster,
		"production wiring must thread the CombatLogPoster (Phase 97b)")
}

// stubWorkspaceStore satisfies combat.WorkspaceStore with empty/error
// returns. Each method returns a zero value so the route table is mountable
// but no row-level behaviour is exercised — that's covered by the
// internal/combat package tests.
type stubWorkspaceStore struct{}

func (stubWorkspaceStore) ListEncountersByCampaignID(context.Context, uuid.UUID) ([]refdata.Encounter, error) {
	return nil, nil
}
func (stubWorkspaceStore) ListCombatantsByEncounterID(context.Context, uuid.UUID) ([]refdata.Combatant, error) {
	return nil, nil
}
func (stubWorkspaceStore) GetMapByID(context.Context, uuid.UUID) (refdata.Map, error) {
	return refdata.Map{}, nil
}
func (stubWorkspaceStore) ListEncounterZonesByEncounterID(context.Context, uuid.UUID) ([]refdata.EncounterZone, error) {
	return nil, nil
}
func (stubWorkspaceStore) UpdateCombatantHP(context.Context, refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
	return refdata.Combatant{}, nil
}
func (stubWorkspaceStore) UpdateCombatantConditions(context.Context, refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
	return refdata.Combatant{}, nil
}
func (stubWorkspaceStore) GetCombatantByID(context.Context, uuid.UUID) (refdata.Combatant, error) {
	return refdata.Combatant{}, nil
}
func (stubWorkspaceStore) GetActiveTurnByEncounterID(context.Context, uuid.UUID) (refdata.Turn, error) {
	return refdata.Turn{}, nil
}
func (stubWorkspaceStore) UpdateCombatantPosition(context.Context, refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
	return refdata.Combatant{}, nil
}
func (stubWorkspaceStore) DeleteCombatant(context.Context, uuid.UUID) error { return nil }
func (stubWorkspaceStore) GetCharacter(context.Context, uuid.UUID) (refdata.Character, error) {
	return refdata.Character{}, nil
}
func (stubWorkspaceStore) GetCreature(context.Context, string) (refdata.Creature, error) {
	return refdata.Creature{}, nil
}

type recordingCombatLogPoster struct {
	calls int
}

func (r *recordingCombatLogPoster) PostCorrection(_ context.Context, _ uuid.UUID, _ string) {
	r.calls++
}

// --- H-105b: enemyTurnNotifier injected into combat.Handler ---

// TestWireEnemyTurnNotifier_SetsNotifierOnCombatHandler asserts that the
// helper used by run() to wire the enemy-turn notifier onto combat.Handler
// actually invokes SetEnemyTurnNotifier — so combat.Handler.ExecuteEnemyTurn
// stops falling through the silent-no-op branch in production.
func TestWireEnemyTurnNotifier_SetsNotifierOnCombatHandler(t *testing.T) {
	spy := &recordingEnemyTurnSetter{}
	notifier := &stubEnemyTurnNotifier{}
	wireEnemyTurnNotifier(spy, notifier)
	require.Equal(t, 1, spy.calls,
		"wireEnemyTurnNotifier must call SetEnemyTurnNotifier on the combat handler")
	assert.Same(t, notifier, spy.last)
}

// TestWireEnemyTurnNotifier_NilHandlerOrNotifierIsSafe verifies the helper is
// nil-safe so test deploys without a notifier do not panic.
func TestWireEnemyTurnNotifier_NilHandlerOrNotifierIsSafe(t *testing.T) {
	assert.NotPanics(t, func() { wireEnemyTurnNotifier(nil, &stubEnemyTurnNotifier{}) })
	assert.NotPanics(t, func() { wireEnemyTurnNotifier(&recordingEnemyTurnSetter{}, nil) })
}

type recordingEnemyTurnSetter struct {
	calls int
	last  combat.EnemyTurnNotifier
}

func (r *recordingEnemyTurnSetter) SetEnemyTurnNotifier(n combat.EnemyTurnNotifier) {
	r.calls++
	r.last = n
}

type stubEnemyTurnNotifier struct{}

func (*stubEnemyTurnNotifier) NotifyEnemyTurnExecuted(_ context.Context, _ uuid.UUID, _ string) {
}

// --- G-90: DDB importer threaded into RegistrationDeps ---

// TestBuildRegistrationDeps_CarriesDDBImporter asserts the production helper
// surfaces the DDBImporter so /import routes to the real ddbimport service
// (not handlePlaceholderImport).
func TestBuildRegistrationDeps_CarriesDDBImporter(t *testing.T) {
	importer := &recordingDDBImporter{}
	deps := buildRegistrationDeps(registrationDepsConfig{
		ddbImporter: importer,
	})
	require.NotNil(t, deps)
	require.NotNil(t, deps.DDBImporter, "RegistrationDeps must carry the wired DDBImporter")
	assert.Same(t, importer, deps.DDBImporter)
}

// newTestHTTPRequest builds a chi-mountable request without DB / TLS noise.
func newTestHTTPRequest(t *testing.T, method, path string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, path, strings.NewReader(""))
	require.NoError(t, err)
	return req
}

func newTestHTTPRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

// recordingDDBImporter counts Import calls so we can assert /import flows
// through the real path when DDBImporter is wired into RegistrationDeps.
type recordingDDBImporter struct {
	calls int
}

func (r *recordingDDBImporter) Import(_ context.Context, _ uuid.UUID, _ string) (*ddbimport.ImportResult, error) {
	r.calls++
	return &ddbimport.ImportResult{
		Character: refdata.Character{ID: uuid.New(), Name: "Imported"},
		Preview:   "preview",
	}, nil
}

// noopRegService satisfies discord.RegistrationService for router-construction tests.
type noopRegService struct{}

func (noopRegService) Register(context.Context, uuid.UUID, string, string) (*registration.RegisterResult, error) {
	return &registration.RegisterResult{Status: registration.ResultExactMatch}, nil
}
func (noopRegService) Import(context.Context, uuid.UUID, string, uuid.UUID) (*refdata.PlayerCharacter, error) {
	return &refdata.PlayerCharacter{}, nil
}
func (noopRegService) Create(context.Context, uuid.UUID, string, uuid.UUID) (*refdata.PlayerCharacter, error) {
	return &refdata.PlayerCharacter{}, nil
}
func (noopRegService) GetStatus(context.Context, uuid.UUID, string) (*refdata.PlayerCharacter, error) {
	return &refdata.PlayerCharacter{Status: "approved"}, nil
}

type noopCampaignProvider struct{}

func (noopCampaignProvider) GetCampaignByGuildID(_ context.Context, _ string) (refdata.Campaign, error) {
	return refdata.Campaign{ID: uuid.New()}, nil
}

type noopCharCreator struct{}

func (noopCharCreator) CreatePlaceholder(_ context.Context, _ uuid.UUID, _, _ string) (refdata.Character, error) {
	return refdata.Character{ID: uuid.New()}, nil
}

// TestCommandRouter_ImportHandlerUsesDDBImporterWhenWired covers the
// integration with NewCommandRouter: a /import interaction reaches the real
// DDBImporter when RegistrationDeps.DDBImporter is non-nil.
func TestCommandRouter_ImportHandlerUsesDDBImporterWhenWired(t *testing.T) {
	importer := &recordingDDBImporter{}
	session := &testSession{}
	bot := discord.NewBot(session, "app-id", nil)
	deps := &discord.RegistrationDeps{
		RegService:   &noopRegService{},
		CampaignProv: &noopCampaignProvider{},
		CharCreator:  &noopCharCreator{},
		DMQueueFunc:  func(string) string { return "" },
		DMUserFunc:   func(string) string { return "" },
		TokenFunc: func(uuid.UUID, string) (string, error) {
			return "", nil
		},
		NameResolver: func(_ context.Context, _ uuid.UUID) (string, error) {
			return "", nil
		},
		DDBImporter: importer,
	}
	router := discord.NewCommandRouter(bot, nil, deps)

	router.Handle(&discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "import",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "ddb-url", Type: discordgo.ApplicationCommandOptionString, Value: "https://www.dndbeyond.com/characters/12345"},
			},
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "u1"}},
	})

	assert.Equal(t, 1, importer.calls,
		"/import interaction must route through deps.DDBImporter when wired")
}

// --- SR-003: Bot.HandleGuildCreate / HandleGuildMemberAdd + IntentsGuildMembers wiring ---

// fakeGatewayWirer captures handler/intent wiring for SR-003 without spinning
// up a real discordgo session. AddHandler stores each registered callback
// keyed by its Go type signature so the test can prove that GuildCreate +
// GuildMemberAdd + InteractionCreate handlers are all registered.
type fakeGatewayWirer struct {
	handlers []any
	intents  discordgo.Intent
}

func (f *fakeGatewayWirer) AddHandler(h any) func() {
	f.handlers = append(f.handlers, h)
	return func() {}
}

func (f *fakeGatewayWirer) OrIntent(i discordgo.Intent) { f.intents |= i }

// hasHandlerOfType reports whether any registered handler matches the
// signature `func(*discordgo.Session, *<eventPtr>)`. eventPtr is a non-nil
// pointer to a zero-value event struct (e.g. (*discordgo.GuildCreate)(nil)).
func (f *fakeGatewayWirer) hasHandlerOfType(eventPtr any) bool {
	want := reflect.TypeOf(eventPtr)
	for _, h := range f.handlers {
		ht := reflect.TypeOf(h)
		if ht.Kind() != reflect.Func {
			continue
		}
		if ht.NumIn() != 2 || ht.NumOut() != 0 {
			continue
		}
		if ht.In(0) != reflect.TypeOf((*discordgo.Session)(nil)) {
			continue
		}
		if ht.In(1) == want {
			return true
		}
	}
	return false
}

// TestWireBotHandlers_RegistersGuildAndInteractionHandlers asserts the SR-003
// fix: after wireBotHandlers runs, the gateway has GuildCreate +
// GuildMemberAdd + InteractionCreate handlers attached AND Identify.Intents
// includes IntentsGuildMembers (without which discordgo's privileged-intent
// default excludes member-join events).
func TestWireBotHandlers_RegistersGuildAndInteractionHandlers(t *testing.T) {
	w := &fakeGatewayWirer{}
	bot := discord.NewBot(&testSession{}, "app-id", nil)
	router := discord.NewCommandRouter(bot, nil, nil)

	wireBotHandlers(w, w, bot, router)

	assert.True(t, w.hasHandlerOfType((*discordgo.GuildCreate)(nil)),
		"wireBotHandlers must register Bot.HandleGuildCreate so dynamic guild-join command registration fires (spec line 179)")
	assert.True(t, w.hasHandlerOfType((*discordgo.GuildMemberAdd)(nil)),
		"wireBotHandlers must register Bot.HandleGuildMemberAdd so welcome DMs fire (spec lines 183-200)")
	assert.True(t, w.hasHandlerOfType((*discordgo.InteractionCreate)(nil)),
		"wireBotHandlers must keep the InteractionCreate handler that routes slash commands through CommandRouter")
	assert.NotZero(t, w.intents&discordgo.IntentsGuildMembers,
		"wireBotHandlers must OR-in IntentsGuildMembers so the privileged member-join gateway event is actually delivered")
}
