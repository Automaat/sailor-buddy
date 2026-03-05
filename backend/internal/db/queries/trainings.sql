-- name: CreateTraining :one
INSERT INTO trainings (user_id, date, name, organizer, cost, url) VALUES (?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetTraining :one
SELECT * FROM trainings WHERE id = ? AND user_id = ?;

-- name: ListTrainings :many
SELECT * FROM trainings WHERE user_id = ? ORDER BY date DESC;

-- name: UpdateTraining :exec
UPDATE trainings SET date = ?, name = ?, organizer = ?, cost = ?, url = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?;

-- name: DeleteTraining :exec
DELETE FROM trainings WHERE id = ? AND user_id = ?;
