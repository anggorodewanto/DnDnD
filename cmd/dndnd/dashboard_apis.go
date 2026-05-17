package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/itempicker"
	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/rest"
	"github.com/ab/dndnd/internal/shops"
)

// dashboardAPIDeps bundles every collaborator the dashboard-side HTTP API
// surfaces need to be live in production. Each handler is constructed
// best-effort: when its required dependencies are nil the helper skips
// mounting that family so headless deploys (test runs without a Discord
// session) still come up clean.
//
// Phase 83b/85/86/87 wiring (high-13): loot, item picker, shops, party rest
// were all "implemented but not mounted" — Svelte UIs called endpoints that
// 404'd. mountDashboardAPIs closes that gap.
type dashboardAPIDeps struct {
	authMiddleware func(http.Handler) http.Handler

	// queries is *refdata.Queries (or any structurally-compatible store).
	// When nil the helper skips every handler family, leaving the router
	// intact for upstream Mount* calls.
	queries *refdata.Queries

	// lootHandler / itemPickerHandler / shopsHandler / partyRestHandler are
	// optional pre-constructed handlers; when nil the helper builds them
	// from queries. Tests can pass mocks here to skip the construction
	// path.
	lootHandler       *loot.APIHandler
	itemPickerHandler *itempicker.Handler
	shopsHandler      *shops.Handler
	partyRestHandler  *rest.PartyRestHandler
}

// mountDashboardAPIs registers the loot, item-picker, shops, and party-rest
// HTTP routes on the given chi router. Best-effort: nil dependencies cause
// the corresponding family to be skipped. Each route is wrapped in
// authMiddleware so cross-tenant access requires an authenticated DM session.
func mountDashboardAPIs(r chi.Router, deps dashboardAPIDeps) {
	if r == nil {
		return
	}
	authMw := deps.authMiddleware
	if authMw == nil {
		authMw = func(next http.Handler) http.Handler { return next }
	}

	mountLootRoutes(r, authMw, deps.lootHandler, deps.queries)
	mountItemPickerRoutes(r, authMw, deps.itemPickerHandler, deps.queries)
	mountShopsRoutes(r, authMw, deps.shopsHandler, deps.queries)
	mountPartyRestRoutes(r, authMw, deps.partyRestHandler)
}

// mountLootRoutes mounts the loot pool dashboard endpoints. URL patterns
// match the chi.URLParam("encounterID") expectations in api_handler.go.
func mountLootRoutes(r chi.Router, authMw func(http.Handler) http.Handler, h *loot.APIHandler, q *refdata.Queries) {
	if h == nil {
		if q == nil {
			return
		}
		h = loot.NewAPIHandler(loot.NewService(q))
	}
	r.Route("/api/campaigns/{campaignID}/encounters/{encounterID}/loot", func(r chi.Router) {
		r.Use(authMw)
		r.Get("/", h.HandleGetLootPool)
		r.Post("/", h.HandleCreateLootPool)
		r.Delete("/", h.HandleClearPool)
		r.Post("/items", h.HandleAddItem)
		r.Delete("/items/{itemID}", h.HandleRemoveItem)
		r.Patch("/items/{itemID}", h.HandleUpdateItem)
		r.Post("/split-gold", h.HandleSplitGold)
		r.Post("/post", h.HandlePostAnnouncement)
		r.Put("/gold", h.HandleSetGold)
	})
	// F-13: lists encounters eligible for a loot pool (campaign-scoped).
	r.With(authMw).Get("/api/campaigns/{campaignID}/loot/eligible-encounters", h.HandleListEligibleEncounters)
}

// mountItemPickerRoutes delegates to itempicker.RegisterRoutes which already
// knows the canonical URL patterns for /search and /creature-inventories.
func mountItemPickerRoutes(r chi.Router, authMw func(http.Handler) http.Handler, h *itempicker.Handler, q *refdata.Queries) {
	if h == nil {
		if q == nil {
			return
		}
		h = itempicker.NewHandler(&itemPickerStore{q: q})
	}
	itempicker.RegisterRoutes(r, h, authMw)
}

// itemPickerStore wraps refdata.Queries to satisfy itempicker.Store,
// adding static gear and consumable data (G-H06).
type itemPickerStore struct {
	q *refdata.Queries
}

func (s *itemPickerStore) ListWeapons(ctx context.Context) ([]refdata.Weapon, error) {
	return s.q.ListWeapons(ctx)
}
func (s *itemPickerStore) ListArmor(ctx context.Context) ([]refdata.Armor, error) {
	return s.q.ListArmor(ctx)
}
func (s *itemPickerStore) ListMagicItems(ctx context.Context) ([]refdata.MagicItem, error) {
	return s.q.ListMagicItems(ctx)
}
func (s *itemPickerStore) ListGear(_ context.Context) ([]itempicker.GearItem, error) {
	return itempicker.StaticGear(), nil
}
func (s *itemPickerStore) ListConsumables(_ context.Context) ([]itempicker.ConsumableItem, error) {
	return itempicker.StaticConsumables(), nil
}
func (s *itemPickerStore) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return s.q.ListCombatantsByEncounterID(ctx, encounterID)
}
func (s *itemPickerStore) GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
	return s.q.GetCharacter(ctx, id)
}

// mountShopsRoutes delegates to shops.RegisterRoutes.
func mountShopsRoutes(r chi.Router, authMw func(http.Handler) http.Handler, h *shops.Handler, q *refdata.Queries) {
	if h == nil {
		if q == nil {
			return
		}
		h = shops.NewHandler(shops.NewService(q))
	}
	shops.RegisterRoutes(r, h, authMw)
}

// mountPartyRestRoutes registers the party-rest endpoint on the chi router.
// The chunk6 follow-up specifies POST /api/campaigns/{campaignID}/party-rest
// as the canonical Svelte target; we mount the routes individually rather
// than via chi.Route so the shared /api/campaigns/{campaignID} prefix does
// not collide with shops / item-picker routes (chi panics on duplicate
// Mount of the same prefix).
func mountPartyRestRoutes(r chi.Router, authMw func(http.Handler) http.Handler, h *rest.PartyRestHandler) {
	if h == nil {
		return
	}
	r.With(authMw).Post("/api/campaigns/{campaignID}/party-rest", h.HandlePartyRest)
	r.With(authMw).Post("/api/campaigns/{campaignID}/interrupt-rest", h.HandleInterruptRest)
}

// --- Party-rest adapters ---
//
// The PartyRestHandler interfaces (PartyCharacterLister, PartyCharacterUpdater,
// PartyEncounterChecker, PartyPlayerNotifier, PartySummaryPoster) are
// satisfied below by light wrappers around *refdata.Queries plus a Discord
// session for player DMs and #roll-history posts. Each is nil-safe so the
// helper degrades gracefully when the DB or session is absent.

// partyCharacterListerAdapter implements rest.PartyCharacterLister by
// reading every approved player character in a campaign and projecting the
// *refdata.Character row into rest.PartyCharacterInfo.
type partyCharacterListerAdapter struct {
	queries *refdata.Queries
}

func newPartyCharacterListerAdapter(q *refdata.Queries) *partyCharacterListerAdapter {
	if q == nil {
		return nil
	}
	return &partyCharacterListerAdapter{queries: q}
}

func (a *partyCharacterListerAdapter) ListPartyCharacters(ctx context.Context, campaignID uuid.UUID) ([]rest.PartyCharacterInfo, error) {
	pcs, err := a.queries.ListPlayerCharactersByCampaignApproved(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("listing approved player characters: %w", err)
	}
	out := make([]rest.PartyCharacterInfo, 0, len(pcs))
	for _, pc := range pcs {
		ch, err := a.queries.GetCharacter(ctx, pc.CharacterID)
		if err != nil {
			continue
		}
		info, err := characterToPartyInfo(ch, pc.DiscordUserID)
		if err != nil {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

// characterToPartyInfo decodes the JSONB blobs on a refdata.Character row
// into the rest.PartyCharacterInfo struct the rest service consumes. Any
// field that fails to parse is left at the type's zero value so the rest
// flow can still complete (a missing class entry simply means no hit dice).
func characterToPartyInfo(ch refdata.Character, discordUserID string) (rest.PartyCharacterInfo, error) {
	info := rest.PartyCharacterInfo{
		ID:               ch.ID,
		Name:             ch.Name,
		DiscordUserID:    discordUserID,
		HPCurrent:        int(ch.HpCurrent),
		HPMax:            int(ch.HpMax),
		HitDiceRemaining: map[string]int{},
		FeatureUses:      map[string]character.FeatureUse{},
		SpellSlots:       map[string]character.SlotInfo{},
	}

	var scores character.AbilityScores
	if err := json.Unmarshal(ch.AbilityScores, &scores); err == nil {
		info.CONModifier = character.AbilityModifier(scores.CON)
	}

	if err := json.Unmarshal(ch.Classes, &info.Classes); err != nil {
		// Leave classes empty; rest flow handles the empty case.
		info.Classes = nil
	}

	if len(ch.HitDiceRemaining) > 0 {
		_ = json.Unmarshal(ch.HitDiceRemaining, &info.HitDiceRemaining)
	}

	if ch.FeatureUses.Valid {
		_ = json.Unmarshal(ch.FeatureUses.RawMessage, &info.FeatureUses)
	}

	if ch.SpellSlots.Valid {
		_ = json.Unmarshal(ch.SpellSlots.RawMessage, &info.SpellSlots)
	}

	if ch.PactMagicSlots.Valid {
		var pact character.PactMagicSlots
		if err := json.Unmarshal(ch.PactMagicSlots.RawMessage, &pact); err == nil && pact.Max > 0 {
			info.PactMagicSlots = &pact
		}
	}
	if ch.CharacterData.Valid {
		if exhaustion, ok := rest.ExhaustionLevelFromCharacterData(ch.CharacterData.RawMessage); ok {
			info.ExhaustionLevel = exhaustion
		}
	}

	return info, nil
}

// partyCharacterUpdaterAdapter implements rest.PartyCharacterUpdater by
// merging the rest-result JSONB fields back into the character row via
// UpdateCharacter.
type partyCharacterUpdaterAdapter struct {
	queries *refdata.Queries
}

func newPartyCharacterUpdaterAdapter(q *refdata.Queries) *partyCharacterUpdaterAdapter {
	if q == nil {
		return nil
	}
	return &partyCharacterUpdaterAdapter{queries: q}
}

func (a *partyCharacterUpdaterAdapter) ApplyRestUpdate(ctx context.Context, u rest.CharacterRestUpdate) error {
	ch, err := a.queries.GetCharacter(ctx, u.CharacterID)
	if err != nil {
		return fmt.Errorf("loading character %s: %w", u.CharacterID, err)
	}

	params := basePartyUpdateParams(ch)
	params.HpCurrent = int32(u.HPCurrent)

	if hitDiceJSON, err := json.Marshal(u.HitDiceRemaining); err == nil {
		params.HitDiceRemaining = hitDiceJSON
	}
	if featureJSON, err := json.Marshal(u.FeatureUses); err == nil {
		params.FeatureUses = pqtype.NullRawMessage{RawMessage: featureJSON, Valid: true}
	}
	if u.SpellSlots != nil {
		if slotJSON, err := json.Marshal(u.SpellSlots); err == nil {
			params.SpellSlots = pqtype.NullRawMessage{RawMessage: slotJSON, Valid: true}
		}
	}
	if u.PactMagicSlots != nil {
		if pactJSON, err := json.Marshal(u.PactMagicSlots); err == nil {
			params.PactMagicSlots = pqtype.NullRawMessage{RawMessage: pactJSON, Valid: true}
		}
	}
	params.CharacterData = pqtype.NullRawMessage{
		RawMessage: rest.CharacterDataWithExhaustion(ch.CharacterData.RawMessage, u.ExhaustionLevel),
		Valid:      true,
	}

	_, err = a.queries.UpdateCharacter(ctx, params)
	return err
}

// basePartyUpdateParams mirrors discord.baseUpdateParams: copies every
// field from the source character so callers only override the rest-mutated
// columns. Kept separate from the discord-package helper to avoid an import
// cycle (cmd/dndnd already imports internal/discord; copying the struct
// init here is the trivial fix).
func basePartyUpdateParams(ch refdata.Character) refdata.UpdateCharacterParams {
	return refdata.UpdateCharacterParams{
		ID:               ch.ID,
		Name:             ch.Name,
		Race:             ch.Race,
		Classes:          ch.Classes,
		Level:            ch.Level,
		AbilityScores:    ch.AbilityScores,
		HpMax:            ch.HpMax,
		HpCurrent:        ch.HpCurrent,
		TempHp:           ch.TempHp,
		Ac:               ch.Ac,
		AcFormula:        ch.AcFormula,
		SpeedFt:          ch.SpeedFt,
		ProficiencyBonus: ch.ProficiencyBonus,
		EquippedMainHand: ch.EquippedMainHand,
		EquippedOffHand:  ch.EquippedOffHand,
		EquippedArmor:    ch.EquippedArmor,
		SpellSlots:       ch.SpellSlots,
		PactMagicSlots:   ch.PactMagicSlots,
		HitDiceRemaining: ch.HitDiceRemaining,
		FeatureUses:      ch.FeatureUses,
		Features:         ch.Features,
		Proficiencies:    ch.Proficiencies,
		Gold:             ch.Gold,
		AttunementSlots:  ch.AttunementSlots,
		Languages:        ch.Languages,
		Inventory:        ch.Inventory,
		CharacterData:    ch.CharacterData,
		DdbUrl:           ch.DdbUrl,
		Homebrew:         ch.Homebrew,
	}
}

// partyEncounterCheckerAdapter implements rest.PartyEncounterChecker by
// scanning the campaign's encounters for one in `status="active"`.
type partyEncounterCheckerAdapter struct {
	queries *refdata.Queries
}

func newPartyEncounterCheckerAdapter(q *refdata.Queries) *partyEncounterCheckerAdapter {
	if q == nil {
		return nil
	}
	return &partyEncounterCheckerAdapter{queries: q}
}

func (a *partyEncounterCheckerAdapter) HasActiveEncounter(ctx context.Context, campaignID uuid.UUID) bool {
	encs, err := a.queries.ListEncountersByCampaignID(ctx, campaignID)
	if err != nil {
		return false
	}
	for _, e := range encs {
		if e.Status == "active" {
			return true
		}
	}
	return false
}

// partyPlayerNotifierAdapter implements rest.PartyPlayerNotifier by sending
// a Discord DM to the player.
type partyPlayerNotifierAdapter struct {
	dm playerDirectMessenger
}

func newPartyPlayerNotifierAdapter(dm playerDirectMessenger) *partyPlayerNotifierAdapter {
	if dm == nil {
		return nil
	}
	return &partyPlayerNotifierAdapter{dm: dm}
}

func (a *partyPlayerNotifierAdapter) NotifyPlayer(_ context.Context, n rest.PlayerNotification) error {
	if n.DiscordUserID == "" {
		return nil
	}
	_, err := a.dm.SendDirectMessage(n.DiscordUserID, n.Message)
	return err
}

// partySummaryPosterAdapter implements rest.PartySummaryPoster by posting
// the rest summary to the campaign's #roll-history channel via Discord.
// Best-effort: when channel lookup fails or the campaign has no
// roll-history channel configured, the post is silently skipped.
type partySummaryPosterAdapter struct {
	session     discord.Session
	queries     *refdata.Queries
	channelProv *campaignChannelLookup
}

func newPartySummaryPosterAdapter(s discord.Session, q *refdata.Queries) *partySummaryPosterAdapter {
	if s == nil || q == nil {
		return nil
	}
	return &partySummaryPosterAdapter{session: s, queries: q, channelProv: newCampaignChannelLookup(q)}
}

func (a *partySummaryPosterAdapter) PostToRollHistory(ctx context.Context, campaignID uuid.UUID, msg string) error {
	if a.session == nil || a.channelProv == nil {
		return nil
	}
	channelIDs, err := a.channelProv.GetChannelIDsForCampaign(ctx, campaignID)
	if err != nil {
		return nil
	}
	rollHistoryCh, ok := channelIDs["roll-history"]
	if !ok || rollHistoryCh == "" {
		return nil
	}
	_, _ = a.session.ChannelMessageSend(rollHistoryCh, msg)
	return nil
}

// noopPartyPlayerNotifier satisfies rest.PartyPlayerNotifier when no
// Discord session is available (test deploys). PartyRestHandler calls the
// notifier directly without a nil guard, so we substitute a silent stub.
type noopPartyPlayerNotifier struct{}

func (noopPartyPlayerNotifier) NotifyPlayer(_ context.Context, _ rest.PlayerNotification) error {
	return nil
}

// noopPartySummaryPoster satisfies rest.PartySummaryPoster when no Discord
// session is available. Same nil-guard rationale as noopPartyPlayerNotifier.
type noopPartySummaryPoster struct{}

func (noopPartySummaryPoster) PostToRollHistory(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

// campaignChannelLookup decodes campaign settings to expose channel_ids
// keyed by campaign id (rather than encounter id like
// CampaignSettingsProvider). The party-rest poster needs this because rest
// is a campaign-level event, not bound to a specific encounter.
type campaignChannelLookup struct {
	queries *refdata.Queries
}

func newCampaignChannelLookup(q *refdata.Queries) *campaignChannelLookup {
	if q == nil {
		return nil
	}
	return &campaignChannelLookup{queries: q}
}

func (l *campaignChannelLookup) GetChannelIDsForCampaign(ctx context.Context, campaignID uuid.UUID) (map[string]string, error) {
	camp, err := l.queries.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if !camp.Settings.Valid {
		return map[string]string{}, nil
	}
	var settings struct {
		ChannelIDs map[string]string `json:"channel_ids"`
	}
	if err := json.Unmarshal(camp.Settings.RawMessage, &settings); err != nil {
		return nil, err
	}
	return settings.ChannelIDs, nil
}

// handleDMMapPNG returns an http.HandlerFunc that renders the encounter map
// with DMSeesAll=true (unfogged) and serves it as image/png. SR-068: gives
// the DM dashboard a production caller for RegenerateMapForDM.
func handleDMMapPNG(regen *mapRegeneratorAdapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "encounterID")
		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "invalid encounter id", http.StatusBadRequest)
			return
		}
		png, err := regen.RegenerateMapForDM(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(png)
	}
}
