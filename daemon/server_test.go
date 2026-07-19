package main

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"github.com/jakewan/finch/core"
	"github.com/jakewan/finch/daemon/finchd"
	finchv1 "github.com/jakewan/finch/daemon/gen/finch/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func startTestServer(t *testing.T) finchv1.FinchServiceClient {
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

func TestPing(t *testing.T) {
	client := startTestServer(t)
	resp, err := client.Ping(context.Background(), &finchv1.PingRequest{})
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if resp.Version != finchd.Version {
		t.Fatalf("expected version %s, got %s", finchd.Version, resp.Version)
	}
}

func TestCreateAndListAccountsViaGRPC(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	createResp, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
		Name: "Test Checking",
		Type: finchv1.AccountType_ACCOUNT_TYPE_CHECKING,
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if createResp.Account.Name != "Test Checking" {
		t.Fatalf("expected name Test Checking, got %s", createResp.Account.Name)
	}
	if createResp.Account.Id == "" {
		t.Fatal("expected non-empty account ID")
	}

	listResp, err := client.ListAccounts(ctx, &finchv1.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(listResp.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(listResp.Accounts))
	}
}

func TestGetAccountHistoryViaGRPC(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	createResp, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
		Name: "History Test",
		Type: finchv1.AccountType_ACCOUNT_TYPE_SAVINGS,
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	histResp, err := client.GetAccountHistory(ctx, &finchv1.GetAccountHistoryRequest{
		AccountId: createResp.Account.Id,
	})
	if err != nil {
		t.Fatalf("GetAccountHistory: %v", err)
	}
	if len(histResp.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(histResp.Events))
	}
	if histResp.Events[0].EventType != "AccountCreated" {
		t.Fatalf("expected AccountCreated, got %s", histResp.Events[0].EventType)
	}
}

func TestListTransactionsViaGRPC(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	acctResp, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
		Name: "Txn List Test",
		Type: finchv1.AccountType_ACCOUNT_TYPE_CHECKING,
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	accountID := acctResp.Account.Id

	// Record two transactions.
	_, err = client.RecordTransaction(ctx, &finchv1.RecordTransactionRequest{
		AccountId: accountID,
		Date:      "2025-01-10",
		Amount:    -5000,
		Name:      "Coffee",
		Status:    finchv1.TransactionStatus_TRANSACTION_STATUS_RECONCILED,
	})
	if err != nil {
		t.Fatalf("RecordTransaction 1: %v", err)
	}
	_, err = client.RecordTransaction(ctx, &finchv1.RecordTransactionRequest{
		AccountId: accountID,
		Date:      "2025-01-15",
		Amount:    -12000,
		Name:      "Groceries",
		Status:    finchv1.TransactionStatus_TRANSACTION_STATUS_SCHEDULED,
	})
	if err != nil {
		t.Fatalf("RecordTransaction 2: %v", err)
	}

	listResp, err := client.ListTransactions(ctx, &finchv1.ListTransactionsRequest{
		AccountId: accountID,
	})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(listResp.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(listResp.Transactions))
	}

	// Verify field mapping on first transaction.
	found := false
	for _, txn := range listResp.Transactions {
		if txn.Name == "Coffee" {
			found = true
			if txn.Amount != -5000 {
				t.Fatalf("expected amount -5000, got %d", txn.Amount)
			}
			if txn.AccountId != accountID {
				t.Fatalf("expected account_id %q, got %q", accountID, txn.AccountId)
			}
			if txn.Date != "2025-01-10" {
				t.Fatalf("expected date 2025-01-10, got %q", txn.Date)
			}
		}
	}
	if !found {
		t.Fatal("expected to find Coffee transaction")
	}

	// Empty account_id returns error.
	_, err = client.ListTransactions(ctx, &finchv1.ListTransactionsRequest{
		AccountId: "",
	})
	if err == nil {
		t.Fatal("expected error for empty account_id")
	}
}

func TestProjectBalancesViaGRPC(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	// Create account and opening balance.
	acctResp, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
		Name: "Projection Test",
		Type: finchv1.AccountType_ACCOUNT_TYPE_CHECKING,
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	_, err = client.RecordTransaction(ctx, &finchv1.RecordTransactionRequest{
		AccountId: acctResp.Account.Id,
		Date:      "2025-01-01",
		Amount:    500000,
		Name:      "Opening Balance",
		Status:    finchv1.TransactionStatus_TRANSACTION_STATUS_RECONCILED,
	})
	if err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}

	_, err = client.CreateRecurringRule(ctx, &finchv1.CreateRecurringRuleRequest{
		AccountId:  acctResp.Account.Id,
		Name:       "Rent",
		Amount:     -150000,
		Frequency:  finchv1.Frequency_FREQUENCY_MONTHLY,
		StartDate:  "2025-01-01",
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	projResp, err := client.ProjectBalances(ctx, &finchv1.ProjectBalancesRequest{
		FromDate:   "2025-01-01",
		ToDate:     "2025-03-31",
		AccountIds: []string{acctResp.Account.Id},
	})
	if err != nil {
		t.Fatalf("ProjectBalances: %v", err)
	}
	if len(projResp.Balances) == 0 {
		t.Fatal("expected non-empty balances")
	}
}

// createRulesAccount is a small helper for the recurring-rule error tests below.
func createRulesAccount(t *testing.T, client finchv1.FinchServiceClient, ctx context.Context) string {
	t.Helper()
	acct, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
		Name: "Rules Acct",
		Type: finchv1.AccountType_ACCOUNT_TYPE_CHECKING,
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	return acct.Account.Id
}

// A malformed recurring-rule create is the caller's fault, so it must surface as
// InvalidArgument (not Internal) — callers depend on the code to tell a bad request
// from a daemon failure (see .claude/rules/api-design.md).
func TestCreateRecurringRuleValidationReturnsInvalidArgument(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	accountID := createRulesAccount(t, client, ctx)

	cases := []struct {
		name string
		req  *finchv1.CreateRecurringRuleRequest
	}{
		{
			name: "monthly day_of_month out of range",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "Rent", Amount: -150000,
				Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
				DayOfMonth: 40,
			},
		},
		{
			name: "semi-monthly day out of range",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "Paycheck", Amount: 200000,
				Frequency: finchv1.Frequency_FREQUENCY_SEMI_MONTHLY, StartDate: "2025-01-01",
				SemiMonthlyDays: []int32{15, 40},
			},
		},
		{
			name: "semi-monthly wrong day count",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "Paycheck", Amount: 200000,
				Frequency: finchv1.Frequency_FREQUENCY_SEMI_MONTHLY, StartDate: "2025-01-01",
				SemiMonthlyDays: []int32{15},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.CreateRecurringRule(ctx, tc.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v (err: %v)", status.Code(err), err)
			}
		})
	}
}

// A valid semi-monthly rule must still be accepted — guards against over-gating
// the count/range validation added above.
func TestCreateRecurringRuleSemiMonthlyValid(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	accountID := createRulesAccount(t, client, ctx)

	resp, err := client.CreateRecurringRule(ctx, &finchv1.CreateRecurringRuleRequest{
		AccountId: accountID, Name: "Paycheck", Amount: 200000,
		Frequency: finchv1.Frequency_FREQUENCY_SEMI_MONTHLY, StartDate: "2025-01-01",
		SemiMonthlyDays: []int32{1, 15},
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}
	if resp.Rule.Id == "" {
		t.Fatal("expected non-empty rule ID")
	}
}

// Updating a rule that does not exist is a NotFound, not an Internal error — the
// other rule mutations (pause/resume/end) already map this correctly.
func TestUpdateRecurringRuleAmountNotFound(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	_, err := client.UpdateRecurringRuleAmount(ctx, &finchv1.UpdateRecurringRuleAmountRequest{
		RuleId:        "does-not-exist",
		NewAmount:     -1000,
		EffectiveDate: "2025-02-01",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound for missing rule, got %v (err: %v)", status.Code(err), err)
	}
}

func TestProjectBalanceOnDateViaGRPC(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	acctResp, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
		Name: "Point Query Test",
		Type: finchv1.AccountType_ACCOUNT_TYPE_CHECKING,
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	_, err = client.RecordTransaction(ctx, &finchv1.RecordTransactionRequest{
		AccountId: acctResp.Account.Id,
		Date:      "2025-01-01",
		Amount:    1000000,
		Name:      "Opening",
		Status:    finchv1.TransactionStatus_TRANSACTION_STATUS_RECONCILED,
	})
	if err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}

	resp, err := client.ProjectBalanceOnDate(ctx, &finchv1.ProjectBalanceOnDateRequest{
		Date:      "2025-01-15",
		AccountId: acctResp.Account.Id,
	})
	if err != nil {
		t.Fatalf("ProjectBalanceOnDate: %v", err)
	}
	if resp.Balance != 1000000 {
		t.Fatalf("expected balance 1000000, got %d", resp.Balance)
	}
}
