---
paths: "app/**/*"
---

# UI Decision Rubric

When a UI design fork has no obvious answer, walk these lenses and record which one decided your lean. They are listed in rough priority — an earlier lens tends to dominate a later one — but this is a heuristic, not a mechanical tie-break; a genuine lens-vs-lens conflict is surfaced as a choice with your lean, not resolved by list position alone.

1. **Locality of action** — Put a write action where the user's attention and data context already are, not on a separate screen they must navigate to.
2. **Pattern consistency** — Echo an established in-app pattern unless an earlier lens says otherwise (e.g. "a screen hosts its create-form inline above the list").
3. **Input economy** — Fewest inputs, fewest controls. Two controls carrying the same meaning on one screen (e.g. two account pickers) are a defect, not a convenience.
4. **Forgiving input** — Make the easy path the correct path: a control that can't express the wrong value beats one that can (an Expense/Income toggle over a signed number the user can mis-sign).
5. **Discoverability** — A capability the user can't find may as well not exist; favor placements reached in the natural flow.

**Engineering constraints** (testability, component independence, complexity) are a separate axis. **UX leads:** walk the UX lenses first to find the desired answer; when a constraint pulls against it, surface the trade explicitly with your lean rather than deciding it yourself. Engineering never silently wins, and the best UX is never amputated before it's weighed. Often the constraint can be satisfied without giving up the UX.

**Worked fork: where the record-transaction form lives.** This rubric was seeded by that decision, shipped as `app/qml/RecordTransactionForm.qml` hosted in `app/qml/TransactionsPanel.qml`. Walking the lenses: a separate screen lost to placing the form on the transactions screen, where the user already has an account in context (locality) and the screen already carries one account selector — a second picker would be the defect lens 3 names (input economy). That screen is a master-detail view, so the form sits above that content, echoing how `AccountsPanel.qml` seats its create-form above its list (pattern consistency). Testability — a *separate axis*, not a UX lens — pulled toward an own-picker or separate-screen form; it was satisfied without bending the UX by injecting the selected account into a standalone, independently-testable component (the form's `accountId`, fed from the panel's selector). That injection is already this app's standing test pattern (the mock-injection convention in `.claude/rules/qt-app.md` § Build and Test), not a result the rubric invented — the rubric's contribution here is the *placement*. The form stays disabled until the selector establishes an account.

The five lenses are a living seed grown from real decisions; add lenses (accessibility, visual hierarchy, …) when a fork actually exercises them, rather than inventing them up front.
