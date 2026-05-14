-- +goose Up
-- SR-038: persist staged DDB re-sync approvals so a process restart does
-- not drop the pending diff before the DM approves or rejects it.
CREATE TABLE pending_ddb_imports (
    id              UUID PRIMARY KEY,
    character_id    UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    params_json     JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pending_ddb_imports_character_id ON pending_ddb_imports(character_id);

-- +goose Down
DROP INDEX IF EXISTS idx_pending_ddb_imports_character_id;
DROP TABLE IF EXISTS pending_ddb_imports;
