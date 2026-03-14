-- +goose Up
CREATE TABLE pending_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters(id) ON DELETE CASCADE,
    combatant_id UUID NOT NULL REFERENCES combatants(id) ON DELETE CASCADE,
    action_text TEXT NOT NULL,
    dm_queue_message_id TEXT NOT NULL DEFAULT '',
    dm_queue_channel_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pending_actions_combatant ON pending_actions(combatant_id);
CREATE INDEX idx_pending_actions_encounter ON pending_actions(encounter_id);

-- +goose Down
DROP TABLE IF EXISTS pending_actions;
