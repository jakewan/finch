package core_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/jakewan/finch/core"
)

// amountInvariantCases is shared by every money-writing path below. The bound is a
// property of the system rather than of any one method, so one table driving all four
// writers is the claim being made — four separate tables would let a path drift out of
// agreement without any test noticing.
var amountInvariantCases = []struct {
	name   string
	amount int64
	wantOK bool
	reason string
}{
	{"zero", 0, false, "a zero amount records an event and shifts no balance"},
	{"ordinary expense", -5000, true, "a non-transfer amount is signed by design"},
	{"ordinary income", 250000, true, "income is the positive half of that same design"},
	{"at the positive cap", core.MaxAmountCents, true, "the cap itself is a storable amount"},
	{"at the negative cap", -core.MaxAmountCents, true, "the bound is on magnitude, not sign"},
	{"one past the positive cap", core.MaxAmountCents + 1, false,
		"past the cap the projection's int64 accumulation loses its overflow headroom"},
	{"one past the negative cap", -core.MaxAmountCents - 1, false,
		"the ceiling binds in both directions"},
	{"math.MaxInt64", math.MaxInt64, false, "the extreme an int64 field admits by default"},
	{"math.MinInt64", math.MinInt64, false,
		"MinInt64 has no positive counterpart — negating it wraps back to itself, so a bound " +
			"check routed through an absolute value would admit it"},
}

// assertInvariant is the shared verdict. A rejection must be discriminable by type:
// the daemon maps it to InvalidArgument via errors.Is, so a rejection that is merely
// non-nil would reach callers as Internal.
func assertInvariant(t *testing.T, err error, wantOK bool, amount int64, reason string) {
	t.Helper()
	if wantOK {
		if err != nil {
			t.Fatalf("amount %d rejected but should be accepted (%s): %v", amount, reason, err)
		}
		return
	}
	if err == nil {
		t.Fatalf("amount %d accepted, want rejection: %s", amount, reason)
	}
	if !errors.Is(err, core.ErrInvalidInput) {
		t.Errorf("error %v is not core.ErrInvalidInput; callers must discriminate by type, not string", err)
	}
}

func TestRecordTransactionEnforcesAmountMagnitude(t *testing.T) {
	for _, tc := range amountInvariantCases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			acct := createTestAccount(t, db)

			_, err := db.RecordTransaction(context.Background(), core.RecordTransactionParams{
				AccountID: acct.ID,
				Date:      date(2025, 1, 15),
				Amount:    tc.amount,
				Name:      "Coffee",
				Status:    core.TransactionStatusReconciled,
			})
			assertInvariant(t, err, tc.wantOK, tc.amount, tc.reason)
		})
	}
}

// A non-transfer rule, deliberately: the transfer variant already has a positive-magnitude
// check, which would satisfy several of these rows without the magnitude bound existing.
func TestCreateRecurringRuleEnforcesAmountMagnitude(t *testing.T) {
	for _, tc := range amountInvariantCases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			acct := createTestAccount(t, db)

			_, err := db.CreateRecurringRule(context.Background(), core.CreateRecurringRuleParams{
				AccountID:  acct.ID,
				Name:       "Rent",
				Amount:     tc.amount,
				Frequency:  core.FrequencyMonthly,
				StartDate:  date(2025, 1, 1),
				DayOfMonth: 1,
			})
			assertInvariant(t, err, tc.wantOK, tc.amount, tc.reason)
		})
	}
}

// The edit path must enforce what creation enforces, or the invariant holds only until
// someone changes the amount.
func TestUpdateRecurringRuleAmountEnforcesAmountMagnitude(t *testing.T) {
	for _, tc := range amountInvariantCases {
		t.Run(tc.name, func(t *testing.T) {
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

			err = db.UpdateRecurringRuleAmount(ctx, rule.ID, tc.amount, date(2025, 2, 1), "")
			assertInvariant(t, err, tc.wantOK, tc.amount, tc.reason)
		})
	}
}

// Where the two rules stack. A transfer carries the universal magnitude bound AND its own
// positive-magnitude requirement, so every negative row is rejected here even though the
// magnitude bound alone would admit it.
func TestCreateTransferEnforcesAmountMagnitude(t *testing.T) {
	for _, tc := range amountInvariantCases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			checking, savings := transferPair(t, db, 0)

			err := db.CreateTransfer(context.Background(), core.CreateTransferParams{
				SourceAccountID:      checking.ID,
				DestinationAccountID: savings.ID,
				Amount:               tc.amount,
				Date:                 date(2025, 1, 15),
				Name:                 "To Savings",
				Status:               core.TransactionStatusReconciled,
			})
			assertInvariant(t, err, tc.wantOK && tc.amount > 0, tc.amount, tc.reason)
		})
	}
}

// The amount is negative and in range on purpose. Zero would be caught by the magnitude
// check first and this test would pass against that error's wrap, never reaching the
// positivity check it exists to type — passing for a reason other than the one it names.
// A negative in-range amount is the only band where positivity is the sole rejecter.
func TestCreateTransferRejectsNonPositiveAmountWithTypedError(t *testing.T) {
	db := openTestDB(t)
	checking, savings := transferPair(t, db, 0)

	err := db.CreateTransfer(context.Background(), core.CreateTransferParams{
		SourceAccountID:      checking.ID,
		DestinationAccountID: savings.ID,
		Amount:               -100,
		Date:                 date(2025, 1, 15),
		Name:                 "To Savings",
		Status:               core.TransactionStatusReconciled,
	})
	if err == nil {
		t.Fatal("CreateTransfer accepted a negative amount; direction comes from the account pair")
	}
	if !errors.Is(err, core.ErrInvalidInput) {
		t.Errorf("error %v is not core.ErrInvalidInput; callers must discriminate by type, not string", err)
	}
}
