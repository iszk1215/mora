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
├── GET  /api/providers     SCM list
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
2. Resolve to `repo_id` via `tracker_coverage` table (managed by coverage: `coverage.FindRepoIDByTrackerID()`)
3. Load repository and repository manager
4. Verify access
5. Inject into context

Coverage-type trackers are created via `POST /api/coverages` (the tracker package has no coverage knowledge). `coverage.CreateCoverageTracker` creates a `tracker` row through the `TrackerCreator` interface and links it via `coverage.Link`, returning 409 when the repository already has a coverage tracker.

## Authentication

| Method | Mechanism | Usage |
|--------|-----------|-------|
| OAuth2 | GitHub/Gitea OAuth | Web login (session cookie) |
| Password | bcrypt hash | Alternative login (`user_password` table) |
| API Key | Bearer token | Programmatic access (`user_api_key` table) |
| Session | Cookie-based | Browser sessions (`MoraSession`) |

## Database Tables

| Subsystem | Tables |
|-----------|--------|
| Server | `scm`, `repository`, `user`, `user_auth`, `user_api_key`, `user_password` |
| Coverage | `coverage`, `coverage_entry`, `coverage_block`, `tracker_coverage` |
| Tracker | `tracker`, `tracker_series`, `tracker_value`, `tracker_member`, `tracker_like` |
| UDM | `udm_metric`, `udm_item`, `udm_value` |

## Frontend

- React Router v7 with `RouterProvider` (not `react-router-dom`)
- ECharts for charts (not chart.js)
- Build output: `server/static/public/` (committed to git)
- Dev server: `make -C frontend dev`
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
