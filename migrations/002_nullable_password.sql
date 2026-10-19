-- Migration 002: Make password_hash column nullable for Google OAuth users
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

