# Gatepass — Tasks

| Field   | Value                                                                      |
|---------|----------------------------------------------------------------------------|
| Version | 1.2 (module tags, architecture checks, audit recorder task)               |
| Date    | 2026-11-03                                                                  |
| Inputs  | [spec.md](spec.md), [plan.md](plan.md), [test_cases.md](test_cases.md), [testing_guide.md](testing_guide.md) |

**How to use:** tasks are grouped by phase from [plan.md](plan.md). The tag after the size (e.g. `` `visits` ``) is the module the work lives in (spec §3.2 back end, §3.4 web/mobile); `platform`, `app` and `infra` are shared, non-business code. Each task lists what it depends on, the requirements it covers, and the test cases that must pass for it to be done. Tick the box when the Definition of Done in plan §5 is met. Size: **S** ≤ ½ day, **M** ≈ 1 day, **L** ≈ 2 days.

---

## P0 — Project setup

- [ ] **T-001 · Repository scaffold** (S) · `infra`
  Layout from spec §3.4: `backend/` (`go mod init`) with `internal/{app,modules,platform,apigen,testutil}` and an empty `module.go` stub per module in spec §3.2; `frontend/src/{app,modules,shared}`; `api/openapi.yaml` + `api/modules/` (skeleton); `e2e/`, `testdata/`; `Makefile` targets `gen`, `lint`, `arch-lint`, `test`, `test-int`, `cover`, `test-e2e`, `build` (→ `bin/gatepass`), `run`; `.golangci.yml`; ESLint + Prettier for the web; `.editorconfig`.
- [ ] **T-002 · Docker Compose** (S) · `infra` · deps T-001
  Services `postgres`, `mailhog`. Named volumes for DB and photo storage. (Integration tests use testcontainers-go and do not need this DB.)
- [ ] **T-003 · Go server skeleton** (M) · `app · platform` · deps T-001
  `cmd/gatepass/main.go` (sub-commands) calling `internal/app` (composition root, spec §3.1); `platform/config` (caarlos0/env, fail fast); chi router assembled by `app.New(cfg, deps)` so tests can construct the whole app; middleware in `platform/httpx`: request ID, `slog` access log, panic recovery, security headers, CORS (`WEB_ORIGIN`), JSON body limit 100 KB; server timeouts and graceful shutdown on SIGTERM; `GET /api/health`.
  Tests: TC-SYS-001, TC-SYS-004
- [ ] **T-004 · Web skeleton** (S) · `web: app` · deps T-001
  Vite + React 19 + TS + Tailwind + React Router + TanStack Query; dev proxy `/api` (incl. `/api/events`) to the Go server.
- [ ] **T-005 · Test infrastructure** (M) · `infra` · deps T-003
  `backend/internal/testutil`: Postgres via testcontainers-go (started once per package with `TestMain`, migrations applied, tables truncated per test), `FakeClock`, in-memory `storage.Store`, fake `Mailer` recording messages, fixture builders, `LoginAs(t, role)` returning an `http.Client` with cookie jar; integration tests behind build tag `integration`; test-only `POST /api/test/clock` (registered only when `APP_ENV=test`); Vitest + RTL + MSW for the web; Playwright config with fake media stream flags.
- [ ] **T-006 · CI pipeline** (S) · `infra` · deps T-005
  GitHub Actions. Go job: `make gen` + `git diff --exit-code`, `gofmt`, `go vet`, golangci-lint, `make arch-lint`, `go test ./...`, `go test -tags=integration -race -coverprofile` with coverage threshold, `govulncheck`. Web job: lint, `tsc`, Vitest, build. Final job: build binary with embedded SPA.

- [ ] **T-007 · Architecture checks** (S) · `infra` · deps T-001
  `.go-arch-lint.yml` with the allowed module dependency graph (spec §3.3); `scripts/check-imports.sh` for the per-file rules; `eslint-plugin-boundaries` config for `frontend/` (and later `mobile/`); `make arch-lint` runs all of them and is part of `make lint` and CI.
  Tests: TC-ARCH-001, TC-ARCH-002

## P1 — Data model and authentication

- [ ] **T-100 · Database roles and local setup** (S) · `infra` · deps T-002
  `backend/db/init/00-roles.sql` (`gatepass_owner`, `gatepass_app`, database, role timeouts); Makefile `db-up`, `db-down`, `db-reset`, `psql`; `DATABASE_URL` and `MIGRATE_DATABASE_URL` in `.env.example`.
  Tests: TC-DBS-001
- [ ] **T-101 · Migrations** (L) · `platform/db` · deps T-100 · FR-9.2
  goose SQL migrations (embedded via `embed.FS`) for all tables, enums and indexes in spec §5.2 incl. `sessions`; default privileges for `gatepass_app` and `REVOKE UPDATE, DELETE ON audit_logs`; every migration has a `Down`; `gatepass migrate up|down|status|create` sub-command; migrations run at start-up with the owner URL; Makefile `migrate-*`.
  Tests: TC-DBS-002, TC-DBS-003, TC-AUD-003
- [ ] **T-101a · GORM setup and models** (M) · `platform/db` · deps T-101
  `platform/db`: open GORM with config G-11 (spec §5.4), slog logger bridge, pool settings, `WithTx(ctx, fn)` with after-commit callbacks, error mapping (G-12); GORM models with explicit tags and `TableName()` inside their owning module; repository interfaces in each module. CI lint rule: no `gorm` import outside `repository.go` / `platform/db`, no `AutoMigrate` anywhere.
  Tests: TC-DBS-004, TC-DBS-005
- [ ] **T-110 · Audit module: recorder** (S) · `audit` · deps T-101a · FR-9.1
  `modules/audit`: `Recorder` interface (`Record(ctx, tx, Entry)`) used inside other modules' transactions; owns `audit_logs`. The read endpoint comes in T-601.
  Covered by: TC-SM-016, TC-AUD-001
- [ ] **T-102 · Seed script** (S) · `app` · deps T-101a
  `gatepass seed` sub-command: fixture users from test_cases.md (emails `<key>@gatepass.local`, password `Gatepass@123`, no forced change) + settings row; prints the credentials; refuses to run when `APP_ENV=production`. `gatepass create-admin --email` for production (forced password change).
- [ ] **T-103 · Core libs** (S) · `platform` · deps T-003
  `platform/clock` (`Clock`: `Now()`, `Today()` in `Asia/Kolkata`), `platform/storage` (`Store`, disk, 0600 files), `platform/mailer` (go-mail), `platform/apperr` error values, `httpx.WriteError` mapping them to spec §8.1 format, JSON decode helper (unknown fields dropped, size-limited).
  Tests: TC-SYS-002, TC-SYS-003
- [ ] **T-104 · Sessions and login/logout/me** (M) · `auth` · deps T-101a, T-103 · FR-1.1, FR-1.3, FR-1.4
  `scs` with `postgresstore` (on GORM's `*sql.DB`), cookie `gp.sid` settings, `IdleTimeout` 30 min, `Lifetime` 12 h, `user_sessions` index for revocation; bcrypt cost 12; lockout after 5 failures; CSRF header middleware.
  Tests: TC-AUTH-001 … TC-AUTH-009
- [ ] **T-105 · Auth middleware** (S) · `auth` · deps T-104 · FR-1.2
  `RequireAuth`, `RequireRole(roles...)`, `PASSWORD_CHANGE_REQUIRED` gate; current user in request context.
  Tests: TC-AUTH-010, TC-AUTH-011
- [ ] **T-106 · Change / forgot / reset password** (M) · `auth` · deps T-104 · FR-1.5
  Tests: TC-AUTH-012 … TC-AUTH-017
- [ ] **T-107 · Rate limiting** (S) · `platform/httpx` · deps T-104 · NFR-5
  `httprate` per IP on login/forgot-password, per user on the rest of the API.
  Tests: TC-AUTH-018
- [ ] **T-108 · User management API** (M) · `users` · deps T-105, T-110 · FR-1.6
  CRUD, activate/deactivate (kills sessions), admin password reset, self-protection rules, audit entries.
  Tests: TC-USR-001 … TC-USR-009
- [ ] **T-109 · Employee search API** (S) · `users` · deps T-105 · FR-3.2
  Tests: TC-USR-010, TC-USR-011

## P2 — State machine and planned visits

- [ ] **T-201 · Contract, enums and validation** (L) · `contract · platform/validation` · deps T-003
  Full contract for spec §8: root `api/openapi.yaml` (shared schemas, error shape) + `api/modules/<module>.yaml`, bundled with `@redocly/cli`; `oapi-codegen` (types + strict chi server into `internal/apigen`) and `openapi-typescript` generation via `make gen`; enums in their owning modules; `platform/validation` custom rules (name, Indian mobile + normaliser, date window, duration step, reason length, ID fields); `testdata/validation_vectors.json`; Go tests that run the vectors.
  Tests: TC-VAL-001 … TC-VAL-012
- [ ] **T-202 · State machine (pure)** (M) · `visits` · deps T-201 · BR-2, BR-4, BR-8
  `modules/visits/statemachine.go`: transition table, `Check(visit, action, actor, now) error`, `AllowedActions(visit, actor, now) []Action`. No DB access; table-driven tests.
  Tests: TC-SM-001 … TC-SM-014
- [ ] **T-203 · Visit code generator** (S) · `visits` · deps T-101a
  Tests: TC-PV-010
- [ ] **T-204 · `visits.Service.Transition()`** (M) · `visits` · deps T-202, T-101a, T-110
  Inside `WithTx`: GORM conditional update `Where("id = ? AND status = ?")…Updates(map)` with `RowsAffected == 0` → `ErrInvalidState` (spec §5.4 G-7), version bump, audit row via `audit.Recorder` in the same transaction, events queued for after commit; lazy expiry helper.
  Tests: TC-SM-015, TC-SM-016
- [ ] **T-205 · Visitor upsert and lookup** (S) · `visitors` · deps T-201, T-101a · Q-5
  Tests: TC-PV-011, TC-PV-012
- [ ] **T-206 · Planned visit create/edit/cancel** (M) · `visits` · deps T-204, T-205, T-203 · FR-2.1–FR-2.6
  Tests: TC-PV-001 … TC-PV-009, TC-PV-013
- [ ] **T-207 · Visit list and detail** (M) · `visits` · deps T-206 · FR-2.8, FR-8.5, BR-9
  Role scoping, filters, pagination, `allowedActions`, timeline.
  Tests: TC-PV-014 … TC-PV-018

## P3 — Walk-ins and approvals

- [ ] **T-301 · Walk-in create** (M) · `visits` · deps T-206, T-109 · FR-3.1–FR-3.3
  Tests: TC-WI-001 … TC-WI-006
- [ ] **T-302 · Approve / reject** (M) · `visits` · deps T-301 · FR-3.4, FR-3.5, BR-3
  Tests: TC-WI-007 … TC-WI-013
- [ ] **T-303 · Security cancel of pending walk-in** (S) · `visits` · deps T-301 · FR-3.9
  Tests: TC-WI-014, TC-WI-015
- [ ] **T-304 · Re-request after reject/expire** (S) · `visits` · deps T-302 · FR-3.8
  Tests: TC-WI-016
- [ ] **T-305 · Concurrency tests for approvals** (S) · `visits` · deps T-302
  Tests: TC-WI-017, TC-WI-018

## P4 — Photos, check-in, Meeting Over, check-out

- [ ] **T-401 · Photo upload pipeline** (L) · `photos` · deps T-204 · FR-4.2, FR-4.3, FR-4.6, NFR-8
  `http.MaxBytesReader` (5 MB) + multipart parse, `http.DetectContentType` (JPEG/PNG), `image.DecodeConfig` pixel limit, decode with auto-orient, resize to 1280 px, JPEG re-encode stepping quality to ≤ 500 KB, replace on retake (old file deleted after commit), status guard. Go fuzz test (`FuzzProcessImage`) for the pipeline.
  Tests: TC-PH-001 … TC-PH-009
- [ ] **T-402 · Photo streaming with permissions** (S) · `photos` · deps T-401 · BR-10
  Tests: TC-PH-010 … TC-PH-013
- [ ] **T-403 · Check-in** (M) · `visits` · deps T-401, T-302 · FR-4.1, FR-4.4, FR-4.5, BR-1, BR-5
  Tests: TC-CI-001 … TC-CI-009
- [ ] **T-404 · Deny entry** (S) · `visits` · deps T-403 · FR-4.7
  Tests: TC-CI-010, TC-CI-011
- [ ] **T-405 · Meeting Over** (S) · `visits` · deps T-403 · FR-5.1, FR-5.2
  Tests: TC-MO-001 … TC-MO-004
- [ ] **T-406 · Check-out and force-close** (M) · `visits` · deps T-405 · FR-6.1, FR-6.2, FR-6.4, BR-4
  Tests: TC-CO-001 … TC-CO-008
- [ ] **T-407 · API-level happy paths** (S) · `app` · deps T-406
  Planned and walk-in flows through the API in one test each.
  Tests: TC-FLOW-001, TC-FLOW-002

## P5 — Notifications, real-time and jobs

- [ ] **T-501 · Notification service and endpoints** (M) · `notifications` · deps T-204 · FR-7.x
  Create row, push SSE event after commit, send email when catalogue says so in a background goroutine (retry ×3 with back-off, errors logged, never returned to caller).
  Tests: TC-NT-001 … TC-NT-010
- [ ] **T-502 · SSE hub and `/api/events`** (M) · `notifications` · deps T-104, T-501 · NFR-2
  Session-authenticated stream, subscribers by user and role, buffered channels with slow-client drop, `notification` and `visit-updated` events, 25 s ping, close on session expiry, clean unsubscribe on disconnect.
  Tests: TC-NT-011 … TC-NT-014
- [ ] **T-503 · Job runner** (S) · `platform/scheduler` · deps T-103
  `platform/scheduler`: `robfig/cron/v3` (location Asia/Kolkata), `Job` type, `pg_try_advisory_xact_lock` inside each job's transaction (spec §10), registration of every module's `Jobs()` in `internal/app`; job methods live in their modules and are callable directly from tests; graceful stop.
  Tests: TC-JOB-012
- [ ] **T-504 · Expiry and reminder jobs** (M) · `visits` · deps T-503, T-302 · FR-3.5, FR-7.2, NFR-3
  Tests: TC-JOB-001 … TC-JOB-004
- [ ] **T-505 · Overstay job** (S) · `visits` · deps T-503, T-403 · FR-5.4, FR-7.5
  Tests: TC-JOB-005, TC-JOB-006
- [ ] **T-506 · No-show job** (S) · `visits` · deps T-503 · BR-6
  Tests: TC-JOB-007, TC-JOB-008
- [ ] **T-507 · Retention jobs** (M) · `photos · visits` · deps T-503, T-401 · §10.2 of requirements
  Tests: TC-JOB-009 … TC-JOB-011
- [ ] **T-508 · Dashboards API** (M) · `visits` · deps T-207, T-505 · FR-8.1, FR-8.2, FR-6.3
  Tests: TC-DB-001 … TC-DB-004

## P6 — Admin, reports and audit

- [ ] **T-601 · Audit read endpoint** (S) · `audit` · deps T-204, T-110 · FR-9.1, FR-9.3
  `GET /visits/:id/audit` in the `audit` module (ADMIN only).
  Tests: TC-AUD-001, TC-AUD-002, TC-AUD-004
- [ ] **T-602 · Settings API** (S) · `settings` · deps T-101a
  Tests: TC-SET-001 … TC-SET-004
- [ ] **T-603 · CSV export** (M) · `reports` · deps T-207 · FR-8.3, FR-8.4
  Streaming CSV, formula-injection escaping (`=`, `+`, `-`, `@` prefixed with `'`).
  Tests: TC-RPT-001 … TC-RPT-004
- [ ] **T-604 · CSV user import** (M) · `users` · deps T-108 · FR-1.6 (Should)
  Tests: TC-USR-012, TC-USR-013

## P7 — Web shell and auth UI

- [ ] **T-701 · API client and query setup** (M) · `web: shared` · deps T-004, T-201
  In `frontend/src/shared/api`: `openapi-fetch` client on generated types; middleware adding `X-Requested-With`, `credentials: 'include'`; typed errors; server time offset from `Date` header; 401 → redirect with return URL.
- [ ] **T-702 · Layout, header, route guards** (M) · `web: app · shared` · deps T-701
  `frontend/src/app` router composing each module's `routes.tsx`; layout in `shared/layout`; role-based redirect from `/`; guards per route table spec §12.1.
  Tests: TC-UI-AUTH-004, TC-UI-AUTH-005
- [ ] **T-703 · Login / forgot / reset / change password pages** (M) · `web: auth` · deps T-702
  Tests: TC-UI-AUTH-001 … TC-UI-AUTH-003, TC-UI-AUTH-006
- [ ] **T-704 · Event stream client and notification bell** (M) · `web: notifications` · deps T-702, T-502
  `EventSource('/api/events')` after login, `onopen` after reconnect → refetch open queries; close on logout; unread badge; dropdown; mark read / all read.
  Tests: TC-UI-AUTH-007

## P8 — Employee and security UI

- [ ] **T-801 · Shared components** (M) · `web: shared` · deps T-702
  In `frontend/src/shared/ui`: `StatusBadge` (colour + text), `Countdown` (server-offset aware), `ConfirmDialog`, `VisitActions` (driven by `allowedActions`), `DataTable` with pagination, form field components with error display.
  Tests: TC-UI-EMP-006, TC-UI-SEC-008
- [ ] **T-802 · Planned visit form (create/edit)** (M) · `web: visits` · deps T-801
  Zod schema mirroring server rules (tested against `testdata/validation_vectors.json`), mobile lookup prefill, date picker limited to window.
  Also: Vitest suite that runs the shared validation vectors against the Zod schemas.
  Tests: TC-UI-EMP-001, TC-UI-EMP-002
- [ ] **T-803 · Employee dashboard and my visits** (M) · `web: visits` · deps T-801, T-508
  Approval cards with countdown + accept/reject, Meeting Over button, upcoming list, cancel.
  Tests: TC-UI-EMP-003 … TC-UI-EMP-005
- [ ] **T-804 · Security dashboard** (M) · `web: gate` · deps T-801, T-508
  Tabs with counts, search, real-time refresh, overstay highlight, one-click check-out with confirm.
  Tests: TC-UI-SEC-001, TC-UI-SEC-002, TC-UI-SEC-007
- [ ] **T-805 · Walk-in form** (M) · `web: walkins` · deps T-801
  Employee typeahead (debounced 300 ms, min 2 chars), mobile prefill.
  Tests: TC-UI-SEC-003
- [ ] **T-806 · Camera capture component** (L) · `web: gate` · deps T-801
  `getUserMedia` with rear-camera preference, preview, capture to JPEG, retake, permission-denied and no-device fallback to file input, stops tracks on unmount.
  Tests: TC-UI-SEC-004, TC-UI-SEC-005
- [ ] **T-807 · Check-in screen** (M) · `web: gate` · deps T-806, T-403, T-404
  Two capture slots, ID type + last 4, privacy notice, "Check in" enabled only when both photos saved, "Deny entry" with reason.
  Tests: TC-UI-SEC-006
- [ ] **T-808 · Visit detail page** (M) · `web: visits` · deps T-801
  Fields, permitted photos, timeline, actions.
  Tests: TC-UI-EMP-007

## P9 — Admin UI

- [ ] **T-901 · Users page** (M) · `web: users` · deps T-801, T-108, T-604
  Tests: TC-UI-ADM-001, TC-UI-ADM-002
- [ ] **T-902 · All visits, filters, CSV export, audit view, force-close** (M) · `web: reports` · deps T-801, T-603, T-601
  Tests: TC-UI-ADM-003, TC-UI-ADM-004
- [ ] **T-903 · Settings page** (S) · `web: settings` · deps T-801, T-602
  Tests: TC-UI-ADM-005

## PA — Store and push accounts (admin work, start day 1)

- [ ] **T-A01 · Google Play Console** (S) · `accounts`
  Developer account, app entry with package `com.<company>.gatepass`, internal testing track, tester list (Google Group or emails).
- [ ] **T-A02 · Apple Developer and App Store Connect** (S) · `accounts`
  Program membership, bundle ID, App Store Connect app entry, TestFlight internal testing group.
- [ ] **T-A03 · Expo / EAS** (S) · `accounts`
  Expo organisation, EAS project ID, access token for the server (`EXPO_ACCESS_TOKEN`, stored as a secret).
- [ ] **T-A04 · Push credentials** (S) · `accounts` · deps T-A01, T-A02, T-A03
  Firebase project + FCM v1 service-account key uploaded to EAS; APNs key generated in Apple Developer and uploaded to EAS.

## P11 — Mobile API support

- [ ] **T-1101 · Mobile sessions** (M) · `auth` · deps T-104 · FR-10.1–FR-10.3, NFR-15
  Second `scs` manager (7 d idle / 30 d absolute) on the same store; `X-Client: mobile` login returns token; bearer-token loading middleware; `MOBILE_NOT_ALLOWED` for SECURITY; CSRF check only for cookie requests; revocation on deactivation/password reset covers mobile sessions.
  Tests: TC-MAUTH-001 … TC-MAUTH-008
- [ ] **T-1102 · Device tokens** (M) · `notifications` · deps T-1101, T-101a
  Migrations for `device_tokens` and `push_tickets`; `POST /devices` (upsert, Expo token format check), `DELETE /devices/current`; logout deletes the session's tokens.
  Tests: TC-PUSH-001, TC-PUSH-002, TC-PUSH-008
- [ ] **T-1103 · Push sender and dispatch** (M) · `notifications` · deps T-1102, T-501 · FR-10.8, NFR-12
  `notifications.PushSender` interface; Expo HTTP implementation (batches ≤ 100, retry ×3, access token); fake implementation when `PUSH_ENABLED=false`; push for catalogue types; content builder (first name + last initial, no personal data), Android channel, `ttl` for approvals; tickets stored.
  Tests: TC-PUSH-003, TC-PUSH-004, TC-PUSH-006, TC-PUSH-007
- [ ] **T-1104 · Push receipts and device cleanup jobs** (S) · `notifications` · deps T-1103, T-503
  Tests: TC-PUSH-005, TC-PUSH-009
- [ ] **T-1105 · App config and version gate** (S) · `appconfig` · deps T-003 · FR-10.12
  `GET /app-config`; middleware returning `426 UPGRADE_REQUIRED` when `X-App-Version` < `MOBILE_MIN_VERSION`.
  Tests: TC-APPCFG-001, TC-APPCFG-002
- [ ] **T-1106 · Shared TS client package** (S) · `packages/client` · deps T-201, T-701
  Move generated types, `openapi-fetch` client and Zod schemas into `packages/client`; web imports from it; root npm workspaces include `mobile`.

## P12 — Mobile app

- [ ] **T-1201 · Expo project scaffold** (M) · `mobile: app · shared` · deps T-1106, T-A03
  `mobile/` with Expo (SDK pinned), module layout from spec §3.4 (`app/` thin route files, `src/modules/{auth,home,visits,notifications,profile}`, `src/shared/`), TypeScript strict, Expo Router, ESLint + `eslint-plugin-boundaries`; `app.config.ts` (`EXPO_PUBLIC_API_URL`, scheme `gatepass`, bundle/package IDs); `eas.json` profiles `development`, `preview`, `production`; Jest (`jest-expo`) + RNTL + MSW; `.maestro/` folder; Makefile `mobile-start`, `mobile-android`, `mobile-test`, `mobile-e2e`.
- [ ] **T-1202 · API layer and authentication** (M) · `mobile: auth · shared` · deps T-1201, T-1101
  Client with bearer token + `X-App-Version` + server time offset; token in `expo-secure-store`; sign-in, forced change password, logout; 401 → sign-in; SECURITY message.
  Tests: TC-MOB-001 … TC-MOB-003
- [ ] **T-1203 · Push integration** (L) · `mobile: notifications` · deps T-1202, T-1102
  Permission explanation + prompt, Expo push token → `POST /devices` on every start, Android channels, foreground banner + refetch, notification tap deep link (cold/background/foreground), logout clean-up.
  Tests: TC-MOB-008, TC-MOB-009, TC-MOB-013
- [ ] **T-1204 · Home screen** (M) · `mobile: home` · deps T-1202
  Approvals with countdown and Accept/Reject (confirm, optional reason), today's visitors with Meeting Over, upcoming; refetch on foreground; 15 s polling while visible.
  Tests: TC-MOB-004, TC-MOB-005, TC-MOB-006, TC-MOB-012
- [ ] **T-1205 · Visits: list, detail, planned visit form** (L) · `mobile: visits` · deps T-1202
  My visits with filter and pull-to-refresh; detail with timeline, `allowedActions`, visitor photo via `expo-image` with auth header (memory cache); create/edit/cancel form on shared Zod schema with native date picker and mobile prefill.
  Tests: TC-MOB-007, TC-MOB-014
- [ ] **T-1206 · Notifications and profile tabs** (S) · `mobile: notifications · profile` · deps T-1202
  Tests: TC-MOB-015
- [ ] **T-1207 · Offline, update-required, biometric lock** (M) · `mobile: shared · auth` · deps T-1204, T-1105
  Persisted query cache (no photos), NetInfo banner, mutations disabled offline; update-required screen; optional biometric lock (FR-10.13).
  Tests: TC-MOB-010, TC-MOB-011, TC-MOB-017
- [ ] **T-1208 · Accessibility pass** (S) · `mobile: all` · deps T-1204, T-1205
  Tests: TC-MOB-016
- [ ] **T-1209 · Maestro E2E flows** (M) · `mobile: .maestro` · deps T-1203 … T-1207
  Tests: TC-ME2E-001 … TC-ME2E-007
- [ ] **T-1210 · First preview build on a physical phone** (S) · `mobile` · deps T-1203, T-A04
  `eas build -p android --profile preview` pointed at the laptop API; install and smoke test per [testing_guide.md](testing_guide.md) §7.
  Tests: TC-DEV-001

## P10 — End-to-end, non-functional, hardening

- [ ] **T-1001 · Playwright E2E suites** (L) · `e2e` · deps P8, P9
  Tests: TC-E2E-001 … TC-E2E-010
- [ ] **T-1002 · Load test (k6)** (M) · `loadtest` · deps P6 · NFR-1
  Tests: TC-NFR-001
- [ ] **T-1003 · Real-time and expiry timing checks** (S) · `app` · deps P5 · NFR-2, NFR-3
  Tests: TC-NFR-002, TC-NFR-003
- [ ] **T-1004 · Accessibility scan** (S) · `web` · deps P9 · NFR-11
  Tests: TC-NFR-004
- [ ] **T-1005 · Security hardening and review** (M) · `all` · deps P6 · NFR-5
  CSP, headers, cookie flags, `govulncheck`, `npm audit`, longer fuzz run of the image pipeline, log scrubbing check.
  Tests: TC-SEC-001 … TC-SEC-008
- [ ] **T-1006 · Production build and deployment docs** (M) · `infra` · deps P9
  Multi-stage Dockerfile (Node stage builds SPA → Go stage builds static binary with `CGO_ENABLED=0` embedding SPA + migrations → distroless runtime), `README.md` (setup, env, backup of DB + photo volume), user guides per role (including installing the mobile app). Update [testing_guide.md](testing_guide.md) with the final commands and ports.
- [ ] **T-1008 · Mobile device testing** (M) · `mobile` · deps T-1210 · NFR-12, NFR-13
  Run the device checklist on at least one Android phone and one iPhone; measure push latency and cold start.
  Tests: TC-DEV-002 … TC-DEV-008, TC-NFR-005, TC-NFR-006
- [ ] **T-1009 · Mobile release builds** (S) · `mobile` · deps T-1008, T-A01, T-A02
  `production` EAS builds; `eas submit` to Google Play internal testing and TestFlight; set `MOBILE_MIN_VERSION`; invite UAT testers.
- [ ] **T-1007 · UAT support and sign-off** (S) · `—` · deps all
