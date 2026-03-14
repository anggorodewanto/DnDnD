-- +goose Up
ALTER TABLE reaction_declarations
    ADD COLUMN is_readied_action BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN spell_name TEXT,
    ADD COLUMN spell_slot_level INTEGER;

-- +goose Down
ALTER TABLE reaction_declarations
    DROP COLUMN IF EXISTS is_readied_action,
    DROP COLUMN IF EXISTS spell_name,
    DROP COLUMN IF EXISTS spell_slot_level;
