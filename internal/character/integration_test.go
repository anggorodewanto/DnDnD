package character_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

func setupTestDB(t *testing.T) (*sql.DB, *refdata.Queries, uuid.UUID) {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	queries := refdata.New(db)
	ctx := context.Background()

	// Create a campaign for FK
	var campaignID uuid.UUID
	err := db.QueryRowContext(ctx,
		`INSERT INTO campaigns (guild_id, dm_user_id, name) VALUES ($1, $2, $3) RETURNING id`,
		"test-guild", "test-dm", "Test Campaign",
	).Scan(&campaignID)
	if err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}
	return db, queries, campaignID
}

func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return b
}

func nullJSON(t *testing.T, v interface{}) pqtype.NullRawMessage {
	t.Helper()
	b := mustJSON(t, v)
	return pqtype.NullRawMessage{RawMessage: b, Valid: true}
}

func TestIntegration_CharacterJSONBRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, queries, campaignID := setupTestDB(t)
	ctx := context.Background()

	// Define complex JSONB data
	classes := []map[string]interface{}{
		{"class": "fighter", "subclass": "champion", "level": 5},
		{"class": "wizard", "subclass": "", "level": 3},
	}
	abilityScores := map[string]int{"str": 16, "dex": 14, "con": 12, "int": 18, "wis": 10, "cha": 8}
	proficiencies := map[string]interface{}{
		"saves":   []string{"str", "con"},
		"skills":  []string{"athletics", "perception", "arcana"},
		"weapons": []string{"simple", "martial"},
		"armor":   []string{"light", "medium", "heavy", "shield"},
	}
	featureUses := map[string]interface{}{
		"action-surge":  map[string]interface{}{"current": 1, "max": 1, "recharge": "short"},
		"second-wind":   map[string]interface{}{"current": 0, "max": 1, "recharge": "short"},
		"arcane-recovery": map[string]interface{}{"current": 1, "max": 1, "recharge": "long"},
	}
	inventory := []map[string]interface{}{
		{"item_id": "longsword", "quantity": 1, "equipped": true, "type": "weapon", "is_magic": false},
		{"item_id": "shield", "quantity": 1, "equipped": true, "type": "armor", "is_magic": false},
		{"item_id": "health-potion", "quantity": 3, "equipped": false, "type": "consumable", "is_magic": true},
	}
	spellSlots := map[string]interface{}{
		"1": map[string]interface{}{"current": 2, "max": 4},
		"2": map[string]interface{}{"current": 3, "max": 3},
		"3": map[string]interface{}{"current": 1, "max": 2},
	}
	hitDiceRemaining := map[string]int{"d10": 5, "d6": 3}
	features := []map[string]interface{}{
		{"name": "Second Wind", "source": "fighter", "level": 1, "description": "Heal 1d10+fighter level"},
		{"name": "Action Surge", "source": "fighter", "level": 2, "description": "Extra action"},
	}
	attunementSlots := []map[string]interface{}{
		{"item_id": "ring-of-protection", "name": "Ring of Protection"},
	}

	params := refdata.CreateCharacterParams{
		CampaignID:       campaignID,
		Name:             "Gandalf the Fighter-Wizard",
		Race:             "human",
		Classes:          mustJSON(t, classes),
		Level:            8,
		AbilityScores:    mustJSON(t, abilityScores),
		HpMax:            67,
		HpCurrent:        55,
		TempHp:           5,
		Ac:               18,
		SpeedFt:          30,
		ProficiencyBonus: 3,
		HitDiceRemaining: mustJSON(t, hitDiceRemaining),
		SpellSlots:       nullJSON(t, spellSlots),
		FeatureUses:      nullJSON(t, featureUses),
		Features:         nullJSON(t, features),
		Proficiencies:    nullJSON(t, proficiencies),
		Gold:             150,
		AttunementSlots:  nullJSON(t, attunementSlots),
		Languages:        []string{"Common", "Elvish", "Draconic"},
		Inventory:        nullJSON(t, inventory),
	}

	created, err := queries.CreateCharacter(ctx, params)
	if err != nil {
		t.Fatalf("CreateCharacter failed: %v", err)
	}

	// Re-read from DB
	got, err := queries.GetCharacter(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCharacter failed: %v", err)
	}

	// Verify scalar fields
	if got.Name != "Gandalf the Fighter-Wizard" {
		t.Errorf("Name = %q, want %q", got.Name, "Gandalf the Fighter-Wizard")
	}
	if got.Level != 8 {
		t.Errorf("Level = %d, want 8", got.Level)
	}
	if got.HpMax != 67 {
		t.Errorf("HpMax = %d, want 67", got.HpMax)
	}
	if got.Gold != 150 {
		t.Errorf("Gold = %d, want 150", got.Gold)
	}
	if got.ProficiencyBonus != 3 {
		t.Errorf("ProficiencyBonus = %d, want 3", got.ProficiencyBonus)
	}

	// Verify JSONB round-trip: classes
	var gotClasses []map[string]interface{}
	if err := json.Unmarshal(got.Classes, &gotClasses); err != nil {
		t.Fatalf("unmarshal classes: %v", err)
	}
	if len(gotClasses) != 2 {
		t.Errorf("classes length = %d, want 2", len(gotClasses))
	}
	if gotClasses[0]["class"] != "fighter" {
		t.Errorf("first class = %v, want fighter", gotClasses[0]["class"])
	}

	// Verify JSONB round-trip: proficiencies
	var gotProf map[string]interface{}
	if err := json.Unmarshal(got.Proficiencies.RawMessage, &gotProf); err != nil {
		t.Fatalf("unmarshal proficiencies: %v", err)
	}
	saves, ok := gotProf["saves"].([]interface{})
	if !ok || len(saves) != 2 {
		t.Errorf("proficiencies.saves = %v, want [str, con]", gotProf["saves"])
	}

	// Verify JSONB round-trip: feature_uses
	var gotFeatureUses map[string]interface{}
	if err := json.Unmarshal(got.FeatureUses.RawMessage, &gotFeatureUses); err != nil {
		t.Fatalf("unmarshal feature_uses: %v", err)
	}
	if len(gotFeatureUses) != 3 {
		t.Errorf("feature_uses count = %d, want 3", len(gotFeatureUses))
	}

	// Verify JSONB round-trip: inventory
	var gotInventory []map[string]interface{}
	if err := json.Unmarshal(got.Inventory.RawMessage, &gotInventory); err != nil {
		t.Fatalf("unmarshal inventory: %v", err)
	}
	if len(gotInventory) != 3 {
		t.Errorf("inventory count = %d, want 3", len(gotInventory))
	}

	// Verify JSONB round-trip: spell_slots
	var gotSlots map[string]interface{}
	if err := json.Unmarshal(got.SpellSlots.RawMessage, &gotSlots); err != nil {
		t.Fatalf("unmarshal spell_slots: %v", err)
	}
	if len(gotSlots) != 3 {
		t.Errorf("spell_slots count = %d, want 3", len(gotSlots))
	}

	// Verify JSONB round-trip: hit_dice_remaining
	var gotHitDice map[string]float64
	if err := json.Unmarshal(got.HitDiceRemaining, &gotHitDice); err != nil {
		t.Fatalf("unmarshal hit_dice_remaining: %v", err)
	}
	if gotHitDice["d10"] != 5 || gotHitDice["d6"] != 3 {
		t.Errorf("hit_dice_remaining = %v, want d10:5 d6:3", gotHitDice)
	}

	// Verify languages (TEXT[] round-trip)
	if len(got.Languages) != 3 {
		t.Errorf("languages count = %d, want 3", len(got.Languages))
	}

	// Verify attunement_slots
	var gotAttune []map[string]interface{}
	if err := json.Unmarshal(got.AttunementSlots.RawMessage, &gotAttune); err != nil {
		t.Fatalf("unmarshal attunement_slots: %v", err)
	}
	if len(gotAttune) != 1 {
		t.Errorf("attunement_slots count = %d, want 1", len(gotAttune))
	}

	// Verify features
	var gotFeatures []map[string]interface{}
	if err := json.Unmarshal(got.Features.RawMessage, &gotFeatures); err != nil {
		t.Fatalf("unmarshal features: %v", err)
	}
	if len(gotFeatures) != 2 {
		t.Errorf("features count = %d, want 2", len(gotFeatures))
	}
}

func TestIntegration_CharacterListByCampaign(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, queries, campaignID := setupTestDB(t)
	ctx := context.Background()

	// Create two characters
	for _, name := range []string{"Zara", "Alice"} {
		_, err := queries.CreateCharacter(ctx, refdata.CreateCharacterParams{
			CampaignID:       campaignID,
			Name:             name,
			Race:             "human",
			Classes:          mustJSON(t, []map[string]interface{}{{"class": "fighter", "level": 1}}),
			Level:            1,
			AbilityScores:    mustJSON(t, map[string]int{"str": 10, "dex": 10, "con": 10, "int": 10, "wis": 10, "cha": 10}),
			HpMax:            10,
			HpCurrent:        10,
			Ac:               10,
			SpeedFt:          30,
			ProficiencyBonus: 2,
			HitDiceRemaining: mustJSON(t, map[string]int{"d10": 1}),
			Languages:        []string{"Common"},
		})
		if err != nil {
			t.Fatalf("CreateCharacter %s failed: %v", name, err)
		}
	}

	chars, err := queries.ListCharactersByCampaign(ctx, campaignID)
	if err != nil {
		t.Fatalf("ListCharactersByCampaign failed: %v", err)
	}
	if len(chars) != 2 {
		t.Fatalf("got %d characters, want 2", len(chars))
	}
	// Should be ordered by name
	if chars[0].Name != "Alice" {
		t.Errorf("first character = %q, want Alice", chars[0].Name)
	}
	if chars[1].Name != "Zara" {
		t.Errorf("second character = %q, want Zara", chars[1].Name)
	}
}

func TestIntegration_CharacterDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, queries, campaignID := setupTestDB(t)
	ctx := context.Background()

	created, err := queries.CreateCharacter(ctx, refdata.CreateCharacterParams{
		CampaignID:       campaignID,
		Name:             "Doomed",
		Race:             "elf",
		Classes:          mustJSON(t, []map[string]interface{}{{"class": "rogue", "level": 1}}),
		Level:            1,
		AbilityScores:    mustJSON(t, map[string]int{"str": 10, "dex": 10, "con": 10, "int": 10, "wis": 10, "cha": 10}),
		HpMax:            8,
		HpCurrent:        8,
		Ac:               11,
		SpeedFt:          30,
		ProficiencyBonus: 2,
		HitDiceRemaining: mustJSON(t, map[string]int{"d8": 1}),
		Languages:        []string{"Common", "Elvish"},
	})
	if err != nil {
		t.Fatalf("CreateCharacter failed: %v", err)
	}

	err = queries.DeleteCharacter(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteCharacter failed: %v", err)
	}

	count, err := queries.CountCharactersByCampaign(ctx, campaignID)
	if err != nil {
		t.Fatalf("CountCharactersByCampaign failed: %v", err)
	}
	if count != 0 {
		t.Errorf("count after delete = %d, want 0", count)
	}
}
