-- +goose Up
CREATE TABLE encounter_zones (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id           UUID NOT NULL REFERENCES encounters(id) ON DELETE CASCADE,
    source_combatant_id    UUID NOT NULL REFERENCES combatants(id) ON DELETE CASCADE,
    source_spell           TEXT NOT NULL,
    shape                  TEXT NOT NULL,
    origin_col             TEXT NOT NULL,
    origin_row             INTEGER NOT NULL,
    dimensions             JSONB NOT NULL,
    anchor_mode            TEXT NOT NULL DEFAULT 'static',
    anchor_combatant_id    UUID REFERENCES combatants(id) ON DELETE SET NULL,
    zone_type              TEXT NOT NULL,
    overlay_color          TEXT NOT NULL,
    marker_icon            TEXT,
    requires_concentration BOOLEAN NOT NULL DEFAULT false,
    expires_at_round       INTEGER,
    zone_triggers          JSONB DEFAULT '[]',
    triggered_this_round   JSONB DEFAULT '{}',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_encounter_zones_encounter_id ON encounter_zones(encounter_id);
CREATE INDEX idx_encounter_zones_source_combatant_id ON encounter_zones(source_combatant_id);
CREATE INDEX idx_encounter_zones_anchor_combatant_id ON encounter_zones(anchor_combatant_id);

-- +goose Down
DROP TABLE IF EXISTS encounter_zones;
