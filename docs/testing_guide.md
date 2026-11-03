# Gatepass — Testing Guide (laptop and mobile phone)

| Field   | Value                                                                                   |
|---------|-----------------------------------------------------------------------------------------|
| Version | 1.0                                                                                     |
| Date    | 2026-11-03                                                                               |
| Inputs  | [spec.md](spec.md), [tasks.md](tasks.md), [test_cases.md](test_cases.md)               |
| Status  | Written before development. Commands match the planned `Makefile` and project layout; task T-1006 updates anything that changes during the build. |

This guide explains how to run and test the whole system — Go API, web portal and employee mobile app — on a Linux laptop, and then on a real Android phone and iPhone.

---

## 1. What runs where

| Part                | Runs on                       | Address (local)                     | Used by                    |
|---------------------|-------------------------------|-------------------------------------|----------------------------|
| PostgreSQL 16       | Docker on the laptop          | `localhost:5432`                    | API                         |
| MailHog (fake email)| Docker on the laptop          | SMTP `localhost:1025`, inbox UI http://localhost:8025 | API; you, to read emails |
| Go API              | Laptop (`make run`)           | http://localhost:8080/api           | Web and mobile app          |
| Web portal (dev)    | Laptop (Vite)                 | http://localhost:5173               | Security, Admin, Employees  |
| Metro (mobile dev server) | Laptop (Expo)           | `localhost:8081`                    | Mobile development build    |
| Mobile app          | Android emulator, Android phone, iPhone | —                         | Employees                   |

```
 Android emulator ──(10.0.2.2:8080)──┐
 Phone on same Wi-Fi ──(<laptop-ip>:8080)──► Go API ──► PostgreSQL
 Phone anywhere ──(https tunnel)─────┘        │ └──► MailHog
 Browser ──(localhost:5173 → proxy)───────────┘ └──► Expo Push ──► phone
```

`APP_ID` below means the app's package / bundle ID chosen in task T-A01 (for example `com.company.gatepass`).

## 2. Prerequisites

### 2.1 On the laptop (Linux)

| Tool                 | Version     | Check command                    | Needed for                          |
|----------------------|-------------|----------------------------------|-------------------------------------|
| Git                  | any         | `git --version`                  | Everything                          |
| Docker + Compose v2  | recent      | `docker compose version`         | Database, MailHog                   |
| Go                   | 1.25+       | `go version`                     | API                                 |
| Node.js + npm        | 22 LTS      | `node -v && npm -v`              | Web, mobile, Playwright             |
| make                 | any         | `make -v`                        | Shortcuts                           |
| Chrome and Firefox   | latest      | —                                | Web testing (two users at once)     |
| Android Studio       | latest      | `adb version`                    | Android emulator, `adb`, Android SDK |
| Maestro              | latest      | `maestro --version`              | Mobile E2E tests                    |
| EAS CLI              | latest      | `eas --version` (`npm i -g eas-cli`) | Cloud builds for real phones     |
| cloudflared (optional) | latest    | `cloudflared --version`          | HTTPS tunnel to the laptop          |

After installing Android Studio, add the SDK tools to your shell profile:

```bash
export ANDROID_HOME="$HOME/Android/Sdk"
export PATH="$PATH:$ANDROID_HOME/platform-tools:$ANDROID_HOME/emulator"
```

Your user must be able to run Docker without `sudo` (`sudo usermod -aG docker $USER`, then log out and in). The Android emulator needs KVM: `ls /dev/kvm` must exist.

### 2.2 Accounts (only for real phones and push)

| Account                    | Needed for                                     | Task  |
|----------------------------|------------------------------------------------|-------|
| Expo (free)                | EAS builds, push service                        | T-A03 |
| Firebase project           | Push on Android (FCM)                           | T-A04 |
| Apple Developer (paid)     | Any build on an iPhone; TestFlight              | T-A02 |
| Google Play Console        | Internal testing track (optional for local testing — APKs can be installed directly) | T-A01 |

> **iPhone note:** iOS apps can only be built on macOS — or in Expo's cloud with `eas build`, which works from Linux. You cannot run the iOS Simulator on Linux, so iPhone testing is done on a real iPhone (§8.3).

## 3. First-time setup

```bash
git clone <repo-url> gatepass && cd gatepass
cp .env.example .env            # defaults work for local development
npm install                     # installs web, mobile, shared client, e2e
make gen                        # generate Go + TypeScript code from api/openapi.yaml
make db-up                      # start PostgreSQL + MailHog in Docker
make migrate-up                 # create tables
make seed                       # create test users (prints the credentials)
```

### Test accounts (seed data)

All passwords: **`Gatepass@123`** (local only).

| Email                     | Role     | Use it for                                  |
|---------------------------|----------|---------------------------------------------|
| `admin@gatepass.local`    | ADMIN    | Web admin; can also host visitors            |
| `empA@gatepass.local`     | EMPLOYEE | Main host — web and mobile app               |
| `empB@gatepass.local`     | EMPLOYEE | Second host (access-control checks)          |
| `guard@gatepass.local`    | SECURITY | Gate / security web portal                   |
| `guard2@gatepass.local`   | SECURITY | Second guard (concurrency, notifications)    |
| `exEmp@gatepass.local`    | EMPLOYEE (inactive) | Login must fail                  |

## 4. Running the system

Use three terminals:

```bash
# Terminal 1 — API (http://localhost:8080)
make run

# Terminal 2 — web portal (http://localhost:5173)
npm run dev -w frontend

# Terminal 3 — mobile dev server (only when testing the app)
make mobile-start               # = cd mobile && npx expo start --dev-client
```

Check the API: `curl http://localhost:8080/api/health` → `{"status":"ok"}`.
Emails sent by the system appear at **http://localhost:8025**.

### 4.1 Test-clock mode (for 30-minute expiry, overstay, no-show)

Waiting 30 minutes is not practical. Start the API in test mode, which enables the test clock:

```bash
APP_ENV=test make run
```

Move server time forward (times are IST):

```bash
curl -X POST http://localhost:8080/api/test/clock \
  -H 'Content-Type: application/json' -H 'X-Requested-With: gatepass' \
  -d '{"now":"2026-11-03T10:31:00+05:30"}'

# back to real time
curl -X POST http://localhost:8080/api/test/clock \
  -H 'Content-Type: application/json' -H 'X-Requested-With: gatepass' \
  -d '{"now":null}'
```

The jobs (expiry, reminders, overstay) use this clock on their next run (within 30–60 s). Never run `APP_ENV=test` on a shared or production server.

### 4.2 Push notifications locally

Push is off by default (`PUSH_ENABLED=false`, the server records pushes instead of sending them). To send real pushes to an emulator or phone, set in `.env`:

```
PUSH_ENABLED=true
EXPO_ACCESS_TOKEN=<token from T-A03>
```

and restart the API. The phone must be running a build that has the FCM/APNs credentials (T-A04).

## 5. Automated tests

| What                              | Command                    | Needs running                      |
|-----------------------------------|----------------------------|------------------------------------|
| Lint everything (incl. module boundary rules) | `make lint`  | —                                  |
| Module boundary rules only        | `make arch-lint`           | —                                  |
| Go unit tests                     | `make test`                | —                                  |
| Go integration tests (+ race detector, coverage) | `make test-int` | Docker (starts its own Postgres)  |
| Web component tests               | `npm test -w frontend`     | —                                  |
| Mobile component tests            | `make mobile-test`         | —                                  |
| Web end-to-end (Playwright)       | `make test-e2e`            | Docker; API is started by the suite in test mode |
| Mobile end-to-end (Maestro)       | `make mobile-e2e`          | API in test mode + Android emulator with dev build (§7) |
| Coverage report (Go)              | `make cover` → opens `coverage.html` | Docker                     |

All test case IDs are in [test_cases.md](test_cases.md). A failing automated test prints its test case ID in the test name (for example `TestWalkIn/TC-WI-010_accept_after_window`).

## 6. Manual testing of the web portal on the laptop

1. Sessions are per browser, so use **two browsers** (or a normal and an incognito window): Chrome as `guard@…` (security), Firefox as `empA@…` (employee).
2. The laptop webcam works for photo capture on `localhost`. Allow camera access when the browser asks. If there is no webcam, the check-in screen offers a file upload — use any JPEG.
3. Run the scenarios in §9 marked "Web".

## 7. Mobile app on the laptop (Android emulator)

### 7.1 Create the emulator (once)

1. Android Studio → **Device Manager** → **Create device** → *Pixel 7* (any recent phone).
2. System image: **API 35, "Google Play"** variant (Google Play is required for push).
3. Finish and press ▶ to start it. `adb devices` should list `emulator-5554`.

### 7.2 Point the app at your laptop

The emulator reaches the laptop at `10.0.2.2`. Create `mobile/.env.local`:

```
EXPO_PUBLIC_API_URL=http://10.0.2.2:8080
```

### 7.3 Build and run the development build

```bash
make mobile-android             # = cd mobile && npx expo run:android
```

The first build takes 5–15 minutes; later runs are fast. The app installs on the emulator and connects to Metro. Sign in as `empA@gatepass.local`. Code changes reload automatically.

> Expo Go (the generic app from the store) is **not** used for Gatepass, because push notifications and some native modules need a development build.

### 7.4 Push on the emulator

- With §4.2 configured and a Google Play emulator image, walk-ins created from the web portal produce real pushes on the emulator.
- Without push set up, simulate tapping a notification by opening the deep link:

```bash
adb shell am start -W -a android.intent.action.VIEW -d "gatepass://visits/<visit-id>" APP_ID
```

- To send a test push directly, read the device token from the database and call Expo:

```bash
make psql
#   select expo_push_token from device_tokens;
curl -X POST https://exp.host/--/api/v2/push/send -H 'Content-Type: application/json' \
  -d '{"to":"ExponentPushToken[xxxx]","title":"Test","body":"Hello from Gatepass"}'
```

### 7.5 Run the Maestro tests

```bash
curl -fsSL "https://get.maestro.mobile.dev" | bash     # install once
APP_ENV=test make run                                   # terminal 1
make mobile-e2e                                         # emulator running, dev build installed
```

Maestro Studio (`maestro studio`) helps when writing or debugging a flow.

## 8. Testing on a real phone

### 8.1 Network: let the phone reach the laptop

1. Put the phone and laptop on the **same Wi-Fi**. (Office Wi-Fi often blocks device-to-device traffic; a phone hotspot or home Wi-Fi is easier. Otherwise use the tunnel in §8.4.)
2. Find the laptop's IP: `hostname -I | awk '{print $1}'` → e.g. `192.168.1.20`.
3. Make the API listen on all interfaces — in `.env`: `HTTP_ADDR=0.0.0.0:8080` — and restart it.
4. Open the firewall (if `ufw` is active):

   ```bash
   sudo ufw allow 8080/tcp     # API
   sudo ufw allow 8081/tcp     # Metro (development builds only)
   ```

5. On the phone's browser open `http://192.168.1.20:8080/api/health` → should show `{"status":"ok"}`. If not, fix this before going further.

### 8.2 Android phone

**Option A — development build over USB (fast iteration)**

1. On the phone: Settings → About phone → tap **Build number** 7 times → Developer options → enable **USB debugging**.
2. Connect by USB, accept the prompt on the phone; `adb devices` lists it.
3. Run:

   ```bash
   cd mobile
   EXPO_PUBLIC_API_URL=http://192.168.1.20:8080 npx expo run:android --device
   ```

**Option B — installable APK from the cloud (no cable, shareable with testers)**

1. Set the API URL for the `preview` profile in `mobile/eas.json` (`"env": { "EXPO_PUBLIC_API_URL": "http://192.168.1.20:8080" }`) or use the tunnel URL from §8.4.
2. Build: `cd mobile && eas build -p android --profile preview`
3. Open the link / QR code shown at the end on the phone, download the APK and install it (allow "Install unknown apps" for the browser).

### 8.3 iPhone

An Apple Developer account (T-A02) is required for any install on an iPhone.

**Option A — development build (for developers)**

```bash
cd mobile
eas device:create                          # open the link on the iPhone to register it (once)
eas build -p ios --profile development     # cloud build, works from Linux
```

Install from the QR code / link on the iPhone (enable **Developer Mode** in Settings → Privacy & Security when asked). Then run `make mobile-start` on the laptop; the app connects to Metro over Wi-Fi.

**Option B — TestFlight (for UAT testers)**

```bash
eas build -p ios --profile production
eas submit -p ios
```

Testers install the **TestFlight** app and accept the invite. TestFlight and `production` builds only allow **HTTPS** API URLs, so use the tunnel (§8.4) or a real server.

### 8.4 HTTPS tunnel (optional)

Use a tunnel when the phone cannot reach the laptop directly, for iPhone non-development builds, and for testing the **web** portal's camera on a phone or tablet (browsers allow the camera only on HTTPS).

```bash
make build && ./bin/gatepass        # serves API + built web portal together on :8080
cloudflared tunnel --url http://localhost:8080
# → https://<random-words>.trycloudflare.com
```

- Mobile app: set `EXPO_PUBLIC_API_URL=https://<random-words>.trycloudflare.com` and rebuild/restart.
- Web on phone/tablet: open the same `https://…` URL; set `WEB_ORIGIN` in `.env` to that URL and restart.

> The tunnel makes your laptop reachable from the internet. Use seed data only, and stop the tunnel (Ctrl+C) when you finish.

### 8.5 Before you start device testing

- [ ] `api/health` opens from the phone's browser
- [ ] App signs in as `empA@gatepass.local`
- [ ] Notifications allowed (Profile tab shows "Notifications: On")
- [ ] For push: `PUSH_ENABLED=true` and `EXPO_ACCESS_TOKEN` set; `device_tokens` has a row for the phone
- [ ] Laptop has the security web portal open as `guard@gatepass.local`

## 9. Manual test scenarios (laptop + phone)

Run these after the automated suites pass. Record **Pass / Fail**, the device, OS version and app build. The IDs link to [test_cases.md](test_cases.md).

| #   | Scenario                         | Where              | Steps                                                                                                        | Expected                                                                                   | Covers |
|-----|----------------------------------|--------------------|--------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------|--------|
| S1  | Planned visit, full flow         | Phone + laptop web | Phone (empA): create planned visit for today → Laptop (guard): Expected tab → open → capture both photos with webcam → Check in → Phone: "Visitor arrived" push → Meeting Over → Laptop: Ready for check-out → Check out | Phone shows timeline ending "Checked out"; email in MailHog for check-in | AC-1, AC-10, TC-DEV-002 |
| S2  | Walk-in accepted from push       | Laptop web + phone | Laptop (guard): Walk-in → whom to meet = empA → submit. Phone locked, app swiped away                          | Push within 10 s; tap opens the request; Accept → laptop card turns Approved within 5 s without reload | AC-9, TC-DEV-002…004 |
| S3  | Walk-in rejected                 | Laptop web + phone | Same as S2, Reject with reason "Not expected"                                                                 | Laptop shows Rejected + reason; no Check-in button                                          | AC-3 |
| S4  | Walk-in expired                  | Laptop web + phone | API in test mode (§4.1). Create walk-in at real time T, set clock to T + 31 min                               | Within 1 min: laptop shows Expired; phone card Expired (after refresh); push "Request expired" | AC-4 |
| S5  | Check-out blocked before MO      | Laptop web         | Check a visitor in; do not mark MO; look at On premises                                                        | No Check-out button; message "Waiting for host to mark Meeting Over"                       | AC-5 |
| S6  | Check-in needs both photos       | Laptop web         | Open check-in, capture only visitor photo                                                                     | Check in button disabled                                                                    | AC-6 |
| S7  | Access control                   | Two browsers       | empB opens the URL of empA's visit                                                                            | Access denied page                                                                          | AC-7 |
| S8  | Wrong date                       | Laptop web         | empA plans visit for tomorrow; guard searches its code and tries check-in                                     | Not in Expected; check-in blocked with the visit date                                       | AC-8 |
| S9  | Logout stops push                | Phone + laptop     | Phone: Profile → Log out. Laptop: walk-in for empA                                                            | No push on phone; email in MailHog; web in-app notification for empA                        | AC-11, TC-DEV-006 |
| S10 | Permission off / on              | Phone              | Deny notifications at the prompt → enable later in OS settings → return to app                               | Warning shown, then gone; next walk-in pushes                                               | TC-DEV-005 |
| S11 | Offline                          | Phone              | Airplane mode on → open app → try Accept → airplane mode off                                                  | Offline banner, actions disabled; after reconnect data refreshes                            | TC-DEV-007 |
| S12 | Security cannot use the app      | Phone              | Sign in to the app as `guard@gatepass.local`                                                                  | "Please use the Gatepass web portal"                                                         | FR-10.1 |
| S13 | Do Not Disturb / battery saver   | Phone              | Enable both, create a walk-in                                                                                 | Record behaviour; email still arrives                                                       | TC-DEV-008 |

## 10. Troubleshooting

| Problem                                          | Likely cause / fix                                                                                         |
|--------------------------------------------------|------------------------------------------------------------------------------------------------------------|
| `make db-up` fails: port 5432 in use             | A local Postgres is running: `sudo systemctl stop postgresql`, or change the port in `docker-compose.yml` and `.env` |
| API exits at start with "missing config"         | A value in `.env` is empty; compare with `.env.example`                                                    |
| Integration tests can't start Postgres           | Docker not running or your user isn't in the `docker` group                                                |
| Web shows 401 straight after login               | Opened the API port (8080) and web port (5173) mixed up; use http://localhost:5173 in development           |
| Camera not available in the browser              | Page not on `localhost` or HTTPS; or permission blocked — reset it in the browser's site settings          |
| Emulator app shows "Network request failed"      | `EXPO_PUBLIC_API_URL` must be `http://10.0.2.2:8080` on the emulator; restart Metro with `npx expo start -c` after changing it |
| Phone can't open `http://<laptop-ip>:8080`       | Different Wi-Fi, `HTTP_ADDR` still `localhost`, firewall, or Wi-Fi client isolation — use a hotspot or the tunnel |
| iPhone app can't reach an `http://` URL          | iOS requires HTTPS outside development builds — use the tunnel                                              |
| No push on the phone                             | Check in order: notifications allowed on the phone; row in `device_tokens`; `PUSH_ENABLED=true` and `EXPO_ACCESS_TOKEN` set; API log shows the push ticket; build has FCM/APNs credentials (T-A04); Android emulator uses a Google Play image |
| Push arrives late on Android                     | Battery optimisation — Settings → Apps → Gatepass → Battery → Unrestricted                                  |
| App shows "Update required"                      | `MOBILE_MIN_VERSION` in `.env` is higher than the app version; lower it or install the newer build          |
| Expiry doesn't happen after moving the test clock | API not started with `APP_ENV=test`, or wait up to 60 s for the next job run                               |
| Want a clean slate                               | `make db-reset` (database), `adb shell pm clear APP_ID` (Android app data), delete and reinstall on iPhone |

## 11. Reporting a bug

Copy this template into the issue tracker:

```
Title: [Web|Mobile|API] <short summary>
Test case / scenario: TC-… or S…
Environment: laptop / emulator / device model + OS version, app build or commit
Account used: e.g. empA@gatepass.local
Steps:
 1.
 2.
Expected:
Actual:
Visit code (GP-…) and time (IST):
Attachments: screenshot / screen recording; API log lines around the time (no passwords)
```
