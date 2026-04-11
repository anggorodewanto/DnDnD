package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// --- fakes ---

type fakeReactionService struct {
	canDeclare        bool
	canDeclareErr     error
	declareResult     refdata.ReactionDeclaration
	declareErr        error
	declareCalledWith struct {
		encounterID uuid.UUID
		combatantID uuid.UUID
		description string
	}
	cancelResult     refdata.ReactionDeclaration
	cancelErr        error
	cancelCalledWith struct {
		combatantID uuid.UUID
		encounterID uuid.UUID
		substring   string
	}
	cancelAllErr        error
	cancelAllCalledWith struct {
		combatantID uuid.UUID
		encounterID uuid.UUID
	}
	listResult []refdata.ReactionDeclaration
	listErr    error
}

func (f *fakeReactionService) CanDeclareReaction(ctx context.Context, encounterID, combatantID uuid.UUID) (bool, error) {
	return f.canDeclare, f.canDeclareErr
}

func (f *fakeReactionService) DeclareReaction(ctx context.Context, encounterID, combatantID uuid.UUID, description string) (refdata.ReactionDeclaration, error) {
	f.declareCalledWith.encounterID = encounterID
	f.declareCalledWith.combatantID = combatantID
	f.declareCalledWith.description = description
	return f.declareResult, f.declareErr
}

func (f *fakeReactionService) CancelReactionByDescription(ctx context.Context, combatantID, encounterID uuid.UUID, descSubstring string) (refdata.ReactionDeclaration, error) {
	f.cancelCalledWith.combatantID = combatantID
	f.cancelCalledWith.encounterID = encounterID
	f.cancelCalledWith.substring = descSubstring
	return f.cancelResult, f.cancelErr
}

func (f *fakeReactionService) CancelAllReactions(ctx context.Context, combatantID, encounterID uuid.UUID) error {
	f.cancelAllCalledWith.combatantID = combatantID
	f.cancelAllCalledWith.encounterID = encounterID
	return f.cancelAllErr
}

func (f *fakeReactionService) ListReactionsByCombatant(ctx context.Context, combatantID, encounterID uuid.UUID) ([]refdata.ReactionDeclaration, error) {
	return f.listResult, f.listErr
}

type fakeReactionEncounterResolver struct {
	encounterID uuid.UUID
	err         error
}

func (f *fakeReactionEncounterResolver) ActiveEncounterForUser(ctx context.Context, guildID, userID string) (uuid.UUID, error) {
	return f.encounterID, f.err
}

type fakeReactionCombatantLookup struct {
	combatantID uuid.UUID
	displayName string
	err         error
}

func (f *fakeReactionCombatantLookup) GetCombatantIDByDiscordUser(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, string, error) {
	return f.combatantID, f.displayName, f.err
}

// cancelRecordingNotifier captures Post and Cancel calls so tests can assert
// handler→notifier interactions.
type cancelRecordingNotifier struct {
	posted     []dmqueue.Event
	nextItemID string
	postErr    error

	cancelCalls []struct {
		itemID string
		reason string
	}
	cancelErr error
}

func (c *cancelRecordingNotifier) Post(_ context.Context, e dmqueue.Event) (string, error) {
	c.posted = append(c.posted, e)
	if c.postErr != nil {
		return "", c.postErr
	}
	if c.nextItemID == "" {
		return "item-default", nil
	}
	return c.nextItemID, nil
}

func (c *cancelRecordingNotifier) Cancel(_ context.Context, itemID, reason string) error {
	c.cancelCalls = append(c.cancelCalls, struct {
		itemID string
		reason string
	}{itemID: itemID, reason: reason})
	return c.cancelErr
}

func (c *cancelRecordingNotifier) Resolve(_ context.Context, _, _ string) error { return nil }
func (c *cancelRecordingNotifier) ResolveWhisper(_ context.Context, _, _ string) error {
	return nil
}
func (c *cancelRecordingNotifier) ResolveSkillCheckNarration(_ context.Context, _, _ string) error {
	return nil
}
func (c *cancelRecordingNotifier) Get(string) (dmqueue.Item, bool) { return dmqueue.Item{}, false }
func (c *cancelRecordingNotifier) ListPending() []dmqueue.Item     { return nil }

// --- helpers ---

func makeReactionInteraction(guildID, userID, subcommand, description string) *discordgo.Interaction {
	sub := &discordgo.ApplicationCommandInteractionDataOption{
		Name: subcommand,
		Type: discordgo.ApplicationCommandOptionSubCommand,
	}
	if description != "" {
		sub.Options = []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "description", Type: discordgo.ApplicationCommandOptionString, Value: description},
		}
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "reaction",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{sub},
		},
	}
}

func newTestReactionHandler(svc *fakeReactionService, enc uuid.UUID, combatantID uuid.UUID) (*ReactionHandler, *mockInventorySession, *fakeReactionService) {
	sess := &mockInventorySession{}
	if svc == nil {
		svc = &fakeReactionService{canDeclare: true}
	}
	resolver := &fakeReactionEncounterResolver{encounterID: enc}
	lookup := &fakeReactionCombatantLookup{combatantID: combatantID, displayName: "Aria"}
	h := NewReactionHandler(sess, svc, resolver, lookup)
	return h, sess, svc
}

// --- tests ---

func TestReactionHandler_Declare_PostsToDMQueue(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()

	svc := &fakeReactionService{
		canDeclare:    true,
		declareResult: refdata.ReactionDeclaration{ID: declID, Description: "Shield if I get hit", Status: "active"},
	}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)

	rec := &cancelRecordingNotifier{nextItemID: "item-xyz"}
	h.SetNotifier(rec)

	interaction := makeReactionInteraction("guild1", "user1", "declare", "Shield if I get hit")
	h.Handle(interaction)

	require.Len(t, rec.posted, 1)
	ev := rec.posted[0]
	assert.Equal(t, dmqueue.KindReactionDeclaration, ev.Kind)
	assert.Equal(t, "Aria", ev.PlayerName)
	assert.Contains(t, ev.Summary, "Shield if I get hit")
	assert.Equal(t, "guild1", ev.GuildID)
	assert.Equal(t, declID.String(), ev.ExtraMetadata["reaction_declaration_id"])

	// Player gets an ephemeral confirmation.
	assert.Contains(t, sess.lastResponse, "Shield if I get hit")

	// Service received the trimmed description and proper IDs.
	assert.Equal(t, enc, svc.declareCalledWith.encounterID)
	assert.Equal(t, combatantID, svc.declareCalledWith.combatantID)
	assert.Equal(t, "Shield if I get hit", svc.declareCalledWith.description)

	// Handler recorded the item ID keyed by declaration ID for later cancel.
	assert.Equal(t, "item-xyz", h.ItemIDForDeclaration(declID))
}

func TestReactionHandler_Declare_EmptyDescription(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	h, sess, svc := newTestReactionHandler(nil, enc, combatantID)
	rec := &cancelRecordingNotifier{}
	h.SetNotifier(rec)

	interaction := makeReactionInteraction("guild1", "user1", "declare", "   ")
	h.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "description")
	assert.Empty(t, rec.posted)
	assert.Empty(t, svc.declareCalledWith.description)
}

func TestReactionHandler_Declare_NoActiveEncounter(t *testing.T) {
	sess := &mockInventorySession{}
	svc := &fakeReactionService{canDeclare: true}
	resolver := &fakeReactionEncounterResolver{err: errors.New("no encounter")}
	lookup := &fakeReactionCombatantLookup{combatantID: uuid.New()}
	h := NewReactionHandler(sess, svc, resolver, lookup)

	interaction := makeReactionInteraction("guild1", "user1", "declare", "Shield")
	h.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "not in an active encounter")
}

func TestReactionHandler_Declare_CombatantLookupFailure(t *testing.T) {
	sess := &mockInventorySession{}
	svc := &fakeReactionService{canDeclare: true}
	resolver := &fakeReactionEncounterResolver{encounterID: uuid.New()}
	lookup := &fakeReactionCombatantLookup{err: errors.New("not a combatant")}
	h := NewReactionHandler(sess, svc, resolver, lookup)

	interaction := makeReactionInteraction("guild1", "user1", "declare", "Shield")
	h.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "combatant")
}

func TestReactionHandler_Declare_ReactionAlreadyUsed(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	svc := &fakeReactionService{canDeclare: false}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)
	rec := &cancelRecordingNotifier{}
	h.SetNotifier(rec)

	interaction := makeReactionInteraction("guild1", "user1", "declare", "Shield")
	h.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "already used your reaction")
	assert.Empty(t, rec.posted, "must not post to dm-queue when reaction unavailable")
	assert.Empty(t, svc.declareCalledWith.description, "must not persist declaration")
}

func TestReactionHandler_Declare_ServiceError(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	svc := &fakeReactionService{canDeclare: true, declareErr: errors.New("db broken")}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)
	rec := &cancelRecordingNotifier{}
	h.SetNotifier(rec)

	interaction := makeReactionInteraction("guild1", "user1", "declare", "Shield")
	h.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed")
	assert.Empty(t, rec.posted)
}

func TestReactionHandler_Declare_CanDeclareError(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	svc := &fakeReactionService{canDeclareErr: errors.New("db down")}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)

	interaction := makeReactionInteraction("guild1", "user1", "declare", "Shield")
	h.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed")
}

func TestReactionHandler_Declare_NilNotifierIsSilent(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()
	svc := &fakeReactionService{
		canDeclare:    true,
		declareResult: refdata.ReactionDeclaration{ID: declID, Description: "Shield"},
	}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)
	// no SetNotifier call — must not panic.

	interaction := makeReactionInteraction("guild1", "user1", "declare", "Shield")
	h.Handle(interaction)

	// The declaration still persists, and the player gets feedback.
	assert.Equal(t, "Shield", svc.declareCalledWith.description)
	assert.Contains(t, sess.lastResponse, "Shield")
}

func TestReactionHandler_Declare_PostFailureStillConfirmsToPlayer(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()
	svc := &fakeReactionService{
		canDeclare:    true,
		declareResult: refdata.ReactionDeclaration{ID: declID, Description: "Shield"},
	}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)
	rec := &cancelRecordingNotifier{postErr: errors.New("discord down")}
	h.SetNotifier(rec)

	interaction := makeReactionInteraction("guild1", "user1", "declare", "Shield")
	h.Handle(interaction)

	// Declaration persisted; player still gets ephemeral ack.
	assert.Equal(t, "Shield", svc.declareCalledWith.description)
	assert.Contains(t, sess.lastResponse, "Shield")
	// No item ID stashed because Post returned error.
	assert.Empty(t, h.ItemIDForDeclaration(declID))
}

func TestReactionHandler_Cancel_EditsDMQueueMessage(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()
	svc := &fakeReactionService{
		canDeclare:    true,
		declareResult: refdata.ReactionDeclaration{ID: declID, Description: "Shield if I get hit"},
		cancelResult:  refdata.ReactionDeclaration{ID: declID, Description: "Shield if I get hit", Status: "cancelled"},
	}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)
	rec := &cancelRecordingNotifier{nextItemID: "item-abc"}
	h.SetNotifier(rec)

	// First declare to build the item mapping.
	h.Handle(makeReactionInteraction("guild1", "user1", "declare", "Shield if I get hit"))

	// Then cancel with a substring match.
	h.Handle(makeReactionInteraction("guild1", "user1", "cancel", "shield"))

	require.Len(t, rec.cancelCalls, 1)
	assert.Equal(t, "item-abc", rec.cancelCalls[0].itemID)
	assert.NotEmpty(t, rec.cancelCalls[0].reason)
	assert.Equal(t, combatantID, svc.cancelCalledWith.combatantID)
	assert.Equal(t, enc, svc.cancelCalledWith.encounterID)
	assert.Equal(t, "shield", svc.cancelCalledWith.substring)
	assert.Contains(t, sess.lastResponse, "Cancelled")
	// Mapping cleaned up after cancel.
	assert.Empty(t, h.ItemIDForDeclaration(declID))
}

func TestReactionHandler_Cancel_NoMatch(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	svc := &fakeReactionService{cancelErr: errors.New("no active reaction matching \"xyz\"")}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)
	rec := &cancelRecordingNotifier{}
	h.SetNotifier(rec)

	h.Handle(makeReactionInteraction("guild1", "user1", "cancel", "xyz"))

	assert.Contains(t, sess.lastResponse, "No active reaction")
	assert.Empty(t, rec.cancelCalls)
}

func TestReactionHandler_Cancel_EmptySubstring(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	h, sess, _ := newTestReactionHandler(nil, enc, combatantID)

	h.Handle(makeReactionInteraction("guild1", "user1", "cancel", "   "))

	assert.Contains(t, sess.lastResponse, "description")
}

func TestReactionHandler_CancelAll_Success(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	decl1 := uuid.New()
	decl2 := uuid.New()

	svc := &fakeReactionService{
		canDeclare: true,
		listResult: []refdata.ReactionDeclaration{
			{ID: decl1, Description: "Shield", Status: "active"},
			{ID: decl2, Description: "OA if G1 moves", Status: "active"},
		},
	}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)
	rec := &cancelRecordingNotifier{nextItemID: "item-aa"}
	h.SetNotifier(rec)

	// Seed the item mapping via two declares.
	svc.declareResult = refdata.ReactionDeclaration{ID: decl1}
	h.Handle(makeReactionInteraction("guild1", "user1", "declare", "Shield"))

	rec.nextItemID = "item-bb"
	svc.declareResult = refdata.ReactionDeclaration{ID: decl2}
	h.Handle(makeReactionInteraction("guild1", "user1", "declare", "OA if G1 moves"))

	// Now cancel-all.
	h.Handle(makeReactionInteraction("guild1", "user1", "cancel-all", ""))

	assert.Equal(t, combatantID, svc.cancelAllCalledWith.combatantID)
	assert.Equal(t, enc, svc.cancelAllCalledWith.encounterID)
	// Both queued items were cancelled on the notifier.
	assert.Len(t, rec.cancelCalls, 2)
	assert.Contains(t, sess.lastResponse, "Cancelled")
}

func TestReactionHandler_CancelAll_ServiceError(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	svc := &fakeReactionService{cancelAllErr: errors.New("boom")}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)

	h.Handle(makeReactionInteraction("guild1", "user1", "cancel-all", ""))
	assert.Contains(t, sess.lastResponse, "Failed")
}

func TestCommandRouter_SetReactionHandler_Overrides(t *testing.T) {
	sess := &mockInventorySession{}
	bot := &Bot{session: sess}
	r := NewCommandRouter(bot, nil)

	enc := uuid.New()
	h := NewReactionHandler(sess, &fakeReactionService{canDeclare: true}, &fakeReactionEncounterResolver{encounterID: enc}, &fakeReactionCombatantLookup{combatantID: uuid.New()})
	r.SetReactionHandler(h)

	// Dispatch a /reaction interaction; it should hit our handler, not the stub.
	interaction := makeReactionInteraction("guild1", "user1", "cancel-all", "")
	r.Handle(interaction)

	// The stub would have said "/reaction is not yet implemented." — ours
	// replies with a cancel-all confirmation.
	assert.Contains(t, sess.lastResponse, "Cancelled all")
}

func TestReactionHandler_UnknownSubcommand(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	h, sess, _ := newTestReactionHandler(nil, enc, combatantID)

	h.Handle(makeReactionInteraction("guild1", "user1", "explode", ""))
	assert.Contains(t, sess.lastResponse, "Unknown")
}

func TestReactionHandler_NoSubcommand(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	h, sess, _ := newTestReactionHandler(nil, enc, combatantID)

	// Build an interaction with no options at all.
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "reaction",
		},
	}
	h.Handle(interaction)
	assert.Contains(t, sess.lastResponse, "Unknown")
}

func TestReactionHandler_Cancel_NoActiveEncounter(t *testing.T) {
	sess := &mockInventorySession{}
	svc := &fakeReactionService{}
	resolver := &fakeReactionEncounterResolver{err: errors.New("no encounter")}
	lookup := &fakeReactionCombatantLookup{combatantID: uuid.New()}
	h := NewReactionHandler(sess, svc, resolver, lookup)

	h.Handle(makeReactionInteraction("guild1", "user1", "cancel", "shield"))
	assert.Contains(t, sess.lastResponse, "not in an active encounter")
}

func TestReactionHandler_Cancel_GenericError(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	svc := &fakeReactionService{cancelErr: errors.New("db went sideways")}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)

	h.Handle(makeReactionInteraction("guild1", "user1", "cancel", "shield"))
	assert.Contains(t, sess.lastResponse, "Failed to cancel reaction")
}

func TestReactionHandler_CancelAll_NoActiveEncounter(t *testing.T) {
	sess := &mockInventorySession{}
	svc := &fakeReactionService{}
	resolver := &fakeReactionEncounterResolver{err: errors.New("no encounter")}
	lookup := &fakeReactionCombatantLookup{combatantID: uuid.New()}
	h := NewReactionHandler(sess, svc, resolver, lookup)

	h.Handle(makeReactionInteraction("guild1", "user1", "cancel-all", ""))
	assert.Contains(t, sess.lastResponse, "not in an active encounter")
}

func TestReactionHandler_Declare_DescriptionOptionMissing(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	h, sess, _ := newTestReactionHandler(nil, enc, combatantID)

	// Build a /reaction declare interaction with NO description option at
	// all (the subcommand fires with an empty Options list). This exercises
	// reactionStringOption's "not found" path.
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "reaction",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "declare", Type: discordgo.ApplicationCommandOptionSubCommand},
			},
		},
	}
	h.Handle(interaction)
	assert.Contains(t, sess.lastResponse, "description")
}

// perUserLookup maps Discord user IDs to (combatantID, displayName) so a
// single ReactionHandler can service multiple players in one test.
type perUserLookup struct {
	byUser map[string]struct {
		combatantID uuid.UUID
		displayName string
	}
}

func (p *perUserLookup) GetCombatantIDByDiscordUser(_ context.Context, _ uuid.UUID, discordUserID string) (uuid.UUID, string, error) {
	entry, ok := p.byUser[discordUserID]
	if !ok {
		return uuid.Nil, "", errors.New("not a combatant")
	}
	return entry.combatantID, entry.displayName, nil
}

// perCombatantService mimics combat.Service's per-combatant DeclareReaction
// behavior by returning a caller-supplied declaration ID based on the
// combatantID argument, so a single handler instance can persist two distinct
// declarations for two players in one test.
type perCombatantService struct {
	fakeReactionService
	nextDeclByCombatant map[uuid.UUID]refdata.ReactionDeclaration
}

func (p *perCombatantService) DeclareReaction(_ context.Context, encounterID, combatantID uuid.UUID, description string) (refdata.ReactionDeclaration, error) {
	p.declareCalledWith.encounterID = encounterID
	p.declareCalledWith.combatantID = combatantID
	p.declareCalledWith.description = description
	if decl, ok := p.nextDeclByCombatant[combatantID]; ok {
		return decl, nil
	}
	return refdata.ReactionDeclaration{}, errors.New("no declaration seeded for combatant")
}

func TestReactionHandler_CancelAll_DoesNotTouchOtherCombatants(t *testing.T) {
	enc := uuid.New()
	combatantA := uuid.New()
	combatantB := uuid.New()
	declA := uuid.New()
	declB := uuid.New()

	svc := &perCombatantService{
		fakeReactionService: fakeReactionService{canDeclare: true},
		nextDeclByCombatant: map[uuid.UUID]refdata.ReactionDeclaration{
			combatantA: {ID: declA, Description: "Shield A", Status: "active"},
			combatantB: {ID: declB, Description: "Shield B", Status: "active"},
		},
	}

	sess := &mockInventorySession{}
	resolver := &fakeReactionEncounterResolver{encounterID: enc}
	lookup := &perUserLookup{byUser: map[string]struct {
		combatantID uuid.UUID
		displayName string
	}{
		"userA": {combatantID: combatantA, displayName: "Aria"},
		"userB": {combatantID: combatantB, displayName: "Bran"},
	}}
	h := NewReactionHandler(sess, svc, resolver, lookup)

	rec := &cancelRecordingNotifier{}
	h.SetNotifier(rec)

	// Player A declares — handler stashes itemID_A.
	rec.nextItemID = "item-A"
	h.Handle(makeReactionInteraction("guild1", "userA", "declare", "Shield A"))

	// Player B declares — handler stashes itemID_B.
	rec.nextItemID = "item-B"
	h.Handle(makeReactionInteraction("guild1", "userB", "declare", "Shield B"))

	// Sanity: both stashed under their own declaration IDs.
	require.Equal(t, "item-A", h.ItemIDForDeclaration(declA))
	require.Equal(t, "item-B", h.ItemIDForDeclaration(declB))

	// Player B runs /reaction cancel-all.
	h.Handle(makeReactionInteraction("guild1", "userB", "cancel-all", ""))

	// Only Player B's item should have been cancelled on the notifier.
	require.Len(t, rec.cancelCalls, 1, "cancel-all must not leak across combatants")
	assert.Equal(t, "item-B", rec.cancelCalls[0].itemID)

	// Player A's stashed entry must still be present so a subsequent
	// Player A /reaction cancel would still find it.
	assert.Equal(t, "item-A", h.ItemIDForDeclaration(declA))
	// Player B's entry is cleared.
	assert.Empty(t, h.ItemIDForDeclaration(declB))
}

func TestReactionHandler_CancelDMQueueItem_NoNotifier(t *testing.T) {
	enc := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()
	svc := &fakeReactionService{
		canDeclare:    true,
		declareResult: refdata.ReactionDeclaration{ID: declID, Description: "Shield"},
		cancelResult:  refdata.ReactionDeclaration{ID: declID, Description: "Shield", Status: "cancelled"},
	}
	h, sess, _ := newTestReactionHandler(svc, enc, combatantID)
	// No notifier wired — declare + cancel must still respond cleanly.

	h.Handle(makeReactionInteraction("guild1", "user1", "declare", "Shield"))
	h.Handle(makeReactionInteraction("guild1", "user1", "cancel", "shield"))
	assert.Contains(t, sess.lastResponse, "Cancelled")
}
