-- Migration 006: Tenants track an optional lease start and end date
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS lease_start_date TIMESTAMPTZ;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS lease_end_date TIMESTAMPTZ;
