# My guide: run and test Gatepass on my laptop

Everything below was checked against the current dev setup. Project root: `/home/coditas-admin/projects/Gatepass`.

## 1. Check what is already running

```bash
docker ps --format '{{.Names}} {{.Ports}}' | grep gatepass     # postgres on 5433, mailhog on 8025
curl -s http://127.0.0.1:8080/api/health                       # API up?
```

If either is down: `make db-up`, then `make run` (leave it running).

## 2. Look at the database in DBeaver

New connection, PostgreSQL:

| Field | Value |
|---|---|
| Host | `localhost` |
| Port | `5433` |
| Database | `gatepass` (tests use `gatepass_test`) |
| Username | `gatepass_owner` (full access) |
| Password | `gatepass_owner` (dev only; also in `.env`) |

The 12 tables already exist: `users`, `visitors`, `visits`, `visit_photos`, `notifications`, `audit_logs`, `settings`, `sessions`, `user_sessions`, `password_reset_tokens`, `visit_code_counters`, `goose_db_version`. To rebuild from scratch: `make db-reset`.

## 3. Start the web app

```bash
make web-install     # first time only
make web-dev         # http://localhost:5173
```

Mail inbox: http://localhost:8025.

## 4. Manual walk-through (about 15 minutes)

Use **two browser profiles or one normal and one private window**, so two people can be signed in at once. All passwords are `Gatepass@123`.

1. **First sign-in and password rule.** Sign in as `empA@gatepass.local`. You must change the password. Try a weak one (rejected), then a valid one.
2. **Planned visit.** As empA, create a planned visit for today (name, mobile, duration). Note the visit code.
3. **Check-in.** In the other window sign in as `guard@gatepass.local`, open the visit, capture visitor and ID photos (a laptop camera works; "choose file" is the fallback), enter the ID type and last 4 digits, check in.
4. **Check-out is blocked.** As guard, try to check out. It must be refused. As empA, press *Meeting Over*. Now guard can check out.
5. **Walk-in accepted.** As guard, register a walk-in for empA. empA's bell shows a notification **without reloading**. Accept it, and the guard screen updates live. Check in, then complete the visit.
6. **Walk-in rejected.** Repeat, but reject with a reason. Guard sees the rejection.
7. **Walk-in expiry.** Register another and leave it for 30 minutes. It should become expired. (To avoid waiting, see the clock trick in section 6.)
8. **Admin.** Sign in as `admin@gatepass.local`. Check the overview, the visits list and its audit trail, create a user, deactivate it, try an invalid setting, and download the CSV.
9. **Access control.** While signed in as empA, open `/admin`. You should see access denied. `exEmp@gatepass.local` should not be able to sign in.
10. **Database check in DBeaver.** `select visit_code, status from visits order by id desc;` and `select * from audit_logs order by seq desc limit 20;` should show every step you took.

## 5. Run the automated tests

```bash
make test            # Go unit tests
make test-int        # Go integration tests, uses gatepass_test (needs db-up)
make web-test        # 140 web tests
make web-typecheck
make lint
```

Browser tests (API and web app must be running):

```bash
cd e2e && npm install && npm run install-browser && npm test
```

Expected: 7 passed. Then `make db-reset` and `make seed` if you want a clean database again.

## 6. Test the 30-minute expiry without waiting

```bash
# stop the normal server, then:
make run-test        # APP_ENV=test enables POST /api/test/clock
```

Use that endpoint (see [docs/testing_guide.md](testing_guide.md)) to move the clock forward 31 minutes, then reload the walk-in. Switch back to `make run` afterwards.

## 7. Try it on my phone (same Wi-Fi)

The employee mobile app is not built yet. The web app does work in a phone browser:

```bash
cd frontend && npm run dev -- --host      # prints a Network URL such as http://192.168.x.x:5173
```

Open that URL on the phone. Notes:

- Phone and laptop must be on the same Wi-Fi, and the laptop firewall must allow port 5173.
- Camera capture in a browser needs HTTPS on a phone, so over plain `http://` use the "choose file" option instead.
- Employee screens (dashboard, walk-in approval, notifications) work well on a phone. Security check-in is meant for a desk or tablet.

## 8. Stop everything

```bash
make db-down         # stops Postgres and MailHog, keeps the data
# Ctrl+C in the terminals running `make run` and `make web-dev`
```

## If something goes wrong

| Symptom | Fix |
|---|---|
| "too many requests" at login | The limit is 10 per minute per IP. Wait a minute. |
| Port 5433 or 8080 in use | Change `GATEPASS_DB_PORT` or `HTTP_ADDR` in `.env`. |
| Login says wrong password after tests | Run `make db-reset`. |
| Web app shows network errors | The API is not running. Run `make run`. |
