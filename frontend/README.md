# Gatepass web app

React 19 + TypeScript (strict) + Vite + Tailwind v4, TanStack Query, React Router, Zod. Node 22.

## Scripts

| Command | What it does |
|---|---|
| `npm run dev` | Dev server on http://localhost:5173, proxies `/api` (incl. the event stream) to the Go API on `127.0.0.1:8080` (`VITE_API_TARGET` overrides) |
| `npm run typecheck` | `tsc --noEmit`, strict |
| `npm run lint` | ESLint, including the module-boundary rules |
| `npm test` | Vitest + Testing Library + MSW (also runs `testdata/validation_vectors.json` against the Zod schemas) |
| `npm run build` | typecheck + production build into `dist/` (`make build` embeds it in the Go binary) |

Browser tests live in [`../e2e`](../e2e) (Playwright; needs the API, `npm run dev` and `make seed`).

## Structure

```
src/app/       router (composes every module's routes), providers, app layout (header, nav, bell)
src/modules/   auth · visits · walkins · gate · notifications · users · reports · settings
               each: pages/ components/ hooks/ routes.tsx and index.ts (its only public entry)
src/shared/    api/ (client, DTO types, server time, time zone helpers) · auth/ (context, guards)
               ui/ (buttons, fields, badge, countdown, dialogs, toasts) · validation/ (Zod schemas)
src/test/      component tests
```

Rules, enforced by `npm run lint`: a module imports another module only through its `index.ts`; `shared/` imports no
module; only `app/` imports every module.

## How it talks to the API

- `src/shared/api/client.ts` is a thin `fetch` wrapper: cookie session (`credentials: 'include'`), the
  `X-Requested-With: gatepass` header on every request, errors as `ApiError` (`status`, `code`, `message`, per-field
  `fields`). A `401` anywhere clears the signed-in user and the guards redirect to `/login?returnTo=…`.
- Types in `src/shared/api/types.ts` are hand-written to match the Go DTOs (there is no OpenAPI file yet).
- Buttons on a visit come only from the server's `allowedActions`; screens never repeat the permission rules.
- Live updates: one `EventSource('/api/events')` while signed in. `visit-updated` only triggers a refetch of every
  query; `notification` updates the bell. After a reconnect everything is refetched.
- Countdowns use the server's `Date` response header as the clock, so a wrong device clock does not mislead.
- Times are shown in Asia/Kolkata.
