-- +goose Up
ALTER TABLE reaction_declarations
    ADD COLUMN counterspell_enemy_spell TEXT,
    ADD COLUMN counterspell_enemy_level INTEGER,
    ADD COLUMN counterspell_slot_used INTEGER,
    ADD COLUMN counterspell_status TEXT,
    ADD COLUMN counterspell_dc INTEGER;

-- +goose Down
ALTER TABLE reaction_declarations
    DROP COLUMN IF EXISTS counterspell_enemy_spell,
    DROP COLUMN IF EXISTS counterspell_enemy_level,
    DROP COLUMN IF EXISTS counterspell_slot_used,
    DROP COLUMN IF EXISTS counterspell_status,
    DROP COLUMN IF EXISTS counterspell_dc;
