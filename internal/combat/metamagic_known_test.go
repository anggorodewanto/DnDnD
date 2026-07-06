package combat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

func metamagicFeatures(slugs ...string) pqtype.NullRawMessage {
	feats := make([]CharacterFeature, 0, len(slugs))
	for _, s := range slugs {
		feats = append(feats, CharacterFeature{Name: s, MechanicalEffect: s})
	}
	raw, err := json.Marshal(feats)
	if err != nil {
		panic(err)
	}
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}

func TestHasMetamagic(t *testing.T) {
	feats := metamagicFeatures("quickened", "twinned")
	if !HasMetamagic(feats, "quickened") {
		t.Error("HasMetamagic should find a learned option")
	}
	if !HasMetamagic(feats, "QUICKENED") {
		t.Error("HasMetamagic should be case-insensitive")
	}
	if HasMetamagic(feats, "subtle") {
		t.Error("HasMetamagic should not find an unlearned option")
	}
	if HasMetamagic(pqtype.NullRawMessage{}, "quickened") {
		t.Error("HasMetamagic should be false for a character with no features")
	}
}

func TestValidateKnownMetamagic(t *testing.T) {
	feats := metamagicFeatures("quickened", "twinned")

	// All chosen options are learned -> ok.
	if err := validateKnownMetamagic(feats, []string{"quickened", "twinned"}); err != nil {
		t.Errorf("all-learned should pass, got %v", err)
	}

	// An option the character never picked is rejected, and the error names it.
	err := validateKnownMetamagic(feats, []string{"quickened", "subtle"})
	if err == nil {
		t.Fatal("expected rejection of an unlearned metamagic option")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "subtle") {
		t.Errorf("error should name the unlearned option, got %q", err.Error())
	}

	// A character with no metamagic features (e.g. a legacy sorcerer never
	// rebuilt in the picker) cannot use any option.
	if err := validateKnownMetamagic(pqtype.NullRawMessage{}, []string{"quickened"}); err == nil {
		t.Error("expected rejection when the character has learned no metamagic")
	}

	// No metamagic requested -> nothing to gate.
	if err := validateKnownMetamagic(pqtype.NullRawMessage{}, nil); err != nil {
		t.Errorf("empty request should pass, got %v", err)
	}
}

// TestMetamagicCatalog_AllWiredInCombat closes the loop the refdata-side
// dead-data guard cannot: every option the builder can persist (the refdata
// catalog) must have a real cast-time consumer here. If a catalog id had no
// sorcery-point cost it would be a pickable-but-uncastable dead option.
func TestMetamagicCatalog_AllWiredInCombat(t *testing.T) {
	for _, m := range refdata.MetamagicCatalog() {
		if _, err := SorceryPointCost(m.ID, 1); err != nil {
			t.Errorf("metamagic %q is in the refdata catalog but has no sorcery-point cost in combat: %v", m.ID, err)
		}
	}
}
