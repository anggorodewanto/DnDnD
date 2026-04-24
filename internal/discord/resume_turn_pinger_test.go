package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/refdata"
)

// fakeResumeCombatService implements the subset of combat queries the pinger needs.
type fakeResumeCombatService struct {
	listFn    func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error)
	getTurnFn func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	getCombFn func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
}

func (f *fakeResumeCombatService) ListEncountersByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error) {
	return f.listFn(ctx, campaignID)
}
func (f *fakeResumeCombatService) GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return f.getTurnFn(ctx, id)
}
func (f *fakeResumeCombatService) GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return f.getCombFn(ctx, id)
}

// fakePlayerLookup implements ResumePlayerLookup.
type fakePlayerLookup struct {
	fn func(ctx context.Context, campaignID, characterID uuid.UUID) (refdata.PlayerCharacter, error)
}

func (f *fakePlayerLookup) GetPlayerCharacterByCharacter(ctx context.Context, campaignID, characterID uuid.UUID) (refdata.PlayerCharacter, error) {
	return f.fn(ctx, campaignID, characterID)
}

func newTestCampaignWithChannels(campaignID uuid.UUID, channels map[string]string) refdata.Campaign {
	raw, _ := json.Marshal(campaign.Settings{
		TurnTimeoutHours: 24,
		DiagonalRule:     "standard",
		ChannelIDs:       channels,
	})
	return refdata.Campaign{
		ID:       campaignID,
		GuildID:  "guild-1",
		Status:   campaign.StatusActive,
		Settings: pqtype.NullRawMessage{RawMessage: raw, Valid: true},
	}
}

// --- Phase 115 Resume re-ping tests ---

func TestResumeTurnPinger_RePingsCurrentTurnPlayer(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	characterID := uuid.New()

	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			if cid != campaignID {
				t.Fatalf("unexpected campaign %s", cid)
			}
			return []refdata.Encounter{
				{ID: uuid.New(), CampaignID: campaignID, Status: "completed", Name: "Old", RoundNumber: 1},
				{
					ID:            encounterID,
					CampaignID:    campaignID,
					Status:        "active",
					Name:          "Rooftop Ambush",
					RoundNumber:   3,
					CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
				},
			}, nil
		},
		getTurnFn: func(_ context.Context, id uuid.UUID) (refdata.Turn, error) {
			if id != turnID {
				t.Fatalf("unexpected turn %s", id)
			}
			return refdata.Turn{ID: turnID, CombatantID: combatantID, RoundNumber: 3, Status: "active"}, nil
		},
		getCombFn: func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				DisplayName: "Thordak",
				CharacterID: uuid.NullUUID{UUID: characterID, Valid: true},
				IsNpc:       false,
			}, nil
		},
	}
	players := &fakePlayerLookup{
		fn: func(_ context.Context, cid, chid uuid.UUID) (refdata.PlayerCharacter, error) {
			return refdata.PlayerCharacter{
				ID:            uuid.New(),
				CampaignID:    cid,
				CharacterID:   chid,
				DiscordUserID: "user-42",
			}, nil
		},
	}

	var sentChannel, sentContent string
	mock := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentChannel = channelID
			sentContent = content
			return &discordgo.Message{ID: "m-1"}, nil
		},
	}

	pinger := NewResumeTurnPinger(mock, combat, players)
	camp := newTestCampaignWithChannels(campaignID, map[string]string{
		"your-turn": "chan-turn",
		"the-story": "chan-story",
	})
	pinger.RePingCurrentTurn(context.Background(), camp)

	if sentChannel != "chan-turn" {
		t.Fatalf("expected send to chan-turn, got %q", sentChannel)
	}
	if !strings.Contains(sentContent, "Rooftop Ambush") {
		t.Fatalf("missing encounter name: %q", sentContent)
	}
	if !strings.Contains(sentContent, "<@user-42>") {
		t.Fatalf("expected @mention of user-42 in content: %q", sentContent)
	}
	if !strings.Contains(sentContent, "Round 3") {
		t.Fatalf("expected round marker: %q", sentContent)
	}
}

func TestResumeTurnPinger_NoActiveEncounter_SkipsPing(t *testing.T) {
	campaignID := uuid.New()
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{ID: uuid.New(), CampaignID: campaignID, Status: "completed"},
				{ID: uuid.New(), CampaignID: campaignID, Status: "preparing"},
			}, nil
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "m-x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, &fakePlayerLookup{})
	camp := newTestCampaignWithChannels(campaignID, map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
	if called {
		t.Fatal("no active encounter: nothing should be sent")
	}
}

func TestResumeTurnPinger_ActiveButNoCurrentTurn_SkipsPing(t *testing.T) {
	campaignID := uuid.New()
	encID := uuid.New()
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{ID: encID, CampaignID: campaignID, Status: "active", CurrentTurnID: uuid.NullUUID{Valid: false}},
			}, nil
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "m-x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, &fakePlayerLookup{})
	camp := newTestCampaignWithChannels(campaignID, map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
	if called {
		t.Fatal("active but no current turn: nothing should be sent")
	}
}

func TestResumeTurnPinger_NPCCombatant_SkipsPing(t *testing.T) {
	campaignID := uuid.New()
	encID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{{
				ID: encID, CampaignID: campaignID, Status: "active", Name: "x",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}}, nil
		},
		getTurnFn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
		},
		getCombFn: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, DisplayName: "Goblin", IsNpc: true, CharacterID: uuid.NullUUID{Valid: false}}, nil
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "m-x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, &fakePlayerLookup{})
	camp := newTestCampaignWithChannels(campaignID, map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
	if called {
		t.Fatal("NPC current turn: nothing should be sent")
	}
}

func TestResumeTurnPinger_MissingYourTurnChannel_SkipsPing(t *testing.T) {
	campaignID := uuid.New()
	encID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	characterID := uuid.New()
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{{
				ID: encID, CampaignID: campaignID, Status: "active", Name: "x",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}}, nil
		},
		getTurnFn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
		},
		getCombFn: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, DisplayName: "Arya", CharacterID: uuid.NullUUID{UUID: characterID, Valid: true}}, nil
		},
	}
	players := &fakePlayerLookup{
		fn: func(_ context.Context, _, _ uuid.UUID) (refdata.PlayerCharacter, error) {
			return refdata.PlayerCharacter{DiscordUserID: "user-1"}, nil
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "m-x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, players)
	camp := newTestCampaignWithChannels(campaignID, map[string]string{"the-story": "s"}) // no your-turn
	pinger.RePingCurrentTurn(context.Background(), camp)
	if called {
		t.Fatal("no your-turn channel: nothing should be sent")
	}
}

func TestResumeTurnPinger_PlayerLookupError_DoesNotPanic(t *testing.T) {
	campaignID := uuid.New()
	encID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	characterID := uuid.New()
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{{
				ID: encID, CampaignID: campaignID, Status: "active", Name: "x",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}}, nil
		},
		getTurnFn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: combatantID}, nil
		},
		getCombFn: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, DisplayName: "Arya", CharacterID: uuid.NullUUID{UUID: characterID, Valid: true}}, nil
		},
	}
	players := &fakePlayerLookup{
		fn: func(_ context.Context, _, _ uuid.UUID) (refdata.PlayerCharacter, error) {
			return refdata.PlayerCharacter{}, sql.ErrNoRows
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "m-x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, players)
	camp := newTestCampaignWithChannels(campaignID, map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp) // should not panic; should skip
	if called {
		t.Fatal("player lookup error: nothing should be sent")
	}
}

func TestResumeTurnPinger_SendError_DoesNotFailResume(t *testing.T) {
	campaignID := uuid.New()
	encID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	characterID := uuid.New()
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{{
				ID: encID, CampaignID: campaignID, Status: "active", Name: "x",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}}, nil
		},
		getTurnFn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: combatantID, RoundNumber: 1}, nil
		},
		getCombFn: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, DisplayName: "Arya", CharacterID: uuid.NullUUID{UUID: characterID, Valid: true}}, nil
		},
	}
	players := &fakePlayerLookup{
		fn: func(_ context.Context, _, _ uuid.UUID) (refdata.PlayerCharacter, error) {
			return refdata.PlayerCharacter{DiscordUserID: "user-1"}, nil
		},
	}
	mock := &MockSession{
		ChannelMessageSendFunc: func(_, _ string) (*discordgo.Message, error) {
			return nil, errors.New("boom")
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, players)
	camp := newTestCampaignWithChannels(campaignID, map[string]string{"your-turn": "chan-turn"})
	// Should not panic; best-effort.
	pinger.RePingCurrentTurn(context.Background(), camp)
}

func TestResumeTurnPinger_NilSession_NoOp(t *testing.T) {
	pinger := NewResumeTurnPinger(nil, &fakeResumeCombatService{}, &fakePlayerLookup{})
	camp := newTestCampaignWithChannels(uuid.New(), map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
}

func TestResumeTurnPinger_ListEncountersError_Skips(t *testing.T) {
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return nil, errors.New("db fail")
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(_, _ string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, &fakePlayerLookup{})
	camp := newTestCampaignWithChannels(uuid.New(), map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
	if called {
		t.Fatal("list error: nothing should be sent")
	}
}

func TestResumeTurnPinger_GetTurnError_Skips(t *testing.T) {
	campaignID := uuid.New()
	encID := uuid.New()
	turnID := uuid.New()
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{{
				ID: encID, CampaignID: campaignID, Status: "active",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}}, nil
		},
		getTurnFn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errors.New("boom")
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(_, _ string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, &fakePlayerLookup{})
	camp := newTestCampaignWithChannels(campaignID, map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
	if called {
		t.Fatal("GetTurn error: nothing should be sent")
	}
}

func TestResumeTurnPinger_GetCombatantError_Skips(t *testing.T) {
	campaignID := uuid.New()
	encID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{{
				ID: encID, CampaignID: campaignID, Status: "active",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}}, nil
		},
		getTurnFn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: combatantID}, nil
		},
		getCombFn: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("no combatant")
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(_, _ string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, &fakePlayerLookup{})
	camp := newTestCampaignWithChannels(campaignID, map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
	if called {
		t.Fatal("GetCombatant error: nothing should be sent")
	}
}

func TestResumeTurnPinger_PCWithoutCharacterID_Skips(t *testing.T) {
	campaignID := uuid.New()
	encID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{{
				ID: encID, CampaignID: campaignID, Status: "active",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}}, nil
		},
		getTurnFn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: combatantID}, nil
		},
		getCombFn: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			// PC flag-wise (not npc) but CharacterID is invalid — defensive path.
			return refdata.Combatant{ID: combatantID, DisplayName: "Mystery", IsNpc: false, CharacterID: uuid.NullUUID{Valid: false}}, nil
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(_, _ string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, &fakePlayerLookup{})
	camp := newTestCampaignWithChannels(campaignID, map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
	if called {
		t.Fatal("PC without CharacterID: nothing should be sent")
	}
}

func TestResumeTurnPinger_PCWithoutDiscordID_Skips(t *testing.T) {
	campaignID := uuid.New()
	encID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	characterID := uuid.New()
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{{
				ID: encID, CampaignID: campaignID, Status: "active",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}}, nil
		},
		getTurnFn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: combatantID}, nil
		},
		getCombFn: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, DisplayName: "X", CharacterID: uuid.NullUUID{UUID: characterID, Valid: true}}, nil
		},
	}
	players := &fakePlayerLookup{
		fn: func(_ context.Context, _, _ uuid.UUID) (refdata.PlayerCharacter, error) {
			return refdata.PlayerCharacter{DiscordUserID: ""}, nil
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(_, _ string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, players)
	camp := newTestCampaignWithChannels(campaignID, map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
	if called {
		t.Fatal("PC without DiscordUserID: nothing should be sent")
	}
}

func TestResumeTurnPinger_SettingsAbsent_Skips(t *testing.T) {
	pinger := NewResumeTurnPinger(&MockSession{}, &fakeResumeCombatService{}, &fakePlayerLookup{})
	camp := refdata.Campaign{ID: uuid.New(), GuildID: "g", Status: "active"} // Settings NOT valid
	pinger.RePingCurrentTurn(context.Background(), camp)
}

func TestResumeTurnPinger_EmptyYourTurnChannel_Skips(t *testing.T) {
	pinger := NewResumeTurnPinger(&MockSession{}, &fakeResumeCombatService{}, &fakePlayerLookup{})
	camp := newTestCampaignWithChannels(uuid.New(), map[string]string{"your-turn": ""}) // empty value
	pinger.RePingCurrentTurn(context.Background(), camp)
}

func TestResumeTurnPinger_NilCombat_NoOp(t *testing.T) {
	pinger := NewResumeTurnPinger(&MockSession{}, nil, &fakePlayerLookup{})
	camp := newTestCampaignWithChannels(uuid.New(), map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
}

func TestResumeTurnPinger_NilPlayers_NoOp(t *testing.T) {
	pinger := NewResumeTurnPinger(&MockSession{}, &fakeResumeCombatService{}, nil)
	camp := newTestCampaignWithChannels(uuid.New(), map[string]string{"your-turn": "chan-turn"})
	pinger.RePingCurrentTurn(context.Background(), camp)
}

func TestResumeTurnPinger_MalformedSettings_Skips(t *testing.T) {
	combat := &fakeResumeCombatService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Encounter, error) {
			return nil, nil
		},
	}
	var called bool
	mock := &MockSession{
		ChannelMessageSendFunc: func(_, _ string) (*discordgo.Message, error) {
			called = true
			return &discordgo.Message{ID: "x"}, nil
		},
	}
	pinger := NewResumeTurnPinger(mock, combat, &fakePlayerLookup{})
	camp := refdata.Campaign{
		ID:       uuid.New(),
		GuildID:  "g",
		Status:   campaign.StatusActive,
		Settings: pqtype.NullRawMessage{RawMessage: []byte("{not-json"), Valid: true},
	}
	pinger.RePingCurrentTurn(context.Background(), camp)
	if called {
		t.Fatal("malformed settings: nothing should be sent")
	}
}
