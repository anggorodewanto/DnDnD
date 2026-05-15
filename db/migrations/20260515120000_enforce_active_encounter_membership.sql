-- +goose Up

-- +goose StatementBegin
-- F-13: Enforce that a character can only be a combatant in one active
-- encounter at a time. The service layer already checks this, but a
-- trigger provides DB-level safety against concurrent inserts.
CREATE OR REPLACE FUNCTION check_active_encounter_membership()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.character_id IS NOT NULL THEN
        IF EXISTS (
            SELECT 1 FROM combatants cb
            JOIN encounters e ON e.id = cb.encounter_id
            WHERE cb.character_id = NEW.character_id
              AND e.status = 'active'
              AND cb.encounter_id != NEW.encounter_id
        ) THEN
            RAISE EXCEPTION 'character % is already a combatant in another active encounter',
                NEW.character_id
                USING ERRCODE = 'unique_violation';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_enforce_active_encounter_membership
    BEFORE INSERT ON combatants
    FOR EACH ROW
    EXECUTE FUNCTION check_active_encounter_membership();

-- +goose Down
DROP TRIGGER IF EXISTS trg_enforce_active_encounter_membership ON combatants;
DROP FUNCTION IF EXISTS check_active_encounter_membership();
