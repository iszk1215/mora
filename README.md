# Mora

Mora is a code coverage tracker that integrates with source code management systems (GitHub, Gitea) to monitor and display code coverage for repositories.

## Install

```bash
go install github.com/iszk1215/mora@latest
```

## Usage

### Start the web server

```bash
mora web
```

Options:
- `-p, --port` - Port number (default: 4000)
- `-c, --config` - Config file path (default: `mora.conf`)
- `-d, --debug` - Enable debug logging

### Commands

```bash
mora web              # Start web server
mora coverage         # Coverage related commands
mora udm              # User/channel management
```

## Configuration

Create a `mora.conf` file (TOML format) to configure the server and SCM integrations.

## Development

See [AGENTS.md](AGENTS.md) for build commands, test instructions, and project structure.

### Quick start for developers

```bash
make run    # Run tests and start server with debug mode
make check  # Run linter
make test   # Run all tests
```
