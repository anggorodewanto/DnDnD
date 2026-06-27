package portal

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// ReviewCharacter is the normalized projection of a character that a DM reviews
// on the approval page. It is built the SAME way for both the pre-edit baseline
// and the current state so the frontend diff (diffStates) compares like-for-like.
// Set-like lists are sorted so a re-ordering never shows as a spurious change.
//
// JSON keys are the diff fields the frontend renders; arrays always marshal as
// [] (never null) so an empty list never diffs against an absent one.
type ReviewCharacter struct {
	Name            string                  `json:"name"`
	Race            string                  `json:"race"`
	Subrace         string                  `json:"subrace,omitempty"`
	Background      string                  `json:"background,omitempty"`
	Classes         []string                `json:"classes"`
	Level           int32                   `json:"level"`
	AbilityScores   character.AbilityScores `json:"ability_scores"`
	HPMax           int32                   `json:"hp_max"`
	AC              int32                   `json:"ac"`
	SpeedFt         int32                   `json:"speed_ft"`
	Skills          []string                `json:"skills"`
	Expertise       []string                `json:"expertise"`
	Saves           []string                `json:"saves"`
	Languages       []string                `json:"languages"`
	Equipment       []string                `json:"equipment"`
	Spells          []string                `json:"spells"`
	WeaponMasteries []string                `json:"weapon_masteries"`
	Features        []string                `json:"features"`
	Appearance      string                  `json:"appearance,omitempty"`
	Backstory       string                  `json:"backstory,omitempty"`
}

// ProjectReview reduces a persisted character to the DM-reviewable projection.
// Most fields come off refdata.Character columns; subrace/background/spells/
// weapon_masteries/appearance/backstory live in the character_data JSONB bag
// (the same place submissionFromCharacter reads them).
func ProjectReview(ch refdata.Character) ReviewCharacter {
	rc := ReviewCharacter{
		Name:    ch.Name,
		Race:    ch.Race,
		Level:   ch.Level,
		HPMax:   ch.HpMax,
		AC:      ch.Ac,
		SpeedFt: ch.SpeedFt,
		// Default every list to non-nil so JSON emits [] not null.
		Classes:         []string{},
		Skills:          []string{},
		Expertise:       []string{},
		Saves:           []string{},
		Languages:       sortedSet(ch.Languages),
		Equipment:       []string{},
		Spells:          []string{},
		WeaponMasteries: []string{},
		Features:        []string{},
	}

	var classes []character.ClassEntry
	if len(ch.Classes) > 0 {
		_ = json.Unmarshal(ch.Classes, &classes)
	}
	rc.Classes = reviewClassLabels(classes)

	if len(ch.AbilityScores) > 0 {
		_ = json.Unmarshal(ch.AbilityScores, &rc.AbilityScores)
	}

	if ch.Proficiencies.Valid {
		var prof character.Proficiencies
		if json.Unmarshal(ch.Proficiencies.RawMessage, &prof) == nil {
			rc.Skills = sortedSet(prof.Skills)
			rc.Expertise = sortedSet(prof.Expertise)
			rc.Saves = sortedSet(prof.Saves)
		}
	}

	if ch.Features.Valid {
		var feats []character.Feature
		if json.Unmarshal(ch.Features.RawMessage, &feats) == nil {
			names := make([]string, 0, len(feats))
			for _, f := range feats {
				if f.Name != "" {
					names = append(names, f.Name)
				}
			}
			rc.Features = sortedSet(names)
		}
	}

	if ch.Inventory.Valid {
		var items []character.InventoryItem
		if json.Unmarshal(ch.Inventory.RawMessage, &items) == nil {
			names := make([]string, 0, len(items))
			for _, it := range items {
				name := it.Name
				if name == "" {
					name = it.ItemID
				}
				if name != "" {
					names = append(names, name)
				}
			}
			rc.Equipment = sortedSet(names)
		}
	}

	if ch.CharacterData.Valid {
		var cd struct {
			Spells          []string `json:"spells"`
			WeaponMasteries []string `json:"weapon_masteries"`
			Subrace         string   `json:"subrace"`
			Background      string   `json:"background"`
			Appearance      string   `json:"appearance"`
			Backstory       string   `json:"backstory"`
		}
		if json.Unmarshal(ch.CharacterData.RawMessage, &cd) == nil {
			rc.Spells = sortedSet(cd.Spells)
			rc.WeaponMasteries = sortedSet(cd.WeaponMasteries)
			rc.Subrace = cd.Subrace
			rc.Background = cd.Background
			rc.Appearance = cd.Appearance
			rc.Backstory = cd.Backstory
		}
	}

	return rc
}

// reviewClassLabels renders multiclass entries as readable labels in stored
// order (primary first), e.g. "Fighter 3 (Champion)" or "Rogue 1". Always
// returns a non-nil slice.
func reviewClassLabels(classes []character.ClassEntry) []string {
	out := make([]string, 0, len(classes))
	for _, ce := range classes {
		if ce.Class == "" {
			continue
		}
		label := fmt.Sprintf("%s %d", reviewDisplayName(ce.Class), ce.Level)
		if ce.Subclass != "" {
			label += fmt.Sprintf(" (%s)", reviewDisplayName(ce.Subclass))
		}
		out = append(out, label)
	}
	return out
}

// reviewDisplayName title-cases a slug for display: "battle-master" ->
// "Battle Master", "fighter" -> "Fighter".
func reviewDisplayName(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '-' || r == '_' })
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// sortedSet returns a sorted copy of in as a non-nil slice (empty input yields
// an empty, non-nil slice so it marshals to []).
func sortedSet(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}
