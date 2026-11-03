# Gatepass

An office visitor gate-pass system. Employees pre-register visitors or approve walk-ins, security photographs and checks visitors in and out, and admins manage users, settings and reports. Visitors never log in.

| Role | What they do |
|---|---|
| **Employee (host)** | Pre-registers a visit (name, date, duration), accepts or rejects walk-ins within 30 minutes, marks *Meeting Over* |
| **Security** | Registers walk-ins, photographs the visitor and ID, checks in, and checks out **only after the host marks Meeting Over** |
| **Admin** | Manages users and settings, views all visits and the audit log, exports CSV, force-closes visits |

## Stack

- **Back end:** Go 1.26, chi, GORM (pgx), goose migrations, server-side sessions in Postgres. A modular monolith with one module per folder under `backend/internal/modules`.
- **Front end:** React 19, TypeScript, Vite, Tailwind v4, TanStack Query, Zod.
- **Database:** PostgreSQL 16 (Docker). Dev email goes to MailHog.
- **Real-time:** Server-Sent Events (`/api/events`).

## Layout

```
backend/    Go API (cmd/, internal/modules/*, internal/platform/*, db/migrations)
frontend/   React web app (src/modules/*)
e2e/        Playwright browser tests
docs/       Requirements, spec, plan, tasks, test cases, testing guide
testdata/   Validation vectors shared by Go and web tests
```

## Prerequisites

- Go 1.26+
- Node.js 22+ and npm
- Docker with Docker Compose
- `make`

## Quick start (development)

```bash
cp .env.example .env        # defaults work for local development
make db-up                  # Postgres (port 5433) and MailHog (port 8025)
make seed                   # migrate and create the development users
make run                    # API on http://127.0.0.1:8080
```

In a second terminal:

```bash
make web-install
make web-dev                # web app on http://localhost:5173
```

Open <http://localhost:5173>.

### Development users

All use the password `Gatepass@123`. You are asked to change it at first sign-in.

| Email | Role |
|---|---|
| `admin@gatepass.local` | Admin |
| `empA@gatepass.local`, `empB@gatepass.local` | Employee |
| `guard@gatepass.local`, `guard2@gatepass.local` | Security |
| `exEmp@gatepass.local` | Employee, deactivated |

Emails (password reset and so on) appear in MailHog at <http://localhost:8025>.

## Tests

```bash
make test           # Go unit tests (no database needed)
make test-int       # Go integration tests (needs `make db-up`), race detector on
make web-test       # web unit tests
make web-typecheck  # web type check
make lint           # gofmt, go vet, module-boundary rules
make cover          # integration tests with a coverage report
```

Browser end-to-end tests need the API and web app running (see Quick start) and the seeded users:

```bash
cd e2e
npm install && npm run install-browser
npm test
```

These tests create extra `e2e.*` users and visits in the dev database. Run `make db-reset` for a clean slate.

## Production-style build

```bash
make build        # builds the web app, embeds it, produces bin/gatepass
./bin/gatepass    # serves the API and the web app together on HTTP_ADDR
```

Create the first admin on a fresh database:

```bash
./bin/gatepass create-admin --email you@company.com
```

Set a strong `SEED_PASSWORD`, real SMTP settings and `APP_ENV=production` for any shared deployment. Do **not** run `make seed` outside development.

## Useful commands

`make help` lists everything. Common ones: `db-up`, `db-down`, `db-reset`, `psql`, `migrate-status`, `run`, `build`.

## Configuration

See [.env.example](.env.example). The main settings are `DATABASE_URL` (runtime role `gatepass_app`), `MIGRATE_DATABASE_URL` (owner role, used by migrations), `STORAGE_DIR` (photo storage), `SMTP_*`, and `HTTP_ADDR`.

## Known limitations

- The login rate limit (10 per minute) counts by the connection's IP address. Behind a reverse proxy or office NAT, many users share one IP, so a trusted-proxy setting is needed before deployment.
- The employee mobile app (React Native / Expo) is planned and not built yet. Use the web app, which works in a phone browser.
- There is no OpenAPI spec. API types are hand-written on both sides.

## Documentation

Start with [docs/requirements.md](docs/requirements.md), then [docs/spec.md](docs/spec.md). Test cases are in [docs/test_cases.md](docs/test_cases.md).
