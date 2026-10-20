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

CREATE TABLE IF NOT EXISTS tenants (
    tenant_id BIGSERIAL PRIMARY KEY,
    tenant_initials VARCHAR(4) NOT NULL,
    property_id BIGINT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_number SMALLINT NOT NULL DEFAULT 1 CHECK (unit_number > 0),
    monthly_rent NUMERIC(12,2) NOT NULL CHECK (monthly_rent >= 0),
    pet_count SMALLINT NOT NULL DEFAULT 0 CHECK (pet_count >= 0),
    move_in_date TIMESTAMPTZ,
    move_out_date TIMESTAMPTZ,
    lease_start_date TIMESTAMPTZ,
    lease_end_date TIMESTAMPTZ,
    security_deposit NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (security_deposit >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- For existing databases, add lease dates
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS lease_start_date TIMESTAMPTZ;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS lease_end_date TIMESTAMPTZ;

-- For existing databases, add security deposit
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS security_deposit NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (security_deposit >= 0);

CREATE INDEX IF NOT EXISTS tenants_property_id_idx ON tenants(property_id);
