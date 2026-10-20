-- Migration 004: Properties track their unit count instead of monthly rent
ALTER TABLE properties DROP COLUMN IF EXISTS monthly_rent;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS units SMALLINT NOT NULL DEFAULT 1 CHECK (units > 0);
