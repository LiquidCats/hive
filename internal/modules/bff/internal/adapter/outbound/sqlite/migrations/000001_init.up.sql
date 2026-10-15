CREATE TABLE IF NOT EXISTS users (
    id blob PRIMARY KEY,
    name varchar(255) not null UNIQUE,
    created_at timestamp not null default CURRENT_TIMESTAMP,
    disabled_at timestamp default NULL
);

CREATE TABLE IF NOT EXISTS user_credentials (
    id blob PRIMARY KEY,
    user_id blob not null REFERENCES users(id),
    -- base64url WebAuthn credential ID, the key authenticators send back on login
    credential_id text not null UNIQUE,
    credential text not null,
    created_at timestamp not null default CURRENT_TIMESTAMP,
    revoked_at timestamp default NULL
);

CREATE INDEX IF NOT EXISTS user_credentials_user_id ON user_credentials(user_id);

-- In-flight registrations and logins, keyed by the challenge the client echoes back.
-- No user FK: a registering user does not exist yet, a passkey login does not know its user yet.
CREATE TABLE IF NOT EXISTS ceremonies (
    id blob PRIMARY KEY,
    challenge text not null UNIQUE,
    type varchar(255) not null,
    username varchar(255) not null default '',
    session text not null,
    created_at timestamp not null default CURRENT_TIMESTAMP
);

-- Server-side sessions: logout and disable revoke immediately.
CREATE TABLE IF NOT EXISTS sessions (
    -- hex sha256 of the cookie token; the raw token is never stored
    token_hash text PRIMARY KEY,
    user_id blob not null REFERENCES users(id),
    -- unix seconds, compared numerically
    expires_at integer not null,
    created_at timestamp not null default CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS sessions_user_id ON sessions(user_id);
