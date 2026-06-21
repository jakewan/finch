# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Qt app: create accounts through a UI form (previously only via MCP tools).

### Changed

- Qt app: a left navigation rail now switches between primary screens (Overview, Accounts, and placeholders for Transactions and Recurring Rules). Account creation moved off its modal dialog onto the Accounts screen — a non-modal form that keeps in-progress input when you navigate away and back.

### Fixed

- Qt app now restores window size and position across restarts.
