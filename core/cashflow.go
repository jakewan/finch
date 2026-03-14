package core

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// MonthlyCashFlow summarizes income, expenses, and net for a single month.
type MonthlyCashFlow struct {
	Month    string // "2025-01"
	Income   int64  // sum of positive transaction amounts (cents)
	Expenses int64  // sum of negative transaction amounts (cents, stored as negative)
	Net      int64  // income + expenses
}

// GetMonthlyCashFlow aggregates income and expenses per month over a date range.
// It reuses ProjectBalances to capture both recorded transactions and recurring
// rule expansions, then groups by month.
func (db *DB) GetMonthlyCashFlow(ctx context.Context, fromMonth, toMonth string, accountIDs []string) ([]MonthlyCashFlow, error) {
	fromDate, err := time.Parse("2006-01", fromMonth)
	if err != nil {
		return nil, fmt.Errorf("parse from_month %q: %w", fromMonth, err)
	}
	toDate, err := time.Parse("2006-01", toMonth)
	if err != nil {
		return nil, fmt.Errorf("parse to_month %q: %w", toMonth, err)
	}
	// Last day of toMonth.
	toDateEnd := toDate.AddDate(0, 1, -1)

	balances, err := db.ProjectBalances(ctx, fromDate, toDateEnd, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("project balances: %w", err)
	}

	// Group transactions by month.
	monthData := make(map[string]*MonthlyCashFlow)
	for _, b := range balances {
		for _, txn := range b.Transactions {
			month := txn.Date.Format("2006-01")
			cf, ok := monthData[month]
			if !ok {
				cf = &MonthlyCashFlow{Month: month}
				monthData[month] = cf
			}
			if txn.Amount > 0 {
				cf.Income += txn.Amount
			} else {
				cf.Expenses += txn.Amount
			}
		}
	}

	// Fill gap months with zero values.
	cursor := fromDate
	for !cursor.After(toDate) {
		month := cursor.Format("2006-01")
		if _, ok := monthData[month]; !ok {
			monthData[month] = &MonthlyCashFlow{Month: month}
		}
		cursor = cursor.AddDate(0, 1, 0)
	}

	// Compute net and collect results.
	result := make([]MonthlyCashFlow, 0, len(monthData))
	for _, cf := range monthData {
		cf.Net = cf.Income + cf.Expenses
		result = append(result, *cf)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Month < result[j].Month
	})

	return result, nil
}
