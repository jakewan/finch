package core_test

import (
	"context"
	"testing"

	"github.com/jakewan/finch/core"
)

func TestProjectBalancesSingleAccount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	// Record a starting balance transaction.
	_, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: acct.ID,
		Date:      date(2025, 1, 1),
		Amount:    500000, // $5000
		Name:      "Opening Balance",
		Status:    core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}

	// Create a monthly recurring rule: rent of -$1500 on the 1st.
	_, err = db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:  acct.ID,
		Name:       "Rent",
		Amount:     -150000,
		Frequency:  core.FrequencyMonthly,
		StartDate:  date(2025, 1, 1),
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	balances, err := db.ProjectBalances(ctx, date(2025, 1, 1), date(2025, 3, 31), []string{acct.ID})
	if err != nil {
		t.Fatalf("ProjectBalances: %v", err)
	}

	if len(balances) == 0 {
		t.Fatal("expected non-empty balances")
	}

	// First day: opening balance ($5000) + rent (-$1500) = $3500
	if balances[0].Balance != 350000 {
		t.Fatalf("expected balance 350000 on first day, got %d", balances[0].Balance)
	}

	// Find Feb 1st balance: $3500 + (-$1500) = $2000
	var febBalance *core.DailyBalance
	for i := range balances {
		if balances[i].Date.Equal(date(2025, 2, 1)) {
			febBalance = &balances[i]
			break
		}
	}
	if febBalance == nil {
		t.Fatal("expected balance entry for Feb 1")
	}
	if febBalance.Balance != 200000 {
		t.Fatalf("expected Feb 1 balance 200000, got %d", febBalance.Balance)
	}

	// Mar 1st: $2000 + (-$1500) = $500
	var marBalance *core.DailyBalance
	for i := range balances {
		if balances[i].Date.Equal(date(2025, 3, 1)) {
			marBalance = &balances[i]
			break
		}
	}
	if marBalance == nil {
		t.Fatal("expected balance entry for Mar 1")
	}
	if marBalance.Balance != 50000 {
		t.Fatalf("expected Mar 1 balance 50000, got %d", marBalance.Balance)
	}
}

func TestProjectBalancesNoDoubleCount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:  acct.ID,
		Name:       "Salary",
		Amount:     300000,
		Frequency:  core.FrequencyMonthly,
		StartDate:  date(2025, 1, 15),
		DayOfMonth: 15,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	// Record a transaction for Jan 15 matching this rule (simulating reconciliation).
	_, err = db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID:       acct.ID,
		Date:            date(2025, 1, 15),
		Amount:          300000,
		Name:            "Salary",
		Status:          core.TransactionStatusReconciled,
		RecurringRuleID: rule.ID,
	})
	if err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}

	balances, err := db.ProjectBalances(ctx, date(2025, 1, 1), date(2025, 2, 28), []string{acct.ID})
	if err != nil {
		t.Fatalf("ProjectBalances: %v", err)
	}

	// Jan 15 should have only one entry (the recorded one), not double-counted.
	var jan15 *core.DailyBalance
	for i := range balances {
		if balances[i].Date.Equal(date(2025, 1, 15)) {
			jan15 = &balances[i]
			break
		}
	}
	if jan15 == nil {
		t.Fatal("expected balance entry for Jan 15")
	}
	if jan15.Balance != 300000 {
		t.Fatalf("expected Jan 15 balance 300000 (no double count), got %d", jan15.Balance)
	}
	if len(jan15.Transactions) != 1 {
		t.Fatalf("expected 1 transaction on Jan 15, got %d", len(jan15.Transactions))
	}
}

func TestProjectBalanceOnDate(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	_, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: acct.ID,
		Date:      date(2025, 1, 1),
		Amount:    1000000,
		Name:      "Opening",
		Status:    core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}

	_, err = db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:  acct.ID,
		Name:       "Rent",
		Amount:     -150000,
		Frequency:  core.FrequencyMonthly,
		StartDate:  date(2025, 1, 1),
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	// Balance on April 1: $10000 + 4 months of -$1500 = $10000 - $6000 = $4000
	balance, err := db.ProjectBalanceOnDate(ctx, date(2025, 4, 1), acct.ID)
	if err != nil {
		t.Fatalf("ProjectBalanceOnDate: %v", err)
	}
	if balance != 400000 {
		t.Fatalf("expected balance 400000, got %d", balance)
	}
}

func TestProjectBalancesTransferNoDoubleCount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	checking := createTestAccount(t, db)
	savings, err := db.CreateAccount(ctx, "Savings", core.AccountTypeSavings)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	// Opening balance in checking.
	_, err = db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: checking.ID,
		Date:      date(2025, 1, 1),
		Amount:    500000,
		Name:      "Opening",
		Status:    core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}

	// Transfer $1000 from checking to savings.
	err = db.CreateTransfer(ctx, core.CreateTransferParams{
		SourceAccountID:      checking.ID,
		DestinationAccountID: savings.ID,
		Amount:               100000,
		Date:                 date(2025, 1, 15),
		Name:                 "Transfer to Savings",
		Status:               core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	balances, err := db.ProjectBalances(ctx, date(2025, 1, 1), date(2025, 1, 31), nil)
	if err != nil {
		t.Fatalf("ProjectBalances: %v", err)
	}

	// Find checking balance on Jan 15.
	var checkingJan15 int64
	var savingsJan15 int64
	for _, b := range balances {
		if b.Date.Equal(date(2025, 1, 15)) {
			switch b.AccountID {
			case checking.ID:
				checkingJan15 = b.Balance
			case savings.ID:
				savingsJan15 = b.Balance
			}
		}
	}

	// Checking: $5000 - $1000 = $4000
	if checkingJan15 != 400000 {
		t.Fatalf("expected checking balance 400000, got %d", checkingJan15)
	}
	// Savings: $0 + $1000 = $1000
	if savingsJan15 != 100000 {
		t.Fatalf("expected savings balance 100000, got %d", savingsJan15)
	}
}

func TestProjectBalancesEmptyAccount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	balances, err := db.ProjectBalances(ctx, date(2025, 1, 1), date(2025, 3, 31), []string{acct.ID})
	if err != nil {
		t.Fatalf("ProjectBalances: %v", err)
	}
	if len(balances) != 0 {
		t.Fatalf("expected 0 balances for empty account, got %d", len(balances))
	}
}

func TestProjectBalancesPausedRuleExcluded(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:  acct.ID,
		Name:       "Subscription",
		Amount:     -1499,
		Frequency:  core.FrequencyMonthly,
		StartDate:  date(2025, 1, 1),
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	if err := db.PauseRecurringRule(ctx, rule.ID); err != nil {
		t.Fatalf("PauseRecurringRule: %v", err)
	}

	balances, err := db.ProjectBalances(ctx, date(2025, 1, 1), date(2025, 3, 31), []string{acct.ID})
	if err != nil {
		t.Fatalf("ProjectBalances: %v", err)
	}
	if len(balances) != 0 {
		t.Fatalf("expected 0 balances (rule paused), got %d", len(balances))
	}
}

func TestProjectBalancesMixedRecurringAndOneOff(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	// Opening balance.
	_, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: acct.ID,
		Date:      date(2025, 1, 1),
		Amount:    200000,
		Name:      "Opening",
		Status:    core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("RecordTransaction opening: %v", err)
	}

	// Monthly subscription.
	_, err = db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:  acct.ID,
		Name:       "Netflix",
		Amount:     -1599,
		Frequency:  core.FrequencyMonthly,
		StartDate:  date(2025, 1, 15),
		DayOfMonth: 15,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	// One-off expense.
	_, err = db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: acct.ID,
		Date:      date(2025, 1, 20),
		Amount:    -5000,
		Name:      "Lunch",
		Status:    core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("RecordTransaction lunch: %v", err)
	}

	balances, err := db.ProjectBalances(ctx, date(2025, 1, 1), date(2025, 2, 28), []string{acct.ID})
	if err != nil {
		t.Fatalf("ProjectBalances: %v", err)
	}

	// Verify the final balance on Feb 15:
	// Opening: $2000.00
	// Jan 15 Netflix: -$15.99 → $1984.01
	// Jan 20 Lunch: -$50.00 → $1934.01
	// Feb 15 Netflix: -$15.99 → $1918.02
	expectedFinal := int64(200000 - 1599 - 5000 - 1599) // 191802
	var feb15 *core.DailyBalance
	for i := range balances {
		if balances[i].Date.Equal(date(2025, 2, 15)) {
			feb15 = &balances[i]
			break
		}
	}
	if feb15 == nil {
		t.Fatal("expected balance entry for Feb 15")
	}
	if feb15.Balance != expectedFinal {
		t.Fatalf("expected Feb 15 balance %d, got %d", expectedFinal, feb15.Balance)
	}
}

func TestProjectBalanceOnDateNoTransactionsOnDate(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	_, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: acct.ID,
		Date:      date(2025, 1, 1),
		Amount:    100000,
		Name:      "Opening",
		Status:    core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}

	// Query a date with no transactions — should show the prior balance.
	balance, err := db.ProjectBalanceOnDate(ctx, date(2025, 1, 10), acct.ID)
	if err != nil {
		t.Fatalf("ProjectBalanceOnDate: %v", err)
	}
	if balance != 100000 {
		t.Fatalf("expected balance 100000, got %d", balance)
	}
}

func TestProjectBalancesMultipleAccounts(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	checking := createTestAccount(t, db)
	savings, err := db.CreateAccount(ctx, "Savings", core.AccountTypeSavings)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	_, err = db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: checking.ID, Date: date(2025, 1, 1), Amount: 500000,
		Name: "Opening", Status: core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("RecordTransaction checking: %v", err)
	}

	_, err = db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: savings.ID, Date: date(2025, 1, 1), Amount: 1000000,
		Name: "Opening", Status: core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("RecordTransaction savings: %v", err)
	}

	// Project all accounts (nil = all).
	balances, err := db.ProjectBalances(ctx, date(2025, 1, 1), date(2025, 1, 1), nil)
	if err != nil {
		t.Fatalf("ProjectBalances: %v", err)
	}

	if len(balances) != 2 {
		t.Fatalf("expected 2 balance entries, got %d", len(balances))
	}

	// Both should be on Jan 1, ordered by account ID.
	foundChecking := false
	foundSavings := false
	for _, b := range balances {
		if b.AccountID == checking.ID && b.Balance == 500000 {
			foundChecking = true
		}
		if b.AccountID == savings.ID && b.Balance == 1000000 {
			foundSavings = true
		}
	}
	if !foundChecking {
		t.Fatal("missing checking balance")
	}
	if !foundSavings {
		t.Fatal("missing savings balance")
	}
}

// TestProjectBalanceOnDateWithRecurringRules verifies the convenience method
// correctly incorporates recurring rule expansions.
func TestProjectBalanceOnDateWithRecurringRules(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	// Opening: $10000
	_, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: acct.ID, Date: date(2025, 1, 1), Amount: 1000000,
		Name: "Opening", Status: core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}

	// Biweekly paycheck of $2000 starting Jan 3.
	_, err = db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID: acct.ID, Name: "Paycheck", Amount: 200000,
		Frequency: core.FrequencyBiweekly, StartDate: date(2025, 1, 3),
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	// Monthly rent of -$1500 on the 1st.
	_, err = db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID: acct.ID, Name: "Rent", Amount: -150000,
		Frequency: core.FrequencyMonthly, StartDate: date(2025, 1, 1), DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule rent: %v", err)
	}

	// Balance on Feb 1:
	// Jan 1: Opening $10000 + Rent -$1500 = $8500
	// Jan 3: Paycheck +$2000 = $10500
	// Jan 17: Paycheck +$2000 = $12500
	// Jan 31: Paycheck +$2000 = $14500
	// Feb 1: Rent -$1500 = $13000
	balance, err := db.ProjectBalanceOnDate(ctx, date(2025, 2, 1), acct.ID)
	if err != nil {
		t.Fatalf("ProjectBalanceOnDate: %v", err)
	}
	expected := int64(1000000 - 150000 + 200000 + 200000 + 200000 - 150000) // 1300000
	if balance != expected {
		t.Fatalf("expected balance %d, got %d", expected, balance)
	}
}
