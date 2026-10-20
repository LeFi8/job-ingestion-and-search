# Testing

Tests replace only what crosses a process boundary. Go tests use fakes at the store and
search interfaces, or start real Postgres and Elasticsearch in containers. UI tests replace
only the network. Nothing mocks the project's own components, hooks or domain logic.

## What each suite covers

| Suite | Runs against | Covers |
| --- | --- | --- |
| Go unit tests (`go test ./...`) | Pure functions and fakes | Raw decoding of every polymorphic field, normalization (salary units and currencies, locations, dates), each approval rule and its boundaries (exactly $100,000 fails), market policies, the idempotency key, the relay's retry and dead-letter logic against a fake store, the search query builder, handler responses and problem bodies |
| Go golden test (`internal/ingestion/evaluate_test.go`) | `testdata/pipeline_fixture.json` | Five feed records (four taken from the sample feed, one added), run through decode, normalization and approval and compared with `expected_decisions.json`: 3 approved, 2 rejected with their exact reasons |
| Go full-sample test (`TestSampleFeedDecisions`, same file) | `data/sample.json`, real lingua | All 20 sample records against `testdata/sample_decisions.json` (11 approved, 9 rejected, exact reason codes), plus a fingerprint of those decisions keyed by `PolicyVersion`, so a change to the sample feed's decisions without a version bump fails |
| Go integration tests (`-tags integration`) | Postgres and Elasticsearch started by testcontainers (`storage/pgtest`, `search/estest`) | Postgres: idempotent save with its outbox row, a policy-version bump re-evaluating, the approved-row invariant enforced by the schema, quarantine, job detail, a whole feed run including duplicate keys. Elasticsearch: the real mapping with typo-tolerant and prefix title search, highlights, country facets, sorting with missing values last, paging, external-version conflicts, writes only through the alias. Both: the relay indexing approved and deleting rejected postings, a crash before commit, poison documents dead-lettered, purging processed rows |
| UI tests (`npm test`) | React Testing Library, network replaced by MSW with fixed responses | The URL as state (hand-edited and stale URLs fall back to defaults, special characters round-trip, back/forward), the request parameters each control sends, debounced search, loading, empty, error and "no longer available" states, no retry on 4xx, paging past the end |

Search behaviour is tested once, in Go against a real Elasticsearch. The UI tests check what
the UI sends and how it renders the response, not search results, so the MSW handlers return
fixed pages and never filter or sort.

## Running them

| Suite | Command |
| --- | --- |
| Go unit tests | `mise run test` or `go -C backend test ./...` |
| Go unit + integration tests (need Docker) | `mise run test:integration` or `go -C backend test -tags integration ./...` |
| UI tests | `npm --prefix frontend test` |

UI tests run with `TZ=America/New_York` (set in `frontend/vite.config.ts`), so date assertions
also prove that a posting date never shifts by a day west of UTC.

## CI

`.github/workflows/ci.yml` runs on pushes and pull requests that touch `backend/`, `frontend/`,
`api/`, `compose.yaml`, `redocly.yaml` or the workflow.

| Job | Check | Local command |
| --- | --- | --- |
| `checks` | Go lint | `go -C backend run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run` |
| `checks` | Generated API server is current | `(cd backend && go generate ./... && git diff --exit-code && test -z "$(git status --porcelain)")` |
| `checks` | Queries match the schema | `go -C backend tool sqlc vet` |
| `checks` | Generated query code is current | `go -C backend tool sqlc diff` |
| `checks` | Migration lint | `(cd backend && npm exec --yes --package=squawk-cli@2.66.0 -- squawk migrations/*.sql)` |
| `checks` | OpenAPI lint | `npm exec --yes --package=@redocly/cli@2.57.0 -- redocly lint api/openapi.yaml` |
| `checks` | Compose file is valid (needs a `.env`) | `docker compose --profile all config --quiet` |
| `tests` | Go unit + integration tests | `go -C backend test -tags integration ./...` |
| `build` | Backend image builds | `docker build backend` |
| `frontend` | Generated API types are current | `(cd frontend && npm run gen:api && git diff --exit-code)` |
| `frontend` | Type check, lint, UI tests, build | `(cd frontend && npm run typecheck && npm run lint && npm test && npm run build)` |

The generated-code checks fail on any uncommitted change, so run them on a clean tree. The
`frontend` job pins Node 24.22.0 in the workflow, separately from `mise.toml`; bump both
together.
