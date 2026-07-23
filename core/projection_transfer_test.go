package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/jakewan/finch/core"
)

// transferPair creates a checking and a savings account with an opening balance
// in checking, the starting point every recurring-transfer scenario below shares.
func transferPair(t *testing.T, db *core.DB, opening int64) (checking, savings *core.Account) {
	t.Helper()
	ctx := context.Background()

	checking = createTestAccount(t, db)
	savings, err := db.CreateAccount(ctx, "Savings", core.AccountTypeSavings)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	if opening != 0 {
		if _, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
			AccountID: checking.ID,
			Date:      date(2025, 1, 1),
			Amount:    opening,
			Name:      "Opening",
			Status:    core.TransactionStatusReconciled,
		}); err != nil {
			t.Fatalf("RecordTransaction: %v", err)
		}
	}
	return checking, savings
}

// balanceOn is a readability wrapper over ProjectBalanceOnDate.
func balanceOn(t *testing.T, db *core.DB, d time.Time, accountID string) int64 {
	t.Helper()
	bal, err := db.ProjectBalanceOnDate(context.Background(), d, accountID)
	if err != nil {
		t.Fatalf("ProjectBalanceOnDate(%s): %v", d.Format(time.DateOnly), err)
	}
	return bal
}

// TestProjectBalancesRecurringTransferConservesMoney is the property that defines
// correctness: a transfer moves money, it does not create or destroy it. The
// combined balance across both accounts must be unchanged on every single day of
// the projection, at every magnitude.
//
// This subsumes the scenarios below — a sign error, an asymmetric dedup, or a leg
// that stops on the wrong date all show up here as a non-constant total, including
// at magnitudes no scenario author thought to write down.
func TestProjectBalancesRecurringTransferConservesMoney(t *testing.T) {
	magnitudes := []struct {
		name   string
		amount int64
	}{
		{"small", 100},
		{"typical", 100000},        // $1,000
		{"large", 500000000},       // $5,000,000
		{"past int32", 3000000000}, // exceeds 2^31 cents
	}

	for _, m := range magnitudes {
		t.Run(m.name, func(t *testing.T) {
			db := openTestDB(t)
			ctx := context.Background()

			opening := m.amount * 12
			checking, savings := transferPair(t, db, opening)

			if _, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
				AccountID:               checking.ID,
				Name:                    "Monthly savings transfer",
				Amount:                  m.amount,
				Frequency:               core.FrequencyMonthly,
				StartDate:               date(2025, 1, 1),
				DayOfMonth:              1,
				IsTransfer:              true,
				TransferTargetAccountID: savings.ID,
			}); err != nil {
				t.Fatalf("CreateRecurringRule: %v", err)
			}

			for d := date(2025, 1, 1); !d.After(date(2025, 6, 30)); d = d.AddDate(0, 0, 1) {
				total := balanceOn(t, db, d, checking.ID) + balanceOn(t, db, d, savings.ID)
				if total != opening {
					t.Fatalf("money not conserved on %s: checking+savings = %d, want %d",
						d.Format(time.DateOnly), total, opening)
				}
			}
		})
	}
}

// TestProjectBalancesRecurringTransferCreditsDestination is the headline behavior:
// each occurrence debits the source and credits the destination by the same amount.
func TestProjectBalancesRecurringTransferCreditsDestination(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 500000) // $5,000

	if _, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:               checking.ID,
		Name:                    "Monthly savings transfer",
		Amount:                  100000, // $1,000, a positive magnitude
		Frequency:               core.FrequencyMonthly,
		StartDate:               date(2025, 1, 1),
		DayOfMonth:              1,
		IsTransfer:              true,
		TransferTargetAccountID: savings.ID,
	}); err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	// After three occurrences (Jan 1, Feb 1, Mar 1): checking $5,000 - $3,000,
	// savings $0 + $3,000.
	if got := balanceOn(t, db, date(2025, 3, 31), checking.ID); got != 200000 {
		t.Errorf("checking balance = %d, want 200000", got)
	}
	if got := balanceOn(t, db, date(2025, 3, 31), savings.ID); got != 300000 {
		t.Errorf("savings balance = %d, want 300000", got)
	}
}

// TestProjectBalancesRecurringTransferDestinationHasNoDataOfItsOwn covers the
// destination account that owns no transactions and no rules. Its projection has
// to start from the inbound rule alone; an account-scoped "earliest relevant date"
// lookup finds nothing for it and would flatline the balance at zero.
func TestProjectBalancesRecurringTransferDestinationHasNoDataOfItsOwn(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 500000)

	if _, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:               checking.ID,
		Name:                    "Monthly savings transfer",
		Amount:                  100000,
		Frequency:               core.FrequencyMonthly,
		StartDate:               date(2025, 1, 1),
		DayOfMonth:              1,
		IsTransfer:              true,
		TransferTargetAccountID: savings.ID,
	}); err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	balances, err := db.ProjectBalances(ctx, date(2025, 1, 1), date(2025, 2, 28), []string{savings.ID})
	if err != nil {
		t.Fatalf("ProjectBalances: %v", err)
	}
	if len(balances) == 0 {
		t.Fatal("destination account projected no balances at all; the inbound rule was never seen")
	}
	if got := balanceOn(t, db, date(2025, 2, 28), savings.ID); got != 200000 {
		t.Errorf("savings balance = %d, want 200000", got)
	}
}

// TestProjectBalancesRecurringTransferPriorOccurrencesInStartingBalance checks the
// starting-balance path rather than the in-range expansion path: occurrences that
// fall before the projection window still have to land in the destination's opening
// figure, or the window silently starts too low.
func TestProjectBalancesRecurringTransferPriorOccurrencesInStartingBalance(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 500000)

	if _, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:               checking.ID,
		Name:                    "Monthly savings transfer",
		Amount:                  100000,
		Frequency:               core.FrequencyMonthly,
		StartDate:               date(2025, 1, 1),
		DayOfMonth:              1,
		IsTransfer:              true,
		TransferTargetAccountID: savings.ID,
	}); err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	// Project only April onward. Jan/Feb/Mar/Apr occurrences precede or open the
	// window; savings must already hold $3,000 before April's lands.
	balances, err := db.ProjectBalances(ctx, date(2025, 4, 1), date(2025, 4, 30), []string{savings.ID})
	if err != nil {
		t.Fatalf("ProjectBalances: %v", err)
	}
	if len(balances) == 0 {
		t.Fatal("expected an April occurrence in the destination's projection")
	}
	if got := balances[0].Balance; got != 400000 {
		t.Errorf("savings balance on Apr 1 = %d, want 400000 (three prior occurrences plus April's)", got)
	}
}

// TestProjectBalancesRecurringTransferPausedMovesNeitherSide guards against the
// pause check being applied on the source path but skipped on the inbound one.
func TestProjectBalancesRecurringTransferPausedMovesNeitherSide(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 500000)

	rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:               checking.ID,
		Name:                    "Monthly savings transfer",
		Amount:                  100000,
		Frequency:               core.FrequencyMonthly,
		StartDate:               date(2025, 1, 1),
		DayOfMonth:              1,
		IsTransfer:              true,
		TransferTargetAccountID: savings.ID,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}
	if err := db.PauseRecurringRule(ctx, rule.ID); err != nil {
		t.Fatalf("PauseRecurringRule: %v", err)
	}

	if got := balanceOn(t, db, date(2025, 3, 31), checking.ID); got != 500000 {
		t.Errorf("checking balance = %d, want 500000 (paused rule must not debit)", got)
	}
	if got := balanceOn(t, db, date(2025, 3, 31), savings.ID); got != 0 {
		t.Errorf("savings balance = %d, want 0 (paused rule must not credit)", got)
	}
}

// TestProjectBalancesRecurringTransferEndDateStopsBothLegs checks that the two legs
// stop on the same date. A destination that keeps crediting past the end date is the
// asymmetry that quietly manufactures money.
func TestProjectBalancesRecurringTransferEndDateStopsBothLegs(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 500000)

	endDate := date(2025, 2, 28)
	if _, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:               checking.ID,
		Name:                    "Monthly savings transfer",
		Amount:                  100000,
		Frequency:               core.FrequencyMonthly,
		StartDate:               date(2025, 1, 1),
		EndDate:                 &endDate,
		DayOfMonth:              1,
		IsTransfer:              true,
		TransferTargetAccountID: savings.ID,
	}); err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	// Only Jan 1 and Feb 1 occur. March must move nothing on either side.
	if got := balanceOn(t, db, date(2025, 5, 31), checking.ID); got != 300000 {
		t.Errorf("checking balance = %d, want 300000 (two occurrences only)", got)
	}
	if got := balanceOn(t, db, date(2025, 5, 31), savings.ID); got != 200000 {
		t.Errorf("savings balance = %d, want 200000 (two occurrences only)", got)
	}
}

// TestProjectBalancesRecurringTransferMaterializedNoDoubleCount records a real
// transfer against the rule's occurrence date. CreateTransfer stamps the rule ID on
// both legs, so both accounts should replace the projection with the recorded entry
// rather than counting both.
func TestProjectBalancesRecurringTransferMaterializedNoDoubleCount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 500000)

	rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:               checking.ID,
		Name:                    "Monthly savings transfer",
		Amount:                  100000,
		Frequency:               core.FrequencyMonthly,
		StartDate:               date(2025, 1, 1),
		DayOfMonth:              1,
		IsTransfer:              true,
		TransferTargetAccountID: savings.ID,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	// Materialize the February occurrence for real.
	if err := db.CreateTransfer(ctx, core.CreateTransferParams{
		SourceAccountID:      checking.ID,
		DestinationAccountID: savings.ID,
		Amount:               100000,
		Date:                 date(2025, 2, 1),
		Name:                 "Monthly savings transfer",
		Status:               core.TransactionStatusReconciled,
		RecurringRuleID:      rule.ID,
	}); err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	// Jan and Feb together move $2,000 — not $3,000.
	if got := balanceOn(t, db, date(2025, 2, 28), checking.ID); got != 300000 {
		t.Errorf("checking balance = %d, want 300000 (February counted once)", got)
	}
	if got := balanceOn(t, db, date(2025, 2, 28), savings.ID); got != 200000 {
		t.Errorf("savings balance = %d, want 200000 (February counted once)", got)
	}
}

// TestProjectBalancesRecurringTransferSingleLegMaterializationCreatesNoMoney is the
// asymmetric case. RecordTransaction accepts a recurring rule ID for one account, so
// the source leg alone can be materialized while the destination leg is not.
//
// Dedup is deliberately scoped to the account that recorded the entry: the recorded
// debit replaces the source's projection, and the destination — which recorded
// nothing — still projects its credit, so the pair stays conserved and the occurrence
// is counted exactly once on each side. Scoping dedup to the rule instead would
// suppress that credit while the recorded debit stands, destroying the money the
// debit moved.
func TestProjectBalancesRecurringTransferSingleLegMaterializationCreatesNoMoney(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	checking, savings := transferPair(t, db, 500000)

	rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:               checking.ID,
		Name:                    "Monthly savings transfer",
		Amount:                  100000,
		Frequency:               core.FrequencyMonthly,
		StartDate:               date(2025, 1, 1),
		DayOfMonth:              1,
		IsTransfer:              true,
		TransferTargetAccountID: savings.ID,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	// Record only the source leg, carrying the rule ID.
	if _, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID:       checking.ID,
		Date:            date(2025, 1, 1),
		Amount:          -100000,
		Name:            "Monthly savings transfer",
		Status:          core.TransactionStatusReconciled,
		RecurringRuleID: rule.ID,
	}); err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}

	// January's occurrence is materialized on the source and must not be
	// double-credited on the destination.
	if got := balanceOn(t, db, date(2025, 1, 31), checking.ID); got != 400000 {
		t.Errorf("checking balance = %d, want 400000", got)
	}
	if got := balanceOn(t, db, date(2025, 1, 31), savings.ID); got != 100000 {
		t.Errorf("savings balance = %d, want 100000 (one credit, not two)", got)
	}
}
