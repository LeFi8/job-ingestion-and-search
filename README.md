# job-ingestion-and-search

Ingests scraped job-posting feeds, approves each posting against a pay, location, language,
employment-type and company-type policy, and lets users search, filter and sort the approved
jobs in a web UI. Rejected postings are kept with their reasons.

![The search UI: job list on the left, the selected job on the right](docs/screenshots/list.png)

## Tech stack

Go (gin) · Postgres · Elasticsearch · React + TypeScript (Vite, TanStack Router and Query) ·
Docker Compose. One OpenAPI contract generates the Go server interface and the TypeScript
types. Why each was chosen: [decisions](docs/decisions.md#tech-stack).

## Architecture

- **`ingest`** reads a feed, normalizes and approves each posting, and writes it to Postgres
  together with an outbox row in one transaction.
- **Postgres** is the source of truth: postings, decisions, reasons, quarantined records and the
  outbox.
- **`worker`** relays the outbox into Elasticsearch: approved postings are indexed, rejected
  ones removed.
- **`api`** serves search from Elasticsearch and job detail from Postgres.
- **`web`** is the React app, talking to the API through one origin.

Diagram, limitations and the path to production: [architecture](docs/architecture.md).

## Repository layout

| Path | What |
| --- | --- |
| [`backend/`](backend/README.md) | Go module: ingestion, approval, worker and API |
| [`frontend/`](frontend/README.md) | React app: search, filter, sort and job detail |
| [`api/openapi.yaml`](api/openapi.yaml) | HTTP contract shared by backend and frontend |
| [`docs/`](docs/) | Architecture, decisions, development and testing |
| `data/seed.json` | The demo feed for the quick start |
| `data/sample.json` | The sample feed from the brief |
| `compose.yaml` | Postgres, Elasticsearch, the backend services and the UI |
| `mise.toml` | Pinned tool versions and task shortcuts |
| `.github/workflows/ci.yml` | CI for backend and frontend |
| [`CLAUDE.md`](CLAUDE.md) | Repo conventions and rules, also read by the AI coding assistant |

## Prerequisites

- Docker with Compose v2 and about 2 GB of free memory (Elasticsearch alone takes 512 MB of
  heap)
- Free ports 5173 (UI), 8080 (API), 5432 (Postgres) and 9200 (Elasticsearch)
- `curl` and `jq` for the check below (optional)

## Quick start

```sh
cp .env.example .env
docker compose --profile all up -d --build --wait
```

With [mise](https://mise.jdx.dev) installed, the same in one task:

```sh
cp .env.example .env
mise trust && mise run compose:all
```

The stack starts empty. Ingest the demo feed (`--rm` drops the one-off container; the summary
prints to the terminal):

```sh
docker compose run --rm ingest /data/seed.json
```

With mise: `mise run ingest:seed`. To also load the brief's sample feed, run the same with
`/data/sample.json` (`mise run ingest:sample`).

Then open <http://localhost:5173>.

`--profile all` runs `migrate` once, then keeps `worker`, `api` and `web` running; without
the profile only Postgres and Elasticsearch start. `ingest` has its own profile, so `up`
never runs it.

Stop with `docker compose --profile all down`. Postgres and Elasticsearch keep their data in
`./volumes/`, so the next `up` starts where you left off.

To run the services on your machine and work on the code, see
[development](docs/development.md).

## Documentation

| Document | Read it for |
| --- | --- |
| [docs/architecture.md](docs/architecture.md) | System flow, known limitations, what I'd do next |
| [docs/decisions.md](docs/decisions.md) | How the brief's open rules were decided, the tech stack, updates and duplicates, UI choices |
| [docs/development.md](docs/development.md) | Local setup, day-to-day commands, making changes, troubleshooting |
| [docs/testing.md](docs/testing.md) | What each test suite covers, how to run it, CI |
| [backend/README.md](backend/README.md) | API reference, packages, input handling, adding a market |
| [frontend/README.md](frontend/README.md) | Running the UI, scripts, regenerating API types |
