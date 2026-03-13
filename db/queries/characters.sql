-- name: GetCharacter :one
SELECT * FROM characters WHERE id = $1;

-- name: ListCharactersByCampaign :many
SELECT * FROM characters WHERE campaign_id = $1 ORDER BY name;

-- name: CreateCharacter :one
INSERT INTO characters (
    campaign_id, name, race, classes, level, ability_scores,
    hp_max, hp_current, temp_hp, ac, ac_formula, speed_ft,
    proficiency_bonus, equipped_main_hand, equipped_off_hand,
    equipped_armor, spell_slots, pact_magic_slots, hit_dice_remaining,
    feature_uses, features, proficiencies, gold, attunement_slots,
    languages, inventory, character_data, ddb_url, homebrew
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12,
    $13, $14, $15,
    $16, $17, $18, $19,
    $20, $21, $22, $23, $24,
    $25, $26, $27, $28, $29
) RETURNING *;

-- name: UpdateCharacter :one
UPDATE characters SET
    name = $2,
    race = $3,
    classes = $4,
    level = $5,
    ability_scores = $6,
    hp_max = $7,
    hp_current = $8,
    temp_hp = $9,
    ac = $10,
    ac_formula = $11,
    speed_ft = $12,
    proficiency_bonus = $13,
    equipped_main_hand = $14,
    equipped_off_hand = $15,
    equipped_armor = $16,
    spell_slots = $17,
    pact_magic_slots = $18,
    hit_dice_remaining = $19,
    feature_uses = $20,
    features = $21,
    proficiencies = $22,
    gold = $23,
    attunement_slots = $24,
    languages = $25,
    inventory = $26,
    character_data = $27,
    ddb_url = $28,
    homebrew = $29,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateCharacterFeatureUses :one
UPDATE characters SET feature_uses = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateCharacterSpellSlots :one
UPDATE characters SET spell_slots = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteCharacter :exec
DELETE FROM characters WHERE id = $1;

-- name: CountCharactersByCampaign :one
SELECT count(*) FROM characters WHERE campaign_id = $1;

-- name: GetCharacterCardMessageID :one
SELECT card_message_id FROM characters WHERE id = $1;

-- name: SetCharacterCardMessageID :exec
UPDATE characters SET card_message_id = $2, updated_at = now() WHERE id = $1;
