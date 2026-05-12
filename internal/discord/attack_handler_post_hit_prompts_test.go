package discord

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// mockClassFeatureService records the StunningStrike / DivineSmite /
// UseBardicInspiration commands the handler dispatches when a prompt's
// "Use" button is clicked.
type mockClassFeatureService struct {
	mu          sync.Mutex
	stunCmds    []combat.StunningStrikeCommand
	smiteCmds   []combat.DivineSmiteCommand
	bardicCmds  []combat.UseBardicInspirationCommand
	stunResult  combat.StunningStrikeResult
	smiteResult combat.DivineSmiteResult
	bardicResult combat.UseBardicInspirationResult
}

func (m *mockClassFeatureService) StunningStrike(_ context.Context, cmd combat.StunningStrikeCommand, _ *dice.Roller) (combat.StunningStrikeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stunCmds = append(m.stunCmds, cmd)
	return m.stunResult, nil
}

func (m *mockClassFeatureService) DivineSmite(_ context.Context, cmd combat.DivineSmiteCommand, _ *dice.Roller) (combat.DivineSmiteResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.smiteCmds = append(m.smiteCmds, cmd)
	return m.smiteResult, nil
}

func (m *mockClassFeatureService) UseBardicInspiration(_ context.Context, cmd combat.UseBardicInspirationCommand, _ *dice.Roller) (combat.UseBardicInspirationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bardicCmds = append(m.bardicCmds, cmd)
	return m.bardicResult, nil
}

// makeAttackInteractionWithChannel mirrors makeAttackInteraction but adds a
// ChannelID so resolvePromptChannel has a fallback when no campaign-settings
// provider is wired.
func makeAttackInteractionWithChannel(channelID string, opts map[string]any) *discordgo.Interaction {
	i := makeAttackInteraction(opts)
	i.ChannelID = channelID
	return i
}

// setupAttackHandlerWithPrompts wires the AttackHandler with a real
// ReactionPromptStore (no TTL forfeit during test), a ClassFeaturePromptPoster,
// and a recording class-feature service. Returns the handler, the capture-
// enabled session, the combat service mock, and the class-feature service.
func setupAttackHandlerWithPrompts(t *testing.T) (
	*AttackHandler,
	*MockSession,
	*[]*discordgo.MessageSend,
	*mockAttackCombatService,
	*mockClassFeatureService,
	uuid.UUID,
) {
	t.Helper()
	sess, sent := captureSentComplex()

	encID := uuid.New()
	turnID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()

	provider := &mockAttackProvider{
		encID: encID,
		enc: refdata.Encounter{
			ID:            encID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			RoundNumber:   3,
		},
		turn: refdata.Turn{
			ID:               turnID,
			CombatantID:      attackerID,
			AttacksRemaining: 1,
		},
		attacker: refdata.Combatant{
			ID: attackerID, ShortID: "AR", DisplayName: "Aria",
			PositionCol: "A", PositionRow: 1,
		},
		target: refdata.Combatant{
			ID: targetID, ShortID: "OS", DisplayName: "Orc",
			PositionCol: "B", PositionRow: 1,
		},
	}
	combatSvc := &mockAttackCombatService{
		attackResult: combat.AttackResult{
			AttackerName: "Aria",
			TargetName:   "Orc",
			WeaponName:   "shortsword",
			Hit:          true,
			IsMelee:      true,
			DistanceFt:   5,
		},
	}
	h := NewAttackHandler(sess, combatSvc, provider, dice.NewRoller(func(_ int) int { return 10 }))
	store := NewReactionPromptStoreWithTTL(sess, time.Hour)
	h.SetClassFeaturePromptPoster(NewClassFeaturePromptPoster(store))
	cfs := &mockClassFeatureService{}
	h.SetClassFeatureService(cfs)
	return h, sess, sent, combatSvc, cfs, encID
}

func TestAttackHandler_MonkHit_PostsStunningStrikePrompt(t *testing.T) {
	h, _, sent, combatSvc, _, _ := setupAttackHandlerWithPrompts(t)
	combatSvc.attackResult.PromptStunningStrikeEligible = true
	combatSvc.attackResult.PromptStunningStrikeKiAvailable = 3

	h.Handle(makeAttackInteractionWithChannel("ch-1", map[string]any{"target": "OS"}))

	require.NotEmpty(t, *sent, "expected ClassFeaturePromptPoster to post the Stunning Strike prompt")
	assert.True(t, sendsContain(*sent, "Stunning Strike"),
		"expected a Stunning Strike prompt in the captured sends")
}

func TestAttackHandler_NotEligibleForStunningStrike_NoPrompt(t *testing.T) {
	h, _, sent, _, _, _ := setupAttackHandlerWithPrompts(t)
	// attackResult.PromptStunningStrikeEligible stays false by default.

	h.Handle(makeAttackInteractionWithChannel("ch-1", map[string]any{"target": "OS"}))

	assert.False(t, sendsContain(*sent, "Stunning Strike"),
		"did not expect a Stunning Strike prompt when not eligible")
}

func TestAttackHandler_PaladinHit_PostsDivineSmitePrompt(t *testing.T) {
	h, _, sent, combatSvc, _, _ := setupAttackHandlerWithPrompts(t)
	combatSvc.attackResult.PromptDivineSmiteEligible = true
	combatSvc.attackResult.PromptDivineSmiteSlots = []int{1, 2}

	h.Handle(makeAttackInteractionWithChannel("ch-1", map[string]any{"target": "OS"}))

	assert.True(t, sendsContain(*sent, "Divine Smite"),
		"expected a Divine Smite prompt in the captured sends")
}

func TestAttackHandler_BardicInspirationHolder_PostsBardicPrompt(t *testing.T) {
	h, _, sent, combatSvc, _, _ := setupAttackHandlerWithPrompts(t)
	combatSvc.attackResult.PromptBardicInspirationEligible = true
	combatSvc.attackResult.PromptBardicInspirationDie = "d8"

	h.Handle(makeAttackInteractionWithChannel("ch-1", map[string]any{"target": "OS"}))

	assert.True(t, sendsContain(*sent, "Bardic Inspiration"),
		"expected a Bardic Inspiration prompt in the captured sends")
}

// sendsContain reports whether any captured ChannelMessageSendComplex
// payload's content contains substr. Used by the post-hit prompt assertions
// so each "we posted a prompt" test reads as a single line.
func sendsContain(sent []*discordgo.MessageSend, substr string) bool {
	for _, msg := range sent {
		if msg != nil && strings.Contains(msg.Content, substr) {
			return true
		}
	}
	return false
}

func TestAttackHandler_StunningStrike_UseKi_InvokesService(t *testing.T) {
	h, _, sent, combatSvc, cfs, _ := setupAttackHandlerWithPrompts(t)
	combatSvc.attackResult.PromptStunningStrikeEligible = true
	combatSvc.attackResult.PromptStunningStrikeKiAvailable = 2

	// Recreate the prompt store via the same internal field so we can drive
	// HandleComponent. The store is the one owned by the poster; we route
	// the click through the same in-memory store that holds the callback.
	h.Handle(makeAttackInteractionWithChannel("ch-1", map[string]any{"target": "OS"}))

	require.NotEmpty(t, *sent, "expected the Stunning Strike prompt to be posted")
	var promptMsg *discordgo.MessageSend
	for _, m := range *sent {
		if m != nil && strings.Contains(m.Content, "Stunning Strike") {
			promptMsg = m
			break
		}
	}
	require.NotNil(t, promptMsg, "Stunning Strike prompt was not captured")
	useBtn := promptMsg.Components[0].(discordgo.ActionsRow).Components[0].(discordgo.Button)

	// Route the click through the prompt store the poster wraps so the
	// registered OnChoice callback fires StunningStrike on the mock service.
	require.NotNil(t, h.classFeaturePrompts)
	h.classFeaturePrompts.prompts.HandleComponent(&discordgo.Interaction{
		Type:   discordgo.InteractionMessageComponent,
		Data:   discordgo.MessageComponentInteractionData{CustomID: useBtn.CustomID},
		Member: &discordgo.Member{User: &discordgo.User{ID: "u1"}},
	})

	require.True(t, waitForStunCmd(cfs, time.Second),
		"expected StunningStrike service to be invoked on Use Ki click")
	cfs.mu.Lock()
	cmd := cfs.stunCmds[0]
	cfs.mu.Unlock()
	assert.Equal(t, "Aria", cmd.Attacker.DisplayName)
	assert.Equal(t, "Orc", cmd.Target.DisplayName)
	assert.Equal(t, 3, cmd.CurrentRound)
}

// waitForStunCmd polls the mock service until at least one StunningStrike
// command is recorded or the deadline elapses. Returns true on success.
func waitForStunCmd(cfs *mockClassFeatureService, within time.Duration) bool {
	delivered := atomic.Bool{}
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		cfs.mu.Lock()
		n := len(cfs.stunCmds)
		cfs.mu.Unlock()
		if n > 0 {
			delivered.Store(true)
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return delivered.Load()
}

