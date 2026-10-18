# Frontend

React SPA for searching jobs. The URL holds all UI state (query, country, sort, page, selected job); server state is cached with TanStack Query. API types are generated from `../api/openapi.yaml`.

Conventions and rules for the whole repo are in [`../CLAUDE.md`](../CLAUDE.md).

## Setup

Node is pinned in `../mise.toml`. From the repo root:

```bash
mise install
mise run compose && mise run migrate && mise run seed
mise run dev   # api + worker + Vite
```

Vite proxies `/api` to `API_URL` (default `http://localhost:8080`, set in `../.env`).

## Scripts

| Command                           | What it does                                                 |
| --------------------------------- | ------------------------------------------------------------ |
| `npm run dev`                     | Vite dev server                                              |
| `npm run build`                   | Type-check and build to `dist/`                              |
| `npm run typecheck`               | `tsc -b`                                                     |
| `npm run lint`                    | ESLint (type-checked rules)                                  |
| `npm test`                        | Vitest with MSW; runs in `TZ=America/New_York`               |
| `npm run format` / `format:check` | Prettier                                                     |
| `npm run gen:api`                 | Regenerate `src/api/schema.d.ts` after changing the API spec |
