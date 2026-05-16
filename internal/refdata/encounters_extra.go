package refdata

import (
	"context"

	"github.com/google/uuid"
)

const countActiveEncountersByCharacterID = `
SELECT COUNT(*) FROM encounters e
JOIN combatants cb ON cb.encounter_id = e.id
WHERE cb.character_id = $1 AND e.status = 'active'
`

// CountActiveEncountersByCharacterID returns the number of active encounters
// a character is currently a combatant in. Used to detect the corrupt state
// where LIMIT 1 in GetActiveEncounterIDByCharacterID would silently mask
// duplicates (finding J-H05).
func (q *Queries) CountActiveEncountersByCharacterID(ctx context.Context, characterID uuid.NullUUID) (int64, error) {
	row := q.db.QueryRowContext(ctx, countActiveEncountersByCharacterID, characterID)
	var count int64
	err := row.Scan(&count)
	return count, err
}
