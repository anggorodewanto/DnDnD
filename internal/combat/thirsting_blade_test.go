package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// warlockAttacksPerTurn resolves a level-5 warlock's attacks-per-action carrying
// the given feature slugs, through the real ResolveTurnResources path. The warlock
// class map is the flat {"1":1}, so any result >1 must come from Thirsting Blade.
func warlockAttacksPerTurn(t *testing.T, feats []CharacterFeature) int32 {
	t.Helper()
	char := makeCharacterWithFeats(8, 8, 3, "club", feats, []CharacterClass{{Class: "warlock", Level: 5}})
	combatant := refdata.Combatant{CharacterID: uuid.NullUUID{UUID: char.ID, Valid: true}}
	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getClassFn = func(_ context.Context, _ string) (refdata.Class, error) {
		return refdata.Class{ID: "warlock", AttacksPerAction: json.RawMessage(`{"1":1}`)}, nil
	}
	_, attacks, err := NewService(store).ResolveTurnResources(context.Background(), combatant)
	require.NoError(t, err)
	return attacks
}

func TestResolveTurnResources_ThirstingBlade_GrantsTwoAttacks(t *testing.T) {
	attacks := warlockAttacksPerTurn(t, []CharacterFeature{
		{Name: "Pact of the Blade", MechanicalEffect: "pact_of_the_blade"},
		{Name: "Thirsting Blade", MechanicalEffect: "thirsting_blade"},
	})
	assert.Equal(t, int32(2), attacks)
}

func TestResolveTurnResources_ThirstingBlade_RequiresPactBlade(t *testing.T) {
	// Thirsting Blade without the pact boon must not grant the extra attack.
	attacks := warlockAttacksPerTurn(t, []CharacterFeature{
		{Name: "Thirsting Blade", MechanicalEffect: "thirsting_blade"},
	})
	assert.Equal(t, int32(1), attacks)
}

func TestResolveTurnResources_PactBladeWithoutThirstingBlade_OneAttack(t *testing.T) {
	attacks := warlockAttacksPerTurn(t, []CharacterFeature{
		{Name: "Pact of the Blade", MechanicalEffect: "pact_of_the_blade"},
	})
	assert.Equal(t, int32(1), attacks)
}
