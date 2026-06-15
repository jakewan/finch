# PR and Commit Conventions

How pull requests, commits, and the changelog work in this project.

## PR Descriptions

(extension point: `pr-description-format`)

Structure a PR body as:

- **Overview** — the purpose: what the change accomplishes and why. Lead with this.
- **How it works** (optional) — only for non-obvious mechanics a reviewer cannot infer from the diff.
- **Issue references** — closing keywords (`Closes #N`); repeat the keyword for each issue (`Closes #1, closes #2`).

Avoid:

- Enumerating the diff file-by-file — the diff already shows what changed.
- Narrating the drafting journey ("earlier this did X, then I changed it").
- Scaffolding headers with no content under them.
- Hard-wrapping prose at a fixed column. Write one long line per paragraph and let the renderer wrap.

## Commit Messages

(extension point: `squash-commit-format`)

Conventional Commits: `type(scope): subject`.

- **Types**: `feat`, `fix`, `refactor`, `docs`, `build`, `ci`, `test`, `chore`.
- **Scopes** (this project's areas): `core`, `daemon`, `mcp`, `api`, `proto`, `app`, `dx`, `skills`, `rules`, `ci`, `build`, `deps`. `dx` covers developer-experience work, mirroring the component the planning skills classify by.
- **Body**: prose explaining *why* the change was made — the motivation, constraint, or problem it solves — not a restatement of the diff.
- **Issue references**: `Closes #N` (or `Related to #N`); repeat the keyword per issue.

## Changelog

(extension point: `changelog-convention`)

This project keeps a changelog in `CHANGELOG.md` following [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A PR requires a changelog entry when it makes a **user-facing change**:

- New, changed, or removed gRPC RPCs or MCP tools.
- Observable behavior changes (projection results, validation, output shape).
- Bug fixes that affect user-visible results.
- Schema or migration changes that affect stored data.

No entry is needed for: internal refactors with no observable effect, test-only changes, CI/build/tooling, documentation, or agent rules and skills.

Add the entry under `## [Unreleased]` in the matching category — `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, or `Security`. Keep it concise, user-facing, and present-tense.

## Post-Merge Next Work

(extension point: `post-merge-next-work-suggestions`)

After a merge, suggest re-orienting before the next work: review the open backlog and the active milestone's status, and when the milestone is at or near completion, assess whether to close it and plan the next one.

## Branch Freshness

(extension point: `freshness-response-policy`)

When assessing how far a branch trails its base:

- **Up to date** — proceed.
- **Modestly behind** — note it; refreshing is optional.
- **Significantly behind, or conflicts are likely** — merge the base branch in first (this project merges, never rebases) before merging the PR.

## Code-Audit Fix vs. Defer

(extension point: `code-audit-practice`)

When acting on review findings:

- **Fix in place** when the finding is small, localized, and within the PR's stated purpose.
- **Defer to a GitHub issue** when addressing it would expand the PR's scope or is tangential to its purpose.
