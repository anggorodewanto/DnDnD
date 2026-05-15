package portal_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePointBuy_AllEights(t *testing.T) {
	scores := portal.PointBuyScores{STR: 8, DEX: 8, CON: 8, INT: 8, WIS: 8, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.NoError(t, err)
}

func TestValidatePointBuy_Standard(t *testing.T) {
	// 15(9) + 14(7) + 13(5) + 12(4) + 10(2) + 8(0) = 27
	scores := portal.PointBuyScores{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.NoError(t, err)
}

func TestValidatePointBuy_TooManyPoints(t *testing.T) {
	scores := portal.PointBuyScores{STR: 15, DEX: 15, CON: 15, INT: 15, WIS: 15, CHA: 15}
	err := portal.ValidatePointBuy(scores)
	assert.ErrorIs(t, err, portal.ErrPointBuyOverspent)
}

func TestValidatePointBuy_ScoreTooLow(t *testing.T) {
	scores := portal.PointBuyScores{STR: 7, DEX: 8, CON: 8, INT: 8, WIS: 8, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.ErrorIs(t, err, portal.ErrScoreOutOfRange)
}

func TestValidatePointBuy_ScoreTooHigh(t *testing.T) {
	scores := portal.PointBuyScores{STR: 18, DEX: 8, CON: 8, INT: 8, WIS: 8, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.ErrorIs(t, err, portal.ErrScoreOutOfRange)
}

func TestValidatePointBuy_Exact27(t *testing.T) {
	// 15(9) + 15(9) + 8(0) + 8(0) + 8(0) + 8(0) = 18 -- underspent is OK
	scores := portal.PointBuyScores{STR: 15, DEX: 15, CON: 8, INT: 8, WIS: 8, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.NoError(t, err)
}

func TestValidateAbilityScores_MethodRules(t *testing.T) {
	tests := []struct {
		name    string
		sub     portal.CharacterSubmission
		wantErr string
	}{
		{
			name: "point buy",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodPointBuy
				return sub
			}(),
		},
		{
			name: "standard array",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodStandardArray
				sub.AbilityScores = portal.PointBuyScores{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8}
				return sub
			}(),
		},
		{
			name: "roll with 4d6 drop lowest details",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodRoll
				sub.AbilityScores = portal.PointBuyScores{STR: 15, DEX: 13, CON: 12, INT: 12, WIS: 9, CHA: 6}
				sub.AbilityRolls = map[string][]int{
					"str": []int{6, 5, 4, 1},
					"dex": []int{6, 4, 3, 1},
					"con": []int{4, 4, 4, 1},
					"int": []int{6, 3, 2, 3},
					"wis": []int{2, 2, 5, 2},
					"cha": []int{1, 2, 3, 1},
				}
				return sub
			}(),
		},
		{
			name: "roll rejects mismatched score",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodRoll
				sub.AbilityScores = portal.PointBuyScores{STR: 10, DEX: 13, CON: 12, INT: 12, WIS: 9, CHA: 6}
				sub.AbilityRolls = map[string][]int{
					"str": []int{6, 5, 4, 1},
					"dex": []int{6, 4, 3, 1},
					"con": []int{4, 4, 4, 1},
					"int": []int{6, 3, 2, 3},
					"wis": []int{2, 2, 5, 2},
					"cha": []int{1, 2, 3, 1},
				}
				return sub
			}(),
			wantErr: "is less than 4d6 drop lowest",
		},
		{
			name: "roll rejects missing roll details",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodRoll
				sub.AbilityScores = portal.PointBuyScores{STR: 18, DEX: 18, CON: 18, INT: 18, WIS: 18, CHA: 18}
				return sub
			}(),
			wantErr: "roll must include four d6 results",
		},
		{
			name: "standard array rejects out of range",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodStandardArray
				sub.AbilityScores = portal.PointBuyScores{STR: 20, DEX: 15, CON: 13, INT: 12, WIS: 10, CHA: 8}
				return sub
			}(),
			wantErr: "standard array score out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := portal.ValidateAbilityScoreGeneration(tt.sub)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateSubmission_EmptyName(t *testing.T) {
	sub := validSubmission()
	sub.Name = ""
	errs := portal.ValidateSubmission(sub)
	assert.Contains(t, errs, "name is required")
}

func TestValidateSubmission_EmptyRace(t *testing.T) {
	sub := validSubmission()
	sub.Race = ""
	errs := portal.ValidateSubmission(sub)
	assert.Contains(t, errs, "race is required")
}

func TestValidateSubmission_EmptyClass(t *testing.T) {
	sub := validSubmission()
	sub.Class = ""
	errs := portal.ValidateSubmission(sub)
	assert.Contains(t, errs, "class is required")
}

func TestValidateSubmission_InvalidPointBuy(t *testing.T) {
	sub := validSubmission()
	sub.AbilityScores = portal.PointBuyScores{STR: 15, DEX: 15, CON: 15, INT: 15, WIS: 15, CHA: 15}
	errs := portal.ValidateSubmission(sub)
	assert.NotEmpty(t, errs)
}

func TestValidateSubmission_Valid(t *testing.T) {
	sub := validSubmission()
	errs := portal.ValidateSubmission(sub)
	assert.Empty(t, errs)
}

func validSubmission() portal.CharacterSubmission {
	return portal.CharacterSubmission{
		Name:          "Thorin",
		Race:          "dwarf",
		Background:    "soldier",
		Class:         "fighter",
		AbilityScores: portal.PointBuyScores{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8},
		Skills:        []string{"athletics", "perception"},
	}
}

func TestBuilderService_CreateCharacter_Valid(t *testing.T) {
	store := &mockBuilderStore{
		charID: "char-uuid-123",
		pcID:   "pc-uuid-456",
	}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "char-uuid-123", result.CharacterID)
	assert.Equal(t, "pc-uuid-456", result.PlayerCharacterID)

	// Verify store was called correctly
	assert.Equal(t, "Thorin", store.lastCharName)
	assert.Equal(t, "fighter", store.lastCharClass)
	assert.Equal(t, "pending", store.lastPCStatus)
	assert.Equal(t, "create", store.lastPCCreatedVia)
	assert.Equal(t, "discord-user-1", store.lastPCDiscordUserID)
}

func TestBuilderService_CreateCharacter_NotifiesDMQueue(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	notifier := &mockDMQueueNotifier{}
	svc := portal.NewBuilderService(store, portal.WithNotifier(notifier))

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.True(t, notifier.called)
	assert.Equal(t, "Thorin", notifier.charName)
	assert.Equal(t, "discord-user-1", notifier.playerID)
	assert.Equal(t, "portal-create", notifier.via)
}

func TestBuilderService_CreateCharacter_NotifierErrorDoesNotFail(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	notifier := &mockDMQueueNotifier{err: errors.New("discord down")}
	svc := portal.NewBuilderService(store, portal.WithNotifier(notifier))

	sub := validSubmission()
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
}

func TestBuilderService_WithLogger(t *testing.T) {
	logger := slog.Default()
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store, portal.WithLogger(logger))
	// Verify it doesn't panic and produces results
	sub := validSubmission()
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
}

func TestBuilderService_NotifierError_WithLogger(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	notifier := &mockDMQueueNotifier{err: errors.New("discord down")}
	logger := slog.Default()
	svc := portal.NewBuilderService(store, portal.WithNotifier(notifier), portal.WithLogger(logger))

	sub := validSubmission()
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
	assert.True(t, notifier.called)
}

func TestBuilderService_RedeemTokenError_WithLogger(t *testing.T) {
	store := &mockBuilderStore{
		charID:         "c-1",
		pcID:           "pc-1",
		redeemTokenErr: errors.New("token expired"),
	}
	logger := slog.Default()
	svc := portal.NewBuilderService(store, portal.WithLogger(logger))

	sub := validSubmission()
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
	// Should succeed even though redeem failed
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
}

func TestBuilderService_CreateCharacter_InvalidSubmission(t *testing.T) {
	store := &mockBuilderStore{}
	svc := portal.NewBuilderService(store)

	sub := portal.CharacterSubmission{} // empty
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestBuilderService_CreateCharacter_StoreError(t *testing.T) {
	store := &mockBuilderStore{
		createCharErr: errors.New("db down"),
	}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestBuilderService_CreateCharacter_RedeemToken(t *testing.T) {
	store := &mockBuilderStore{
		charID: "char-1",
		pcID:   "pc-1",
	}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "tok-abc", store.lastRedeemedToken)
}

func TestBuilderService_CreateCharacter_RejectsDisallowedAbilityMethod(t *testing.T) {
	store := &mockBuilderStore{charID: "char-1", pcID: "pc-1"}
	provider := portal.StaticAbilityMethodProvider([]portal.AbilityScoreMethod{portal.AbilityMethodPointBuy})
	svc := portal.NewBuilderService(store, portal.WithAbilityMethodProvider(provider))

	sub := validSubmission()
	sub.AbilityMethod = portal.AbilityMethodStandardArray
	sub.AbilityScores = portal.PointBuyScores{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8}

	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ability score method standard_array is not allowed")
}

// mockBuilderStore implements portal.BuilderStore for testing.
type mockBuilderStore struct {
	charID         string
	pcID           string
	createCharErr  error
	createPCErr    error
	redeemTokenErr error

	lastCharName        string
	lastCharClass       string
	lastCharSubrace     string
	lastCharClasses     []character.ClassEntry
	lastCharEquipment   []string
	lastCharProfBonus   int
	lastPCStatus        string
	lastPCCreatedVia    string
	lastPCDiscordUserID string
	lastRedeemedToken   string
}

func (m *mockBuilderStore) CreateCharacterRecord(_ context.Context, p portal.CreateCharacterParams) (string, error) {
	m.lastCharName = p.Name
	m.lastCharClass = p.Class
	m.lastCharSubrace = p.Subrace
	m.lastCharClasses = p.Classes
	m.lastCharEquipment = p.Equipment
	m.lastCharProfBonus = p.ProfBonus
	if m.createCharErr != nil {
		return "", m.createCharErr
	}
	return m.charID, nil
}

func (m *mockBuilderStore) CreatePlayerCharacterRecord(_ context.Context, p portal.CreatePlayerCharacterParams) (string, error) {
	m.lastPCStatus = p.Status
	m.lastPCCreatedVia = p.CreatedVia
	m.lastPCDiscordUserID = p.DiscordUserID
	if m.createPCErr != nil {
		return "", m.createPCErr
	}
	return m.pcID, nil
}

func (m *mockBuilderStore) RedeemToken(_ context.Context, token string) error {
	m.lastRedeemedToken = token
	return m.redeemTokenErr
}

// mockDMQueueNotifier implements portal.DMQueueNotifier for testing.
type mockDMQueueNotifier struct {
	called   bool
	charName string
	playerID string
	via      string
	err      error
}

func (m *mockDMQueueNotifier) NotifyDMQueue(ctx context.Context, charName, playerDiscordID, via string) error {
	m.called = true
	m.charName = charName
	m.playerID = playerDiscordID
	m.via = via
	return m.err
}

func TestBuilderService_CreateCharacter_PCStoreError(t *testing.T) {
	store := &mockBuilderStore{
		charID:      "char-1",
		createPCErr: errors.New("pc db error"),
	}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pc db error")
}

func TestBuilderService_CreateCharacter_Barbarian(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Class = "barbarian"
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
}

func TestBuilderService_CreateCharacter_Wizard(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Class = "wizard"
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
}

func TestBuilderService_CreateCharacter_Paladin(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Class = "paladin"
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
}

func TestBuilderService_CreateCharacter_Rogue(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Class = "rogue"
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
}

func TestBuilderService_CreateCharacter_WithEquipment(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Equipment = []string{"longsword", "chain-mail", "shield"}
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
	assert.Equal(t, []string{"longsword", "chain-mail", "shield"}, store.lastCharEquipment)
}

// TestBuilderService_CreateCharacter_PassesMulticlass confirms a payload
// with `Classes` set is threaded through CreateCharacterParams so the
// adapter can persist the multiclass entries.
func TestBuilderService_CreateCharacter_PassesMulticlass(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Classes = []character.ClassEntry{
		{Class: "fighter", Subclass: "champion", Level: 5},
		{Class: "wizard", Subclass: "evocation", Level: 3},
	}

	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
	assert.NoError(t, err)
	assert.Len(t, store.lastCharClasses, 2)
	assert.Equal(t, "fighter", store.lastCharClasses[0].Class)
	assert.Equal(t, "champion", store.lastCharClasses[0].Subclass)
	assert.Equal(t, "wizard", store.lastCharClasses[1].Class)
}

func TestBuilderService_CreateCharacter_ProficiencyBonusFromTotalLevel(t *testing.T) {
	tests := []struct {
		name    string
		classes []character.ClassEntry
		want    int
	}{
		{
			name: "fighter 5",
			classes: []character.ClassEntry{
				{Class: "fighter", Subclass: "champion", Level: 5},
			},
			want: 3,
		},
		{
			name: "fighter 3 rogue 2",
			classes: []character.ClassEntry{
				{Class: "fighter", Subclass: "champion", Level: 3},
				{Class: "rogue", Subclass: "thief", Level: 2},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
			svc := portal.NewBuilderService(store)

			sub := validSubmission()
			sub.Class = tt.classes[0].Class
			sub.Subclass = tt.classes[0].Subclass
			sub.Classes = tt.classes

			_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, store.lastCharProfBonus)
		})
	}
}

// TestBuilderService_CreateCharacter_PassesSubrace confirms the subrace
// from the submission ends up on CreateCharacterParams.
func TestBuilderService_CreateCharacter_PassesSubrace(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Race = "elf"
	sub.Subrace = "high-elf"

	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
	assert.NoError(t, err)
	assert.Equal(t, "high-elf", store.lastCharSubrace)
}

func TestPointBuyCost(t *testing.T) {
	tests := []struct {
		score int
		cost  int
	}{
		{8, 0},
		{9, 1},
		{10, 2},
		{11, 3},
		{12, 4},
		{13, 5},
		{14, 7},
		{15, 9},
	}
	for _, tt := range tests {
		cost, err := portal.PointBuyCost(tt.score)
		assert.NoError(t, err)
		assert.Equal(t, tt.cost, cost, "score %d", tt.score)
	}
}
