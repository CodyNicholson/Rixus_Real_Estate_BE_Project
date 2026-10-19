-- Migration 003: Drop password_hash as authentication relies on Cloudflare Access Google OAuth
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;

