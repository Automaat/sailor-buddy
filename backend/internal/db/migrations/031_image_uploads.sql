CREATE TABLE image_upload_requests (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    window_start TIMESTAMPTZ NOT NULL,
    requests BIGINT NOT NULL CHECK (requests > 0)
);

CREATE TABLE image_uploads (
    url TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('reserved', 'writing', 'ready', 'deleting')),
    owned BOOLEAN NOT NULL DEFAULT FALSE,
    byte_size BIGINT NOT NULL CHECK (byte_size >= 0 AND byte_size <= 5242880),
    retained BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (NOT retained OR state = 'ready')
);
CREATE INDEX image_uploads_abandoned ON image_uploads (user_id, created_at)
    WHERE NOT retained;
CREATE INDEX image_uploads_user ON image_uploads (user_id);
