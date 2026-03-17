---
name: milestone-lifecycle
description: >-
  Structure the transition between planning units — assess milestone completion,
  evaluate critical-path gates, and propose the next milestone with an initial
  issue set. Most valuable when a milestone is complete or near-complete; for
  mid-milestone orientation, /project-summary is lighter weight.
---

# Milestone Lifecycle

Structured planning transitions for milestone lifecycle management. Assesses whether the active planning unit is complete, checks critical-path gates, and proposes the next planning unit with an initial issue set. All mutating actions require user approval.

| Layer        | Skill                      | Role                        |
| ------------ | -------------------------- | --------------------------- |
| Orientation  | `/project-summary`         | Where are we now?           |
| Analysis     | `/backlog-review`          | What's in the backlog?      |
| **Planning** | **`/milestone-lifecycle`** | **What should we do next?** |

## Step 1: Gather Data

Run these three commands in parallel (no additional API calls after this):

**Command A** — open milestones with descriptions:

```bash
gh api 'repos/{owner}/{repo}/milestones?state=open' --jq '.[] | {number, title, description, open_issues, closed_issues}'
```

**Note**: The URL must be single-quoted to prevent zsh from interpreting `?` as a glob character.

**Command B** — all open issues with metadata:

```bash
gh issue list --state open --limit 200 --json number,title,labels,milestone,body,createdAt,updatedAt
```

**Command C** — recently closed issues (for completion assessment):

```bash
gh issue list --state closed --limit 50 --json number,title,labels,milestone,closedAt,stateReason
```

## Step 2: Classify Issues and Determine Mode

### Classify issues by component

Use the same two-pass heuristic as `/project-summary` and `/backlog-review`:

#### Pass 1: Label match (takes precedence)

| Label | Component |
|---|---|
| `app-independence` | **app** |
| `github_actions` | **infra** |
| `developer-experience` | **dx** |

If an issue matches a label rule, assign that component and skip Pass 2 for that issue.

#### Pass 2: Title keyword heuristic (case-insensitive)

For remaining issues, match title keywords in this priority order (first match wins):

| Component | Title keywords |
|---|---|
| **core** | core, domain, event, migration, database, sqlite, accounting, account, transaction, transfer, balance, recurring, debt, interest |
| **api** | proto, grpc, rpc, api, daemon, handler, reflection |
| **mcp** | mcp |
| **app** | app, qt, qml, ui, chart, visual, cash flow |
| **infra** | ci, build, install, action, workflow, recipe |

Issues matching none are **unclassified**.

### Determine operating mode

Based on Command A results. **Important**: Command A's `open_issues`/`closed_issues` counts can be stale — always use Command B (the issue list) as the authoritative source for actual issue counts in a milestone.

- **No open milestones** → **Cold-start mode**. Skip Steps 3–4, proceed to Step 5 with cold-start framing.
- **One open milestone** → Use it as the active milestone. Proceed to Step 3.
- **Multiple open milestones** → Present a numbered list with progress counts and ask the user which milestone to evaluate. Proceed to Step 3 with the chosen milestone.

Present the mode determination:

```markdown
## Operating Mode

**Active milestone**: "Core — Foundation" (3 open / 7 closed)
**Mode**: Completion assessment
```

Or for cold start:

```markdown
## Operating Mode

**No active milestones found.**
**Mode**: Cold start — proposing first planning unit
```

## Step 3: Completion Assessment

Header: `## Completion Assessment`

Skip if cold-start mode.

Assess the active milestone's state by categorizing it:

- **Complete**: Zero open issues in the milestone. All work is done.
- **Near-complete**: 1–2 open issues remain, none in the current gating component (see Step 4). These may be non-blocking follow-ups or minor items that can carry forward.
- **In-progress**: 3+ open issues remain, or any issues in the gating component remain open.

Present as:

```markdown
**"Core — Foundation"**: [Complete / Near-complete / In-progress]

- N issues closed, M remaining
- [If near-complete] Remaining: #42 - Title (non-blocking), #87 - Title (non-blocking)
- [If in-progress] Gating component issues: #132 - Title
```

For **Complete** or **Near-complete**: proceed to Step 4 (gate assessment) and Step 5 (transition proposal).

For **In-progress**: present the remaining issues grouped by component, and end with:

```markdown
The active planning unit still has open work. To proceed with transition planning anyway, say "continue." Otherwise, focus on the remaining issues.
```

Wait for user confirmation before proceeding past an in-progress milestone.

## Step 4: Gate Assessment

Header: `## Critical-Path Gate`

Finch's dependency graph: **core → api → mcp / app**

Non-gated components (run in parallel, not blocked by the dependency chain): **dx**

From Command B, find all open issues grouped by component. Determine the current gate:

- Find the earliest component (in dependency order: core → api → mcp/app) that has open issues. That is the "current gate."
- If no issues remain in core, api, mcp, or app, state "All gates cleared."

Present:

```markdown
**Current gate**: core
**Open issues in gating component**: N

- #132 - Title
- #178 - Title

**Downstream components blocked**: api, mcp, app
**Non-gated components (unaffected)**: dx
```

Or if a gate has cleared:

```markdown
**Gate cleared**: core has zero open issues.
**Newly unblocked**: api (next in dependency order)

This is a significant planning signal — the next milestone could shift focus downstream.
```

If the gate assessment reveals a newly unblocked downstream component, recommend running `/backlog-review` for a full readiness assessment if one hasn't been run recently.

## Step 5: Transition Proposal

Header: `## Transition Proposal`

This step synthesizes a proposal for the next planning unit.

### 5a. Milestone Recap

State the completed/completing milestone's purpose (from its description field in Command A). One sentence acknowledging what was accomplished.

For cold-start mode: skip this subsection.

### 5b. Component Focus

Based on gate assessment: which component should the next planning unit focus on?

- If the current component's gate is **not yet cleared**, the next planning unit likely stays in the same component.
- If the gate **cleared**, the next planning unit could shift to the newly unblocked downstream component, or could continue deepening the current component.

State the recommendation with rationale.

### 5c. Backlog Scan for Candidates

From Command B, filter open issues that are:

- Not assigned to any milestone

Group by relevance to the proposed component:

**Direct continuation** — issues in the target component that advance its goals.

**Newly unblocked** — issues in a downstream component that was just unblocked by a gate clearing. Only present this group if a gate cleared in Step 4.

**Cross-component candidates** — well-defined issues in other components that could be pulled in if the planning unit has capacity.

## Step 6: Next Planning Unit Proposal

Header: `## Proposed: Next Planning Unit`

Synthesize the transition proposal into a concrete milestone.

**Title**: Follow the naming convention `"{Component} — {Unit Name}"` with an em-dash (—). Use the component name mapping from the Rules section.

**Purpose statement**: Draft a 1–2 sentence description for the milestone's description field. This should state the planning unit's purpose and what success looks like.

**Initial issue set**: List the proposed issues with rationale for each inclusion.

**Scope check**: Note the issue count. If more than 8–10 issues, flag as potentially too large for a single planning unit and suggest splitting.

Present:

```markdown
### "Core — Foundation"

**Purpose**: [Draft description — e.g., "Establish typed domain errors and complete the core domain surface area needed for reliable API exposure."]

**Proposed issues** (N total):

- #47 - Title — [rationale]
- #40 - Title — [rationale]

**Scope assessment**: N issues. [Within range / Consider splitting — more than 10 issues suggests this could be two planning units.]
```

For **cold-start mode**: frame the proposal as "first" rather than "next." Use the gate assessment to identify which component should get the first planning unit. Draw from the full open backlog for candidates.

## Step 7: Execution Commands

Header: `## Proposed Commands`

Generate the `gh` CLI commands needed to execute the transition. Present them in execution order, each with an explicit approval gate. **Never auto-execute — wait for user confirmation at each phase.**

### Phase 1: Close completed milestone

Skip if cold-start mode or milestone is in-progress.

```markdown
### Phase 1: Close completed milestone

\`\`\`bash
gh api repos/{owner}/{repo}/milestones/{number} -X PATCH -f state=closed
\`\`\`

**Approve Phase 1?** (yes / no / skip)
```

### Phase 2: Handle near-complete stragglers

Only present if the milestone was near-complete with remaining issues. Surface each remaining issue and ask the user to decide: carry forward to new milestone or close.

```markdown
### Phase 2: Carry forward remaining issues

These issues were open in the completed milestone. How should each be handled?

- #42 - Title — carry forward / close?
- #87 - Title — carry forward / close?
```

### Phase 3: Create new milestone

```markdown
### Phase 3: Create new milestone

\`\`\`bash
gh api repos/{owner}/{repo}/milestones -X POST -f title="Core — Foundation" -f description="[purpose statement]"
\`\`\`

**Approve Phase 3?** (yes / no / edit title / edit description)
```

### Phase 4: Assign issues to new milestone

```markdown
### Phase 4: Assign issues

\`\`\`bash
gh issue edit 47 --milestone "Core — Foundation"
gh issue edit 40 --milestone "Core — Foundation"
\`\`\`

**Approve Phase 4?** (yes / no / modify list)
```

## Step 8: Closing

```markdown
---

All proposed commands require your approval. This skill does not modify any GitHub state autonomously.

**Related skills:**

- Run `/backlog-review` for deeper backlog health analysis before planning transitions
- Run `/project-summary` for a quick orientation after the transition is complete
```

## Rules

- **User approval required**: Every `gh` command that modifies GitHub state is proposed, never auto-executed. Wait for explicit user confirmation at each phase gate.
- **Three API calls**: All analysis is derived from Step 1 data. No additional API calls during analysis steps.
- **Read-only analysis, proposed-only execution**: The skill's analysis steps (2–6) never modify state. Execution commands in Step 7 are proposals presented for approval.
- **Component name mapping for milestone titles**: core → "Core", api → "API", mcp → "MCP", app → "App", infra → "Infrastructure", dx → "DX". Always use the format `"{Component} — {Unit Name}"` with an em-dash (—).
- **Complements, does not replace**: This skill structures the planning transition decision. `/project-summary` provides orientation, `/backlog-review` provides analysis depth. Each skill has a distinct role.
