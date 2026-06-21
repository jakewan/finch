# CI Conventions

The CI workflow runs build, test, and lint checks (Go test and lint, plus the Qt app build and its Quick Test suite). Catching failures locally avoids a slow push-wait-fail cycle. The checks below are scoped by which part of the tree a change touches.

## Local Checks by Change Area

Run the checks matching the files a change touches before pushing:

- **Go modules** (`core/`, `daemon/`, `mcp/`) — `just test` and `just lint`.
- **Qt app** (`app/`) — `just test-app`. This builds the app *and* runs the Qt Quick Test suite, mirroring what CI runs, so it is the fast feedback path.
- **Protobuf** (`proto/`) — `just proto` to regenerate. Generated code lives in `daemon/gen/` and is not committed, so it must regenerate cleanly.

This list is the single source of truth for the supplies below — they reference it rather than restating commands.

## Path-Scoping CI Without Deadlocking Required Checks

`test-and-lint` and `proto-check` (in `ci-go.yml`) are required status checks enforced by the `main` branch ruleset. A required check that never reports leaves a PR stuck "Expected" and unmergeable.

This constrains how Go CI is scoped to skip unrelated changes: NEVER add an `on: paths` filter to a workflow that hosts a required check. A path-filtered workflow that doesn't match produces no check run, so the required check never reports and the PR cannot merge. Instead, keep the workflow triggering on every PR and gate the expensive jobs at the *job* level — a cheap `changes` job (using `dorny/paths-filter`) sets per-area outputs, and the required jobs use `if:` to skip when their area is untouched. A job skipped via `if:` still reports its check as passing, so unrelated PRs stay mergeable while consuming no compute. The gate job needs `pull-requests: read`, supplied by a workflow-level `permissions:` block.

The Qt `build` job is NOT a required check, so `ci-qt.yml` can safely use a workflow-level `on: paths` filter.

## Pre-Merge Checks

(extension point: `pre-merge-checks`)

Before merging a PR, run the checks from "Local Checks by Change Area" matching the changed files. Surface any failure as a blocker and resolve it before proceeding.

## Readiness Checks

(extension point: `pr-readiness-checks`)

When reporting PR health, run the checks from "Local Checks by Change Area" matching the changed files and report each result. Also confirm no generated code (`daemon/gen/`) is tracked in the diff.

## Waste Patterns

(extension point: `pr-waste-patterns`)

Beyond the conflict-marker baseline, scan added lines for:

- Debug output left behind — `fmt.Println` / `log.Println` used for ad-hoc tracing.
- Unaddressed markers — `TODO`, `FIXME`, `HACK`.
- Test shortcuts — `t.Skip(...)` or focused/disabled tests committed unintentionally.
- `//nolint` directives without a trailing reason.
- Committed generated code under `daemon/gen/`.

## Stale-PR Smoke Tests

(extension point: `stale-pr-smoke-tests`)

After merging the base branch into a stale branch, run beyond the standard test/lint pass:

- `just proto` then `just all` — a clean regenerate-and-build is the only signal for proto or generated-code drift the merge may have introduced (generated code is not committed).
- `just test-app` when `app/` is touched — it builds the app and runs the Qt Quick Test suite (superseding a plain `just build-app`).
