# Proto Conventions and API Layer

## Proto Message Naming

Each RPC has `<Operation>Request` and `<Operation>Response` message pairs. Enums ALWAYS have `_UNSPECIFIED = 0` as the first value. Field numbers are sequential with no gaps or reuse.

## Daemon Adapter Pattern

Daemon server methods adapt between proto types and core domain types. Proto enums and core enums share numeric values, enabling direct type casting (e.g., `core.AccountType(req.Type)`). If numeric alignment breaks, introduce an explicit mapping function rather than renumbering either side.

## gRPC Error Categorization

Map domain errors to appropriate gRPC status codes:

- `codes.NotFound` — aggregate does not exist
- `codes.InvalidArgument` — validation failure (bad input from caller)
- `codes.AlreadyExists` — duplicate creation attempt
- `codes.Internal` — unexpected errors only (database failures, marshaling errors)

NEVER use `codes.Internal` as a catch-all. Callers depend on status codes to distinguish recoverable from non-recoverable failures.

## Validation at the Boundary

Validate request fields in daemon handlers before calling core methods. Core methods also validate (defense in depth), but user-facing error messages originate from the API layer. This means the daemon should produce clear, specific `InvalidArgument` messages for malformed requests.

## MCP Delegation Pattern

Each MCP tool handler delegates to exactly one gRPC call. Define separate `*Input` and `*Output` structs per tool for schema generation. Tool handler factories return closures that capture the gRPC client.

## Context Deadline Discipline

Daemon handlers SHOULD set timeouts on core operations via `context.WithTimeout`. MCP handlers SHOULD propagate deadlines from the MCP framework. A slow query must not block indefinitely.
