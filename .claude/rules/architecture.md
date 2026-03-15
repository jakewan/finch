# System Architecture and Boundaries

## Layer Boundaries

The dependency graph is strictly one-directional:

- **Core** has no proto, gRPC, or MCP dependencies. It defines domain types and logic only.
- **Daemon** imports core. It adapts core types to proto types and serves gRPC.
- **MCP** imports the daemon's generated gRPC client. It adapts gRPC responses to MCP tool results.
- **App** communicates exclusively via gRPC. It has no Go dependencies.

NEVER introduce a dependency that violates this direction (e.g., core importing proto types, MCP importing core directly).

## Module Isolation

Each component is a separate Go module with its own `go.mod`. Inter-module dependencies use `replace` directives pointing to relative paths (e.g., `replace github.com/jakewan/finch/core => ../core`).

## Process Isolation

The daemon is the sole database owner. All other components access data through gRPC over a Unix socket. No component other than the daemon should open or reference the SQLite database.

## Interface-Based Repository Boundary

Core SHOULD expose interfaces defining the domain contract (e.g., per-aggregate or a unified store interface). This formalizes the boundary between domain logic and its consumers, and enables isolated testing of the daemon without requiring a real database.

## Double-Entry Accounting

Transfers create paired transactions: negative on source, positive on destination, plus a linking event for traceability. Both sides MUST be written in a single database transaction to maintain accounting consistency.

## Projection Composition

Complex projections are built by composing simpler operations: starting balance computation, recurring rule occurrence expansion, deduplication of recorded vs. projected entries, and running balance accumulation. Each step is a distinct function that can be tested independently.
