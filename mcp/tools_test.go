package main

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jakewan/finch/core"
	"github.com/jakewan/finch/daemon/finchd"
	finchv1 "github.com/jakewan/finch/daemon/gen/finch/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// startTestBackend spins up a real gRPC server backed by a temp SQLite DB
// and returns a client suitable for constructing MCP handler closures.
func startTestBackend(t *testing.T) finchv1.FinchServiceClient {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := core.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() { _ = lis.Close() })

	srv := grpc.NewServer()
	finchv1.RegisterFinchServiceServer(srv, finchd.NewServer(db))
	t.Cleanup(func() { srv.Stop() })

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("serve: %v", err)
		}
	}()

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return finchv1.NewFinchServiceClient(conn)
}

// createTestAccount creates a checking account and returns its ID.
func createTestAccount(t *testing.T, client finchv1.FinchServiceClient) string {
	t.Helper()
	ctx := context.Background()
	resp, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
		Name: "Test Checking",
		Type: finchv1.AccountType_ACCOUNT_TYPE_CHECKING,
	})
	if err != nil {
		t.Fatalf("create test account: %v", err)
	}
	return resp.Account.Id
}

// createTestRule creates a monthly recurring rule and returns its ID.
func createTestRule(t *testing.T, client finchv1.FinchServiceClient, accountID string) string {
	t.Helper()
	ctx := context.Background()
	resp, err := client.CreateRecurringRule(ctx, &finchv1.CreateRecurringRuleRequest{
		AccountId:  accountID,
		Name:       "Monthly Rent",
		Amount:     -150000,
		Frequency:  finchv1.Frequency_FREQUENCY_MONTHLY,
		StartDate:  "2025-01-01",
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("create test rule: %v", err)
	}
	return resp.Rule.Id
}

// --- Ping ---

func TestPingTool(t *testing.T) {
	client := startTestBackend(t)
	handler := newPingHandler(client)

	t.Run("returns_daemon_version", func(t *testing.T) {
		_, out, err := handler(context.Background(), nil, PingInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Version != finchd.Version {
			t.Fatalf("expected version %s, got %s", finchd.Version, out.Version)
		}
	})
}

// --- Account Tools ---

func TestAccountTools(t *testing.T) {
	client := startTestBackend(t)
	ctx := context.Background()

	t.Run("create_account", func(t *testing.T) {
		handler := newCreateAccountHandler(client)

		t.Run("creates_account_with_short_form_type", func(t *testing.T) {
			_, out, err := handler(ctx, nil, CreateAccountInput{
				Name: "My Checking",
				Type: "CHECKING",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Account.ID == "" {
				t.Fatal("expected non-empty account ID")
			}
			if out.Account.Name != "My Checking" {
				t.Fatalf("expected name 'My Checking', got %q", out.Account.Name)
			}
			if !strings.Contains(out.Account.Type, "CHECKING") {
				t.Fatalf("expected type containing CHECKING, got %q", out.Account.Type)
			}
		})

		t.Run("accepts_prefixed_ACCOUNT_TYPE_CHECKING", func(t *testing.T) {
			_, out, err := handler(ctx, nil, CreateAccountInput{
				Name: "Prefixed",
				Type: "ACCOUNT_TYPE_SAVINGS",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out.Account.Type, "SAVINGS") {
				t.Fatalf("expected type containing SAVINGS, got %q", out.Account.Type)
			}
		})

		t.Run("normalizes_lowercase_and_whitespace", func(t *testing.T) {
			cases := []struct {
				name  string
				input string
			}{
				{"lowercase", "checking"},
				{"mixed case", "Checking"},
				{"with whitespace", "  CHECKING  "},
				{"lowercase prefixed", "account_type_checking"},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					_, _, err := handler(ctx, nil, CreateAccountInput{
						Name: "Norm " + tc.name,
						Type: tc.input,
					})
					if err != nil {
						t.Fatalf("input %q: unexpected error: %v", tc.input, err)
					}
				})
			}
		})

		t.Run("rejects_unknown_type_with_sorted_valid_options", func(t *testing.T) {
			_, _, err := handler(ctx, nil, CreateAccountInput{
				Name: "Bad",
				Type: "INVALID_TYPE",
			})
			if err == nil {
				t.Fatal("expected error for unknown account type")
			}
			errMsg := err.Error()
			if !strings.Contains(errMsg, "unknown account type") {
				t.Fatalf("expected 'unknown account type' in error, got: %s", errMsg)
			}
			if !strings.Contains(errMsg, "CHECKING") {
				t.Fatalf("expected valid options in error, got: %s", errMsg)
			}
		})
	})

	t.Run("list_accounts", func(t *testing.T) {
		// Use a fresh backend for isolation.
		freshClient := startTestBackend(t)
		handler := newListAccountsHandler(freshClient)

		t.Run("returns_empty_list_when_no_accounts", func(t *testing.T) {
			_, out, err := handler(ctx, nil, ListAccountsInput{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Accounts) != 0 {
				t.Fatalf("expected 0 accounts, got %d", len(out.Accounts))
			}
		})

		t.Run("returns_all_created_accounts", func(t *testing.T) {
			createHandler := newCreateAccountHandler(freshClient)
			_, _, err := createHandler(ctx, nil, CreateAccountInput{Name: "Acct1", Type: "CHECKING"})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			_, _, err = createHandler(ctx, nil, CreateAccountInput{Name: "Acct2", Type: "SAVINGS"})
			if err != nil {
				t.Fatalf("create: %v", err)
			}

			_, out, err := handler(ctx, nil, ListAccountsInput{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Accounts) != 2 {
				t.Fatalf("expected 2 accounts, got %d", len(out.Accounts))
			}
		})
	})

	t.Run("get_account_history", func(t *testing.T) {
		handler := newGetAccountHistoryHandler(client)
		accountID := createTestAccount(t, client)

		t.Run("returns_creation_event_with_correct_fields", func(t *testing.T) {
			_, out, err := handler(ctx, nil, GetAccountHistoryInput{AccountID: accountID})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Events) < 1 {
				t.Fatal("expected at least 1 event")
			}
			evt := out.Events[0]
			if evt.EventType != "AccountCreated" {
				t.Fatalf("expected AccountCreated, got %q", evt.EventType)
			}
			if evt.Sequence != 1 {
				t.Fatalf("expected sequence 1, got %d", evt.Sequence)
			}
			if evt.RecordedAt == 0 {
				t.Fatal("expected non-zero recorded_at")
			}
		})
	})
}

// --- Recurring Rule Tools ---

func TestRecurringRuleTools(t *testing.T) {
	client := startTestBackend(t)
	ctx := context.Background()
	accountID := createTestAccount(t, client)

	t.Run("create_recurring_rule", func(t *testing.T) {
		handler := newCreateRecurringRuleHandler(client)

		t.Run("creates_monthly_rule", func(t *testing.T) {
			_, out, err := handler(ctx, nil, CreateRecurringRuleInput{
				AccountID:  accountID,
				Name:       "Rent",
				Amount:     -150000,
				Frequency:  "MONTHLY",
				StartDate:  "2025-01-01",
				DayOfMonth: 1,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Rule.ID == "" {
				t.Fatal("expected non-empty rule ID")
			}
			if out.Rule.Name != "Rent" {
				t.Fatalf("expected name 'Rent', got %q", out.Rule.Name)
			}
			if out.Rule.Amount != -150000 {
				t.Fatalf("expected amount -150000, got %d", out.Rule.Amount)
			}
		})

		t.Run("accepts_prefixed_FREQUENCY_MONTHLY", func(t *testing.T) {
			_, out, err := handler(ctx, nil, CreateRecurringRuleInput{
				AccountID:  accountID,
				Name:       "Paycheck",
				Amount:     300000,
				Frequency:  "FREQUENCY_BIWEEKLY",
				StartDate:  "2025-01-10",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out.Rule.Frequency, "BIWEEKLY") {
				t.Fatalf("expected frequency containing BIWEEKLY, got %q", out.Rule.Frequency)
			}
		})

		t.Run("normalizes_lowercase_frequency", func(t *testing.T) {
			cases := []struct {
				name       string
				input      string
				dayOfMonth int32
			}{
				{"lowercase", "weekly", 0},
				{"mixed case", "Weekly", 0},
				{"with whitespace", "  MONTHLY  ", 1},
				{"lowercase prefixed", "frequency_yearly", 0},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					_, _, err := handler(ctx, nil, CreateRecurringRuleInput{
						AccountID:  accountID,
						Name:       "Norm " + tc.name,
						Amount:     1000,
						Frequency:  tc.input,
						StartDate:  "2025-06-01",
						DayOfMonth: tc.dayOfMonth,
					})
					if err != nil {
						t.Fatalf("input %q: unexpected error: %v", tc.input, err)
					}
				})
			}
		})

		t.Run("rejects_unknown_frequency_with_sorted_valid_options", func(t *testing.T) {
			_, _, err := handler(ctx, nil, CreateRecurringRuleInput{
				AccountID: accountID,
				Name:      "Bad",
				Amount:    1000,
				Frequency: "QUARTERLY",
				StartDate: "2025-01-01",
			})
			if err == nil {
				t.Fatal("expected error for unknown frequency")
			}
			errMsg := err.Error()
			if !strings.Contains(errMsg, "unknown frequency") {
				t.Fatalf("expected 'unknown frequency' in error, got: %s", errMsg)
			}
			if !strings.Contains(errMsg, "MONTHLY") {
				t.Fatalf("expected valid options in error, got: %s", errMsg)
			}
		})

		t.Run("maps_all_output_fields_correctly", func(t *testing.T) {
			_, out, err := handler(ctx, nil, CreateRecurringRuleInput{
				AccountID:  accountID,
				Name:       "Full Fields",
				Amount:     -50000,
				Frequency:  "MONTHLY",
				StartDate:  "2025-03-01",
				EndDate:    "2025-12-31",
				DayOfMonth: 15,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			r := out.Rule
			if r.AccountID != accountID {
				t.Fatalf("expected account_id %q, got %q", accountID, r.AccountID)
			}
			if r.StartDate != "2025-03-01" {
				t.Fatalf("expected start_date 2025-03-01, got %q", r.StartDate)
			}
			if r.EndDate != "2025-12-31" {
				t.Fatalf("expected end_date 2025-12-31, got %q", r.EndDate)
			}
			if r.DayOfMonth != 15 {
				t.Fatalf("expected day_of_month 15, got %d", r.DayOfMonth)
			}
			if r.Paused {
				t.Fatal("expected paused=false for new rule")
			}
		})
	})

	t.Run("list_recurring_rules", func(t *testing.T) {
		freshClient := startTestBackend(t)
		handler := newListRecurringRulesHandler(freshClient)

		t.Run("returns_empty_list_when_none_exist", func(t *testing.T) {
			_, out, err := handler(ctx, nil, ListRecurringRulesInput{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Rules) != 0 {
				t.Fatalf("expected 0 rules, got %d", len(out.Rules))
			}
		})

		t.Run("filters_by_account_id", func(t *testing.T) {
			acctID := createTestAccount(t, freshClient)
			createTestRule(t, freshClient, acctID)

			// Create a second account with its own rule.
			resp, err := freshClient.CreateAccount(ctx, &finchv1.CreateAccountRequest{
				Name: "Other",
				Type: finchv1.AccountType_ACCOUNT_TYPE_SAVINGS,
			})
			if err != nil {
				t.Fatalf("create other account: %v", err)
			}
			createTestRule(t, freshClient, resp.Account.Id)

			_, out, err := handler(ctx, nil, ListRecurringRulesInput{AccountID: acctID})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Rules) != 1 {
				t.Fatalf("expected 1 rule for account, got %d", len(out.Rules))
			}
		})

		t.Run("returns_all_when_no_filter", func(t *testing.T) {
			_, out, err := handler(ctx, nil, ListRecurringRulesInput{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Rules) < 2 {
				t.Fatalf("expected at least 2 rules, got %d", len(out.Rules))
			}
		})
	})

	t.Run("update_recurring_rule_amount", func(t *testing.T) {
		handler := newUpdateRecurringRuleAmountHandler(client)
		ruleID := createTestRule(t, client, accountID)

		t.Run("updates_amount_successfully", func(t *testing.T) {
			_, _, err := handler(ctx, nil, UpdateRecurringRuleAmountInput{
				RuleID:        ruleID,
				NewAmount:     -175000,
				EffectiveDate: "2025-06-01",
				Reason:        "rent increase",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})

	t.Run("get_recurring_rule_history", func(t *testing.T) {
		handler := newGetRecurringRuleHistoryHandler(client)
		ruleID := createTestRule(t, client, accountID)

		// Update the rule so there's more than one event.
		updateHandler := newUpdateRecurringRuleAmountHandler(client)
		_, _, err := updateHandler(ctx, nil, UpdateRecurringRuleAmountInput{
			RuleID:        ruleID,
			NewAmount:     -200000,
			EffectiveDate: "2025-07-01",
		})
		if err != nil {
			t.Fatalf("update rule: %v", err)
		}

		t.Run("returns_creation_and_update_events", func(t *testing.T) {
			_, out, err := handler(ctx, nil, GetRecurringRuleHistoryInput{RuleID: ruleID})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Events) < 2 {
				t.Fatalf("expected at least 2 events, got %d", len(out.Events))
			}
			if out.Events[0].EventType != "RecurringRuleCreated" {
				t.Fatalf("expected first event RecurringRuleCreated, got %q", out.Events[0].EventType)
			}
		})
	})
}

// --- Transaction Tools ---

func TestTransactionTools(t *testing.T) {
	client := startTestBackend(t)
	ctx := context.Background()
	accountID := createTestAccount(t, client)

	t.Run("record_transaction", func(t *testing.T) {
		handler := newRecordTransactionHandler(client)

		t.Run("records_with_short_form_status", func(t *testing.T) {
			_, out, err := handler(ctx, nil, RecordTransactionInput{
				AccountID: accountID,
				Date:      "2025-01-15",
				Amount:    -5000,
				Name:      "Coffee",
				Status:    "RECONCILED",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Transaction.ID == "" {
				t.Fatal("expected non-empty transaction ID")
			}
			if out.Transaction.Name != "Coffee" {
				t.Fatalf("expected name 'Coffee', got %q", out.Transaction.Name)
			}
		})

		t.Run("accepts_prefixed_TRANSACTION_STATUS_RECONCILED", func(t *testing.T) {
			_, _, err := handler(ctx, nil, RecordTransactionInput{
				AccountID: accountID,
				Date:      "2025-01-16",
				Amount:    -3000,
				Name:      "Lunch",
				Status:    "TRANSACTION_STATUS_SCHEDULED",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		t.Run("normalizes_lowercase_status", func(t *testing.T) {
			cases := []struct {
				name  string
				input string
			}{
				{"lowercase", "reconciled"},
				{"mixed case", "Projected"},
				{"with whitespace", "  SCHEDULED  "},
				{"lowercase prefixed", "transaction_status_reconciled"},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					_, _, err := handler(ctx, nil, RecordTransactionInput{
						AccountID: accountID,
						Date:      "2025-02-01",
						Amount:    -100,
						Name:      "Norm " + tc.name,
						Status:    tc.input,
					})
					if err != nil {
						t.Fatalf("input %q: unexpected error: %v", tc.input, err)
					}
				})
			}
		})

		t.Run("rejects_unknown_status_with_sorted_valid_options", func(t *testing.T) {
			_, _, err := handler(ctx, nil, RecordTransactionInput{
				AccountID: accountID,
				Date:      "2025-01-17",
				Amount:    -100,
				Name:      "Bad",
				Status:    "PENDING",
			})
			if err == nil {
				t.Fatal("expected error for unknown status")
			}
			errMsg := err.Error()
			if !strings.Contains(errMsg, "unknown status") {
				t.Fatalf("expected 'unknown status' in error, got: %s", errMsg)
			}
			if !strings.Contains(errMsg, "RECONCILED") {
				t.Fatalf("expected valid options in error, got: %s", errMsg)
			}
		})

		t.Run("maps_all_output_fields_correctly", func(t *testing.T) {
			ruleID := createTestRule(t, client, accountID)
			_, out, err := handler(ctx, nil, RecordTransactionInput{
				AccountID:       accountID,
				Date:            "2025-03-01",
				Amount:          -150000,
				Name:            "March Rent",
				Description:     "Monthly rent payment",
				Status:          "RECONCILED",
				RecurringRuleID: ruleID,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			txn := out.Transaction
			if txn.AccountID != accountID {
				t.Fatalf("expected account_id %q, got %q", accountID, txn.AccountID)
			}
			if txn.Date != "2025-03-01" {
				t.Fatalf("expected date 2025-03-01, got %q", txn.Date)
			}
			if txn.Amount != -150000 {
				t.Fatalf("expected amount -150000, got %d", txn.Amount)
			}
			if txn.Description != "Monthly rent payment" {
				t.Fatalf("expected description, got %q", txn.Description)
			}
			if txn.RecurringRuleID != ruleID {
				t.Fatalf("expected recurring_rule_id %q, got %q", ruleID, txn.RecurringRuleID)
			}
		})
	})

	t.Run("list_transactions", func(t *testing.T) {
		handler := newListTransactionsHandler(client)

		t.Run("returns_empty_list_when_no_transactions", func(t *testing.T) {
			freshClient := startTestBackend(t)
			emptyAcctID := createTestAccount(t, freshClient)
			emptyHandler := newListTransactionsHandler(freshClient)

			_, out, err := emptyHandler(ctx, nil, ListTransactionsInput{
				AccountID: emptyAcctID,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Transactions) != 0 {
				t.Fatalf("expected 0 transactions, got %d", len(out.Transactions))
			}
		})

		t.Run("returns_recorded_transactions", func(t *testing.T) {
			recHandler := newRecordTransactionHandler(client)
			_, _, err := recHandler(ctx, nil, RecordTransactionInput{
				AccountID: accountID,
				Date:      "2025-06-01",
				Amount:    -7500,
				Name:      "List Test Txn 1",
				Status:    "RECONCILED",
			})
			if err != nil {
				t.Fatalf("record txn 1: %v", err)
			}
			_, _, err = recHandler(ctx, nil, RecordTransactionInput{
				AccountID: accountID,
				Date:      "2025-06-02",
				Amount:    -3000,
				Name:      "List Test Txn 2",
				Status:    "SCHEDULED",
			})
			if err != nil {
				t.Fatalf("record txn 2: %v", err)
			}

			_, out, err := handler(ctx, nil, ListTransactionsInput{
				AccountID: accountID,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Transactions) < 2 {
				t.Fatalf("expected at least 2 transactions, got %d", len(out.Transactions))
			}
		})

		t.Run("maps_all_output_fields", func(t *testing.T) {
			ruleID := createTestRule(t, client, accountID)
			recHandler := newRecordTransactionHandler(client)
			_, _, err := recHandler(ctx, nil, RecordTransactionInput{
				AccountID:       accountID,
				Date:            "2025-07-01",
				Amount:          -150000,
				Name:            "Mapped Txn",
				Description:     "Full field test",
				Status:          "RECONCILED",
				RecurringRuleID: ruleID,
			})
			if err != nil {
				t.Fatalf("record txn: %v", err)
			}

			_, out, err := handler(ctx, nil, ListTransactionsInput{
				AccountID: accountID,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			found := false
			for _, txn := range out.Transactions {
				if txn.Name == "Mapped Txn" {
					found = true
					if txn.AccountID != accountID {
						t.Fatalf("expected account_id %q, got %q", accountID, txn.AccountID)
					}
					if txn.Date != "2025-07-01" {
						t.Fatalf("expected date 2025-07-01, got %q", txn.Date)
					}
					if txn.Amount != -150000 {
						t.Fatalf("expected amount -150000, got %d", txn.Amount)
					}
					if txn.Description != "Full field test" {
						t.Fatalf("expected description 'Full field test', got %q", txn.Description)
					}
					if txn.RecurringRuleID != ruleID {
						t.Fatalf("expected recurring_rule_id %q, got %q", ruleID, txn.RecurringRuleID)
					}
					if txn.Status != "TRANSACTION_STATUS_RECONCILED" {
						t.Fatalf("expected status TRANSACTION_STATUS_RECONCILED, got %q", txn.Status)
					}
					if txn.ID == "" {
						t.Fatal("expected non-empty transaction ID")
					}
				}
			}
			if !found {
				t.Fatal("expected to find 'Mapped Txn' in list")
			}
		})
	})

	t.Run("update_transaction_status", func(t *testing.T) {
		handler := newUpdateTransactionStatusHandler(client)

		// Create a transaction to update.
		recHandler := newRecordTransactionHandler(client)
		_, txnOut, err := recHandler(ctx, nil, RecordTransactionInput{
			AccountID: accountID,
			Date:      "2025-04-01",
			Amount:    -200,
			Name:      "Status Test",
			Status:    "PROJECTED",
		})
		if err != nil {
			t.Fatalf("create transaction: %v", err)
		}
		txnID := txnOut.Transaction.ID

		t.Run("updates_status_successfully", func(t *testing.T) {
			_, _, err := handler(ctx, nil, UpdateTransactionStatusInput{
				TransactionID: txnID,
				NewStatus:     "SCHEDULED",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		t.Run("rejects_unknown_status", func(t *testing.T) {
			_, _, err := handler(ctx, nil, UpdateTransactionStatusInput{
				TransactionID: txnID,
				NewStatus:     "INVALID",
			})
			if err == nil {
				t.Fatal("expected error for unknown status")
			}
			if !strings.Contains(err.Error(), "unknown status") {
				t.Fatalf("expected 'unknown status' in error, got: %s", err.Error())
			}
		})
	})

	t.Run("create_transfer", func(t *testing.T) {
		handler := newCreateTransferHandler(client)

		// Create a destination account.
		resp, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
			Name: "Savings",
			Type: finchv1.AccountType_ACCOUNT_TYPE_SAVINGS,
		})
		if err != nil {
			t.Fatalf("create destination account: %v", err)
		}
		destID := resp.Account.Id

		t.Run("creates_transfer_between_accounts", func(t *testing.T) {
			_, _, err := handler(ctx, nil, CreateTransferInput{
				SourceAccountID:      accountID,
				DestinationAccountID: destID,
				Amount:               50000,
				Date:                 "2025-05-01",
				Name:                 "Savings Transfer",
				Status:               "RECONCILED",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		t.Run("accepts_both_status_forms", func(t *testing.T) {
			_, _, err := handler(ctx, nil, CreateTransferInput{
				SourceAccountID:      accountID,
				DestinationAccountID: destID,
				Amount:               10000,
				Date:                 "2025-05-15",
				Name:                 "Prefixed Transfer",
				Status:               "TRANSACTION_STATUS_SCHEDULED",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		t.Run("rejects_unknown_status", func(t *testing.T) {
			_, _, err := handler(ctx, nil, CreateTransferInput{
				SourceAccountID:      accountID,
				DestinationAccountID: destID,
				Amount:               10000,
				Date:                 "2025-05-20",
				Name:                 "Bad Transfer",
				Status:               "INVALID",
			})
			if err == nil {
				t.Fatal("expected error for unknown status")
			}
			if !strings.Contains(err.Error(), "unknown status") {
				t.Fatalf("expected 'unknown status' in error, got: %s", err.Error())
			}
		})
	})
}

// --- Projection Tools ---

func TestProjectionTools(t *testing.T) {
	client := startTestBackend(t)
	ctx := context.Background()
	accountID := createTestAccount(t, client)

	// Seed data: opening balance + recurring rule.
	recHandler := newRecordTransactionHandler(client)
	_, _, err := recHandler(ctx, nil, RecordTransactionInput{
		AccountID: accountID,
		Date:      "2025-01-01",
		Amount:    500000,
		Name:      "Opening Balance",
		Status:    "RECONCILED",
	})
	if err != nil {
		t.Fatalf("seed opening balance: %v", err)
	}

	ruleHandler := newCreateRecurringRuleHandler(client)
	_, _, err = ruleHandler(ctx, nil, CreateRecurringRuleInput{
		AccountID:  accountID,
		Name:       "Rent",
		Amount:     -150000,
		Frequency:  "MONTHLY",
		StartDate:  "2025-01-01",
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("seed recurring rule: %v", err)
	}

	t.Run("project_balances", func(t *testing.T) {
		handler := newProjectBalancesHandler(client)

		t.Run("returns_daily_balances_with_transactions", func(t *testing.T) {
			_, out, err := handler(ctx, nil, ProjectBalancesInput{
				FromDate:   "2025-01-01",
				ToDate:     "2025-03-31",
				AccountIDs: []string{accountID},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Balances) == 0 {
				t.Fatal("expected non-empty balances")
			}
			// First entry should have the opening balance date.
			first := out.Balances[0]
			if first.Date != "2025-01-01" {
				t.Fatalf("expected first date 2025-01-01, got %q", first.Date)
			}
			if first.AccountID != accountID {
				t.Fatalf("expected account_id %q, got %q", accountID, first.AccountID)
			}
		})

		t.Run("maps_projected_transaction_fields_correctly", func(t *testing.T) {
			_, out, err := handler(ctx, nil, ProjectBalancesInput{
				FromDate:   "2025-02-01",
				ToDate:     "2025-02-28",
				AccountIDs: []string{accountID},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Feb should have a projected rent transaction.
			found := false
			for _, b := range out.Balances {
				for _, txn := range b.Transactions {
					if txn.Name == "Rent" {
						found = true
						if txn.Amount != -150000 {
							t.Fatalf("expected amount -150000, got %d", txn.Amount)
						}
						if txn.AccountID != accountID {
							t.Fatalf("expected account_id %q, got %q", accountID, txn.AccountID)
						}
						if !txn.IsProjected {
							t.Fatal("expected is_projected=true for recurring expansion")
						}
					}
				}
			}
			if !found {
				t.Fatal("expected to find projected Rent transaction in Feb")
			}
		})

		t.Run("returns_empty_for_no_activity", func(t *testing.T) {
			freshClient := startTestBackend(t)
			emptyAcctID := createTestAccount(t, freshClient)
			emptyHandler := newProjectBalancesHandler(freshClient)

			_, out, err := emptyHandler(ctx, nil, ProjectBalancesInput{
				FromDate:   "2025-06-01",
				ToDate:     "2025-06-30",
				AccountIDs: []string{emptyAcctID},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Balances) != 0 {
				t.Fatalf("expected 0 balances for empty account, got %d", len(out.Balances))
			}
		})
	})

	// Seed a one-off income transaction in Feb for aggregation tests later.
	_, _, err = recHandler(ctx, nil, RecordTransactionInput{
		AccountID: accountID,
		Date:      "2025-02-15",
		Amount:    200000,
		Name:      "Bonus Income",
		Status:    "RECONCILED",
	})
	if err != nil {
		t.Fatalf("seed bonus income: %v", err)
	}

	t.Run("project_balance_on_date", func(t *testing.T) {
		handler := newProjectBalanceOnDateHandler(client)

		t.Run("returns_correct_balance", func(t *testing.T) {
			_, out, err := handler(ctx, nil, ProjectBalanceOnDateInput{
				Date:      "2025-03-01",
				AccountID: accountID,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Jan 1: opening 500000 + rent -150000 = 350000
			// Feb 1: rent -150000 → 200000
			// Feb 15: bonus +200000 → 400000
			// Mar 1: rent -150000 → 250000
			if out.Balance != 250000 {
				t.Fatalf("expected balance 250000, got %d", out.Balance)
			}
		})

		t.Run("returns_zero_for_empty_account", func(t *testing.T) {
			freshClient := startTestBackend(t)
			emptyAcctID := createTestAccount(t, freshClient)
			emptyHandler := newProjectBalanceOnDateHandler(freshClient)

			_, out, err := emptyHandler(ctx, nil, ProjectBalanceOnDateInput{
				Date:      "2025-06-15",
				AccountID: emptyAcctID,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Balance != 0 {
				t.Fatalf("expected balance 0 for empty account, got %d", out.Balance)
			}
		})
	})
}

// --- Aggregation Tools ---

func TestAggregationTools(t *testing.T) {
	client := startTestBackend(t)
	ctx := context.Background()
	accountID := createTestAccount(t, client)

	// Seed data: opening balance + recurring expense + one-off income.
	recHandler := newRecordTransactionHandler(client)
	_, _, err := recHandler(ctx, nil, RecordTransactionInput{
		AccountID: accountID,
		Date:      "2025-01-01",
		Amount:    500000,
		Name:      "Opening Balance",
		Status:    "RECONCILED",
	})
	if err != nil {
		t.Fatalf("seed opening balance: %v", err)
	}

	ruleHandler := newCreateRecurringRuleHandler(client)
	_, _, err = ruleHandler(ctx, nil, CreateRecurringRuleInput{
		AccountID:  accountID,
		Name:       "Rent",
		Amount:     -150000,
		Frequency:  "MONTHLY",
		StartDate:  "2025-01-01",
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("seed recurring rule: %v", err)
	}

	_, _, err = recHandler(ctx, nil, RecordTransactionInput{
		AccountID: accountID,
		Date:      "2025-02-15",
		Amount:    200000,
		Name:      "Bonus Income",
		Status:    "RECONCILED",
	})
	if err != nil {
		t.Fatalf("seed bonus income: %v", err)
	}

	t.Run("get_monthly_cash_flow", func(t *testing.T) {
		handler := newGetMonthlyCashFlowHandler(client)

		t.Run("returns_monthly_income_and_expenses", func(t *testing.T) {
			_, out, err := handler(ctx, nil, GetMonthlyCashFlowInput{
				FromMonth:  "2025-01",
				ToMonth:    "2025-03",
				AccountIDs: []string{accountID},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Months) != 3 {
				t.Fatalf("expected 3 months, got %d", len(out.Months))
			}

			// Jan: income=500000, expenses=-150000, net=350000
			jan := out.Months[0]
			if jan.Month != "2025-01" {
				t.Fatalf("expected month 2025-01, got %q", jan.Month)
			}
			if jan.Income != 500000 {
				t.Fatalf("jan income: expected 500000, got %d", jan.Income)
			}
			if jan.Expenses != -150000 {
				t.Fatalf("jan expenses: expected -150000, got %d", jan.Expenses)
			}
			if jan.Net != 350000 {
				t.Fatalf("jan net: expected 350000, got %d", jan.Net)
			}

			// Feb: income=200000, expenses=-150000, net=50000
			feb := out.Months[1]
			if feb.Income != 200000 {
				t.Fatalf("feb income: expected 200000, got %d", feb.Income)
			}
			if feb.Expenses != -150000 {
				t.Fatalf("feb expenses: expected -150000, got %d", feb.Expenses)
			}

			// Mar: income=0, expenses=-150000, net=-150000
			mar := out.Months[2]
			if mar.Income != 0 {
				t.Fatalf("mar income: expected 0, got %d", mar.Income)
			}
			if mar.Expenses != -150000 {
				t.Fatalf("mar expenses: expected -150000, got %d", mar.Expenses)
			}
		})

		t.Run("handles_empty_date_range", func(t *testing.T) {
			freshClient := startTestBackend(t)
			emptyAcctID := createTestAccount(t, freshClient)
			emptyHandler := newGetMonthlyCashFlowHandler(freshClient)

			_, out, err := emptyHandler(ctx, nil, GetMonthlyCashFlowInput{
				FromMonth:  "2025-06",
				ToMonth:    "2025-08",
				AccountIDs: []string{emptyAcctID},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Months) != 3 {
				t.Fatalf("expected 3 months (zero-filled), got %d", len(out.Months))
			}
			for _, m := range out.Months {
				if m.Income != 0 || m.Expenses != 0 || m.Net != 0 {
					t.Fatalf("expected zero values for month %s, got income=%d expenses=%d net=%d", m.Month, m.Income, m.Expenses, m.Net)
				}
			}
		})

		t.Run("returns_continuous_months_with_zero_fills", func(t *testing.T) {
			_, out, err := handler(ctx, nil, GetMonthlyCashFlowInput{
				FromMonth:  "2025-01",
				ToMonth:    "2025-06",
				AccountIDs: []string{accountID},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Months) != 6 {
				t.Fatalf("expected 6 continuous months, got %d", len(out.Months))
			}
			expectedMonths := []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06"}
			for i, m := range out.Months {
				if m.Month != expectedMonths[i] {
					t.Fatalf("expected month %s at index %d, got %s", expectedMonths[i], i, m.Month)
				}
			}
		})
	})

	t.Run("get_balance_time_series", func(t *testing.T) {
		handler := newGetBalanceTimeSeriesHandler(client)

		t.Run("returns_daily_balance_points", func(t *testing.T) {
			_, out, err := handler(ctx, nil, GetBalanceTimeSeriesInput{
				FromDate:   "2025-01-01",
				ToDate:     "2025-01-07",
				Interval:   "DAILY",
				AccountIDs: []string{accountID},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Points) != 7 {
				t.Fatalf("expected 7 daily points, got %d", len(out.Points))
			}
			// First point: Jan 1 has opening 500000 + rent -150000 = 350000
			first := out.Points[0]
			if first.Date != "2025-01-01" {
				t.Fatalf("expected first date 2025-01-01, got %q", first.Date)
			}
			if len(first.Balances) != 1 {
				t.Fatalf("expected 1 account balance, got %d", len(first.Balances))
			}
			if first.Balances[0].Balance != 350000 {
				t.Fatalf("expected balance 350000 on Jan 1, got %d", first.Balances[0].Balance)
			}
		})

		t.Run("returns_weekly_sampled_points", func(t *testing.T) {
			_, out, err := handler(ctx, nil, GetBalanceTimeSeriesInput{
				FromDate:   "2025-01-01",
				ToDate:     "2025-01-28",
				Interval:   "WEEKLY",
				AccountIDs: []string{accountID},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// 4 weekly points: Jan 1, Jan 8, Jan 15, Jan 22
			if len(out.Points) != 4 {
				t.Fatalf("expected 4 weekly points, got %d", len(out.Points))
			}
			if out.Points[0].Date != "2025-01-01" {
				t.Fatalf("expected first date 2025-01-01, got %q", out.Points[0].Date)
			}
			if out.Points[1].Date != "2025-01-08" {
				t.Fatalf("expected second date 2025-01-08, got %q", out.Points[1].Date)
			}
		})

		t.Run("returns_monthly_sampled_points", func(t *testing.T) {
			_, out, err := handler(ctx, nil, GetBalanceTimeSeriesInput{
				FromDate:   "2025-01-01",
				ToDate:     "2025-03-01",
				Interval:   "MONTHLY",
				AccountIDs: []string{accountID},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Points) != 3 {
				t.Fatalf("expected 3 monthly points, got %d", len(out.Points))
			}
			// Jan 1: 350000, Feb 1: 200000, Mar 1: 250000
			if out.Points[0].Balances[0].Balance != 350000 {
				t.Fatalf("expected Jan balance 350000, got %d", out.Points[0].Balances[0].Balance)
			}
			if out.Points[1].Balances[0].Balance != 200000 {
				t.Fatalf("expected Feb balance 200000, got %d", out.Points[1].Balances[0].Balance)
			}
			if out.Points[2].Balances[0].Balance != 250000 {
				t.Fatalf("expected Mar balance 250000, got %d", out.Points[2].Balances[0].Balance)
			}
		})

		t.Run("includes_per_account_breakdown", func(t *testing.T) {
			// Create a second account with its own transaction.
			resp, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
				Name: "Savings",
				Type: finchv1.AccountType_ACCOUNT_TYPE_SAVINGS,
			})
			if err != nil {
				t.Fatalf("create savings account: %v", err)
			}
			savingsID := resp.Account.Id

			_, _, err = recHandler(ctx, nil, RecordTransactionInput{
				AccountID: savingsID,
				Date:      "2025-01-01",
				Amount:    1000000,
				Name:      "Savings Opening",
				Status:    "RECONCILED",
			})
			if err != nil {
				t.Fatalf("seed savings: %v", err)
			}

			_, out, err := handler(ctx, nil, GetBalanceTimeSeriesInput{
				FromDate:   "2025-01-01",
				ToDate:     "2025-01-01",
				Interval:   "DAILY",
				AccountIDs: []string{accountID, savingsID},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Points) != 1 {
				t.Fatalf("expected 1 point, got %d", len(out.Points))
			}
			if len(out.Points[0].Balances) != 2 {
				t.Fatalf("expected 2 account balances, got %d", len(out.Points[0].Balances))
			}
		})

		t.Run("normalizes_interval_input", func(t *testing.T) {
			cases := []struct {
				name  string
				input string
			}{
				{"lowercase", "daily"},
				{"prefixed", "TIME_SERIES_INTERVAL_WEEKLY"},
				{"with whitespace", "  MONTHLY  "},
				{"mixed case prefixed", "time_series_interval_daily"},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					_, _, err := handler(ctx, nil, GetBalanceTimeSeriesInput{
						FromDate:   "2025-01-01",
						ToDate:     "2025-01-01",
						Interval:   tc.input,
						AccountIDs: []string{accountID},
					})
					if err != nil {
						t.Fatalf("input %q: unexpected error: %v", tc.input, err)
					}
				})
			}
		})

		t.Run("rejects_unknown_interval", func(t *testing.T) {
			_, _, err := handler(ctx, nil, GetBalanceTimeSeriesInput{
				FromDate:   "2025-01-01",
				ToDate:     "2025-01-31",
				Interval:   "QUARTERLY",
				AccountIDs: []string{accountID},
			})
			if err == nil {
				t.Fatal("expected error for unknown interval")
			}
			errMsg := err.Error()
			if !strings.Contains(errMsg, "unknown interval") {
				t.Fatalf("expected 'unknown interval' in error, got: %s", errMsg)
			}
			if !strings.Contains(errMsg, "DAILY") {
				t.Fatalf("expected valid options in error, got: %s", errMsg)
			}
		})
	})
}
