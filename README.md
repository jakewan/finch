# Finch

Personal finance projection tool. Four components communicating via gRPC over Unix socket:

- **core/** — Go package with domain logic and SQLite storage (imported by daemon)
- **daemon/** — Go binary, gRPC server, sole database owner, runs as systemd user service
- **mcp/** — Go binary, MCP server that delegates to daemon via gRPC
- **app/** — C++/Qt/QML application, gRPC client of daemon

## Prerequisites

### Go toolchain and dev tools

Finch uses [mise](https://mise.jdx.dev/) to manage tool versions. See `mise.toml` for pinned versions of Go, buf, protoc plugins, and golangci-lint.

```bash
mise install
```

### System packages (Qt app only)

The C++/Qt app requires system-level packages. On Ubuntu/Debian:

```bash
sudo apt install cmake qt6-base-dev qt6-declarative-dev libgrpc++-dev protobuf-compiler-grpc
```

## Getting Started

```bash
mise install       # Install pinned tool versions
just proto         # Generate protobuf Go code
just all           # Build daemon and MCP server
just test          # Run all Go tests
just lint          # Run golangci-lint on all modules
```

## Running Locally

### As a systemd user service (recommended)

```bash
just install-service
```

This builds the daemon, copies it to `~/.local/bin/`, installs the systemd unit, and starts the service.

### Manual

```bash
daemon/finch-daemon &   # Start the gRPC server
mcp/finch-mcp           # Start the MCP server (connects to daemon)
```

## Project Layout

| Directory | Description |
|-----------|-------------|
| `core/` | Domain logic and SQLite storage (Go module) |
| `daemon/` | gRPC server binary (Go module, depends on core) |
| `mcp/` | MCP server binary (Go module, depends on core) |
| `app/` | Qt/QML desktop application (C++) |
| `proto/` | Protobuf service definitions |
