package registration

import "testing"

func TestValidTransitions_ChangesRequestedToPending(t *testing.T) {
	allowed, ok := validTransitions["changes_requested"]
	if !ok || !allowed["pending"] {
		t.Fatal("expected changes_requested -> pending to be allowed")
	}
}

func TestValidTransitions_RejectedToPending(t *testing.T) {
	allowed, ok := validTransitions["rejected"]
	if !ok || !allowed["pending"] {
		t.Fatal("expected rejected -> pending to be allowed")
	}
}
