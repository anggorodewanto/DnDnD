-- +goose Up
CREATE TABLE reaction_declarations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id    UUID NOT NULL REFERENCES encounters(id) ON DELETE CASCADE,
    combatant_id    UUID NOT NULL REFERENCES combatants(id) ON DELETE CASCADE,
    description     TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at         TIMESTAMPTZ,
    used_on_round   INTEGER
);

CREATE INDEX idx_reaction_declarations_encounter_id ON reaction_declarations(encounter_id);
CREATE INDEX idx_reaction_declarations_combatant_id ON reaction_declarations(combatant_id);

-- +goose Down
DROP TABLE IF EXISTS reaction_declarations;
