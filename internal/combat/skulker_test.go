package combat

import (
	"encoding/json"
	"testing"

	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
)

// --- Bug 2: the Skulker feat was seeded but inert ---
//
// The root cause is not Skulker-specific. hasFeatureEffect compared the feature's
// `mechanical_effect` to the wanted id with a flat EqualFold, but seeded feats
// store a JSON-ENCODED ARRAY there, exactly as it appears on Windreth's live row:
//
//	{"name":"Skulker","source":"feat",
//	 "mechanical_effect":"[{\"effect_type\":\"blindsight_10\"},…]"}
//
// So every lookup against an array-shaped feature returned false: Skulker's
// missed_attack_hidden_no_reveal, and equally the feat-id gates for
// great-weapon-master / sharpshooter / crossbow-expert / tavern-brawler, whose
// ids match neither the blob nor the display name.

func featuresJSON(t *testing.T, raw string) pqtype.NullRawMessage {
	t.Helper()
	return pqtype.NullRawMessage{RawMessage: json.RawMessage(raw), Valid: true}
}

// windrethSkulkerFeatures mirrors the live characters row byte-for-byte.
func windrethSkulkerFeatures(t *testing.T) pqtype.NullRawMessage {
	t.Helper()
	return featuresJSON(t, `[{
		"name": "Skulker",
		"level": 0,
		"source": "feat",
		"description": "Feat: Skulker",
		"mechanical_effect": "[{\"effect_type\":\"blindsight_10\"},{\"effect_type\":\"hide_advantage_in_combat\"},{\"effect_type\":\"missed_attack_hidden_no_reveal\"}]"
	}]`)
}

func TestHasFeatureEffect_ReadsNestedEffectTypeArray(t *testing.T) {
	feats := windrethSkulkerFeatures(t)

	assert.True(t, hasFeatureEffect(feats, "missed_attack_hidden_no_reveal"))
	assert.True(t, hasFeatureEffect(feats, "blindsight_10"))
	assert.True(t, hasFeatureEffect(feats, "hide_advantage_in_combat"))
	assert.False(t, hasFeatureEffect(feats, "uncanny_dodge"))
}

func TestHasFeatureEffect_MatchesFeatIDFromName(t *testing.T) {
	// Forge's live row: name "Great Weapon Master", id-style gate "great-weapon-master".
	feats := featuresJSON(t, `[{
		"name": "Great Weapon Master",
		"source": "feat",
		"mechanical_effect": "[{\"effect_type\":\"bonus_action_attack_on_crit_or_kill\"}]"
	}]`)

	assert.True(t, HasFeat(feats, "great-weapon-master"))
	assert.True(t, hasFeatureEffect(feats, "bonus_action_attack_on_crit_or_kill"))
	assert.False(t, HasFeat(feats, "sharpshooter"))
}

// The pre-existing flat-string shape must keep working.
func TestHasFeatureEffect_FlatStringStillMatches(t *testing.T) {
	feats := featuresJSON(t, `[{"name":"Fighting Style: Two-Weapon","mechanical_effect":"two_weapon_fighting"}]`)

	assert.True(t, HasFightingStyle(feats, "two_weapon_fighting"))
	assert.False(t, HasFightingStyle(feats, "dueling"))
}

func TestHasFeatureEffect_EmptyAndMalformed(t *testing.T) {
	assert.False(t, hasFeatureEffect(pqtype.NullRawMessage{}, "anything"))
	assert.False(t, hasFeatureEffect(featuresJSON(t, `[]`), "anything"))
	assert.False(t, hasFeatureEffect(featuresJSON(t, `[{"name":"X","mechanical_effect":"[not json"}]`), "anything"))
}

// --- the reveal gate itself ---

func TestKeepsHiddenOnMiss(t *testing.T) {
	skulker := windrethSkulkerFeatures(t)
	plain := featuresJSON(t, `[{"name":"Alert","mechanical_effect":"initiative_bonus_5"}]`)

	tests := []struct {
		name     string
		features pqtype.NullRawMessage
		hit      bool
		want     bool
	}{
		{"skulker miss keeps hidden", skulker, false, true},
		{"skulker hit reveals", skulker, true, false},
		{"no feat miss reveals", plain, false, false},
		{"no feat hit reveals", plain, true, false},
		{"no features at all reveals", pqtype.NullRawMessage{}, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, keepsHiddenOnMiss(tt.features, tt.hit))
		})
	}
}
