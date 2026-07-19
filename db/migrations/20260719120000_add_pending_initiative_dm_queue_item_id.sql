-- +goose Up
-- APP-5 (pre-combat DM-queue surfacing): stash the id of the #dm-queue /
-- DM-Console item that a staged /initiative roll posts, so re-rolling can
-- cancel the prior item, /initiative clear:true can cancel it, and StartCombat
-- can cancel every consumed item. Nullable: a row staged before this column
-- existed (or with no notifier wired) simply carries NULL.
ALTER TABLE pending_initiatives ADD COLUMN IF NOT EXISTS dm_queue_item_id TEXT;

-- +goose Down
ALTER TABLE pending_initiatives DROP COLUMN IF EXISTS dm_queue_item_id;
