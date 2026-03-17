---
name: project-summary
description: >-
  Surface component inventory, critical-path gate status, and issue hygiene
  for session orientation
---

# Project Summary

Structured overview of project state for session orientation. Replaces manual `gh issue list` queries with a single skill invocation that surfaces what matters most.

## Step 1: Gather Data

Run these three commands in parallel (no additional API calls after this):

**Command A** — all open issues with metadata:

```bash
gh issue list --state open --limit 200 --json number,title,labels,milestone,updatedAt,createdAt,body
```

**Command B** — active milestone progress:

```bash
gh api repos/{owner}/{repo}/milestones --jq '.[] | select(.state == "open") | {title, open: .open_issues, closed: .closed_issues}'
```

**Command C** — open PRs with CI status:

```bash
gh pr list --state open --json number,title,headRefName,statusCheckRollup,updatedAt
```

## Step 2: Classify Each Issue by Component

For each issue from Command A, assign a component in two passes:

### Pass 1: Label match (takes precedence)

| Label | Component |
|---|---|
| `app-independence` | **app** |
| `github_actions` | **infra** |

If an issue matches a label rule, assign that component and skip Pass 2 for that issue.

### Pass 2: Title keyword heuristic (case-insensitive)

For remaining issues, match title keywords in this priority order (first match wins):

| Component | Title keywords |
|---|---|
| **core** | core, domain, event, migration, database, sqlite, accounting, account, transaction, transfer, balance, recurring, debt, interest |
| **api** | proto, grpc, rpc, api, daemon, handler, reflection |
| **mcp** | mcp |
| **app** | app, qt, qml, ui, chart, visual, cash flow |
| **infra** | ci, build, install, action, workflow, recipe |

- Issues matching none are **unclassified** (surfaced in hygiene signals).
- This heuristic can be replaced with `component:*` labels if adopted later.

Also extract for each issue:
- **Milestone**: assigned milestone title, or none
- **Days since last update**: `today - updatedAt`
- **Days since creation**: `today - createdAt`
- **Has description**: body is non-empty and at least 20 characters

## Step 3: Present Critical-Path Gate Status

Header: `## Critical-Path Gate`

Finch's dependency graph is: **core → api → mcp / app**

Determine the current gate by checking which layer in dependency order has open issues:

1. If **core** has open issues → "Core has N open issues. Core changes gate API, MCP, and App layers."
2. Else if **api** has open issues → "API has N open issues. API changes gate MCP and App layers. Core is clear."
3. Else → "All upstream layers are clear. MCP and App work is unblocked."

List the open issues in the gating layer: `- #number - title`

## Step 4: Present Active Milestone Progress

Header: `## Active Milestones`

For each open milestone (from Command B), show:

```markdown
**milestone title**: X open / Y closed
```

List the open issues assigned to that milestone (from Command A): `- #number - title`

If no open milestones exist, state: "No active milestones."

## Step 5: Present Component Inventory

Header: `## Component Inventory`

For each component in dependency order (`core`, `api`, `mcp`, `app`, `infra`):

```markdown
### component — N open

- #number - title
```

If a component has zero open issues, show: `### component — clear`

## Step 6: Present Open PRs

Header: `## Open PRs`

For each open PR (from Command C), show:

```markdown
- #number - title (branch: `headRefName`, CI: status)
```

Where status is derived from `statusCheckRollup`: "passing", "failing", "pending", or "no checks".

If no open PRs exist, state: "No open PRs."

## Step 7: Present Hygiene Signals

Header: `## Hygiene Signals`

Run four checks. Show "None" for empty categories (confirms the check ran):

1. **Unlabeled issues** — open issues with no labels at all
2. **Unclassified issues** — open issues that matched no component heuristic (from Step 2)
3. **Unmilestoned backlog (>30 days)** — issues older than 30 days (by `createdAt`) with no milestone. If the count exceeds 10, append: *Consider running /backlog-review for milestone triage.*
4. **Stale issues** — issues with no update in 30+ days (by `updatedAt`)
5. **Missing descriptions** — issues whose body is empty or under 20 characters

Format each as:

```markdown
**Unlabeled issues**: None
**Unclassified issues**: #42, #87
**Unmilestoned backlog (>30 days)**: #15 - title
**Stale issues**: #15 - title (last updated YYYY-MM-DD)
**Missing descriptions**: None
```

## Step 8: Present Recommendations

Header: `## What's Next`

Produce 3-5 ranked recommendations using this priority. Every recommendation must reference specific issue numbers — no meta-process suggestions.

1. **Critical-path gate issues** in the earliest gating layer — "These gate all downstream work"
2. **Active milestone issues** not yet started — "Current planning unit has open work"
3. **Bugs** (`bug` label) not in a milestone — "Bugs accumulate friction"
4. **High-value labels** (`budget-critical`, `app-independence`) not in a milestone — "Labeled priorities without a planning home"
5. **Earliest-component backlog** without milestones — "Candidates for the next planning unit"

Each recommendation: one sentence with specific issue numbers.

## Step 9: Closing Note

End with: "This is a read-only summary. To act on any item, discuss it explicitly."

## Rules

- **Read-only**: Never relabel, close, or modify issues autonomously
- **Three API calls only**: All grouping and analysis happens client-side from Step 1 results
- **Critical-path first**: The most important constraint always surfaces at the top
- **No auto-invocation**: Never invoke other skills (e.g., `/backlog-review`) from within this skill. Suggestions to run other skills are text recommendations for the user to act on
