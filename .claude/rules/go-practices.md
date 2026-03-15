# Go Idioms and Conventions

## Error Wrapping

ALWAYS use `fmt.Errorf("context: %w", err)`. Each call site adds human-readable context describing what operation failed. Errors MUST be unwrappable via `errors.Is`/`errors.As`.

## Resource Cleanup

ALWAYS defer cleanup for closeable resources:

```go
defer func() { _ = resource.Close() }()
```

This applies to DB connections, gRPC connections, SQL rows, and network listeners. The `_ =` suppresses linter warnings about unchecked close errors.

## Enum Validation

Unspecified/zero enum values are always invalid. Validation functions check range (e.g., `freq >= FrequencyWeekly && freq <= FrequencyYearly`), not exhaustive match against every value. This ensures new enum values are valid by default if they fall within the range.

## Time Handling

- UTC everywhere. NEVER store or compare times in local zones.
- `time.DateOnly` format (`"2006-01-02"`) for date strings in JSON, proto, and SQL.
- Unix timestamps (`INTEGER`) for database storage of full timestamps.
- Use `time.Parse(time.DateOnly, s)` and `t.Format(time.DateOnly)` consistently.

## Params Structs

Complex operations (3+ parameters, optional fields) use dedicated parameter structs. Optional fields use pointer types (`*string`, `*time.Time`). This is clearer than long argument lists and enables named-field initialization.

## Scanner Helpers

Each entity type has a dedicated `scan*()` function that iterates `*sql.Rows`, handles null columns via `sql.NullString`, defers `rows.Close()`, and returns a typed slice. Scanner functions are the single point of truth for column-to-field mapping.

## Nullable Columns

Optional fields use `sql.NullString`/`sql.NullInt64` for scanning, `*string`/`*time.Time` in domain types, and `nilIfEmpty()` helpers for conversion between representations.

## Input Sanitization

Trim whitespace from string inputs before validation. A name that is whitespace-only MUST fail the non-empty check. Enforce reasonable length limits on user-provided strings (names, descriptions).

## Graceful Shutdown

Signal handling (SIGINT/SIGTERM) runs in a separate goroutine. Use `GracefulStop()` (not `Stop()`) to allow in-flight operations to complete. Defer cleanup for all resources opened during initialization.
