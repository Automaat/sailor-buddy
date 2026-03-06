CREATE TABLE crew_assignments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cruise_id BIGINT NOT NULL REFERENCES cruises(id) ON DELETE CASCADE,
    crew_member_id BIGINT NOT NULL REFERENCES crew_members(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'zalogant',
    patent_number TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cruise_id, crew_member_id)
);
