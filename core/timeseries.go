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

	// Build a map: accountID → date → balance (cumulative end-of-day).
	// We track the running balance per account across the daily data.
	acctBalanceOnDate := make(map[string]map[string]int64) // accountID → dateStr → balance
	for _, id := range resolvedIDs {
		acctBalanceOnDate[id] = make(map[string]int64)
	}
	for _, db := range dailyBalances {
		dateKey := db.Date.Format(time.DateOnly)
		acctBalanceOnDate[db.AccountID][dateKey] = db.Balance
	}

	// Generate sample dates.
	sampleDates := generateSampleDates(from, to, interval)

	// For each sample date, find the most recent known balance for each account.
	result := make([]BalanceTimePoint, len(sampleDates))
	for i, sampleDate := range sampleDates {
		dateKey := sampleDate.Format(time.DateOnly)
		point := BalanceTimePoint{Date: sampleDate}
		for _, id := range resolvedIDs {
			bal := lookupBalance(acctBalanceOnDate[id], startBalances[id], from, sampleDate, dateKey)
			point.Balances = append(point.Balances, AccountBalance{
				AccountID: id,
				Balance:   bal,
			})
		}
		result[i] = point
	}

	return result, nil
}

// lookupBalance finds the most recent known balance on or before sampleDate.
func lookupBalance(dateMap map[string]int64, startBalance int64, from, sampleDate time.Time, sampleKey string) int64 {
	// Check exact date first.
	if bal, ok := dateMap[sampleKey]; ok {
		return bal
	}
	// Walk backward from sampleDate to from looking for the last known balance.
	cursor := sampleDate.AddDate(0, 0, -1)
	for !cursor.Before(from) {
		key := cursor.Format(time.DateOnly)
		if bal, ok := dateMap[key]; ok {
			return bal
		}
		cursor = cursor.AddDate(0, 0, -1)
	}
	// No daily data found in range — use starting balance.
	return startBalance
}

func generateSampleDates(from, to time.Time, interval TimeSeriesInterval) []time.Time {
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
		}
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})
	return dates
}
