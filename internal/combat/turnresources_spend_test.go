package combat

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestSpendTurnResourceParams_BooleanResources(t *testing.T) {
	turnID := uuid.New()

	tests := []struct {
		name     string
		resource ResourceType
		want     func(p SpendParams) bool
	}{
		{
			name:     "action sets only spend_action",
			resource: ResourceAction,
			want: func(p SpendParams) bool {
				return p.SpendAction && !p.SpendBonusAction && !p.SpendReaction && !p.SpendFreeInteract
			},
		},
		{
			name:     "bonus action sets only spend_bonus_action",
			resource: ResourceBonusAction,
			want: func(p SpendParams) bool {
				return p.SpendBonusAction && !p.SpendAction && !p.SpendReaction && !p.SpendFreeInteract
			},
		},
		{
			name:     "reaction sets only spend_reaction",
			resource: ResourceReaction,
			want: func(p SpendParams) bool {
				return p.SpendReaction && !p.SpendAction && !p.SpendBonusAction && !p.SpendFreeInteract
			},
		},
		{
			name:     "free interact sets only spend_free_interact",
			resource: ResourceFreeInteract,
			want: func(p SpendParams) bool {
				return p.SpendFreeInteract && !p.SpendAction && !p.SpendBonusAction && !p.SpendReaction
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SpendTurnResourceParams(turnID, tc.resource)
			if err != nil {
				t.Fatalf("SpendTurnResourceParams(%s) returned error: %v", tc.resource, err)
			}
			if got.ID != turnID {
				t.Errorf("ID = %v, want %v", got.ID, turnID)
			}
			if !tc.want(got) {
				t.Errorf("wrong column targeted for %s: %+v", tc.resource, got)
			}
		})
	}
}

// A targeted spend must never carry state for resources it is not spending —
// that is the whole point of the query. Guarding it here so a future edit that
// reintroduces a full-struct copy fails loudly.
func TestSpendTurnResourceParams_TouchesNothingElse(t *testing.T) {
	got, err := SpendTurnResourceParams(uuid.New(), ResourceBonusAction)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SpendAction || got.SpendReaction || got.SpendFreeInteract {
		t.Errorf("bonus-action spend leaked into other resources: %+v", got)
	}
}

func TestSpendTurnResourceParams_RejectsNonBooleanResources(t *testing.T) {
	for _, resource := range []ResourceType{ResourceMovement, ResourceAttack, ResourceType("nonsense")} {
		t.Run(string(resource), func(t *testing.T) {
			_, err := SpendTurnResourceParams(uuid.New(), resource)
			if err == nil {
				t.Fatalf("SpendTurnResourceParams(%s) = nil error, want error", resource)
			}
			if !errors.Is(err, ErrUnspendableResource) {
				t.Errorf("error = %v, want ErrUnspendableResource", err)
			}
		})
	}
}
