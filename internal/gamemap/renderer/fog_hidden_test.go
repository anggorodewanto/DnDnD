package renderer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// F-H03: Hidden combatants (IsVisible=false) must be excluded from player view.
func TestFilterCombatantsForFog_HiddenExcluded(t *testing.T) {
	fow := &FogOfWar{
		Width:  5,
		Height: 5,
		States: make([]VisibilityState, 25),
	}
	fow.States[2*5+2] = Visible

	combatants := []Combatant{
		{ShortID: "E1", Col: 2, Row: 2, IsPlayer: false, IsVisible: true},  // visible enemy on visible tile
		{ShortID: "E2", Col: 2, Row: 2, IsPlayer: false, IsVisible: false}, // hidden enemy on visible tile
		{ShortID: "P1", Col: 2, Row: 2, IsPlayer: true, IsVisible: false},  // hidden player (still shown)
	}

	filtered := filterCombatantsForFog(combatants, fow)

	byID := make(map[string]Combatant)
	for _, c := range filtered {
		byID[c.ShortID] = c
	}

	assert.Contains(t, byID, "E1", "visible enemy should be included")
	assert.NotContains(t, byID, "E2", "hidden enemy must be excluded from player view")
	assert.Contains(t, byID, "P1", "players always shown regardless of IsVisible")
}

// F-H03: DMSeesAll=true must show hidden combatants.
func TestFilterCombatantsForFog_HiddenShownForDM(t *testing.T) {
	fow := &FogOfWar{
		Width:     5,
		Height:    5,
		States:    make([]VisibilityState, 25),
		DMSeesAll: true,
	}

	combatants := []Combatant{
		{ShortID: "E1", Col: 2, Row: 2, IsPlayer: false, IsVisible: false},
	}

	filtered := filterCombatantsForFog(combatants, fow)
	assert.Len(t, filtered, 1, "DMSeesAll must show hidden combatants")
}
