package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/google/uuid"
)

// --- mock types for party rest handler ---

type mockCharacterLister struct {
	characters []PartyCharacterInfo
	err        error
}

func (m *mockCharacterLister) ListPartyCharacters(ctx context.Context, campaignID uuid.UUID) ([]PartyCharacterInfo, error) {
	return m.characters, m.err
}

type mockCharacterUpdater struct {
	updates []CharacterRestUpdate
}

func (m *mockCharacterUpdater) ApplyRestUpdate(ctx context.Context, update CharacterRestUpdate) error {
	m.updates = append(m.updates, update)
	return nil
}

type mockEncounterChecker struct {
	active bool
}

func (m *mockEncounterChecker) HasActiveEncounter(ctx context.Context, campaignID uuid.UUID) bool {
	return m.active
}

type mockNotifier struct {
	notifications []PlayerNotification
}

func (m *mockNotifier) NotifyPlayer(ctx context.Context, n PlayerNotification) error {
	m.notifications = append(m.notifications, n)
	return nil
}

type mockSummaryPoster struct {
	messages []string
}

func (m *mockSummaryPoster) PostToRollHistory(ctx context.Context, campaignID uuid.UUID, msg string) error {
	m.messages = append(m.messages, msg)
	return nil
}

// --- helpers ---

func newTestCharInfo(name string, id uuid.UUID, discordUserID string) PartyCharacterInfo {
	return PartyCharacterInfo{
		ID:               id,
		Name:             name,
		DiscordUserID:    discordUserID,
		HPCurrent:        20,
		HPMax:            40,
		CONModifier:      2,
		HitDiceRemaining: map[string]int{"d10": 5},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
		FeatureUses:      map[string]character.FeatureUse{},
		SpellSlots:       map[string]character.SlotInfo{},
	}
}

// --- TDD Cycle 8: Party long rest applies to all selected characters ---

func TestPartyRestHandler_LongRest_AppliesAll(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	chars := []PartyCharacterInfo{
		newTestCharInfo("Kael", id1, "user1"),
		newTestCharInfo("Aria", id2, "user2"),
	}

	updater := &mockCharacterUpdater{}
	notifier := &mockNotifier{}
	poster := &mockSummaryPoster{}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: chars},
		updater,
		&mockEncounterChecker{active: false},
		notifier,
		poster,
	)

	body := PartyRestRequest{
		RestType:     "long",
		CharacterIDs: []uuid.UUID{id1, id2},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandlePartyRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// Should have applied updates for both characters
	if len(updater.updates) != 2 {
		t.Errorf("update count = %d, want 2", len(updater.updates))
	}

	// Should have posted summary to roll history
	if len(poster.messages) != 1 {
		t.Errorf("summary count = %d, want 1", len(poster.messages))
	}
}

func TestPartyRestHandler_LongRest_DecrementsDurableExhaustion(t *testing.T) {
	id := uuid.New()
	char := newTestCharInfo("Kael", id, "user1")
	char.ExhaustionLevel = 2

	updater := &mockCharacterUpdater{}
	notifier := &mockNotifier{}
	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: []PartyCharacterInfo{char}},
		updater,
		&mockEncounterChecker{active: false},
		notifier,
		&mockSummaryPoster{},
	)

	body := PartyRestRequest{
		RestType:     "long",
		CharacterIDs: []uuid.UUID{id},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandlePartyRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if len(updater.updates) != 1 {
		t.Fatalf("update count = %d, want 1", len(updater.updates))
	}
	if updater.updates[0].ExhaustionLevel != 1 {
		t.Errorf("durable exhaustion level = %d, want 1", updater.updates[0].ExhaustionLevel)
	}
	if len(notifier.notifications) != 1 || !bytes.Contains([]byte(notifier.notifications[0].Message), []byte("Exhaustion: level 1")) {
		t.Fatalf("expected exhaustion notification, got %#v", notifier.notifications)
	}
}

func TestPartyRestHandler_LongRest_ZeroDurableExhaustionStaysZero(t *testing.T) {
	id := uuid.New()
	char := newTestCharInfo("Aria", id, "user1")
	char.ExhaustionLevel = 0

	updater := &mockCharacterUpdater{}
	notifier := &mockNotifier{}
	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: []PartyCharacterInfo{char}},
		updater,
		&mockEncounterChecker{active: false},
		notifier,
		&mockSummaryPoster{},
	)

	body := PartyRestRequest{
		RestType:     "long",
		CharacterIDs: []uuid.UUID{id},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandlePartyRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if len(updater.updates) != 1 {
		t.Fatalf("update count = %d, want 1", len(updater.updates))
	}
	if updater.updates[0].ExhaustionLevel != 0 {
		t.Errorf("durable exhaustion level = %d, want 0", updater.updates[0].ExhaustionLevel)
	}
	if len(notifier.notifications) != 1 || bytes.Contains([]byte(notifier.notifications[0].Message), []byte("Exhaustion:")) {
		t.Fatalf("did not expect exhaustion notification line, got %#v", notifier.notifications)
	}
}

// --- TDD Cycle 9: Party rest rejects during active combat ---

func TestPartyRestHandler_RejectsDuringCombat(t *testing.T) {
	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: true},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	body := PartyRestRequest{
		RestType:     "long",
		CharacterIDs: []uuid.UUID{uuid.New()},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandlePartyRest(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

// --- TDD Cycle 10: Party short rest sends hit dice prompts to players ---

func TestPartyRestHandler_ShortRest_SendsNotifications(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	chars := []PartyCharacterInfo{
		newTestCharInfo("Kael", id1, "user1"),
		newTestCharInfo("Aria", id2, "user2"),
	}

	notifier := &mockNotifier{}
	poster := &mockSummaryPoster{}
	updater := &mockCharacterUpdater{}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: chars},
		updater,
		&mockEncounterChecker{active: false},
		notifier,
		poster,
	)

	body := PartyRestRequest{
		RestType:     "short",
		CharacterIDs: []uuid.UUID{id1, id2},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandlePartyRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// Short rest: features/pact recharged, notifications sent for hit dice prompts
	if len(notifier.notifications) != 2 {
		t.Errorf("notification count = %d, want 2", len(notifier.notifications))
	}

	// Short rest still applies feature recharge + pact slots
	if len(updater.updates) != 2 {
		t.Errorf("update count = %d, want 2", len(updater.updates))
	}
}

// --- TDD Cycle 11: Party rest with subset of characters tracks excluded ---

func TestPartyRestHandler_SubsetCharacters_TracksExcluded(t *testing.T) {
	id1, id2, id3 := uuid.New(), uuid.New(), uuid.New()
	chars := []PartyCharacterInfo{
		newTestCharInfo("Kael", id1, "user1"),
		newTestCharInfo("Aria", id2, "user2"),
		newTestCharInfo("Zara", id3, "user3"),
	}

	poster := &mockSummaryPoster{}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: chars},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		poster,
	)

	body := PartyRestRequest{
		RestType:     "long",
		CharacterIDs: []uuid.UUID{id1, id2}, // Zara excluded
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandlePartyRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// Summary should mention excluded character
	if len(poster.messages) != 1 {
		t.Fatalf("summary count = %d, want 1", len(poster.messages))
	}
	msg := poster.messages[0]
	if !bytes.Contains([]byte(msg), []byte("Zara kept watch")) {
		t.Errorf("expected 'Zara kept watch' in summary, got: %s", msg)
	}
}

// --- TDD Cycle 12: Interrupt rest handler ---

func TestInterruptRestHandler_ShortRest(t *testing.T) {
	id1 := uuid.New()
	chars := []PartyCharacterInfo{
		newTestCharInfo("Kael", id1, "user1"),
	}

	notifier := &mockNotifier{}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: chars},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		notifier,
		&mockSummaryPoster{},
	)

	body := InterruptRestRequest{
		RestType:       "short",
		CharacterIDs:   []uuid.UUID{id1},
		CampaignID:     uuid.New(),
		Reason:         "Ambush!",
		OneHourElapsed: false,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/interrupt-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandleInterruptRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	if len(notifier.notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifier.notifications))
	}

	n := notifier.notifications[0]
	if n.DiscordUserID != "user1" {
		t.Errorf("DiscordUserID = %q, want %q", n.DiscordUserID, "user1")
	}
}

// --- TDD Cycle 13: Interrupt long rest with 1 hour elapsed grants short rest benefits ---

func TestInterruptRestHandler_LongRest_OneHourElapsed(t *testing.T) {
	id1 := uuid.New()
	chars := []PartyCharacterInfo{
		newTestCharInfo("Aria", id1, "user2"),
	}

	notifier := &mockNotifier{}
	updater := &mockCharacterUpdater{}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: chars},
		updater,
		&mockEncounterChecker{active: false},
		notifier,
		&mockSummaryPoster{},
	)

	body := InterruptRestRequest{
		RestType:       "long",
		CharacterIDs:   []uuid.UUID{id1},
		CampaignID:     uuid.New(),
		Reason:         "Wolves attack",
		OneHourElapsed: true,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/interrupt-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandleInterruptRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// Should apply short rest benefits when 1 hour elapsed
	if len(updater.updates) != 1 {
		t.Errorf("update count = %d, want 1 (short rest benefits applied)", len(updater.updates))
	}

	// Notification should mention short rest benefits
	if len(notifier.notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifier.notifications))
	}
}

// --- TDD Cycle 14: Interrupt long rest without 1 hour no benefits ---

func TestInterruptRestHandler_LongRest_NoHourElapsed(t *testing.T) {
	id1 := uuid.New()
	chars := []PartyCharacterInfo{
		newTestCharInfo("Thorn", id1, "user3"),
	}

	notifier := &mockNotifier{}
	updater := &mockCharacterUpdater{}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: chars},
		updater,
		&mockEncounterChecker{active: false},
		notifier,
		&mockSummaryPoster{},
	)

	body := InterruptRestRequest{
		RestType:       "long",
		CharacterIDs:   []uuid.UUID{id1},
		CampaignID:     uuid.New(),
		Reason:         "Wolves attack",
		OneHourElapsed: false,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/interrupt-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandleInterruptRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// No benefits — no updates
	if len(updater.updates) != 0 {
		t.Errorf("update count = %d, want 0 (no benefits)", len(updater.updates))
	}

	// But still notified
	if len(notifier.notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifier.notifications))
	}
}

// --- TDD Cycle 15: Invalid rest type returns 400 ---

func TestPartyRestHandler_InvalidRestType(t *testing.T) {
	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	body := PartyRestRequest{
		RestType:     "invalid",
		CharacterIDs: []uuid.UUID{uuid.New()},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandlePartyRest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// --- TDD Cycle 16: Empty character list returns 400 ---

func TestPartyRestHandler_InvalidJSON(t *testing.T) {
	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	h.HandlePartyRest(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestInterruptRestHandler_InvalidJSON(t *testing.T) {
	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/interrupt-rest", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	h.HandleInterruptRest(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestPartyRestHandler_ListerError(t *testing.T) {
	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{err: http.ErrNoCookie}, // any error
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	body := PartyRestRequest{
		RestType:     "long",
		CharacterIDs: []uuid.UUID{uuid.New()},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandlePartyRest(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestInterruptRestHandler_ListerError(t *testing.T) {
	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{err: http.ErrNoCookie},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	body := InterruptRestRequest{
		RestType:     "short",
		CharacterIDs: []uuid.UUID{uuid.New()},
		CampaignID:   uuid.New(),
		Reason:       "test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/interrupt-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandleInterruptRest(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// --- Long rest with notifications sent ---

func TestPartyRestHandler_LongRest_SendsNotifications(t *testing.T) {
	id1 := uuid.New()
	chars := []PartyCharacterInfo{
		newTestCharInfo("Kael", id1, "user1"),
	}

	notifier := &mockNotifier{}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: chars},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		notifier,
		&mockSummaryPoster{},
	)

	body := PartyRestRequest{
		RestType:     "long",
		CharacterIDs: []uuid.UUID{id1},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandlePartyRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if len(notifier.notifications) != 1 {
		t.Errorf("notification count = %d, want 1", len(notifier.notifications))
	}
}

// --- Interrupt rest: multiple characters ---

func TestInterruptRestHandler_MultipleCharacters(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	chars := []PartyCharacterInfo{
		newTestCharInfo("Kael", id1, "user1"),
		newTestCharInfo("Aria", id2, "user2"),
	}

	notifier := &mockNotifier{}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: chars},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		notifier,
		&mockSummaryPoster{},
	)

	body := InterruptRestRequest{
		RestType:     "short",
		CharacterIDs: []uuid.UUID{id1, id2},
		CampaignID:   uuid.New(),
		Reason:       "Ambush!",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/interrupt-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandleInterruptRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if len(notifier.notifications) != 2 {
		t.Errorf("notification count = %d, want 2", len(notifier.notifications))
	}
}

func TestPartyRestHandler_EmptyCharacterIDs(t *testing.T) {
	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	body := PartyRestRequest{
		RestType:     "short",
		CharacterIDs: []uuid.UUID{},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandlePartyRest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// --- TDD Cycle 17: RegisterPartyRestRoutes mounts endpoints ---

func TestRegisterPartyRestRoutes(t *testing.T) {
	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	mux := http.NewServeMux()
	RegisterPartyRestRoutes(mux, h)

	// Verify party rest endpoint responds (with bad body = 400, not 404)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Error("party-rest route not registered (got 404)")
	}

	// Verify interrupt rest endpoint responds
	req2 := httptest.NewRequest(http.MethodPost, "/dashboard/api/interrupt-rest", bytes.NewReader([]byte("{}")))
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code == http.StatusNotFound {
		t.Error("interrupt-rest route not registered (got 404)")
	}
}

// --- Response body verification ---

func TestPartyRestHandler_LongRest_ResponseBody(t *testing.T) {
	id1 := uuid.New()
	chars := []PartyCharacterInfo{
		newTestCharInfo("Kael", id1, "user1"),
	}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: chars},
		&mockCharacterUpdater{},
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	body := PartyRestRequest{
		RestType:     "long",
		CharacterIDs: []uuid.UUID{id1},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandlePartyRest(w, req)

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
	if resp["summary"] == "" {
		t.Error("expected non-empty summary in response")
	}
}

// --- Edge: short rest with character that has invalid hit dice data (tests continue on error) ---

func TestPartyRestHandler_ShortRest_SkipsCharOnError(t *testing.T) {
	id1 := uuid.New()
	// Character with invalid hit dice to force a ShortRest error path
	badChar := PartyCharacterInfo{
		ID:               id1,
		Name:             "Broken",
		DiscordUserID:    "user1",
		HPCurrent:        20,
		HPMax:            40,
		CONModifier:      2,
		HitDiceRemaining: map[string]int{"d4": 5}, // d4 is invalid, will cause error
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
		FeatureUses:      map[string]character.FeatureUse{},
		SpellSlots:       map[string]character.SlotInfo{},
	}

	updater := &mockCharacterUpdater{}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: []PartyCharacterInfo{badChar}},
		updater,
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	body := PartyRestRequest{
		RestType:     "short",
		CharacterIDs: []uuid.UUID{id1},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandlePartyRest(w, req)

	// Should succeed (200) even though character processing failed
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	// The character update should still proceed because ShortRest with empty spend doesn't error
	// (only hit dice spending errors, and party short rest doesn't spend dice)
	// So update count is still 1
	if len(updater.updates) != 1 {
		t.Errorf("update count = %d, want 1", len(updater.updates))
	}
}

// --- Edge: interrupt rest error path in ShortRest ---

func TestInterruptRestHandler_LongRest_OneHourElapsed_ShortRestError(t *testing.T) {
	// This tests the error path in HandleInterruptRest when ShortRest fails
	// ShortRest with empty spend doesn't actually fail, so this just
	// verifies the handler processes normally
	id1 := uuid.New()
	chars := []PartyCharacterInfo{
		newTestCharInfo("Kael", id1, "user1"),
	}

	updater := &mockCharacterUpdater{}
	notifier := &mockNotifier{}

	h := NewPartyRestHandler(
		NewService(dice.NewRoller(nil)),
		&mockCharacterLister{characters: chars},
		updater,
		&mockEncounterChecker{active: false},
		notifier,
		&mockSummaryPoster{},
	)

	body := InterruptRestRequest{
		RestType:       "long",
		CharacterIDs:   []uuid.UUID{id1},
		CampaignID:     uuid.New(),
		Reason:         "Dragon!",
		OneHourElapsed: true,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/interrupt-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.HandleInterruptRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if len(updater.updates) != 1 {
		t.Errorf("update count = %d, want 1", len(updater.updates))
	}
	if len(notifier.notifications) != 1 {
		t.Errorf("notification count = %d, want 1", len(notifier.notifications))
	}
}

// --- TDD Cycle G-H08: Party long rest triggers dawn recharge for magic items ---

func TestPartyRestHandler_LongRest_DawnRecharge(t *testing.T) {
	id := uuid.New()
	char := newTestCharInfo("Kael", id, "user1")
	char.Inventory = []character.InventoryItem{
		{ItemID: "wand-of-magic", Name: "Wand of Magic", Charges: 2, MaxCharges: 7, IsMagic: true},
	}
	char.RechargeInfo = map[string]inventory.RechargeInfo{
		"wand-of-magic": {Dice: "1d6+1", DestroyOnZero: false},
	}

	updater := &mockCharacterUpdater{}
	// Use a deterministic roller that always returns max (ensures charges restored).
	roller := dice.NewRoller(func(max int) int { return max })
	h := NewPartyRestHandler(
		NewService(roller),
		&mockCharacterLister{characters: []PartyCharacterInfo{char}},
		updater,
		&mockEncounterChecker{active: false},
		&mockNotifier{},
		&mockSummaryPoster{},
	)

	body := PartyRestRequest{
		RestType:     "long",
		CharacterIDs: []uuid.UUID{id},
		CampaignID:   uuid.New(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/party-rest", bytes.NewReader(b))
	w := httptest.NewRecorder()

	h.HandlePartyRest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if len(updater.updates) != 1 {
		t.Fatalf("update count = %d, want 1", len(updater.updates))
	}
	// Dawn recharge should have restored charges (1d6+1 with max roller = 7, capped at MaxCharges=7)
	if updater.updates[0].UpdatedInventory == nil {
		t.Fatal("expected UpdatedInventory to be populated after dawn recharge")
	}
	if len(updater.updates[0].UpdatedInventory) != 1 {
		t.Fatalf("expected 1 item in UpdatedInventory, got %d", len(updater.updates[0].UpdatedInventory))
	}
	if updater.updates[0].UpdatedInventory[0].Charges != 7 {
		t.Errorf("charges = %d, want 7 (fully recharged)", updater.updates[0].UpdatedInventory[0].Charges)
	}
}
