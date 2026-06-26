-- +goose Up
-- ammunition_id links an ammunition weapon (the `ammunition` property) to the
-- canonical ammo item it consumes (items.id): light/hand/heavy-crossbow ->
-- crossbow-bolt, shortbow/longbow -> arrow, sling -> sling-bullet,
-- blowgun -> blowgun-needle. This replaces combat's "crossbow" -> "Bolts"
-- substring heuristic and lets the ammo matcher match by item id instead of a
-- name keyword. It is a LOGICAL reference (no enforced FK) so refdata seeding
-- order stays free — see docs/live-play/issues.md ISSUE-017 phase 2.
ALTER TABLE weapons ADD COLUMN ammunition_id TEXT;

-- +goose Down
ALTER TABLE weapons DROP COLUMN ammunition_id;
