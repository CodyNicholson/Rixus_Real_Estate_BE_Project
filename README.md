# Rixus_Real_Estate_BE_Project
The BE for the Rixus Real Estate Project

## Running
Go REST API backed by Postgres.

    export DATABASE_URL=******localhost:5432/rixus
    export JWT_SECRET=<at least 32 random characters>
    export CORS_ORIGIN=http://localhost:3000   # optional
    go run ./cmd/server

Tables are created on startup (`internal/db/schema.sql`, mirrored in `migrations/`).

Endpoints: `GET /health`, `POST /api/auth/register`, `POST /api/auth/login` (both return `{"token": ...}`),
`GET|POST /api/properties` (require a JWT in the `Authorization: Bearer <token>` header),
`GET /swagger/` (interactive Swagger UI API documentation).
