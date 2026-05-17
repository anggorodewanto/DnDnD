-- +goose Up
ALTER TABLE reaction_declarations ADD COLUMN dm_queue_item_id TEXT;

-- +goose Down
ALTER TABLE reaction_declarations DROP COLUMN IF EXISTS dm_queue_item_id;
