package refdata

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

const listPlayerCharactersByStatus = `-- name: ListPlayerCharactersByStatus :many
SELECT pc.id, pc.campaign_id, pc.character_id, pc.discord_user_id, pc.status,
       pc.dm_feedback, pc.created_via, pc.created_at, pc.updated_at,
       c.name AS character_name, c.race, c.level, c.classes, c.hp_max,
       c.hp_current, c.ac, c.speed_ft, c.ability_scores, c.languages, c.ddb_url
FROM player_characters pc
JOIN characters c ON c.id = pc.character_id
WHERE pc.campaign_id = $1 AND pc.status = $2
ORDER BY pc.created_at
`

// ListPlayerCharactersByStatusRow is the result type for the joined query.
type ListPlayerCharactersByStatusRow struct {
	PlayerCharacter
	CharacterName string          `json:"character_name"`
	Race          string          `json:"race"`
	Level         int32           `json:"level"`
	Classes       json.RawMessage `json:"classes"`
	HpMax         int32           `json:"hp_max"`
	HpCurrent     int32           `json:"hp_current"`
	Ac            int32           `json:"ac"`
	SpeedFt       int32           `json:"speed_ft"`
	AbilityScores json.RawMessage `json:"ability_scores"`
	Languages     []string        `json:"languages"`
	DdbUrl        sql.NullString  `json:"ddb_url"`
}

// ListPlayerCharactersByStatusParams are the parameters for the query.
type ListPlayerCharactersByStatusParams struct {
	CampaignID uuid.UUID `json:"campaign_id"`
	Status     string    `json:"status"`
}

func (q *Queries) ListPlayerCharactersByStatus(ctx context.Context, arg ListPlayerCharactersByStatusParams) ([]ListPlayerCharactersByStatusRow, error) {
	rows, err := q.db.QueryContext(ctx, listPlayerCharactersByStatus, arg.CampaignID, arg.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListPlayerCharactersByStatusRow{}
	for rows.Next() {
		var i ListPlayerCharactersByStatusRow
		if err := rows.Scan(
			&i.ID,
			&i.CampaignID,
			&i.CharacterID,
			&i.DiscordUserID,
			&i.PlayerCharacter.Status,
			&i.DmFeedback,
			&i.CreatedVia,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.CharacterName,
			&i.Race,
			&i.Level,
			&i.Classes,
			&i.HpMax,
			&i.HpCurrent,
			&i.Ac,
			&i.SpeedFt,
			&i.AbilityScores,
			pq.Array(&i.Languages),
			&i.DdbUrl,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getPlayerCharacterWithCharacter = `-- name: GetPlayerCharacterWithCharacter :one
SELECT pc.id, pc.campaign_id, pc.character_id, pc.discord_user_id, pc.status,
       pc.dm_feedback, pc.created_via, pc.created_at, pc.updated_at,
       c.name AS character_name, c.race, c.level, c.classes, c.hp_max,
       c.hp_current, c.ac, c.speed_ft, c.ability_scores, c.languages, c.ddb_url
FROM player_characters pc
JOIN characters c ON c.id = pc.character_id
WHERE pc.id = $1
`

// GetPlayerCharacterWithCharacterRow is the result type for the joined single-row query.
type GetPlayerCharacterWithCharacterRow = ListPlayerCharactersByStatusRow

func (q *Queries) GetPlayerCharacterWithCharacter(ctx context.Context, id uuid.UUID) (GetPlayerCharacterWithCharacterRow, error) {
	row := q.db.QueryRowContext(ctx, getPlayerCharacterWithCharacter, id)
	var i GetPlayerCharacterWithCharacterRow
	err := row.Scan(
		&i.ID,
		&i.CampaignID,
		&i.CharacterID,
		&i.DiscordUserID,
		&i.PlayerCharacter.Status,
		&i.DmFeedback,
		&i.CreatedVia,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.CharacterName,
		&i.Race,
		&i.Level,
		&i.Classes,
		&i.HpMax,
		&i.HpCurrent,
		&i.Ac,
		&i.SpeedFt,
		&i.AbilityScores,
		pq.Array(&i.Languages),
		&i.DdbUrl,
	)
	return i, err
}
