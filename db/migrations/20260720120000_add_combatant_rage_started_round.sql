-- +goose Up
-- 2024 PHB Rage duration model: a Rage "lasts until the end of your next
-- turn", so the turn on which it was activated can never be the turn that ends
-- it (the live bug: Forge raged, missed one handaxe swing, and the rage
-- evaporated at end of that same turn).
--
-- rage_started_round records the encounter round in which the rage was last
-- STARTED or EXTENDED (the /bonus rage bonus-action extension refreshes it).
-- The end-of-turn sweep treats "rage_started_round == the ending turn's round"
-- as an unconditional grace window, which is exactly the RAW "until the end of
-- your NEXT turn" horizon — one column expresses both the activation grace and
-- the bonus-action extension.
--
-- Nullable: rows raging before this column existed simply carry NULL, which
-- reads as "no grace window" and falls back to the pre-existing
-- attacked/took-damage activity check.
ALTER TABLE combatants ADD COLUMN IF NOT EXISTS rage_started_round INTEGER;

-- +goose Down
ALTER TABLE combatants DROP COLUMN IF EXISTS rage_started_round;
