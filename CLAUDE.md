# Finch — Personal Finance Projection Tool

## Quick Start

```bash
just all      # Build everything
just test     # Run all tests
just lint     # Run linters
just proto    # Regenerate protobuf code
```

## Architecture

Four components communicating via gRPC over Unix socket:

1. **core/** — Go package with domain logic and SQLite storage (imported by daemon)
2. **daemon/** — Go binary, gRPC server, sole database owner, runs as systemd user service
3. **mcp/** — Go binary, MCP server that delegates to daemon via gRPC
4. **app/** — C++/Qt/QML application, gRPC client of daemon

## Key Conventions

- **Protobuf contract:** `proto/finch/v1/finch.proto` defines the API. Run `just proto` after changes.
- **Separate Go modules:** `core/`, `daemon/`, and `mcp/` each have their own `go.mod`.
- **Pure Go SQLite:** Uses `modernc.org/sqlite` (no cgo required for Go builds).
- **Buf for protobuf:** `buf.yaml` + `buf.gen.yaml` at repo root. Never use raw `protoc`.
- **Socket path:** `$XDG_RUNTIME_DIR/finch/finch.sock` (Linux), `/tmp/finch-$UID/finch.sock` (fallback).
- **DB path:** `~/.local/share/finch/finch.db` (Linux), `~/Library/Application Support/finch/finch.db` (macOS). Override with `FINCH_DB_PATH`.

## Testing

```bash
just test         # All Go tests
just test-core    # Core package only
just test-daemon  # Daemon only
just test-mcp     # MCP server only
```

## Go Module Dependencies

The daemon and mcp modules depend on core via `replace` directives pointing to `../core`.
