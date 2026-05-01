# AGENTS.md

Go module: `github.com/iszk1215/mora` (Go 1.23+, toolchain go1.24.2)

## Commands

- `make` - build, test, coverage.html
- `make test` - `go test -v ./...`
- `make check` - golangci-lint run
- `make generate` - mockgen for `mora/mockscm` and `mora/udm`
- `make run` - test then `bin/mora web --debug`

## Structure

- `main.go` → `mora/cmd` (cobra CLI)
- `mora/server` - web server (chi router, sqlite3 via sqlx)
- `mora/core` - client/interfaces
- `mora/udm` - user/channel management
- `mora/mockscm` - SCM mocks (build tag `//go:build !oss`)

## Notes

- Binary output: `bin/mora`
- Default config: `mora.conf` (flag: `-c`)
- Server default port: 4000 (flag: `-p`)
- Tests use in-memory sqlite3 (`sqlite3`, `:memory:?_loc=auto`)
- Static files embedded in `mora/server/static`
- Coverage: `make coverage.html` (requires `coverage.out` from `go test -coverprofile`)
