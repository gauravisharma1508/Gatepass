# Gatepass — common tasks. `make help` lists them. Values come from .env (copy .env.example first).
-include .env
export

NODE_BIN ?=
BACKEND  := backend
WEB      := frontend

.PHONY: help
help:  ## show this help
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-16s %s\n", $$1, $$2}'

# ---- database ----
.PHONY: db-up db-down db-reset psql migrate-up migrate-down migrate-status seed
db-up:  ## start PostgreSQL and MailHog in Docker
	docker compose up -d
db-down:  ## stop PostgreSQL and MailHog
	docker compose down
db-reset:  ## wipe the local database, recreate, migrate and seed
	docker compose down -v && docker compose up -d && sleep 6 && $(MAKE) migrate-up seed
psql:  ## open psql as the application role
	psql "$(DATABASE_URL)"
migrate-up:  ## apply all migrations (owner role)
	cd $(BACKEND) && go run ./cmd/gatepass migrate up
migrate-down:  ## roll back the last migration
	cd $(BACKEND) && go run ./cmd/gatepass migrate down
migrate-status:  ## show migration status
	cd $(BACKEND) && go run ./cmd/gatepass migrate status
seed:  ## create the development users (password in SEED_PASSWORD)
	cd $(BACKEND) && go run ./cmd/gatepass seed

# ---- run ----
.PHONY: run run-test web-install web-dev
run:  ## run the API on HTTP_ADDR (migrates on start)
	cd $(BACKEND) && go run ./cmd/gatepass
run-test:  ## run the API in test mode (enables POST /api/test/clock)
	cd $(BACKEND) && APP_ENV=test go run ./cmd/gatepass
web-install:  ## install web dependencies
	cd $(WEB) && npm install
web-dev:  ## run the web app (Vite) on http://localhost:5173
	cd $(WEB) && npm run dev

# ---- quality ----
.PHONY: lint arch-lint fuzz test test-int cover web-test web-typecheck
lint: arch-lint  ## gofmt check, go vet, module-boundary rules
	cd $(BACKEND) && test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	cd $(BACKEND) && go vet ./... && go vet -tags=integration ./...
arch-lint:  ## check the module dependency rules (spec section 3.3)
	cd $(BACKEND) && go test -count=1 ./internal/archtest/
fuzz:  ## fuzz the photo pipeline for 60 seconds
	cd $(BACKEND) && go test ./internal/modules/photos -run '^$$' -fuzz FuzzProcess -fuzztime 60s
test:  ## Go unit tests (no database needed)
	cd $(BACKEND) && go test -race ./...
test-int:  ## Go integration tests against the local test database (make db-up first)
	cd $(BACKEND) && go test -race -tags=integration -count=1 ./...
cover:  ## integration tests with a coverage report (backend/coverage.html)
	cd $(BACKEND) && go test -race -tags=integration -count=1 -coverpkg=./internal/... -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html && go tool cover -func=coverage.out | tail -1
web-test:  ## web unit tests
	cd $(WEB) && npm test
web-typecheck:  ## web type check
	cd $(WEB) && npm run typecheck

# ---- build ----
.PHONY: build
build:  ## build the web app, embed it, and build bin/gatepass
	cd $(WEB) && npm ci && npm run build
	rm -rf $(BACKEND)/webui/dist && mkdir -p $(BACKEND)/webui/dist && cp -r $(WEB)/dist/. $(BACKEND)/webui/dist/
	cd $(BACKEND) && CGO_ENABLED=0 go build -trimpath -o ../bin/gatepass ./cmd/gatepass
