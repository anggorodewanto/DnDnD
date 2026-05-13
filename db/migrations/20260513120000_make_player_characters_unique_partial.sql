-- +goose Up
--
-- SR-012: Make player_characters uniqueness ignore retired rows.
--
-- The Phase 8 migration (20260310120007) created two inline table-level
-- UNIQUE constraints:
--   UNIQUE(campaign_id, discord_user_id)  -- conventional name
--                                            player_characters_campaign_id_discord_user_id_key
--   UNIQUE(campaign_id, character_id)     -- conventional name
--                                            player_characters_campaign_id_character_id_key
--
-- Spec line 40 promises that after the DM approves a /retire, "the player is
-- unlinked — they can now /create-character, /import, or /register a new
-- character." Spec line 43 separately requires retired rows to remain in the
-- DB ("for story continuity — the DM can re-activate them from the
-- dashboard if needed"). The retire approval path only sets status='retired'
-- on the existing row, so the original full-table UNIQUE constraints
-- contradict both promises:
--
--   * A second /register with the same (campaign_id, discord_user_id) fails
--     with 23505 even after retire approval.
--   * Re-activating a retired character via the dashboard cannot recreate
--     the same (campaign_id, character_id) pair either.
--
-- Fix: drop the two full-table constraints and replace each with a partial
-- UNIQUE INDEX scoped to non-retired rows. This is functionally equivalent
-- to a table-level UNIQUE for INSERT-time enforcement, but lets retired
-- rows sit alongside a fresh pending/approved row for the same key.
--
-- Forward-only (authorized by the user; the app is not live anywhere). No
-- existing rows are converted or deleted; the change is a pure schema swap.
-- A goose Down stanza is deliberately omitted: if you need to revert,
-- restore from backup or hand-write the inverse migration. Per repo
-- convention (e.g. 20260415120000_add_encounter_mode.sql) recent migrations
-- frequently omit Down stanzas when reverting in place is not desired.
--
-- Audit before writing: grep -rn "player_characters_campaign" in
-- internal/ and cmd/ returns no hits, so no Go code relies on the old
-- constraint name in error handling. None of the queries in
-- db/queries/player_characters.sql use ON CONFLICT, so the partial index
-- swap is transparent to sqlc; `sqlc generate` was not re-run.

ALTER TABLE player_characters
    DROP CONSTRAINT IF EXISTS player_characters_campaign_id_discord_user_id_key;

ALTER TABLE player_characters
    DROP CONSTRAINT IF EXISTS player_characters_campaign_id_character_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_player_characters_unique_active_discord_user
    ON player_characters (campaign_id, discord_user_id)
    WHERE status != 'retired';

CREATE UNIQUE INDEX IF NOT EXISTS idx_player_characters_unique_active_character
    ON player_characters (campaign_id, character_id)
    WHERE status != 'retired';
