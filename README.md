# Finch

Finch projects your personal finances forward — model accounts, recurring rules, and transfers, then see where your balances land over time.

You interact with it two ways, both backed by a single daemon that owns your data:

- a **desktop GUI** (the Qt app), and
- **any AI assistant** that speaks the [Model Context Protocol](https://modelcontextprotocol.io) (Claude Desktop and others) — the MCP server is what makes your finances reachable from an assistant, not just a terminal.

## Architecture

Four components communicating via gRPC over a Unix socket:

- **core/** — Go package with domain logic and SQLite storage (imported by daemon)
- **daemon/** — Go binary, gRPC server, sole database owner, runs as systemd user service
- **mcp/** — Go binary, MCP server exposing Finch to AI assistants; delegates to the daemon via gRPC
- **app/** — C++/Qt/QML desktop application, gRPC client of daemon

## Prerequisites

### Go toolchain and dev tools

Finch uses [mise](https://mise.jdx.dev/) to manage tool versions. See `mise.toml` for pinned versions of Go, buf, protoc plugins, and golangci-lint.

```bash
mise install
```

### System packages (Qt app only)

The C++/Qt app requires system-level packages. On Ubuntu/Debian:

```bash
sudo apt install \
  cmake \
  protobuf-compiler protobuf-compiler-grpc libprotobuf-dev libgrpc++-dev \
  qt6-base-dev qt6-declarative-dev qt6-charts-dev \
  libqt6quicktest6 \
  qml6-module-qtquick-controls qml6-module-qtquick-templates \
  qml6-module-qtquick-layouts qml6-module-qtquick-window \
  qml6-module-qtqml-workerscript qml6-module-qtqml-models \
  qml6-module-qtcharts qml6-module-qt-labs-settings qml6-module-qttest
```

The split matters: the `*-dev` packages are needed to **build**, while the `qml6-module-*`
packages are loaded at **runtime** — both to run the app and to run its Qt Quick Test suite
(`just test-app`). A build can succeed but the app or tests fail to start if a runtime QML
module is missing. `libqt6quicktest6` is the Qt Quick Test runtime library (its build-time
link symlink ships in `qt6-declarative-dev`); `qml6-module-qttest` is the matching QML module.
CI (`.github/workflows/ci-qt.yml`) builds the app and runs the headless test suite, so it
installs the build- and test-time subset of this list — it omits the modules only a full app
launch needs (e.g. `qml6-module-qt-labs-settings`), and `cmake` is preinstalled on the GitHub
runner. When you change a dependency, update this list and CI's apt list as the change
warrants.

## Getting Started

```bash
mise install       # Install pinned tool versions
just proto         # Generate protobuf Go code
just all           # Build daemon and MCP server
just build-app     # Build the Qt desktop app
just test          # Run all Go tests
just test-app      # Build and run the Qt app's Qt Quick Test suite
just lint          # Run golangci-lint on all modules
```

## Running Locally

### As a systemd user service (recommended)

```bash
just install-service
```

This builds the daemon, copies it to `~/.local/bin/`, installs the systemd unit, and starts the service.

### Install the MCP server

```bash
just install-mcp
```

This builds the MCP binary and copies it to `~/.local/bin/`.

### Install the desktop app

```bash
just install-app
```

This builds the Qt app, copies the `finch-app` binary to `~/.local/bin/`, installs the desktop entry to `~/.local/share/applications/`, and the icon to `~/.local/share/icons/hicolor/scalable/apps/`. Finch then appears in the desktop application launcher (e.g. KDE Plasma's Kickoff). The app connects to the daemon, so install and start the service first.

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

## Development

This project is developed with assistance from [Claude Code](https://claude.com/claude-code).
