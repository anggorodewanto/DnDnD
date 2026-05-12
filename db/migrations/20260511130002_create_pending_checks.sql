-- +goose Up
-- F-81-dm-prompted-checks: DM-initiated check prompts. Analog to
-- pending_saves for the /save flow — persists an outstanding skill /
-- ability check ask so the player can resolve it through /check after
-- bot restarts. Status moves pending → rolled (or forfeited) on player
-- response or DM-side cancellation.
CREATE TABLE pending_checks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id  UUID NOT NULL REFERENCES encounters(id) ON DELETE CASCADE,
    combatant_id  UUID NOT NULL REFERENCES combatants(id) ON DELETE CASCADE,
    skill         TEXT NOT NULL,
    dc            INT NOT NULL DEFAULT 0,
    reason        TEXT,
    status        TEXT NOT NULL DEFAULT 'pending',
    roll_result   INT,
    success       BOOLEAN,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pending_checks_combatant_status ON pending_checks(combatant_id, status);
CREATE INDEX idx_pending_checks_encounter_status ON pending_checks(encounter_id, status);

-- +goose Down
DROP TABLE IF EXISTS pending_checks;
