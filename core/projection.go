package core

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// ProjectedTransaction represents a single transaction entry in a projection.
type ProjectedTransaction struct {
	Date            time.Time
	Amount          int64
	Name            string
	AccountID       string
	RecurringRuleID string
	Status          TransactionStatus
	IsProjected     bool // true if generated from a recurring rule, false if from a recorded transaction
}

// DailyBalance represents the projected balance for one account on one day.
type DailyBalance struct {
	Date         time.Time
	AccountID    string
	Balance      int64
	Transactions []ProjectedTransaction
}

// ProjectionComparison shows how a projection changed between two points in time.
type ProjectionComparison struct {
	TargetDate    time.Time
	AsOfDate1     time.Time
	AsOfDate2     time.Time
	AccountDeltas []AccountDelta
}

// AccountDelta shows the balance difference for one account between two projections.
type AccountDelta struct {
	AccountID       string
	BalanceAsOf1    int64
	BalanceAsOf2    int64
	Delta           int64
}

// ProjectBalances computes daily balances for the given date range by merging
// recorded transactions with expanded recurring rule occurrences.
func (db *DB) ProjectBalances(ctx context.Context, from, to time.Time, accountIDs []string) ([]DailyBalance, error) {
	var accounts []Account
	var err error

	if len(accountIDs) > 0 {
		for _, id := range accountIDs {
			acct, err := db.getAccount(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("get account %s: %w", id, err)
			}
			accounts = append(accounts, *acct)
		}
	} else {
		accounts, err = db.ListAccounts(ctx)
		if err != nil {
			return nil, fmt.Errorf("list accounts: %w", err)
		}
	}

	var allBalances []DailyBalance

	for _, acct := range accounts {
		balances, err := db.projectAccountBalances(ctx, acct.ID, from, to)
		if err != nil {
			return nil, fmt.Errorf("project account %s: %w", acct.ID, err)
		}
		allBalances = append(allBalances, balances...)
	}

	sort.Slice(allBalances, func(i, j int) bool {
		if allBalances[i].Date.Equal(allBalances[j].Date) {
			return allBalances[i].AccountID < allBalances[j].AccountID
		}
		return allBalances[i].Date.Before(allBalances[j].Date)
	})

	return allBalances, nil
}

// ProjectBalanceOnDate returns the projected balance for a single account on a specific date.
// It projects from the earliest relevant date through the target date to capture
// all recorded transactions and recurring rule expansions.
func (db *DB) ProjectBalanceOnDate(ctx context.Context, targetDate time.Time, accountID string) (int64, error) {
	// Find the earliest relevant date (earliest transaction or rule start).
	earliest, err := db.earliestDateForAccount(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if earliest == nil {
		return 0, nil // No data for this account.
	}

	balances, err := db.projectAccountBalances(ctx, accountID, *earliest, targetDate)
	if err != nil {
		return 0, err
	}
	if len(balances) == 0 {
		return 0, nil
	}
	return balances[len(balances)-1].Balance, nil
}

func (db *DB) projectAccountBalances(ctx context.Context, accountID string, from, to time.Time) ([]DailyBalance, error) {
	// Load recurring rules (needed for both starting balance and range projection).
	rules, err := db.ListRecurringRules(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list recurring rules: %w", err)
	}

	// Starting balance = recorded transactions before `from` + recurring rule
	// expansions before `from` (excluding dates that have recorded transactions
	// linked to the same rule).
	beforeFrom := from.AddDate(0, 0, -1)
	recordedBalance, err := db.computeBalanceUpTo(ctx, accountID, beforeFrom)
	if err != nil {
		return nil, fmt.Errorf("compute recorded balance: %w", err)
	}

	// Build a set of (date, ruleID) pairs that already have recorded transactions
	// before the range, so we don't double-count recurring rule expansions.
	priorRecorded, err := db.recordedRuleDatesBefore(ctx, accountID, from)
	if err != nil {
		return nil, fmt.Errorf("load prior recorded rule dates: %w", err)
	}

	// Add recurring rule occurrences before `from` to the starting balance.
	startingBalance := recordedBalance
	for _, rule := range rules {
		if rule.Paused {
			continue
		}
		priorOccurrences := ExpandOccurrences(
			rule.Frequency, rule.StartDate, rule.StartDate, beforeFrom,
			rule.DayOfMonth, rule.SemiMonthlyDays, rule.EndDate,
		)
		for _, occ := range priorOccurrences {
			dateKey := occ.Format(time.DateOnly)
			if priorRecorded[dateKey] != nil && priorRecorded[dateKey][rule.ID] {
				continue
			}
			startingBalance += rule.Amount
		}
	}

	// Collect all transaction entries in the [from, to] range.
	var entries []ProjectedTransaction

	// 1. Recorded transactions in the date range.
	txns, err := db.listTransactionsInRange(ctx, accountID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list transactions in range: %w", err)
	}
	recordedDates := make(map[string]map[string]bool) // date -> recurringRuleID -> exists
	for _, t := range txns {
		entries = append(entries, ProjectedTransaction{
			Date:            t.Date,
			Amount:          t.Amount,
			Name:            t.Name,
			AccountID:       accountID,
			RecurringRuleID: t.RecurringRuleID,
			Status:          t.Status,
			IsProjected:     false,
		})
		if t.RecurringRuleID != "" {
			dateKey := t.Date.Format(time.DateOnly)
			if recordedDates[dateKey] == nil {
				recordedDates[dateKey] = make(map[string]bool)
			}
			recordedDates[dateKey][t.RecurringRuleID] = true
		}
	}

	// 2. Expand recurring rules in the [from, to] range.
	for _, rule := range rules {
		if rule.Paused {
			continue
		}
		occurrences := ExpandOccurrences(
			rule.Frequency, rule.StartDate, from, to,
			rule.DayOfMonth, rule.SemiMonthlyDays, rule.EndDate,
		)
		for _, occ := range occurrences {
			dateKey := occ.Format(time.DateOnly)
			if recordedDates[dateKey] != nil && recordedDates[dateKey][rule.ID] {
				continue
			}
			entries = append(entries, ProjectedTransaction{
				Date:            occ,
				Amount:          rule.Amount,
				Name:            rule.Name,
				AccountID:       accountID,
				RecurringRuleID: rule.ID,
				Status:          TransactionStatusProjected,
				IsProjected:     true,
			})
		}
	}

	// Sort entries by date, then name for stable ordering.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date.Equal(entries[j].Date) {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Date.Before(entries[j].Date)
	})

	// Build daily balances.
	balancesByDate := make(map[string]*DailyBalance)
	var dateOrder []string

	runningBalance := startingBalance
	for _, entry := range entries {
		dateKey := entry.Date.Format(time.DateOnly)
		runningBalance += entry.Amount

		if _, exists := balancesByDate[dateKey]; !exists {
			balancesByDate[dateKey] = &DailyBalance{
				Date:      entry.Date,
				AccountID: accountID,
			}
			dateOrder = append(dateOrder, dateKey)
		}
		balancesByDate[dateKey].Balance = runningBalance
		balancesByDate[dateKey].Transactions = append(balancesByDate[dateKey].Transactions, entry)
	}

	result := make([]DailyBalance, len(dateOrder))
	for i, dk := range dateOrder {
		result[i] = *balancesByDate[dk]
	}
	return result, nil
}

// earliestDateForAccount finds the earliest date relevant for projection:
// the minimum of the earliest transaction date and earliest recurring rule start date.
func (db *DB) earliestDateForAccount(ctx context.Context, accountID string) (*time.Time, error) {
	var earliest *time.Time

	var txnDate sql.NullString
	row := db.conn.QueryRowContext(ctx,
		"SELECT MIN(date) FROM transactions WHERE account_id = ?", accountID)
	if err := row.Scan(&txnDate); err != nil {
		return nil, fmt.Errorf("earliest transaction date: %w", err)
	}
	if txnDate.Valid {
		t, err := time.Parse(time.DateOnly, txnDate.String)
		if err != nil {
			return nil, fmt.Errorf("parse transaction date: %w", err)
		}
		earliest = &t
	}

	var ruleDate sql.NullString
	row = db.conn.QueryRowContext(ctx,
		"SELECT MIN(start_date) FROM recurring_rules WHERE account_id = ? AND paused = 0", accountID)
	if err := row.Scan(&ruleDate); err != nil {
		return nil, fmt.Errorf("earliest rule start date: %w", err)
	}
	if ruleDate.Valid {
		t, err := time.Parse(time.DateOnly, ruleDate.String)
		if err != nil {
			return nil, fmt.Errorf("parse rule date: %w", err)
		}
		if earliest == nil || t.Before(*earliest) {
			earliest = &t
		}
	}

	return earliest, nil
}

// recordedRuleDatesBefore returns a map of date -> ruleID for all transactions
// with a recurring_rule_id before the given date. Used to avoid double-counting
// recurring rule expansions when computing starting balances.
func (db *DB) recordedRuleDatesBefore(ctx context.Context, accountID string, before time.Time) (map[string]map[string]bool, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT date, recurring_rule_id FROM transactions
		 WHERE account_id = ? AND date < ? AND recurring_rule_id IS NOT NULL`,
		accountID, before.Format(time.DateOnly),
	)
	if err != nil {
		return nil, fmt.Errorf("query recorded rule dates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]map[string]bool)
	for rows.Next() {
		var dateStr, ruleID string
		if err := rows.Scan(&dateStr, &ruleID); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if result[dateStr] == nil {
			result[dateStr] = make(map[string]bool)
		}
		result[dateStr][ruleID] = true
	}
	return result, rows.Err()
}

// computeBalanceUpTo returns the sum of all transaction amounts on or before the given date.
func (db *DB) computeBalanceUpTo(ctx context.Context, accountID string, upTo time.Time) (int64, error) {
	var balance int64
	row := db.conn.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM transactions
		 WHERE account_id = ? AND date <= ?`,
		accountID, upTo.Format(time.DateOnly),
	)
	if err := row.Scan(&balance); err != nil {
		return 0, fmt.Errorf("sum transactions: %w", err)
	}
	return balance, nil
}

// listTransactionsInRange returns transactions for an account within [from, to].
func (db *DB) listTransactionsInRange(ctx context.Context, accountID string, from, to time.Time) ([]Transaction, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, account_id, date, amount, name, description, status, recurring_rule_id
		 FROM transactions
		 WHERE account_id = ? AND date >= ? AND date <= ?
		 ORDER BY date, name`,
		accountID, from.Format(time.DateOnly), to.Format(time.DateOnly),
	)
	if err != nil {
		return nil, fmt.Errorf("query transactions in range: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanTransactions(rows)
}

// getAccount returns a single account by ID.
func (db *DB) getAccount(ctx context.Context, id string) (*Account, error) {
	row := db.conn.QueryRowContext(ctx,
		"SELECT id, name, type, created_at, updated_at FROM accounts WHERE id = ?", id)
	var a Account
	var createdUnix, updatedUnix int64
	if err := row.Scan(&a.ID, &a.Name, &a.Type, &createdUnix, &updatedUnix); err != nil {
		return nil, fmt.Errorf("get account %s: %w", id, err)
	}
	a.CreatedAt = time.Unix(createdUnix, 0).UTC()
	a.UpdatedAt = time.Unix(updatedUnix, 0).UTC()
	return &a, nil
}
