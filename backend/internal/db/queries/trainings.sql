-- name: CreateTraining :one
INSERT INTO trainings (user_id, date, name, organizer, cost, url) VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: GetTraining :one
SELECT * FROM trainings WHERE id = $1 AND user_id = $2;

-- name: ListTrainings :many
-- A set after_id starts the page after the cursor's sort key. The keyset
-- tuple sorts descending like the ORDER BY: a NULL date first.
SELECT * FROM trainings
WHERE user_id = sqlc.arg(user_id)
  AND (sqlc.narg(after_id)::BIGINT IS NULL
   OR (date IS NULL, COALESCE(date, ''), id)
    < (sqlc.narg(after_date)::TEXT IS NULL,
       COALESCE(sqlc.narg(after_date)::TEXT, ''), sqlc.narg(after_id)::BIGINT))
ORDER BY date DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountTrainings :one
SELECT COUNT(*)::BIGINT FROM trainings WHERE user_id = $1;

-- name: UpdateTraining :exec
UPDATE trainings SET date = $1, name = $2, organizer = $3, cost = $4, url = $5, updated_at = CURRENT_TIMESTAMP WHERE id = $6 AND user_id = $7;

-- name: DeleteTraining :exec
DELETE FROM trainings WHERE id = $1 AND user_id = $2;
