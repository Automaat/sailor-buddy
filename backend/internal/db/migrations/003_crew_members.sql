CREATE TABLE crew_members (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    user_id BIGINT REFERENCES users(id),
    full_name TEXT NOT NULL,
    email TEXT,
    patent_number TEXT,
    phone TEXT,
    pzz_license_type TEXT,
    pzz_license_number TEXT,
    emergency_contact_name TEXT,
    emergency_contact_phone TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
