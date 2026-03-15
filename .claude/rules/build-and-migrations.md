# Build Pipeline and Schema Evolution

## Buf Pipeline

`buf lint` + `buf generate` produces all proto code. NEVER use raw `protoc`. Generated code lives in `daemon/gen/` and is NOT committed to the repository — it is regenerated at build time.

## Just Targets

`just proto` MUST run before `just build-*`. The `just all` target runs the full dependency chain: proto → build-daemon → build-mcp. Per-module targets (`test-core`, `lint-daemon`, etc.) exist for focused development.

## Embedded Migrations

SQL migration files live in `core/migrations/` and are embedded in the binary via `//go:embed`. Numeric prefix determines execution order (e.g., `001_initial.sql`, `002_event_store.sql`). Migrations run automatically on `DB.Open()` and are tracked in the `schema_version` table.

## Adding a Migration

1. Create `NNN_descriptive_name.sql` with the next sequence number.
2. Each migration runs in its own database transaction.
3. Test the migration against both a fresh database and one with existing data and schema.
4. NEVER modify an existing migration file — create a new migration instead.

## Foreign Key Enforcement

`PRAGMA foreign_keys = ON` is executed on every `DB.Open()`. All reference columns in the schema define explicit foreign key constraints. Referential integrity is enforced by the database, not the application.
