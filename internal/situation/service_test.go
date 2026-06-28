package situation

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeProvider is a configurable Provider for unit-testing the Service. Each
// slice/pointer is returned verbatim; each *Err field forces that source to
// fail so the best-effort / errors.Join behaviour can be exercised.
type fakeProvider struct {
	queue      []QueueRow
	approvals  []ApprovalRow
	levelUps   []LevelUpRow
	encounter  *EncounterRow
	actions    []TimelineRow
	narration  []TimelineRow
	resolution []TimelineRow

	queueErr      error
	approvalsErr  error
	levelUpsErr   error
	encounterErr  error
	actionsErr    error
	narrationErr  error
	resolutionErr error

	gotEncounterID string
}

func (f *fakeProvider) QueueItems(_ context.Context, _ string) ([]QueueRow, error) {
	return f.queue, f.queueErr
}
func (f *fakeProvider) Approvals(_ context.Context, _ string) ([]ApprovalRow, error) {
	return f.approvals, f.approvalsErr
}
func (f *fakeProvider) LevelUps(_ context.Context, _ string) ([]LevelUpRow, error) {
	return f.levelUps, f.levelUpsErr
}
func (f *fakeProvider) Encounter(_ context.Context, _ string) (*EncounterRow, error) {
	return f.encounter, f.encounterErr
}
func (f *fakeProvider) ActionEvents(_ context.Context, encounterID string) ([]TimelineRow, error) {
	f.gotEncounterID = encounterID
	return f.actions, f.actionsErr
}
func (f *fakeProvider) NarrationEvents(_ context.Context, _ string) ([]TimelineRow, error) {
	return f.narration, f.narrationErr
}
func (f *fakeProvider) ResolutionEvents(_ context.Context, _ string) ([]TimelineRow, error) {
	return f.resolution, f.resolutionErr
}

func ts(min int) time.Time {
	return time.Date(2026, 6, 25, 8, min, 0, 0, time.UTC)
}

func TestBuild_EmptyCampaign(t *testing.T) {
	s := NewService(&fakeProvider{})
	got, err := s.Build(context.Background(), "camp-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Pending) != 0 {
		t.Errorf("Pending = %d, want 0", len(got.Pending))
	}
	if got.State.HasEncounter {
		t.Errorf("HasEncounter = true, want false")
	}
	if len(got.Timeline) != 0 {
		t.Errorf("Timeline = %d, want 0", len(got.Timeline))
	}
	if got.NextStep != "" {
		t.Errorf("NextStep = %q, want empty", got.NextStep)
	}
}

func TestBuild_UnifiesAndPrioritizesPending(t *testing.T) {
	f := &fakeProvider{
		queue: []QueueRow{
			{ID: "q-rest", Kind: "rest_request", Player: "Vale", Summary: "short rest", CreatedAt: ts(1)},
			{ID: "q-enemy", Kind: "enemy_turn_ready", Player: "Ghoul", Summary: "NPC turn", CreatedAt: ts(5)},
			{ID: "q-whisper", Kind: "player_whisper", Player: "Forge", Summary: "hit 15", CreatedAt: ts(3)},
		},
		approvals: []ApprovalRow{{ID: "a-1", Name: "Mira", Race: "Elf", Level: 1, CreatedAt: ts(2)}},
		levelUps:  []LevelUpRow{{ID: "l-1", Name: "Forge", CreatedAt: ts(4)}},
	}
	got, err := NewService(f).Build(context.Background(), "camp-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Pending) != 5 {
		t.Fatalf("Pending = %d, want 5", len(got.Pending))
	}
	// enemy_turn_ready is combat-blocking → must sort first.
	if got.Pending[0].ID != "q-enemy" {
		t.Errorf("Pending[0] = %q, want q-enemy (combat-blocking first)", got.Pending[0].ID)
	}
	// approval + level-up are the least urgent → must sort last.
	last := got.Pending[len(got.Pending)-1]
	if last.Source != SourceApproval && last.Source != SourceLevelUp {
		t.Errorf("least-urgent item Source = %q, want approval/levelup", last.Source)
	}
	// Sources are tagged.
	var sawApproval, sawLevelUp bool
	for _, p := range got.Pending {
		if p.Source == SourceApproval {
			sawApproval = true
			if p.Label == "" {
				t.Errorf("approval item has empty Label")
			}
		}
		if p.Source == SourceLevelUp {
			sawLevelUp = true
		}
	}
	if !sawApproval || !sawLevelUp {
		t.Errorf("missing unified sources: approval=%v levelup=%v", sawApproval, sawLevelUp)
	}
}

func TestBuild_PriorityTieBreaksByCreatedAt(t *testing.T) {
	f := &fakeProvider{
		queue: []QueueRow{
			{ID: "newer", Kind: "freeform_action", Player: "A", CreatedAt: ts(9)},
			{ID: "older", Kind: "freeform_action", Player: "B", CreatedAt: ts(1)},
		},
	}
	got, _ := NewService(f).Build(context.Background(), "camp-1")
	if got.Pending[0].ID != "older" {
		t.Errorf("Pending[0] = %q, want older (same priority → oldest first)", got.Pending[0].ID)
	}
}

func TestBuild_StateMarksCurrentTurnAndPosition(t *testing.T) {
	f := &fakeProvider{
		encounter: &EncounterRow{
			ID: "enc-1", Name: "Crypt", Mode: "combat", Status: "active", Round: 1,
			CurrentTurnID: "cb-forge",
			Combatants: []CombatantRow{
				{ID: "cb-forge", Name: "Forge", ShortID: "FO", HPCurrent: 32, HPMax: 32, AC: 14, Col: "E", Row: 7, IsNPC: false, IsAlive: true},
				{ID: "cb-ghoul", Name: "Ghoul", ShortID: "G1", HPCurrent: 22, HPMax: 22, AC: 12, Col: "C", Row: 7, IsNPC: true, IsAlive: true, Conditions: []ConditionInfo{{Name: "prone"}}},
			},
		},
	}
	got, err := NewService(f).Build(context.Background(), "camp-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.State.HasEncounter {
		t.Fatal("HasEncounter = false, want true")
	}
	if got.State.Round != 1 || got.State.Mode != "combat" {
		t.Errorf("State = round %d mode %q, want round 1 mode combat", got.State.Round, got.State.Mode)
	}
	if got.State.CurrentTurn != "Forge" {
		t.Errorf("CurrentTurn = %q, want Forge", got.State.CurrentTurn)
	}
	var forge, ghoul CombatantView
	for _, c := range got.State.Combatants {
		switch c.ShortID {
		case "FO":
			forge = c
		case "G1":
			ghoul = c
		}
	}
	if !forge.IsCurrent {
		t.Errorf("Forge IsCurrent = false, want true")
	}
	if ghoul.IsCurrent {
		t.Errorf("Ghoul IsCurrent = true, want false")
	}
	if forge.Position != "E7" {
		t.Errorf("Forge Position = %q, want E7", forge.Position)
	}
	if len(ghoul.Conditions) != 1 || ghoul.Conditions[0].Name != "prone" {
		t.Errorf("Ghoul Conditions = %v, want [prone]", ghoul.Conditions)
	}
}

// Tier 1 DM-Console fields: a DM must read concentration, temp HP, exhaustion,
// death saves, rage, initiative order, and condition *metadata* (duration /
// source / expiry) off the Console instead of tracking them by hand. buildState
// must surface all of them and return combatants in initiative (turn) order.
func TestBuild_StateExposesTier1Fields(t *testing.T) {
	f := &fakeProvider{
		encounter: &EncounterRow{
			ID: "enc-1", Name: "Cellar", Mode: "combat", Status: "active", Round: 3,
			CurrentTurnID: "cb-forge",
			Combatants: []CombatantRow{
				// Deliberately NOT in initiative order, to prove buildState sorts.
				{ID: "cb-ghoul9", Name: "Ghoul", ShortID: "G1", InitiativeOrder: 4, Initiative: 9, HPCurrent: 22, HPMax: 22, AC: 12, IsNPC: true, IsAlive: true},
				{ID: "cb-ghoul19", Name: "Ghoul", ShortID: "G2", InitiativeOrder: 1, Initiative: 19, HPCurrent: 13, HPMax: 22, AC: 12, IsNPC: true, IsAlive: true,
					Conditions: []ConditionInfo{{Name: "poisoned", DurationRounds: 3, SourceSpell: "ray of sickness", SourceID: "cb-vale", ExpiresOn: "end_of_turn"}}},
				{ID: "cb-vale", Name: "Vale", ShortID: "VA", InitiativeOrder: 2, Initiative: 15, HPCurrent: 19, HPMax: 24, TempHP: 5, AC: 11, Concentration: "Hold Person"},
				{ID: "cb-forge", Name: "Forge", ShortID: "FO", InitiativeOrder: 3, Initiative: 12, HPCurrent: 0, HPMax: 32, AC: 14, Exhaustion: 1, IsRaging: true, RageRoundsRemaining: 9, IsAlive: true,
					DeathSaves: &DeathSaves{Successes: 1, Failures: 2}},
			},
		},
	}
	got, err := NewService(f).Build(context.Background(), "camp-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Combatants come back in initiative (turn) order, not input order.
	var order []string
	by := map[string]CombatantView{}
	for _, c := range got.State.Combatants {
		order = append(order, c.ShortID)
		by[c.ShortID] = c
	}
	if want := []string{"G2", "VA", "FO", "G1"}; !equalStrings(order, want) {
		t.Errorf("turn order = %v, want %v (sorted by initiative_order)", order, want)
	}

	if by["VA"].TempHP != 5 {
		t.Errorf("Vale TempHP = %d, want 5", by["VA"].TempHP)
	}
	if by["VA"].Concentration != "Hold Person" {
		t.Errorf("Vale Concentration = %q, want Hold Person", by["VA"].Concentration)
	}
	if by["VA"].Initiative != 15 {
		t.Errorf("Vale Initiative = %d, want 15", by["VA"].Initiative)
	}
	if by["VA"].DeathSaves != nil {
		t.Errorf("Vale DeathSaves = %+v, want nil (not dying)", by["VA"].DeathSaves)
	}
	if !by["FO"].IsRaging || by["FO"].RageRoundsRemaining != 9 {
		t.Errorf("Forge raging=%v rounds=%d, want true/9", by["FO"].IsRaging, by["FO"].RageRoundsRemaining)
	}
	if by["FO"].Exhaustion != 1 {
		t.Errorf("Forge Exhaustion = %d, want 1", by["FO"].Exhaustion)
	}
	if ds := by["FO"].DeathSaves; ds == nil || ds.Successes != 1 || ds.Failures != 2 {
		t.Errorf("Forge DeathSaves = %+v, want {1,2}", ds)
	}
	if c := by["G2"].Conditions; len(c) != 1 || c[0].Name != "poisoned" || c[0].DurationRounds != 3 ||
		c[0].SourceSpell != "ray of sickness" || c[0].ExpiresOn != "end_of_turn" {
		t.Errorf("G2 condition metadata = %+v, want poisoned/3/ray of sickness/end_of_turn", c)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBuild_TimelineMergedSortedTruncated(t *testing.T) {
	actions := make([]TimelineRow, 0, 25)
	for i := range 25 {
		actions = append(actions, TimelineRow{At: ts(i), Actor: "Forge", Summary: "attack"})
	}
	f := &fakeProvider{
		encounter:  &EncounterRow{ID: "enc-1", Status: "active"},
		actions:    actions,
		narration:  []TimelineRow{{At: ts(59), Actor: "DM", Summary: "scene opens"}},
		resolution: []TimelineRow{{At: ts(58), Actor: "DM", Summary: "resolved throw"}},
	}
	got, err := NewService(f).Build(context.Background(), "camp-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Timeline) != timelineLimit {
		t.Fatalf("Timeline = %d, want %d (truncated)", len(got.Timeline), timelineLimit)
	}
	// Newest first: narration at minute 59 wins.
	if got.Timeline[0].Source != "narration" || got.Timeline[0].Summary != "scene opens" {
		t.Errorf("Timeline[0] = %+v, want narration 'scene opens' first", got.Timeline[0])
	}
	if got.Timeline[1].Source != "resolution" {
		t.Errorf("Timeline[1].Source = %q, want resolution", got.Timeline[1].Source)
	}
	// Descending order overall.
	for i := 1; i < len(got.Timeline); i++ {
		if got.Timeline[i-1].At.Before(got.Timeline[i].At) {
			t.Errorf("timeline not sorted desc at %d", i)
		}
	}
	// ActionEvents must be fetched with the active encounter's ID.
	if f.gotEncounterID != "enc-1" {
		t.Errorf("ActionEvents got encounterID %q, want enc-1", f.gotEncounterID)
	}
}

func TestBuild_NextStep_NPCTurnTakesPriority(t *testing.T) {
	f := &fakeProvider{
		queue: []QueueRow{{ID: "q1", Kind: "player_whisper", Player: "Forge", Summary: "hi", CreatedAt: ts(1)}},
		encounter: &EncounterRow{
			ID: "enc-1", Mode: "combat", Status: "active", Round: 2, CurrentTurnID: "cb-ghoul",
			Combatants: []CombatantRow{
				{ID: "cb-ghoul", Name: "Ghoul", ShortID: "G1", IsNPC: true, IsAlive: true},
			},
		},
	}
	got, _ := NewService(f).Build(context.Background(), "camp-1")
	if got.NextStep == "" {
		t.Fatal("NextStep empty, want NPC-turn prompt")
	}
	if !contains(got.NextStep, "Ghoul") {
		t.Errorf("NextStep = %q, want it to mention the NPC Ghoul", got.NextStep)
	}
}

func TestBuild_NextStep_FallsBackToTopPending(t *testing.T) {
	f := &fakeProvider{
		queue: []QueueRow{{ID: "q1", Kind: "freeform_action", Player: "Forge", Summary: "throw axe", CreatedAt: ts(1)}},
	}
	got, _ := NewService(f).Build(context.Background(), "camp-1")
	if !contains(got.NextStep, "Forge") {
		t.Errorf("NextStep = %q, want it to reference the pending item from Forge", got.NextStep)
	}
}

func TestBuild_JoinsSourceErrorsButStillBuilds(t *testing.T) {
	f := &fakeProvider{
		queue:        []QueueRow{{ID: "q1", Kind: "freeform_action", Player: "Forge", CreatedAt: ts(1)}},
		approvalsErr: errors.New("approvals db down"),
		narrationErr: errors.New("narration db down"),
	}
	got, err := NewService(f).Build(context.Background(), "camp-1")
	if err == nil {
		t.Fatal("expected joined source error, got nil")
	}
	// Partial view still built: the working source survives.
	if len(got.Pending) != 1 {
		t.Errorf("Pending = %d, want 1 (working source survives the error)", len(got.Pending))
	}
}

func TestQueueKindLabelAndPriority_AllKinds(t *testing.T) {
	cases := []struct {
		kind     string
		label    string
		priority int
	}{
		{"enemy_turn_ready", "Enemy Turn Ready", priorityCombatBlocking},
		{"opportunity_attack", "Opportunity Attack", priorityCombatBlocking},
		{"map_render_failure", "Map Render Failed", prioritySystemAlert},
		{"reaction_declaration", "Reaction Declaration", priorityReaction},
		{"freeform_action", "Freeform Action", priorityPlayerAction},
		{"player_whisper", "Player Whisper", priorityPlayerAction},
		{"consumable", "Consumable Usage", priorityPlayerAction},
		{"narrative_teleport", "Narrative Teleport", priorityPlayerAction},
		{"channel_divinity", "Channel Divinity", priorityPlayerAction},
		{"skill_check_narration", "Skill Check Narration", priorityNarration},
		{"rest_request", "Rest Request", priorityHousekeeping},
		{"undo_request", "Undo Request", priorityHousekeeping},
		{"retire_request", "Retire Request", priorityHousekeeping},
		{"something_new", "Notification", priorityDefault},
	}
	for _, c := range cases {
		if got := queueKindLabel(c.kind); got != c.label {
			t.Errorf("queueKindLabel(%q) = %q, want %q", c.kind, got, c.label)
		}
		if got := queueKindPriority(c.kind); got != c.priority {
			t.Errorf("queueKindPriority(%q) = %d, want %d", c.kind, got, c.priority)
		}
	}
}

func TestApprovalSummaryAndPosition_EdgeCases(t *testing.T) {
	if got := approvalSummary(ApprovalRow{}); got != "Character awaiting approval" {
		t.Errorf("approvalSummary(empty) = %q", got)
	}
	if got := approvalSummary(ApprovalRow{Race: "Dwarf"}); got != "Dwarf awaiting approval" {
		t.Errorf("approvalSummary(race-only) = %q", got)
	}
	if got := approvalSummary(ApprovalRow{Level: 3}); got != "level 3 awaiting approval" {
		t.Errorf("approvalSummary(level-only) = %q", got)
	}
	if got := formatPosition("", 0); got != "" {
		t.Errorf("formatPosition(empty) = %q, want empty", got)
	}
	if got := formatPosition("K", 6); got != "K6" {
		t.Errorf("formatPosition(K,6) = %q, want K6", got)
	}
}

func TestBuild_NextStep_PendingWithoutSummary(t *testing.T) {
	f := &fakeProvider{
		queue: []QueueRow{{ID: "q1", Kind: "rest_request", Player: "Vale", Summary: "  ", CreatedAt: ts(1)}},
	}
	got, _ := NewService(f).Build(context.Background(), "camp-1")
	if !contains(got.NextStep, "Vale") || contains(got.NextStep, ":") {
		t.Errorf("NextStep = %q, want a no-summary prompt mentioning Vale without a colon", got.NextStep)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
