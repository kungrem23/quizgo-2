CREATE TABLE IF NOT EXISTS refresh_sessions (
    id UUID PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    secret_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS refresh_sessions_user_id_idx ON refresh_sessions(user_id);
CREATE INDEX IF NOT EXISTS refresh_sessions_expires_at_idx ON refresh_sessions(expires_at);
