-- The captain of a leg or voyage is a person (crew record), so a captain
-- without an account is a proper captain too. captain_user_id is replaced by
-- a join through the person: a captain's rights and sea time follow
-- crew_members.user_id, so they move with the person when it is linked to an
-- account, unlinked or merged.
--
-- The backfill takes the person of the account in captain_user_id. Where that
-- account has no person, captain_name stays as the display fallback.

ALTER TABLE trips
    ADD COLUMN captain_person_id BIGINT REFERENCES crew_members(id) ON DELETE SET NULL;

ALTER TABLE voyages
    ADD COLUMN captain_person_id BIGINT REFERENCES crew_members(id) ON DELETE SET NULL;

UPDATE trips t SET captain_person_id = cm.id
FROM crew_members cm
WHERE t.captain_user_id IS NOT NULL AND cm.user_id = t.captain_user_id;

UPDATE voyages v SET captain_person_id = cm.id
FROM crew_members cm
WHERE v.captain_user_id IS NOT NULL AND cm.user_id = v.captain_user_id;

ALTER TABLE trips DROP COLUMN captain_user_id;
ALTER TABLE voyages DROP COLUMN captain_user_id;

CREATE INDEX idx_trips_captain_person_id ON trips(captain_person_id) WHERE captain_person_id IS NOT NULL;
CREATE INDEX idx_voyages_captain_person_id ON voyages(captain_person_id) WHERE captain_person_id IS NOT NULL;
