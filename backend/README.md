# Backend

Four Go binaries share one module: `migrate` applies the schema, `ingest` decodes, normalizes
and approves a feed into Postgres, `worker` relays the outbox into Elasticsearch, and `api`
serves search from Elasticsearch and job detail from Postgres.

This page covers what is specific to the backend: feed identity, the API, the packages, input
handling and how to add a market. Setting up and running the stack is in
[docs/development.md](../docs/development.md), the tests and CI in
[docs/testing.md](../docs/testing.md), the system flow in
[docs/architecture.md](../docs/architecture.md) and the policy choices in
[docs/decisions.md](../docs/decisions.md).

## Re-ingesting a feed

A posting's identity is its source plus company, title, location and posting date, compared
case- and whitespace-insensitively, with locations and dates compared after normalization.
Re-ingesting the same feed is a no-op. An edited salary, description or other field updates
the stored posting in place and re-runs approval. An edited title, location or posting date
creates a new posting, and the old one stays as it was. Identity uses normalized locations
and dates, so changing the country aliases, the US state list or date parsing changes the
identity of the postings they affect in the same way.

If one feed contains several elements with the same identity, the first one is stored and the
rest are logged as `job.duplicate_key` and counted as `duplicate`.

The source defaults to the file name without its extension. Pass `--source=<id>` when the
file name changes between runs (for example dated exports), or every run creates a new set
of postings.

## API

The contract is [`api/openapi.yaml`](../api/openapi.yaml) (OpenAPI 3.0.3). Requests are
validated against it before the handler runs.

`GET /api/jobs`:

| Param      | Rule                                                              | Default                                            |
| ---------- | ----------------------------------------------------------------- | -------------------------------------------------- |
| `q`        | string, at most 200 characters                                    | none                                               |
| `country`  | repeatable, `^[A-Z]{2}$`, at most 20                              | none                                               |
| `sort`     | `relevance`, `salary_desc`, `salary_asc`, `date_desc`, `date_asc` | `relevance` when `q` is set, otherwise `date_desc` |
| `page`     | 1–200                                                             | 1                                                  |
| `per_page` | 1–50                                                              | 20                                                 |

`page` × `per_page` is capped at 10,000, the Elasticsearch result window. Jobs without a
salary or date sort last in both directions.

`GET /api/jobs/{id}` returns one approved job with its full description, `region` and
`company_type`, read from Postgres ([why](../docs/architecture.md#system-flow)).

Errors are `application/problem+json` with `title` and `status`, plus `detail` on 400, 503 and
a missing job's 404:

| Status | When                                                                                                            |
| ------ | --------------------------------------------------------------------------------------------------------------- |
| 400    | A parameter fails validation; `detail` names it                                                                 |
| 404    | Unknown route, or no approved job with that id                                                                  |
| 405    | Wrong method; the response carries `Allow`                                                                      |
| 500    | A handler panicked, or a stored job cannot be rendered (bad salary unit or company type)                        |
| 503    | Elasticsearch (search) or Postgres (job detail) is down or slower than 5 s; the error is logged, never returned |

A client that disconnects mid-search gets no response and is logged at INFO with status 499.
`GET /healthz` returns 200 and is not part of the contract.

With the sample feed loaded:

```sh
curl -s 'localhost:8080/api/jobs' | jq '.total'                                  # 11
curl -s 'localhost:8080/api/jobs?q=enginer' | jq '[.items[].title]'              # typo-tolerant match on "Engineer"
curl -s 'localhost:8080/api/jobs?country=CA' | jq '.total, .facets.country'     # 2; facets still count US, CA and GB
curl -s 'localhost:8080/api/jobs?sort=salary_desc' | jq '[.items[].salary_usd_annual]'
curl -s 'localhost:8080/api/jobs?sort=date_asc' | jq '.items[-1].title'          # "Growth Marketing Manager" (no date)
curl -si 'localhost:8080/api/jobs?sort=bogus' | head -1                          # HTTP/1.1 400 Bad Request
curl -s "localhost:8080/api/jobs/$(curl -s localhost:8080/api/jobs | jq -r '.items[0].id')" | jq '.title, .company_type'
```

## Packages

Packages are named by their role in the pipeline (`ingestion`, `approval`, `storage`,
`search`, `outbox`, `httpapi`), never by the technology behind them. The technology goes into
file and type names instead (`storage/postgres.go`, `langdetect.Lingua`, `search/client.go`),
so callers import a role and replacing a library touches one package, not its importers. Each
role that talks to infrastructure is consumed through a small interface declared by its
caller (`outbox.Store`, `outbox.JobIndexer`, `httpapi.JobSearcher`, `httpapi.JobReader`), which is
also where the tests put their fakes.

The brief suggests `models`, `ingestion`, `approval` and `storage`. They map to:

| Brief       | Directory                                           | Role                                                                                       | Technology                                 |
| ----------- | --------------------------------------------------- | ------------------------------------------------------------------------------------------ | ------------------------------------------ |
| `models`    | `internal/posting`                                  | Domain types, issue codes, country, currency and enum lookup                               | stdlib, `shopspring/decimal`, `civil.Date` |
| `ingestion` | `internal/ingestion`                                | `Evaluate`, `Run` and the idempotency key                                                  | stdlib                                     |
|             | `internal/ingestion/raw`                            | Feed decoding and quarantine of malformed records                                          | `encoding/json`                            |
|             | `internal/ingestion/normalize`                      | Raw records to `posting.Posting` plus issues                                               | stdlib                                     |
| `approval`  | `internal/approval`                                 | Gates, market policies, `Evaluate`                                                         | stdlib                                     |
|             | `internal/fx`                                       | Static USD conversion rates                                                                | stdlib                                     |
|             | `internal/langdetect`                               | Language detection adapter                                                                 | `lingua-go`                                |
| `storage`   | `internal/storage`                                  | `Store`: posting upsert with outbox row, quarantine, outbox claiming and purge, job detail | Postgres, `pgx/v5`                         |
|             | `internal/storage/queries`, `internal/storage/sqlc` | SQL queries and the code generated from them (do not edit)                                 | `sqlc`                                     |
|             | `internal/storage/pgtest`                           | Integration-test helper that starts Postgres                                               | testcontainers-go                          |
|             | `internal/search`                                   | Index mapping, bulk indexing, search query and highlights                                  | Elasticsearch, `go-elasticsearch/v9`       |
|             | `internal/search/estest`                            | Integration-test helper that starts Elasticsearch                                          | testcontainers-go                          |

Everything else:

| Directory                                            | Role                                                                         | Technology                       |
| ---------------------------------------------------- | ---------------------------------------------------------------------------- | -------------------------------- |
| `internal/outbox`                                    | Relay: claims outbox rows, bulk-indexes them, retries and dead-letters       | stdlib                           |
| `internal/httpapi`                                   | Router, request validation, problem responses; `openapi.gen.go` is generated | gin, `oapi-codegen`, kin-openapi |
| `internal/config`                                    | Per-binary environment loaders                                               | stdlib                           |
| `internal/ptr`                                       | Pointer helpers                                                              | stdlib                           |
| `cmd/migrate`, `cmd/ingest`, `cmd/worker`, `cmd/api` | The four binaries                                                            | goose (`cmd/migrate`)            |
| `migrations`                                         | SQL migrations, embedded via `embed.go`                                      | goose                            |

## Input handling

The feed is scraped data, so one bad record never stops the rest:

- A non-object element, or a record with a wrong-typed field, is quarantined with its index
  and raw bytes; the other records continue.
- A non-numeric salary still ingests and is rejected with `salary_invalid`, as is any amount
  above 10,000,000 (`MaxSalaryAmount`).
- A file that is not one valid JSON array fails as a whole.
- NUL characters are dropped from text fields; the original record is kept as `bytea` in `raw`.

Not built: streaming decode for very large feeds. The whole file is read into memory.

## Adding the remote-UK market

Add one entry to `DefaultPolicy().Markets` in `internal/approval/policy.go`, **before**
`Remote-anywhere`. Markets are matched in order and the first match wins, so placed after it
the entry never matches.

```go
{Name: "Remote-UK", Matches: RemoteIn(posting.Country("GB")), MinAnnualUSD: decimal.RequireFromString("90000"), MinHourlyUSD: decimal.RequireFromString("45"), Languages: []posting.Language{posting.English}},
```

Then bump `PolicyVersion` in the same file and re-ingest the feed (`mise run seed`). The stored
content hash includes the policy version, so every posting is re-evaluated and re-enqueued,
and the worker reindexes them.
