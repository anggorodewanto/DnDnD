package check

import "testing"

// TestGroupCheck_Exhaustion pins that a group-check participant's 2024
// exhaustion (-2 x level) lowers their d20 total vs the DC, mirroring the
// SingleCheck fold. GroupCheck is the live DM-driven path (dashboard), where
// every participant is a combatant carrying an ExhaustionLevel.
func TestGroupCheck_Exhaustion(t *testing.T) {
	svc := NewService(fixedRoller(14))

	result := svc.GroupCheck(GroupCheckInput{
		DC: 15,
		Participants: []GroupParticipant{
			// 14 + 5 - 6 (exhaustion 3) = 13 < 15 -> fail.
			{Name: "Exhausted", Modifier: 5, ExhaustionLevel: 3},
			// 14 + 5 - 0 = 19 >= 15 -> pass (control).
			{Name: "Fresh", Modifier: 5, ExhaustionLevel: 0},
		},
	})

	if result.Passed != 1 || result.Failed != 1 {
		t.Fatalf("expected 1 passed / 1 failed, got %d/%d", result.Passed, result.Failed)
	}
	if got := result.Results[0]; got.Passed || got.D20.Total != 13 {
		t.Errorf("exhausted participant: expected fail with total 13, got passed=%v total=%d", got.Passed, got.D20.Total)
	}
	if got := result.Results[1]; !got.Passed || got.D20.Total != 19 {
		t.Errorf("fresh participant: expected pass with total 19, got passed=%v total=%d", got.Passed, got.D20.Total)
	}
}

// TestContestedCheck_Exhaustion pins that exhaustion folds into BOTH sides of
// a contested check (each roll is a 2024 d20 Test). The initiator side is what
// the discord handler populates today (from input.ExhaustionLevel); the
// opponent side is folded at the function so it is correct the moment the
// resolver carries opponent exhaustion.
func TestContestedCheck_Exhaustion(t *testing.T) {
	t.Run("initiator exhaustion flips the result", func(t *testing.T) {
		svc := NewService(fixedRoller(12))
		result := svc.ContestedCheck(ContestedCheckInput{
			// 12 + 5 - 4 (exhaustion 2) = 13.
			Initiator: ContestedParticipant{Name: "Aria", Modifier: 5, ExhaustionLevel: 2},
			// 12 + 2 - 0 = 14.
			Opponent: ContestedParticipant{Name: "Goblin", Modifier: 2, ExhaustionLevel: 0},
		})
		if result.Winner != "Goblin" {
			t.Errorf("expected Goblin to win (13 < 14), got %q", result.Winner)
		}
		if result.InitiatorTotal != 13 || result.OpponentTotal != 14 {
			t.Errorf("expected 13 vs 14, got %d vs %d", result.InitiatorTotal, result.OpponentTotal)
		}
	})

	t.Run("opponent exhaustion flips the result", func(t *testing.T) {
		svc := NewService(fixedRoller(12))
		result := svc.ContestedCheck(ContestedCheckInput{
			// 12 + 2 - 0 = 14.
			Initiator: ContestedParticipant{Name: "Aria", Modifier: 2, ExhaustionLevel: 0},
			// 12 + 5 - 6 (exhaustion 3) = 11.
			Opponent: ContestedParticipant{Name: "Goblin", Modifier: 5, ExhaustionLevel: 3},
		})
		if result.Winner != "Aria" {
			t.Errorf("expected Aria to win (14 > 11), got %q", result.Winner)
		}
		if result.InitiatorTotal != 14 || result.OpponentTotal != 11 {
			t.Errorf("expected 14 vs 11, got %d vs %d", result.InitiatorTotal, result.OpponentTotal)
		}
	})
}
