-- +goose Up
-- Table layout required by alexedwards/scs postgresstore.
CREATE TABLE sessions (
    token  text PRIMARY KEY,
    data   bytea       NOT NULL,
    expiry timestamptz NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions (expiry);

-- Lets deactivation / password reset delete every session of one user.
CREATE TABLE user_sessions (
    token      text PRIMARY KEY REFERENCES sessions (token) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL
);
CREATE INDEX user_sessions_user_idx ON user_sessions (user_id);

CREATE TABLE password_reset_tokens (
    id         uuid PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text        NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE password_reset_tokens;
DROP TABLE user_sessions;
DROP TABLE sessions;
