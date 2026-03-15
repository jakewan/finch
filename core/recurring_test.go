package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/jakewan/finch/core"
)

func createTestAccount(t *testing.T, db *core.DB) *core.Account {
	t.Helper()
	acct, err := db.CreateAccount(context.Background(), "Test Account", core.AccountTypeChecking)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	return acct
}

func TestCreateAndListRecurringRules(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID: acct.ID,
		Name:      "Rent",
		Amount:    150000, // $1500.00
		Frequency: core.FrequencyMonthly,
		StartDate: date(2025, 1, 1),
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}
	if rule.ID == "" {
		t.Fatal("expected non-empty rule ID")
	}
	if rule.Amount != 150000 {
		t.Fatalf("expected amount 150000, got %d", rule.Amount)
	}

	rules, err := db.ListRecurringRules(ctx, acct.ID)
	if err != nil {
		t.Fatalf("ListRecurringRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Name != "Rent" {
		t.Fatalf("expected Rent, got %s", rules[0].Name)
	}
}

func TestCreateRecurringRuleValidation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	tests := []struct {
		name   string
		params core.CreateRecurringRuleParams
	}{
		{"empty name", core.CreateRecurringRuleParams{AccountID: acct.ID, Frequency: core.FrequencyMonthly, DayOfMonth: 1, StartDate: date(2025, 1, 1)}},
		{"empty account", core.CreateRecurringRuleParams{Name: "Test", Frequency: core.FrequencyMonthly, DayOfMonth: 1, StartDate: date(2025, 1, 1)}},
		{"invalid frequency", core.CreateRecurringRuleParams{AccountID: acct.ID, Name: "Test", Frequency: 99, StartDate: date(2025, 1, 1)}},
		{"monthly without day", core.CreateRecurringRuleParams{AccountID: acct.ID, Name: "Test", Frequency: core.FrequencyMonthly, StartDate: date(2025, 1, 1)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.CreateRecurringRule(ctx, tc.params); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestUpdateRecurringRuleAmount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:  acct.ID,
		Name:       "Car Payment",
		Amount:     45000,
		Frequency:  core.FrequencyMonthly,
		StartDate:  date(2025, 1, 15),
		DayOfMonth: 15,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	if err := db.UpdateRecurringRuleAmount(ctx, rule.ID, 50000, date(2025, 6, 1), "rate adjustment"); err != nil {
		t.Fatalf("UpdateRecurringRuleAmount: %v", err)
	}

	rules, err := db.ListRecurringRules(ctx, acct.ID)
	if err != nil {
		t.Fatalf("ListRecurringRules: %v", err)
	}
	if rules[0].Amount != 50000 {
		t.Fatalf("expected updated amount 50000, got %d", rules[0].Amount)
	}

	// Verify event history records the change.
	events, err := db.GetRecurringRuleHistory(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetRecurringRuleHistory: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].EventType != "RecurringRuleAmountChanged" {
		t.Fatalf("expected RecurringRuleAmountChanged, got %s", events[1].EventType)
	}
}

func TestPauseAndResumeRecurringRule(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:  acct.ID,
		Name:       "Subscription",
		Amount:     1499,
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

	rules, err := db.ListRecurringRules(ctx, acct.ID)
	if err != nil {
		t.Fatalf("ListRecurringRules: %v", err)
	}
	if !rules[0].Paused {
		t.Fatal("expected rule to be paused")
	}

	if err := db.ResumeRecurringRule(ctx, rule.ID); err != nil {
		t.Fatalf("ResumeRecurringRule: %v", err)
	}

	rules, err = db.ListRecurringRules(ctx, acct.ID)
	if err != nil {
		t.Fatalf("ListRecurringRules: %v", err)
	}
	if rules[0].Paused {
		t.Fatal("expected rule to be resumed")
	}
}

func TestEndRecurringRule(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	t.Run("sets end date on rule without one", func(t *testing.T) {
		rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
			AccountID:  acct.ID,
			Name:       "Gym Membership",
			Amount:     -5000,
			Frequency:  core.FrequencyMonthly,
			StartDate:  date(2025, 1, 1),
			DayOfMonth: 15,
		})
		if err != nil {
			t.Fatalf("CreateRecurringRule: %v", err)
		}

		endDate := date(2025, 6, 30)
		if err := db.EndRecurringRule(ctx, rule.ID, endDate); err != nil {
			t.Fatalf("EndRecurringRule: %v", err)
		}

		rules, err := db.ListRecurringRules(ctx, acct.ID)
		if err != nil {
			t.Fatalf("ListRecurringRules: %v", err)
		}
		var found bool
		for _, r := range rules {
			if r.ID == rule.ID {
				found = true
				if r.EndDate == nil {
					t.Fatal("expected non-nil end date")
				}
				if !r.EndDate.Equal(endDate) {
					t.Fatalf("expected end date %s, got %s", endDate, *r.EndDate)
				}
			}
		}
		if !found {
			t.Fatal("rule not found in list")
		}
	})

	t.Run("produces RecurringRuleEnded event", func(t *testing.T) {
		rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
			AccountID:  acct.ID,
			Name:       "Streaming Service",
			Amount:     -1499,
			Frequency:  core.FrequencyMonthly,
			StartDate:  date(2025, 1, 1),
			DayOfMonth: 1,
		})
		if err != nil {
			t.Fatalf("CreateRecurringRule: %v", err)
		}

		if err := db.EndRecurringRule(ctx, rule.ID, date(2025, 12, 31)); err != nil {
			t.Fatalf("EndRecurringRule: %v", err)
		}

		events, err := db.GetRecurringRuleHistory(ctx, rule.ID)
		if err != nil {
			t.Fatalf("GetRecurringRuleHistory: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}
		if events[1].EventType != "RecurringRuleEnded" {
			t.Fatalf("expected RecurringRuleEnded, got %s", events[1].EventType)
		}
	})

	t.Run("rejects empty rule ID", func(t *testing.T) {
		if err := db.EndRecurringRule(ctx, "", date(2025, 6, 30)); err == nil {
			t.Fatal("expected error for empty rule ID")
		}
	})

	t.Run("rejects zero end date", func(t *testing.T) {
		rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
			AccountID:  acct.ID,
			Name:       "Zero Date Test",
			Amount:     -100,
			Frequency:  core.FrequencyWeekly,
			StartDate:  date(2025, 1, 1),
		})
		if err != nil {
			t.Fatalf("CreateRecurringRule: %v", err)
		}
		if err := db.EndRecurringRule(ctx, rule.ID, time.Time{}); err == nil {
			t.Fatal("expected error for zero end date")
		}
	})
}

func TestListRecurringRulesAllAccounts(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	acct1 := createTestAccount(t, db)
	acct2, err := db.CreateAccount(ctx, "Savings", core.AccountTypeSavings)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	_, err = db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID: acct1.ID, Name: "Rule A", Amount: 100, Frequency: core.FrequencyWeekly,
		StartDate: date(2025, 1, 1),
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule A: %v", err)
	}
	_, err = db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID: acct2.ID, Name: "Rule B", Amount: 200, Frequency: core.FrequencyWeekly,
		StartDate: date(2025, 1, 1),
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule B: %v", err)
	}

	// Filter by account.
	rules, err := db.ListRecurringRules(ctx, acct1.ID)
	if err != nil {
		t.Fatalf("ListRecurringRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule for acct1, got %d", len(rules))
	}

	// All accounts.
	allRules, err := db.ListRecurringRules(ctx, "")
	if err != nil {
		t.Fatalf("ListRecurringRules all: %v", err)
	}
	if len(allRules) != 2 {
		t.Fatalf("expected 2 rules total, got %d", len(allRules))
	}
}

func TestRecurringRuleWithEndDate(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	endDate := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)
	rule, err := db.CreateRecurringRule(ctx, core.CreateRecurringRuleParams{
		AccountID:  acct.ID,
		Name:       "Temp Subscription",
		Amount:     999,
		Frequency:  core.FrequencyMonthly,
		StartDate:  date(2025, 1, 1),
		EndDate:    &endDate,
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	rules, err := db.ListRecurringRules(ctx, acct.ID)
	if err != nil {
		t.Fatalf("ListRecurringRules: %v", err)
	}
	if rules[0].EndDate == nil {
		t.Fatal("expected non-nil end date")
	}
	if !rules[0].EndDate.Equal(endDate) {
		t.Fatalf("expected end date %s, got %s", endDate, *rules[0].EndDate)
	}
	_ = rule
}
