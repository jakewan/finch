# Build Pipeline and Schema Evolution

## Buf Pipeline

`buf lint` + `buf generate` produces all proto code. NEVER use raw `protoc`. Generated code lives in `daemon/gen/` and is NOT committed to the repository — it is regenerated at build time.

## Just Targets

Generated code is a prerequisite of every target that loads a daemon or mcp package — building, testing, linting, scanning, and `just tidy` alike, since production and test files in both modules import `daemon/gen/`. Each of those recipes declares `proto` as a dependency rather than relying on the caller to remember, and `just` runs it once per invocation however many legs ask for it. The `core` recipes deliberately do not, so `just test-core` and its siblings need no protobuf toolchain. Per-module targets (`test-core`, `lint-daemon`, etc.) exist for focused development.

A recipe that loads Go packages without generated code present does not degrade quietly — it fails to load, naming the missing `daemon/gen/finch/v1` import. Any new recipe touching daemon or mcp wants the same `proto` dependency.

## Embedded Migrations

SQL migration files live in `core/migrations/` and are embedded in the binary via `//go:embed`. Numeric prefix determines execution order (e.g., `001_initial.sql`, `002_event_store.sql`). Migrations run automatically on `DB.Open()` and are tracked in the `schema_version` table.

## Adding a Migration

1. Create `NNN_descriptive_name.sql` with the next sequence number.
2. Each migration runs in its own database transaction.
3. Test the migration against both a fresh database and one with existing data and schema.
4. NEVER modify an existing migration file — create a new migration instead.

## Foreign Key Enforcement

`PRAGMA foreign_keys = ON` is executed on every `DB.Open()`. All reference columns in the schema define explicit foreign key constraints. Referential integrity is enforced by the database, not the application.
