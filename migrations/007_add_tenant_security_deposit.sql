-- Migration 007: Tenants track a security deposit, defaulting to 0
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS security_deposit NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (security_deposit >= 0);
