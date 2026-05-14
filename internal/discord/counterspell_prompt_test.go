package discord

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
)

type mockCounterspellService struct {
	mu                 sync.Mutex
	triggerPrompt      combat.CounterspellPrompt
	triggerErr         error
	triggerCalledWith  *struct{ DeclarationID uuid.UUID; EnemySpellName string; EnemyCastLevel int; IsSubtle bool }
	resolveResult      combat.CounterspellResult
	resolveErr         error
	resolveCalledLvl   int
	passResult         combat.CounterspellResult
	passErr            error
	passCalls          int32
	forfeitResult      combat.CounterspellResult
	forfeitErr         error
	forfeitCalls       int32
}

func (m *mockCounterspellService) TriggerCounterspell(ctx context.Context, declID uuid.UUID, name string, lvl int, subtle bool, enemyCasterID uuid.UUID) (combat.CounterspellPrompt, error) {
	m.mu.Lock()
	m.triggerCalledWith = &struct{ DeclarationID uuid.UUID; EnemySpellName string; EnemyCastLevel int; IsSubtle bool }{declID, name, lvl, subtle}
	m.mu.Unlock()
	return m.triggerPrompt, m.triggerErr
}
func (m *mockCounterspellService) ResolveCounterspell(ctx context.Context, declID uuid.UUID, lvl int) (combat.CounterspellResult, error) {
	m.mu.Lock()
	m.resolveCalledLvl = lvl
	m.mu.Unlock()
	return m.resolveResult, m.resolveErr
}
func (m *mockCounterspellService) PassCounterspell(ctx context.Context, declID uuid.UUID) (combat.CounterspellResult, error) {
	atomic.AddInt32(&m.passCalls, 1)
	return m.passResult, m.passErr
}
func (m *mockCounterspellService) ForfeitCounterspell(ctx context.Context, declID uuid.UUID) (combat.CounterspellResult, error) {
	atomic.AddInt32(&m.forfeitCalls, 1)
	return m.forfeitResult, m.forfeitErr
}

func TestCounterspellPromptPoster_Trigger_BuildsSlotButtonsPlusPass(t *testing.T) {
	mock, sent := captureSentComplex()
	svc := &mockCounterspellService{
		triggerPrompt: combat.CounterspellPrompt{
			DeclarationID:  uuid.New(),
			CasterName:     "Hex",
			EnemySpellName: "Fireball",
			AvailableSlots: []int{3, 4, 5},
		},
	}
	prompts := NewReactionPromptStoreWithTTL(mock, time.Hour)
	poster := NewCounterspellPromptPoster(svc, prompts, mock)

	if err := poster.Trigger(context.Background(), CounterspellPromptArgs{
		DeclarationID:  uuid.New(),
		EnemySpellName: "Fireball",
		EnemyCastLevel: 3,
		ChannelID:      "ch-1",
	}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if len(*sent) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(*sent))
	}
	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	if len(row.Components) != 4 {
		t.Fatalf("expected 4 buttons (3 slots + pass), got %d", len(row.Components))
	}
	for i, want := range []string{":3", ":4", ":5", ":pass"} {
		btn := row.Components[i].(discordgo.Button)
		if !strings.HasSuffix(btn.CustomID, want) {
			t.Errorf("button %d CustomID = %q, want suffix %q", i, btn.CustomID, want)
		}
	}
}

func TestCounterspellPromptPoster_Trigger_SubtleBypass(t *testing.T) {
	mock, _ := captureSentComplex()
	var msgs []string
	var mu sync.Mutex
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		mu.Lock()
		msgs = append(msgs, content)
		mu.Unlock()
		return &discordgo.Message{}, nil
	}
	svc := &mockCounterspellService{triggerErr: combat.ErrSubtleSpellNotCounterspellable}
	poster := NewCounterspellPromptPoster(svc, NewReactionPromptStore(mock), mock)

	err := poster.Trigger(context.Background(), CounterspellPromptArgs{
		DeclarationID:  uuid.New(),
		EnemySpellName: "Fireball",
		IsSubtle:       true,
		ChannelID:      "ch-1",
	})
	if err != nil {
		t.Errorf("Trigger with subtle should not error, got %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 info message, got %d", len(msgs))
	}
	if !strings.Contains(strings.ToLower(msgs[0]), "subtl") {
		t.Errorf("expected subtle-bypass message, got %q", msgs[0])
	}
}

func TestCounterspellPromptPoster_Trigger_BubblesOtherError(t *testing.T) {
	mock, _ := captureSentComplex()
	svc := &mockCounterspellService{triggerErr: errors.New("kaboom")}
	poster := NewCounterspellPromptPoster(svc, NewReactionPromptStore(mock), mock)
	if err := poster.Trigger(context.Background(), CounterspellPromptArgs{ChannelID: "ch-1"}); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestCounterspellPromptPoster_OnChoice_RoutesSlotAndPass(t *testing.T) {
	mock, sent := captureSentComplex()
	declID := uuid.New()
	svc := &mockCounterspellService{
		triggerPrompt: combat.CounterspellPrompt{
			DeclarationID:  declID,
			CasterName:     "Hex",
			EnemySpellName: "Fireball",
			AvailableSlots: []int{3, 5},
		},
		resolveResult: combat.CounterspellResult{
			Outcome:        combat.CounterspellCountered,
			CasterName:     "Hex",
			EnemySpellName: "Fireball",
			SlotUsed:       5,
		},
		passResult: combat.CounterspellResult{
			Outcome:        combat.CounterspellPassed,
			CasterName:     "Hex",
			EnemySpellName: "Fireball",
		},
	}
	prompts := NewReactionPromptStoreWithTTL(mock, time.Hour)
	poster := NewCounterspellPromptPoster(svc, prompts, mock)
	if err := poster.Trigger(context.Background(), CounterspellPromptArgs{ChannelID: "ch-1"}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}

	// Click slot=5
	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	btn := row.Components[1].(discordgo.Button) // 0=3, 1=5
	prompts.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: btn.CustomID},
	})
	if svc.resolveCalledLvl != 5 {
		t.Errorf("ResolveCounterspell called with %d, want 5", svc.resolveCalledLvl)
	}

	// New trigger, click pass
	*sent = nil
	if err := poster.Trigger(context.Background(), CounterspellPromptArgs{ChannelID: "ch-1"}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	row = (*sent)[0].Components[0].(discordgo.ActionsRow)
	passBtn := row.Components[len(row.Components)-1].(discordgo.Button)
	prompts.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: passBtn.CustomID},
	})
	if atomic.LoadInt32(&svc.passCalls) != 1 {
		t.Errorf("PassCounterspell calls = %d, want 1", atomic.LoadInt32(&svc.passCalls))
	}
}

func TestCounterspellPromptPoster_Forfeit_CallsForfeitOnTimeout(t *testing.T) {
	mock, _ := captureSentComplex()
	svc := &mockCounterspellService{
		triggerPrompt: combat.CounterspellPrompt{
			DeclarationID:  uuid.New(),
			CasterName:     "Hex",
			EnemySpellName: "Fireball",
			AvailableSlots: []int{3},
		},
		forfeitResult: combat.CounterspellResult{
			Outcome:        combat.CounterspellForfeited,
			CasterName:     "Hex",
			EnemySpellName: "Fireball",
		},
	}
	prompts := NewReactionPromptStoreWithTTL(mock, 10*time.Millisecond)
	poster := NewCounterspellPromptPoster(svc, prompts, mock)
	if err := poster.Trigger(context.Background(), CounterspellPromptArgs{ChannelID: "ch-1"}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&svc.forfeitCalls) == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("ForfeitCounterspell was not called within timeout")
}
