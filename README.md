# Rixus Real Estate Backend
The backend for the Rixus Real Estate Project

## Running

### Option 1: Locally with Go & Postgres
Go REST API backed by Postgres.

    export DATABASE_URL=postgres://localhost:5432/rixus?sslmode=disable
    export JWT_SECRET=<at least 32 random characters>
    export CORS_ORIGIN=http://localhost:3000   # required; comma-separated list of exact origins, no "*"
    go run ./cmd/server

### Option 2: Running with Docker Compose (Recommended)
Spins up both the Go app and a Postgres container in one command:

    docker compose up --build

### Option 3: Building and Running Dockerfile Directly

    docker build -t rixus-api .
    docker run -p 8080:8080 \
      -e DATABASE_URL="postgres://host.docker.internal:5432/rixus?sslmode=disable" \
      -e JWT_SECRET="your-32-plus-char-secret" \
      rixus-api

Tables are created on startup (`internal/db/schema.sql`, mirrored in `migrations/`).

Endpoints: `GET /health`, `POST /api/login` (or `/api/auth/login`), `POST /api/register` (or `/api/auth/register`),
`POST /api/refresh` (or `/api/auth/refresh`), `POST /api/logout` (or `/api/auth/logout`),
`GET|POST /api/properties` (require a JWT in the `Authorization: Bearer <token>` header),
`GET /swagger/` (interactive Swagger UI API documentation — navigating to `/`, `/swagger`, or `/docs` automatically redirects here).

