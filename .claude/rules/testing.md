# Testing Strategy and Conventions

## Test Isolation

Each test gets its own in-memory SQLite database via `openTestDB(t)`. Tests NEVER share database state. This eliminates ordering dependencies and enables parallel execution.

## Per-Component Strategy

- **Core:** Table-driven unit tests for domain logic, edge cases, and validation boundaries. Test event production (correct event types and payloads), not just read model state.
- **Daemon:** Integration tests against a real (in-memory) database via gRPC. These verify the full request path: proto deserialization → validation → core call → response serialization.
- **MCP:** BDD-style tests verifying tool behavior end-to-end. Each tool's test describes the behavior from the user's perspective.

## Test Helpers

Shared helpers (`openTestDB`, `createTestAccount`, `date()`) live in `*_test.go` files within the package under test. Keep helpers minimal and focused — a helper that does too much obscures what the test is actually verifying.

## What to Test

Test domain invariants, validation boundaries, error conditions, and event production. Do NOT test framework plumbing, generated code, or trivial getters/setters. A test earns its keep by catching regressions in behavior that matters.

## Qt App

Build verification (`just build-app`) is the current minimum for the Qt app. Introduce QML automated testing when the app has form-based interactions and data mutation workflows worth protecting with regression tests.
