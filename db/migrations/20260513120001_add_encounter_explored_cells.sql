-- +goose Up
--
-- SR-031: Persist per-encounter explored cells across bot restarts.
--
-- Before this migration, mapRegeneratorAdapter kept the explored-tile set
-- entirely in memory (`map[uuid.UUID]map[int]bool` on the adapter struct).
-- A redeploy / crash wiped every party's exploration history mid-campaign,
-- which is incompatible with the async-first design (see SUMMARY.md §2 in
-- batch-10).
--
-- Minimal-change approach: stash the packed tile-index set as a JSONB
-- column on the existing `encounters` row. Exploration grows monotonically
-- per encounter and is read/written as a single blob on each map regen, so
-- one column outperforms a separate `encounter_explored_cells(x, y)` table
-- for this access pattern (no per-tile mutations, only union-and-overwrite).
--
-- The column stores a JSON array of integer tile indexes
-- (row*width + col, matching the existing in-memory key). Default '[]' so
-- existing rows are immediately usable without a separate backfill.
--
-- Forward-only: the app is not live anywhere, so no row conversion is
-- required and a goose Down stanza is intentionally omitted — restore from
-- backup if a revert is needed (mirrors the recent
-- 20260513120000_make_player_characters_unique_partial.sql convention).

ALTER TABLE encounters
    ADD COLUMN explored_cells JSONB NOT NULL DEFAULT '[]';
