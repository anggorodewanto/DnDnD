package main

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/refdata"
)

// economyPostFunc re-posts the active combatant's remaining action economy to
// #your-turn after an economy-consuming action persists. It is fired from the
// persistence-boundary decorators below so EVERY action — the combat-package
// commands plus the Discord move/fly/use/give handlers — re-posts the running
// tally without each handler having to know about #your-turn. UpdateTurnActions
// is the persistence chokepoint every resource change funnels through, which is
// why decorating it covers them all.
type economyPostFunc func(ctx context.Context, turn refdata.Turn)

// turnEconomyStore is the read slice the poster needs to resolve the combatant
// behind a persisted turn. *refdata.Queries satisfies it.
type turnEconomyStore interface {
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
}

// turnEconomyPoster posts the "📋 Remaining: …" line to #your-turn after each
// economy-consuming action. It reuses combat.FormatRemainingResources so the
// wording matches the after-command line already shown in #combat-log. Posts
// fire only for player turns (NPC economy changes during enemy turns stay
// silent) and only when a #your-turn channel is configured. Best-effort: every
// dependency hiccup is swallowed so a Discord-side error can never roll back
// the persisted action.
type turnEconomyPoster struct {
	session discord.Session
	csp     discord.CampaignSettingsProvider
	store   turnEconomyStore
}

func newTurnEconomyPoster(session discord.Session, csp discord.CampaignSettingsProvider, store turnEconomyStore) *turnEconomyPoster {
	if session == nil || csp == nil || store == nil {
		return nil
	}
	return &turnEconomyPoster{session: session, csp: csp, store: store}
}

// economyPostFor builds the #your-turn economy poster from the Discord handler
// deps and returns its Post callback, or nil when a dependency (notably a
// Discord session) is missing so every wrap site degrades to a plain
// pass-through.
func economyPostFor(deps discordHandlerDeps) economyPostFunc {
	poster := newTurnEconomyPoster(deps.session, deps.campaignSettings, deps.queries)
	if poster == nil {
		return nil
	}
	return poster.Post
}

// Post re-posts the remaining economy asynchronously. It detaches from the
// caller's context — which is frequently a request or DB-transaction context
// about to be cancelled — and recovers from panics so a notifier failure can
// never escape into (or stall) the action path that triggered it.
func (p *turnEconomyPoster) Post(ctx context.Context, turn refdata.Turn) {
	dctx := context.WithoutCancel(ctx)
	go func() {
		defer func() { _ = recover() }()
		p.postSync(dctx, turn)
	}()
}

// postSync is the synchronous core, exercised directly by tests.
func (p *turnEconomyPoster) postSync(ctx context.Context, turn refdata.Turn) {
	channelIDs, err := p.csp.GetChannelIDs(ctx, turn.EncounterID)
	if err != nil {
		return
	}
	yourTurnCh, ok := channelIDs["your-turn"]
	if !ok || yourTurnCh == "" {
		return
	}
	combatant, err := p.store.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		return
	}
	// #your-turn is the player channel — stay silent for NPC/enemy economy
	// changes (e.g. a monster spending its action on its own turn).
	if combatant.IsNpc {
		return
	}
	_, _ = p.session.ChannelMessageSend(yourTurnCh, combat.FormatRemainingResources(turn, &combatant))
}

// economyNotifyingStore decorates a combat.Store so every UpdateTurnActions —
// the persistence chokepoint for the combat-package action handlers (attack,
// standard actions, monk, rage, spellcasting, reactions, …) — re-posts the
// remaining economy to #your-turn on success.
type economyNotifyingStore struct {
	combat.Store
	post economyPostFunc
}

// newEconomyNotifyingStore wraps inner so successful turn-action persists fan a
// #your-turn economy update. A nil inner or nil post returns inner unwrapped so
// headless / Discord-less wiring is a plain pass-through.
func newEconomyNotifyingStore(inner combat.Store, post economyPostFunc) combat.Store {
	if inner == nil || post == nil {
		return inner
	}
	return economyNotifyingStore{Store: inner, post: post}
}

func (s economyNotifyingStore) UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	turn, err := s.Store.UpdateTurnActions(ctx, arg)
	if err != nil {
		return turn, err
	}
	s.post(ctx, turn)
	return turn, nil
}

// economyTurnProvider decorates the move/fly turn provider (the bare
// *refdata.Queries) so /move and /fly re-post the remaining economy after each
// successful persist — one post per /move step. It embeds discord.MoveTurnProvider
// so GetTurn is promoted and only UpdateTurnActions is overridden. Callers
// construct it only with a non-nil post (see economyPostFor wiring).
type economyTurnProvider struct {
	discord.MoveTurnProvider
	post economyPostFunc
}

func newEconomyTurnProvider(inner discord.MoveTurnProvider, post economyPostFunc) *economyTurnProvider {
	return &economyTurnProvider{MoveTurnProvider: inner, post: post}
}

func (p *economyTurnProvider) UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	turn, err := p.MoveTurnProvider.UpdateTurnActions(ctx, arg)
	if err != nil {
		return turn, err
	}
	p.post(ctx, turn)
	return turn, nil
}

// economyUseGiveProvider decorates the /use and /give turn adapter so both
// re-post remaining economy after consuming the per-turn free interaction. It
// embeds discord.UseCombatProvider (GetActiveTurnForCharacter is promoted) and
// satisfies discord.GiveTurnProvider too, since the two interfaces share the
// same method set. Callers construct it only with a non-nil post.
type economyUseGiveProvider struct {
	discord.UseCombatProvider
	post economyPostFunc
}

func (p *economyUseGiveProvider) UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	turn, err := p.UseCombatProvider.UpdateTurnActions(ctx, arg)
	if err != nil {
		return turn, err
	}
	p.post(ctx, turn)
	return turn, nil
}

// SpendTurnResources decorates the targeted CAS spend, which is the path /use
// takes (/give still spends its free interaction through UpdateTurnActions).
// Without this override the embedded provider's method is promoted verbatim
// and /use stops re-posting economy to #your-turn. An error means the spend
// changed nothing — including sql.ErrNoRows for an already-spent resource — so
// there is no new economy to announce.
func (p *economyUseGiveProvider) SpendTurnResources(ctx context.Context, arg refdata.SpendTurnResourcesParams) (refdata.Turn, error) {
	turn, err := p.UseCombatProvider.SpendTurnResources(ctx, arg)
	if err != nil {
		return turn, err
	}
	p.post(ctx, turn)
	return turn, nil
}
