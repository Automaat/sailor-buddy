-- Person link requests: a user asks to be linked to an existing person
-- without an account ("To ja"), so voyages recorded before the account
-- existed count for it. Linking waits for admin approval; approval merges
-- the account's own person into the requested one. An account has at most
-- one pending request. person_name keeps the name the user claimed, since
-- approval renames the person to the account's name. A merge of the
-- requested person moves its requests to the kept person.

CREATE TABLE person_link_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    person_id BIGINT NOT NULL REFERENCES crew_members(id) ON DELETE CASCADE,
    person_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    decided_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX person_link_requests_one_pending
    ON person_link_requests(user_id) WHERE status = 'pending';

CREATE INDEX idx_person_link_requests_person_id ON person_link_requests(person_id);
CREATE INDEX idx_person_link_requests_pending ON person_link_requests(created_at) WHERE status = 'pending';
