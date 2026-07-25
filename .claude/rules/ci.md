# CI Conventions

The CI workflow runs build, test, and lint checks (Go test and lint, plus the Qt app build and its Quick Test suite). Catching failures locally avoids a slow push-wait-fail cycle. The checks below are scoped by which part of the tree a change touches.

## Local Checks by Change Area

Run the checks matching the files a change touches before pushing:

- **Go modules** (`core/`, `daemon/`, `mcp/`) — `just test` and `just lint`.
- **Qt app** (`app/`) — `just test-app`. This builds the app *and* runs the Qt Quick Test suite, mirroring what CI runs, so it is the fast feedback path.
- **Protobuf** (`proto/`) — `just proto` to regenerate. Generated code lives in `daemon/gen/` and is not committed, so it must regenerate cleanly.

This list is the single source of truth for the supplies below — they reference it rather than restating commands.

## Run on Demand, Enforced by CI

`just vuln` scans all three Go modules with `govulncheck`. It is deliberately **not** in the list above: the scan takes minutes, and that list feeds the pre-merge and readiness supplies, so including it would impose the cost at every merge on top of CI already running it. It is also not in the pre-push hook, for the same reason. Run it when changing dependencies or when you want the answer before pushing; otherwise let CI be the enforcing surface.

It needs generated proto code, so run `just proto` first.

### What the gate guarantees

`govulncheck` fails only on advisories it can **statically reach** from finch's own code. A scan reports a silent tail of advisories in imported packages and required modules that never surface, and static reachability is defeated by reflection and interface dispatch. So a green scan means "no reachable advisory," not "no known-vulnerable dependency." The broader question — is any dependency in the graph known-vulnerable at all — is answered by Dependabot alerts, which cover the full transitive closure but cannot tell you whether the code is reachable. The two are complements; neither alone is coverage.

So when scoping a dependency bump, read both surfaces rather than whichever one raised the alarm. `govulncheck -C <module> -scan module` lists every advisory affecting a module in the graph with no reachability filtering, which is what surfaces the two blind spots. Note the flag shape: module mode accepts no package pattern, so a trailing `./...` is rejected.

Each surface can be the only one that sees a given advisory. A Dependabot alert names **one** advisory, and its stated fix version clears that advisory rather than the package — an unalerted sibling in the same package can need a higher version. Going the other way, an advisory with no Go vulnerability database entry is invisible to `govulncheck` in every mode, and Dependabot is the only place it appears. A bump scoped from one surface alone closes some of what it looks like it closed.

The scan covers Go modules only. The Qt app's C++ dependencies (gRPC and protobuf from apt, Qt from the install action) are covered by neither mechanism.

### When an advisory has no available fix

The `govulncheck` step runs inside `test-and-lint`, a required check, so a reachable advisory blocks every Go-touching PR. When one lands with no released fix, the escape is the **bypass actor on the `main` ruleset** — not editing the workflow, which cannot work: `.github/workflows/ci-go.yml` is inside the `changes` job's `go` filter, so the PR that would disable the gate is itself blocked by the gate.

Using the bypass obliges filing an issue recording the advisory and the reason, so the exception is visible and gets revisited.

### Version pins are security-relevant

The Go pin in `mise.toml` is an exact patch, and it carries an end-of-life clock rather than being a compatibility floor: Go supports only the two most recent minors, and `govulncheck` fails on standard-library advisories, so a pin on an unsupported minor makes every new stdlib advisory unfixable against a merge-blocking gate. Keep it within one minor of current. The `golangci-lint` pin is coupled to it — a linter built against an older Go than the code it analyzes degrades to unreliable analysis.

These versions are pinned in both `mise.toml` and the workflows. `test-and-lint` asserts the two agree and fails with a PR annotation if they drift; collapsing the duplication to a single source is tracked separately.

## Path-Scoping CI Without Deadlocking Required Checks

`test-and-lint` and `proto-check` (in `ci-go.yml`) are required status checks enforced by the `main` branch ruleset. A required check that never reports leaves a PR stuck "Expected" and unmergeable.

This constrains how Go CI is scoped to skip unrelated changes: NEVER add an `on: paths` filter to a workflow that hosts a required check. A path-filtered workflow that doesn't match produces no check run, so the required check never reports and the PR cannot merge. Instead, keep the workflow triggering on every PR and gate the expensive jobs at the *job* level — a cheap `changes` job (using `dorny/paths-filter`) sets per-area outputs, and the required jobs use `if:` to skip when their area is untouched. A job skipped via `if:` still reports its check as passing, so unrelated PRs stay mergeable while consuming no compute. The gate job needs `pull-requests: read`, supplied by a workflow-level `permissions:` block.

The Qt `build` job is NOT a required check, so `ci-qt.yml` can safely use a workflow-level `on: paths` filter. The same applies to `security-scan.yml`, which uses one to self-test on the PR that changes it — the prohibition above is scoped to workflows hosting a required check, not to path filters generally.

`security-scan.yml` is a separate workflow rather than a job in `ci-go.yml` because a `schedule` trigger there would fire the `changes` gate job, and `dorny/paths-filter` has no base ref to diff against on a schedule event.

Two decay modes to know about, since both fail silently:

- GitHub disables scheduled triggers on public repos after 60 days without repository activity — the same quiet stretch the scheduled scan exists to cover. The disable notice arrives by email only.
- A failed scheduled run notifies only the last committer to the workflow file. `security-scan.yml` therefore opens or updates a GitHub issue on failure rather than relying on that.

When adding a file that participates in Go CI — a workflow it reuses, or config holding versions it asserts against — add it to the `changes` job's `go` filter. That filter lists paths individually, so a new file is invisible to the required job until it is named there, and a PR touching only that file skips `test-and-lint` and reports passing.

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
