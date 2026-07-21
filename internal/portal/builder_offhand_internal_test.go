package portal

import (
	"database/sql"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
)

// A dual-wielded off-hand weapon persisted in the equipped_off_hand column must
// survive a GET -> PUT edit round-trip, so submissionFromCharacter has to copy
// it back into the submission.
func TestSubmissionFromCharacter_RestoresDualWieldOffHand(t *testing.T) {
	ch := refdata.Character{
		Name:            "Duelist",
		Race:            "human",
		EquippedOffHand: sql.NullString{String: "dagger", Valid: true},
	}

	sub := submissionFromCharacter(ch)

	if sub.EquippedOffHand != "dagger" {
		t.Errorf("EquippedOffHand = %q, want dagger", sub.EquippedOffHand)
	}
}

// A shield off-hand round-trips via the Equipment list, so it must NOT also be
// copied into EquippedOffHand — that would double-count it on rebuild.
func TestSubmissionFromCharacter_ShieldOffHandNotCopied(t *testing.T) {
	ch := refdata.Character{
		Name:            "Knight",
		Race:            "human",
		EquippedOffHand: sql.NullString{String: "shield", Valid: true},
	}

	sub := submissionFromCharacter(ch)

	if sub.EquippedOffHand != "" {
		t.Errorf("EquippedOffHand = %q, want empty (shield rides the equipment list)", sub.EquippedOffHand)
	}
}
