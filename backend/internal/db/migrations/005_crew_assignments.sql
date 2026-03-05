CREATE TABLE crew_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cruise_id INTEGER NOT NULL REFERENCES cruises(id) ON DELETE CASCADE,
    crew_member_id INTEGER NOT NULL REFERENCES crew_members(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'zalogant',
    patent_number TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cruise_id, crew_member_id)
);
