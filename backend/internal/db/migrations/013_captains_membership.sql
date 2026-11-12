-- Club-specific roles: a board-curated captain group, membership status
-- (statute: AGH students are members, other students sympathizers, everyone
-- else a guest), and the captain assigned to each trip/voyage.

ALTER TABLE users
    ADD COLUMN is_captain BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN membership TEXT NOT NULL DEFAULT 'czlonek'
        CHECK (membership IN ('czlonek', 'sympatyk', 'gosc'));

-- captain_name stays as the display snapshot (docs, imported history); the
-- API overwrites it with the user's name whenever captain_user_id is set.
ALTER TABLE trips
    ADD COLUMN captain_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE voyages
    ADD COLUMN captain_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN captain_notes TEXT;

CREATE INDEX idx_trips_captain_user_id ON trips(captain_user_id) WHERE captain_user_id IS NOT NULL;
CREATE INDEX idx_voyages_captain_user_id ON voyages(captain_user_id) WHERE captain_user_id IS NOT NULL;

-- Backfill: link free-text captain names that match exactly one user.
WITH unique_names AS (
    SELECT name, MIN(id) AS id FROM users GROUP BY name HAVING COUNT(*) = 1
)
UPDATE trips t SET captain_user_id = u.id
FROM unique_names u WHERE t.captain_name = u.name;

WITH unique_names AS (
    SELECT name, MIN(id) AS id FROM users GROUP BY name HAVING COUNT(*) = 1
)
UPDATE voyages v SET captain_user_id = u.id
FROM unique_names u WHERE v.captain_name = u.name;

UPDATE users SET is_captain = true
WHERE patent_number IS NOT NULL AND patent_number <> ''
  AND id IN (
    SELECT captain_user_id FROM trips WHERE captain_user_id IS NOT NULL
    UNION
    SELECT captain_user_id FROM voyages WHERE captain_user_id IS NOT NULL
);
