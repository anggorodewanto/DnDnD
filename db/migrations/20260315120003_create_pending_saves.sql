-- +goose Up
CREATE TABLE pending_saves (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters(id) ON DELETE CASCADE,
    combatant_id UUID NOT NULL REFERENCES combatants(id) ON DELETE CASCADE,
    ability TEXT NOT NULL,
    dc INT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    roll_result INT,
    success BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pending_saves_combatant_status ON pending_saves(combatant_id, status);
CREATE INDEX idx_pending_saves_encounter_status ON pending_saves(encounter_id, status);

-- +goose Down
DROP TABLE IF EXISTS pending_saves;
