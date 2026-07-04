-- +goose Up
-- Make deleting a character atomic so the DM dashboard "Delete character" action
-- works. A bare DELETE FROM characters previously failed on three NO ACTION
-- foreign keys: player_characters (the always-present player mapping), and the
-- optional combatants / loot_pool_items references. Cascade the mapping and
-- null the optional references instead of blocking the delete.
ALTER TABLE player_characters DROP CONSTRAINT player_characters_character_id_fkey,
  ADD CONSTRAINT player_characters_character_id_fkey
    FOREIGN KEY (character_id) REFERENCES characters(id) ON DELETE CASCADE;

ALTER TABLE combatants DROP CONSTRAINT combatants_character_id_fkey,
  ADD CONSTRAINT combatants_character_id_fkey
    FOREIGN KEY (character_id) REFERENCES characters(id) ON DELETE SET NULL;

ALTER TABLE loot_pool_items DROP CONSTRAINT loot_pool_items_claimed_by_fkey,
  ADD CONSTRAINT loot_pool_items_claimed_by_fkey
    FOREIGN KEY (claimed_by) REFERENCES characters(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE player_characters DROP CONSTRAINT player_characters_character_id_fkey,
  ADD CONSTRAINT player_characters_character_id_fkey
    FOREIGN KEY (character_id) REFERENCES characters(id);

ALTER TABLE combatants DROP CONSTRAINT combatants_character_id_fkey,
  ADD CONSTRAINT combatants_character_id_fkey
    FOREIGN KEY (character_id) REFERENCES characters(id);

ALTER TABLE loot_pool_items DROP CONSTRAINT loot_pool_items_claimed_by_fkey,
  ADD CONSTRAINT loot_pool_items_claimed_by_fkey
    FOREIGN KEY (claimed_by) REFERENCES characters(id);
