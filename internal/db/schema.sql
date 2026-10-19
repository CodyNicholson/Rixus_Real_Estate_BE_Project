CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT,
    name TEXT NOT NULL,
    user_role TEXT NOT NULL CHECK (user_role IN ('ADMIN', 'VIEWER')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ensure existing databases allow passwordless/OAuth users
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

CREATE TABLE IF NOT EXISTS properties (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    street_address TEXT NOT NULL,
    city TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT '',
    zip TEXT NOT NULL DEFAULT '',
    monthly_rent NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS properties_user_id_idx ON properties(user_id);
