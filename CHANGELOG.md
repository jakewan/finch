# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Qt app: create accounts through a UI form (previously only via MCP tools).
- Qt app: view an account's individual transactions on the Transactions screen — pick an account, browse its transactions in a list, and select one to see its full details (previously only the aggregated balance chart was visible).
- Qt app: record a manual transaction against an account from the Transactions screen — enter a name, amount, and Expense/Income, with a live signed-amount preview and the date defaulting to today (previously only via MCP tools).
- Qt app: view and create an account's recurring rules on the Recurring Rules screen — pick an account, browse its rules, and add a new rule at any frequency (weekly, biweekly, semi-monthly, monthly, or yearly) with an Expense/Income amount and an optional end date (previously only via MCP tools).

### Changed

- Qt app: a left navigation rail now switches between primary screens (Overview, Accounts, and placeholders for Transactions and Recurring Rules). Account creation moved off its modal dialog onto the Accounts screen — a non-modal form that keeps in-progress input when you navigate away and back.

### Fixed

- Qt app now restores window size and position across restarts.
- Daemon returns `InvalidArgument` (not `Internal`) when a recurring-rule create has an out-of-range day-of-month or malformed semi-monthly days, and `NotFound` (not `Internal`) when updating the amount of a rule that does not exist — so callers can tell a bad request from a daemon failure.
- Recurring transfer rules now credit the destination account. Previously a recurring transfer was projected as an ordinary expense on its source account and the destination was never credited, so projected balances and the balance time series both understated the destination.
- Monthly cash flow no longer counts a transfer as both income and an expense. Moving money between your own accounts is neither, so transfers are reported in a new `transfers` total instead of inflating income and expenses. Income plus expenses plus transfers equals the period's balance change, so a cash-flow report can still be reconciled against the balance chart — for a single-account view this means a transfer no longer changes that account's net cash flow, but the movement is still visible.
- Projected transactions and recorded transactions now carry an `is_transfer` flag over gRPC and MCP, so a client can tell why a transfer appears in projected balances but not in cash-flow income.
- Recurring transfer rules are validated on create and on amount changes: the destination must be an existing account other than the source, and the amount must be positive (direction comes from the source and destination pair). Invalid input returns `InvalidArgument` rather than `Internal`.
