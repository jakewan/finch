package core_test

import (
	"testing"
	"time"

	"github.com/jakewan/finch/core"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func datePtr(y int, m time.Month, d int) *time.Time {
	t := date(y, m, d)
	return &t
}

func TestExpandWeekly(t *testing.T) {
	start := date(2025, 1, 6) // Monday
	from := date(2025, 1, 6)
	to := date(2025, 1, 27)

	dates := core.ExpandOccurrences(core.FrequencyWeekly, start, from, to, 0, [2]int{}, nil)
	if len(dates) != 4 {
		t.Fatalf("expected 4 weekly occurrences, got %d", len(dates))
	}
	expected := []time.Time{
		date(2025, 1, 6), date(2025, 1, 13), date(2025, 1, 20), date(2025, 1, 27),
	}
	for i, d := range dates {
		if !d.Equal(expected[i]) {
			t.Errorf("occurrence %d: expected %s, got %s", i, expected[i], d)
		}
	}
}

func TestExpandBiweekly(t *testing.T) {
	start := date(2025, 1, 3) // Friday
	from := date(2025, 1, 1)
	to := date(2025, 2, 28)

	dates := core.ExpandOccurrences(core.FrequencyBiweekly, start, from, to, 0, [2]int{}, nil)
	expected := []time.Time{
		date(2025, 1, 3), date(2025, 1, 17), date(2025, 1, 31), date(2025, 2, 14), date(2025, 2, 28),
	}
	if len(dates) != len(expected) {
		t.Fatalf("expected %d biweekly occurrences, got %d", len(expected), len(dates))
	}
	for i, d := range dates {
		if !d.Equal(expected[i]) {
			t.Errorf("occurrence %d: expected %s, got %s", i, expected[i], d)
		}
	}
}

func TestExpandSemiMonthly(t *testing.T) {
	from := date(2025, 1, 1)
	to := date(2025, 3, 31)
	days := [2]int{1, 15}

	dates := core.ExpandOccurrences(core.FrequencySemiMonthly, from, from, to, 0, days, nil)
	if len(dates) != 6 {
		t.Fatalf("expected 6 semi-monthly occurrences, got %d", len(dates))
	}
	expected := []time.Time{
		date(2025, 1, 1), date(2025, 1, 15),
		date(2025, 2, 1), date(2025, 2, 15),
		date(2025, 3, 1), date(2025, 3, 15),
	}
	for i, d := range dates {
		if !d.Equal(expected[i]) {
			t.Errorf("occurrence %d: expected %s, got %s", i, expected[i], d)
		}
	}
}

func TestExpandMonthly(t *testing.T) {
	start := date(2025, 1, 15)
	from := date(2025, 1, 1)
	to := date(2025, 4, 30)

	dates := core.ExpandOccurrences(core.FrequencyMonthly, start, from, to, 15, [2]int{}, nil)
	if len(dates) != 4 {
		t.Fatalf("expected 4 monthly occurrences, got %d", len(dates))
	}
}

func TestExpandMonthlyDayCapping(t *testing.T) {
	start := date(2025, 1, 31)
	from := date(2025, 1, 1)
	to := date(2025, 4, 30)

	dates := core.ExpandOccurrences(core.FrequencyMonthly, start, from, to, 31, [2]int{}, nil)
	// Jan 31, Feb 28, Mar 31, Apr 30
	expected := []time.Time{
		date(2025, 1, 31), date(2025, 2, 28), date(2025, 3, 31), date(2025, 4, 30),
	}
	if len(dates) != len(expected) {
		t.Fatalf("expected %d, got %d", len(expected), len(dates))
	}
	for i, d := range dates {
		if !d.Equal(expected[i]) {
			t.Errorf("occurrence %d: expected %s, got %s", i, expected[i], d)
		}
	}
}

func TestExpandMonthlyLeapYear(t *testing.T) {
	start := date(2024, 1, 29)
	from := date(2024, 1, 1)
	to := date(2024, 3, 31)

	dates := core.ExpandOccurrences(core.FrequencyMonthly, start, from, to, 29, [2]int{}, nil)
	expected := []time.Time{
		date(2024, 1, 29), date(2024, 2, 29), date(2024, 3, 29),
	}
	if len(dates) != len(expected) {
		t.Fatalf("expected %d, got %d", len(expected), len(dates))
	}
	for i, d := range dates {
		if !d.Equal(expected[i]) {
			t.Errorf("occurrence %d: expected %s, got %s", i, expected[i], d)
		}
	}
}

func TestExpandYearly(t *testing.T) {
	start := date(2020, 3, 15)
	from := date(2023, 1, 1)
	to := date(2026, 12, 31)

	dates := core.ExpandOccurrences(core.FrequencyYearly, start, from, to, 0, [2]int{}, nil)
	expected := []time.Time{
		date(2023, 3, 15), date(2024, 3, 15), date(2025, 3, 15), date(2026, 3, 15),
	}
	if len(dates) != len(expected) {
		t.Fatalf("expected %d yearly occurrences, got %d", len(expected), len(dates))
	}
}

func TestExpandWithEndDate(t *testing.T) {
	start := date(2025, 1, 1)
	from := date(2025, 1, 1)
	to := date(2025, 12, 31)
	endDate := datePtr(2025, 3, 15)

	dates := core.ExpandOccurrences(core.FrequencyMonthly, start, from, to, 1, [2]int{}, endDate)
	// Should stop at end date: Jan 1, Feb 1, Mar 1
	if len(dates) != 3 {
		t.Fatalf("expected 3 occurrences with end date, got %d", len(dates))
	}
}

func TestExpandWeeklyFromMidRange(t *testing.T) {
	start := date(2025, 1, 1)
	from := date(2025, 1, 10) // Start querying after the rule started
	to := date(2025, 1, 31)

	dates := core.ExpandOccurrences(core.FrequencyWeekly, start, from, to, 0, [2]int{}, nil)
	// First occurrence on or after Jan 10 from a Jan 1 weekly: Jan 15, Jan 22, Jan 29
	if len(dates) != 3 {
		t.Fatalf("expected 3 occurrences, got %d", len(dates))
	}
	if !dates[0].Equal(date(2025, 1, 15)) {
		t.Errorf("first occurrence: expected 2025-01-15, got %s", dates[0])
	}
}
