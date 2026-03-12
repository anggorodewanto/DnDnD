-- +goose Up

CREATE TABLE encounters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id     UUID NOT NULL REFERENCES campaigns(id),
    map_id          UUID REFERENCES maps(id),
    name            TEXT NOT NULL,
    display_name    TEXT,
    template_id     UUID REFERENCES encounter_templates(id),
    status          TEXT NOT NULL DEFAULT 'preparing' CHECK (status IN ('preparing', 'active', 'completed')),
    round_number    INTEGER NOT NULL DEFAULT 0,
    current_turn_id UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_encounters_campaign_id ON encounters(campaign_id);

CREATE TABLE combatants (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id                UUID NOT NULL REFERENCES encounters(id) ON DELETE CASCADE,
    character_id                UUID REFERENCES characters(id),
    creature_ref_id             TEXT,
    short_id                    TEXT NOT NULL,
    display_name                TEXT NOT NULL,
    initiative_roll             INTEGER NOT NULL DEFAULT 0,
    initiative_order            INTEGER NOT NULL DEFAULT 0,
    position_col                TEXT NOT NULL DEFAULT 'A',
    position_row                INTEGER NOT NULL DEFAULT 1,
    altitude_ft                 INTEGER NOT NULL DEFAULT 0,
    hp_max                      INTEGER NOT NULL,
    hp_current                  INTEGER NOT NULL,
    temp_hp                     INTEGER NOT NULL DEFAULT 0,
    ac                          INTEGER NOT NULL,
    conditions                  JSONB NOT NULL DEFAULT '[]',
    exhaustion_level            INTEGER NOT NULL DEFAULT 0,
    death_saves                 JSONB,
    is_visible                  BOOLEAN NOT NULL DEFAULT true,
    is_alive                    BOOLEAN NOT NULL DEFAULT true,
    is_npc                      BOOLEAN NOT NULL DEFAULT false,
    is_raging                   BOOLEAN NOT NULL DEFAULT false,
    rage_rounds_remaining       INTEGER,
    rage_attacked_this_round    BOOLEAN NOT NULL DEFAULT false,
    rage_took_damage_this_round BOOLEAN NOT NULL DEFAULT false,
    is_wild_shaped              BOOLEAN NOT NULL DEFAULT false,
    wild_shape_creature_ref     TEXT,
    wild_shape_original         JSONB,
    summoner_id                 UUID REFERENCES combatants(id),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_combatants_encounter_id ON combatants(encounter_id);

CREATE TABLE turns (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id            UUID NOT NULL REFERENCES encounters(id) ON DELETE CASCADE,
    combatant_id            UUID NOT NULL REFERENCES combatants(id) ON DELETE CASCADE,
    round_number            INTEGER NOT NULL,
    status                  TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'skipped')),
    movement_remaining_ft   INTEGER NOT NULL DEFAULT 30,
    action_used             BOOLEAN NOT NULL DEFAULT false,
    bonus_action_used       BOOLEAN NOT NULL DEFAULT false,
    bonus_action_spell_cast BOOLEAN NOT NULL DEFAULT false,
    action_spell_cast       BOOLEAN NOT NULL DEFAULT false,
    reaction_used           BOOLEAN NOT NULL DEFAULT false,
    free_interact_used      BOOLEAN NOT NULL DEFAULT false,
    attacks_remaining       INTEGER NOT NULL DEFAULT 1,
    has_disengaged          BOOLEAN NOT NULL DEFAULT false,
    action_surged           BOOLEAN NOT NULL DEFAULT false,
    started_at              TIMESTAMPTZ,
    timeout_at              TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_turns_encounter_id ON turns(encounter_id);

-- Add the circular FK from encounters to turns now that both tables exist
ALTER TABLE encounters ADD CONSTRAINT fk_encounters_current_turn
    FOREIGN KEY (current_turn_id) REFERENCES turns(id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE action_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    turn_id         UUID NOT NULL REFERENCES turns(id) ON DELETE CASCADE,
    encounter_id    UUID NOT NULL REFERENCES encounters(id) ON DELETE CASCADE,
    action_type     TEXT NOT NULL,
    actor_id        UUID NOT NULL REFERENCES combatants(id),
    target_id       UUID REFERENCES combatants(id),
    description     TEXT,
    before_state    JSONB NOT NULL,
    after_state     JSONB NOT NULL,
    dice_rolls      JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_action_log_encounter_id ON action_log(encounter_id);
CREATE INDEX idx_action_log_turn_id ON action_log(turn_id);

-- +goose Down
DROP TABLE IF EXISTS action_log;
ALTER TABLE encounters DROP CONSTRAINT IF EXISTS fk_encounters_current_turn;
DROP TABLE IF EXISTS turns;
DROP TABLE IF EXISTS combatants;
DROP TABLE IF EXISTS encounters;
