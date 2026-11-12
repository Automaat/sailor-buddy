-- A club cruise is either a sea cruise (rejs morski) or an inland one (rejs
-- śródlądowy). Existing cruises are all sea cruises.

ALTER TABLE cruises
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'sea'
        CHECK (kind IN ('sea', 'inland'));
