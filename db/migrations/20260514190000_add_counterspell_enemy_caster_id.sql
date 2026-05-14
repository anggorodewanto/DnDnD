-- +goose Up
ALTER TABLE reaction_declarations
    ADD COLUMN counterspell_enemy_caster_id UUID;

-- +goose Down
ALTER TABLE reaction_declarations
    DROP COLUMN IF EXISTS counterspell_enemy_caster_id;
