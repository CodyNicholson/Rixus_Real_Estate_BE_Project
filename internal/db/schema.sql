CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    user_role TEXT NOT NULL DEFAULT 'VIEWER' CHECK (user_role IN ('ADMIN', 'VIEWER')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- For existing databases, drop password_hash as authentication relies on Google OAuth
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users ALTER COLUMN user_role SET DEFAULT 'VIEWER';

CREATE TABLE IF NOT EXISTS properties (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    street_address TEXT NOT NULL,
    city TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT '',
    zip TEXT NOT NULL DEFAULT '',
    units SMALLINT NOT NULL DEFAULT 1 CHECK (units > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- For existing databases, replace monthly_rent with units
ALTER TABLE properties DROP COLUMN IF EXISTS monthly_rent;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS units SMALLINT NOT NULL DEFAULT 1 CHECK (units > 0);

CREATE INDEX IF NOT EXISTS properties_user_id_idx ON properties(user_id);
