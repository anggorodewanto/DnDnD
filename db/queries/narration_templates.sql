-- name: InsertNarrationTemplate :one
INSERT INTO narration_templates (campaign_id, name, category, body)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetNarrationTemplate :one
SELECT *
FROM narration_templates
WHERE id = $1;

-- name: ListNarrationTemplatesByCampaign :many
SELECT *
FROM narration_templates
WHERE campaign_id = @campaign_id
  AND (@category::text = '' OR category = @category::text)
  AND (@search::text = '' OR name ILIKE '%' || @search::text || '%' OR body ILIKE '%' || @search::text || '%')
ORDER BY category ASC, name ASC;

-- name: UpdateNarrationTemplate :one
UPDATE narration_templates
SET name       = $2,
    category   = $3,
    body       = $4,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteNarrationTemplate :exec
DELETE FROM narration_templates
WHERE id = $1;
