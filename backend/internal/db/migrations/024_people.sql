-- People: crew_members is the club's one list of people. Every account has
-- exactly one person (crew_members.user_id is unique); a person without an
-- account keeps user_id NULL. other_names holds alternative spellings and
-- maiden names that search matches.
--
-- The backfill never links an account to an existing person, even with a
-- matching e-mail: linking waits for admin approval. Each account without a
-- person gets a new one built from its name, e-mail, phone and patent.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM crew_members WHERE user_id IS NOT NULL
        GROUP BY user_id HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'people: an account is linked to more than one crew member; unlink the duplicates before migrating';
    END IF;
END $$;

ALTER TABLE crew_members
    ADD COLUMN other_names JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(other_names) = 'array'),
    ADD CONSTRAINT crew_members_user_id_key UNIQUE (user_id);

INSERT INTO crew_members (user_id, full_name, email, phone, patent_number)
SELECT u.id,
       COALESCE(NULLIF(btrim(u.name), ''), u.email),
       u.email, u.phone, u.patent_number
FROM users u
WHERE NOT EXISTS (SELECT 1 FROM crew_members cm WHERE cm.user_id = u.id)
ORDER BY u.id;
