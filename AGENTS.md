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
- `make frontend` - `npm run build` (frontend)
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
- Dev server: `make -C frontend dev`

## Workflow

- All work must be done in `feature/<name>` branches
- Merge to `main` only when explicitly instructed by the user

## Notes

- All source code (.go, .ts, .tsx, .css, etc.) must contain only ASCII characters. No Japanese or other non-ASCII characters in code files.
- Binary output: `bin/mora`
- Default config: `mora.conf` (flag: `-c`)
- Server default port: 4000 (flag: `-p`)
- Tests use in-memory sqlite3 (`sqlite3`, `:memory:?_loc=auto`)
- Static files embedded in `server/static`
- Coverage: `make coverage.html` (requires `coverage.out` from `go test -coverprofile`); frontend coverage via `make frontend-coverage` (outputs `frontend/coverage/lcov.info`)
- `frontend/Makefile`: renamed `coverage` target to `coverage-report` to avoid collision with `coverage/` directory; uses file-based dependency on `coverage/lcov.info` for incremental builds
- `upload.sh` post-processes lcov.info paths: `sed -i 's|^SF:src/|SF:frontend/src/|g'` (changes frontend-relative to repo-root-relative for mora upload)
- Two remotes: `origin` (GitHub, https://github.com/iszk1215/mora) and `gitea` (http://localhost:3001/kazuhisa/mora)

## Documentation

- [docs/system-overview.md](docs/system-overview.md) - Architecture, packages, routes, middleware, auth
- [docs/specs/tracker.md](docs/specs/tracker.md) - Tracker API spec (endpoints, auth, data model)
- [docs/specs/tracker-search.md](docs/specs/tracker-search.md) - Tracker search spec (top page search feature)
- [docs/specs/coverage.md](docs/specs/coverage.md) - Coverage URL spec (URLs, middleware, upload)
- [docs/decisions/0001-use-sqlite3.md](docs/decisions/0001-use-sqlite3.md) - ADR: SQLite3
- [docs/decisions/0002-use-go-chi.md](docs/decisions/0002-use-go-chi.md) - ADR: chi router

## UDM (User Defined Metrics)

Tracks custom metrics beyond code coverage. Data model: Metric → Item → Value (stored in `udm_metric`, `udm_item`, `udm_value` tables).

CLI: `mora udm metric [--create|--delete|--list]` / `mora udm value [--add|--list|--clear]`
