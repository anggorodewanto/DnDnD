-- +goose Up
ALTER TABLE pending_actions
    ADD COLUMN dm_queue_item_id UUID
        REFERENCES dm_queue_items(id) ON DELETE SET NULL;

CREATE INDEX idx_pending_actions_dm_queue_item_id
    ON pending_actions (dm_queue_item_id);

-- +goose Down
DROP INDEX IF EXISTS idx_pending_actions_dm_queue_item_id;
ALTER TABLE pending_actions DROP COLUMN IF EXISTS dm_queue_item_id;
