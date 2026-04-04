package portal_test

import (
	"context"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/stretchr/testify/assert"
)

func TestBuilderStoreAdapter_Implements_BuilderStore(t *testing.T) {
	// Compile-time check that BuilderStoreAdapter implements BuilderStore.
	var _ portal.BuilderStore = (*portal.BuilderStoreAdapter)(nil)
	assert.True(t, true)
}

func TestNewBuilderStoreAdapter(t *testing.T) {
	adapter := portal.NewBuilderStoreAdapter(nil, nil)
	assert.NotNil(t, adapter)
}

func TestBuilderStoreAdapter_RedeemToken_NilTokenSvc(t *testing.T) {
	adapter := portal.NewBuilderStoreAdapter(nil, nil)
	err := adapter.RedeemToken(context.Background(), "some-token")
	assert.NoError(t, err)
}


func TestDeriveCharacterSpeed_Default(t *testing.T) {
	// classHitDie is tested indirectly; test the exported DeriveSpeed.
	assert.Equal(t, 30, portal.DeriveSpeed("human"))
}

func TestClassHitDie(t *testing.T) {
	tests := []struct {
		class  string
		hitDie string
	}{
		{"barbarian", "d12"},
		{"fighter", "d10"},
		{"paladin", "d10"},
		{"ranger", "d10"},
		{"sorcerer", "d6"},
		{"wizard", "d6"},
		{"rogue", "d8"},
		{"cleric", "d8"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.hitDie, portal.ClassHitDie(tt.class), "class: %s", tt.class)
	}
}

func TestDeriveHP(t *testing.T) {
	scores := character.AbilityScores{STR: 10, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 10}
	// Fighter (d10) at level 1 with CON 14 (+2): 10 + 2 = 12
	hp := portal.DeriveHP("fighter", scores)
	assert.Equal(t, 12, hp)
}

func TestDeriveAC(t *testing.T) {
	scores := character.AbilityScores{STR: 10, DEX: 14, INT: 10, WIS: 10, CHA: 10, CON: 10}
	// No armor: 10 + DEX mod (2) = 12
	ac := portal.DeriveAC(scores)
	assert.Equal(t, 12, ac)
}
