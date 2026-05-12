-- +goose Up
-- F-89d: persist in-flight ASI/Feat choice prompts so a process restart
-- does not drop pending DM approvals. The ASI handler historically held
-- these in an in-memory sync.RWMutex map; this table backs the same data
-- with the schema { character_id PK, snapshot_json, created_at }.
CREATE TABLE pending_asi (
    character_id    UUID PRIMARY KEY,
    snapshot_json   JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS pending_asi;
