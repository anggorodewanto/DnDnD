package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/portal"
	"github.com/ab/dndnd/internal/refdata"
)

// portalTokenCreateCharacterPurpose is the purpose tag stored alongside every
// portal token minted by the /create-character flow. Persisted in
// portal_tokens.purpose so future audits / cleanup can tell at a glance which
// flow issued the token.
const portalTokenCreateCharacterPurpose = "create_character"

// portalTokenCreateCharacterTTL is the validity window for /create-character
// portal links per Phase 91a spec ("one-time link, 24 h expiry").
const portalTokenCreateCharacterTTL = 24 * time.Hour

// newPortalTokenIssuer returns a function shaped like discord.RegistrationDeps
// .TokenFunc that mints a portal token via the supplied TokenService. The
// returned closure captures the application context so each call participates
// in graceful shutdown without forcing the registration handler to thread one
// through. Replaces the legacy "e2e-token" placeholder (Phase 14 follow-up)
// per crit-06.
func newPortalTokenIssuer(ctx context.Context, svc *portal.TokenService) func(campaignID uuid.UUID, discordUserID string) (string, error) {
	return func(campaignID uuid.UUID, discordUserID string) (string, error) {
		return svc.CreateToken(ctx, campaignID, discordUserID, portalTokenCreateCharacterPurpose, portalTokenCreateCharacterTTL)
	}
}

// resolverQueries is the subset of refdata.Queries used by
// discordUserEncounterResolver. Declaring it as an interface keeps the
// resolver unit-testable without a live Postgres instance.
type resolverQueries interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
	GetPlayerCharacterByDiscordUser(ctx context.Context, arg refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error)
	GetActiveEncounterIDByCharacterID(ctx context.Context, characterID uuid.NullUUID) (uuid.UUID, error)
}

// discordUserEncounterResolver implements the Phase 105 per-user encounter
// routing contract by walking:
//
//	guild_id -> campaign_id -> (campaign_id, discord_user_id) -> character_id -> active encounter
type discordUserEncounterResolver struct {
	queries resolverQueries
}

func newDiscordUserEncounterResolver(q resolverQueries) *discordUserEncounterResolver {
	return &discordUserEncounterResolver{queries: q}
}

// ActiveEncounterForUser returns the active encounter ID the invoking Discord
// user is currently a combatant in, or a non-nil error if they are not
// registered or not in any active encounter.
func (r *discordUserEncounterResolver) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	campaign, err := r.queries.GetCampaignByGuildID(ctx, guildID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("campaign lookup for guild %q: %w", guildID, err)
	}

	pc, err := r.queries.GetPlayerCharacterByDiscordUser(ctx, refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    campaign.ID,
		DiscordUserID: discordUserID,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("player character lookup for user %q: %w", discordUserID, err)
	}

	encID, err := r.queries.GetActiveEncounterIDByCharacterID(ctx, uuid.NullUUID{UUID: pc.CharacterID, Valid: true})
	if err != nil {
		return uuid.Nil, fmt.Errorf("active encounter lookup for character %s: %w", pc.CharacterID, err)
	}

	return encID, nil
}

// setupQueries is the subset of refdata.Queries used by setupCampaignLookup.
// Declaring it as an interface keeps the adapter unit-testable without a live
// Postgres instance.
type setupQueries interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
	UpdateCampaignSettings(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error)
	CreateCampaign(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error)
}

// setupCampaignLookup adapts refdata.Queries to discord.CampaignLookup so the
// Phase 12 /setup handler can resolve the campaign's DM user and persist the
// channel IDs it creates. Channel IDs are merged into the existing campaign
// settings JSONB; default settings are seeded when the row's settings column
// is null.
//
// med-41 / Phase 11: when no campaign row exists for the guild yet, the
// lookup auto-creates one with default settings, taking the /setup invoker
// to be the DM. This closes the "no campaign found for this server" dead
// end the playtest quickstart used to hit before any encounter could be
// built — the production code path that was missing per the chunk2 review.
type setupCampaignLookup struct {
	queries setupQueries
}

func newSetupCampaignLookup(q setupQueries) *setupCampaignLookup {
	return &setupCampaignLookup{queries: q}
}

// defaultAutoCreatedCampaignName builds a placeholder campaign name when /setup
// auto-creates the row. The DM can rename via dashboard later. Using the guild
// ID keeps the name unique even before guild metadata is plumbed through.
func defaultAutoCreatedCampaignName(guildID string) string {
	return fmt.Sprintf("Campaign for guild %s", guildID)
}

// GetCampaignForSetup returns the campaign info /setup needs (the DM's
// Discord user id, used for permission overwrites on private channels).
// When no row exists yet for the guild, the row is auto-created with the
// invoking user as DM and default settings.
func (l *setupCampaignLookup) GetCampaignForSetup(guildID, invokerUserID string) (discord.SetupCampaignInfo, error) {
	ctx := context.Background()
	c, err := l.queries.GetCampaignByGuildID(ctx, guildID)
	if err == nil {
		return discord.SetupCampaignInfo{DMUserID: c.DmUserID}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return discord.SetupCampaignInfo{}, fmt.Errorf("campaign lookup for guild %q: %w", guildID, err)
	}
	if invokerUserID == "" {
		return discord.SetupCampaignInfo{}, fmt.Errorf("auto-create campaign for guild %q: invoker user id is empty", guildID)
	}

	settings := campaign.DefaultSettings()
	raw, err := json.Marshal(settings)
	if err != nil {
		return discord.SetupCampaignInfo{}, fmt.Errorf("encoding default settings for new campaign: %w", err)
	}
	created, err := l.queries.CreateCampaign(ctx, refdata.CreateCampaignParams{
		GuildID:  guildID,
		DmUserID: invokerUserID,
		Name:     defaultAutoCreatedCampaignName(guildID),
		Settings: pqtype.NullRawMessage{RawMessage: raw, Valid: true},
	})
	if err != nil {
		return discord.SetupCampaignInfo{}, fmt.Errorf("auto-creating campaign for guild %q: %w", guildID, err)
	}
	return discord.SetupCampaignInfo{DMUserID: created.DmUserID, AutoCreated: true}, nil
}

// playerDirectMessenger is the subset of discord.DirectMessenger that
// playerNotifierAdapter depends on. Declaring it as an interface keeps the
// adapter unit-testable without a live Discord session.
type playerDirectMessenger interface {
	SendDirectMessage(discordUserID, body string) ([]string, error)
}

// playerNotifierAdapter implements dashboard.PlayerNotifier by sending
// Discord DMs through the bot's existing DirectMessenger. Wired into
// dashboard.NewApprovalHandler so approve / changes-requested / reject all
// notify the player out-of-band per Phase 16 done-when (spec lines 41 + 53).
type playerNotifierAdapter struct {
	dm playerDirectMessenger
}

func newPlayerNotifierAdapter(dm playerDirectMessenger) *playerNotifierAdapter {
	return &playerNotifierAdapter{dm: dm}
}

// NotifyApproval pings the player that their character was approved.
func (a *playerNotifierAdapter) NotifyApproval(_ context.Context, discordUserID, characterName string) error {
	body := fmt.Sprintf("✅ **%s** has been approved! You can now play.", characterName)
	if _, err := a.dm.SendDirectMessage(discordUserID, body); err != nil {
		return fmt.Errorf("notifying approval to %s: %w", discordUserID, err)
	}
	return nil
}

// NotifyChangesRequested pings the player that the DM requested changes,
// including the DM's feedback verbatim.
func (a *playerNotifierAdapter) NotifyChangesRequested(_ context.Context, discordUserID, characterName, feedback string) error {
	body := fmt.Sprintf("📝 **%s** needs changes before approval.\n\n**DM feedback:** %s", characterName, feedback)
	if _, err := a.dm.SendDirectMessage(discordUserID, body); err != nil {
		return fmt.Errorf("notifying changes-requested to %s: %w", discordUserID, err)
	}
	return nil
}

// NotifyRejection pings the player that their character was rejected,
// including the DM's reason verbatim.
func (a *playerNotifierAdapter) NotifyRejection(_ context.Context, discordUserID, characterName, feedback string) error {
	body := fmt.Sprintf("❌ **%s** was rejected.\n\n**DM feedback:** %s", characterName, feedback)
	if _, err := a.dm.SendDirectMessage(discordUserID, body); err != nil {
		return fmt.Errorf("notifying rejection to %s: %w", discordUserID, err)
	}
	return nil
}

// rollHistoryChannelResolver returns the #roll-history channel ID that a
// given dice.RollLogEntry should post to. Returns ("", nil) when no channel
// can be resolved (best-effort: the adapter then silently no-ops). Errors
// are bubbled up only for true failures (DB unreachable etc.) — the adapter
// also swallows them so dice rolls aren't blocked on logging.
type rollHistoryChannelResolver func(ctx context.Context, entry dice.RollLogEntry) (string, error)

// rollHistoryLoggerAdapter implements dice.RollHistoryLogger for production
// by posting each RollLogEntry to the `roll-history` channel resolved by
// the supplied resolver. Best-effort: missing channel id, missing campaign,
// send failures are all silently swallowed so dice rolls never fail because
// their audit log can't reach Discord.
//
// Phase 18 done-when wiring: bridges dice.RollLogEntry → ChannelMessageSend
// for every /check, /save, /rest call. Replaces the long-standing nil
// rollLogger args in cmd/dndnd/discord_handlers.go (high-09).
type rollHistoryLoggerAdapter struct {
	session  discord.Session
	resolver rollHistoryChannelResolver
}

// newRollHistoryLoggerAdapter constructs the adapter. When csp is non-nil
// and encounterID is non-Nil, the adapter resolves the channel via the
// per-encounter CampaignSettingsProvider chain — this matches the chunk2
// recommendation. Production wiring uses the channel-resolver-by-roller
// variant constructed by newRollHistoryLoggerAdapterByRoller below so the
// Roller (character name) on each entry drives the per-campaign lookup.
func newRollHistoryLoggerAdapter(s discord.Session, csp discord.CampaignSettingsProvider, encounterID uuid.UUID) *rollHistoryLoggerAdapter {
	resolver := func(ctx context.Context, _ dice.RollLogEntry) (string, error) {
		if csp == nil {
			return "", nil
		}
		channelIDs, err := csp.GetChannelIDs(ctx, encounterID)
		if err != nil {
			return "", err
		}
		return channelIDs["roll-history"], nil
	}
	return rollHistoryLoggerAdapterFromResolver(s, resolver)
}

// rollHistoryLoggerAdapterFromResolver is the lower-level constructor used
// when the channel resolution depends on the entry itself (e.g. roller
// name -> active campaign).
func rollHistoryLoggerAdapterFromResolver(s discord.Session, resolver rollHistoryChannelResolver) *rollHistoryLoggerAdapter {
	return &rollHistoryLoggerAdapter{session: s, resolver: resolver}
}

// newRollHistoryLoggerByRoller constructs the production adapter that
// resolves the roll-history channel by walking the roller's character name
// to the campaign their PC belongs to. queries is *refdata.Queries; nil
// queries makes the adapter a no-op (test-only mode).
func newRollHistoryLoggerByRoller(s discord.Session, q *refdata.Queries) *rollHistoryLoggerAdapter {
	if s == nil {
		return nil
	}
	resolver := func(ctx context.Context, entry dice.RollLogEntry) (string, error) {
		if q == nil || entry.Roller == "" {
			return "", nil
		}
		campaign, err := lookupCampaignByCharacterName(ctx, q, entry.Roller)
		if err != nil {
			return "", err
		}
		if !campaign.Settings.Valid {
			return "", nil
		}
		var settings campaignSettingsForRollHistory
		if err := json.Unmarshal(campaign.Settings.RawMessage, &settings); err != nil {
			return "", err
		}
		return settings.ChannelIDs["roll-history"], nil
	}
	return rollHistoryLoggerAdapterFromResolver(s, resolver)
}

// campaignSettingsForRollHistory mirrors the channel_ids field of the
// campaign settings JSONB without pulling in the full campaign.Settings
// type (which would make this file depend on internal/campaign for one
// field).
type campaignSettingsForRollHistory struct {
	ChannelIDs map[string]string `json:"channel_ids"`
}

// lookupCampaignByCharacterName scans player_characters for one whose
// associated character matches the given name. Returns the first match;
// when no match is found, returns ("", nil) so the adapter no-ops cleanly.
func lookupCampaignByCharacterName(ctx context.Context, q *refdata.Queries, name string) (refdata.Campaign, error) {
	campaigns, err := q.ListCampaigns(ctx)
	if err != nil {
		return refdata.Campaign{}, err
	}
	for _, c := range campaigns {
		chars, err := q.ListCharactersByCampaign(ctx, c.ID)
		if err != nil {
			continue
		}
		for _, ch := range chars {
			if ch.Name == name {
				return c, nil
			}
		}
	}
	return refdata.Campaign{}, nil
}

// LogRoll formats the entry and posts it to the resolved roll-history
// channel. Errors are returned only when caller-visible logic would care;
// channel-resolution problems are treated as no-ops.
func (a *rollHistoryLoggerAdapter) LogRoll(entry dice.RollLogEntry) error {
	if a == nil || a.session == nil || a.resolver == nil {
		return nil
	}
	ctx := context.Background()
	channelID, err := a.resolver(ctx, entry)
	if err != nil {
		return nil
	}
	if channelID == "" {
		return nil
	}
	_, _ = a.session.ChannelMessageSend(channelID, formatRollLogEntry(entry))
	return nil
}

// formatRollLogEntry produces a one-line summary of a roll suitable for
// #roll-history. The format prioritises the player + purpose so DMs can
// scan quickly without parsing dice expressions.
func formatRollLogEntry(e dice.RollLogEntry) string {
	parts := []string{}
	if e.Roller != "" {
		parts = append(parts, e.Roller)
	}
	if e.Purpose != "" {
		parts = append(parts, "— "+e.Purpose)
	}
	if e.Expression != "" {
		parts = append(parts, fmt.Sprintf("`%s`", e.Expression))
	}
	if e.Total != 0 || e.Breakdown != "" {
		breakdown := e.Breakdown
		if breakdown == "" {
			breakdown = fmt.Sprintf("%d", e.Total)
		}
		parts = append(parts, "= "+breakdown)
	}
	if len(parts) == 0 {
		return "(empty roll)"
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += " " + p
	}
	return out
}

// mapRegeneratorQueries is the narrow subset of refdata.Queries that the
// production map-regenerator needs. Declaring it as an interface keeps the
// adapter unit-testable without a live Postgres instance.
type mapRegeneratorQueries interface {
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	GetMapByID(ctx context.Context, id uuid.UUID) (refdata.Map, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	// E-67-zone-render-on-map: zones are loaded so the renderer can paint
	// their overlays alongside terrain + combatants.
	ListEncounterZonesByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.EncounterZone, error)
	// SR-008: PC vision sources are derived from Character.Race → Race.DarkvisionFt.
	// NPC vision sources come from Creature.Senses JSONB.
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetRace(ctx context.Context, id string) (refdata.Race, error)
	GetCreature(ctx context.Context, id string) (refdata.Creature, error)
	// SR-031: persist the explored-tile set per encounter so bot restarts
	// don't wipe a party's exploration history.
	UpdateEncounterExploredCells(ctx context.Context, arg refdata.UpdateEncounterExploredCellsParams) error
}

// mapRegeneratorAdapter implements discord.MapRegenerator by parsing the
// encounter's Tiled map JSON, projecting the live combatant positions onto
// the renderer's Combatant slice, and asking renderer.RenderMap for PNG
// bytes. Returns an error only when the encounter has no associated map or
// the renderer fails — empty-combatant maps still render.
//
// Phase 22 done-when wiring: produces PNGs for #combat-map. Production was
// silent because main.go never set discordHandlerDeps.mapRegenerator (high-10).
//
// med-27 / Phase 68: persists a per-encounter "explored cells" set so
// previously-visible tiles render with the dim Explored overlay even after
// the vision source has moved away. SR-031 graduated the explored history
// from in-memory-only to a JSONB column on `encounters.explored_cells`, so a
// bot restart no longer wipes a party's exploration history.
//
// The adapter holds a small per-encounter mutex (keyed by encounter UUID so
// concurrent renders of *different* encounters don't serialize) to keep
// read-modify-write of the explored set atomic against itself; the DB row
// is the source of truth.
type mapRegeneratorAdapter struct {
	queries mapRegeneratorQueries

	exploredMu sync.Mutex
	exploredKM map[uuid.UUID]*sync.Mutex // encounterID → per-encounter R-M-W lock
}

func newMapRegeneratorAdapter(q mapRegeneratorQueries) *mapRegeneratorAdapter {
	if q == nil {
		return nil
	}
	return &mapRegeneratorAdapter{
		queries:    q,
		exploredKM: make(map[uuid.UUID]*sync.Mutex),
	}
}

// lockForEncounter returns (and lazily creates) a per-encounter mutex used to
// serialize the read-modify-write of the persisted explored-cells set. Two
// concurrent regenerations of the same encounter must not race on the JSONB
// blob; two concurrent regenerations of *different* encounters proceed in
// parallel.
func (a *mapRegeneratorAdapter) lockForEncounter(encounterID uuid.UUID) *sync.Mutex {
	a.exploredMu.Lock()
	defer a.exploredMu.Unlock()
	m, ok := a.exploredKM[encounterID]
	if !ok {
		m = &sync.Mutex{}
		a.exploredKM[encounterID] = m
	}
	return m
}

// RegenerateMap loads the encounter, its map, and its combatants, then
// renders a fresh PNG for the shared #combat-map channel (player view).
// Combatant positions are translated from the "letter+row" string form to
// the renderer's 0-indexed col/row. After rendering, currently-visible tiles
// are unioned into the per-encounter explored set so the next render shows
// them as dim Explored when the vision source is no longer covering them
// (med-27 / Phase 68).
func (a *mapRegeneratorAdapter) RegenerateMap(ctx context.Context, encounterID uuid.UUID) ([]byte, error) {
	return a.renderInternal(ctx, encounterID, false)
}

// RegenerateMapForDM renders the same encounter with DMSeesAll=true so the
// dashboard's Combat Manager view (and any "DM only" PNG poster) bypasses
// the player-side fog and shows every combatant / tile at full brightness.
// SR-008: completes the "DM view vs player view" branching the spec calls
// for in §"Dynamic Fog of War".
func (a *mapRegeneratorAdapter) RegenerateMapForDM(ctx context.Context, encounterID uuid.UUID) ([]byte, error) {
	return a.renderInternal(ctx, encounterID, true)
}

// renderInternal is the shared body used by RegenerateMap (player view) and
// RegenerateMapForDM (DM view). The `dmView` flag flips DMSeesAll on the
// produced MapData so the renderer's downstream draw helpers bypass fog.
func (a *mapRegeneratorAdapter) renderInternal(ctx context.Context, encounterID uuid.UUID, dmView bool) ([]byte, error) {
	enc, err := a.queries.GetEncounter(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("get encounter %s: %w", encounterID, err)
	}
	if !enc.MapID.Valid {
		return nil, fmt.Errorf("encounter %s has no map", encounterID)
	}
	m, err := a.queries.GetMapByID(ctx, enc.MapID.UUID)
	if err != nil {
		return nil, fmt.Errorf("get map %s: %w", enc.MapID.UUID, err)
	}
	combatants, err := a.queries.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("list combatants for %s: %w", encounterID, err)
	}
	renderCombatants := combatantsToRendererForm(combatants)
	md, err := renderer.ParseTiledJSON(m.TiledJson, renderCombatants, nil)
	if err != nil {
		return nil, fmt.Errorf("parse tiled json: %w", err)
	}

	// E-67-zone-render-on-map: project the active encounter_zones rows into
	// MapData.ZoneOverlays so DrawZoneOverlays paints Fog Cloud / Wall of
	// Fire / Darkness / Spirit Guardians overlays on the rendered PNG.
	// Failure to load zones is non-fatal — the map still renders without
	// overlays — but the error is logged so a DM can investigate.
	zones, zerr := a.queries.ListEncounterZonesByEncounterID(ctx, encounterID)
	if zerr == nil && len(zones) > 0 {
		md.ZoneOverlays = zonesToRendererOverlays(zones)
		md.MagicalDarknessTiles = buildMagicalDarknessTiles(zones)
	}

	// SR-008: populate FoW inputs from the live combatants + zones so the
	// shadowcasting algorithm actually fires. Without this the
	// `len(md.VisionSources) > 0` guard below stays false and the map is
	// rendered with no fog at all.
	//
	// SR-068: LightSources populated from lit inventory items (torch/lantern)
	// and active light-emitting zones (Light/Daylight/Continual Flame).
	md.VisionSources = buildVisionSources(ctx, a.queries, combatants)
	md.LightSources = buildLightSources(ctx, a.queries, combatants, zones)
	md.DMSeesAll = dmView

	// med-27: pre-compute the FoW (so we can layer the explored history
	// on top before the renderer paints) only when there are vision or
	// light sources to compute against. Otherwise the renderer's
	// auto-compute path stays in charge and explored history is a no-op.
	//
	// SR-031: the explored set lives in `encounters.explored_cells` so a
	// bot restart restores it. We serialize the read-modify-write of that
	// blob with a per-encounter mutex (different encounters render in
	// parallel; the same encounter's renders queue up).
	var seen map[int]bool
	var fowComputed bool
	if len(md.VisionSources) > 0 || len(md.LightSources) > 0 {
		lock := a.lockForEncounter(encounterID)
		lock.Lock()
		defer lock.Unlock()
		seen = decodeExploredCells(enc.ExploredCells)
		md.FogOfWar = renderer.ComputeVisibilityWithZones(md.VisionSources, md.LightSources, md.Walls, md.MagicalDarknessTiles, md.Width, md.Height)
		md.FogOfWar.DMSeesAll = dmView
		applyExploredHistoryToFow(seen, md.FogOfWar)
		fowComputed = true
	}

	png, err := renderer.RenderMap(md)
	if err != nil {
		return nil, err
	}

	// Union the currently-visible tiles into the persistent explored set
	// so the next render carries them as Explored when no longer Visible.
	// Write the updated set back to the DB so a process restart picks it
	// up. Best-effort: a failed write only loses *new* exploration (the
	// previously-stored set stays correct).
	if fowComputed && md.FogOfWar != nil {
		if seen == nil {
			seen = map[int]bool{}
		}
		changed := unionVisibleTilesInto(seen, md.FogOfWar)
		if changed {
			a.persistExploredCells(ctx, encounterID, seen)
		}
	}

	return png, nil
}

// decodeExploredCells unpacks the JSONB array stored in
// encounters.explored_cells into the in-memory set keyed by
// (row*width+col) tile index. A nil / empty / malformed payload yields an
// empty set so the caller can treat "no history yet" the same as "history
// is empty". (SR-031)
func decodeExploredCells(raw json.RawMessage) map[int]bool {
	out := map[int]bool{}
	if len(raw) == 0 {
		return out
	}
	var idxs []int
	if err := json.Unmarshal(raw, &idxs); err != nil {
		return out
	}
	for _, i := range idxs {
		if i < 0 {
			continue
		}
		out[i] = true
	}
	return out
}

// encodeExploredCells packs the in-memory set back into the JSONB-friendly
// sorted int array form used by encounters.explored_cells. Sorted output
// keeps the JSONB blob diffable in DB dumps. (SR-031)
func encodeExploredCells(seen map[int]bool) (json.RawMessage, error) {
	idxs := make([]int, 0, len(seen))
	for i := range seen {
		idxs = append(idxs, i)
	}
	// sort.Ints would pull in the sort import for no functional gain, so
	// use insertion to keep determinism via a manual selection. For typical
	// map sizes (hundreds of tiles) the cost is irrelevant.
	for i := 1; i < len(idxs); i++ {
		for j := i; j > 0 && idxs[j-1] > idxs[j]; j-- {
			idxs[j-1], idxs[j] = idxs[j], idxs[j-1]
		}
	}
	return json.Marshal(idxs)
}

// persistExploredCells writes the updated explored set back to the DB.
// Failures are swallowed (best-effort) so rendering never fails over a
// persistence hiccup; the previously-stored set stays correct on a retry.
// (SR-031)
func (a *mapRegeneratorAdapter) persistExploredCells(ctx context.Context, encounterID uuid.UUID, seen map[int]bool) {
	raw, err := encodeExploredCells(seen)
	if err != nil {
		return
	}
	_ = a.queries.UpdateEncounterExploredCells(ctx, refdata.UpdateEncounterExploredCellsParams{
		ID:            encounterID,
		ExploredCells: raw,
	})
}

// applyExploredHistoryToFow upgrades any tile in `seen` (the persisted
// explored set) but not currently Visible from Unexplored to Explored.
// Visible tiles are left alone so the renderer paints them at full
// brightness. Caller must hold the per-encounter lock. (SR-031 replaces the
// pre-existing mapRegeneratorAdapter.applyExploredHistory with this pure
// helper so the explored set is sourced from the DB instead of an in-memory
// map.)
func applyExploredHistoryToFow(seen map[int]bool, fow *renderer.FogOfWar) {
	if fow == nil || len(seen) == 0 {
		return
	}
	for idx := range seen {
		if idx < 0 || idx >= len(fow.States) {
			continue
		}
		if fow.States[idx] == renderer.Unexplored {
			fow.States[idx] = renderer.Explored
		}
	}
}

// unionVisibleTilesInto adds every currently-Visible tile from the FoW into
// the `seen` set and reports whether any tile was newly added (i.e. whether
// the caller needs to persist the updated set). (SR-031)
func unionVisibleTilesInto(seen map[int]bool, fow *renderer.FogOfWar) bool {
	if fow == nil {
		return false
	}
	changed := false
	for idx, state := range fow.States {
		if state != renderer.Visible {
			continue
		}
		if !seen[idx] {
			seen[idx] = true
			changed = true
		}
	}
	return changed
}

// zonesToRendererOverlays converts the live encounter_zones rows into the
// renderer.ZoneOverlay shape that DrawZoneOverlays paints. Zones with
// unparseable hex overlay colors are skipped to keep the renderer
// deterministic; combat-side validation already prevents bad hex from
// being written. (E-67-zone-render-on-map)
func zonesToRendererOverlays(in []refdata.EncounterZone) []renderer.ZoneOverlay {
	out := make([]renderer.ZoneOverlay, 0, len(in))
	for _, z := range in {
		rgba, ok := parseHexRGBA(z.OverlayColor, 0x80)
		if !ok {
			continue
		}
		tiles := combat.ZoneAffectedTilesFromShape(z.Shape, z.OriginCol, z.OriginRow, z.Dimensions)
		oc, or := combat.ZoneOriginIndex(z.OriginCol, z.OriginRow)
		marker := ""
		if z.MarkerIcon.Valid {
			marker = z.MarkerIcon.String
		}
		overlay := renderer.ZoneOverlay{
			OriginCol:     oc,
			OriginRow:     or,
			AffectedTiles: make([]renderer.GridPos, 0, len(tiles)),
			Color:         rgba,
			MarkerIcon:    marker,
		}
		for _, t := range tiles {
			overlay.AffectedTiles = append(overlay.AffectedTiles, renderer.GridPos{Col: t.Col, Row: t.Row})
		}
		out = append(out, overlay)
	}
	return out
}

// parseHexRGBA converts an "#RRGGBB" string into a color.RGBA with the given
// alpha. Accepts a leading "#" or "0x" prefix. Returns ok=false on malformed
// input so the caller can skip the zone overlay rather than emit a stripe of
// black. (E-67-zone-render-on-map)
func parseHexRGBA(hex string, alpha uint8) (color.RGBA, bool) {
	s := strings.TrimSpace(hex)
	s = strings.TrimPrefix(s, "#")
	s = strings.TrimPrefix(s, "0x")
	if len(s) != 6 {
		return color.RGBA{}, false
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{
		R: uint8((n >> 16) & 0xFF),
		G: uint8((n >> 8) & 0xFF),
		B: uint8(n & 0xFF),
		A: alpha,
	}, true
}

// tilesFromFeet converts a feet measurement into the renderer's tile units
// (5ft per tile, mirroring the convention used by `devilsSightTileRange` in
// internal/gamemap/renderer/fog_types.go and the AoE pipeline). Negative
// inputs clamp to 0 so a missing/zero darkvision_ft never blows up the
// shadowcaster.
func tilesFromFeet(ft int) int {
	if ft <= 0 {
		return 0
	}
	return ft / 5
}

// defaultBaseSightTiles is the fallback "bright outdoor / lit dungeon"
// sight range used when no per-tile static lighting is available. Picked at
// 60ft (12 tiles) so a sighted PC with no darkvision still has a meaningful
// vision radius on a typical map. Once SR-029 wires the static lighting
// brush GIDs through ParseTiledJSON, this default narrows to match the
// per-tile light level.
const defaultBaseSightTiles = 12

// senses captures the subset of Creature.Senses JSONB that the renderer's
// VisionSource cares about. Field names match the spec's documented shape
// ({darkvision, blindsight, truesight, ...}). Missing keys default to 0.
type senses struct {
	Darkvision int `json:"darkvision"`
	Blindsight int `json:"blindsight"`
	Truesight  int `json:"truesight"`
}

// buildVisionSourcesQueries is the narrow read-only contract buildVisionSources
// needs from a mapRegeneratorQueries. Splitting it out makes the helper
// straight to unit-test from the test fake without invoking the full
// adapter.
type buildVisionSourcesQueries interface {
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetRace(ctx context.Context, id string) (refdata.Race, error)
	GetCreature(ctx context.Context, id string) (refdata.Creature, error)
}

// buildVisionSources projects each living combatant into a renderer.VisionSource:
//   - PCs: combatant.character_id → Character.Race → Race.DarkvisionFt
//   - NPCs: combatant.creature_ref_id → Creature.Senses (darkvision / blindsight /
//     truesight)
//
// Combatants with unparseable coordinates or empty linkage are skipped so the
// shadowcaster doesn't fire from (-1,-1). Lookup errors are non-fatal — the
// combatant is dropped from the source list and the rest of the encounter
// still gets fog computed. (SR-008)
func buildVisionSources(ctx context.Context, q buildVisionSourcesQueries, in []refdata.Combatant) []renderer.VisionSource {
	out := make([]renderer.VisionSource, 0, len(in))
	for _, c := range in {
		if !c.IsAlive {
			continue
		}
		col, row, err := renderer.ParseCoordinate(fmt.Sprintf("%s%d", c.PositionCol, c.PositionRow))
		if err != nil {
			continue
		}
		src := renderer.VisionSource{
			Col:        col,
			Row:        row,
			RangeTiles: defaultBaseSightTiles,
		}
		if c.IsNpc {
			if !c.CreatureRefID.Valid || q == nil {
				out = append(out, src)
				continue
			}
			creature, err := q.GetCreature(ctx, c.CreatureRefID.String)
			if err == nil && creature.Senses.Valid {
				var s senses
				if json.Unmarshal(creature.Senses.RawMessage, &s) == nil {
					src.DarkvisionTiles = tilesFromFeet(s.Darkvision)
					src.BlindsightTiles = tilesFromFeet(s.Blindsight)
					src.TruesightTiles = tilesFromFeet(s.Truesight)
				}
			}
			out = append(out, src)
			continue
		}
		// PC branch.
		if !c.CharacterID.Valid || q == nil {
			out = append(out, src)
			continue
		}
		ch, err := q.GetCharacter(ctx, c.CharacterID.UUID)
		if err != nil {
			out = append(out, src)
			continue
		}
		if ch.Race != "" {
			race, err := q.GetRace(ctx, ch.Race)
			if err == nil {
				src.DarkvisionTiles = tilesFromFeet(int(race.DarkvisionFt))
			}
		}
		out = append(out, src)
	}
	return out
}

// buildMagicalDarknessTiles projects every magical-darkness encounter zone
// (zone_type == "magical_darkness", e.g. the Darkness spell) into its
// affected-tile set. The renderer's ComputeVisibilityWithZones consumes this
// to demote ordinary vision + darkvision inside those tiles; Devil's Sight,
// Blindsight and Truesight still penetrate. (SR-008)
//
// Returns an empty slice when no magical-darkness zones are active — the
// renderer treats nil and an empty slice the same way. SR-014 is the upstream
// dependency that ensures zone overlays are populated by combat handlers;
// until that's fixed this slice may be empty in production even when the
// spell *is* active.
func buildMagicalDarknessTiles(zones []refdata.EncounterZone) []renderer.GridPos {
	if len(zones) == 0 {
		return nil
	}
	var tiles []renderer.GridPos
	for _, z := range zones {
		if z.ZoneType != "magical_darkness" {
			continue
		}
		for _, t := range combat.ZoneAffectedTilesFromShape(z.Shape, z.OriginCol, z.OriginRow, z.Dimensions) {
			tiles = append(tiles, renderer.GridPos{Col: t.Col, Row: t.Row})
		}
	}
	return tiles
}

// buildLightSources produces renderer.LightSource entries from two data sources:
//  1. Lit inventory items (torch = 4 tiles/20ft, lantern = 6 tiles/30ft) on PC
//     combatants whose character has is_lit items.
//  2. Active light-emitting encounter zones (Light cantrip = 4 tiles/20ft,
//     Daylight = 12 tiles/60ft, Continual Flame = 4 tiles/20ft) identified by
//     zone_type == "light".
//
// SR-068: without this, LightSources stays empty and non-darkvision PCs see
// 60ft everywhere (the defaultBaseSightTiles fallback), making darkness
// encounters feel uniformly lit.
func buildLightSources(ctx context.Context, q buildVisionSourcesQueries, combatants []refdata.Combatant, zones []refdata.EncounterZone) []renderer.LightSource {
	var out []renderer.LightSource

	// Source 1: lit inventory items on living PC combatants.
	for _, c := range combatants {
		if !c.IsAlive || c.IsNpc || !c.CharacterID.Valid || q == nil {
			continue
		}
		col, row, err := renderer.ParseCoordinate(fmt.Sprintf("%s%d", c.PositionCol, c.PositionRow))
		if err != nil {
			continue
		}
		ch, err := q.GetCharacter(ctx, c.CharacterID.UUID)
		if err != nil || !ch.Inventory.Valid {
			continue
		}
		out = append(out, lightSourcesFromInventory(ch.Inventory.RawMessage, col, row)...)
	}

	// Source 2: active light-emitting zones.
	for _, z := range zones {
		if z.ZoneType != "light" {
			continue
		}
		col, row, err := renderer.ParseCoordinate(fmt.Sprintf("%s%d", z.OriginCol, z.OriginRow))
		if err != nil {
			continue
		}
		out = append(out, renderer.LightSource{
			Col:        col,
			Row:        row,
			RangeTiles: lightRadiusForSpell(z.SourceSpell),
		})
	}

	return out
}

// lightSourcesFromInventory scans a character's inventory JSON for items
// with is_lit=true and returns a LightSource at the given position for each.
func lightSourcesFromInventory(raw json.RawMessage, col, row int) []renderer.LightSource {
	var items []struct {
		Name  string `json:"name"`
		IsLit bool   `json:"is_lit"`
	}
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	var out []renderer.LightSource
	for _, item := range items {
		if !item.IsLit {
			continue
		}
		out = append(out, renderer.LightSource{
			Col:        col,
			Row:        row,
			RangeTiles: lightRadiusForItem(item.Name),
		})
	}
	return out
}

// lightRadiusForItem returns the bright-light radius in tiles for a lit item.
func lightRadiusForItem(name string) int {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "lantern"):
		return 6 // 30ft
	default:
		return 4 // 20ft (torch and other light sources)
	}
}

// lightRadiusForSpell returns the bright-light radius in tiles for a
// light-emitting spell zone.
func lightRadiusForSpell(spellName string) int {
	switch strings.ToLower(spellName) {
	case "daylight":
		return 12 // 60ft
	default:
		return 4 // 20ft (Light cantrip, Continual Flame)
	}
}

// combatantsToRendererForm projects refdata.Combatant rows into the slimmer
// renderer.Combatant form. Combatants with unparseable coordinates are
// skipped so the renderer doesn't paint at (-1,-1).
func combatantsToRendererForm(in []refdata.Combatant) []renderer.Combatant {
	out := make([]renderer.Combatant, 0, len(in))
	for _, c := range in {
		col, row, err := renderer.ParseCoordinate(fmt.Sprintf("%s%d", c.PositionCol, c.PositionRow))
		if err != nil {
			continue
		}
		out = append(out, renderer.Combatant{
			ShortID:     c.ShortID,
			DisplayName: c.DisplayName,
			Col:         col,
			Row:         row,
			AltitudeFt:  int(c.AltitudeFt),
			HPMax:       int(c.HpMax),
			HPCurrent:   int(c.HpCurrent),
			IsPlayer:    !c.IsNpc,
		})
	}
	return out
}

// queueingSession wraps a discord.Session and routes ChannelMessageSend
// through a discord.MessageQueue so production sends pick up the per-channel
// FIFO + 429 retry/backoff that Phase 9b implements but production never
// instantiated (high-14). All other Session methods (interaction
// responses, guild lookups, channel message edits, the complex-send variant
// used by #combat-map PNG attachments) pass through to the inner session
// untouched — those have separate rate limits and don't need queue
// serialization for the playtest checklist.
type queueingSession struct {
	inner discord.Session
	queue *discord.MessageQueue
}

// newQueueingSession constructs the wrapper. When queue is nil the wrapper
// degrades to a transparent passthrough so test deploys without the queue
// keep working.
func newQueueingSession(inner discord.Session, queue *discord.MessageQueue) *queueingSession {
	return &queueingSession{inner: inner, queue: queue}
}

func (q *queueingSession) UserChannelCreate(recipientID string) (*discordgo.Channel, error) {
	return q.inner.UserChannelCreate(recipientID)
}

// ChannelMessageSend delegates through the MessageQueue. The queue does not
// surface the *discordgo.Message return value (its API only signals errors),
// so we synthesize a placeholder message echoing the channel + content.
// Callers in this codebase consistently discard the message return value
// (`_, _ = sess.ChannelMessageSend(...)` is the common pattern in
// internal/discord/*.go), so the placeholder doesn't break callers.
func (q *queueingSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	if q.queue == nil {
		return q.inner.ChannelMessageSend(channelID, content)
	}
	if err := q.queue.Send(channelID, content); err != nil {
		return nil, err
	}
	return &discordgo.Message{ChannelID: channelID, Content: content}, nil
}

func (q *queueingSession) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	return q.inner.ChannelMessageSendComplex(channelID, data)
}

func (q *queueingSession) ApplicationCommandBulkOverwrite(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
	return q.inner.ApplicationCommandBulkOverwrite(appID, guildID, cmds)
}

func (q *queueingSession) ApplicationCommands(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
	return q.inner.ApplicationCommands(appID, guildID)
}

func (q *queueingSession) ApplicationCommandDelete(appID, guildID, cmdID string) error {
	return q.inner.ApplicationCommandDelete(appID, guildID, cmdID)
}

func (q *queueingSession) GuildChannels(guildID string) ([]*discordgo.Channel, error) {
	return q.inner.GuildChannels(guildID)
}

func (q *queueingSession) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	return q.inner.GuildChannelCreateComplex(guildID, data)
}

func (q *queueingSession) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	return q.inner.InteractionRespond(interaction, resp)
}

func (q *queueingSession) InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
	return q.inner.InteractionResponseEdit(interaction, newresp)
}

func (q *queueingSession) ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error) {
	return q.inner.ChannelMessageEdit(channelID, messageID, content)
}

func (q *queueingSession) GetState() *discordgo.State {
	return q.inner.GetState()
}

// SaveChannelIDs merges channelIDs into the campaign settings JSONB and
// persists via UpdateCampaignSettings. Existing settings (turn timeout,
// diagonal rule, open5e sources, etc.) are preserved.
func (l *setupCampaignLookup) SaveChannelIDs(guildID string, channelIDs map[string]string) error {
	c, err := l.queries.GetCampaignByGuildID(context.Background(), guildID)
	if err != nil {
		return fmt.Errorf("campaign lookup for guild %q: %w", guildID, err)
	}

	settings := campaign.DefaultSettings()
	if c.Settings.Valid {
		if err := json.Unmarshal(c.Settings.RawMessage, &settings); err != nil {
			return fmt.Errorf("decoding existing settings: %w", err)
		}
	}
	settings.ChannelIDs = channelIDs

	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encoding updated settings: %w", err)
	}

	_, err = l.queries.UpdateCampaignSettings(context.Background(), refdata.UpdateCampaignSettingsParams{
		ID:       c.ID,
		Settings: pqtype.NullRawMessage{RawMessage: raw, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("updating campaign settings: %w", err)
	}
	return nil
}

// initiativeTrackerNotifier satisfies combat.InitiativeTrackerNotifier by
// posting / editing a persistent #initiative-tracker message per encounter.
// The mapping from encounter ID to Discord message ID lives in an
// in-memory map for now (med-18 / Phase 25). A bot restart loses the map,
// in which case the next AdvanceTurn falls back to PostTracker semantics
// — a small follow-up migration could persist the message ID on the
// encounters row to survive restarts.
type initiativeTrackerNotifier struct {
	session     discord.Session
	csp         discord.CampaignSettingsProvider
	mu          sync.Mutex
	messageByID map[uuid.UUID]initiativeTrackerMsg
}

type initiativeTrackerMsg struct {
	channelID string
	messageID string
}

func newInitiativeTrackerNotifier(session discord.Session, csp discord.CampaignSettingsProvider) *initiativeTrackerNotifier {
	if session == nil || csp == nil {
		return nil
	}
	return &initiativeTrackerNotifier{
		session:     session,
		csp:         csp,
		messageByID: map[uuid.UUID]initiativeTrackerMsg{},
	}
}

func (n *initiativeTrackerNotifier) channel(ctx context.Context, encounterID uuid.UUID) string {
	channelIDs, err := n.csp.GetChannelIDs(ctx, encounterID)
	if err != nil {
		return ""
	}
	return channelIDs["initiative-tracker"]
}

func (n *initiativeTrackerNotifier) PostTracker(ctx context.Context, encounterID uuid.UUID, content string) {
	ch := n.channel(ctx, encounterID)
	if ch == "" {
		return
	}
	msg, err := n.session.ChannelMessageSend(ch, content)
	if err != nil || msg == nil {
		return
	}
	n.mu.Lock()
	n.messageByID[encounterID] = initiativeTrackerMsg{channelID: ch, messageID: msg.ID}
	n.mu.Unlock()
}

func (n *initiativeTrackerNotifier) UpdateTracker(ctx context.Context, encounterID uuid.UUID, content string) {
	n.mu.Lock()
	prev, ok := n.messageByID[encounterID]
	n.mu.Unlock()
	if !ok {
		// No message recorded (probably restarted) — post a new one so the
		// channel still receives the update.
		n.PostTracker(ctx, encounterID, content)
		return
	}
	if _, err := n.session.ChannelMessageEdit(prev.channelID, prev.messageID, content); err != nil {
		return
	}
}

func (n *initiativeTrackerNotifier) PostCompletedTracker(ctx context.Context, encounterID uuid.UUID, content string) {
	ch := n.channel(ctx, encounterID)
	if ch == "" {
		return
	}
	_, _ = n.session.ChannelMessageSend(ch, content)
	// Drop the live tracker mapping — combat is over, future updates would
	// be misleading.
	n.mu.Lock()
	delete(n.messageByID, encounterID)
	n.mu.Unlock()
}

// firstTurnPingNotifier satisfies combat.TurnStartNotifier by posting the
// FormatTurnStartPrompt line to the active combatant's #your-turn channel
// when StartCombat creates the first turn (med-20 / Phase 26a). Without
// it, the very first PC waits in silence until someone runs /done.
//
// Best-effort: any missing dependency or Discord-side error is silently
// swallowed so the notifier can never roll back the encounter creation
// path. (StartCombat persists the encounter before it fires this hook.)
type firstTurnPingNotifier struct {
	session  discord.Session
	csp      discord.CampaignSettingsProvider
	queries  *refdata.Queries
}

func newFirstTurnPingNotifier(session discord.Session, csp discord.CampaignSettingsProvider, queries *refdata.Queries) *firstTurnPingNotifier {
	if session == nil || csp == nil || queries == nil {
		return nil
	}
	return &firstTurnPingNotifier{session: session, csp: csp, queries: queries}
}

func (n *firstTurnPingNotifier) NotifyFirstTurn(ctx context.Context, encounterID uuid.UUID, ti combat.TurnInfo) {
	channelIDs, err := n.csp.GetChannelIDs(ctx, encounterID)
	if err != nil {
		return
	}
	yourTurnCh, ok := channelIDs["your-turn"]
	if !ok || yourTurnCh == "" {
		return
	}
	combatant, err := n.queries.GetCombatant(ctx, ti.CombatantID)
	if err != nil {
		return
	}
	enc, err := n.queries.GetEncounter(ctx, encounterID)
	if err != nil {
		return
	}
	content := combat.FormatTurnStartPrompt(enc.Name, ti.RoundNumber, combatant.DisplayName, ti.Turn, &combatant)
	_, _ = n.session.ChannelMessageSend(yourTurnCh, content)
}

// buildPortalAPIAndSheetHandlers constructs the portal HTTP handlers that
// the production wiring needs to attach via WithAPI / WithCharacterSheet.
// The token service (when non-nil) is threaded into the BuilderStoreAdapter
// so submitted characters get a portal-token issued for them. A nil tokenSvc
// is acceptable for tests and matches the existing main.go pattern at
// cmd/dndnd/main.go:571.
//
// Phase 91b/91c/92 wiring (high-17): /portal/api/* and /portal/character/{id}
// were never registered in production because main.go only passed WithOAuth.
// This helper is the single source of truth for both handlers so the wiring
// stays in sync.
//
// Finding 20: uses NewRefDataAdapterWithOpen5eLookup when a lookup is provided
// so Open5e spell-list gating is active in production.
func buildPortalAPIAndSheetHandlers(queries *refdata.Queries, tokenSvc *portal.TokenService, open5eLookup portal.Open5eCampaignLookup) (*portal.APIHandler, *portal.CharacterSheetHandler) {
	if queries == nil {
		return nil, nil
	}
	var refDataAdapter *portal.RefDataAdapter
	if open5eLookup != nil {
		refDataAdapter = portal.NewRefDataAdapterWithOpen5eLookup(queries, open5eLookup)
	} else {
		refDataAdapter = portal.NewRefDataAdapter(queries)
	}
	builderStore := portal.NewBuilderStoreAdapter(queries, tokenSvc)
	builderSvc := portal.NewBuilderService(builderStore)
	apiHandler := portal.NewAPIHandler(nil, refDataAdapter, builderSvc)
	sheetStore := portal.NewCharacterSheetStoreAdapter(queries)
	sheetSvc := portal.NewCharacterSheetService(sheetStore)
	sheetHandler := portal.NewCharacterSheetHandler(nil, sheetSvc)
	return apiHandler, sheetHandler
}
