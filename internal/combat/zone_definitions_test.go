package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 13: LookupZoneDefinition finds known spells case-insensitively
func TestLookupZoneDefinition_KnownSpells(t *testing.T) {
	tests := []struct {
		name      string
		spellName string
		wantFound bool
		wantType  string
		wantColor string
	}{
		{"fog cloud exact", "fog cloud", true, "heavy_obscurement", "#808080"},
		{"Fog Cloud mixed case", "Fog Cloud", true, "heavy_obscurement", "#808080"},
		{"spirit guardians", "spirit guardians", true, "damage", "#FFD700"},
		{"wall of fire", "wall of fire", true, "damage", "#FF4400"},
		{"darkness", "darkness", true, "magical_darkness", "#330033"},
		{"cloud of daggers", "cloud of daggers", true, "damage", "#C0C0C0"},
		{"moonbeam", "moonbeam", true, "damage", "#ADD8E6"},
		{"silence", "silence", true, "control", "#4488CC"},
		{"stinking cloud", "stinking cloud", true, "control", "#558822"},
		{"unknown spell", "magic missile", false, "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			def, ok := LookupZoneDefinition(tc.spellName)
			assert.Equal(t, tc.wantFound, ok)
			if ok {
				assert.Equal(t, tc.wantType, def.ZoneType)
				assert.Equal(t, tc.wantColor, def.OverlayColor)
			}
		})
	}
}

// TDD Cycle 14: Damage zone definitions have triggers
func TestZoneDefinition_DamageZonesHaveTriggers(t *testing.T) {
	damageSpells := []string{"spirit guardians", "wall of fire", "cloud of daggers", "moonbeam"}
	for _, spell := range damageSpells {
		def, ok := LookupZoneDefinition(spell)
		require.True(t, ok, "expected to find %s", spell)
		assert.NotEmpty(t, def.Triggers, "%s should have triggers", spell)
		assert.True(t, def.RequiresConcentration, "%s should require concentration", spell)
	}
}

// TDD Cycle 15: Non-damage zone definitions may not have triggers
func TestZoneDefinition_ControlZonesNoTriggers(t *testing.T) {
	def, ok := LookupZoneDefinition("fog cloud")
	require.True(t, ok)
	assert.Empty(t, def.Triggers)

	def, ok = LookupZoneDefinition("darkness")
	require.True(t, ok)
	assert.Empty(t, def.Triggers)

	def, ok = LookupZoneDefinition("silence")
	require.True(t, ok)
	assert.Empty(t, def.Triggers)
}

// Stinking cloud has enter trigger
func TestZoneDefinition_StinkingCloudHasEnterTrigger(t *testing.T) {
	def, ok := LookupZoneDefinition("stinking cloud")
	require.True(t, ok)
	require.Len(t, def.Triggers, 1)
	assert.Equal(t, "enter", def.Triggers[0].Trigger)
	assert.Equal(t, "save", def.Triggers[0].Effect)
}

func TestKnownZoneDefinitions_Count(t *testing.T) {
	assert.Len(t, KnownZoneDefinitions, 8)
}
