-- Migration 005: Tenants belong to a property and occupy one of its units
CREATE TABLE IF NOT EXISTS tenants (
    tenant_id BIGSERIAL PRIMARY KEY,
    tenant_initials VARCHAR(4) NOT NULL,
    property_id BIGINT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_number SMALLINT NOT NULL DEFAULT 1 CHECK (unit_number > 0),
    monthly_rent NUMERIC(12,2) NOT NULL CHECK (monthly_rent >= 0),
    pet_count SMALLINT NOT NULL DEFAULT 0 CHECK (pet_count >= 0),
    move_in_date TIMESTAMPTZ,
    move_out_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS tenants_property_id_idx ON tenants(property_id);
