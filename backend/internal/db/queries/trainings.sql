-- name: CreateTraining :one
INSERT INTO trainings (user_id, date, name, organizer, cost, url) VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: GetTraining :one
SELECT * FROM trainings WHERE id = $1 AND user_id = $2;

-- name: ListTrainings :many
SELECT * FROM trainings WHERE user_id = $1 ORDER BY date DESC;

-- name: UpdateTraining :exec
UPDATE trainings SET date = $1, name = $2, organizer = $3, cost = $4, url = $5, updated_at = CURRENT_TIMESTAMP WHERE id = $6 AND user_id = $7;

-- name: DeleteTraining :exec
DELETE FROM trainings WHERE id = $1 AND user_id = $2;
