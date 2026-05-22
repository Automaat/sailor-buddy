-- Cruise enrollments: members signing up at the cruise level; an admin later
-- assigns each accepted enrollment to a specific trip within the cruise.

CREATE TABLE cruise_enrollments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cruise_id BIGINT NOT NULL REFERENCES cruises(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trip_id BIGINT REFERENCES trips(id) ON DELETE SET NULL,
    note TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cruise_id, user_id)
);

CREATE INDEX idx_cruise_enrollments_trip_id
    ON cruise_enrollments(trip_id) WHERE trip_id IS NOT NULL;
