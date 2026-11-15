-- Hidden drafts: a cruise, trip or voyage with a NULL published_at is
-- hidden from everyone but admins and the captain group. A trip or voyage
-- is visible only while it and its cruise are both published. New rows
-- start hidden; rows that exist now stay public.
ALTER TABLE cruises ADD COLUMN published_at TIMESTAMPTZ;
ALTER TABLE trips ADD COLUMN published_at TIMESTAMPTZ;
ALTER TABLE voyages ADD COLUMN published_at TIMESTAMPTZ;

UPDATE cruises SET published_at = COALESCE(created_at, CURRENT_TIMESTAMP);
UPDATE trips SET published_at = COALESCE(created_at, CURRENT_TIMESTAMP);
UPDATE voyages SET published_at = COALESCE(created_at, CURRENT_TIMESTAMP);
