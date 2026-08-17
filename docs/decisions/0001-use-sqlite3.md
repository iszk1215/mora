# ADR 0001: Use libSQL (go-libsql) with Turso Cloud

## Status

Accepted

## Context

Mora needs a database to store coverage data, user accounts, tracker metrics, and configuration. The system runs as a single-binary web server intended for small-to-medium teams.

The original implementation used SQLite3 via `github.com/mattn/go-sqlite3` (CGo driver). Mora is now deployed on Cloud Run, where the container filesystem is ephemeral. A local SQLite file would be lost on every restart. Turso (libSQL Cloud) provides a hosted SQLite-compatible database that requires no SQL migration, while local `file:` mode remains available for development and demo.

## Decision

Use libSQL via `github.com/tursodatabase/go-libsql` (CGo driver) with `github.com/jmoiron/sqlx` as the query builder. All new code and tests use the `libsql` driver exclusively — `mattn/go-sqlite3` is no longer directly imported.

- **Production (Cloud Run):** Remote connection via `libsql://<db>.turso.io?authToken=<token>`, configured via `TursoURL` / `TursoAuthToken` in `mora.conf` (or `TURSO_DATABASE_URL` / `TURSO_AUTH_TOKEN` environment variables).
- **Development (local):** Local file via `file:mora.db` or `file::memory:`, configured via `DatabaseFilename` (empty defaults to in-memory).
- **Docker / external DB:** Connect to any libSQL-compatible server (`libsql://` or `https://` scheme).

## Consequences

### Positive

- Zero SQL migration required: libSQL is wire-compatible with SQLite, so existing `.db` files and all SQL (`datetime()`, `COLLATE NOCASE`, `AUTOINCREMENT`, etc.) work unchanged
- Turso Cloud removes the ephemeral-filesystem constraint on Cloud Run without introducing a different database engine
- Single-file local database remains available for development, testing, and demo mode (`:memory:`)
- `sqlx` provides struct scanning without sacrificing raw SQL control

### Negative

- CGo requirement complicates cross-compilation and CI builds (only `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` supported by go-libsql)
- Remote Turso connections introduce network latency and require auth token management
- Concurrent write throughput is limited by SQLite's file-level locking (local mode)
- `PRAGMA journal_mode=WAL` and `PRAGMA busy_timeout` are local-file settings only; they must be skipped or handled differently for remote connections
- Remote connections enable foreign key enforcement by default (`foreign_keys=ON`); test helpers that do not create parent tables must explicitly set `PRAGMA foreign_keys = OFF`

### Mitigations

- Tests use in-memory libSQL (`:memory:`) for speed; helpers default to `PRAGMA foreign_keys = OFF` to replicate the previous mattn driver behavior, with explicit `PRAGMA foreign_keys = ON` where FK cascade behavior is under test
- Demo mode uses in-memory libSQL with seeded data
- `SetMaxOpenConns(1)` is retained for both local and remote modes
