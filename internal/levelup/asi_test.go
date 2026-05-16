package levelup

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
)

func TestApplyPlus2_RejectsWhenExceeds20(t *testing.T) {
	scores := character.AbilityScores{STR: 19}
	_, err := applyPlus2(scores, "str")
	if err == nil {
		t.Fatal("expected error when +2 would exceed 20, got nil")
	}
}
