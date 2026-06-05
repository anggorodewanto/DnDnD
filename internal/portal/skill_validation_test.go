package portal

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tieflingWarlock returns a base submission (no skills set) for a tiefling
// warlock with the Acolyte background (BG = insight, religion).
func tieflingWarlockSub(skills ...string) CharacterSubmission {
	return CharacterSubmission{
		Name:       "Zariel",
		Race:       "Tiefling",
		Background: "Acolyte",
		Classes: []character.ClassEntry{
			{Class: "Warlock", Level: 1, IsPrimary: true},
		},
		Skills: skills,
	}
}

func halfElfWarlockSub(skills ...string) CharacterSubmission {
	return CharacterSubmission{
		Name:       "Tav",
		Race:       "Half-Elf",
		Background: "Acolyte",
		Classes: []character.ClassEntry{
			{Class: "Warlock", Level: 1, IsPrimary: true},
		},
		Skills: skills,
	}
}

// tieflingTraits is the (empty of skill grants) trait set for a tiefling.
func tieflingTraits() []character.Feature {
	return []character.Feature{
		{Name: "Hellish Resistance", MechanicalEffect: "resistance_fire"},
		{Name: "Infernal Legacy", MechanicalEffect: "cantrip_thaumaturgy,level3_hellish_rebuke,level5_darkness"},
	}
}

// halfElfTraits carries the half-elf "gain 2 skills of your choice" trait.
func halfElfTraits() []character.Feature {
	return []character.Feature{
		{Name: "Fey Ancestry", MechanicalEffect: "advantage_saves_charmed,immune_magical_sleep"},
		{Name: "Skill Versatility", MechanicalEffect: "gain_2_skill_proficiencies"},
	}
}

// elfTraits carries a fixed race skill grant (Keen Senses → perception).
func elfTraits() []character.Feature {
	return []character.Feature{
		{Name: "Keen Senses", MechanicalEffect: "proficiency_perception"},
		{Name: "Fey Ancestry", MechanicalEffect: "advantage_saves_charmed,immune_magical_sleep"},
	}
}

func TestValidateSubmittedSkills_TieflingWarlock_Valid(t *testing.T) {
	// BG = insight, religion; RC = 0; warlock from-list includes arcana, deception.
	sub := tieflingWarlockSub("insight", "religion", "arcana", "deception")
	err := validateSubmittedSkills(sub, tieflingTraits())
	assert.NoError(t, err)
}

func TestValidateSubmittedSkills_TieflingWarlock_AllSkills_Rejected(t *testing.T) {
	all := make([]string, 0, len(character.SkillAbilityMap))
	for s := range character.SkillAbilityMap {
		all = append(all, s)
	}
	sub := tieflingWarlockSub(all...)
	err := validateSubmittedSkills(sub, tieflingTraits())
	require.Error(t, err)
}

func TestValidateSubmittedSkills_OffListClassSkill_Rejected(t *testing.T) {
	// stealth is NOT in the warlock from-list and RC=0 → no budget for it.
	sub := tieflingWarlockSub("insight", "religion", "arcana", "stealth")
	err := validateSubmittedSkills(sub, tieflingTraits())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stealth")
}

func TestValidateSubmittedSkills_MissingLockedSkill_Rejected(t *testing.T) {
	// religion (a locked background skill) is missing.
	sub := tieflingWarlockSub("insight", "arcana", "deception")
	err := validateSubmittedSkills(sub, tieflingTraits())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "religion")
}

func TestValidateSubmittedSkills_TooManyClassPicks_Rejected(t *testing.T) {
	// Three class-list picks but warlock CN=2 and RC=0.
	sub := tieflingWarlockSub("insight", "religion", "arcana", "deception", "history")
	err := validateSubmittedSkills(sub, tieflingTraits())
	require.Error(t, err)
}

func TestValidateSubmittedSkills_DuplicateSlug_Rejected(t *testing.T) {
	sub := tieflingWarlockSub("insight", "religion", "arcana", "arcana")
	err := validateSubmittedSkills(sub, tieflingTraits())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestValidateSubmittedSkills_NonCanonicalSlug_Rejected(t *testing.T) {
	sub := tieflingWarlockSub("insight", "religion", "arcana", "basket-weaving")
	err := validateSubmittedSkills(sub, tieflingTraits())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "basket-weaving")
}

func TestValidateSubmittedSkills_HalfElf_TwoOffListWithinBudget_Valid(t *testing.T) {
	// RC=2: 2 class-list picks (arcana, deception) + 2 arbitrary off-list (acrobatics, stealth).
	sub := halfElfWarlockSub("insight", "religion", "arcana", "deception", "acrobatics", "stealth")
	err := validateSubmittedSkills(sub, halfElfTraits())
	assert.NoError(t, err)
}

func TestValidateSubmittedSkills_HalfElf_ThirdOffList_Rejected(t *testing.T) {
	// A 3rd off-list skill exceeds RC=2.
	sub := halfElfWarlockSub("insight", "religion", "arcana", "deception", "acrobatics", "stealth", "athletics")
	err := validateSubmittedSkills(sub, halfElfTraits())
	require.Error(t, err)
}

func TestValidateSubmittedSkills_RaceFixedSkillMustBePresent(t *testing.T) {
	// Elf grants fixed perception (RF). It must appear in S (it is a locked skill).
	sub := CharacterSubmission{
		Name:       "Legolas",
		Race:       "Elf",
		Background: "Acolyte",
		Classes:    []character.ClassEntry{{Class: "Warlock", Level: 1, IsPrimary: true}},
		Skills:     []string{"insight", "religion", "arcana"}, // perception (RF) missing
	}
	err := validateSubmittedSkills(sub, elfTraits())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "perception")
}

func TestValidateSubmittedSkills_RaceFixedSkillPresent_Valid(t *testing.T) {
	// Elf perception present; the chosen class pick (arcana) fits CN=2.
	sub := CharacterSubmission{
		Name:       "Legolas",
		Race:       "Elf",
		Background: "Acolyte",
		Classes:    []character.ClassEntry{{Class: "Warlock", Level: 1, IsPrimary: true}},
		Skills:     []string{"insight", "religion", "perception", "arcana"},
	}
	err := validateSubmittedSkills(sub, elfTraits())
	assert.NoError(t, err)
}

func TestValidateSubmittedSkills_EmptySkills_NoError(t *testing.T) {
	// Empty client skills: validation is skipped (the derived fallback is trusted).
	sub := tieflingWarlockSub()
	err := validateSubmittedSkills(sub, tieflingTraits())
	assert.NoError(t, err)
}

func TestClassSkillChoices_Warlock(t *testing.T) {
	from, choose := classSkillChoices([]character.ClassEntry{{Class: "Warlock", Level: 1}})
	assert.Equal(t, 2, choose)
	assert.Contains(t, from, "arcana")
	assert.Contains(t, from, "deception")
	assert.NotContains(t, from, "stealth")
}

func TestClassSkillChoices_NoClass(t *testing.T) {
	from, choose := classSkillChoices(nil)
	assert.Empty(t, from)
	assert.Equal(t, 0, choose)
}

func TestParseRaceFixedSkills_KeenSenses(t *testing.T) {
	got := parseRaceFixedSkills(elfTraits())
	assert.Equal(t, []string{"perception"}, got)
}

func TestParseRaceFixedSkills_IgnoresNonSkillProficiencies(t *testing.T) {
	traits := []character.Feature{
		{Name: "Dwarven Combat Training", MechanicalEffect: "proficiency_battleaxe,proficiency_handaxe"},
	}
	assert.Empty(t, parseRaceFixedSkills(traits))
}

func TestParseRaceChooseCount_HalfElf(t *testing.T) {
	assert.Equal(t, 2, parseRaceChooseCount(halfElfTraits()))
}

func TestParseRaceChooseCount_Tiefling_Zero(t *testing.T) {
	assert.Equal(t, 0, parseRaceChooseCount(tieflingTraits()))
}
