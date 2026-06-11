package discord

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

var _ GiveCharacterStore = (*mockGiveCharacterStore)(nil)

// mockGiveCharacterStore captures updates for both giver and receiver.
type mockGiveCharacterStore struct {
	updates map[uuid.UUID]json.RawMessage
	err     error // return error on update
}

func (m *mockGiveCharacterStore) UpdateTwoCharacterInventories(ctx context.Context, id1 uuid.UUID, inv1 pqtype.NullRawMessage, id2 uuid.UUID, inv2 pqtype.NullRawMessage) error {
	if m.err != nil {
		return m.err
	}
	if m.updates == nil {
		m.updates = make(map[uuid.UUID]json.RawMessage)
	}
	m.updates[id1] = inv1.RawMessage
	m.updates[id2] = inv2.RawMessage
	return nil
}

// mockGiveTargetResolver resolves a name/ID to a character.
type mockGiveTargetResolver struct {
	chars map[string]refdata.Character
}

func (m *mockGiveTargetResolver) ResolveTarget(ctx context.Context, campaignID uuid.UUID, nameOrID string) (refdata.Character, error) {
	char, ok := m.chars[nameOrID]
	if !ok {
		return refdata.Character{}, assert.AnError
	}
	return char, nil
}

// mockGiveCombatProvider returns combatant positions for adjacency check.
type mockGiveCombatProvider struct {
	combatants []refdata.Combatant
	inCombat   bool
}

func (m *mockGiveCombatProvider) GetCombatantsForGuild(ctx context.Context, guildID string) ([]refdata.Combatant, bool, error) {
	return m.combatants, m.inCombat, nil
}

func makeGiveInteraction(guildID, userID, itemID, target string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "give",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "item", Type: discordgo.ApplicationCommandOptionString, Value: itemID},
				{Name: "target", Type: discordgo.ApplicationCommandOptionString, Value: target},
			},
		},
	}
}

func TestGiveHandler_Success(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)

	receiverItems := []character.InventoryItem{}
	receiverItemsJSON, _ := json.Marshal(receiverItems)

	store := &mockGiveCharacterStore{}

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         giverID,
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: giverItemsJSON, Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {
				ID:         receiverID,
				CampaignID: campID,
				Name:       "Gorak",
				Inventory:  pqtype.NullRawMessage{RawMessage: receiverItemsJSON, Valid: true},
			},
		}},
		store,
		nil, // no combat
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Aria")
	assert.Contains(t, sess.lastResponse, "Gorak")
	assert.Contains(t, sess.lastResponse, "Healing Potion")

	// Both inventories should be updated
	assert.Len(t, store.updates, 2)
}

func TestGiveHandler_ItemNotFound(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         uuid.New(),
			CampaignID: campID,
			Name:       "Aria",
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {ID: uuid.New(), Name: "Gorak"},
		}},
		&mockGiveCharacterStore{},
		nil,
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "not found")
}

func TestGiveHandler_TargetNotFound(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         uuid.New(),
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: giverItemsJSON, Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{}},
		&mockGiveCharacterStore{},
		nil,
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "NOBODY")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "target")
}

func TestGiveHandler_NoCampaign(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{err: assert.AnError},
		&mockInventoryCharacterLookup{},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{}},
		&mockGiveCharacterStore{},
		nil,
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestGiveHandler_NoCharacter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{err: assert.AnError},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{}},
		&mockGiveCharacterStore{},
		nil,
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestGiveHandler_PersistError(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)

	store := &mockGiveCharacterStore{err: assert.AnError}

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID: giverID, CampaignID: campID, Name: "Aria",
			Inventory: pqtype.NullRawMessage{RawMessage: giverItemsJSON, Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {ID: receiverID, CampaignID: campID, Name: "Gorak"},
		}},
		store,
		nil,
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to save")
}

// mockGiveTurnProvider implements GiveTurnProvider for med-35 tests.
type mockGiveTurnProvider struct {
	turn      refdata.Turn
	inCombat  bool
	lookupErr error
	updates   []refdata.UpdateTurnActionsParams
}

func (m *mockGiveTurnProvider) GetActiveTurnForCharacter(_ context.Context, _ string, _ uuid.UUID) (refdata.Turn, bool, error) {
	if m.lookupErr != nil {
		return refdata.Turn{}, false, m.lookupErr
	}
	return m.turn, m.inCombat, nil
}

func (m *mockGiveTurnProvider) UpdateTurnActions(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	m.updates = append(m.updates, arg)
	return refdata.Turn{ID: arg.ID, FreeInteractUsed: arg.FreeInteractUsed}, nil
}

func TestGiveHandler_InCombat_DeductsFreeInteraction(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()
	turnID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)
	receiverItemsJSON, _ := json.Marshal([]character.InventoryItem{})

	store := &mockGiveCharacterStore{}
	turnProv := &mockGiveTurnProvider{turn: refdata.Turn{ID: turnID, FreeInteractUsed: false}, inCombat: true}

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         giverID,
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: giverItemsJSON, Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {
				ID:         receiverID,
				CampaignID: campID,
				Name:       "Gorak",
				Inventory:  pqtype.NullRawMessage{RawMessage: receiverItemsJSON, Valid: true},
			},
		}},
		store,
		nil,
	)
	handler.SetTurnProvider(turnProv)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Healing Potion")
	if assert.Len(t, turnProv.updates, 1, "expected one turn update") {
		assert.True(t, turnProv.updates[0].FreeInteractUsed, "/give in combat should consume the free interaction")
	}
	assert.Len(t, store.updates, 2, "both inventories should still update")
}

func TestGiveHandler_InCombat_FreeInteractionAlreadySpent_Rejected(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()
	turnID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)

	store := &mockGiveCharacterStore{}
	turnProv := &mockGiveTurnProvider{turn: refdata.Turn{ID: turnID, FreeInteractUsed: true}, inCombat: true}

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         giverID,
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: giverItemsJSON, Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {ID: receiverID, CampaignID: campID, Name: "Gorak"},
		}},
		store,
		nil,
	)
	handler.SetTurnProvider(turnProv)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Cannot give item")
	assert.Empty(t, store.updates, "rejection must not transfer items")
	assert.Empty(t, turnProv.updates, "rejection must not persist a turn update")
}

// --- T25: adjacency/range check, public #the-story post, receiver DM ---

// recordingGiveSession captures every public channel send and every DM-channel
// creation so the T25 tests can assert the story post and receiver DM fired.
// GuildChannels returns a #the-story channel so resolveStoryChannel succeeds.
type recordingGiveSession struct {
	mockInventorySession
	storyChannelID  string
	dmChannelID     string
	channelSends    []channelSend
	dmCreatedFor    []string
	failChannelSend bool
	failDMCreate    bool
}

type channelSend struct {
	channelID string
	content   string
}

func (m *recordingGiveSession) GuildChannels(string) ([]*discordgo.Channel, error) {
	if m.storyChannelID == "" {
		return nil, nil
	}
	return []*discordgo.Channel{
		{ID: m.storyChannelID, Name: theStoryChannelName, Type: discordgo.ChannelTypeGuildText},
	}, nil
}

func (m *recordingGiveSession) UserChannelCreate(recipientID string) (*discordgo.Channel, error) {
	m.dmCreatedFor = append(m.dmCreatedFor, recipientID)
	if m.failDMCreate {
		return nil, assert.AnError
	}
	return &discordgo.Channel{ID: m.dmChannelID}, nil
}

func (m *recordingGiveSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	if m.failChannelSend {
		return nil, assert.AnError
	}
	m.channelSends = append(m.channelSends, channelSend{channelID: channelID, content: content})
	return &discordgo.Message{}, nil
}

// mockGivePlayerLookup maps a character to its player-character row (for the
// receiver's Discord user ID used by the DM).
type mockGivePlayerLookup struct {
	pcByChar map[uuid.UUID]refdata.PlayerCharacter
	err      error
}

func (m *mockGivePlayerLookup) GetPlayerCharacterByCharacter(_ context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error) {
	if m.err != nil {
		return refdata.PlayerCharacter{}, m.err
	}
	pc, ok := m.pcByChar[arg.CharacterID]
	if !ok {
		return refdata.PlayerCharacter{}, assert.AnError
	}
	return pc, nil
}

// giveItemsJSON marshals a single healing-potion stack for the giver.
func giveItemsJSON() []byte {
	b, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	})
	return b
}

func emptyItemsJSON() []byte {
	b, _ := json.Marshal([]character.InventoryItem{})
	return b
}

// newT25GiveHandler builds a GiveHandler wired with the combat provider so the
// range check is exercised, plus the recording session and player lookup.
func newT25GiveHandler(sess Session, campID, giverID, receiverID uuid.UUID, combatProv GiveCombatProvider) *GiveHandler {
	return NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         giverID,
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: giveItemsJSON(), Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {
				ID:         receiverID,
				CampaignID: campID,
				Name:       "Gorak",
				Inventory:  pqtype.NullRawMessage{RawMessage: emptyItemsJSON(), Valid: true},
			},
		}},
		&mockGiveCharacterStore{},
		combatProv,
	)
}

func TestGiveHandler_RangeTooFar_RejectsAndDoesNotTransfer(t *testing.T) {
	sess := &recordingGiveSession{storyChannelID: "story1"}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	// Giver at A1, receiver at A4: 3 tiles = 15ft > 5ft.
	combatProv := &mockGiveCombatProvider{
		inCombat: true,
		combatants: []refdata.Combatant{
			{CharacterID: uuid.NullUUID{UUID: giverID, Valid: true}, PositionCol: "A", PositionRow: 1},
			{CharacterID: uuid.NullUUID{UUID: receiverID, Valid: true}, PositionCol: "A", PositionRow: 4},
		},
	}

	handler := newT25GiveHandler(sess, campID, giverID, receiverID, combatProv)
	store := handler.store.(*mockGiveCharacterStore)

	handler.Handle(makeGiveInteraction("guild1", "user1", "healing-potion", "GK"))

	assert.Contains(t, sess.lastResponse, "too far away")
	assert.Contains(t, sess.lastResponse, "15ft")
	assert.Empty(t, store.updates, "out-of-range give must not transfer items")
	assert.Empty(t, sess.channelSends, "rejected give must not post to #the-story")
}

func TestGiveHandler_Adjacent_Allows(t *testing.T) {
	sess := &recordingGiveSession{storyChannelID: "story1"}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	// Giver at A1, receiver at B2: Chebyshev 1 tile = 5ft (adjacent).
	combatProv := &mockGiveCombatProvider{
		inCombat: true,
		combatants: []refdata.Combatant{
			{CharacterID: uuid.NullUUID{UUID: giverID, Valid: true}, PositionCol: "A", PositionRow: 1},
			{CharacterID: uuid.NullUUID{UUID: receiverID, Valid: true}, PositionCol: "B", PositionRow: 2},
		},
	}

	handler := newT25GiveHandler(sess, campID, giverID, receiverID, combatProv)
	store := handler.store.(*mockGiveCharacterStore)

	handler.Handle(makeGiveInteraction("guild1", "user1", "healing-potion", "GK"))

	assert.Contains(t, sess.lastResponse, "Healing Potion")
	assert.Len(t, store.updates, 2, "adjacent give must transfer items")
}

func TestGiveHandler_OutOfCombatNoPositions_SkipsRangeCheck(t *testing.T) {
	sess := &recordingGiveSession{storyChannelID: "story1"}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	// No combatants for the guild: pure out-of-combat trade. Range cannot be
	// enforced, so the give must succeed.
	combatProv := &mockGiveCombatProvider{inCombat: false}

	handler := newT25GiveHandler(sess, campID, giverID, receiverID, combatProv)
	store := handler.store.(*mockGiveCharacterStore)

	handler.Handle(makeGiveInteraction("guild1", "user1", "healing-potion", "GK"))

	assert.Contains(t, sess.lastResponse, "Healing Potion")
	assert.Len(t, store.updates, 2, "out-of-combat give must transfer items")
}

// errGiveCombatProvider returns an error from GetCombatantsForGuild so the
// range check's defensive error path is exercised.
type errGiveCombatProvider struct{}

func (errGiveCombatProvider) GetCombatantsForGuild(context.Context, string) ([]refdata.Combatant, bool, error) {
	return nil, false, assert.AnError
}

func TestGiveHandler_CombatProviderError_SkipsRangeCheck(t *testing.T) {
	sess := &recordingGiveSession{storyChannelID: "story1"}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	handler := newT25GiveHandler(sess, campID, giverID, receiverID, errGiveCombatProvider{})
	store := handler.store.(*mockGiveCharacterStore)

	handler.Handle(makeGiveInteraction("guild1", "user1", "healing-potion", "GK"))

	assert.Contains(t, sess.lastResponse, "Healing Potion")
	assert.Len(t, store.updates, 2, "combat lookup error must not block the give")
}

func TestGiveHandler_OneCombatantUnpositioned_SkipsRangeCheck(t *testing.T) {
	sess := &recordingGiveSession{storyChannelID: "story1"}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	// Giver positioned, receiver in the encounter but without a position.
	combatProv := &mockGiveCombatProvider{
		inCombat: true,
		combatants: []refdata.Combatant{
			{CharacterID: uuid.NullUUID{UUID: giverID, Valid: true}, PositionCol: "A", PositionRow: 1},
			{CharacterID: uuid.NullUUID{UUID: receiverID, Valid: true}, PositionCol: "", PositionRow: 0},
		},
	}

	handler := newT25GiveHandler(sess, campID, giverID, receiverID, combatProv)
	store := handler.store.(*mockGiveCharacterStore)

	handler.Handle(makeGiveInteraction("guild1", "user1", "healing-potion", "GK"))

	assert.Contains(t, sess.lastResponse, "Healing Potion")
	assert.Len(t, store.updates, 2, "unpositioned party must not be range-blocked")
}

func TestGiveHandler_PostsToStoryChannel(t *testing.T) {
	sess := &recordingGiveSession{storyChannelID: "story1"}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	handler := newT25GiveHandler(sess, campID, giverID, receiverID, &mockGiveCombatProvider{})
	handler.Handle(makeGiveInteraction("guild1", "user1", "healing-potion", "GK"))

	if assert.Len(t, sess.channelSends, 1, "give must post once to #the-story") {
		got := sess.channelSends[0]
		assert.Equal(t, "story1", got.channelID)
		assert.Contains(t, got.content, "Aria")
		assert.Contains(t, got.content, "Gorak")
		assert.Contains(t, got.content, "Healing Potion")
	}
}

func TestGiveHandler_DMsReceiver(t *testing.T) {
	sess := &recordingGiveSession{storyChannelID: "story1", dmChannelID: "dm1"}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	handler := newT25GiveHandler(sess, campID, giverID, receiverID, &mockGiveCombatProvider{})
	handler.SetPlayerLookup(&mockGivePlayerLookup{pcByChar: map[uuid.UUID]refdata.PlayerCharacter{
		receiverID: {CharacterID: receiverID, DiscordUserID: "receiver-discord"},
	}})

	handler.Handle(makeGiveInteraction("guild1", "user1", "healing-potion", "GK"))

	assert.Equal(t, []string{"receiver-discord"}, sess.dmCreatedFor, "receiver must be DMed")
	// One send to #the-story (story1) plus one DM (dm1).
	var dmSent bool
	for _, s := range sess.channelSends {
		if s.channelID != "dm1" {
			continue
		}
		dmSent = true
		assert.Contains(t, s.content, "Healing Potion")
		assert.Contains(t, s.content, "Aria")
	}
	assert.True(t, dmSent, "receiver DM body must be sent")
}

func TestGiveHandler_PostAndDMFailures_AreSwallowed(t *testing.T) {
	sess := &recordingGiveSession{storyChannelID: "story1", dmChannelID: "dm1", failChannelSend: true, failDMCreate: true}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	handler := newT25GiveHandler(sess, campID, giverID, receiverID, &mockGiveCombatProvider{})
	handler.SetPlayerLookup(&mockGivePlayerLookup{pcByChar: map[uuid.UUID]refdata.PlayerCharacter{
		receiverID: {CharacterID: receiverID, DiscordUserID: "receiver-discord"},
	}})
	store := handler.store.(*mockGiveCharacterStore)

	handler.Handle(makeGiveInteraction("guild1", "user1", "healing-potion", "GK"))

	// Give still succeeds despite story/DM send failures.
	assert.Contains(t, sess.lastResponse, "Healing Potion")
	assert.Len(t, store.updates, 2, "give must succeed even when notifications fail")
}

func TestGiveHandler_OutOfCombat_NoCost(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)
	receiverItemsJSON, _ := json.Marshal([]character.InventoryItem{})

	store := &mockGiveCharacterStore{}
	turnProv := &mockGiveTurnProvider{inCombat: false}

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         giverID,
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: giverItemsJSON, Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {
				ID:         receiverID,
				CampaignID: campID,
				Name:       "Gorak",
				Inventory:  pqtype.NullRawMessage{RawMessage: receiverItemsJSON, Valid: true},
			},
		}},
		store,
		nil,
	)
	handler.SetTurnProvider(turnProv)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Healing Potion")
	assert.Empty(t, turnProv.updates, "out-of-combat /give must not deduct any resource")
	assert.Len(t, store.updates, 2)
}
