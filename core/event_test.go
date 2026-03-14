package core_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jakewan/finch/core"
)

func TestAppendAndLoadEvents(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	events, err := db.AppendEvents(ctx, "test_aggregate", "agg-1", []core.NewEvent{
		{EventType: "Created", Payload: json.RawMessage(`{"name":"first"}`)},
		{EventType: "Updated", Payload: json.RawMessage(`{"name":"second"}`)},
	})
	if err != nil {
		t.Fatalf("AppendEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Sequence != 1 {
		t.Fatalf("expected sequence 1, got %d", events[0].Sequence)
	}
	if events[1].Sequence != 2 {
		t.Fatalf("expected sequence 2, got %d", events[1].Sequence)
	}

	loaded, err := db.LoadEvents(ctx, "test_aggregate", "agg-1")
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded events, got %d", len(loaded))
	}
	if loaded[0].EventType != "Created" {
		t.Fatalf("expected Created, got %s", loaded[0].EventType)
	}
	if loaded[1].EventType != "Updated" {
		t.Fatalf("expected Updated, got %s", loaded[1].EventType)
	}
}

func TestAppendEventsSequenceContinues(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_, err := db.AppendEvents(ctx, "test", "agg-1", []core.NewEvent{
		{EventType: "First", Payload: json.RawMessage(`{}`)},
	})
	if err != nil {
		t.Fatalf("first append: %v", err)
	}

	events, err := db.AppendEvents(ctx, "test", "agg-1", []core.NewEvent{
		{EventType: "Second", Payload: json.RawMessage(`{}`)},
	})
	if err != nil {
		t.Fatalf("second append: %v", err)
	}
	if events[0].Sequence != 2 {
		t.Fatalf("expected sequence 2 for second append, got %d", events[0].Sequence)
	}
}

func TestLoadEventsSince(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_, err := db.AppendEvents(ctx, "test", "agg-1", []core.NewEvent{
		{EventType: "A", Payload: json.RawMessage(`{}`)},
		{EventType: "B", Payload: json.RawMessage(`{}`)},
		{EventType: "C", Payload: json.RawMessage(`{}`)},
	})
	if err != nil {
		t.Fatalf("AppendEvents: %v", err)
	}

	events, err := db.LoadEventsSince(ctx, "test", "agg-1", 1)
	if err != nil {
		t.Fatalf("LoadEventsSince: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events after sequence 1, got %d", len(events))
	}
	if events[0].EventType != "B" {
		t.Fatalf("expected B, got %s", events[0].EventType)
	}
}

func TestLoadAllEvents(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_, err := db.AppendEvents(ctx, "type_a", "agg-1", []core.NewEvent{
		{EventType: "X", Payload: json.RawMessage(`{}`)},
	})
	if err != nil {
		t.Fatalf("append agg-1: %v", err)
	}
	_, err = db.AppendEvents(ctx, "type_a", "agg-2", []core.NewEvent{
		{EventType: "Y", Payload: json.RawMessage(`{}`)},
	})
	if err != nil {
		t.Fatalf("append agg-2: %v", err)
	}
	// Different aggregate type — should not appear.
	_, err = db.AppendEvents(ctx, "type_b", "agg-3", []core.NewEvent{
		{EventType: "Z", Payload: json.RawMessage(`{}`)},
	})
	if err != nil {
		t.Fatalf("append agg-3: %v", err)
	}

	events, err := db.LoadAllEvents(ctx, "type_a")
	if err != nil {
		t.Fatalf("LoadAllEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for type_a, got %d", len(events))
	}
}

func TestLoadEventsEmpty(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	events, err := db.LoadEvents(ctx, "nonexistent", "no-such-id")
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}
