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
