package renderer

import (
	"image/color"
	"testing"
)

func TestHealthTier_String(t *testing.T) {
	tests := []struct {
		tier HealthTier
		want string
	}{
		{TierUninjured, "Uninjured"},
		{TierScratched, "Scratched"},
		{TierBloodied, "Bloodied"},
		{TierCritical, "Critical"},
		{TierDying, "Dying"},
		{TierDead, "Dead"},
		{TierStable, "Stable"},
		{HealthTier(99), "Unknown"},
	}
	for _, tc := range tests {
		if got := tc.tier.String(); got != tc.want {
			t.Errorf("HealthTier(%d).String() = %q, want %q", tc.tier, got, tc.want)
		}
	}
}

func TestHealthTier_TierColor(t *testing.T) {
	// Just verify each tier returns a distinct non-zero color.
	seen := map[color.RGBA]bool{}
	for _, tier := range []HealthTier{TierUninjured, TierScratched, TierBloodied, TierCritical, TierDying, TierDead, TierStable} {
		c := tier.TierColor()
		if c.A == 0 {
			t.Errorf("tier %v has zero alpha", tier)
		}
		seen[c] = true
	}
	if len(seen) < 5 { // at least 5 distinct colors
		t.Errorf("expected at least 5 distinct tier colors, got %d", len(seen))
	}
	// Unknown tier
	unknown := HealthTier(99).TierColor()
	if unknown.A == 0 {
		t.Error("unknown tier should still return a color")
	}
}

func TestCombatant_HealthTier(t *testing.T) {
	tests := []struct {
		name string
		c    Combatant
		want HealthTier
	}{
		{"full HP", Combatant{HPMax: 100, HPCurrent: 100}, TierUninjured},
		{"99%", Combatant{HPMax: 100, HPCurrent: 99}, TierScratched},
		{"75%", Combatant{HPMax: 100, HPCurrent: 75}, TierScratched},
		{"74%", Combatant{HPMax: 100, HPCurrent: 74}, TierBloodied},
		{"25%", Combatant{HPMax: 100, HPCurrent: 25}, TierBloodied},
		{"24%", Combatant{HPMax: 100, HPCurrent: 24}, TierCritical},
		{"1%", Combatant{HPMax: 100, HPCurrent: 1}, TierCritical},
		{"0 HP dying", Combatant{HPMax: 100, HPCurrent: 0, IsDying: true}, TierDying},
		{"0 HP dead", Combatant{HPMax: 100, HPCurrent: 0}, TierDead},
		{"0 HP stable", Combatant{HPMax: 100, HPCurrent: 0, IsStable: true}, TierStable},
		{"zero max HP", Combatant{HPMax: 0, HPCurrent: 0}, TierDead},
		{"positive HP zero max", Combatant{HPMax: 0, HPCurrent: 5}, TierUninjured},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.c.HealthTier(); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTerrainType_String(t *testing.T) {
	tests := []struct {
		terrain TerrainType
		want    string
	}{
		{TerrainOpenGround, "Open Ground"},
		{TerrainDifficultTerrain, "Difficult Terrain"},
		{TerrainWater, "Water"},
		{TerrainLava, "Lava"},
		{TerrainPit, "Pit"},
		{TerrainType(99), "Unknown"},
	}
	for _, tc := range tests {
		if got := tc.terrain.String(); got != tc.want {
			t.Errorf("TerrainType(%d).String() = %q, want %q", tc.terrain, got, tc.want)
		}
	}
}

func TestTerrainType_TerrainColor(t *testing.T) {
	for _, terrain := range []TerrainType{TerrainOpenGround, TerrainDifficultTerrain, TerrainWater, TerrainLava, TerrainPit} {
		c := terrain.TerrainColor()
		if c.A == 0 {
			t.Errorf("terrain %v has zero alpha", terrain)
		}
	}
	// Unknown terrain returns default
	unknown := TerrainType(99).TerrainColor()
	if unknown.A == 0 {
		t.Error("unknown terrain should still return a color")
	}
}
