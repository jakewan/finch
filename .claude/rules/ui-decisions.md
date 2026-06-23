---
paths: "app/**/*"
---

# UI Decision Rubric

*Supplies the `design-fork-adjudication` extension point, scoped to UI forks. The slot marker is metadata only; the lenses below read as standalone project guidance without it.*

When a user-facing design fork has no obvious answer, walk these lenses and record which one settled it. They are listed in rough priority — an earlier lens tends to dominate a later one — but this is a heuristic, not a mechanical tie-break; a genuine lens-vs-lens conflict is a decision to make in the open, not to resolve by list position alone.

1. **Locality of action** — Put a write action where the user's attention and data context already are, not on a separate screen they must navigate to.
2. **Pattern consistency** — Echo an established in-app pattern unless an earlier lens says otherwise (e.g. "a screen hosts its create-form inline above the list").
3. **Input economy** — Fewest inputs, fewest controls. Two controls carrying the same meaning on one screen (e.g. two account pickers) are a defect, not a convenience.
4. **Forgiving input** — Make the easy path the correct path: a control that can't express the wrong value beats one that can (an Expense/Income toggle over a signed number the user can mis-sign).
5. **Discoverability** — A capability the user can't find may as well not exist; favor placements reached in the natural flow.

**Engineering constraints** (testability, component independence, complexity) are a separate axis. **UX leads:** walk the UX lenses first to find the desired answer; when a constraint pulls against it, surface the trade explicitly and decide in the open. Engineering never silently wins, and the best UX is never amputated before it's weighed. Often the constraint can be satisfied without giving up the UX.

**Illustrative fork (forward-looking — describes unbuilt work).** This rubric was seeded by a decision not yet implemented: where a form for recording a transaction should live. Walking the lenses: a separate screen loses to placing the form on the transactions screen, where the user already has an account in context (locality) and the screen already carries one account selector — a second picker would be the defect lens 3 names (input economy). That screen is a master-detail view, so the form sits above that content, echoing how the accounts screen seats its create-form above its list (pattern consistency). Testability — a *separate axis*, not a UX lens — pulled toward an own-picker or separate-screen form; it is satisfied without bending the UX by injecting the target account into a standalone, independently-testable component. That injection is already this app's standing test pattern (see the Qt app rule's mock-injection convention), not a result the rubric invented — the rubric's contribution here is the *placement*. Recording needs a selected account, which the screen's selector already establishes.

Because this example describes unbuilt work, treat it as illustrative; replace it with the shipped decision once that form exists.

The five lenses are a living seed grown from real decisions; add lenses (accessibility, visual hierarchy, …) when a fork actually exercises them, rather than inventing them up front.
