# Frontend

React SPA for searching jobs. The URL holds all UI state (query, country, sort, page, selected job); server state is cached with TanStack Query. API types are generated from `../api/openapi.yaml`.

Conventions and rules for the whole repo are in [`../CLAUDE.md`](../CLAUDE.md).

## Running the UI

Setup and the full local stack are in [docs/development.md](../docs/development.md). With the
API already running on `:8080`, start only the UI on <http://localhost:5173>:

```bash
npm --prefix frontend ci
npm --prefix frontend run dev   # or: mise run web
```

Vite proxies `/api` to `API_URL` (default `http://localhost:8080`, set in `../.env`) in both
`npm run dev` and `npm run preview`, so the browser talks to one origin
([why](../docs/architecture.md#system-flow)). In the Docker quick start the `web` service
serves the built app with `vite preview`.

## Scripts

| Command                           | What it does                                                 |
| --------------------------------- | ------------------------------------------------------------ |
| `npm run dev`                     | Vite dev server                                              |
| `npm run build`                   | Type-check and build to `dist/`                              |
| `npm run preview`                 | Serve `dist/` with the `/api` proxy                          |
| `npm run typecheck`               | `tsc -b`                                                     |
| `npm run lint`                    | ESLint (type-checked rules)                                  |
| `npm test`                        | Vitest with MSW; runs in `TZ=America/New_York`               |
| `npm run format` / `format:check` | Prettier                                                     |
| `npm run gen:api`                 | Regenerate `src/api/schema.d.ts` after changing the API spec |

## Generated and copied code

`src/api/schema.d.ts` is generated from `../api/openapi.yaml` (`npm run gen:api`) and never edited by hand. `src/components/ui/` holds shadcn components, copied in with `npx shadcn add <component>` and edited like any other source. How to change the API contract, and every other generator, is in [docs/development.md](../docs/development.md#generating-code).

Tests and the CI job are described in [docs/testing.md](../docs/testing.md).
