# Testing Strategy and Conventions

## Test Isolation

Each test gets its own temporary on-disk SQLite database via `openTestDB(t)` (which uses `t.TempDir()` for automatic cleanup). Tests NEVER share database state. This eliminates ordering dependencies and enables parallel execution.

## Per-Component Strategy

- **Core:** Table-driven unit tests for domain logic, edge cases, and validation boundaries. Test event production (correct event types and payloads), not just read model state.
- **Daemon:** Integration tests against a real (in-memory) database via gRPC. These verify the full request path: proto deserialization → validation → core call → response serialization.
- **MCP:** BDD-style tests verifying tool behavior end-to-end. Each tool's test describes the behavior from the user's perspective.

## Test Helpers

Shared helpers (`openTestDB`, `createTestAccount`, `date()`) live in `*_test.go` files within the package under test. Keep helpers minimal and focused — a helper that does too much obscures what the test is actually verifying.

## What to Test

Test domain invariants, validation boundaries, error conditions, and event production. Do NOT test framework plumbing, generated code, or trivial getters/setters. A test earns its keep by catching regressions in behavior that matters.

## Qt App

The Qt app has a Qt Quick Test suite, run via `just test-app` (which builds the app, then runs the specs). A spec drives the real shipping component with an injected mock client and asserts on the calls that component makes — so behavior is verified, not just compilation. Build verification alone is no longer the ceiling; a form or view with input validation or data mutation earns a spec. See `qt-app.md` for the harness mechanics (test-only QML module, mock injection, headless/style settings).
