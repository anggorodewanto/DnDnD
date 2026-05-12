package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// TestFormatZoneTriggerResults_Empty verifies the helper returns "" when
// no triggers fired so callers can safely concatenate without producing
// blank combat-log lines.
func TestFormatZoneTriggerResults_Empty(t *testing.T) {
	got := FormatZoneTriggerResults("Aria", nil)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// TestFormatZoneTriggerResults_DamageTrigger verifies a damage-effect
// zone produces a DM prompt line naming the source spell, the trigger
// (e.g. "start_of_turn"), and an explicit "roll damage" cue.
func TestFormatZoneTriggerResults_DamageTrigger(t *testing.T) {
	results := []combat.ZoneTriggerResult{
		{
			SourceSpell: "Spirit Guardians",
			Effect:      "damage",
			Trigger:     "start_of_turn",
		},
	}
	got := FormatZoneTriggerResults("Orc", results)
	if !strings.Contains(got, "Orc") {
		t.Errorf("expected combatant name in output, got %q", got)
	}
	if !strings.Contains(got, "Spirit Guardians") {
		t.Errorf("expected source spell in output, got %q", got)
	}
	if !strings.Contains(got, "start_of_turn") {
		t.Errorf("expected trigger type in output, got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "roll damage") {
		t.Errorf("expected damage cue, got %q", got)
	}
}

// TestFormatZoneTriggerResults_SaveTrigger verifies a save-effect zone
// (e.g. Spirit Guardians entry) produces a "prompt save" cue.
func TestFormatZoneTriggerResults_SaveTrigger(t *testing.T) {
	results := []combat.ZoneTriggerResult{
		{
			SourceSpell: "Spirit Guardians",
			Effect:      "save",
			Trigger:     "enter",
		},
	}
	got := FormatZoneTriggerResults("Goblin", results)
	if !strings.Contains(strings.ToLower(got), "prompt save") {
		t.Errorf("expected 'prompt save' cue, got %q", got)
	}
}

// TestFormatZoneTriggerResults_MultipleZones verifies two simultaneous
// trigger results render one line each.
func TestFormatZoneTriggerResults_MultipleZones(t *testing.T) {
	results := []combat.ZoneTriggerResult{
		{SourceSpell: "Spirit Guardians", Effect: "damage", Trigger: "start_of_turn"},
		{SourceSpell: "Wall of Fire", Effect: "damage", Trigger: "start_of_turn"},
	}
	got := FormatZoneTriggerResults("Orc", results)
	if !strings.Contains(got, "Spirit Guardians") || !strings.Contains(got, "Wall of Fire") {
		t.Errorf("expected both source spells, got %q", got)
	}
	lines := strings.Split(got, "\n")
	// 1 header + 2 trigger lines
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d (output: %q)", len(lines), got)
	}
}

// TestPostZoneTriggerResultsToCombatLog_EmptyResults verifies the post
// helper short-circuits silently when the trigger slice is empty.
func TestPostZoneTriggerResultsToCombatLog_EmptyResults(t *testing.T) {
	sess := &mockMoveSession{}
	got := PostZoneTriggerResultsToCombatLog(
		context.Background(), sess, nil, uuid.New(), refdata.Combatant{DisplayName: "Aria"}, nil,
	)
	if got != "" {
		t.Errorf("expected empty post for no results, got %q", got)
	}
	if len(sess.channelSends) != 0 {
		t.Errorf("expected no channel sends, got %d", len(sess.channelSends))
	}
}

// TestPostZoneTriggerResultsToCombatLog_NoCSP verifies the post helper
// short-circuits when no CampaignSettingsProvider is wired (cannot resolve
// the #combat-log channel).
func TestPostZoneTriggerResultsToCombatLog_NoCSP(t *testing.T) {
	sess := &mockMoveSession{}
	got := PostZoneTriggerResultsToCombatLog(
		context.Background(), sess, nil, uuid.New(),
		refdata.Combatant{DisplayName: "Aria"},
		[]combat.ZoneTriggerResult{{SourceSpell: "Spirit Guardians", Effect: "damage"}},
	)
	if got != "" {
		t.Errorf("expected silent no-op when csp is nil, got %q", got)
	}
}

// stubCombatLogCSP routes channel ID lookups for combat_log tests.
type stubCombatLogCSP struct {
	channelIDs map[string]string
	err        error
}

func (s *stubCombatLogCSP) GetChannelIDs(_ context.Context, _ uuid.UUID) (map[string]string, error) {
	return s.channelIDs, s.err
}

// TestPostZoneTriggerResultsToCombatLog_Posts verifies the trigger
// summary lands on #combat-log when both CSP and results are present.
func TestPostZoneTriggerResultsToCombatLog_Posts(t *testing.T) {
	sess := &mockMoveSession{}
	csp := &stubCombatLogCSP{
		channelIDs: map[string]string{"combat-log": "ch-cl"},
	}
	got := PostZoneTriggerResultsToCombatLog(
		context.Background(), sess, csp, uuid.New(),
		refdata.Combatant{DisplayName: "Orc"},
		[]combat.ZoneTriggerResult{{
			SourceSpell: "Spirit Guardians", Effect: "damage", Trigger: "start_of_turn",
		}},
	)
	if got != "ch-cl" {
		t.Errorf("expected combat-log channel posted, got %q", got)
	}
	if len(sess.channelSends) != 1 {
		t.Fatalf("expected 1 channel send, got %d", len(sess.channelSends))
	}
	if !strings.Contains(sess.channelSends[0].Content, "Spirit Guardians") {
		t.Errorf("expected zone label in posted content, got %q", sess.channelSends[0].Content)
	}
}
