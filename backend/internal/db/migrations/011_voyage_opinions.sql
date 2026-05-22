-- Generated crew opinion documents, one per (voyage, crew member).

CREATE TABLE voyage_opinions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    voyage_id BIGINT NOT NULL REFERENCES voyages(id) ON DELETE CASCADE,
    crew_member_id BIGINT NOT NULL REFERENCES crew_members(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    file_format TEXT NOT NULL DEFAULT 'pdf',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_voyage_opinion_voyage_crew UNIQUE (voyage_id, crew_member_id)
);
