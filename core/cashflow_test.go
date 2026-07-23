package core_test

import (
	"context"
	"testing"

	"github.com/jakewan/finch/core"
)

func monthCashFlow(t *testing.T, flows []core.MonthlyCashFlow, month string) core.MonthlyCashFlow {
	t.Helper()
	for _, cf := range flows {
		if cf.Month == month {
			return cf
		}
	}
	t.Fatalf("no cash flow entry for %s", month)
	return core.MonthlyCashFlow{}
}

// TestMonthlyCashFlowExcludesProjectedTransfers pins the accounting meaning of a
// transfer: moving money between your own accounts is neither income nor an expense.
// Counting both legs would inflate gross income by the user's own savings rate.
func TestMonthlyCashFlowExcludesProjectedTransfers(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 500000)

	if _, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:               checking.ID,
		Name:                    "Monthly savings transfer",
		Amount:                  100000,
		Frequency:               core.FrequencyMonthly,
		StartDate:               date(2025, 2, 1),
		DayOfMonth:              1,
		IsTransfer:              true,
		TransferTargetAccountID: savings.ID,
	}); err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	flows, err := db.GetMonthlyCashFlow(ctx, "2025-02", "2025-02", nil)
	if err != nil {
		t.Fatalf("GetMonthlyCashFlow: %v", err)
	}

	feb := monthCashFlow(t, flows, "2025-02")
	if feb.Income != 0 {
		t.Errorf("Income = %d, want 0: a transfer into savings is not income", feb.Income)
	}
	if feb.Expenses != 0 {
		t.Errorf("Expenses = %d, want 0: a transfer out of checking is not an expense", feb.Expenses)
	}
	if feb.Net != 0 {
		t.Errorf("Net = %d, want 0", feb.Net)
	}
}

// TestMonthlyCashFlowExcludesRecordedTransfers is the same rule for a materialized
// transfer. Without this, a transfer changes classification the moment it stops being
// a projection and becomes a recorded row — the number moves for no visible reason.
func TestMonthlyCashFlowExcludesRecordedTransfers(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 500000)

	if err := db.CreateTransfer(ctx, core.CreateTransferParams{
		SourceAccountID:      checking.ID,
		DestinationAccountID: savings.ID,
		Amount:               100000,
		Date:                 date(2025, 2, 15),
		Name:                 "Transfer to Savings",
		Status:               core.TransactionStatusReconciled,
	}); err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	flows, err := db.GetMonthlyCashFlow(ctx, "2025-02", "2025-02", nil)
	if err != nil {
		t.Fatalf("GetMonthlyCashFlow: %v", err)
	}

	feb := monthCashFlow(t, flows, "2025-02")
	if feb.Income != 0 {
		t.Errorf("Income = %d, want 0", feb.Income)
	}
	if feb.Expenses != 0 {
		t.Errorf("Expenses = %d, want 0", feb.Expenses)
	}
}

// TestMonthlyCashFlowStillCountsOrdinaryActivity is the boundary: the transfer
// exclusion must not swallow real income and expenses sharing the same month.
func TestMonthlyCashFlowStillCountsOrdinaryActivity(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 0)

	if _, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: checking.ID,
		Date:      date(2025, 2, 1),
		Amount:    400000,
		Name:      "Paycheck",
		Status:    core.TransactionStatusReconciled,
	}); err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}
	if _, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: checking.ID,
		Date:      date(2025, 2, 3),
		Amount:    -150000,
		Name:      "Rent",
		Status:    core.TransactionStatusReconciled,
	}); err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}
	if err := db.CreateTransfer(ctx, core.CreateTransferParams{
		SourceAccountID:      checking.ID,
		DestinationAccountID: savings.ID,
		Amount:               100000,
		Date:                 date(2025, 2, 15),
		Name:                 "Transfer to Savings",
		Status:               core.TransactionStatusReconciled,
	}); err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	flows, err := db.GetMonthlyCashFlow(ctx, "2025-02", "2025-02", nil)
	if err != nil {
		t.Fatalf("GetMonthlyCashFlow: %v", err)
	}

	feb := monthCashFlow(t, flows, "2025-02")
	if feb.Income != 400000 {
		t.Errorf("Income = %d, want 400000", feb.Income)
	}
	if feb.Expenses != -150000 {
		t.Errorf("Expenses = %d, want -150000", feb.Expenses)
	}
	if feb.Net != 250000 {
		t.Errorf("Net = %d, want 250000", feb.Net)
	}
}

// TestMonthlyCashFlowSingleAccountExcludesTransfer records the second-order effect of
// the exclusion. For a single-account query a transfer out used to depress Net; now it
// contributes nothing, which is the accounting-correct answer but a visible change.
func TestMonthlyCashFlowSingleAccountExcludesTransfer(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 0)

	if err := db.CreateTransfer(ctx, core.CreateTransferParams{
		SourceAccountID:      checking.ID,
		DestinationAccountID: savings.ID,
		Amount:               100000,
		Date:                 date(2025, 2, 15),
		Name:                 "Transfer to Savings",
		Status:               core.TransactionStatusReconciled,
	}); err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	flows, err := db.GetMonthlyCashFlow(ctx, "2025-02", "2025-02", []string{checking.ID})
	if err != nil {
		t.Fatalf("GetMonthlyCashFlow: %v", err)
	}

	feb := monthCashFlow(t, flows, "2025-02")
	if feb.Net != 0 {
		t.Errorf("Net = %d, want 0: money moved to another own account is not net outflow", feb.Net)
	}
	// The money left the account even though it was not an expense, so it has to be
	// reported somewhere or the statement cannot be reconciled against the balance.
	if feb.Transfers != -100000 {
		t.Errorf("Transfers = %d, want -100000", feb.Transfers)
	}
}

// TestMonthlyCashFlowReconcilesAgainstBalanceChange is the property that makes the
// report auditable: over any period, income plus expenses plus transfers must equal
// the change in projected balance. Classifying transfers correctly without reporting
// them would satisfy the other tests while making this one impossible.
func TestMonthlyCashFlowReconcilesAgainstBalanceChange(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 0)

	if _, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: checking.ID, Date: date(2025, 2, 1), Amount: 400000,
		Name: "Paycheck", Status: core.TransactionStatusReconciled,
	}); err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}
	if _, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: checking.ID, Date: date(2025, 2, 3), Amount: -150000,
		Name: "Rent", Status: core.TransactionStatusReconciled,
	}); err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}
	if err := db.CreateTransfer(ctx, core.CreateTransferParams{
		SourceAccountID: checking.ID, DestinationAccountID: savings.ID,
		Amount: 100000, Date: date(2025, 2, 15),
		Name: "Transfer to Savings", Status: core.TransactionStatusReconciled,
	}); err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	flows, err := db.GetMonthlyCashFlow(ctx, "2025-02", "2025-02", []string{checking.ID})
	if err != nil {
		t.Fatalf("GetMonthlyCashFlow: %v", err)
	}
	feb := monthCashFlow(t, flows, "2025-02")

	balanceChange := balanceOn(t, db, date(2025, 2, 28), checking.ID) -
		balanceOn(t, db, date(2025, 1, 31), checking.ID)
	if got := feb.Income + feb.Expenses + feb.Transfers; got != balanceChange {
		t.Errorf("income+expenses+transfers = %d, but the balance moved %d", got, balanceChange)
	}
}
