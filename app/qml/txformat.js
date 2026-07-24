// Shared transaction-formatting helpers, lifted verbatim from TransactionsPanel so the
// record form's live amount preview and the panel's master/detail both read from one source.
// Stateless pure functions, so .pragma library (one shared instance, no per-import context).
.pragma library

// TransactionStatus enum (proto int 0-3) -> human label. No app-wide status map exists yet;
// the proto enum values are mirrored here (Projected=1, Scheduled=2, Reconciled=3).
function statusLabel(status) {
    switch (status) {
    case 1: return "Projected"
    case 2: return "Scheduled"
    case 3: return "Reconciled"
    default: return "Unspecified"
    }
}

// Signed int64 cents -> sign-correct currency. Math.abs keeps the "-" ahead of the "$"
// ("-$12.34", not "$-12.34").
function formatAmount(cents) {
    return (cents < 0 ? "-$" : "$") + Math.abs(cents / 100).toFixed(2)
}

// Whether a rule actually moves money between two accounts — flagged as a transfer AND carrying
// a destination. Both halves are required, matching the condition core's effectOn applies before
// deriving direction. Single definition because every consumer must agree: a renderer that
// treated a target-less transfer as directed would announce a destination it cannot name.
function isDirectedTransfer(rule) {
    return !!(rule && rule.isTransfer && rule.transferTargetAccountId)
}

// A recurring rule's effect on one account, in signed cents. Mirrors core's effectOn in
// core/projection.go, branch for branch, so the app and the projection engine cannot disagree
// about which way a rule moves money.
//
// A transfer rule stores a POSITIVE magnitude and belongs to its source account; direction comes
// from which side of the transfer the viewing account sits on. A non-transfer rule applies its
// stored signed amount as-is — and so does a rule flagged as a transfer with no target, which is
// the third branch. That state is not reachable through the API (the daemon and core both reject
// it), but the branch is reproduced anyway: a partial copy of authority logic is the thing that
// drifts from its original.
//
// Takes the viewing account rather than assuming the source, because rules are currently fetched
// scoped to their owner and that will stop being true once inbound transfers are listable.
function ruleAmount(rule, accountId) {
    if (isDirectedTransfer(rule)) {
        if (accountId === rule.accountId)
            return -rule.amount
        if (accountId === rule.transferTargetAccountId)
            return rule.amount
    }
    return rule.amount
}

// Frequency enum (proto int 1-5) -> human label. Mirrors the proto values (Weekly=1 …
// Yearly=5); 0 is the unspecified sentinel, which a stored rule never carries.
function frequencyLabel(frequency) {
    switch (frequency) {
    case 1: return "Weekly"
    case 2: return "Biweekly"
    case 3: return "Semi-monthly"
    case 4: return "Monthly"
    case 5: return "Yearly"
    default: return "Unspecified"
    }
}
