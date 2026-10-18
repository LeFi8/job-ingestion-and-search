# Job Ingestion & Search

Go backend and a React SPA in `frontend/`. The API contract is `api/openapi.yaml`; Go server code and TS types are generated from it.

## Architecture

Data flows one way: `cmd/ingest` → `ingestion` (normalize + `approval.Engine`) → Postgres `postings` + `outbox` rows in one transaction → `cmd/worker` (`outbox.Relay`) → Elasticsearch. `cmd/api` (gin, `httpapi`) serves search from ES and job detail from Postgres.

- Postgres is the source of truth; ES is a derived read model. Never write to ES outside the outbox relay. Rebuild it with `mise run reindex`.
- ES writes use external versioning with the outbox id as the version, so stale or replayed writes return 409 and are treated as success (`search/bulk.go`).
- The worker writes through the `jobs` alias (`RequireAlias`) to the concrete index `jobs_v1` (`search/client.go`, `search/mapping.json`). A mapping change needs a new index name, not an in-place edit.
- `contentHash` covers the raw posting and `approval.PolicyVersion`, so re-ingesting the same file is a no-op. Bump `PolicyVersion` (`approval/policy.go`) whenever approval rules change.
- Outbox rows that keep failing are dead-lettered, not retried forever (`storage/outbox.go`).

## Commands

Tools are pinned in `mise.toml` (Go, Node). Run everything through mise or with `eval "$(mise env -s bash)"`. `mise` loads `.env`; copy `.env.example` if it is missing.

- Stack: `mise run compose` (Postgres + ES only), then `mise run migrate`, `mise run seed` (loads `data/sample.json`), `mise run dev` (api + worker + Vite). `mise run compose:all` runs migrate, seed, api and worker in Docker instead.
- Backend (in `backend/`):
  - `mise run test` — unit tests.
  - `mise run test:integration` — `-tags integration`, needs Docker (testcontainers via `storage/pgtest`, `search/estest`).
  - `golangci-lint run` — lint config enables the `integration` tag, so integration files are linted too.
  - `go generate ./...` — oapi-codegen only. sqlc is separate: `go tool sqlc generate`. Generated code is committed.
  - New migrations go in `backend/migrations/` (goose, embedded). CI lints them with squawk and checks `go tool sqlc vet` / `go tool sqlc diff`.
- Frontend (in `frontend/`): `npm run dev | build | typecheck | lint | test`; `npm run gen:api` after changing `api/openapi.yaml`. Vite proxies `/api` to `API_URL` (default `localhost:8080`).
- API spec lint: `npx @redocly/cli lint api/openapi.yaml` (from repo root).

## Conventions

- Backend packages are named by role (`ingestion`, `approval`, `storage`, `search`, `httpapi`, `outbox`); the technology goes in type or file names (`storage/postgres.go`, `langdetect.Lingua`).
- SQL lives in `backend/internal/storage/queries/*.sql`; wrap sqlc rows in `storage` types instead of leaking `sqlc` types to other packages.
- Config comes only from env vars via `internal/config`; a missing var is an error, never a default.
- Frontend: the URL is the only UI state (`routes/jobsSearch.ts`); server state lives in TanStack Query (`api/queries.ts`). API types come only from the generated `src/api/schema.d.ts`. The UI never re-implements search, filtering or sorting.
- Frontend tests run with `TZ=America/New_York` (set in `vite.config.ts`); date assertions depend on it.
- Pin exact dependency versions (`frontend/.npmrc` has `save-exact=true`).
- Tests mock only the network (MSW in the frontend, fakes at the store/search interfaces in Go). Never mock components, hooks or domain logic.

## Rules

- Never suppress lint or type checks: no `eslint-disable`, `@ts-ignore`, `@ts-expect-error`, `//nolint` or similar. Restructure the code so the rule passes; if that is impossible, stop and ask.
- Never edit generated files (`backend/internal/storage/sqlc/`, `backend/internal/httpapi/openapi.gen.go`, `frontend/src/api/schema.d.ts`); change the source and regenerate.
- Changing `api/openapi.yaml` means regenerating both sides: `go -C backend generate ./...` and `npm --prefix frontend run gen:api`.
- Before finishing, run the checks for the side you touched: backend `mise run test` + `golangci-lint run` (plus `mise run test:integration` when touching `storage`, `search` or `outbox`); frontend `npm run typecheck && npm run lint && npm test`.
