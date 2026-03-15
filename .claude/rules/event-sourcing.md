# Event Sourcing and Domain Modeling

## Events as Source of Truth

Every domain state change MUST produce at least one event. Events are the source of truth; read model tables (`accounts`, `transactions`, `recurring_rules`) are derived views maintained for query efficiency.

## Aggregate Pattern

Each aggregate type (`account`, `transaction`, `recurring_rule`) has its own event stream with monotonically increasing sequence numbers. Event types are string constants defined per aggregate (e.g., `EventAccountCreated`, `EventRecurringRulePaused`). Each event type has a dedicated payload struct with JSON tags.

## Transaction Boundaries

Domain operations follow this sequence within a single database transaction:

1. Validate input
2. Append event(s) via `appendEvents(ctx, tx, ...)`
3. Update the read model table
4. Commit (or rollback on any error)

NEVER append events outside a transaction. NEVER update a read model without a corresponding event.

## Event Replay

Read models MUST be rebuildable from the event log. Provide replay functions that reconstruct aggregate state from events. This is the recovery path if a read model diverges from the event log due to a migration bug or schema change.

## Domain Error Types

Define typed errors in `core` for common failure modes: not found, invalid input, conflict/duplicate. Use `errors.Is`/`errors.As` for programmatic error handling. NEVER rely on string matching to distinguish error conditions.

Typed errors enable proper gRPC status code mapping in the daemon layer (see `api-design.md`).
