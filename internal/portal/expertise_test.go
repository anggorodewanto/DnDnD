package portal

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
)

// classExpertiseGrant must mirror the SRD: only Rogue and Bard grant skill
// Expertise, gated by class level. Rogue: 2 at L1, +2 at L6. Bard: 2 at L3,
// +2 at L10. Every other class (and a Rogue/Bard below its first gate) grants 0.
func TestClassExpertiseGrant(t *testing.T) {
	cases := []struct {
		class string
		level int
		want  int
	}{
		{"rogue", 1, 2},
		{"rogue", 5, 2},
		{"rogue", 6, 4},
		{"rogue", 20, 4},
		{"bard", 1, 0},
		{"bard", 2, 0},
		{"bard", 3, 2},
		{"bard", 9, 2},
		{"bard", 10, 4},
		{"bard", 20, 4},
		{"fighter", 20, 0},
		{"wizard", 6, 0},
		{"", 1, 0},
	}
	for _, tc := range cases {
		got := classExpertiseGrant(tc.class, tc.level)
		assert.Equalf(t, tc.want, got, "classExpertiseGrant(%q, %d)", tc.class, tc.level)
	}
}

// classExpertiseGrant is case-insensitive on the class name, matching the way
// classSkillChoices lowercases its switch.
func TestClassExpertiseGrant_CaseInsensitive(t *testing.T) {
	assert.Equal(t, 2, classExpertiseGrant("Rogue", 1))
	assert.Equal(t, 2, classExpertiseGrant("BARD", 3))
}

// submissionExpertiseGrant reads the grant for the starting (first) class entry,
// using its explicit level (default 1 from SubmissionClasses).
func TestSubmissionExpertiseGrant(t *testing.T) {
	rogue := CharacterSubmission{Class: "rogue"} // L1 via SubmissionClasses default
	assert.Equal(t, 2, submissionExpertiseGrant(rogue))

	bardL3 := CharacterSubmission{Classes: []character.ClassEntry{{Class: "bard", Level: 3, IsPrimary: true}}}
	assert.Equal(t, 2, submissionExpertiseGrant(bardL3))

	bardL1 := CharacterSubmission{Class: "bard"} // L1 → no expertise yet
	assert.Equal(t, 0, submissionExpertiseGrant(bardL1))

	fighter := CharacterSubmission{Class: "fighter"}
	assert.Equal(t, 0, submissionExpertiseGrant(fighter))
}

// validateSubmittedExpertise rejects illegal expertise sets and accepts legal
// ones. Expertise must be a subset of the proficient skills and never exceed
// the class+level grant.
func TestValidateSubmittedExpertise(t *testing.T) {
	t.Run("empty is always allowed", func(t *testing.T) {
		sub := CharacterSubmission{Class: "fighter", Skills: []string{"athletics"}}
		assert.NoError(t, validateSubmittedExpertise(sub))
	})

	t.Run("rogue L1 two proficient skills is legal", func(t *testing.T) {
		sub := CharacterSubmission{
			Class:     "rogue",
			Skills:    []string{"stealth", "perception", "acrobatics"},
			Expertise: []string{"stealth", "perception"},
		}
		assert.NoError(t, validateSubmittedExpertise(sub))
	})

	t.Run("expertise on a non-proficient skill is rejected", func(t *testing.T) {
		sub := CharacterSubmission{
			Class:     "rogue",
			Skills:    []string{"stealth"},
			Expertise: []string{"arcana"},
		}
		assert.Error(t, validateSubmittedExpertise(sub))
	})

	t.Run("too many expertise picks is rejected", func(t *testing.T) {
		sub := CharacterSubmission{
			Class:     "rogue",
			Skills:    []string{"stealth", "perception", "acrobatics"},
			Expertise: []string{"stealth", "perception", "acrobatics"},
		}
		assert.Error(t, validateSubmittedExpertise(sub))
	})

	t.Run("non-expert class with any expertise is rejected", func(t *testing.T) {
		sub := CharacterSubmission{
			Class:     "fighter",
			Skills:    []string{"athletics"},
			Expertise: []string{"athletics"},
		}
		assert.Error(t, validateSubmittedExpertise(sub))
	})

	t.Run("duplicate and non-canonical slugs are rejected", func(t *testing.T) {
		dup := CharacterSubmission{
			Class:     "rogue",
			Skills:    []string{"stealth", "perception"},
			Expertise: []string{"stealth", "stealth"},
		}
		assert.Error(t, validateSubmittedExpertise(dup))

		bogus := CharacterSubmission{
			Class:     "rogue",
			Skills:    []string{"stealth"},
			Expertise: []string{"not-a-skill"},
		}
		assert.Error(t, validateSubmittedExpertise(bogus))
	})
}
