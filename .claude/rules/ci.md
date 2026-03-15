# CI Conventions

## Build Verification

ALWAYS build the Qt app (`just build-app`) before pushing changes to `app/` files. The CI workflow only runs a build check — catching compilation errors locally avoids a slow push-wait-fail cycle.

## Go Checks

ALWAYS run `just test` and `just lint` before pushing changes to Go modules (`core/`, `daemon/`, `mcp/`).
