# Gatepass — Test Cases

| Field   | Value                                                                  |
|---------|------------------------------------------------------------------------|
| Version | 1.2 (adds architecture-rule cases, §29)                                |
| Date    | 2026-11-03                                                              |
| Inputs  | [requirements.md](requirements.md), [spec.md](spec.md), [tasks.md](tasks.md) |

## How to read this document

- **Level:** `U` Go unit test (`go test`, no DB), `I` Go API integration test (`go test -tags=integration -race`, `httptest` + real Postgres via testcontainers), `C` front-end component (Vitest + React Testing Library + MSW), `E` end-to-end (Playwright), `ME` mobile end-to-end (Maestro), `M` manual on a physical phone, `N` non-functional.
- Go tests are table-driven where a case lists several inputs; each input is a named sub-test (`t.Run`).
- **Priority:** `P1` blocks release · `P2` must pass before UAT · `P3` nice to have.
- **Ref:** requirement or spec section the case proves.
- **Time:** all time-based cases use `FakeClock` (unit/integration) or the test-only clock endpoint `POST /api/test/clock` (E2E; registered only when `APP_ENV=test`). Times are IST unless marked `Z`.

### Standard fixtures (seeded per test file)

Email = `<key>@gatepass.local`, password `Gatepass@123` (seed data, development and test only).

| Key      | User                       | Role     |
|----------|----------------------------|----------|
| `admin`  | Asha Admin                 | ADMIN    |
| `empA`   | Anita Desai                | EMPLOYEE |
| `empB`   | Bharat Mehta               | EMPLOYEE |
| `guard`  | Suresh Guard               | SECURITY |
| `guard2` | Vikram Guard               | SECURITY |
| `exEmp`  | Old Employee (inactive)    | EMPLOYEE |

Default visitor: `Ravi Kumar`, mobile `9876543210`, company `Acme`. Default "now": **2026-11-03 10:00 IST**. Test images: `visitor.jpg` (1.2 MB, with GPS EXIF), `id.png` (800 KB), `big.jpg` (6 MB), `fake.jpg` (text file), `anim.gif`, `bomb.png` (20 000 × 20 000).

---

## 1. System (TC-SYS)

| ID         | Lvl | Scenario                          | Steps / data                                                    | Expected                                                                                           | Ref       | P  |
|------------|-----|-----------------------------------|-----------------------------------------------------------------|----------------------------------------------------------------------------------------------------|-----------|----|
| TC-SYS-001 | I   | Health check                      | `GET /api/health` with DB up; then with DB stopped              | 200 `{status:"ok"}`; with DB down 503                                                              | spec §8.2 | P2 |
| TC-SYS-002 | U   | "Today" uses office time zone     | Clock at `2026-11-03T18:31Z`                                     | `today()` = `2026-11-04`                                                                            | BR-11     | P1 |
| TC-SYS-003 | I   | Error format                      | Unknown route; route that throws unexpected error               | 404 `NOT_FOUND`; 500 `INTERNAL_ERROR` with no stack trace in body; error logged with request ID     | spec §8.1 | P2 |
| TC-SYS-004 | I   | Graceful shutdown                 | Start a slow request and an open event stream; send SIGTERM     | In-flight request completes; event stream closed; running job finishes; process exits 0 within 15 s | spec §13  | P2 |

## 2. Authentication (TC-AUTH)

| ID          | Lvl | Scenario                               | Steps / data                                                                  | Expected                                                                                          | Ref        | P  |
|-------------|-----|----------------------------------------|-------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------|------------|----|
| TC-AUTH-001 | I   | Valid login                            | `POST /auth/login` empA correct password                                      | 200 with user (no password hash); `gp.sid` cookie `HttpOnly`, `SameSite=Lax`                       | FR-1.1     | P1 |
| TC-AUTH-002 | I   | Email is case-insensitive              | Login with `ANITA@…` upper case                                                | 200                                                                                               | spec §7    | P2 |
| TC-AUTH-003 | I   | Wrong password                         | Correct email, wrong password                                                  | 401 `Invalid email or password`                                                                    | FR-1.1     | P1 |
| TC-AUTH-004 | I   | Unknown email                          | Email not in DB                                                                | 401 with the **same** message as 003                                                               | NFR-5      | P1 |
| TC-AUTH-005 | I   | Inactive user                          | Login as exEmp with correct password                                           | 401 same generic message                                                                           | FR-1.6     | P1 |
| TC-AUTH-006 | I   | Lockout                                | 5 wrong passwords, then correct password; advance clock 15 min, retry           | 6th attempt 423 `ACCOUNT_LOCKED`; after 15 min 200; failure counter reset to 0                     | spec §7    | P1 |
| TC-AUTH-007 | I   | Logout                                 | Login, `POST /auth/logout`, then `GET /auth/me`                                | 204, then 401                                                                                      | FR-1.4     | P1 |
| TC-AUTH-008 | I   | Idle timeout                           | Login; advance 29 min, call API; advance 29 min, call; advance 31 min, call     | First two 200 (rolling); last 401                                                                  | FR-1.3     | P1 |
| TC-AUTH-009 | I   | CSRF header required                   | Authenticated `POST /visits/planned` without `X-Requested-With`                | 403 `FORBIDDEN`; GET requests unaffected                                                           | spec §7    | P1 |
| TC-AUTH-010 | I   | Role guard                             | No session → `GET /visits`; empA → `POST /visits/walk-in`                       | 401 `UNAUTHENTICATED`; 403 `FORBIDDEN`                                                             | FR-1.2     | P1 |
| TC-AUTH-011 | I   | Forced password change                 | User with `must_change_password` calls `GET /visits`, `GET /auth/me`           | `/visits` 403 `PASSWORD_CHANGE_REQUIRED`; `/auth/me` 200 with `mustChangePassword:true`            | spec §7    | P1 |
| TC-AUTH-012 | I   | Change password                        | Correct current, new `NewPass123`                                              | 200; flag cleared; login with new password works, old fails                                        | spec §7    | P1 |
| TC-AUTH-013 | I   | Change password invalid                | Wrong current; new `short1`; new `allletters`                                  | 400 `VALIDATION_ERROR` with field errors                                                           | spec §7    | P2 |
| TC-AUTH-014 | I   | Forgot password                        | Known email; unknown email                                                     | Both 202; only known email receives a reset mail                                                   | FR-1.5     | P2 |
| TC-AUTH-015 | I   | Reset password                         | Use emailed token; use same token again                                        | 200 and new password works; second use 400                                                         | FR-1.5     | P2 |
| TC-AUTH-016 | I   | Expired reset token                    | Advance clock 31 min, then reset                                               | 400                                                                                               | FR-1.5     | P2 |
| TC-AUTH-017 | I   | Reset side effects                     | Locked user with 2 sessions resets password                                    | Lock cleared; both old sessions 401                                                                | spec §7    | P3 |
| TC-AUTH-018 | I   | Login rate limit                       | 11 login requests within 1 min from one IP                                     | 11th 429 `RATE_LIMITED`                                                                            | NFR-5      | P2 |

## 3. User management (TC-USR)

| ID         | Lvl | Scenario                          | Steps / data                                                       | Expected                                                                                      | Ref     | P  |
|------------|-----|-----------------------------------|--------------------------------------------------------------------|-----------------------------------------------------------------------------------------------|---------|----|
| TC-USR-001 | I   | Create user                       | Admin `POST /users` valid EMPLOYEE                                 | 201; `mustChangePassword:true`; audit `USER_CREATED`                                           | FR-1.6  | P1 |
| TC-USR-002 | I   | Duplicates                        | Same email in different case; same employee code                   | 409 `DUPLICATE` for both                                                                      | FR-1.6  | P2 |
| TC-USR-003 | I   | Non-admin blocked                 | empA and guard call `POST /users`, `GET /users`                    | 403                                                                                           | FR-1.2  | P1 |
| TC-USR-004 | I   | Validation                        | role `SUPERUSER`; email `abc`                                      | 400 with field errors                                                                         | FR-1.6  | P2 |
| TC-USR-005 | I   | Update user                       | Admin changes empB department and role; admin changes own role      | First 200 + audit; own role change 403                                                         | FR-1.6  | P2 |
| TC-USR-006 | I   | Deactivate                        | empB logged in; admin deactivates empB; empB calls API, then logs in | empB's next call 401; login 401; empB not returned by employee search                         | FR-1.6  | P1 |
| TC-USR-007 | I   | Cannot deactivate self            | Admin deactivates own account                                       | 403                                                                                           | spec §8.2 | P2 |
| TC-USR-008 | I   | Reactivate                        | Activate exEmp; login                                               | 200                                                                                           | FR-1.6  | P2 |
| TC-USR-009 | I   | Admin password reset              | Admin sets temp password for empA                                   | Old password fails; new one works; `mustChangePassword:true`                                   | FR-1.6  | P2 |
| TC-USR-010 | I   | Employee search                   | guard `GET /employees/search?q=an`                                  | Returns empA (and admin if matching); never guard/exEmp; ≤ 10 results; only `id,name,employeeCode,department` | FR-3.2 | P1 |
| TC-USR-011 | I   | Employee search guards            | `q=a`; empA calls the endpoint                                      | 400; 403                                                                                      | FR-3.2  | P2 |
| TC-USR-012 | I   | CSV import valid                  | CSV with 3 valid rows                                                | 3 users created; per-row result `created`                                                       | FR-1.6  | P3 |
| TC-USR-013 | I   | CSV import mixed                  | Rows: valid, duplicate email, bad role                              | 1 created; 2 failed with reasons; no partial row data saved for failures                        | FR-1.6  | P3 |

## 4. Validation rules (TC-VAL) — unit

Each case's data lives in `testdata/validation_vectors.json` and runs in **both** the Go validator tests and the front-end Zod tests; both must agree.

| ID         | Scenario                       | Data                                                                    | Expected                                    | Ref       | P  |
|------------|--------------------------------|-------------------------------------------------------------------------|---------------------------------------------|-----------|----|
| TC-VAL-001 | Valid names                    | `Ravi Kumar`, `D'Souza`, `A. Rao`, `Mary-Jane`                           | Pass                                        | spec §8.4 | P2 |
| TC-VAL-002 | Invalid names                  | `R`, 101 chars, `Ravi2`, `   `                                           | Fail                                        | spec §8.4 | P2 |
| TC-VAL-003 | Mobile normalisation           | `9876543210`, `+919876543210`, `09876543210`, ` 98765 43210 `            | All → `+919876543210`                       | spec §8.4 | P1 |
| TC-VAL-004 | Invalid mobile                 | `5876543210`, `987654321`, `98765432101`, `98765abcde`                   | Fail                                        | spec §8.4 | P1 |
| TC-VAL-005 | Email                          | empty (ok), `a@b.co` (ok), `a@b` (fail)                                  | As stated                                   | spec §8.4 | P3 |
| TC-VAL-006 | Purpose length                 | 2, 3, 250, 251 chars                                                     | Fail, pass, pass, fail                      | spec §8.4 | P3 |
| TC-VAL-007 | Visit date window              | Today, today+30, today+31, yesterday (clock 23:50 IST)                   | Pass, pass, fail, fail                      | FR-2.3    | P1 |
| TC-VAL-008 | Duration                       | 30, 45, 480 / 15, 50, 495                                                | Pass / fail                                 | spec §8.4 | P2 |
| TC-VAL-009 | Arrival time                   | `09:30` / `24:00`, `9:3`                                                 | Pass / fail                                 | spec §8.4 | P3 |
| TC-VAL-010 | Reason (deny, force-close)     | missing, `ok`, 5 chars, 251 chars                                        | Fail, fail, pass, fail                      | spec §8.4 | P2 |
| TC-VAL-011 | ID fields                      | idType `PAN` + `AB12`; idType `CARD`; last4 `123`, `12345`, `12-4`        | First passes; others fail                   | FR-4.3    | P2 |
| TC-VAL-012 | Trimming and unknown fields    | `"  Ravi  "`; extra field `status:"CHECKED_OUT"`                          | Trimmed to `Ravi`; extra field dropped      | spec §8.4 | P1 |

## 5. State machine (TC-SM)

| ID        | Lvl | Scenario                                    | Steps / data                                                                                       | Expected                                                                                     | Ref         | P  |
|-----------|-----|---------------------------------------------|----------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------|-------------|----|
| TC-SM-001 | U   | All allowed transitions                     | Parameterised over every row of spec §6.2 with correct actor and guards satisfied                  | Each returns the target status                                                               | spec §6.2   | P1 |
| TC-SM-002 | U   | All other transitions rejected              | Exhaustive matrix of 11 statuses × all actions minus allowed rows                                  | Every one `INVALID_STATE`                                                                    | spec §6.2   | P1 |
| TC-SM-003 | U   | Final statuses are locked                   | `allowedActions` for each final status, every role                                                 | Empty list                                                                                   | BR-8        | P1 |
| TC-SM-004 | U   | Approval boundary                           | Approve at deadline − 1 s; at deadline exactly                                                     | Allowed; `APPROVAL_WINDOW_CLOSED`                                                             | BR-3        | P1 |
| TC-SM-005 | U   | Reject after deadline                       | Reject at deadline + 1 s                                                                           | `APPROVAL_WINDOW_CLOSED`                                                                      | BR-3        | P1 |
| TC-SM-006 | U   | Only host decides                           | empB approves empA's walk-in                                                                       | `FORBIDDEN`                                                                                  | FR-3.4      | P1 |
| TC-SM-007 | U   | Wrong date check-in                         | `SCHEDULED` for tomorrow, both photos                                                              | `WRONG_VISIT_DATE`                                                                            | BR-5        | P1 |
| TC-SM-008 | U   | Photos required                             | Only visitor photo; only ID photo; none                                                           | `PHOTOS_REQUIRED` for all                                                                     | BR-1        | P1 |
| TC-SM-009 | U   | Check-out needs MO                          | guard checks out `CHECKED_IN`                                                                      | `INVALID_STATE`                                                                              | BR-4        | P1 |
| TC-SM-010 | U   | Force-close actor                           | admin from `CHECKED_IN` with reason; guard tries same                                             | Allowed → `CHECKED_OUT`; guard `FORBIDDEN`                                                    | FR-6.4      | P1 |
| TC-SM-011 | U   | Meeting Over actor                          | guard and empB mark MO on empA's visit                                                             | `FORBIDDEN`                                                                                  | FR-5.2      | P1 |
| TC-SM-012 | U   | allowedActions — host on pending            | empA, `PENDING_APPROVAL`, before and after deadline                                                | `[APPROVE, REJECT]`; then `[]`                                                                | spec §8.3   | P2 |
| TC-SM-013 | U   | allowedActions — security                   | `SCHEDULED` today without photos; with both photos; `MEETING_OVER`; `CHECKED_IN`                   | `[UPLOAD_PHOTOS, DENY]`; `[UPLOAD_PHOTOS, CHECK_IN, DENY]`; `[CHECK_OUT]`; `[]`               | spec §8.3   | P2 |
| TC-SM-014 | U   | allowedActions — admin                      | admin on empA's `CHECKED_IN`; admin as host of own `CHECKED_IN`                                     | `[FORCE_CLOSE]`; `[MEETING_OVER, FORCE_CLOSE]`                                                | spec §4     | P2 |
| TC-SM-015 | I   | Concurrent transition                       | Two parallel `POST /check-out` on same `MEETING_OVER` visit (guard, guard2)                        | Exactly one 200, other 409; one audit row; `version` +1 only                                  | spec §6.3   | P1 |
| TC-SM-016 | I   | Audit written atomically                    | Successful transition; transition that fails a guard                                               | Success: audit row with actor, role, from, to, reason. Failure: no audit row, version unchanged | FR-9.1     | P1 |

## 6. Planned visits (TC-PV)

| ID        | Lvl | Scenario                         | Steps / data                                                                             | Expected                                                                                              | Ref        | P  |
|-----------|-----|----------------------------------|------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|------------|----|
| TC-PV-001 | I   | Create planned visit             | empA posts valid body for today, 60 min                                                   | 201; `SCHEDULED`; `type PLANNED`; host = empA; code `GP-20261103-0001`; audit `VISIT_CREATED`          | FR-2.1, FR-2.4 | P1 |
| TC-PV-002 | I   | Host cannot be spoofed           | empA posts with `hostId = empB`                                                            | 201 with host = empA                                                                                   | FR-2.2     | P1 |
| TC-PV-003 | I   | Required fields                   | Missing name, mobile, purpose, date, duration                                             | 400; `details.fields` lists all five                                                                   | FR-2.1     | P1 |
| TC-PV-004 | I   | Date window                       | Yesterday; today + 31                                                                       | 400 for both                                                                                           | FR-2.3     | P1 |
| TC-PV-005 | I   | Security cannot plan              | guard posts planned visit                                                                   | 403                                                                                                    | spec §4    | P2 |
| TC-PV-006 | I   | Edit                              | empA changes date and duration of own `SCHEDULED`                                          | 200; version +1; audit `VISIT_UPDATED` with changed fields in metadata                                  | FR-2.5     | P1 |
| TC-PV-007 | I   | Edit guards                       | empB edits empA's visit; empA edits own `CHECKED_IN` visit                                  | 403; 409 `INVALID_STATE`                                                                               | FR-2.5     | P1 |
| TC-PV-008 | I   | Cancel                            | empA cancels own `SCHEDULED`; cancels again                                                | 200 `CANCELLED`; second 409                                                                            | FR-2.6     | P1 |
| TC-PV-009 | I   | Cancelled cannot check in         | guard uploads photos / checks in cancelled visit                                            | 409                                                                                                    | FR-2.6     | P1 |
| TC-PV-010 | I   | Visit code                        | 3 visits on 2026-11-03, 1 on 2026-11-04; 20 parallel creates                                | `…-0001..0003`, then `GP-20261104-0001`; parallel codes all unique                                      | spec §5.3  | P2 |
| TC-PV-011 | I   | Visitor reuse                     | Two visits with same mobile, second with name `Ravi K.`                                     | Same `visitor_id`; visitor name updated; first visit keeps `visitor_name` `Ravi Kumar`                   | Q-5        | P2 |
| TC-PV-012 | I   | Visitor lookup                    | `GET /visitors/lookup?mobile=9876543210`; unknown mobile; `mobile=123`                      | 200 name/email/company only; 404; 400                                                                  | Q-5        | P3 |
| TC-PV-013 | I   | Admin as host                     | admin creates planned visit                                                                 | 201 with host = admin                                                                                  | spec §4    | P3 |
| TC-PV-014 | I   | List scoping                      | Visits for empA and empB; each role calls `GET /visits`                                     | empA sees only own; guard and admin see all                                                            | BR-9       | P1 |
| TC-PV-015 | I   | List filters and paging           | `status=SCHEDULED&status=CHECKED_IN`, `from/to`, `q` by code/name/mobile; `pageSize=101`     | Correct subsets and `total`; `pageSize=101` → 400                                                       | FR-2.8     | P2 |
| TC-PV-016 | I   | Detail access                     | empB `GET /visits/<empA visit>`; random UUID                                                | 403; 404                                                                                               | BR-9, AC-7 | P1 |
| TC-PV-017 | I   | Detail content                    | Visit with photos and 3 transitions                                                         | Has `allowedActions`, `hasVisitorPhoto`, `hasIdPhoto`, timeline in time order; no `storage_key`         | FR-8.5     | P2 |
| TC-PV-018 | I   | Lazy expiry on read               | Pending walk-in, clock past deadline, job not run; `GET /visits/:id`                        | Returns `EXPIRED`; audit actor `SYSTEM`                                                                 | spec §6.3  | P1 |

## 7. Walk-ins and approvals (TC-WI)

| ID        | Lvl | Scenario                             | Steps / data                                                                                   | Expected                                                                                                   | Ref         | P  |
|-----------|-----|--------------------------------------|------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------|-------------|----|
| TC-WI-001 | I   | Create walk-in                       | guard posts walk-in for empA at 10:00                                                          | 201; `PENDING_APPROVAL`; `UNPLANNED`; date today; deadline 10:30; created_by guard; empA gets `APPROVAL_REQUESTED` + email | FR-3.1, FR-3.3 | P1 |
| TC-WI-002 | I   | Date fixed to today                  | Body includes `visitDate` tomorrow                                                              | 201 with `visitDate` today                                                                                  | FR-3.1      | P2 |
| TC-WI-003 | I   | Invalid host                         | `hostId` = exEmp; = guard2; = random UUID                                                        | 400 for each                                                                                                | FR-3.2      | P1 |
| TC-WI-004 | I   | Employees cannot create walk-ins     | empA posts walk-in                                                                              | 403                                                                                                         | spec §4     | P2 |
| TC-WI-005 | I   | Duplicate pending                    | Second walk-in for same mobile while first pending                                              | 409 `VISITOR_ALREADY_ACTIVE`                                                                                 | spec §8.4   | P2 |
| TC-WI-006 | I   | Visitor already inside               | Walk-in for a mobile whose other visit is `CHECKED_IN`                                          | 409 `VISITOR_ALREADY_ACTIVE`                                                                                 | spec §8.4   | P2 |
| TC-WI-007 | I   | Accept                               | empA approves at 10:15                                                                          | 200 `APPROVED`; `decided_at` set; guard & guard2 get `APPROVAL_DECIDED`; `visit-updated` emitted             | FR-3.4, AC-2 | P1 |
| TC-WI-008 | I   | Reject with reason                   | empA rejects with "Not expecting anyone"                                                         | 200 `REJECTED`; reason stored; security notified                                                             | FR-3.4, AC-3 | P1 |
| TC-WI-009 | I   | Reject without reason                | empA rejects with empty body                                                                     | 200 `REJECTED`                                                                                              | FR-3.4      | P3 |
| TC-WI-010 | I   | Accept after window                  | Clock 10:30:01, empA approves                                                                    | 409 `APPROVAL_WINDOW_CLOSED`; visit now `EXPIRED` (audit actor `SYSTEM`)                                     | FR-3.5, AC-4 | P1 |
| TC-WI-011 | I   | Only host may decide                 | empB approves; guard approves                                                                    | 403; 403                                                                                                    | FR-3.4      | P1 |
| TC-WI-012 | I   | Decide twice                         | Approve, then reject the same visit                                                              | Second 409 `INVALID_STATE`                                                                                   | BR-8        | P2 |
| TC-WI-013 | I   | Window fixed at creation             | Create at 10:00 (window 30); admin sets window 15; approve at 10:20                               | 200 (deadline still 10:30)                                                                                   | spec §8.2   | P2 |
| TC-WI-014 | I   | Security cancels pending             | guard cancels with reason "Visitor left"                                                         | 200 `CANCELLED`; empA can no longer approve (409)                                                            | FR-3.9      | P2 |
| TC-WI-015 | I   | Host cannot cancel walk-in           | empA calls cancel on pending walk-in                                                              | 403                                                                                                         | spec §6.2   | P3 |
| TC-WI-016 | I   | New request after reject             | After TC-WI-008, guard creates walk-in for same visitor → empB                                   | 201 new visit; old visit unchanged and still `REJECTED`                                                     | FR-3.8      | P2 |
| TC-WI-017 | I   | Race: accept vs expiry               | Clock at deadline; run `expireApprovals` and `approve` concurrently (50 iterations)               | Every iteration: exactly one transition recorded; final status `EXPIRED` and approve gets 409              | spec §6.3   | P1 |
| TC-WI-018 | I   | Race: accept vs reject               | Approve and reject in parallel                                                                   | One 200, other 409; one audit transition                                                                     | spec §6.3   | P2 |

## 8. Photos (TC-PH)

| ID        | Lvl | Scenario                         | Steps / data                                                               | Expected                                                                                       | Ref          | P  |
|-----------|-----|----------------------------------|----------------------------------------------------------------------------|------------------------------------------------------------------------------------------------|--------------|----|
| TC-PH-001 | I   | Upload visitor photo             | guard `PUT /visits/:id/photos/VISITOR` with `visitor.jpg`, `SCHEDULED` today | 200; stored JPEG ≤ 500 KB; audit `PHOTO_CAPTURED`                                              | FR-4.2, NFR-8 | P1 |
| TC-PH-002 | I   | PNG accepted                     | `PUT …/ID_DOCUMENT` `id.png`, idType `AADHAAR`                              | 200; stored as JPEG                                                                            | NFR-8        | P2 |
| TC-PH-003 | I   | Too large                        | `big.jpg` (6 MB)                                                           | 413 `FILE_TOO_LARGE`; nothing stored                                                           | spec §11     | P2 |
| TC-PH-004 | I   | Wrong content                    | `fake.jpg` (text), `anim.gif`                                              | 415 `UNSUPPORTED_MEDIA_TYPE`                                                                   | NFR-5        | P1 |
| TC-PH-005 | I   | ID metadata                      | ID photo without `idType`; with `PAN` + `AB12`                              | 400; 200 with idType and last4 saved                                                           | FR-4.3       | P1 |
| TC-PH-006 | I   | Metadata stripped                | Upload `visitor.jpg` (has GPS); read stored file metadata                  | No EXIF/GPS                                                                                    | spec §11     | P1 |
| TC-PH-007 | I   | Retake                           | Upload VISITOR twice                                                        | One row for VISITOR; first file deleted from storage                                            | FR-4.6       | P2 |
| TC-PH-008 | I   | Upload state guard               | Upload to `CHECKED_IN`, `PENDING_APPROVAL`, `SCHEDULED` tomorrow            | 409 for each                                                                                    | spec §11     | P1 |
| TC-PH-009 | I   | Role guard on upload             | empA uploads to own visit                                                   | 403                                                                                            | spec §4      | P2 |
| TC-PH-010 | I   | Security views photos            | guard GETs VISITOR and ID_DOCUMENT                                          | 200 `image/jpeg`; `Cache-Control: private, no-store`                                           | BR-10        | P1 |
| TC-PH-011 | I   | Host photo rights                | empA GETs VISITOR; GETs ID_DOCUMENT                                         | 200; 403                                                                                       | BR-10        | P1 |
| TC-PH-012 | I   | Other employee                   | empB GETs empA visit's VISITOR photo                                        | 403                                                                                            | BR-10        | P1 |
| TC-PH-013 | I   | Purged / no leak                 | GET after purge; scan all JSON responses in suite for `storage_key` / path   | 404; no response contains a storage path                                                        | spec §11     | P2 |

## 9. Check-in and deny (TC-CI)

| ID        | Lvl | Scenario                       | Steps / data                                                       | Expected                                                                                             | Ref          | P  |
|-----------|-----|--------------------------------|--------------------------------------------------------------------|------------------------------------------------------------------------------------------------------|--------------|----|
| TC-CI-001 | I   | Check in planned               | `SCHEDULED` today, both photos; guard checks in                    | 200 `CHECKED_IN`; `checked_in_at/by` set; empA gets `VISITOR_CHECKED_IN` + email                       | FR-4.5       | P1 |
| TC-CI-002 | I   | Check in approved walk-in      | `APPROVED` with both photos                                        | 200 `CHECKED_IN`                                                                                     | FR-3.7       | P1 |
| TC-CI-003 | I   | Photos missing                 | No ID photo; no visitor photo                                      | 422 `PHOTOS_REQUIRED` for each                                                                        | BR-1, AC-6   | P1 |
| TC-CI-004 | I   | Wrong date                     | `SCHEDULED` for tomorrow                                           | 422 `WRONG_VISIT_DATE`                                                                                | BR-5, AC-8   | P1 |
| TC-CI-005 | I   | Walk-in not approved           | `PENDING_APPROVAL`, `REJECTED`, `EXPIRED`                          | 409 for each                                                                                          | BR-2         | P1 |
| TC-CI-006 | I   | Closed visits                  | `CANCELLED`, `NO_SHOW`                                             | 409                                                                                                   | BR-8         | P2 |
| TC-CI-007 | I   | Visitor already inside         | Same mobile has another visit `CHECKED_IN`                         | 409 `VISITOR_ALREADY_ACTIVE`                                                                           | spec §8.4    | P2 |
| TC-CI-008 | I   | Role guard                     | empA and admin call check-in                                       | 403                                                                                                   | spec §4      | P1 |
| TC-CI-009 | I   | Expected today list            | Seed today `SCHEDULED`, today `APPROVED`, tomorrow `SCHEDULED`, today `CHECKED_OUT`; search by mobile and code | Only first two returned; search finds by mobile and code                      | FR-4.1       | P2 |
| TC-CI-010 | I   | Deny entry                     | guard denies `SCHEDULED` with "Name on ID does not match"          | 200 `DENIED`; reason stored; empA notified `VISITOR_DENIED`                                            | FR-4.7       | P2 |
| TC-CI-011 | I   | Deny guards                    | Deny without reason; deny a `CHECKED_IN` visit                     | 400; 409                                                                                              | FR-4.7       | P2 |

## 10. Meeting Over (TC-MO)

| ID        | Lvl | Scenario                      | Steps / data                                       | Expected                                                              | Ref     | P  |
|-----------|-----|-------------------------------|----------------------------------------------------|-----------------------------------------------------------------------|---------|----|
| TC-MO-001 | I   | Host marks MO                 | empA on own `CHECKED_IN`                            | 200 `MEETING_OVER`; time set; guard & guard2 get `MEETING_OVER`        | FR-5.1, FR-5.3 | P1 |
| TC-MO-002 | I   | Others cannot mark MO         | empB; guard                                         | 403; 403                                                              | FR-5.2  | P1 |
| TC-MO-003 | I   | Wrong state                   | MO on `SCHEDULED`; MO on `MEETING_OVER`             | 409; 409                                                              | spec §6.2 | P2 |
| TC-MO-004 | I   | Admin as host                 | admin marks MO on own hosted visit                  | 200                                                                   | spec §4 | P3 |

## 11. Check-out and force-close (TC-CO)

| ID        | Lvl | Scenario                     | Steps / data                                                         | Expected                                                                                      | Ref         | P  |
|-----------|-----|------------------------------|----------------------------------------------------------------------|-----------------------------------------------------------------------------------------------|-------------|----|
| TC-CO-001 | I   | Check out                    | guard on `MEETING_OVER`                                               | 200 `CHECKED_OUT`; time/by set; empA notified `VISITOR_CHECKED_OUT`                            | FR-6.2      | P1 |
| TC-CO-002 | I   | Blocked before MO            | guard on `CHECKED_IN`                                                 | 409 `INVALID_STATE`, message "Waiting for host to mark Meeting Over"                           | BR-4, AC-5  | P1 |
| TC-CO-003 | I   | Double check-out             | Check out twice                                                       | Second 409                                                                                    | BR-8        | P2 |
| TC-CO-004 | I   | Role guard                   | empA and admin call check-out                                         | 403                                                                                           | spec §4     | P1 |
| TC-CO-005 | I   | Force-close                  | admin on `CHECKED_IN` with "Host on leave, visitor left at 5 pm"      | 200 `CHECKED_OUT`; audit action `FORCE_CLOSED` with reason                                     | FR-6.4      | P1 |
| TC-CO-006 | I   | Force-close reason required  | No reason; reason `ok`                                                | 400 for both                                                                                  | FR-6.4      | P1 |
| TC-CO-007 | I   | Force-close guards           | guard force-closes; admin force-closes `SCHEDULED`                    | 403; 409                                                                                      | spec §6.2   | P2 |
| TC-CO-008 | I   | On-premises list             | Visits in every status; `GET /visits?onPremises=true` as guard        | Only `CHECKED_IN` and `MEETING_OVER`                                                           | FR-6.3      | P1 |

## 12. Full API flows (TC-FLOW)

| ID          | Lvl | Scenario                 | Steps                                                                                          | Expected                                                                                                                  | Ref   | P  |
|-------------|-----|--------------------------|------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------|-------|----|
| TC-FLOW-001 | I   | Planned visit end to end | empA creates → guard uploads 2 photos → check-in → empA MO → guard check-out                    | Final `CHECKED_OUT`; all timestamps set; audit sequence `VISIT_CREATED, PHOTO_CAPTURED×2, CHECKED_IN, MEETING_OVER, CHECKED_OUT` | AC-1 | P1 |
| TC-FLOW-002 | I   | Walk-in end to end       | guard creates → empA approves → photos → check-in → MO → check-out                              | Final `CHECKED_OUT`; audit includes `APPROVED` transition                                                                 | AC-2 | P1 |

## 13. Notifications and real-time (TC-NT)

| ID        | Lvl | Scenario                         | Steps / data                                                                  | Expected                                                                                     | Ref        | P  |
|-----------|-----|----------------------------------|-------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------|------------|----|
| TC-NT-001 | I   | Approval request                 | Create walk-in for empA                                                        | 1 `APPROVAL_REQUESTED` row for empA; 1 email to empA (fake mailer)                            | FR-7.1     | P1 |
| TC-NT-002 | I   | Visitor arrived                  | Check-in                                                                       | `VISITOR_CHECKED_IN` row + email to host                                                      | FR-7.4     | P1 |
| TC-NT-003 | I   | Meeting over                     | Host marks MO                                                                  | `MEETING_OVER` row for each active SECURITY user                                              | FR-7.6     | P1 |
| TC-NT-004 | I   | Decision outcomes                | Approve; reject; expire; cancel (separate visits)                              | Security gets `APPROVAL_DECIDED` each time; host also gets it on expiry                        | FR-7.3     | P1 |
| TC-NT-005 | I   | Deny and check-out               | Deny one visit; check out another                                              | Host gets `VISITOR_DENIED`; host gets `VISITOR_CHECKED_OUT`                                    | FR-7.7     | P3 |
| TC-NT-006 | I   | Inactive recipients skipped      | Deactivate guard2; host marks MO                                               | Only guard gets the notification                                                              | FR-1.6     | P3 |
| TC-NT-007 | I   | Email failure isolation          | Mailer throws; create walk-in                                                   | 201; in-app notification created; 3 send attempts; error logged                               | spec §9.1  | P1 |
| TC-NT-008 | I   | List notifications               | empA `GET /notifications`                                                       | Only empA's, newest first, correct `unreadCount`                                              | FR-7.x     | P2 |
| TC-NT-009 | I   | Mark read                        | empA marks own; empA marks empB's                                               | 200 and count −1; 404                                                                          | FR-7.x     | P2 |
| TC-NT-010 | I   | Mark all read                    | `POST /notifications/read-all`                                                  | `unreadCount` 0                                                                                | FR-7.x     | P3 |
| TC-NT-011 | I   | Event stream authentication      | `GET /api/events` without cookie; with empA cookie                              | 401, no stream; 200 `text/event-stream`, subscribed as empA/EMPLOYEE (does not receive security-only events) | spec §9.2  | P1 |
| TC-NT-012 | I   | Event delivery                   | empA and empB streams open; walk-in created for empA                            | empA receives `event: notification` within 5 s; empB receives nothing                         | NFR-2      | P1 |
| TC-NT-013 | I   | visit-updated after commit       | Approve (success); approve that fails (409)                                     | Success: `visit-updated` to host, all SECURITY and ADMIN streams. Failure: no event            | spec §6.3  | P2 |
| TC-NT-014 | I   | Slow client does not block       | Open a stream that never reads; emit 100 events to SECURITY                     | Other SECURITY streams receive all events on time; the stalled stream is dropped and unsubscribed | spec §9.2  | P2 |

## 14. Background jobs (TC-JOB)

| ID         | Lvl | Scenario                         | Steps / data                                                                                       | Expected                                                                                        | Ref        | P  |
|------------|-----|----------------------------------|----------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------|------------|----|
| TC-JOB-001 | I   | Expire approvals                 | Two pending walk-ins: deadline 10:30 and 10:45; clock 10:31; run job                                 | First `EXPIRED` (actor `SYSTEM`), host + security notified; second unchanged                    | FR-3.5     | P1 |
| TC-JOB-002 | I   | Expiry is idempotent             | Run job twice                                                                                       | One audit row, one set of notifications                                                         | spec §10   | P1 |
| TC-JOB-003 | I   | Approval reminder                | Created 10:00; clock 10:20; run reminder job twice; also a visit approved at 10:05                   | One `APPROVAL_REMINDER` for pending visit; none for approved visit                              | FR-7.2     | P2 |
| TC-JOB-004 | I   | No reminder after deadline       | Clock 10:31; reminder job before expiry job                                                         | No reminder sent                                                                                | FR-7.2     | P3 |
| TC-JOB-005 | I   | Overstay alert                   | Checked in 10:00, 60 min; clock 11:01; run job twice                                                | One `OVERSTAY` each to host and security; `isOverstay:true` in API                              | FR-5.4     | P2 |
| TC-JOB-006 | I   | No false overstay                | Same visit at 10:59; another `MEETING_OVER` at 11:30                                                | No alerts                                                                                       | FR-5.4     | P2 |
| TC-JOB-007 | I   | No-show                          | Yesterday `SCHEDULED` + yesterday `APPROVED` + today `SCHEDULED`; run at 00:05                      | First two `NO_SHOW`; today's untouched                                                          | BR-6       | P1 |
| TC-JOB-008 | I   | No-show at IST midnight          | Visit on 2026-11-03; run at 23:59 IST; then at 2026-11-04 00:05 IST (`2026-11-03T18:35Z`)           | Unchanged at 23:59; `NO_SHOW` at 00:05                                                          | BR-11      | P1 |
| TC-JOB-009 | I   | Photo retention                  | Visits final 91 days ago, 89 days ago, and one still `CHECKED_IN` from 100 days ago                  | Only the 91-day visit's files and rows removed; `photos_purged_at` set                          | Req §10.2  | P1 |
| TC-JOB-010 | I   | Record retention                 | Visit final 366 days ago with notifications and audit rows                                          | Visit and its notifications deleted; audit rows remain                                          | Req §10.2  | P2 |
| TC-JOB-011 | I   | Token cleanup                    | Expired and valid reset tokens                                                                      | Only expired removed                                                                            | spec §10   | P3 |
| TC-JOB-012 | I   | Single runner                    | Two job runners call `expireApprovals` at once                                                       | Job body executes once (advisory lock); no duplicate side effects                               | spec §10   | P2 |

## 15. Dashboards (TC-DB)

| ID        | Lvl | Scenario                        | Steps / data                                                             | Expected                                                                                     | Ref     | P  |
|-----------|-----|---------------------------------|--------------------------------------------------------------------------|----------------------------------------------------------------------------------------------|---------|----|
| TC-DB-001 | I   | Security dashboard buckets      | Seed one visit in every status, today                                     | `expectedToday`, `pendingApprovals`, `onPremises`, `readyForCheckout` contain exactly the right visits; `counts` match | FR-8.2 | P1 |
| TC-DB-002 | I   | Overstay bucket                 | One overstayed `CHECKED_IN`, one within time                              | Only the first in `overstays`                                                                 | FR-8.2  | P2 |
| TC-DB-003 | I   | Employee dashboard              | empA and empB visits in several statuses                                  | empA sees only own; `pendingApprovals` include `approvalSecondsLeft`                          | FR-8.1  | P1 |
| TC-DB-004 | I   | Dashboard access                | empA calls `/dashboard/security`; guard calls `/dashboard/employee`       | 403; 403                                                                                     | spec §8.2 | P2 |

## 16. Audit (TC-AUD)

| ID         | Lvl | Scenario                     | Steps / data                                               | Expected                                                                               | Ref     | P  |
|------------|-----|------------------------------|------------------------------------------------------------|----------------------------------------------------------------------------------------|---------|----|
| TC-AUD-001 | I   | Complete trail               | Run TC-FLOW-001 flow                                        | Every step has actor, role, from/to status, timestamp                                   | FR-9.1  | P1 |
| TC-AUD-002 | I   | Audit access                 | admin, empA, guard call `/visits/:id/audit`                 | 200; 403; 403                                                                          | FR-9.3  | P1 |
| TC-AUD-003 | I   | Append-only at DB level      | Using app DB role, run `UPDATE` and `DELETE` on `audit_logs` | Both fail with permission denied                                                        | FR-9.2  | P1 |
| TC-AUD-004 | I   | User actions audited         | Create user, change role, deactivate                        | Three audit rows with admin as actor                                                    | FR-9.1  | P2 |

## 17. Settings (TC-SET)

| ID         | Lvl | Scenario                    | Steps / data                                                                     | Expected                                    | Ref       | P  |
|------------|-----|-----------------------------|----------------------------------------------------------------------------------|---------------------------------------------|-----------|----|
| TC-SET-001 | I   | Read and update             | admin GET, PATCH `approvalWindowMin: 20`                                          | 200; value saved; audit entry               | BR-3      | P2 |
| TC-SET-002 | I   | Range validation            | window 5; reminder 20 with window 20; photo retention 3; record retention 30     | 400 for each                                | spec §8.2 | P2 |
| TC-SET-003 | I   | Admin only                  | empA and guard call GET/PATCH                                                     | 403                                         | spec §4   | P1 |
| TC-SET-004 | I   | New window applies to new walk-ins | Set window 20; create walk-in at 10:00                                     | Deadline 10:20                              | BR-3      | P2 |

## 18. Reports (TC-RPT)

| ID         | Lvl | Scenario                 | Steps / data                                                       | Expected                                                                                     | Ref       | P  |
|------------|-----|--------------------------|--------------------------------------------------------------------|----------------------------------------------------------------------------------------------|-----------|----|
| TC-RPT-001 | I   | CSV export               | admin `GET /reports/visits.csv?from=…&to=…&status=CHECKED_OUT`      | `text/csv`; header row; only matching rows; times in IST                                     | FR-8.4    | P2 |
| TC-RPT-002 | I   | No sensitive data        | Export visits with photos and ID last-4                            | No photo paths, no `idLast4` column                                                           | Req §10.2 | P1 |
| TC-RPT-003 | I   | Formula injection        | Visitor company `=HYPERLINK("http://x")`                            | Cell exported as `'=HYPERLINK("http://x")`                                                    | NFR-5     | P2 |
| TC-RPT-004 | I   | Admin only               | empA and guard request export                                       | 403                                                                                          | FR-8.3    | P1 |

## 19. Front-end components (TC-UI-*) — Vitest + RTL + MSW (mock `EventSource`)

| ID             | Scenario                                | Steps / data                                                                  | Expected                                                                                           | Ref        | P  |
|----------------|-----------------------------------------|-------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------|------------|----|
| TC-UI-AUTH-001 | Login form                              | Submit empty; API returns 401; API returns 423                                | Field errors; "Invalid email or password"; lockout message                                          | FR-1.1     | P1 |
| TC-UI-AUTH-002 | Forgot password                         | Submit any email                                                               | Same confirmation text regardless of API result                                                     | FR-1.5     | P3 |
| TC-UI-AUTH-003 | Reset with bad token                    | API returns 400                                                                | Error with link to request a new reset                                                              | FR-1.5     | P3 |
| TC-UI-AUTH-004 | Role redirect                           | `/` as each role                                                               | `/employee`, `/security`, `/admin`                                                                  | spec §12.1 | P2 |
| TC-UI-AUTH-005 | Route guards                            | empA opens `/security`; logged-out user opens `/employee/visits`              | Access-denied page; redirect to `/login?returnTo=/employee/visits`, then back after login          | FR-1.2     | P1 |
| TC-UI-AUTH-006 | Forced password change                  | `/auth/me` returns `mustChangePassword:true`                                   | Every route redirects to `/change-password`                                                         | spec §7    | P2 |
| TC-UI-AUTH-007 | Notification bell                       | Mock event stream emits `notification`; click "Mark all read"                    | Badge increments without reload; then shows 0                                                       | FR-7.x     | P2 |
| TC-UI-EMP-001  | Planned visit form                      | Invalid mobile, duration 50, date beyond window; then valid submit            | Inline errors matching server rules; date picker disables out-of-window days; success → My Visits  | FR-2.1     | P1 |
| TC-UI-EMP-002  | Mobile prefill                          | Enter known mobile, blur                                                       | Name/email/company filled; still editable                                                           | Q-5        | P3 |
| TC-UI-EMP-003  | Approval countdown                      | Pending card with 65 s left; advance timers                                    | Shows `01:05`, ticks down; at 0 buttons disabled and card shows "Expired"                           | FR-3.5     | P1 |
| TC-UI-EMP-004  | Reject flow                             | Click Reject                                                                   | Confirm dialog with optional reason; API called with reason                                         | FR-3.4     | P2 |
| TC-UI-EMP-005  | Meeting Over button                     | Own visits in `CHECKED_IN`, `SCHEDULED`                                        | Button only on `CHECKED_IN` (via `allowedActions`)                                                  | FR-5.1     | P1 |
| TC-UI-EMP-006  | Status badge                            | Render every status                                                            | Text label always present; distinct colour; passes contrast check                                   | NFR-11     | P3 |
| TC-UI-EMP-007  | Visit detail as host                    | Host views own checked-in visit                                                | Visitor photo shown, ID photo not requested; timeline rendered                                      | BR-10      | P2 |
| TC-UI-SEC-001  | Security dashboard tabs                 | Mock dashboard response                                                        | 5 tabs with correct counts and lists                                                                | FR-8.2     | P1 |
| TC-UI-SEC-002  | Live refresh                            | Mock event stream emits `visit-updated` for a pending visit                     | Dashboard refetches; card moves from Pending to Expected                                            | NFR-2      | P1 |
| TC-UI-SEC-003  | Whom-to-meet typeahead                  | Type `a`, then `an`; type free text and submit                                 | No call for 1 char; one debounced call; submit blocked until an employee is chosen                  | FR-3.2     | P1 |
| TC-UI-SEC-004  | Camera capture                          | Mock `getUserMedia` stream; capture; retake                                    | Preview shown; captured image displayed; retake returns to preview                                  | FR-4.2     | P1 |
| TC-UI-SEC-005  | Camera fallback and cleanup             | `getUserMedia` rejects `NotAllowedError`; unmount after success                | File input shown with message; all media tracks stopped on unmount                                  | FR-4.2     | P2 |
| TC-UI-SEC-006  | Check-in screen                         | Only visitor photo saved; then both; click Deny with empty reason              | Check-in disabled, then enabled; Deny requires reason (5+ chars)                                    | BR-1, FR-4.7 | P1 |
| TC-UI-SEC-007  | Check-out                               | Ready-for-check-out tab                                                        | Check-out button with confirm; not shown on On-premises items still `CHECKED_IN`                    | BR-4       | P1 |
| TC-UI-SEC-008  | Countdown uses server time              | Device clock 10 min ahead; server `Date` header correct                         | Countdown shows true remaining time                                                                 | spec §12.2 | P2 |
| TC-UI-ADM-001  | Create / edit user                      | Submit form; API returns 409 for email                                         | Error shown on email field                                                                          | FR-1.6     | P2 |
| TC-UI-ADM-002  | Deactivate user                         | Click Deactivate, confirm                                                      | Confirm dialog; row shows Inactive                                                                  | FR-1.6     | P2 |
| TC-UI-ADM-003  | Visits filters and export               | Set status + date range; click Export                                          | List refetches with filters; download URL contains same filters                                     | FR-8.3     | P2 |
| TC-UI-ADM-004  | Force-close and audit                   | Open checked-in visit; force-close without reason; with reason                 | Blocked; then succeeds and audit list shows `FORCE_CLOSED`                                          | FR-6.4     | P2 |
| TC-UI-ADM-005  | Settings form                           | Reminder ≥ window                                                               | Inline error; save disabled                                                                          | spec §8.2  | P3 |

## 20. End-to-end (TC-E2E) — Playwright, fake camera

Each test uses separate browser contexts per role so real-time updates between users are exercised.

| ID          | Scenario                                       | Steps                                                                                                                                           | Expected                                                                                                     | Ref   | P  |
|-------------|------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------|-------|----|
| TC-E2E-001  | Planned visit happy path                       | empA creates visit for today → guard finds it under Expected, captures both photos, checks in → empA sees "Visitor arrived", marks MO → guard checks out | Visit `CHECKED_OUT`; empA's visit detail shows full timeline                                                  | AC-1  | P1 |
| TC-E2E-002  | Walk-in accepted                               | guard registers walk-in for empA → empA sees request with countdown, accepts                                                                    | Guard's Pending card turns Approved **without page reload** within 5 s; check-in then works                   | AC-2  | P1 |
| TC-E2E-003  | Walk-in rejected                               | guard registers walk-in → empA rejects with reason                                                                                              | Guard sees Rejected with reason; no Check-in button                                                          | AC-3  | P1 |
| TC-E2E-004  | Walk-in expired                                | guard registers walk-in → advance test clock 31 min                                                                                             | Both see Expired; empA's Accept button gone; guard notified                                                   | AC-4  | P1 |
| TC-E2E-005  | Check-out blocked before MO                    | Visitor checked in; guard opens On premises                                                                                                     | No Check-out action; status shows "Waiting for host to mark Meeting Over"                                     | AC-5  | P1 |
| TC-E2E-006  | Check-in blocked without photos                | guard opens check-in screen, captures only visitor photo                                                                                        | Check in button disabled                                                                                     | AC-6  | P1 |
| TC-E2E-007  | Access control                                 | empB pastes URL of empA's visit                                                                                                                 | Access-denied page; no visit data visible                                                                    | AC-7  | P1 |
| TC-E2E-008  | Wrong date                                     | empA plans visit for tomorrow; guard searches by code and tries check-in                                                                        | Visit not in Expected; check-in shows "This visit is scheduled for 04 Nov 2026"                               | AC-8  | P1 |
| TC-E2E-009  | Admin force-close                              | Visitor checked in; host never marks MO; admin force-closes with reason                                                                         | Visit `CHECKED_OUT`; leaves guard's On-premises list; audit shows reason                                      | FR-6.4 | P2 |
| TC-E2E-010  | First login and session                        | Admin creates user → user logs in → forced change password → logs out; log in again, idle 31 min (test clock), click anything               | Password changed; after idle redirected to login                                                             | FR-1.3 | P2 |

## 21. Non-functional (TC-NFR)

| ID          | Scenario                     | Method                                                                                                       | Pass criteria                                        | Ref    | P  |
|-------------|------------------------------|--------------------------------------------------------------------------------------------------------------|------------------------------------------------------|--------|----|
| TC-NFR-001  | Load                         | k6: 100 virtual users, 10 min, mix 50% dashboard reads, 30% list/search, 20% transitions, on production-like data (50 000 visits) | p95 < 2 s; error rate < 1%                          | NFR-1  | P1 |
| TC-NFR-002  | Real-time latency            | 50 approvals; measure commit → security SSE event                                                             | p95 < 5 s, max < 10 s                               | NFR-2  | P1 |
| TC-NFR-003  | Expiry accuracy              | Staging with real clock, window 10 min, 20 walk-ins; record `EXPIRED` time − deadline                          | All ≤ 60 s                                           | NFR-3  | P1 |
| TC-NFR-004  | Accessibility                | axe-core in Playwright on login, both dashboards, planned form, walk-in form, check-in, visit detail          | No serious or critical violations                    | NFR-11 | P2 |
| TC-NFR-005  | Push latency                 | Physical Android phone and iPhone on Wi-Fi; 20 walk-ins each; time from server send log to notification shown (screen recording timestamps) | p95 < 10 s | NFR-12 | P2 |
| TC-NFR-006  | App cold start               | Release build on a mid-range Android phone; 5 cold starts to home screen (`adb shell am start -W`) | Median < 3 s | NFR-13 | P3 |

## 22. Security (TC-SEC)

| ID          | Scenario                     | Method                                                                                                  | Pass criteria                                                                     | Ref    | P  |
|-------------|------------------------------|---------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------|--------|----|
| TC-SEC-001  | Security headers             | Inspect responses (prod config)                                                                         | CSP with `img-src 'self' blob:`, `frame-ancestors 'none'`, `nosniff`, HSTS         | NFR-5  | P1 |
| TC-SEC-002  | Cookie flags                 | Login in prod config                                                                                    | `Secure`, `HttpOnly`, `SameSite=Lax`                                               | NFR-5  | P1 |
| TC-SEC-003  | IDOR sweep                   | Script calls every `/visits/:id/*` endpoint as empB on empA's visits                                     | All 403                                                                           | BR-9   | P1 |
| TC-SEC-004  | Injection                    | SQL/NoSQL payloads in `q`, filters, path params                                                          | No 500; no extra data returned                                                    | NFR-5  | P1 |
| TC-SEC-005  | Stored XSS                   | Purpose `<img src=x onerror=alert(1)>`; view in all screens                                              | Rendered as text; no script runs                                                  | NFR-5  | P1 |
| TC-SEC-006  | Malicious images             | Upload `bomb.png`, JPEG/HTML polyglot                                                                    | `bomb.png` rejected by pixel limit before full decode (413 `FILE_TOO_LARGE`); polyglot re-encoded as pure JPEG; server memory stable; `FuzzProcessImage` runs 10 min with no panic | NFR-5  | P2 |
| TC-SEC-007  | Log hygiene                  | Run full integration suite; grep logs                                                                    | No passwords, reset tokens, session IDs, or full mobile numbers                    | spec §13 | P1 |
| TC-SEC-008  | Production surface           | Start with `APP_ENV=production`; call `/api/test/clock`; run `govulncheck ./...` and `npm audit` (web)    | 404; no reachable Go vulnerabilities; no high/critical npm vulnerabilities         | NFR-5  | P1 |

## 23. Database setup (TC-DBS)

| ID          | Lvl | Scenario                         | Steps / data                                                                                         | Expected                                                                                                   | Ref        | P  |
|-------------|-----|----------------------------------|------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------|------------|----|
| TC-DBS-001  | I   | Local setup from scratch         | Clean machine: `make db-reset`                                                                        | Roles `gatepass_owner` and `gatepass_app` exist; migrations applied; seed users present; `make run` starts  | spec §5.5  | P2 |
| TC-DBS-002  | I   | Migrations are reversible        | Empty DB: `migrate up`, `migrate down` to version 0, `migrate up` again; `migrate status`             | Every step succeeds; final schema identical to first `up` (compare `pg_dump --schema-only`); no pending      | spec §5.5  | P1 |
| TC-DBS-003  | I   | App role privileges              | Connected as `gatepass_app`: `CREATE TABLE`, `DROP TABLE visits`, `INSERT`/`SELECT` on `visits`, `UPDATE` on a table added by a later test migration | DDL fails; DML on `visits` works; new table is usable (default privileges applied)            | spec §5.5  | P1 |
| TC-DBS-004  | I   | GORM conventions                 | Insert duplicate email via repo; fetch missing ID; clear `decision_reason` via repo update; capture SQL log | `apperr.ErrDuplicate`; `apperr.ErrNotFound`; column set to NULL (not skipped); log shows `?`/`$n` placeholders, no visitor names or mobiles | spec §5.4  | P1 |
| TC-DBS-005  | I   | Transaction helper               | `WithTx` registering an after-commit callback: (a) returns nil, (b) returns error, (c) panics          | (a) data committed, callback runs once; (b) rolled back, callback not run; (c) rolled back, panic re-raised, callback not run | spec §5.4  | P1 |

## 24. Mobile API — sessions (TC-MAUTH)

| ID           | Lvl | Scenario                          | Steps / data                                                                                   | Expected                                                                                          | Ref        | P  |
|--------------|-----|-----------------------------------|------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------|------------|----|
| TC-MAUTH-001 | I   | Mobile login                      | `POST /auth/login` with `X-Client: mobile` as empA; then `GET /visits` with `Authorization: Bearer <token>` | 200 `{ user, token }`, **no** `Set-Cookie`; bearer request 200                         | spec §7.1  | P1 |
| TC-MAUTH-002 | I   | Role restriction                  | Mobile login as guard; as admin                                                                 | guard 403 `MOBILE_NOT_ALLOWED`; admin 200                                                          | FR-10.1    | P1 |
| TC-MAUTH-003 | I   | CSRF rule by auth type            | Bearer `POST /visits/:id/approve` without `X-Requested-With`; cookie request without it         | Bearer 200; cookie 403                                                                             | spec §7.1  | P1 |
| TC-MAUTH-004 | I   | Mobile session lifetime           | Use token every 6 days for 29 days; then idle 7 days 1 min; separately use daily until day 30 + 1 min | Valid while used; 401 after 7 days idle; 401 after 30 days even with daily use               | NFR-15     | P1 |
| TC-MAUTH-005 | I   | Web sessions unchanged            | Web login, idle 31 min                                                                           | 401 (30-min rule still applies to web)                                                             | FR-1.3     | P1 |
| TC-MAUTH-006 | I   | Logout from one phone             | empA logged in on phone 1 and phone 2, both registered devices; logout phone 1                  | Phone 1 token 401 and its device row deleted; phone 2 token and device unaffected                  | AC-11      | P1 |
| TC-MAUTH-007 | I   | Revocation                        | Admin deactivates empA; separately empB resets password                                          | All their mobile tokens 401; all their device rows deleted                                          | spec §7.1  | P1 |
| TC-MAUTH-008 | I   | Bad tokens                        | Random token; token of destroyed session; malformed header `Bearer`                              | 401 for each                                                                                        | NFR-5      | P1 |

## 25. Mobile API — push and app config (TC-PUSH, TC-APPCFG)

| ID            | Lvl | Scenario                          | Steps / data                                                                                   | Expected                                                                                          | Ref        | P  |
|---------------|-----|-----------------------------------|------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------|------------|----|
| TC-PUSH-001   | I   | Register device                   | empA `POST /devices` with `ExponentPushToken[abc]`; empB registers same token later             | 201 for empA; after empB call the row belongs to empB/its session; `last_seen_at` updated          | spec §8.2  | P1 |
| TC-PUSH-002   | I   | Register validation               | Token `xyz`; web cookie session calls `/devices`                                                | 400; 403                                                                                          | spec §8.2  | P2 |
| TC-PUSH-003   | I   | Walk-in push content              | empA has 2 devices; guard creates walk-in for Ravi Kumar (Acme)                                   | Fake sender gets 2 messages: title "Visitor waiting for you", body contains "Ravi K. from Acme", `data {type: APPROVAL_REQUESTED, visitId}`, `channelId approvals`, `ttl` ≈ 1800; payload has no mobile number | FR-10.8, Req constraint 4 | P1 |
| TC-PUSH-004   | I   | Push failure isolation            | Sender returns 503 three times                                                                   | Walk-in 201; 3 attempts logged; in-app + email still created                                       | spec §9.3  | P1 |
| TC-PUSH-005   | I   | Receipts job                      | Tickets: one `DeviceNotRegistered` (20 min old), one `ok` (20 min old), one 5 min old; run job    | Bad device row deleted; both old tickets removed; 5-min ticket untouched                           | spec §10   | P2 |
| TC-PUSH-006   | I   | Host without app                  | empB has no devices; walk-in for empB                                                            | No push call; in-app + email sent                                                                  | Req §8.7   | P2 |
| TC-PUSH-007   | I   | Push catalogue                    | Trigger every notification type                                                                  | Pushes only for types marked "Yes" in spec §9.1; overstay pushed to host only; expiry pushed to host; none to SECURITY users | spec §9.1 | P1 |
| TC-PUSH-008   | I   | Remove current device             | empA with 2 devices calls `DELETE /devices/current` from session 1                               | Only session 1's device row removed                                                               | spec §8.2  | P3 |
| TC-PUSH-009   | I   | Stale devices                     | Device last seen 31 days ago and 29 days ago; run `purgeDevices`                                  | Only the 31-day device removed                                                                     | spec §10   | P3 |
| TC-APPCFG-001 | I   | App config                        | `GET /app-config` without auth                                                                    | 200 with `minSupportedVersion`, `latestVersion`, `supportEmail` from env                           | FR-10.12   | P2 |
| TC-APPCFG-002 | I   | Version gate                      | `MOBILE_MIN_VERSION=1.2.0`; bearer `GET /visits` with `X-App-Version: 1.1.9`, `1.2.0`; web request without header; `/app-config` with `1.0.0` | 426 `UPGRADE_REQUIRED`; 200; 200; 200 | FR-10.12   | P2 |

## 26. Mobile app components (TC-MOB) — Jest + React Native Testing Library + MSW

| ID          | Scenario                              | Steps / data                                                                                   | Expected                                                                                          | Ref        | P  |
|-------------|---------------------------------------|------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------|------------|----|
| TC-MOB-001  | Sign-in screen                        | Empty submit; API 401; API 423; API 403 `MOBILE_NOT_ALLOWED`                                    | Field errors; "Invalid email or password"; lockout message; "Please use the Gatepass web portal"  | FR-10.1    | P1 |
| TC-MOB-002  | Token storage                         | Successful sign-in; logout; any API 401                                                         | Token written to SecureStore (mock) only; logout calls API and clears token + cache; 401 → sign-in screen | NFR-14 | P1 |
| TC-MOB-003  | Forced password change                | `/auth/me` returns `mustChangePassword: true`                                                   | Only change-password screen reachable until done                                                   | FR-10.2    | P2 |
| TC-MOB-004  | Approval countdown                    | Pending card, 65 s left, device clock 10 min off, server `Date` correct; advance timers         | Shows true remaining time; at 0 buttons disabled and card shows "Expired"                          | FR-10.4    | P1 |
| TC-MOB-005  | Accept / Reject                       | Tap Accept → confirm; tap Reject → enter reason → confirm                                       | Correct API calls; list refetched; success toast                                                   | FR-10.5    | P1 |
| TC-MOB-006  | Meeting Over button                   | Today's visitors with `CHECKED_IN` and `SCHEDULED`                                              | Button only for `CHECKED_IN` (from `allowedActions`)                                               | FR-10.5    | P1 |
| TC-MOB-007  | Planned visit form validation         | Run all field cases from `testdata/validation_vectors.json` through the shared Zod schema; submit valid form | Same pass/fail as Go validator; valid submit creates visit and returns to list         | FR-10.6    | P1 |
| TC-MOB-008  | Notification tap                      | Simulate notification response with `data {visitId}` in cold start and background states        | App opens `visits/[id]` for that visit (after sign-in check)                                       | FR-10.8    | P1 |
| TC-MOB-009  | Push permission                       | Permission denied; then granted                                                                 | Denied: home shows warning with "Open settings"; granted: token fetched and `POST /devices` called | FR-10.9    | P1 |
| TC-MOB-010  | Offline                               | NetInfo offline with cached data                                                                | Offline banner; cached lists visible; Accept/Reject/MO/Save disabled                               | FR-10.11   | P2 |
| TC-MOB-011  | Update required                       | `/app-config` min version above app version; or any API 426                                     | Update-required screen with store/TestFlight link; no other screen reachable                      | FR-10.12   | P2 |
| TC-MOB-012  | Data freshness                        | App goes background → foreground; push received in foreground; home visible for 30 s            | Refetch on foreground and on push; two polling calls in 30 s; no polling when home not focused     | spec §12.3 | P2 |
| TC-MOB-013  | Logout cleanup                        | Logout with delivered notifications present                                                     | Delivered notifications dismissed; API logout called (removes device server-side)                  | AC-11      | P2 |
| TC-MOB-014  | Visit detail photos                   | Detail of checked-in visit                                                                      | Visitor photo requested with bearer header; no request for ID photo; cache policy memory           | FR-10.7, BR-10 | P2 |
| TC-MOB-015  | Notifications tab                     | 3 unread; tap one; "Mark all read"                                                              | Badge 3 → 2 → 0; tap opens visit                                                                   | FR-10.10   | P3 |
| TC-MOB-016  | Accessibility                         | Render key screens at font scale 2.0; query by accessibility label                              | Every action has a label/role; no text truncation of action buttons                               | NFR-11     | P2 |
| TC-MOB-017  | Biometric lock                        | Lock enabled; app background 6 min → foreground; auth fails, then succeeds                      | Content hidden until successful authentication                                                    | FR-10.13   | P3 |

## 27. Mobile end-to-end (TC-ME2E) — Maestro, Android emulator

Run against the local API with `APP_ENV=test` (test clock available) and a `development` build of the app. Server-side setup and checks (creating a walk-in, reading status) are done through the API from the Maestro `runScript` step. A notification tap is simulated by opening the deep link `gatepass://visits/<id>`; real push arrival is covered by the device checklist (TC-DEV).

| ID           | Scenario                     | Steps                                                                                         | Expected                                                                                   | Ref     | P  |
|--------------|------------------------------|-----------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------|---------|----|
| TC-ME2E-001  | Sign in                      | Launch, sign in as empA                                                                       | Home shows Pending approvals, Today, Upcoming sections                                      | FR-10.2 | P1 |
| TC-ME2E-002  | Create planned visit         | Visits → + → fill form for today → Save                                                       | Visit in My visits; API shows `SCHEDULED` with host empA                                    | FR-10.6 | P1 |
| TC-ME2E-003  | Accept walk-in               | API: guard creates walk-in for empA → wait for card (≤ 20 s) → Accept → confirm                | API status `APPROVED`                                                                       | AC-9    | P1 |
| TC-ME2E-004  | Reject walk-in               | Same, Reject with reason                                                                      | API status `REJECTED` with reason                                                           | FR-10.5 | P2 |
| TC-ME2E-005  | Expiry                       | Walk-in created; test clock +31 min; pull to refresh                                           | Card shows Expired; no Accept button                                                        | AC-4    | P2 |
| TC-ME2E-006  | Meeting Over                 | API: visit checked in for empA → Home → Meeting Over → confirm                                 | API status `MEETING_OVER`                                                                   | AC-10   | P1 |
| TC-ME2E-007  | Deep link and logout         | Open `gatepass://visits/<id>` → detail shown; Profile → Log out                               | Correct visit opens; after logout API shows no device token for that session                 | AC-11   | P1 |

## 28. Device testing (TC-DEV) — manual, physical phones

Level `M` (manual). Follow [testing_guide.md](testing_guide.md) §7–§8 for set-up. Record device model, OS version and app build for each run.

| ID          | Scenario                                   | Steps                                                                                         | Expected                                                                                  | Ref     | P  |
|-------------|--------------------------------------------|-----------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------|---------|----|
| TC-DEV-001  | Install and connect                        | Install `preview` build on Android phone on same Wi-Fi as laptop; sign in                     | Sign-in works against laptop API; home loads                                              | FR-10.2 | P1 |
| TC-DEV-002  | Push on Android in every app state         | Create walk-in from laptop (security web) with the app (a) open, (b) in background, (c) swiped away, (d) phone locked | Push arrives within 10 s each time with sound; content as specified | AC-9, NFR-12 | P1 |
| TC-DEV-003  | Push on iPhone in every app state          | Same as DEV-002 on iPhone (TestFlight or development build)                                   | Same                                                                                      | AC-9    | P1 |
| TC-DEV-004  | Tap from killed state                      | App swiped away; tap the push                                                                 | App opens directly on that visit; Accept works; security web shows `APPROVED` within 5 s   | AC-9    | P1 |
| TC-DEV-005  | Permission off then on                     | Deny notifications at first prompt; enable later in OS settings; return to app                | Warning shown while off; after return, warning gone and next walk-in push arrives          | FR-10.9 | P2 |
| TC-DEV-006  | Logout stops push                          | Log out on phone; create walk-in for that user                                                | No push on the phone; email and web in-app notification still arrive                      | AC-11   | P1 |
| TC-DEV-007  | Offline and recovery                       | Airplane mode on: open app; turn off                                                          | Offline banner, actions disabled; after reconnect data refreshes and actions work          | FR-10.11 | P2 |
| TC-DEV-008  | Do Not Disturb / battery saver             | Enable DND and battery saver; create walk-in                                                  | Behaviour recorded (Android `approvals` channel high importance); email still arrives      | Req risks | P3 |

## 29. Architecture rules (TC-ARCH)

These run in CI as part of `make lint`. Each check is tested against a small fixture tree (`backend/internal/testutil/archfixtures/`, `frontend/test/archfixtures/`) that contains deliberate violations, so a broken rule configuration is caught too.

| ID          | Lvl | Scenario                         | Steps / data                                                                                                      | Expected                                                                                              | Ref        | P  |
|-------------|-----|----------------------------------|-------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|------------|----|
| TC-ARCH-001 | U   | Go module boundaries             | Run `make arch-lint` on the real tree; then on fixtures where (a) `visits` imports `notifications`, (b) `platform/clock` imports `modules/users`, (c) `gorm` is imported in a module's `service.go`, (d) `apigen` is imported in `service.go`, (e) `reports` imports `photos` (not in the allowed graph) | Real tree passes. Each fixture fails with a message naming the file and rule (M-3/M-5/G-2/M-6/M-8) | spec §3.3  | P1 |
| TC-ARCH-002 | U   | Web and mobile module boundaries | ESLint on the real `frontend/` and `mobile/`; then on fixtures where (a) `modules/gate` imports `modules/visits/components/X` directly, (b) `shared/` imports a module, (c) a mobile route file in `app/` contains screen logic instead of re-exporting a module screen | Real trees pass. Fixtures (a) and (b) fail with `boundaries/element-types` / `boundaries/entry-point` errors; (c) fails the `app/` size rule (≤ 10 lines per route file) | spec §3.4 | P2 |

---

## Coverage summary

| Area                    | Cases | Acceptance criteria covered |
|-------------------------|-------|-----------------------------|
| Auth & users            | 31    | AC-7                        |
| Validation & state machine | 28 | AC-5, AC-6, AC-8            |
| Planned visits          | 18    | AC-1, AC-7                  |
| Walk-ins                | 18    | AC-2, AC-3, AC-4            |
| Photos, check-in/out, MO | 36   | AC-1, AC-5, AC-6, AC-8      |
| Notifications, jobs, dashboards | 30 | AC-2, AC-4             |
| Audit, settings, reports | 12   | —                           |
| Front-end components    | 27    | AC-2…AC-7                   |
| E2E                     | 10    | AC-1…AC-8                   |
| Non-functional & security | 14  | —                           |
| System & API flows      | 6     | AC-1, AC-2                  |
| Database setup          | 5     | —                           |
| Mobile API (sessions, push, config) | 19 | AC-9, AC-11           |
| Mobile app components   | 17    | AC-9, AC-10, AC-11          |
| Mobile E2E (Maestro)    | 7     | AC-4, AC-9, AC-10, AC-11    |
| Device testing (manual) | 8     | AC-9, AC-11                 |
| Architecture rules      | 2     | —                           |
| **Total**               | **288** |                           |
