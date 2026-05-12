-- +goose Up
-- C-35: DM dashboard advantage/disadvantage override for the next attack
-- roll of a targeted combatant. Stored as a nullable text column so a NULL
-- value means "no override". The column is consumed (set to NULL) by the
-- combat service the first time the affected combatant rolls an attack.
ALTER TABLE combatants
    ADD COLUMN next_attack_adv_override TEXT;

-- +goose Down
ALTER TABLE combatants
    DROP COLUMN next_attack_adv_override;
