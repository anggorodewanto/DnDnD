package ddbimport

import (
	"testing"
)

// Minimal DDB JSON fixture representing a Level 5 Human Fighter.
var testDDBJSON = []byte(`{
	"id": 12345678,
	"success": true,
	"message": "Character successfully retrieved.",
	"data": {
		"id": 12345678,
		"name": "Thorin Ironforge",
		"race": {
			"fullName": "Human",
			"baseName": "Human",
			"isSubRace": false
		},
		"classes": [
			{
				"definition": {
					"name": "Fighter",
					"hitDice": 10
				},
				"subclassDefinition": {
					"name": "Champion"
				},
				"level": 5
			}
		],
		"stats": [
			{"id": 1, "name": null, "value": 16},
			{"id": 2, "name": null, "value": 14},
			{"id": 3, "name": null, "value": 15},
			{"id": 4, "name": null, "value": 10},
			{"id": 5, "name": null, "value": 12},
			{"id": 6, "name": null, "value": 8}
		],
		"bonusStats": [
			{"id": 1, "name": null, "value": null},
			{"id": 2, "name": null, "value": null},
			{"id": 3, "name": null, "value": null},
			{"id": 4, "name": null, "value": null},
			{"id": 5, "name": null, "value": null},
			{"id": 6, "name": null, "value": null}
		],
		"overrideStats": [
			{"id": 1, "name": null, "value": null},
			{"id": 2, "name": null, "value": null},
			{"id": 3, "name": null, "value": null},
			{"id": 4, "name": null, "value": null},
			{"id": 5, "name": null, "value": null},
			{"id": 6, "name": null, "value": null}
		],
		"baseHitPoints": 44,
		"bonusHitPoints": 0,
		"overrideHitPoints": null,
		"removedHitPoints": 5,
		"temporaryHitPoints": 3,
		"inventory": [
			{
				"id": 1001,
				"definition": {
					"name": "Longsword",
					"type": "Weapon",
					"filterType": "Weapon",
					"canAttune": false,
					"magic": false,
					"rarity": "Common",
					"description": "A standard longsword."
				},
				"equipped": true,
				"quantity": 1
			},
			{
				"id": 1002,
				"definition": {
					"name": "Chain Mail",
					"type": "Armor",
					"filterType": "Armor",
					"armorClass": 16,
					"canAttune": false,
					"magic": false,
					"rarity": "Common",
					"description": "Heavy armor."
				},
				"equipped": true,
				"quantity": 1
			},
			{
				"id": 1003,
				"definition": {
					"name": "Shield",
					"type": "Armor",
					"filterType": "Armor",
					"armorClass": 2,
					"canAttune": false,
					"magic": false,
					"rarity": "Common",
					"description": "A wooden shield."
				},
				"equipped": true,
				"quantity": 1
			}
		],
		"currencies": {
			"gp": 50,
			"sp": 20,
			"cp": 100,
			"ep": 0,
			"pp": 0
		},
		"modifiers": {
			"race": [
				{
					"type": "language",
					"subType": "common",
					"friendlyTypeName": "Language",
					"friendlySubtypeName": "Common"
				},
				{
					"type": "language",
					"subType": "dwarvish",
					"friendlyTypeName": "Language",
					"friendlySubtypeName": "Dwarvish"
				}
			],
			"class": [
				{
					"type": "proficiency",
					"subType": "saving-throws",
					"friendlyTypeName": "Saving Throws",
					"friendlySubtypeName": "Strength"
				},
				{
					"type": "proficiency",
					"subType": "saving-throws",
					"friendlyTypeName": "Saving Throws",
					"friendlySubtypeName": "Constitution"
				},
				{
					"type": "proficiency",
					"subType": "athletics",
					"friendlyTypeName": "Skill",
					"friendlySubtypeName": "Athletics"
				}
			],
			"background": [],
			"item": [],
			"feat": [],
			"condition": []
		},
		"spells": {
			"class": [],
			"race": [],
			"item": [],
			"feat": []
		},
		"options": {
			"class": [],
			"race": [],
			"feat": []
		},
		"actions": {
			"class": [],
			"race": [],
			"feat": []
		},
		"classSpells": [],
		"customDefenseAdjustments": [],
		"customSenses": [],
		"customSpeeds": [],
		"customProficiencies": [],
		"customActions": [],
		"characterValues": [],
		"conditions": [],
		"deathSaves": {
			"failCount": 0,
			"successCount": 0,
			"isStabilized": true
		},
		"adjustmentStats": null,
		"bonusStats": [
			{"id": 1, "name": null, "value": null},
			{"id": 2, "name": null, "value": null},
			{"id": 3, "name": null, "value": null},
			{"id": 4, "name": null, "value": null},
			{"id": 5, "name": null, "value": null},
			{"id": 6, "name": null, "value": null}
		],
		"overrideStats": [
			{"id": 1, "name": null, "value": null},
			{"id": 2, "name": null, "value": null},
			{"id": 3, "name": null, "value": null},
			{"id": 4, "name": null, "value": null},
			{"id": 5, "name": null, "value": null},
			{"id": 6, "name": null, "value": null}
		],
		"campaign": null,
		"creatures": [],
		"vehicles": [],
		"notes": {
			"personalPossessions": ""
		},
		"preferences": {
			"useHomebrewContent": false
		}
	}
}`)

func TestParseDDBJSON_Name(t *testing.T) {
	result, err := ParseDDBJSON(testDDBJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "Thorin Ironforge" {
		t.Errorf("Name = %q, want %q", result.Name, "Thorin Ironforge")
	}
}

func TestParseDDBJSON_Race(t *testing.T) {
	result, err := ParseDDBJSON(testDDBJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Race != "Human" {
		t.Errorf("Race = %q, want %q", result.Race, "Human")
	}
}

func TestParseDDBJSON_Classes(t *testing.T) {
	result, err := ParseDDBJSON(testDDBJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(result.Classes))
	}
	c := result.Classes[0]
	if c.Class != "Fighter" {
		t.Errorf("Class = %q, want %q", c.Class, "Fighter")
	}
	if c.Subclass != "Champion" {
		t.Errorf("Subclass = %q, want %q", c.Subclass, "Champion")
	}
	if c.Level != 5 {
		t.Errorf("Level = %d, want %d", c.Level, 5)
	}
}

func TestParseDDBJSON_Level(t *testing.T) {
	result, err := ParseDDBJSON(testDDBJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Level != 5 {
		t.Errorf("Level = %d, want %d", result.Level, 5)
	}
}

func TestParseDDBJSON_AbilityScores(t *testing.T) {
	result, err := ParseDDBJSON(testDDBJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AbilityScores.STR != 16 {
		t.Errorf("STR = %d, want %d", result.AbilityScores.STR, 16)
	}
	if result.AbilityScores.DEX != 14 {
		t.Errorf("DEX = %d, want %d", result.AbilityScores.DEX, 14)
	}
	if result.AbilityScores.CON != 15 {
		t.Errorf("CON = %d, want %d", result.AbilityScores.CON, 15)
	}
	if result.AbilityScores.INT != 10 {
		t.Errorf("INT = %d, want %d", result.AbilityScores.INT, 10)
	}
	if result.AbilityScores.WIS != 12 {
		t.Errorf("WIS = %d, want %d", result.AbilityScores.WIS, 12)
	}
	if result.AbilityScores.CHA != 8 {
		t.Errorf("CHA = %d, want %d", result.AbilityScores.CHA, 8)
	}
}

func TestParseDDBJSON_HP(t *testing.T) {
	result, err := ParseDDBJSON(testDDBJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// baseHitPoints=44, bonusHitPoints=0 => hpMax=44
	if result.HPMax != 44 {
		t.Errorf("HPMax = %d, want %d", result.HPMax, 44)
	}
	// hpCurrent = hpMax - removedHitPoints = 44 - 5 = 39
	if result.HPCurrent != 39 {
		t.Errorf("HPCurrent = %d, want %d", result.HPCurrent, 39)
	}
	if result.TempHP != 3 {
		t.Errorf("TempHP = %d, want %d", result.TempHP, 3)
	}
}

func TestParseDDBJSON_AC(t *testing.T) {
	result, err := ParseDDBJSON(testDDBJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Chain Mail (16) + Shield (2) = 18
	if result.AC != 18 {
		t.Errorf("AC = %d, want %d", result.AC, 18)
	}
}

func TestParseDDBJSON_Inventory(t *testing.T) {
	result, err := ParseDDBJSON(testDDBJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Inventory) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Inventory))
	}
	if result.Inventory[0].Name != "Longsword" {
		t.Errorf("first item = %q, want %q", result.Inventory[0].Name, "Longsword")
	}
}

func TestParseDDBJSON_Gold(t *testing.T) {
	result, err := ParseDDBJSON(testDDBJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 50gp + 2gp(20sp) + 1gp(100cp) = 53
	if result.Gold != 53 {
		t.Errorf("Gold = %d, want %d", result.Gold, 53)
	}
}

func TestParseDDBJSON_Languages(t *testing.T) {
	result, err := ParseDDBJSON(testDDBJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Languages) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(result.Languages))
	}
}

func TestParseDDBJSON_InvalidJSON(t *testing.T) {
	_, err := ParseDDBJSON([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseDDBJSON_EmptyData(t *testing.T) {
	_, err := ParseDDBJSON([]byte(`{"data": null}`))
	if err == nil {
		t.Fatal("expected error for null data")
	}
}

func TestParseDDBJSON_OverrideStats(t *testing.T) {
	json := []byte(`{
		"data": {
			"name": "Test",
			"race": {"fullName": "Elf"},
			"classes": [{"definition": {"name": "Wizard", "hitDice": 6}, "level": 1}],
			"stats": [
				{"id": 1, "value": 10},
				{"id": 2, "value": 10},
				{"id": 3, "value": 10},
				{"id": 4, "value": 10},
				{"id": 5, "value": 10},
				{"id": 6, "value": 10}
			],
			"bonusStats": [
				{"id": 1, "value": null},
				{"id": 2, "value": 2},
				{"id": 3, "value": null},
				{"id": 4, "value": null},
				{"id": 5, "value": null},
				{"id": 6, "value": null}
			],
			"overrideStats": [
				{"id": 1, "value": 20},
				{"id": 2, "value": null},
				{"id": 3, "value": null},
				{"id": 4, "value": null},
				{"id": 5, "value": null},
				{"id": 6, "value": null}
			],
			"baseHitPoints": 8,
			"bonusHitPoints": 0,
			"removedHitPoints": 0,
			"temporaryHitPoints": 0,
			"inventory": [],
			"currencies": {"gp": 0, "sp": 0, "cp": 0, "ep": 0, "pp": 0},
			"modifiers": {"race": [], "class": [], "background": [], "item": [], "feat": [], "condition": []},
			"spells": {"class": [], "race": [], "item": [], "feat": []}
		}
	}`)
	result, err := ParseDDBJSON(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// STR overridden to 20
	if result.AbilityScores.STR != 20 {
		t.Errorf("STR = %d, want 20 (overridden)", result.AbilityScores.STR)
	}
	// DEX = base(10) + bonus(2) = 12
	if result.AbilityScores.DEX != 12 {
		t.Errorf("DEX = %d, want 12 (base+bonus)", result.AbilityScores.DEX)
	}
}

func TestParseDDBJSON_MulticlassLevel(t *testing.T) {
	json := []byte(`{
		"data": {
			"name": "Multi",
			"race": {"fullName": "Half-Elf"},
			"classes": [
				{"definition": {"name": "Fighter", "hitDice": 10}, "level": 3},
				{"definition": {"name": "Rogue", "hitDice": 8}, "level": 2}
			],
			"stats": [
				{"id": 1, "value": 14},
				{"id": 2, "value": 16},
				{"id": 3, "value": 12},
				{"id": 4, "value": 10},
				{"id": 5, "value": 10},
				{"id": 6, "value": 10}
			],
			"bonusStats": [
				{"id": 1, "value": null},
				{"id": 2, "value": null},
				{"id": 3, "value": null},
				{"id": 4, "value": null},
				{"id": 5, "value": null},
				{"id": 6, "value": null}
			],
			"overrideStats": [
				{"id": 1, "value": null},
				{"id": 2, "value": null},
				{"id": 3, "value": null},
				{"id": 4, "value": null},
				{"id": 5, "value": null},
				{"id": 6, "value": null}
			],
			"baseHitPoints": 30,
			"bonusHitPoints": 0,
			"removedHitPoints": 0,
			"temporaryHitPoints": 0,
			"inventory": [],
			"currencies": {"gp": 0, "sp": 0, "cp": 0, "ep": 0, "pp": 0},
			"modifiers": {"race": [], "class": [], "background": [], "item": [], "feat": [], "condition": []},
			"spells": {"class": [], "race": [], "item": [], "feat": []}
		}
	}`)
	result, err := ParseDDBJSON(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Level != 5 {
		t.Errorf("Level = %d, want 5 (3+2)", result.Level)
	}
	if len(result.Classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(result.Classes))
	}
}
