# Build all Go components
all: proto build-daemon build-mcp

# Generate protobuf Go code
proto:
    buf lint
    buf generate

# Build the daemon binary
build-daemon: proto
    cd daemon && go build -o finch-daemon .

# Build the MCP server binary
build-mcp: proto
    cd mcp && go build -o finch-mcp .

# Build the Qt app
build-app:
    cmake -S app -B app/build
    cmake --build app/build --target finch-app

# Build and run the Qt app's Qt Quick Test suite (kept separate from `test`,
# which is Go-only — folding it in would force a Qt/C++ build in Go-only environments)
test-app:
    cmake -S app -B app/build
    cmake --build app/build
    ctest --test-dir app/build --output-on-failure

# Run all Go tests
test: test-core test-daemon test-mcp

# Run core package tests
test-core:
    cd core && go test ./...

# Run daemon tests
test-daemon: proto
    cd daemon && go test ./...

# Run MCP server tests
test-mcp: proto
    cd mcp && go test ./...

# Run golangci-lint on all Go modules
lint: lint-core lint-daemon lint-mcp

lint-core:
    cd core && golangci-lint run ./...

lint-daemon: proto
    cd daemon && golangci-lint run ./...

lint-mcp: proto
    cd mcp && golangci-lint run ./...

# Scan all Go modules for known vulnerabilities.
# Reports only advisories reachable from finch's own code; CI enforces the same check.
# For the complementary enumerating scan, see `.claude/rules/ci.md`.
vuln: vuln-core vuln-daemon vuln-mcp

vuln-core:
    cd core && govulncheck ./...

vuln-daemon: proto
    cd daemon && govulncheck ./...

vuln-mcp: proto
    cd mcp && govulncheck ./...

# Format all Go code
fmt:
    cd core && go fmt ./...
    cd daemon && go fmt ./...
    cd mcp && go fmt ./...

# Reconcile go.mod/go.sum across all Go modules (run after a dependency change)
tidy: proto
    cd core && go mod tidy
    cd daemon && go mod tidy
    cd mcp && go mod tidy

# Remove build artifacts
clean:
    rm -f daemon/finch-daemon mcp/finch-mcp
    rm -rf app/build
    rm -rf daemon/gen

# Install all components
install: install-service install-mcp

# Install systemd user service
# Uses cp+mv (atomic replacement) so the install succeeds even while the
# daemon binary is running — Linux prevents overwriting a running binary
# with cp alone ("text file busy"), but mv replaces the directory entry
# while the running process retains its file descriptor to the old inode.
install-service: build-daemon
    mkdir -p ~/.local/bin
    cp daemon/finch-daemon ~/.local/bin/finch-daemon.tmp && mv ~/.local/bin/finch-daemon.tmp ~/.local/bin/finch-daemon
    mkdir -p ~/.config/systemd/user
    cp daemon/finch.service ~/.config/systemd/user/
    systemctl --user daemon-reload
    systemctl --user enable --now finch.service

# Install MCP server binary (see install-service for cp+mv rationale)
install-mcp: build-mcp
    mkdir -p ~/.local/bin
    cp mcp/finch-mcp ~/.local/bin/finch-mcp.tmp && mv ~/.local/bin/finch-mcp.tmp ~/.local/bin/finch-mcp

# Uses cp+mv atomic replacement (see install-service for rationale).
# Install the Qt desktop app: binary, desktop entry, and icon (Linux / XDG)
install-app: build-app
    mkdir -p ~/.local/bin
    cp app/build/finch-app ~/.local/bin/finch-app.tmp && mv ~/.local/bin/finch-app.tmp ~/.local/bin/finch-app
    mkdir -p ~/.local/share/applications
    cp app/finch.desktop ~/.local/share/applications/
    mkdir -p ~/.local/share/icons/hicolor/scalable/apps
    cp app/finch.svg ~/.local/share/icons/hicolor/scalable/apps/finch.svg
    touch ~/.local/share/icons/hicolor
    # desktop-file-utils may be absent on minimal systems; skip rather than fail the install
    if command -v update-desktop-database >/dev/null 2>&1; then update-desktop-database ~/.local/share/applications; fi

# Install git hooks via lefthook
hooks:
    lefthook install
