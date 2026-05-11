package discord

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestMetamagicPromptPoster_PromptEmpowered_OneButtonPerDie(t *testing.T) {
	mock, sent := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, time.Hour)
	poster := NewMetamagicPromptPoster(store)

	gotIdx := atomic.Int32{}
	gotIdx.Store(-1)
	gotForfeit := atomic.Bool{}
	err := poster.PromptEmpowered(EmpoweredPromptArgs{
		ChannelID:  "ch-1",
		SpellName:  "Fireball",
		DiceRolls:  []int{2, 6, 1, 4},
		MaxRerolls: 2,
	}, func(res EmpoweredPromptResult) {
		if res.Forfeited {
			gotForfeit.Store(true)
			return
		}
		gotIdx.Store(int32(res.SelectedIndex))
	})
	if err != nil {
		t.Fatalf("PromptEmpowered: %v", err)
	}
	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	if len(row.Components) != 4 {
		t.Fatalf("expected 4 die buttons, got %d", len(row.Components))
	}
	// Click die at index 2 (value=1, prime target for reroll)
	btn := row.Components[2].(discordgo.Button)
	store.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: btn.CustomID},
	})
	if got := gotIdx.Load(); got != 2 {
		t.Errorf("SelectedIndex = %d, want 2", got)
	}
	if gotForfeit.Load() {
		t.Errorf("forfeited unexpectedly")
	}
}

func TestMetamagicPromptPoster_PromptEmpowered_ForfeitOnTimeout(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, 10*time.Millisecond)
	poster := NewMetamagicPromptPoster(store)

	gotForfeit := atomic.Bool{}
	if err := poster.PromptEmpowered(EmpoweredPromptArgs{
		ChannelID:  "ch-1",
		SpellName:  "Magic Missile",
		DiceRolls:  []int{1, 2, 3},
		MaxRerolls: 1,
	}, func(res EmpoweredPromptResult) {
		if res.Forfeited {
			gotForfeit.Store(true)
		}
	}); err != nil {
		t.Fatalf("PromptEmpowered: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if gotForfeit.Load() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("forfeit did not fire")
}

func TestMetamagicPromptPoster_PromptCareful_OneButtonPerTarget(t *testing.T) {
	mock, sent := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, time.Hour)
	poster := NewMetamagicPromptPoster(store)

	got := atomic.Int32{}
	got.Store(-1)
	if err := poster.PromptCareful(CarefulPromptArgs{
		ChannelID:    "ch-1",
		SpellName:    "Fireball",
		TargetNames:  []string{"Orc A", "Orc B", "Goblin"},
		MaxProtected: 2,
	}, func(res CarefulPromptResult) {
		got.Store(int32(res.SelectedIndex))
	}); err != nil {
		t.Fatalf("PromptCareful: %v", err)
	}
	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	if len(row.Components) != 3 {
		t.Fatalf("expected 3 target buttons, got %d", len(row.Components))
	}
	if !strings.Contains((*sent)[0].Content, "2") {
		t.Errorf("content should mention max-protected count, got %q", (*sent)[0].Content)
	}
	btn := row.Components[1].(discordgo.Button)
	store.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: btn.CustomID},
	})
	if g := got.Load(); g != 1 {
		t.Errorf("SelectedIndex = %d, want 1", g)
	}
}

func TestMetamagicPromptPoster_PromptHeightened_OneButtonPerTarget(t *testing.T) {
	mock, sent := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, time.Hour)
	poster := NewMetamagicPromptPoster(store)

	got := atomic.Int32{}
	got.Store(-1)
	if err := poster.PromptHeightened(HeightenedPromptArgs{
		ChannelID:   "ch-1",
		SpellName:   "Hold Person",
		TargetNames: []string{"Bandit Captain", "Bandit"},
	}, func(res HeightenedPromptResult) {
		got.Store(int32(res.SelectedIndex))
	}); err != nil {
		t.Fatalf("PromptHeightened: %v", err)
	}
	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	btn := row.Components[0].(discordgo.Button)
	store.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: btn.CustomID},
	})
	if g := got.Load(); g != 0 {
		t.Errorf("SelectedIndex = %d, want 0", g)
	}
}

func TestMetamagicPromptPoster_RejectsEmptyInputs(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStore(mock)
	poster := NewMetamagicPromptPoster(store)

	if err := poster.PromptEmpowered(EmpoweredPromptArgs{ChannelID: "ch", SpellName: "x"},
		func(EmpoweredPromptResult) {}); err == nil {
		t.Errorf("expected error for empty dice rolls")
	}
	if err := poster.PromptCareful(CarefulPromptArgs{ChannelID: "ch", SpellName: "x"},
		func(CarefulPromptResult) {}); err == nil {
		t.Errorf("expected error for empty target names")
	}
	if err := poster.PromptHeightened(HeightenedPromptArgs{ChannelID: "ch", SpellName: "x"},
		func(HeightenedPromptResult) {}); err == nil {
		t.Errorf("expected error for empty target names")
	}
}

// suppress unused-import warning when context not referenced anywhere
var _ = context.Background
