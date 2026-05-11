package combat

import "strings"

// ZoneDefinition holds the default zone parameters for a known persistent AoE spell.
//
// AnchorMode controls whether the zone moves with the caster or stays fixed
// at the cast location. "static" zones (the default) live at OriginCol/Row
// for their duration. "combatant" zones (e.g. Spirit Guardians, Aura of
// Protection) follow the caster — UpdateZoneAnchor is called whenever the
// anchor combatant moves. (med-26 / Phase 67)
type ZoneDefinition struct {
	SpellName             string
	OverlayColor          string
	MarkerIcon            string
	Shape                 string
	ZoneType              string
	AnchorMode            string
	RequiresConcentration bool
	Triggers              []ZoneTrigger
}

// ZoneTrigger describes when a zone effect fires and what it does.
type ZoneTrigger struct {
	Trigger string                 `json:"trigger"` // "enter", "start_of_turn"
	Effect  string                 `json:"effect"`  // "damage", "save"
	Details map[string]interface{} `json:"details,omitempty"`
}

// KnownZoneDefinitions maps spell names (lowercase) to their zone definitions.
var KnownZoneDefinitions = map[string]ZoneDefinition{
	"fog cloud": {
		SpellName:             "Fog Cloud",
		OverlayColor:          "#808080",
		MarkerIcon:            "\u2601",
		Shape:                 "circle",
		ZoneType:              "heavy_obscurement",
		RequiresConcentration: true,
	},
	"spirit guardians": {
		SpellName:             "Spirit Guardians",
		OverlayColor:          "#FFD700",
		MarkerIcon:            "\u2728",
		Shape:                 "circle",
		ZoneType:              "damage",
		AnchorMode:            "combatant",
		RequiresConcentration: true,
		Triggers: []ZoneTrigger{
			{Trigger: "enter", Effect: "damage"},
			{Trigger: "start_of_turn", Effect: "damage"},
		},
	},
	"wall of fire": {
		SpellName:             "Wall of Fire",
		OverlayColor:          "#FF4400",
		MarkerIcon:            "\U0001f525",
		Shape:                 "line",
		ZoneType:              "damage",
		RequiresConcentration: true,
		Triggers: []ZoneTrigger{
			{Trigger: "enter", Effect: "damage"},
			{Trigger: "start_of_turn", Effect: "damage"},
		},
	},
	"darkness": {
		SpellName:             "Darkness",
		OverlayColor:          "#330033",
		MarkerIcon:            "\u25FC",
		Shape:                 "circle",
		ZoneType:              "magical_darkness",
		RequiresConcentration: true,
	},
	"cloud of daggers": {
		SpellName:             "Cloud of Daggers",
		OverlayColor:          "#C0C0C0",
		MarkerIcon:            "\u2694",
		Shape:                 "square",
		ZoneType:              "damage",
		RequiresConcentration: true,
		Triggers: []ZoneTrigger{
			{Trigger: "enter", Effect: "damage"},
			{Trigger: "start_of_turn", Effect: "damage"},
		},
	},
	"moonbeam": {
		SpellName:             "Moonbeam",
		OverlayColor:          "#ADD8E6",
		MarkerIcon:            "\U0001f319",
		Shape:                 "circle",
		ZoneType:              "damage",
		RequiresConcentration: true,
		Triggers: []ZoneTrigger{
			{Trigger: "enter", Effect: "damage"},
			{Trigger: "start_of_turn", Effect: "damage"},
		},
	},
	"silence": {
		SpellName:             "Silence",
		OverlayColor:          "#4488CC",
		MarkerIcon:            "\U0001f507",
		Shape:                 "circle",
		ZoneType:              "control",
		RequiresConcentration: true,
	},
	"stinking cloud": {
		SpellName:             "Stinking Cloud",
		OverlayColor:          "#558822",
		MarkerIcon:            "\u2601",
		Shape:                 "circle",
		ZoneType:              "control",
		RequiresConcentration: true,
		Triggers: []ZoneTrigger{
			{Trigger: "enter", Effect: "save"},
		},
	},
}

// LookupZoneDefinition returns the zone definition for a spell name (case-insensitive).
func LookupZoneDefinition(spellName string) (ZoneDefinition, bool) {
	def, ok := KnownZoneDefinitions[strings.ToLower(spellName)]
	return def, ok
}
