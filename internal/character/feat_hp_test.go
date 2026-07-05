package character

import "testing"

// FeatFlatHPBonus sums the flat per-level hit points granted by a character's
// feat features (Tough's +2 per level), detected by the mechanical-effect slug
// carried on each feature's serialized MechanicalEffect — not the feat name.
func TestFeatFlatHPBonus(t *testing.T) {
	toughEffect := `[{"effect_type":"hp_plus_2_per_level"}]`
	tests := []struct {
		name     string
		features []Feature
		level    int32
		want     int32
	}{
		{
			name:     "tough scales with level",
			features: []Feature{{Name: "Tough", Source: "feat", MechanicalEffect: toughEffect}},
			level:    4,
			want:     8,
		},
		{
			name: "sums across multiple flat-hp feats",
			features: []Feature{
				{Name: "Tough", Source: "feat", MechanicalEffect: toughEffect},
				{Name: "Tougher", Source: "feat", MechanicalEffect: toughEffect},
			},
			level: 3,
			want:  12,
		},
		{
			name:     "non-hp effect grants nothing",
			features: []Feature{{Name: "Alert", Source: "feat", MechanicalEffect: `[{"effect_type":"bonus_initiative","value":"5"}]`}},
			level:    4,
			want:     0,
		},
		{
			name:     "empty mechanical effect",
			features: []Feature{{Name: "Skilled", Source: "feat"}},
			level:    4,
			want:     0,
		},
		{
			name:     "unparseable mechanical effect",
			features: []Feature{{Name: "Broken", Source: "feat", MechanicalEffect: "not-json"}},
			level:    4,
			want:     0,
		},
		{
			name:     "no features",
			features: nil,
			level:    4,
			want:     0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FeatFlatHPBonus(tc.features, tc.level); got != tc.want {
				t.Errorf("FeatFlatHPBonus() = %d, want %d", got, tc.want)
			}
		})
	}
}
