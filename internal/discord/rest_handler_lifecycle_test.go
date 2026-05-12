package discord

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// fakeRestPublisher records the encounter IDs the rest publisher fan-out
// receives. Concurrent safe so the rest handler's mid-flow snapshots
// don't race the assertion.
type fakeRestPublisher struct {
	mu    sync.Mutex
	calls []uuid.UUID
}

func (f *fakeRestPublisher) PublishEncounterSnapshot(_ context.Context, encounterID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, encounterID)
	return nil
}

func (f *fakeRestPublisher) all() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]uuid.UUID, len(f.calls))
	copy(out, f.calls)
	return out
}

type fakeRestEncounterLookup struct{ encID uuid.UUID }

func (f *fakeRestEncounterLookup) ActiveEncounterIDForCharacter(_ context.Context, _ uuid.UUID) (uuid.UUID, bool, error) {
	if f.encID == uuid.Nil {
		return uuid.Nil, false, nil
	}
	return f.encID, true, nil
}

func buildPaladinForLongRest(campaignID uuid.UUID) refdata.Character {
	scores, _ := json.Marshal(character.AbilityScores{STR: 14, DEX: 12, CON: 14, INT: 8, WIS: 10, CHA: 16})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "paladin", Level: 5}})
	hitDice, _ := json.Marshal(map[string]int{"d10": 3})
	featureUses, _ := json.Marshal(map[string]character.FeatureUse{})
	spellSlots, _ := json.Marshal(map[string]character.SlotInfo{
		"1": {Current: 1, Max: 4},
		"2": {Current: 0, Max: 2},
	})
	return refdata.Character{
		ID:               uuid.New(),
		CampaignID:       campaignID,
		Name:             "Gorden",
		Level:            5,
		HpMax:            44,
		HpCurrent:        20,
		AbilityScores:    scores,
		Classes:          classes,
		HitDiceRemaining: hitDice,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUses, Valid: true},
		SpellSlots:       pqtype.NullRawMessage{RawMessage: spellSlots, Valid: true},
	}
}

func TestRestHandler_LongRest_PostsPrepareReminderForPaladin(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	campaignID := uuid.New()
	char := buildPaladinForLongRest(campaignID)

	roller := dice.NewRoller(func(max int) int { return 6 })
	h := NewRestHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errNoEncounter
		}},
		&mockRestCharacterUpdater{
			updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		&mockCheckRollLogger{},
		nil,
	)
	h.Handle(makeRestInteraction("long"))

	require.Contains(t, responded, "/prepare", "paladin must receive the prepare reminder after a long rest (E-65)")
}

func TestRestHandler_LongRest_PublishesSnapshotForActiveCombatant(t *testing.T) {
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
		return nil
	}

	campaignID := uuid.New()
	char := buildPaladinForLongRest(campaignID)

	roller := dice.NewRoller(func(max int) int { return 6 })
	h := NewRestHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			// /rest itself is gated by ActiveEncounterForUser — for the
			// sibling-encounter scenario, /rest is allowed (current user
			// isn't in combat) but a DIFFERENT character could still be
			// referenced. For simplicity we assert that publisher fires
			// when the rest.Service-side encounter lookup returns a hit.
			return uuid.Nil, errNoEncounter
		}},
		&mockRestCharacterUpdater{
			updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		&mockCheckRollLogger{},
		nil,
	)

	encID := uuid.New()
	pub := &fakeRestPublisher{}
	h.SetPublisher(pub, &fakeRestEncounterLookup{encID: encID})

	h.Handle(makeRestInteraction("long"))

	assert.Equal(t, []uuid.UUID{encID}, pub.all(), "long-rest must fan out one publish to the active encounter (H-104b)")
}

func TestRestHandler_LongRest_NoPublisher_NoCrash(t *testing.T) {
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
		return nil
	}

	campaignID := uuid.New()
	char := buildPaladinForLongRest(campaignID)

	roller := dice.NewRoller(func(max int) int { return 6 })
	h := NewRestHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errNoEncounter
		}},
		&mockRestCharacterUpdater{
			updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		&mockCheckRollLogger{},
		nil,
	)

	// Intentionally do NOT SetPublisher.
	h.Handle(makeRestInteraction("long"))

	// Reaching this line without panic is the test.
	_ = strings.TrimSpace("ok")
}
