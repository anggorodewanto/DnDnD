package rest

import (
	"strings"
	"testing"
)

func TestFormatShortRestResult_ItemStudied(t *testing.T) {
	result := ShortRestResult{
		HPBefore:        50,
		HPAfter:         50,
		HPMax:           50,
		HPHealed:        0,
		ItemStudied:     true,
		StudiedItemName: "Ring of Invisibility",
	}

	got := FormatShortRestResult("Gandalf", result)

	if !strings.Contains(got, "Identified **Ring of Invisibility** during rest") {
		t.Errorf("expected item study line, got:\n%s", got)
	}
}

func TestFormatShortRestResult_NoItemStudied(t *testing.T) {
	result := ShortRestResult{
		HPBefore: 50,
		HPAfter:  50,
		HPMax:    50,
		HPHealed: 0,
	}

	got := FormatShortRestResult("Gandalf", result)

	if strings.Contains(got, "Identified") {
		t.Errorf("expected no item study line, got:\n%s", got)
	}
}
