package charactercard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShortID_SingleWord(t *testing.T) {
	id := ShortID("Aria", nil)
	assert.Equal(t, "AR", id)
}

func TestShortID_TwoWords(t *testing.T) {
	id := ShortID("Thorn Ironheart", nil)
	assert.Equal(t, "TI", id)
}

func TestShortID_ThreeWords(t *testing.T) {
	id := ShortID("Sir Reginald Blackthorn", nil)
	assert.Equal(t, "SRB", id)
}

func TestShortID_Lowercase(t *testing.T) {
	id := ShortID("aria", nil)
	assert.Equal(t, "AR", id)
}

func TestShortID_DuplicateAppendNumber(t *testing.T) {
	existing := []string{"AR"}
	id := ShortID("Aria", existing)
	assert.Equal(t, "AR2", id)
}

func TestShortID_MultipleDuplicates(t *testing.T) {
	existing := []string{"AR", "AR2"}
	id := ShortID("Aria", existing)
	assert.Equal(t, "AR3", id)
}

func TestShortID_EmptyName(t *testing.T) {
	id := ShortID("", nil)
	assert.Equal(t, "X", id)
}

func TestShortID_SingleCharName(t *testing.T) {
	id := ShortID("A", nil)
	assert.Equal(t, "A", id)
}
