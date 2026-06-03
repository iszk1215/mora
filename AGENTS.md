# AGENTS.md

Go module: `github.com/iszk1215/mora` (Go 1.23+, toolchain go1.24.2)

## Commands

- `make` - build, test, coverage.html
- `make test` - `go test -v ./...`
- `make check` - golangci-lint run
- `make generate` - mockgen for `mockscm` and `udm`
- `make run` - test then `bin/mora web --debug`

## Structure

- `cmd/` - entry point + CLI (cobra, all `package main`)
- `server` - web server (chi router, sqlite3 via sqlx)
- `core` - client/interfaces
- `udm` - user defined metrics (UDM)
- `mockscm` - SCM mocks (build tag `//go:build !oss`)

## Notes

- Binary output: `bin/mora`
- Default config: `mora.conf` (flag: `-c`)
- Server default port: 4000 (flag: `-p`)
- Tests use in-memory sqlite3 (`sqlite3`, `:memory:?_loc=auto`)
- Static files embedded in `server/static`
- Coverage: `make coverage.html` (requires `coverage.out` from `go test -coverprofile`)

## UDM (User Defined Metrics)

Tracks custom metrics beyond code coverage. Data model: Metric → Item → Value (stored in `udm_metric`, `udm_item`, `udm_value` tables).

CLI: `mora udm metric [--create|--delete|--list]` / `mora udm value [--add|--list|--clear]`
