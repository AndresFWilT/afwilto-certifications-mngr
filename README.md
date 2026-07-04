# afwilto-certifications-mngr

Go + PostgreSQL backend for the Certifications Journey module: plan, organize,
track and complete professional certifications.

- **Spec:** `frontend/docs/specs/11-certifications-journey.md` (module) and
  `frontend/docs/specs/11-certifications-journey/slice-*.md` (per-slice).
- **Contract:** `api/openapi.yaml` — contract-first; handlers satisfy it and
  API tests validate conformance against it.
- **Architecture:** hexagonal per `backend/CLAUDE.md` §11.4 — pure domain in
  `internal/certification/domain`, use cases behind in-ports, pgx + disk
  adapters behind out-ports, composition root in `cmd/api/main.go`.

## Run

| Mode | Command |
|---|---|
| Platform | `docker compose up -d` from the lab root (service `certifications-mngr`, port 8082) |
| Dev | `make run` with `DATABASE_URL` pointing at a local Postgres |
| Tests | `make test` (repository integration tests need Docker for testcontainers) |

## Configuration (env)

| Var | Default | Purpose |
|---|---|---|
| `PORT` | `8082` | HTTP listen port |
| `DATABASE_URL` | — | Postgres DSN |
| `STORAGE_DIR` | `./data/uploads` | Attachment file storage (out-port adapter) |
| `ALLOW_ORIGINS` | shell + MFE localhost | CORS allowlist |
| `MAX_UPLOAD_BYTES` | `10485760` | Attachment size cap (413 beyond) |
| `MIGRATE_ON_START` | `true` | Run golang-migrate up at boot |
