-- +goose Up
--
-- Phase 118 — Concentration Cleanup Integration.
--
-- Concentration was previously implicit: callers passed
-- `cmd.CurrentConcentration` (a string) into Cast/AoE flows, and read-time
-- code (e.g. /status) inferred it from `encounter_zones`. That inference is
-- lossy because Bless / Hold Person / Hex / Invisibility never create zones.
-- Phase 118 wires `BreakConcentration` into the production paths
-- (damage CON-save failure, incapacitation, Silence entry, replacing a
-- concentration spell, voluntary drop), all of which need to query
-- "is this caster concentrating, and on what?" without reaching into spell
-- artifacts. Add an authoritative store on `combatants`.
--
-- Both columns are nullable. NULL = not concentrating. The id column stores
-- the spell ID (e.g. "bless") so callers can look up `refdata.Spell` to
-- determine V/S components etc.; the name column is preserved separately so
-- formatted log lines do not require a spell lookup.

ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS concentration_spell_id   TEXT,
    ADD COLUMN IF NOT EXISTS concentration_spell_name TEXT;

-- +goose Down

ALTER TABLE combatants
    DROP COLUMN IF EXISTS concentration_spell_id,
    DROP COLUMN IF EXISTS concentration_spell_name;
