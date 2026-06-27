-- +goose Up
-- Persistent (out-of-combat) conditions for a character. Mirrors the JSON shape
-- of combatants.conditions (an array of combat.CombatCondition). Lets the DM set
-- conditions on the character sheet outside combat; these seed the combatant at
-- combat start. Defaults to an empty array so existing rows are unaffected.
ALTER TABLE characters ADD COLUMN conditions JSONB NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE characters DROP COLUMN conditions;
