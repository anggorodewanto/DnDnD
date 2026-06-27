-- +goose Up
-- review_before holds the DM-reviewable projection (ReviewCharacter JSON) of a
-- character as it was the last time the DM approved it. It is captured when a
-- player edit transitions an approved character back to pending, so the
-- approval page can show a before -> after diff of what the player changed. It
-- is cleared on approve (the new state becomes the implicit baseline) and left
-- untouched on changes_requested / reject so the diff survives resubmits.
-- Nullable: new submissions and DM self-edits never set it. See
-- docs/dm-character-review-diff.md.
ALTER TABLE player_characters ADD COLUMN review_before JSONB;

-- +goose Down
ALTER TABLE player_characters DROP COLUMN review_before;
