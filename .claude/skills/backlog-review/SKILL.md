---
name: backlog-review
description: >-
  Deep backlog health analysis — issue quality, staleness, component balance,
  overlap detection, and milestone candidates. Use for dedicated grooming
  sessions, not quick orientation (use /project-summary for that).
---

# Backlog Review

Comprehensive backlog health analysis for dedicated grooming sessions. Produces a structured report for discussion, then transitions to collaborative decision-making.

## Step 1: Gather Data

Run these four commands in parallel:

**Command A** — all open issues with full metadata:

```bash
gh issue list --state open --limit 200 --json number,title,labels,milestone,updatedAt,createdAt,body,comments
```

**Command B** — recently closed issues (for overlap detection and trajectory):

```bash
gh issue list --state closed --limit 50 --json number,title,labels,closedAt,stateReason
```

**Command C** — active milestone progress:

```bash
gh api repos/{owner}/{repo}/milestones --jq '.[] | select(.state == "open") | {title, open: .open_issues, closed: .closed_issues, number: .number, description}'
```

**Command D** — label inventory:

```bash
gh label list --limit 50 --json name,description
```

## Step 2: Classify Each Issue by Component

Use the same two-pass heuristic as `/project-summary`:

### Pass 1: Label match (takes precedence)

| Label | Component |
|---|---|
| `app-independence` | **app** |
| `github_actions` | **infra** |
| `developer-experience` | **dx** |

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

Issues matching none are **unclassified**.

Also extract for each issue:
- **Last substantive activity**: the most recent of (a) last comment `createdAt`, or (b) issue `createdAt` if no comments. This is the primary staleness signal — `updatedAt` is unreliable because triage operations (labeling, milestone assignment) bump it without meaningful engagement.
- **Body quality**: body length, presence of checklists, acceptance criteria language

## Step 3: Present Issue Quality Audit

Header: `## Issue Quality Audit`

Evaluate all open issues against quality heuristics. Show "None" for empty categories.

**Needs decomposition** — issues with 3+ checklist items (`- [ ]` or `- [x]`) or 4+ numbered list items with substantive content. These are candidates for breaking into sub-issues. Exclude checklists that appear under an "Acceptance criteria" heading — those are verification steps, not work items.

**Missing description** — body is null, empty, or under 30 characters. An issue without a description lacks clear purpose.

**Missing acceptance criteria** — non-milestone issues labeled `enhancement` whose body doesn't contain structured requirements (checklist markers, `## ` section headers, or acceptance language like "accept", "criteria", "done when", "verify", "should").

**Unclassified issues** — open issues that matched no component heuristic (from Step 2).

**Unlabeled issues** — open issues with no labels at all.

Format each group as a list of `#number - title` with a brief note. If a group is empty, show "None."

## Step 4: Present Staleness Analysis

Header: `## Staleness Analysis`

Use **last substantive activity** (last comment date, or issue creation date if no comments) as the primary staleness signal — NOT `updatedAt`.

```markdown
**Stale (60+ days since last substantive activity, no milestone)**

- #23 - Title — last comment YYYY-MM-DD (N days), last updated YYYY-MM-DD

**Stale (60+ days since last substantive activity, has milestone)**

- #45 - Title — in "milestone name", last comment YYYY-MM-DD (noted but lower concern)

**Aging (30-60 days since last substantive activity, no milestone)**

- #67 - Title — created YYYY-MM-DD, no comments
```

If no issues match any category: "Backlog is fresh — no staleness concerns."

## Step 5: Present Component Balance

Header: `## Component Balance`

Build a summary table of issue distribution across components, using dependency order:

```markdown
| Component    | Active | % of Backlog | Signal      |
| ------------ | ------ | ------------ | ----------- |
| core         | 5      | 33%          |             |
| api          | 1      | 7%           |             |
| mcp          | 0      | 0%           | starved     |
| app          | 5      | 33%          |             |
| infra        | 1      | 7%           |             |
| dx           | 2      | 13%          |             |
| (unclassified) | 1   | 7%           | needs label |
```

- **% of Backlog** = active count / total active count across all components
- **Overloaded**: Flag non-gating components that exceed 2x the average of other non-gating components
- **Starved**: Flag components with zero issues
- **Needs label**: Flag the unclassified bucket if non-empty

Follow the table with 2-3 observation sentences about the distribution.

## Step 6: Present Overlap Detection

Header: `## Possible Overlaps`

Apply conservative heuristics — present candidates for human judgment, never declare duplicates unilaterally.

**Title similarity**: Flag pairs of open issues whose titles share 3+ significant words (excluding common words: "add", "fix", "update", "support", "implement", "the", "for", "and", "in", "to", "a", "an", "no", "way").

**Cross-reference clustering**: Issues whose bodies contain `#N` references to other _open_ issues. Group into clusters only when 3+ open issues form a mutual reference web. Briefly assess whether each cluster represents coordinated work (good — note it and move on) or duplicated scope (needs attention — explain why).

**Open-vs-closed overlap**: Open issues whose titles closely match recently closed issues from Command B — possible "was this already resolved?" situations.

Format:

```markdown
**May address similar concerns:**

- #42 "Title A" ↔ #87 "Title B" — both touch [shared concern]; verify scope boundaries

**Cross-referenced clusters:**

- Cluster: #23, #45, #67 — coordinated work around [topic]; no action needed
- Cluster: #88, #90 — possible duplicated scope: both describe [overlapping concern]

**Possibly resolved:**

- #89 "Title" — may overlap with closed #124 (closed YYYY-MM-DD, reason: completed)
```

If no overlaps are detected: "No obvious overlaps detected — backlog has good clarity."

## Step 7: Present Milestone Candidates

Header: `## Milestone Candidates`

Filter to issues that are: open and not assigned to any milestone. Rank by readiness.

**If an active milestone exists**: State the milestone's theme (from its description in Command C) before ranking. Candidates that advance the milestone's purpose rank higher.

- **High confidence** — issues that directly advance the active milestone's theme, OR bugs in the gating component, OR issues referenced by current milestone issues (dependencies of active work)
- **Medium confidence** — well-defined issues in the gating component that don't directly advance the milestone theme but are good next-milestone candidates
- **Good work, different planning unit** — well-defined issues outside the gating component; note which component and future planning unit they'd belong to
- **Needs refinement first** — issues with quality problems identified in Step 3; note: "Fix quality first, then consider for milestone"

**If no active milestones exist (cold start)**: Rank purely by component dependency order (core first) and issue quality. Note that `/milestone-lifecycle` can help propose the first milestone.

Format: `- #number - title — component, [readiness rationale]`

## Step 8: Present Backlog Trajectory

Header: `## Backlog Trajectory`

Using Command A (open issues) and Command B (recently closed issues), assess net change over 30 days:

- Count issues **created in the last 30 days** (from Command A's `createdAt`)
- Count issues **closed in the last 30 days** (from Command B's `closedAt`)
- Calculate net change: `created - closed`

Present as:

```markdown
**Last 30 days**: N created, M closed (net +/- K)
```

Add a one-sentence interpretation:

- Net positive: "Backlog is growing — new work is being identified faster than it's being resolved."
- Net zero: "Backlog is stable — roughly matching creation and completion rates."
- Net negative: "Backlog is shrinking — closing faster than creating. Good momentum."

## Step 9: Present Key Findings & Discussion

Header: `## Key Findings`

Synthesize the most important findings from all previous sections into 3-5 bullet points, each with a concrete recommended action. These should be the items that actually warrant grooming attention — not a summary of every section.

Prioritize findings by strategic significance:

1. Gate status changes (a component layer cleared, unblocking downstream)
2. Stale issues with no engagement (potential backlog rot)
3. Component imbalances that suggest misallocated effort
4. Milestone candidates ready to pull in
5. Overlap clusters that need scope clarification

After the key findings, transition to discussion:

```markdown
---

This is a read-only analysis — no issues were modified. To act on findings:

- **Decompose an oversized issue**: discuss breakdown strategy
- **Add to milestone**: discuss which candidates to pull in
- **Close stale issues**: discuss which have lost relevance
- **Resolve overlaps**: discuss whether to merge, clarify scope, or close duplicates

Which area would you like to discuss first?
```

## Rules

- **Read-only**: Never relabel, close, or modify issues autonomously
- **Four API calls**: All grouping and analysis happens client-side from Step 1 results
- **Conservative overlap detection**: Present possible overlaps for human judgment; never declare duplicates unilaterally
- **Complements /project-summary**: This skill goes deeper on backlog health; `/project-summary` is for quick session orientation
- **Substantive activity over metadata**: Always prefer comment timestamps and creation dates over `updatedAt` for staleness analysis. Triage touches are not engagement.
