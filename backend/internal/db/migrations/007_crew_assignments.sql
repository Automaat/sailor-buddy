-- Crew assignments link a crew member to exactly one trip or one voyage.

CREATE TABLE crew_assignments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    trip_id BIGINT REFERENCES trips(id) ON DELETE CASCADE,
    voyage_id BIGINT REFERENCES voyages(id) ON DELETE CASCADE,
    crew_member_id BIGINT NOT NULL REFERENCES crew_members(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'zalogant',
    patent_number TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT crew_assignments_one_parent
        CHECK ((trip_id IS NULL) <> (voyage_id IS NULL))
);

CREATE UNIQUE INDEX crew_assignments_trip_member_uniq
    ON crew_assignments(trip_id, crew_member_id) WHERE trip_id IS NOT NULL;
CREATE UNIQUE INDEX crew_assignments_voyage_member_uniq
    ON crew_assignments(voyage_id, crew_member_id) WHERE voyage_id IS NOT NULL;
