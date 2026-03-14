package core

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// TimeSeriesInterval controls the sampling frequency.
type TimeSeriesInterval int

const (
	TimeSeriesDaily   TimeSeriesInterval = 1
	TimeSeriesWeekly  TimeSeriesInterval = 2
	TimeSeriesMonthly TimeSeriesInterval = 3
)

// AccountBalance holds the balance for one account at a point in time.
type AccountBalance struct {
	AccountID string
	Balance   int64
}

// BalanceTimePoint holds per-account balances at a sampled date.
type BalanceTimePoint struct {
	Date     time.Time
	Balances []AccountBalance
}

// GetBalanceTimeSeries samples account balances at the given interval over
// a date range. It builds on ProjectBalances for the daily data, then
// samples at the requested interval.
func (db *DB) GetBalanceTimeSeries(ctx context.Context, from, to time.Time, interval TimeSeriesInterval, accountIDs []string) ([]BalanceTimePoint, error) {
	// Resolve which accounts to include.
	var resolvedIDs []string
	if len(accountIDs) > 0 {
		resolvedIDs = accountIDs
	} else {
		accounts, err := db.ListAccounts(ctx)
		if err != nil {
			return nil, fmt.Errorf("list accounts: %w", err)
		}
		for _, a := range accounts {
			resolvedIDs = append(resolvedIDs, a.ID)
		}
	}
	if len(resolvedIDs) == 0 {
		return nil, nil
	}

	// Get starting balance per account (balance on day before `from`).
	startBalances := make(map[string]int64)
	for _, id := range resolvedIDs {
		bal, err := db.ProjectBalanceOnDate(ctx, from.AddDate(0, 0, -1), id)
		if err != nil {
			return nil, fmt.Errorf("starting balance for %s: %w", id, err)
		}
		startBalances[id] = bal
	}

	// Get daily balances in the range.
	dailyBalances, err := db.ProjectBalances(ctx, from, to, resolvedIDs)
	if err != nil {
		return nil, fmt.Errorf("project balances: %w", err)
	}

	// Build sorted known dates per account for efficient sampling.
	acctBalanceOnDate := make(map[string]map[string]int64)
	for _, id := range resolvedIDs {
		acctBalanceOnDate[id] = make(map[string]int64)
	}
	for _, daily := range dailyBalances {
		dateKey := daily.Date.Format(time.DateOnly)
		acctBalanceOnDate[daily.AccountID][dateKey] = daily.Balance
	}

	// Build sorted date lists per account for linear sampling.
	acctSortedDates := make(map[string][]string)
	for id, dateMap := range acctBalanceOnDate {
		dates := make([]string, 0, len(dateMap))
		for d := range dateMap {
			dates = append(dates, d)
		}
		sort.Strings(dates)
		acctSortedDates[id] = dates
	}

	// Generate sample dates.
	sampleDates, err := generateSampleDates(from, to, interval)
	if err != nil {
		return nil, err
	}

	// Track last-known-balance cursors per account for linear sampling.
	acctCursors := make(map[string]int)
	for _, id := range resolvedIDs {
		acctCursors[id] = 0
	}

	result := make([]BalanceTimePoint, len(sampleDates))
	for i, sampleDate := range sampleDates {
		sampleKey := sampleDate.Format(time.DateOnly)
		point := BalanceTimePoint{Date: sampleDate}
		for _, id := range resolvedIDs {
			bal := sampleBalance(acctBalanceOnDate[id], acctSortedDates[id], &acctCursors, id, sampleKey, startBalances[id])
			point.Balances = append(point.Balances, AccountBalance{
				AccountID: id,
				Balance:   bal,
			})
		}
		result[i] = point
	}

	return result, nil
}

// sampleBalance finds the most recent known balance on or before sampleKey
// by advancing a cursor through sorted known dates. O(1) amortized per sample.
func sampleBalance(dateMap map[string]int64, sortedDates []string, cursors *map[string]int, id, sampleKey string, startBalance int64) int64 {
	cursor := (*cursors)[id]
	// Advance cursor to the last known date <= sampleKey.
	for cursor < len(sortedDates) && sortedDates[cursor] <= sampleKey {
		cursor++
	}
	(*cursors)[id] = cursor

	if cursor > 0 {
		return dateMap[sortedDates[cursor-1]]
	}
	return startBalance
}

func generateSampleDates(from, to time.Time, interval TimeSeriesInterval) ([]time.Time, error) {
	var dates []time.Time
	cursor := from
	for !cursor.After(to) {
		dates = append(dates, cursor)
		switch interval {
		case TimeSeriesDaily:
			cursor = cursor.AddDate(0, 0, 1)
		case TimeSeriesWeekly:
			cursor = cursor.AddDate(0, 0, 7)
		case TimeSeriesMonthly:
			cursor = cursor.AddDate(0, 1, 0)
		default:
			return nil, fmt.Errorf("unsupported interval: %d", interval)
		}
	}
	return dates, nil
}
