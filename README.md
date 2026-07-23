# Finch

Finch projects your personal finances forward — model accounts, recurring rules, and transfers, then see where your balances land over time.

You interact with it two ways, both backed by a single daemon that owns your data:

- a **desktop GUI** (the Qt app), and
- **any AI assistant** that speaks the [Model Context Protocol](https://modelcontextprotocol.io) (Claude Code and others) — the MCP server is what makes your finances reachable from an assistant, not just a terminal.

## Architecture

Four components communicating via gRPC over a Unix socket:

- **core/** — Go package with domain logic and SQLite storage (imported by daemon)
- **daemon/** — Go binary, gRPC server, sole database owner, runs as systemd user service
- **mcp/** — Go binary, MCP server exposing Finch to AI assistants; delegates to the daemon via gRPC
- **app/** — C++/Qt/QML desktop application, gRPC client of daemon

## Prerequisites

### Go toolchain and dev tools

Finch uses [mise](https://mise.jdx.dev/) to manage tool versions. See `mise.toml` for pinned versions of Go, just, buf, protoc plugins, and golangci-lint.

```bash
mise install
```

This installs every tool the build needs, including the `just` command runner the targets below are written for. mise has to be [activated in your shell](https://mise.jdx.dev/getting-started.html) for those tools to land on your `PATH` — without it, prefix commands with `mise exec --`.

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
just vuln          # Scan Go modules for known vulnerabilities (run just proto first)
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

This builds the MCP binary and copies it to `~/.local/bin/`. That installs the binary and nothing more — an MCP client still has to be told to spawn it, which is the next section. (`just install` does this and the daemon service together.)

### Connect it to an assistant

Finch's MCP server is a stdio server: your assistant spawns it as a child process and talks to it over stdin and stdout. You never launch it yourself.

Finch is build-from-source today — there are no prebuilt binaries — so start from [Prerequisites](#prerequisites), then install the daemon and the MCP server. You don't need the Qt system packages for this; the MCP server and daemon are Go-only.

```bash
mise install
just proto            # daemon/gen/ is generated and not committed; the MCP build needs it
just install-service  # the MCP server exits at startup if it can't reach the daemon
just install-mcp
```

Running the daemon *as a service* means Linux with systemd. The daemon and MCP server also carry macOS paths — the socket lives at `/tmp/finch-$UID/finch.sock` there — so the [manual path](#manual) can start the daemon on macOS, though it isn't tested and the troubleshooting below assumes systemd.

**Claude Code:**

```bash
claude mcp add --scope user finch ~/.local/bin/finch-mcp
```

`--scope user` makes Finch available in every project. Without it you get `local` scope — private to you, and only in the directory you ran the command from — which is what you want if Finch should appear only while you're working in one checkout. Your shell expands the `~` before `claude` records it, so the stored command is an absolute path.

**Any other MCP client:** add an entry to the client's MCP configuration file, consulting its documentation for where that file lives:

```json
{
  "mcpServers": {
    "finch": {
      "command": "/absolute/path/to/finch-mcp"
    }
  }
}
```

Use an absolute path — many clients don't expand `~`.

Verify by asking your assistant to run Finch's `ping` tool. It reports the daemon version.

#### When the server doesn't start

The MCP server writes errors to stderr, which most clients capture in an MCP server log rather than showing you. Read that log first and match the message:

| Message | Cause | Fix |
|---|---|---|
| `daemon socket not found at ...` | The daemon isn't running — or it is, and the server is looking in the wrong runtime directory. Compare the path in the message against `echo "$XDG_RUNTIME_DIR"/finch/finch.sock`. | `systemctl --user status finch.service`, and `just install-service` if the unit isn't installed. |
| `XDG_RUNTIME_DIR not set` | The client spawned the server without that variable, which is how the server locates the daemon's socket. A desktop-launched client may not pass it through. | Set it explicitly on the server entry, below. |
| `ping daemon: ...` | A socket file exists but nothing answers — a crashed daemon that left the socket behind (refused immediately), or a hung one (after a five-second timeout). | `systemctl --user restart finch.service` |

Don't diagnose the second case by checking whether `$XDG_RUNTIME_DIR/finch/finch.sock` exists from your shell: your shell has the variable set, so the check passes while the server keeps failing. Reading the variable is fine — it's how you get the value for the fix. Run `echo "$XDG_RUNTIME_DIR"` in a normal terminal and paste the result:

```json
{
  "mcpServers": {
    "finch": {
      "command": "/absolute/path/to/finch-mcp",
      "env": { "XDG_RUNTIME_DIR": "/run/user/1234" }
    }
  }
}
```

A wrong value here doesn't produce the same message twice — the server finds the variable set, builds a socket path under it, and reports `daemon socket not found` instead. If that message names a directory you don't recognize, this is why. With Claude Code the equivalent is `claude mcp add -e XDG_RUNTIME_DIR=/run/user/1234 ...`, though a terminal-launched client inherits the variable already and won't hit this.

Finch can't be reached from a sandboxed client (Flatpak, Snap) today. The sandbox gives the spawned server its own filesystem namespace, so neither the installed binary nor the daemon's socket is visible inside it, and pinning `XDG_RUNTIME_DIR` doesn't help — the sandbox rewrites it by design. Use a client running unsandboxed on the same machine as the daemon.

#### Working on Finch itself

Most clients spawn the MCP server once, at client startup, with no live reload. After changing anything under `mcp/`, `proto/`, `core/`, or `daemon/`, reinstall and restart your assistant session. The repo's `mcp-reload` skill (`.claude/skills/mcp-reload/`) walks that loop.

You register the same installed binary a user does — the path doesn't change while you work on Finch, only the scope you registered it at.

### Install the desktop app

```bash
just install-app
```

This builds the Qt app, copies the `finch-app` binary to `~/.local/bin/`, installs the desktop entry to `~/.local/share/applications/`, and the icon to `~/.local/share/icons/hicolor/scalable/apps/`. Finch then appears in the desktop application launcher (e.g. KDE Plasma's Kickoff). The app connects to the daemon, so install and start the service first.

### Manual

Run the daemon without systemd — an alternative to the service, not a supplement:

```bash
systemctl --user stop finch.service   # if the service is installed and running
daemon/finch-daemon
```

Stop the service first. The daemon removes and recreates the socket at startup, so a hand-started daemon silently takes it over from a running service and you end up with two processes on the same database. `systemctl --user start finch.service` puts things back.

There is no manual step for the MCP server: your assistant spawns `finch-mcp` itself (see [Connect it to an assistant](#connect-it-to-an-assistant)). Running the binary from a shell only checks that it can reach the daemon — with no client on the other end it just blocks.

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
