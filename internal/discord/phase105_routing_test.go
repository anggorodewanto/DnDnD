package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// Phase 105 iteration 2: command routing by combatant membership.
//
// Each test simulates a single guild with TWO simultaneous active encounters
// (X and Y). Player A (user-a) is in encounter X; player B (user-b) is in
// encounter Y. The encounter provider resolves via combatant membership, so
// the same slash command from user-a vs user-b must land on different
// encounters rather than picking one arbitrarily per guild.

type phase105Routing struct {
	encounterX, encounterY uuid.UUID
}

// provider returns a MoveEncounterProvider (shape-compatible with all the
// other *EncounterProvider interfaces in this package) that disambiguates by
// discord user ID. Unknown users get an error.
func (p phase105Routing) userProvider() func(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	return func(_ context.Context, _, discordUserID string) (uuid.UUID, error) {
		switch discordUserID {
		case "user-a":
			return p.encounterX, nil
		case "user-b":
			return p.encounterY, nil
		}
		return uuid.Nil, errors.New("not in any active encounter")
	}
}

// --- /move ---

func TestPhase105_MoveRoutesByCombatantMembership(t *testing.T) {
	routing := phase105Routing{encounterX: uuid.New(), encounterY: uuid.New()}

	cases := []struct {
		name           string
		userID         string
		wantEncounter  uuid.UUID
	}{
		{"player A", "user-a", routing.encounterX},
		{"player B", "user-b", routing.encounterY},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sess := &mockMoveSession{}
			handler, _, _, _ := setupMoveHandler(sess)

			var seenEncounters []uuid.UUID
			handler.encounterProvider = &mockMoveEncounterProvider{
				activeEncounterForUser: routing.userProvider(),
			}
			handler.combatService = &mockMoveService{
				getEncounter: func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
					seenEncounters = append(seenEncounters, id)
					return refdata.Encounter{
						ID:            id,
						Status:        "active",
						CurrentTurnID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
						MapID:         uuid.NullUUID{UUID: uuid.New(), Valid: true},
					}, nil
				},
				getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
					return refdata.Combatant{PositionCol: "A", PositionRow: 1, IsAlive: true}, nil
				},
				listCombatants: func(_ context.Context, id uuid.UUID) ([]refdata.Combatant, error) {
					seenEncounters = append(seenEncounters, id)
					return nil, nil
				},
				updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
					return refdata.Combatant{}, nil
				},
			}
			handler.mapProvider = &mockMoveMapProvider{
				getByID: func(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
					return refdata.Map{TiledJson: tiledJSON5x5()}, nil
				},
			}

			interaction := &discordgo.Interaction{
				Type:    discordgo.InteractionApplicationCommand,
				GuildID: "guild1",
				Member:  &discordgo.Member{User: &discordgo.User{ID: tc.userID}},
				Data: discordgo.ApplicationCommandInteractionData{
					Name: "move",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{Name: "coordinate", Value: "D4", Type: discordgo.ApplicationCommandOptionString},
					},
				},
			}
			handler.Handle(interaction)

			if len(seenEncounters) == 0 {
				t.Fatalf("expected combat service to be hit with an encounter ID")
			}
			if seenEncounters[0] != tc.wantEncounter {
				t.Errorf("expected encounter %s, got %s", tc.wantEncounter, seenEncounters[0])
			}
		})
	}
}

// --- /fly ---

func TestPhase105_FlyRoutesByCombatantMembership(t *testing.T) {
	routing := phase105Routing{encounterX: uuid.New(), encounterY: uuid.New()}

	for _, tc := range []struct {
		name          string
		userID        string
		wantEncounter uuid.UUID
	}{
		{"player A", "user-a", routing.encounterX},
		{"player B", "user-b", routing.encounterY},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sess := &mockMoveSession{}

			var seenEncounter uuid.UUID
			combatSvc := &mockMoveService{
				getEncounter: func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
					seenEncounter = id
					return refdata.Encounter{
						ID:            id,
						Status:        "active",
						CurrentTurnID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
					}, nil
				},
				getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
					return refdata.Combatant{IsAlive: true}, nil
				},
				listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
					return nil, nil
				},
				updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
					return refdata.Combatant{}, nil
				},
			}
			turnProv := &mockMoveTurnProvider{
				getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
					return refdata.Turn{MovementRemainingFt: 60}, nil
				},
			}
			encProv := &mockMoveEncounterProvider{activeEncounterForUser: routing.userProvider()}

			handler := NewFlyHandler(sess, combatSvc, turnProv, encProv)
			interaction := &discordgo.Interaction{
				Type:    discordgo.InteractionApplicationCommand,
				GuildID: "guild1",
				Member:  &discordgo.Member{User: &discordgo.User{ID: tc.userID}},
				Data: discordgo.ApplicationCommandInteractionData{
					Name: "fly",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{Name: "altitude", Value: float64(20), Type: discordgo.ApplicationCommandOptionInteger},
					},
				},
			}
			handler.Handle(interaction)

			if seenEncounter != tc.wantEncounter {
				t.Errorf("expected encounter %s, got %s", tc.wantEncounter, seenEncounter)
			}
		})
	}
}

// --- /distance ---

func TestPhase105_DistanceRoutesByCombatantMembership(t *testing.T) {
	routing := phase105Routing{encounterX: uuid.New(), encounterY: uuid.New()}

	for _, tc := range []struct {
		name          string
		userID        string
		wantEncounter uuid.UUID
	}{
		{"player A", "user-a", routing.encounterX},
		{"player B", "user-b", routing.encounterY},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sess := &mockMoveSession{}

			var seenEncounter uuid.UUID
			combatSvc := &mockMoveService{
				getEncounter: func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
					seenEncounter = id
					return refdata.Encounter{ID: id, Status: "active"}, nil
				},
				getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
					return refdata.Combatant{}, nil
				},
				listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
					return nil, nil
				},
				updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
					return refdata.Combatant{}, nil
				},
			}
			turnProv := &mockMoveTurnProvider{
				getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
					return refdata.Turn{}, nil
				},
			}
			encProv := &mockMoveEncounterProvider{activeEncounterForUser: routing.userProvider()}

			handler := NewDistanceHandler(sess, combatSvc, turnProv, encProv)
			interaction := &discordgo.Interaction{
				Type:    discordgo.InteractionApplicationCommand,
				GuildID: "guild1",
				Member:  &discordgo.Member{User: &discordgo.User{ID: tc.userID}},
				Data: discordgo.ApplicationCommandInteractionData{
					Name: "distance",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{Name: "target", Value: "G1", Type: discordgo.ApplicationCommandOptionString},
					},
				},
			}
			handler.Handle(interaction)

			if seenEncounter != tc.wantEncounter {
				t.Errorf("expected encounter %s, got %s", tc.wantEncounter, seenEncounter)
			}
		})
	}
}

// --- /done ---

func TestPhase105_DoneRoutesByCombatantMembership(t *testing.T) {
	routing := phase105Routing{encounterX: uuid.New(), encounterY: uuid.New()}

	for _, tc := range []struct {
		name          string
		userID        string
		wantEncounter uuid.UUID
	}{
		{"player A", "user-a", routing.encounterX},
		{"player B", "user-b", routing.encounterY},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sess := &mockMoveSession{}
			handler, _, _, _, _ := setupFullDoneHandler(sess)

			var seenEncounter uuid.UUID
			handler.encounterProvider = &mockMoveEncounterProvider{activeEncounterForUser: routing.userProvider()}
			origSvc := handler.combatService.(*mockMoveService)
			handler.combatService = &mockMoveService{
				getEncounter: func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
					seenEncounter = id
					return origSvc.getEncounter(ctx, id)
				},
				getCombatant:       origSvc.getCombatant,
				listCombatants:     origSvc.listCombatants,
				updateCombatantPos: origSvc.updateCombatantPos,
			}

			interaction := &discordgo.Interaction{
				Type:    discordgo.InteractionApplicationCommand,
				GuildID: "guild1",
				Member:  &discordgo.Member{User: &discordgo.User{ID: tc.userID}},
				Data: discordgo.ApplicationCommandInteractionData{Name: "done"},
			}
			// Only the player who owns the combatant in the returned encounter is
			// allowed to end the turn; for this routing test the setupFullDoneHandler
			// grants user1 as owner, so map both users to user1 via the campaign DM.
			handler.SetCampaignProvider(&mockDoneCampaignProvider{
				getCampaignByGuildID: func(_ context.Context, _ string) (refdata.Campaign, error) {
					return refdata.Campaign{DmUserID: tc.userID}, nil
				},
			})
			handler.Handle(interaction)

			if seenEncounter != tc.wantEncounter {
				t.Errorf("expected encounter %s, got %s", tc.wantEncounter, seenEncounter)
			}
		})
	}
}

// --- /summon /command ---

func TestPhase105_SummonCommandRoutesByCombatantMembership(t *testing.T) {
	routing := phase105Routing{encounterX: uuid.New(), encounterY: uuid.New()}

	for _, tc := range []struct {
		name          string
		userID        string
		wantEncounter uuid.UUID
	}{
		{"player A", "user-a", routing.encounterX},
		{"player B", "user-b", routing.encounterY},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var seenEncounter uuid.UUID
			svc := &mockSummonCommandService{
				commandCreatureFn: func(_ context.Context, input combat.CommandCreatureInput) (combat.CommandCreatureResult, error) {
					seenEncounter = input.EncounterID
					return combat.CommandCreatureResult{CombatLog: "ok"}, nil
				},
			}
			mock := newTestMock()
			mock.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
				return nil
			}

			handler := NewSummonCommandHandler(mock, svc)
			// Replace the encounter provider with a user-aware one matching
			// the routing table (mockSummonEncounterProvider is a constant mock,
			// so we wrap the user provider in a minimal adapter).
			handler.SetEncounterProvider(&summonCommandUserAwareProvider{fn: routing.userProvider()})
			handler.SetPlayerLookup(&mockSummonPlayerLookup{
				combatantID:   uuid.New(),
				combatantName: "Summoner",
			})

			interaction := &discordgo.Interaction{
				Type:    discordgo.InteractionApplicationCommand,
				GuildID: "guild1",
				Member:  &discordgo.Member{User: &discordgo.User{ID: tc.userID}},
				Data: discordgo.ApplicationCommandInteractionData{
					Name: "command",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{Name: "creature_id", Value: "A1", Type: discordgo.ApplicationCommandOptionString},
						{Name: "action", Value: "attack", Type: discordgo.ApplicationCommandOptionString},
					},
				},
			}
			handler.Handle(interaction)

			if seenEncounter != tc.wantEncounter {
				t.Errorf("expected encounter %s, got %s", tc.wantEncounter, seenEncounter)
			}
		})
	}
}

// summonCommandUserAwareProvider adapts a user-aware func to the
// SummonCommandEncounterProvider interface.
type summonCommandUserAwareProvider struct {
	fn func(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

func (p *summonCommandUserAwareProvider) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	return p.fn(ctx, guildID, discordUserID)
}

// --- /check uses combatant membership to pick conditions ---

func TestPhase105_CheckUsesInvokersEncounterForConditions(t *testing.T) {
	routing := phase105Routing{encounterX: uuid.New(), encounterY: uuid.New()}

	var seenEncounter uuid.UUID
	combatantLookup := &mockCheckCombatantLookup{
		listFn: func(_ context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
			seenEncounter = encounterID
			return nil, nil
		},
	}
	encProv := &mockCheckEncounterProvider{
		fnUser: routing.userProvider(),
	}

	_, ok := lookupCombatConditions(context.Background(), encProv, combatantLookup, "guild1", "user-b", uuid.New())
	_ = ok
	if seenEncounter != routing.encounterY {
		t.Errorf("expected check to query encounter Y for user-b, got %s", seenEncounter)
	}

	seenEncounter = uuid.Nil
	_, ok = lookupCombatConditions(context.Background(), encProv, combatantLookup, "guild1", "user-a", uuid.New())
	_ = ok
	if seenEncounter != routing.encounterX {
		t.Errorf("expected check to query encounter X for user-a, got %s", seenEncounter)
	}
}

// --- /recap ---

func TestPhase105_RecapRoutesByCombatantMembership(t *testing.T) {
	routing := phase105Routing{encounterX: uuid.New(), encounterY: uuid.New()}

	for _, tc := range []struct {
		name          string
		userID        string
		wantEncounter uuid.UUID
	}{
		{"player A", "user-a", routing.encounterX},
		{"player B", "user-b", routing.encounterY},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sess := &mockRecapSession{}

			var seenEncounter uuid.UUID
			svc := &mockRecapService{
				getEncounter: func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
					seenEncounter = id
					return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
				},
				listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
					return nil, nil
				},
			}
			encProv := &mockRecapEncounterProvider{activeEncounterForUser: routing.userProvider()}

			handler := NewRecapHandler(sess, svc, encProv, nil)
			interaction := &discordgo.Interaction{
				Type:    discordgo.InteractionApplicationCommand,
				GuildID: "guild1",
				Member:  &discordgo.Member{User: &discordgo.User{ID: tc.userID}},
				Data: discordgo.ApplicationCommandInteractionData{
					Name: "recap",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{Name: "rounds", Value: float64(1), Type: discordgo.ApplicationCommandOptionInteger},
					},
				},
			}
			handler.Handle(interaction)

			if seenEncounter != tc.wantEncounter {
				t.Errorf("expected encounter %s, got %s", tc.wantEncounter, seenEncounter)
			}
		})
	}
}

// --- Sanity: an unknown user is rejected by the provider and the handler
// returns the expected "no active encounter" message rather than silently
// picking one. ---

func TestPhase105_UnknownUser_MoveRejected(t *testing.T) {
	routing := phase105Routing{encounterX: uuid.New(), encounterY: uuid.New()}

	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	handler.encounterProvider = &mockMoveEncounterProvider{activeEncounterForUser: routing.userProvider()}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "stranger"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "move",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "coordinate", Value: "D4", Type: discordgo.ApplicationCommandOptionString},
			},
		},
	}
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "No active encounter for you") {
		t.Errorf("expected routing rejection, got: %s", content)
	}
}
