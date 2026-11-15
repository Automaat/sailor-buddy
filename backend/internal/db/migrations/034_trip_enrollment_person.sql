-- A leg sign-up belongs to a person, not an account, so an admin can sign
-- up a person without an account. The account behind a sign-up is its
-- person's crew_members.user_id: it is not stored twice, so linking an
-- account to a person and merging people keep sign-ups consistent.
--
-- Every account has exactly one person, so each existing sign-up maps to
-- one. The migration stops when one does not, rather than drop the sign-up.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM trip_enrollments te
        WHERE NOT EXISTS (SELECT 1 FROM crew_members cm WHERE cm.user_id = te.user_id)
    ) THEN
        RAISE EXCEPTION 'trip enrollments: a sign-up''s account has no person; give every account its person before migrating';
    END IF;
END $$;

ALTER TABLE trip_enrollments
    ADD COLUMN crew_member_id BIGINT REFERENCES crew_members(id) ON DELETE CASCADE;

UPDATE trip_enrollments te SET crew_member_id = cm.id
FROM crew_members cm
WHERE cm.user_id = te.user_id;

ALTER TABLE trip_enrollments
    ALTER COLUMN crew_member_id SET NOT NULL,
    ADD CONSTRAINT trip_enrollments_trip_id_crew_member_id_key UNIQUE (trip_id, crew_member_id),
    DROP COLUMN user_id;

-- A person's sign-ups: merges move them and the caller's own sign-ups are
-- looked up through the person.
CREATE INDEX idx_trip_enrollments_crew_member_id ON trip_enrollments(crew_member_id);
