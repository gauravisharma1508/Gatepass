# Mobile app: brief for a new session (fast-track scope)

Goal: an employee mobile app for Gatepass, built quickly for a submission. Read first: `README.md`, `docs/spec.md` (sections 7.1 and the mobile parts), `docs/requirements.md`, and `frontend/src/modules` (reuse the API client types and the Zod validation; shared vectors are in `testdata/validation_vectors.json`).

State: the Go back end (`backend/`) and the web app (`frontend/`) are finished and tested. The dev stack is `make db-up`, `make seed`, `make run`. Seed users are in README.md.

## Stack (decided)
React Native + Expo (managed), TypeScript strict, Expo Router, TanStack Query, Zod. **Not Flutter.** Run on a phone with Expo Go.

## Back end work (small)
- `POST /auth/login` with header `X-Client: mobile` returns `{ user, token }` and sets no cookie. Later requests use `Authorization: Bearer <token>` and need no CSRF header. Security users get `403 MOBILE_NOT_ALLOWED`.
- Keep every existing web behaviour and test passing (`make test`, `make test-int`).
- Add integration tests for the bearer login, bearer access, logout, and the Security block.
- SSE (`/api/events`) must work with the bearer token too, or the app polls instead.

## App scope (in)
Sign in, forced change-password, employee dashboard, my visits with filters, planned-visit form (create, edit, cancel), visit detail with timeline, walk-in accept/reject with a server-time countdown, Meeting Over, notifications list. Store the token in `expo-secure-store`.

## Out for now (say so in the final report)
Push notifications (needs an Expo account and a dev build), offline cache, biometric lock, `/app-config` version gate, Maestro tests. Use pull-to-refresh and polling or SSE for live updates.

## Done means
Typecheck and lint clean, Jest tests for the form validation, countdown and the main screens, a real run against the back end, and a short "how to run on my phone" section added to `docs/my_test_guide.md`. Report honestly what was and was not tested. Do not commit.
