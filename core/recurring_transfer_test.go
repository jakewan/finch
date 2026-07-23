package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jakewan/finch/core"
)

// validTransferParams is the baseline a transfer rule must satisfy; each test below
// spoils exactly one field so the assertion names the rule being enforced.
func validTransferParams(source, target string) core.CreateRecurringRuleParams {
	return core.CreateRecurringRuleParams{
		AccountID:               source,
		Name:                    "Monthly savings transfer",
		Amount:                  100000,
		Frequency:               core.FrequencyMonthly,
		StartDate:               date(2025, 1, 1),
		DayOfMonth:              1,
		IsTransfer:              true,
		TransferTargetAccountID: target,
	}
}

// TestCreateRecurringRuleRejectsUnprojectableTransfers guards the create path. Each
// case is a state the projection engine cannot honor, so it must never reach storage.
func TestCreateRecurringRuleRejectsUnprojectableTransfers(t *testing.T) {
	tests := []struct {
		name   string
		spoil  func(p *core.CreateRecurringRuleParams, savingsID string)
		reason string
	}{
		{
			name:   "empty target",
			spoil:  func(p *core.CreateRecurringRuleParams, _ string) { p.TransferTargetAccountID = "" },
			reason: "a transfer with no destination has nowhere to credit",
		},
		{
			name:   "non-existent target",
			spoil:  func(p *core.CreateRecurringRuleParams, _ string) { p.TransferTargetAccountID = "no-such-account" },
			reason: "crediting an account that does not exist silently drops the money",
		},
		{
			name:   "target same as source",
			spoil:  func(p *core.CreateRecurringRuleParams, _ string) { p.TransferTargetAccountID = p.AccountID },
			reason: "a self-transfer debits and credits the same account",
		},
		{
			name:   "zero amount",
			spoil:  func(p *core.CreateRecurringRuleParams, _ string) { p.Amount = 0 },
			reason: "a transfer must move a positive magnitude",
		},
		{
			name:   "negative amount",
			spoil:  func(p *core.CreateRecurringRuleParams, _ string) { p.Amount = -100000 },
			reason: "direction comes from source and destination, not the sign",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			ctx := context.Background()
			checking, savings := transferPair(t, db, 0)

			params := validTransferParams(checking.ID, savings.ID)
			tc.spoil(&params, savings.ID)

			_, err := db.CreateRecurringRule(ctx, params)
			if err == nil {
				t.Fatalf("CreateRecurringRule succeeded, want rejection: %s", tc.reason)
			}
			if !errors.Is(err, core.ErrInvalidInput) {
				t.Errorf("error %v is not core.ErrInvalidInput; callers must discriminate by type, not string", err)
			}
		})
	}
}

// TestCreateRecurringRuleAllowsNegativeNonTransferAmount pins the boundary of the
// rule above: the positive-magnitude requirement applies to transfers only. An
// ordinary expense rule stays signed.
func TestCreateRecurringRuleAllowsNegativeNonTransferAmount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	if _, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:  acct.ID,
		Name:       "Rent",
		Amount:     -150000,
		Frequency:  core.FrequencyMonthly,
		StartDate:  date(2025, 1, 1),
		DayOfMonth: 1,
	}); err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}
}

// TestUpdateRecurringRuleAmountRejectsUnprojectableTransfers guards the other write
// path. Validating only on create leaves the invariant holding right up until someone
// edits the rule.
func TestUpdateRecurringRuleAmountRejectsUnprojectableTransfers(t *testing.T) {
	amounts := []struct {
		name   string
		amount int64
	}{
		{"zero", 0},
		{"negative", -100000},
	}

	for _, tc := range amounts {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			ctx := context.Background()
			checking, savings := transferPair(t, db, 0)

			rule, err := db.CreateRecurringRule(ctx, validTransferParams(checking.ID, savings.ID))
			if err != nil {
				t.Fatalf("CreateRecurringRule: %v", err)
			}

			err = db.UpdateRecurringRuleAmount(ctx, rule.ID, tc.amount, date(2025, 2, 1), "edit")
			if err == nil {
				t.Fatal("UpdateRecurringRuleAmount succeeded, want rejection: an edit must not restore a state creation forbids")
			}
			if !errors.Is(err, core.ErrInvalidInput) {
				t.Errorf("error %v is not core.ErrInvalidInput", err)
			}
		})
	}
}

// TestUpdateRecurringRuleAmountAllowsNegativeForNonTransfer is the counterpart
// boundary on the update path.
func TestUpdateRecurringRuleAmountAllowsNegativeForNonTransfer(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
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

	if err := db.UpdateRecurringRuleAmount(ctx, rule.ID, -160000, date(2025, 2, 1), "rent increase"); err != nil {
		t.Fatalf("UpdateRecurringRuleAmount: %v", err)
	}
}
