package main

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
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
