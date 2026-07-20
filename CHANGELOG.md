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
