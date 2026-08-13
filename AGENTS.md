# AGENTS.md

Go module: `github.com/iszk1215/mora` (Go 1.25.0, no toolchain directive)

## Commands

- `make` - build + test + coverage.html + lint + frontend build
- `make test` - `go test -v $(GO_PKGS)`
- `make lint` - golangci-lint run
- `make frontend-lint` - `npm run lint` (frontend ESLint)
- `make lint-all` - lint + frontend-lint
- `make generate` - mockgen for `mockscm` and `udm`
- `make run` - `go test $(GO_PKGS)` then `bin/mora web --debug`
- `make frontend-build` - `npm run build` (frontend)
- `make frontend-test` - `npm run test` (frontend)
- `make test-all` - frontend-test + test
- `make frontend-coverage` - `npm run test:coverage` (frontend, outputs `frontend/coverage/lcov.info`)
- `make coverage-all` - `coverage.out` + `frontend-coverage`
- `make clean` - rm coverage.out coverage.html bin/mora

## Structure

- `main.go` → `cmd` (cobra CLI)
- `server` - web server (chi router, sqlite3 via sqlx)
- `core` - client/interfaces
- `udm` - user defined metrics (UDM)
- `mockscm` - SCM mocks (build tag `//go:build !oss`)

## Frontend

- React Router v7: import from `react-router` (not `react-router-dom`); use `react-router/dom` for `RouterProvider`
- Charts use ECharts (not chart.js)
- React 19 + ReactDOM 19 (matching `@types/react` 19)
- Vite 8 (rolldown bundler), Tailwind CSS v4 (`@tailwindcss/vite` plugin, no PostCSS), TypeScript 6.0
- Test: Vitest + `@testing-library/react` + jsdom
- Build output: `server/static/public/` (committed to git, `emptyOutDir: true`)
- Dev server: `cd frontend && npm run dev -- --no-open`

## Workflow

- All work must be done in `feature/<name>` branches
- Merge to `main` only when explicitly instructed by the user (use `merge-feature` skill)
- Use `proceed-implementation` skill for implementing new features
- Use `merge-feature` skill for merging completed feature branches into main

## Notes

- All source code (.go, .ts, .tsx, .css, etc.) must contain only ASCII characters. No Japanese or other non-ASCII characters in code files.
- Binary output: `bin/mora`
- Default config: `mora.conf` (flag: `-c`)
- Server default port: 4000 (flag: `-p`)
- Tests use in-memory sqlite3 (`sqlite3`, `:memory:?_loc=auto`)
- Static files embedded in `server/static`
- Coverage: `make coverage.html` (requires `coverage.out` from `go test -coverprofile`); frontend coverage via `make frontend-coverage` (outputs `frontend/coverage/lcov.info`)
- Frontend coverage: `make frontend-coverage` uses a file-based dependency on `frontend/coverage/lcov.info` for incremental builds
- Swagger docs: all API handlers carry swaggo annotations; `make swagger` runs `swag init -g main.go -o docs --parseFuncBody` (also part of `make generate`). Generated `docs/` is committed. Swagger UI served at `/swagger/`. Handlers registered as closures (e.g. api-keys, signup, auth) annotate inside the function body, hence `--parseFuncBody` is required.
- Two remotes: `origin` (GitHub, https://github.com/iszk1215/mora) and `gitea` (http://localhost:3001/kazuhisa/mora)

## Documentation

- [docs/system-overview.md](docs/system-overview.md) - Architecture, packages, routes, middleware, auth
- [docs/specs/tracker.md](docs/specs/tracker.md) - Tracker API spec (endpoints, auth, data model)
- [docs/specs/tracker-search.md](docs/specs/tracker-search.md) - Tracker search spec (top page search feature)
- [docs/specs/user-page.md](docs/specs/user-page.md) - User page spec (/users/:userName)
- [docs/specs/coverage.md](docs/specs/coverage.md) - Coverage URL spec (URLs, middleware, upload)
- [docs/decisions/0001-use-sqlite3.md](docs/decisions/0001-use-sqlite3.md) - ADR: SQLite3
- [docs/decisions/0002-use-go-chi.md](docs/decisions/0002-use-go-chi.md) - ADR: chi router

## UDM (User Defined Metrics)

Tracks custom metrics beyond code coverage. Data model: Metric → Item → Value (stored in `udm_metric`, `udm_item`, `udm_value` tables).

CLI: `mora udm metric [--create|--delete|--list]` / `mora udm value [--add|--list|--clear]`

## Data Migrations

`mora migrate -c mora.conf` runs one-time non-destructive data migrations directly on the DB file (recorded in `schema_migrations`). Currently `udm_to_tracker`: migrates UDM metrics/items/values into public `tracker`/`tracker_series`/`tracker_value` rows owned by the admin (id=1).
