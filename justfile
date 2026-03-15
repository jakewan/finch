# Build all Go components
all: proto build-daemon build-mcp

# Generate protobuf Go code
proto:
    buf lint
    buf generate

# Build the daemon binary
build-daemon:
    cd daemon && go build -o finch-daemon .

# Build the MCP server binary
build-mcp:
    cd mcp && go build -o finch-mcp .

# Build the Qt app
build-app:
    cmake -S app -B app/build
    cmake --build app/build

# Run all Go tests
test: test-core test-daemon test-mcp

# Run core package tests
test-core:
    cd core && go test ./...

# Run daemon tests
test-daemon:
    cd daemon && go test ./...

# Run MCP server tests
test-mcp:
    cd mcp && go test ./...

# Run golangci-lint on all Go modules
lint: lint-core lint-daemon lint-mcp

lint-core:
    cd core && golangci-lint run ./...

lint-daemon:
    cd daemon && golangci-lint run ./...

lint-mcp:
    cd mcp && golangci-lint run ./...

# Format all Go code
fmt:
    cd core && go fmt ./...
    cd daemon && go fmt ./...
    cd mcp && go fmt ./...

# Remove build artifacts
clean:
    rm -f daemon/finch-daemon mcp/finch-mcp
    rm -rf app/build
    rm -rf daemon/gen

# Install all components
install: install-service install-mcp

# Install systemd user service
install-service: build-daemon
    mkdir -p ~/.local/bin
    cp daemon/finch-daemon ~/.local/bin/
    mkdir -p ~/.config/systemd/user
    cp daemon/finch.service ~/.config/systemd/user/
    systemctl --user daemon-reload
    systemctl --user enable --now finch.service

# Install MCP server binary
install-mcp: build-mcp
    mkdir -p ~/.local/bin
    cp mcp/finch-mcp ~/.local/bin/
