# Finch — Personal Finance Projection Tool

## Quick Start

```bash
just hooks    # Install git hooks (one-time setup)
just all      # Build everything
just test     # Run all tests
just lint     # Run linters
just proto    # Regenerate protobuf code
just vuln     # Scan Go modules for known vulnerabilities (needs `just proto` first)
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
- **Socket path:** `$XDG_RUNTIME_DIR/finch/finch.sock` (Linux), `/tmp/finch-$UID/finch.sock` (macOS). Any other platform is a hard error — there is no fallback.
- **DB path:** `~/.local/share/finch/finch.db` (Linux), `~/Library/Application Support/finch/finch.db` (macOS). Override with `FINCH_DB_PATH`.

## Testing

```bash
just test         # All Go tests
just test-core    # Core package only
just test-daemon  # Daemon only
just test-mcp     # MCP server only
```

## Go Module Dependencies

The daemon and mcp modules depend on core via `replace` directives pointing to `../core`. Because they resolve core's full dependency graph through those directives, a dependency change that shifts core's transitive versions (e.g. a Dependabot bump) leaves their `go.sum` stale. Run `just tidy` to reconcile all three modules after any such change.

CI verifies this rather than trusting it: `test-and-lint` runs `go mod tidy -diff` per module, so a missed reconciliation fails the build instead of sitting latent.

## CI Notes

- buf-action `breaking` must be `false` until the base branch has proto files (otherwise the breaking change check fails with no baseline to compare against).
- buf-action `pr_comment` requires write permissions — set to `false` if not needed.
- CI must run `buf generate` (with protoc plugin installation) before lint/test steps, since generated protobuf code is not committed.

## Linting

- golangci-lint v2 moved generated-code exclusion to `linters.exclusions.paths` (not `issues.exclude-dirs` or `run.exclude-dirs` — both are invalid in v2).
