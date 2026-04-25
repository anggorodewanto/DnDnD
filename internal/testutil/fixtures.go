package testutil

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// queryRunner is the subset of refdata.Queries the fixture helpers use. Tests
// can pass a *refdata.Queries directly; defining the interface keeps the
// helpers decoupled from any single concrete implementation.
type queryRunner interface {
	CreateCampaign(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error)
	CreateCharacter(ctx context.Context, arg refdata.CreateCharacterParams) (refdata.Character, error)
	CreatePlayerCharacter(ctx context.Context, arg refdata.CreatePlayerCharacterParams) (refdata.PlayerCharacter, error)
	CreateEncounter(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error)
	CreateCombatant(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error)
	CreateMap(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error)
}

// NewTestCampaign creates a campaign row with a unique guild_id suffix so
// parallel-capable tests cannot collide on the (guild_id, dm_user_id)
// uniqueness constraint. The supplied guildID is used as a stable prefix
// to make failure messages easier to read.
func NewTestCampaign(t *testing.T, q queryRunner, guildID string) refdata.Campaign {
	t.Helper()
	camp, err := q.CreateCampaign(context.Background(), refdata.CreateCampaignParams{
		GuildID:  guildID + "-" + uuid.NewString(),
		DmUserID: "dm-" + uuid.NewString(),
		Name:     "Test Campaign",
		Settings: pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
	})
	if err != nil {
		t.Fatalf("NewTestCampaign: CreateCampaign failed: %v", err)
	}
	return camp
}

// NewTestCharacter creates a level-N character with canned stats. Defaults
// pick values that pass the schema CHECK constraints (positive HP, AC, etc.).
// Callers needing custom fields can mutate the returned struct or use a
// dedicated helper.
func NewTestCharacter(t *testing.T, q queryRunner, campaignID uuid.UUID, name string, level int) refdata.Character {
	t.Helper()
	levelStr := strconv.Itoa(level)
	classes := json.RawMessage(`[{"class":"fighter","level":` + levelStr + `}]`)
	scores := json.RawMessage(`{"str":16,"dex":14,"con":14,"int":10,"wis":12,"cha":8}`)
	hp := int32(10 + level*8)
	char, err := q.CreateCharacter(context.Background(), refdata.CreateCharacterParams{
		CampaignID:       campaignID,
		Name:             name,
		Race:             "human",
		Classes:          classes,
		Level:            int32(level),
		AbilityScores:    scores,
		HpMax:            hp,
		HpCurrent:        hp,
		TempHp:           0,
		Ac:               16,
		SpeedFt:          30,
		ProficiencyBonus: profBonusForLevel(level),
		HitDiceRemaining: json.RawMessage(`{"d10":` + levelStr + `}`),
		Languages:        []string{"common"},
	})
	if err != nil {
		t.Fatalf("NewTestCharacter: CreateCharacter failed: %v", err)
	}
	return char
}

// NewTestPlayerCharacter links a character to a Discord user with status
// "approved" and created_via "register" — the most common shape needed by
// downstream tests.
func NewTestPlayerCharacter(t *testing.T, q queryRunner, campaignID, characterID uuid.UUID, discordUserID string) refdata.PlayerCharacter {
	t.Helper()
	pc, err := q.CreatePlayerCharacter(context.Background(), refdata.CreatePlayerCharacterParams{
		CampaignID:    campaignID,
		CharacterID:   characterID,
		DiscordUserID: discordUserID,
		Status:        "approved",
		CreatedVia:    "register",
	})
	if err != nil {
		t.Fatalf("NewTestPlayerCharacter: CreatePlayerCharacter failed: %v", err)
	}
	return pc
}

// NewTestEncounter creates an encounter for the campaign with no map and a
// "preparing" status — callers that need other shapes should use the lower
// level CreateEncounter directly.
func NewTestEncounter(t *testing.T, q queryRunner, campaignID uuid.UUID) refdata.Encounter {
	t.Helper()
	enc, err := q.CreateEncounter(context.Background(), refdata.CreateEncounterParams{
		CampaignID:  campaignID,
		Name:        "Test Encounter",
		Status:      "preparing",
		RoundNumber: 0,
	})
	if err != nil {
		t.Fatalf("NewTestEncounter: CreateEncounter failed: %v", err)
	}
	return enc
}

// NewTestCombatant creates a player-character-backed combatant in the given
// encounter with a sane default position (A,1) and full HP/AC.
func NewTestCombatant(t *testing.T, q queryRunner, encounterID, characterID uuid.UUID) refdata.Combatant {
	t.Helper()
	short := "c-" + uuid.NewString()[:8]
	comb, err := q.CreateCombatant(context.Background(), refdata.CreateCombatantParams{
		EncounterID:     encounterID,
		CharacterID:     uuid.NullUUID{UUID: characterID, Valid: true},
		ShortID:         short,
		DisplayName:     "Test Combatant",
		InitiativeRoll:  10,
		InitiativeOrder: 1,
		PositionCol:     "A",
		PositionRow:     1,
		AltitudeFt:      0,
		HpMax:           20,
		HpCurrent:       20,
		TempHp:          0,
		Ac:              15,
		Conditions:      json.RawMessage(`[]`),
		ExhaustionLevel: 0,
		IsVisible:       true,
		IsAlive:         true,
		IsNpc:           false,
	})
	if err != nil {
		t.Fatalf("NewTestCombatant: CreateCombatant failed: %v", err)
	}
	return comb
}

// NewTestMap creates a 20x20 empty map suitable for combat or exploration
// tests. Callers needing background images or tileset refs should use
// CreateMap directly.
func NewTestMap(t *testing.T, q queryRunner, campaignID uuid.UUID) refdata.Map {
	t.Helper()
	m, err := q.CreateMap(context.Background(), refdata.CreateMapParams{
		CampaignID:    campaignID,
		Name:          "Test Map",
		WidthSquares:  20,
		HeightSquares: 20,
		TiledJson:     json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("NewTestMap: CreateMap failed: %v", err)
	}
	return m
}

// profBonusForLevel returns the standard 5e proficiency bonus for a given
// total character level.
func profBonusForLevel(level int) int32 {
	if level <= 4 {
		return 2
	}
	if level <= 8 {
		return 3
	}
	if level <= 12 {
		return 4
	}
	if level <= 16 {
		return 5
	}
	return 6
}
