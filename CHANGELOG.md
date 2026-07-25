# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Qt app: create accounts through a UI form (previously only via MCP tools).
- Qt app: view an account's individual transactions on the Transactions screen — pick an account, browse its transactions in a list, and select one to see its full details (previously only the aggregated balance chart was visible).
- Qt app: record a manual transaction against an account from the Transactions screen — enter a name, amount, and Expense/Income, with a live signed-amount preview and the date defaulting to today (previously only via MCP tools).
- Qt app: view and create an account's recurring rules on the Recurring Rules screen — pick an account, browse its rules, and add a new rule at any frequency (weekly, biweekly, semi-monthly, monthly, or yearly) with an optional end date (previously only via MCP tools). A rule is either an Expense/Income amount on the selected account or a recurring transfer to another account, picked from a "Transfer to" list. Transfer rules show as a debit on the account they leave, naming the destination.

### Changed

- Qt app: a left navigation rail now switches between primary screens (Overview, Accounts, and placeholders for Transactions and Recurring Rules). Account creation moved off its modal dialog onto the Accounts screen — a non-modal form that keeps in-progress input when you navigate away and back.

### Fixed

- Qt app now restores window size and position across restarts.
- Daemon returns `InvalidArgument` (not `Internal`) when a recurring-rule create has an out-of-range day-of-month or malformed semi-monthly days, and `NotFound` (not `Internal`) when updating the amount of a rule that does not exist — so callers can tell a bad request from a daemon failure.
- Recurring transfer rules now credit the destination account. Previously a recurring transfer was projected as an ordinary expense on its source account and the destination was never credited, so projected balances and the balance time series both understated the destination.
- Monthly cash flow no longer counts a transfer as both income and an expense. Moving money between your own accounts is neither, so transfers are reported in a new `transfers` total instead of inflating income and expenses. Income plus expenses plus transfers equals the period's balance change, so a cash-flow report can still be reconciled against the balance chart — for a single-account view this means a transfer no longer changes that account's net cash flow, but the movement is still visible.
- Projected transactions and recorded transactions now carry an `is_transfer` flag over gRPC and MCP, so a client can tell why a transfer appears in projected balances but not in cash-flow income.
- Recurring transfer rules are validated on create and on amount changes: the destination must be an existing account other than the source, and the amount must be positive (direction comes from the source and destination pair). Invalid input returns `InvalidArgument` rather than `Internal`.
- The Qt app now shows a recurring transfer rule as a debit on the account it leaves, naming the destination. Previously it displayed the rule's stored magnitude as a credit on the account being debited, with no indication of where the money went — affecting any transfer rule created through the MCP tools.
- The Qt app's amount field on the Transactions and Recurring Rules screens now caps an amount at 12 integer digits, just under $1 trillion. Previously the integer part was unbounded, and an amount around $90 trillion or more could be recorded imprecisely and without warning — that is where the app stops representing that many cents exactly. The field also now refuses amounts with more than two decimal places, in exponent notation, or carrying trailing text; those were already refused as you typed, and could reach the field only from code, so no recorded amount was affected by them.

### Security

- Updated Go dependencies to close four advisories that finch's own code reached: an authorization bypass in gRPC affecting the daemon's request routing (GO-2026-4762), two advisories in the MCP SDK (GO-2026-4773, GO-2026-4770), and an HTTP/2 infinite loop in `golang.org/x/net` (GO-2026-4918). The Go toolchain also moved to 1.26.5, closing eleven standard-library advisories in the daemon and MCP server.
- Known vulnerabilities in Go dependencies are now detected rather than discovered: `govulncheck` runs on every pull request touching Go code and blocks the merge on an advisory reachable from finch's code, a weekly scheduled scan covers advisories published between changes, and `just vuln` runs the same scan locally. This covers the Go daemon and MCP server; the Qt app's C++ dependencies are not yet in scope.
- Updated `google.golang.org/grpc` to 1.82.1 and `golang.org/x/net` to 0.57.0 in the daemon and MCP server, which also carries `golang.org/x/text` to 0.40.0 — closing nine advisories. None of the nine was reachable from finch's own code, so this is proactive rather than a repair for exposed behavior: the seven `golang.org/x/net` advisories need HTML, IDNA, or DNS-message parsing that finch does not do (GO-2026-5025 through GO-2026-5030, and GO-2026-5942), and the `golang.org/x/text` one needs a text-processing path finch does not reach (GO-2026-5970). The gRPC advisory has no Go vulnerability database entry, so it is named by its GitHub identifier instead (GHSA-hrxh-6v49-42gf); two of its three impacts are confined to the xDS RBAC authorization engine, which finch does not use, leaving a denial-of-service via an HTTP/2 rapid-reset mitigation bypass in the transport — and the daemon accepts connections only on a Unix socket reachable by the local user.
