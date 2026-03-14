package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Event represents a persisted domain event.
type Event struct {
	ID            int64
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       json.RawMessage
	RecordedAt    time.Time
	Sequence      int64
}

// NewEvent holds the data needed to append a new event (before persistence assigns ID/sequence).
type NewEvent struct {
	EventType string
	Payload   json.RawMessage
}

// appendEvents inserts events for a single aggregate within an existing transaction.
// It assigns monotonically increasing sequence numbers starting after the current max.
func appendEvents(ctx context.Context, tx *sql.Tx, aggregateType, aggregateID string, events []NewEvent) ([]Event, error) {
	if len(events) == 0 {
		return nil, nil
	}

	var maxSeq int64
	row := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(sequence), 0) FROM events WHERE aggregate_type = ? AND aggregate_id = ?",
		aggregateType, aggregateID,
	)
	if err := row.Scan(&maxSeq); err != nil {
		return nil, fmt.Errorf("read max sequence: %w", err)
	}

	now := time.Now().UTC()
	recorded := make([]Event, len(events))

	for i, ne := range events {
		seq := maxSeq + int64(i) + 1
		result, err := tx.ExecContext(ctx,
			`INSERT INTO events (aggregate_type, aggregate_id, event_type, payload, recorded_at, sequence)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			aggregateType, aggregateID, ne.EventType, string(ne.Payload), now.Unix(), seq,
		)
		if err != nil {
			return nil, fmt.Errorf("insert event %s seq %d: %w", ne.EventType, seq, err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("last insert id: %w", err)
		}
		recorded[i] = Event{
			ID:            id,
			AggregateType: aggregateType,
			AggregateID:   aggregateID,
			EventType:     ne.EventType,
			Payload:       ne.Payload,
			RecordedAt:    now,
			Sequence:      seq,
		}
	}
	return recorded, nil
}

// AppendEvents persists events for a single aggregate in its own transaction.
func (db *DB) AppendEvents(ctx context.Context, aggregateType, aggregateID string, events []NewEvent) ([]Event, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	recorded, err := appendEvents(ctx, tx, aggregateType, aggregateID, events)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return recorded, nil
}

// LoadEvents returns all events for a specific aggregate, ordered by sequence.
func (db *DB) LoadEvents(ctx context.Context, aggregateType, aggregateID string) ([]Event, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, aggregate_type, aggregate_id, event_type, payload, recorded_at, sequence
		 FROM events
		 WHERE aggregate_type = ? AND aggregate_id = ?
		 ORDER BY sequence`,
		aggregateType, aggregateID,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanEvents(rows)
}

// LoadEventsSince returns events for an aggregate after a given sequence number.
func (db *DB) LoadEventsSince(ctx context.Context, aggregateType, aggregateID string, afterSequence int64) ([]Event, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, aggregate_type, aggregate_id, event_type, payload, recorded_at, sequence
		 FROM events
		 WHERE aggregate_type = ? AND aggregate_id = ? AND sequence > ?
		 ORDER BY sequence`,
		aggregateType, aggregateID, afterSequence,
	)
	if err != nil {
		return nil, fmt.Errorf("query events since %d: %w", afterSequence, err)
	}
	defer func() { _ = rows.Close() }()
	return scanEvents(rows)
}

// LoadAllEvents returns all events of a given aggregate type, ordered by aggregate_id then sequence.
func (db *DB) LoadAllEvents(ctx context.Context, aggregateType string) ([]Event, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, aggregate_type, aggregate_id, event_type, payload, recorded_at, sequence
		 FROM events
		 WHERE aggregate_type = ?
		 ORDER BY aggregate_id, sequence`,
		aggregateType,
	)
	if err != nil {
		return nil, fmt.Errorf("query all events for %s: %w", aggregateType, err)
	}
	defer func() { _ = rows.Close() }()
	return scanEvents(rows)
}

func scanEvents(rows *sql.Rows) ([]Event, error) {
	var events []Event
	for rows.Next() {
		var e Event
		var recordedUnix int64
		var payload string
		if err := rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &payload, &recordedUnix, &e.Sequence); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.Payload = json.RawMessage(payload)
		e.RecordedAt = time.Unix(recordedUnix, 0).UTC()
		events = append(events, e)
	}
	return events, rows.Err()
}
