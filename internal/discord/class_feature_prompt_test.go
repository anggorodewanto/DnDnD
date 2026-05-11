package discord

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestClassFeaturePromptPoster_StunningStrike_UseKi(t *testing.T) {
	mock, sent := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, time.Hour)
	poster := NewClassFeaturePromptPoster(store)

	useKi := atomic.Bool{}
	forfeited := atomic.Bool{}
	if err := poster.PromptStunningStrike(StunningStrikePromptArgs{
		ChannelID:    "ch-1",
		AttackerName: "Monk",
		TargetName:   "Goblin",
		KiAvailable:  3,
	}, func(res StunningStrikePromptResult) {
		if res.Forfeited {
			forfeited.Store(true)
		}
		if res.UseKi {
			useKi.Store(true)
		}
	}); err != nil {
		t.Fatalf("PromptStunningStrike: %v", err)
	}
	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	if len(row.Components) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(row.Components))
	}
	// First button = Use Ki
	useBtn := row.Components[0].(discordgo.Button)
	store.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: useBtn.CustomID},
	})
	if !useKi.Load() {
		t.Errorf("UseKi was not selected")
	}
	if forfeited.Load() {
		t.Errorf("unexpected forfeit")
	}
}

func TestClassFeaturePromptPoster_StunningStrike_Forfeit(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, 10*time.Millisecond)
	poster := NewClassFeaturePromptPoster(store)

	forfeited := atomic.Bool{}
	if err := poster.PromptStunningStrike(StunningStrikePromptArgs{
		ChannelID:    "ch-1",
		AttackerName: "Monk",
		TargetName:   "Goblin",
		KiAvailable:  2,
	}, func(res StunningStrikePromptResult) {
		if res.Forfeited {
			forfeited.Store(true)
		}
	}); err != nil {
		t.Fatalf("PromptStunningStrike: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if forfeited.Load() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("forfeit did not fire")
}

func TestClassFeaturePromptPoster_DivineSmite_SlotPicker(t *testing.T) {
	mock, sent := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, time.Hour)
	poster := NewClassFeaturePromptPoster(store)

	got := atomic.Int32{}
	got.Store(-1)
	if err := poster.PromptDivineSmite(DivineSmitePromptArgs{
		ChannelID:      "ch-1",
		AttackerName:   "Paladin",
		TargetName:     "Demon",
		AvailableSlots: []int{1, 2, 3},
	}, func(res DivineSmitePromptResult) {
		if res.UseSlot {
			got.Store(int32(res.SlotLevel))
		}
	}); err != nil {
		t.Fatalf("PromptDivineSmite: %v", err)
	}
	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	if len(row.Components) != 4 {
		t.Fatalf("expected 4 buttons (3 slots + skip), got %d", len(row.Components))
	}
	if !strings.Contains((*sent)[0].Content, "Demon") {
		t.Errorf("content should mention target, got %q", (*sent)[0].Content)
	}
	// Click slot 2 (index 1)
	btn := row.Components[1].(discordgo.Button)
	store.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: btn.CustomID},
	})
	if g := got.Load(); g != 2 {
		t.Errorf("SlotLevel = %d, want 2", g)
	}
}

func TestClassFeaturePromptPoster_DivineSmite_Skip(t *testing.T) {
	mock, sent := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, time.Hour)
	poster := NewClassFeaturePromptPoster(store)

	skipped := atomic.Bool{}
	useSlot := atomic.Bool{}
	if err := poster.PromptDivineSmite(DivineSmitePromptArgs{
		ChannelID:      "ch-1",
		AttackerName:   "Paladin",
		TargetName:     "Demon",
		AvailableSlots: []int{1},
	}, func(res DivineSmitePromptResult) {
		if !res.UseSlot && !res.Forfeited {
			skipped.Store(true)
		}
		if res.UseSlot {
			useSlot.Store(true)
		}
	}); err != nil {
		t.Fatalf("PromptDivineSmite: %v", err)
	}
	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	skipBtn := row.Components[len(row.Components)-1].(discordgo.Button)
	store.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: skipBtn.CustomID},
	})
	if !skipped.Load() {
		t.Errorf("skipped not registered")
	}
	if useSlot.Load() {
		t.Errorf("UseSlot fired on skip")
	}
}

func TestClassFeaturePromptPoster_UncannyDodge_Halve(t *testing.T) {
	mock, sent := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, time.Hour)
	poster := NewClassFeaturePromptPoster(store)

	halved := atomic.Bool{}
	if err := poster.PromptUncannyDodge(UncannyDodgePromptArgs{
		ChannelID:      "ch-1",
		DefenderName:   "Rogue",
		AttackerName:   "Orc",
		IncomingDamage: 12,
	}, func(res UncannyDodgePromptResult) {
		if res.Halve {
			halved.Store(true)
		}
	}); err != nil {
		t.Fatalf("PromptUncannyDodge: %v", err)
	}
	row := (*sent)[0].Components[0].(discordgo.ActionsRow)
	if len(row.Components) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(row.Components))
	}
	halveBtn := row.Components[0].(discordgo.Button)
	store.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: halveBtn.CustomID},
	})
	if !halved.Load() {
		t.Errorf("Halve was not selected")
	}
}

func TestClassFeaturePromptPoster_BardicInspiration_30sTimeout(t *testing.T) {
	mock, sent := captureSentComplex()
	// override TTL to 15ms to keep test fast; production wires 30s via DefaultReactionPromptTTL
	store := NewReactionPromptStoreWithTTL(mock, 15*time.Millisecond)
	poster := NewClassFeaturePromptPoster(store)

	forfeited := atomic.Bool{}
	if err := poster.PromptBardicInspiration(BardicInspirationPromptArgs{
		ChannelID:   "ch-1",
		HolderName:  "Fighter",
		Die:         "d8",
		Context:     "attack roll",
	}, func(res BardicInspirationPromptResult) {
		if res.Forfeited {
			forfeited.Store(true)
		}
	}); err != nil {
		t.Fatalf("PromptBardicInspiration: %v", err)
	}
	if len((*sent)[0].Components[0].(discordgo.ActionsRow).Components) != 2 {
		t.Fatalf("expected 2 buttons")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if forfeited.Load() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("forfeit did not fire")
}

func TestClassFeaturePromptPoster_BardicInspiration_Use(t *testing.T) {
	mock, sent := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, time.Hour)
	poster := NewClassFeaturePromptPoster(store)

	used := atomic.Bool{}
	if err := poster.PromptBardicInspiration(BardicInspirationPromptArgs{
		ChannelID:   "ch-1",
		HolderName:  "Fighter",
		Die:         "d8",
		Context:     "attack roll",
	}, func(res BardicInspirationPromptResult) {
		if res.UseDie {
			used.Store(true)
		}
	}); err != nil {
		t.Fatalf("PromptBardicInspiration: %v", err)
	}
	useBtn := (*sent)[0].Components[0].(discordgo.ActionsRow).Components[0].(discordgo.Button)
	store.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: useBtn.CustomID},
	})
	if !used.Load() {
		t.Errorf("UseDie not selected")
	}
}

func TestClassFeaturePromptPoster_RejectsEmptyInputs(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStore(mock)
	poster := NewClassFeaturePromptPoster(store)
	if err := poster.PromptDivineSmite(DivineSmitePromptArgs{
		ChannelID:      "ch-1",
		AttackerName:   "P",
		TargetName:     "T",
		AvailableSlots: nil,
	}, func(DivineSmitePromptResult) {}); err == nil {
		t.Errorf("expected error for no available slots")
	}
}
