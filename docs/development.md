# Development

How to run the stack on your machine, work on it, and check a change before pushing. Every
command runs from the repo root. To only see the system running, the Docker
[quick start](../README.md#quick-start) is enough.

## Tools

- Docker with Compose v2, for Postgres and Elasticsearch
- Go 1.27.2 and Node 24.22.0, both pinned in `mise.toml`
- [mise](https://mise.jdx.dev), optional: it installs the pinned tools, loads `.env` and wraps
  each step below as a task. Every task is one plain command, listed next to it.

```sh
cp .env.example .env
mise trust && mise install        # with mise
npm --prefix frontend ci
```

Without mise, export `.env` into each shell before running the plain commands:

```sh
set -a; . ./.env; set +a
```

`.env` holds host values (`localhost` URLs). The Compose services in the `all` profile
override only the hostnames, so the one file serves both setups. Keep `POSTGRES_PASSWORD`
URL-safe: Compose builds `DATABASE_URL` from it, and `@`, `/`, `:` or `#` break the URL.

## First run

| Step | mise | Plain |
| --- | --- | --- |
| Start Postgres and Elasticsearch | `mise run compose` | `docker compose up -d --wait` |
| Apply migrations | `mise run migrate` | `go -C backend run ./cmd/migrate` |
| Ingest the sample feed | `mise run seed` | `go -C backend run ./cmd/ingest ../data/sample.json` |
| Run worker, API and UI together | `mise run dev` | the three commands below, one terminal each |
| Index into Elasticsearch | `mise run worker` | `go -C backend run ./cmd/worker` |
| Serve the API on `:8080` | `mise run api` | `go -C backend run ./cmd/api` |
| Serve the UI on `:5173` | `mise run web` | `npm --prefix frontend run dev` |

Open <http://localhost:5173>. Vite proxies `/api` to `API_URL` (`http://localhost:8080`).
Check the backend on its own:

```sh
curl -s localhost:8080/healthz -o /dev/null -w '%{http_code}\n'   # 200
curl -s localhost:8080/api/jobs | jq .total                        # 11
```

Each binary reads only its own environment keys and exits listing every missing or invalid
one:

| Binary | Keys |
| --- | --- |
| `cmd/migrate`, `cmd/ingest` | `DATABASE_URL` |
| `cmd/worker` | `DATABASE_URL`, `ELASTICSEARCH_URL`, `WORKER_BATCH_SIZE` (1–10000), `WORKER_POLL_INTERVAL` |
| `cmd/api` | `API_ADDR`, `DATABASE_URL`, `ELASTICSEARCH_URL` |
| UI (`vite`) | `API_URL` |

## Day to day

- **Ingest another feed:** `go -C backend run ./cmd/ingest <file>`. Re-running the same file
  changes nothing; see [re-ingesting a feed](../backend/README.md#re-ingesting-a-feed).
- **Rebuild the search index** from Postgres: `mise run reindex`
  (`go -C backend run ./cmd/ingest --reindex`), with the worker running. Needed after
  Elasticsearch lost its data or after a mapping change.
- **Inspect the database:** `docker compose exec postgres psql -U jobs jobs`, for example
  `select title, status, reasons from postings;`.
- **Migration status:**
  `mise exec -- sh -c 'cd backend && go tool goose -dir migrations postgres "$DATABASE_URL" status'`.
- **Switch between the Docker stack and the host stack:** stop one before starting the other;
  both use ports 8080 and 5173. They share the data in `./volumes/`.

## Commands

Run from the repo root. `mise tasks` lists the mise shortcuts.

| Task | Backend | Frontend |
| --- | --- | --- |
| Unit tests | `mise run test` (`go -C backend test ./...`) | `npm --prefix frontend test` |
| Integration tests (need Docker) | `mise run test:integration` (`go -C backend test -tags integration ./...`) | |
| Lint | `go -C backend run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run` | `npm --prefix frontend run lint` |
| Format | same command with `fmt` instead of `run` (gofmt + goimports) | `npm --prefix frontend run format` |
| Type check | `go -C backend vet ./...` | `npm --prefix frontend run typecheck` |
| Build | `go -C backend build ./...` | `npm --prefix frontend run build` |

The Go linter config (`backend/.golangci.yml`) enables the `integration` build tag, so
integration test files are linted too. golangci-lint is fetched at its pinned version on first
run. `sqlc`, `goose` and `oapi-codegen` are `tool` entries in `backend/go.mod` and run with
`go tool <name>`; there is nothing to install separately. Lint and type errors are fixed in
the code, never suppressed with `//nolint`, `eslint-disable`, `@ts-ignore` or `squawk-ignore`.

## Generating code

| What | Command | Writes | When |
| --- | --- | --- | --- |
| A new migration file | `go -C backend tool goose -s -dir migrations create <name> sql` | `backend/migrations/0000N_<name>.sql` | Schema change |
| Query code | `go -C backend tool sqlc generate` | `backend/internal/storage/sqlc/` | After editing a migration or `internal/storage/queries/*.sql` |
| API server and models | `go -C backend generate ./...` | `backend/internal/httpapi/openapi.gen.go` | After editing `api/openapi.yaml` |
| API types for the UI | `npm --prefix frontend run gen:api` | `frontend/src/api/schema.d.ts` | After editing `api/openapi.yaml` |
| A UI component | `cd frontend && npx shadcn add <component>` | `frontend/src/components/ui/` | A new primitive (dialog, tabs, …) |

Generated code is committed and never edited by hand; CI fails when any of it is stale. shadcn
components are the exception: they are copied into the repo as source, so they are ours to
edit and must pass the same lint and type checks as the rest of the code.

## Making changes

**A migration.** Create the file with the goose command above (`-s` keeps the sequential
numbering), fill in `-- +goose Up` (delete the `Down` section; migrations are forward-only),
start it with `SET lock_timeout` and `SET statement_timeout`, apply it with `mise run migrate`, and
regenerate the query code. Lint it before pushing:
`(cd backend && npm exec --yes --package=squawk-cli@2.66.0 -- squawk migrations/*.sql)`.
Migrations are forward-only and run as their own step, never at application start, so a
breaking change goes in stages: add the new shape, move the code over, then drop the old one in
a later migration. Nothing is released yet, so `00001_init.sql` is still edited in place; after
pulling a change to it, delete `./volumes/postgres` and run `mise run migrate` again.

**A query.** Edit `backend/internal/storage/queries/*.sql`, run
`go -C backend tool sqlc generate`, and check it with
`go -C backend tool sqlc vet && go -C backend tool sqlc diff`. Return `storage` types from the
store; the generated `sqlc` types stay inside the `storage` package.

**The API contract.** Edit `api/openapi.yaml`, then regenerate both sides and lint the spec:

```sh
go -C backend generate ./...
npm --prefix frontend run gen:api
npm exec --yes --package=@redocly/cli@2.57.0 -- redocly lint api/openapi.yaml
```

**Approval rules.** Each rule is a function in `backend/internal/approval/rules.go`, listed
in order by `DefaultRules`. Bump `PolicyVersion` in `backend/internal/approval/policy.go` after
any change that can move a decision: `internal/approval`, the FX table in `internal/fx`,
`internal/ingestion/normalize` or `internal/langdetect`. The version is part of each posting's
content hash, so the next ingest of a feed re-evaluates every posting in it instead of skipping
them as unchanged. `TestSampleFeedDecisions` fails when the sample feed's decisions change
without a bump; add the new version's fingerprint to
`backend/internal/ingestion/testdata/sample_decisions.json` and never edit an existing one. Adding a market is a policy entry; see
[adding the remote-UK market](../backend/README.md#adding-the-remote-uk-market).

**The search mapping.** Mappings cannot change in place. Edit
`backend/internal/search/mapping.json`, give the index a new name (`indexName` in
`backend/internal/search/client.go`, for example `jobs_v2`), delete the old index locally
(`curl -X DELETE localhost:9200/jobs_v1`), restart the worker (it creates the new index
behind the `jobs` alias) and run `mise run reindex`.

**A dependency.** Pin exact versions on both sides. Go: `go -C backend get <module>@<version>`
then `go -C backend mod tidy`; a new code-generation tool goes in with
`go -C backend get -tool <module>@<version>`. Frontend: `npm --prefix frontend install <package>`
(`frontend/.npmrc` sets `save-exact`). Commit `go.sum` or `package-lock.json` with it.

Repo conventions and rules are in [CLAUDE.md](../CLAUDE.md).

## Before you push

Run the checks for the side you touched:

- **Backend:** unit tests, lint, and the integration tests when you changed `storage`,
  `search` or `outbox`.
- **Frontend:** `npm --prefix frontend run typecheck && npm --prefix frontend run lint && npm --prefix frontend test`.
- **Either:** regenerate whatever the [table above](#generating-code) says your change needs.

What each suite covers, and the full list of CI checks, is in [testing](testing.md).

## Troubleshooting

- **`mise ERROR … not trusted`:** run `mise trust` once in a fresh clone.
- **Compose fails with "port is already allocated":** something else uses 5432, 9200, 8080 or
  5173. Stop it, or the other copy of this stack.
- **Search returns 0 jobs but the seed says `unchanged=20`:** the index is missing while
  Postgres still has the postings (for example after deleting `./volumes/elasticsearch`). Run
  `mise run reindex`.
- **`set POSTGRES_PASSWORD in .env`:** `.env` is missing; copy it from `.env.example`.
- **The UI shows "Search is temporarily unavailable":** the API cannot reach Elasticsearch;
  check `docker compose ps`.
