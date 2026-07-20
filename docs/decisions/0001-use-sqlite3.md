# ADR 0001: Use SQLite3 with mattn/go-sqlite3

## Status

Accepted

## Context

Mora needs a database to store coverage data, user accounts, tracker metrics, and configuration. The system runs as a single-binary web server intended for small-to-medium teams.

## Decision

Use SQLite3 via `github.com/mattn/go-sqlite3` (CGo driver) with `github.com/jmoiron/sqlx` as the query builder.

## Consequences

### Positive

- Zero runtime dependencies: no separate database server to install or manage
- Single-file database: easy backup, migration, and demo mode (`:memory:`)
- Sufficient performance for the expected workload (coverage uploads, tracker writes)
- `sqlx` provides struct scanning without sacrificing raw SQL control

### Negative

- CGo requirement complicates cross-compilation and CI builds
- Concurrent write throughput is limited by SQLite's file-level locking
- Not suitable for multi-server deployments (single-writer constraint)

### Future: PostgreSQL Support

SQLite3 is sufficient for single-server deployments, but PostgreSQL support is planned for production environments requiring concurrent access and better write throughput. The `sqlx` abstraction makes this feasible — most queries use standard SQL, and the `database/sql` interface supports multiple drivers. A migration path would involve:

- Introducing a `driver` config field (`"sqlite3"` or `"postgres"`)
- Handling SQL dialect differences (e.g., `AUTOINCREMENT` vs `SERIAL`, `DATETIME` vs `TIMESTAMP`)
- Keeping SQLite3 as the default for development and demo mode

### Mitigations

- Tests use in-memory SQLite (`:memory:?_loc=auto`) for speed
- Demo mode uses in-memory SQLite with seeded data
- Write contention is low in practice (coverage uploads are infrequent)
