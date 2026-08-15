# System Overview

## What is Mora

Mora is a code coverage tracker that integrates with GitHub and Gitea. It monitors repositories, tracks coverage over time, and provides a web UI for visualization.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.25 |
| HTTP Router | go-chi/chi v5 |
| Database | SQLite3 (mattn/go-sqlite3) |
| ORM/Query | sqlx |
| CLI | Cobra |
| Frontend | React 19, TypeScript 6, Vite 8, Tailwind CSS v4 |
| Charts | ECharts |
| Routing | React Router v7 |
| Testing (Go) | stretchr/testify, go.uber.org/mock |
| Testing (FE) | Vitest, @testing-library/react, Playwright (E2E) |

## Package Structure

```
main.go            Entry point
cmd/               Cobra CLI (root.go, web.go)
config/            TOML configuration
core/              Shared interfaces (Repository, RepositoryClient, context helpers)
render/            HTTP response helpers (JSON, error responses)
server/            Web server, auth, session, stores, static file serving
tracker/           Time-series tracker (CRUD, series, values, members, likes)
coverage/          Coverage tracking (store, handler, upload, profile parser)
udm/               User Defined Metrics (metric -> item -> value)
mockscm/           SCM mocks for testing
version/           Version info
frontend/          React SPA
e2e/               E2E test infrastructure (mock OAuth provider)
```

## Architecture Diagram

```
                    ┌─────────────┐
                    │   Browser   │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │   chi.Router │
                    │  (server.go) │
                    └──────┬──────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
   ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
   │   tracker   │ │  coverage   │ │     udm     │
   │   handler   │ │   handler   │ │   handler   │
   └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
          │                │                │
          └────────────────┼────────────────┘
                           │
                    ┌──────▼──────┐
                    │   SQLite3   │
                    └─────────────┘
```

## Route Mounting (server.go)

```
/                           Session middleware
├── GET  /api/providers     SCM/login provider list
├── GET  /api/me            Current user
├── GET  /api/config        Server config
├── /api/user/me/api-keys/* API key management
├── /api/repos
│   ├── GET  /              Repository list
│   └── /{repo_id}/udm/*    UDM (injectRepo middleware)
├── /api/trackers/*         Tracker (requireTrackerAuth)
├── /api/coverages
│   ├── POST /              Create coverage tracker (requireTrackerAuth)
│   └── /{trackerId}/*      Coverage (injectTrackerCoverage)
├── /login/*                OAuth login
├── /logout/*               Logout
├── /api/signup/*           User signup
├── /api/auth/*             Password auth
├── /swagger/*              Swagger UI (generated from swaggo annotations)
└── /                       SPA frontend (static files)
```

## Middleware Chain

### injectRepo (`/api/repos/{repo_id}/*`)
1. Parse `repo_id` from URL
2. Load repository and repository manager
3. Verify access (session token or API key)
4. Inject `core.Repository` and `core.RepositoryClient` into context

### requireTrackerAuth (`/api/trackers/*`)
1. Session cookie -> extract user ID
2. API Key bearer token -> look up user
3. Neither -> pass-through (anonymous, no 401)

### injectTrackerCoverage (`/api/coverages/{trackerId}/*`)
1. Parse `trackerId` from URL
2. Resolve to the linked repository via the `tracker_coverage` table (managed by coverage: `coverage.FindRepoByTrackerID()`), which stores the scm/namespace/name/url directly
3. Load repository and repository manager
4. Verify access
5. Inject into context

Coverage-type trackers are created via `POST /api/coverages` (the tracker package has no coverage knowledge). `coverage.CreateCoverageTracker` creates a `tracker` row through the `TrackerCreator` interface and links it via `coverage.Link`, returning 409 when the repository already has a coverage tracker.

## Authentication

| Method | Mechanism | Usage |
|--------|-----------|-------|
| OAuth2 | GitHub/Gitea/Google OAuth | Web login (session cookie); Google is login-only (no repository access) |
| Password | bcrypt hash | Alternative login (`user_password` table) |
| API Key | Bearer token | Programmatic access (`user_api_key` table) |
| Session | Cookie-based | Browser sessions (`MoraSession`) |

Google is configured as an `[[scm]]` entry with `scm = "google"` (login-only
provider, no SCM client). The default endpoints point at Google's OAuth2
endpoints; a non-Google `url` (e.g. for E2E mocks) derives the endpoints from
the configured URL. On signup the suggested username is derived from the
email's local part (sanitized), since Google accounts have no username.

## Usernames

- Usernames are unique (case-insensitive, enforced by the
  `idx_user_username` UNIQUE index on `user.username`).
- A username must be a URL-safe string: lowercase ASCII letters, digits,
  `-` and `_`, 1-32 characters, starting and ending with a letter or digit.
- A set of reserved names (`admin`, `api`, `login`, ...) cannot be claimed.
- On signup the provider username is used as the default but sanitized to
  conform to the rules; when the chosen name is taken, the confirm endpoint
  returns `409` with a `suggested_username` alternative.
- Usernames are currently immutable. The validation/suggestion logic lives in
  `server/username.go` as pure functions so a future rename feature can reuse
  it; `user.id` remains the stable identifier (usernames are not yet used in
  URLs).

## Database Tables

| Subsystem | Tables |
|-----------|--------|
| Server | `scm`, `repository`, `user`, `user_auth`, `user_api_key`, `user_password` |
| Coverage | `coverage`, `coverage_entry`, `coverage_block`, `tracker_coverage` |
| Tracker | `tracker`, `tracker_series`, `tracker_value`, `tracker_member`, `tracker_like` |
| UDM | `udm_metric`, `udm_item`, `udm_value` |
| Migrations | `schema_migrations` |

## Data Migrations

`mora migrate` (see `cmd/migrate.go`, `udm/migrate.go`) runs one-time, non-destructive
data migrations directly against the database file from the server config
(`mora.conf`, flag `-c`). Applied migrations are recorded in the
`schema_migrations` table, so re-running the command is a no-op.

Current migrations:

- **`udm_to_tracker`**: migrates repository-scoped UDM data
  (`udm_metric` -> `udm_item` -> `udm_value`) into repository-independent
  trackers (`tracker` -> `tracker_series` -> `tracker_value`). Each metric
  becomes a `public` `tracker` owned by the admin user (id=1), each item becomes
  an `int` series, and values are copied as floats. UDM tables/API/CLI are left
  intact; migration tracks are created via `tracker.NewService` first.

## Frontend

- React Router v7 with `RouterProvider` (not `react-router-dom`)
- ECharts for charts (not chart.js)
- Build output: `server/static/public/` (committed to git)
- Dev server: `cd frontend && npm run dev -- --no-open`
- All source files must contain only ASCII characters

## Build & Test

| Command | Description |
|---------|-------------|
| `make` | Build + test + lint + frontend build |
| `make test` | Go tests |
| `make frontend-test` | Frontend unit tests (Vitest) |
| `make test-all` | Frontend + Go tests |
| `make frontend-e2e` | E2E tests (Playwright) |
| `make lint` | golangci-lint |
| `make frontend-lint` | ESLint |
| `make run` | Test + start server (--debug) |

## Demo Mode

`mora web --demo` starts with in-memory SQLite and seeded test data:
- 5 users (demo, alice, bob, charlie, dave)
- ~40-50 trackers across all users with random visibility (public/private)
- 1-3 series per tracker, 10-20 values per series
- Random likes between users on public trackers
