package main

import (
	"context"
	"math"
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

// createRulesAccount is a small helper for the recurring-rule error tests below, and the
// base account of createTransferPair.
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
		{
			name: "whitespace account_id",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: "   ", Name: "Rent", Amount: -150000,
				Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
				DayOfMonth: 1,
			},
		},
		{
			name: "out-of-range frequency value",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "Rent", Amount: -150000,
				Frequency: finchv1.Frequency(99), StartDate: "2025-01-01",
			},
		},
		{
			name: "transfer without a target account",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "To Savings", Amount: 100000,
				Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
				DayOfMonth: 1, IsTransfer: true,
			},
		},
		{
			name: "transfer targeting its own account",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "To Savings", Amount: 100000,
				Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
				DayOfMonth: 1, IsTransfer: true, TransferTargetAccountId: accountID,
			},
		},
		{
			name: "transfer with a negative amount",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "To Savings", Amount: -100000,
				Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
				DayOfMonth: 1, IsTransfer: true, TransferTargetAccountId: "some-other-account",
			},
		},
		{
			name: "zero amount on a non-transfer rule",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "Rent", Amount: 0,
				Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
				DayOfMonth: 1,
			},
		},
		{
			name: "amount one past the cap",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "Rent", Amount: core.MaxAmountCents + 1,
				Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
				DayOfMonth: 1,
			},
		},
		{
			name: "amount one past the negative cap",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "Rent", Amount: -core.MaxAmountCents - 1,
				Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
				DayOfMonth: 1,
			},
		},
		{
			name: "math.MaxInt64 amount",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "Rent", Amount: math.MaxInt64,
				Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
				DayOfMonth: 1,
			},
		},
		{
			name: "math.MinInt64 amount",
			req: &finchv1.CreateRecurringRuleRequest{
				AccountId: accountID, Name: "Rent", Amount: math.MinInt64,
				Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
				DayOfMonth: 1,
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

// A transfer rule naming an account that does not exist is the case the boundary
// cannot check syntactically: confirming the target requires a database read, which
// belongs in core. It must still reach the caller as a typed status rather than
// falling through to Internal.
func TestCreateRecurringRuleUnknownTransferTargetIsNotInternal(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	accountID := createRulesAccount(t, client, ctx)

	_, err := client.CreateRecurringRule(ctx, &finchv1.CreateRecurringRuleRequest{
		AccountId: accountID, Name: "To Savings", Amount: 100000,
		Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
		DayOfMonth: 1, IsTransfer: true,
		TransferTargetAccountId: "00000000-0000-0000-0000-000000000000",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err: %v)", status.Code(err), err)
	}
}

// Editing a transfer rule's amount is the second write path, and it must be gated at
// the boundary exactly as creation is — otherwise the invariant holds only until
// someone edits the rule.
func TestUpdateRecurringRuleAmountTransferRejectsNonPositive(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	sourceID := createRulesAccount(t, client, ctx)

	target, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
		Name: "Savings", Type: finchv1.AccountType_ACCOUNT_TYPE_SAVINGS,
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	rule, err := client.CreateRecurringRule(ctx, &finchv1.CreateRecurringRuleRequest{
		AccountId: sourceID, Name: "To Savings", Amount: 100000,
		Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
		DayOfMonth: 1, IsTransfer: true, TransferTargetAccountId: target.Account.Id,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	// The zero row is now answered by the universal magnitude bound before the transfer
	// rule sees it, so -100000 is the row still proving the positivity check itself.
	for _, amount := range []int64{0, -100000} {
		_, err := client.UpdateRecurringRuleAmount(ctx, &finchv1.UpdateRecurringRuleAmountRequest{
			RuleId: rule.Rule.Id, NewAmount: amount, EffectiveDate: "2025-02-01",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("amount %d: expected InvalidArgument, got %v (err: %v)", amount, status.Code(err), err)
		}
	}
}

// A valid transfer rule must still be accepted — the counterpart to the rejection
// cases above, guarding against over-gating the new transfer validation.
func TestCreateRecurringRuleTransferValid(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	sourceID := createRulesAccount(t, client, ctx)

	target, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
		Name: "Savings", Type: finchv1.AccountType_ACCOUNT_TYPE_SAVINGS,
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	rule, err := client.CreateRecurringRule(ctx, &finchv1.CreateRecurringRuleRequest{
		AccountId: sourceID, Name: "To Savings", Amount: 100000,
		Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
		DayOfMonth: 1, IsTransfer: true, TransferTargetAccountId: target.Account.Id,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}
	if !rule.Rule.IsTransfer || rule.Rule.TransferTargetAccountId != target.Account.Id {
		t.Fatalf("transfer fields not round-tripped: %+v", rule.Rule)
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

// createTransferPair returns the two distinct accounts a transfer needs. (Nothing
// currently rejects a transfer whose source and destination are the same account —
// the recurring-rule path guards that, this one does not.)
func createTransferPair(t *testing.T, client finchv1.FinchServiceClient, ctx context.Context) (string, string) {
	t.Helper()
	source := createRulesAccount(t, client, ctx)
	dest, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
		Name: "Savings", Type: finchv1.AccountType_ACCOUNT_TYPE_SAVINGS,
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	return source, dest.Account.Id
}

// recordTransaction had no daemon validation test at all, so this covers the whole RPC
// rather than only the amount clause it was added for. Every case is the caller's fault
// and must surface as InvalidArgument, never Internal (see .claude/rules/api-design.md).
func TestRecordTransactionValidationReturnsInvalidArgument(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	accountID := createRulesAccount(t, client, ctx)

	valid := func() *finchv1.RecordTransactionRequest {
		return &finchv1.RecordTransactionRequest{
			AccountId: accountID, Date: "2025-01-15", Amount: -5000, Name: "Coffee",
			Status: finchv1.TransactionStatus_TRANSACTION_STATUS_RECONCILED,
		}
	}

	cases := []struct {
		name  string
		spoil func(r *finchv1.RecordTransactionRequest)
	}{
		{"empty name", func(r *finchv1.RecordTransactionRequest) { r.Name = "" }},
		{"whitespace name", func(r *finchv1.RecordTransactionRequest) { r.Name = "   " }},
		{"empty account_id", func(r *finchv1.RecordTransactionRequest) { r.AccountId = "" }},
		{"whitespace account_id", func(r *finchv1.RecordTransactionRequest) { r.AccountId = "   " }},
		{"unspecified status", func(r *finchv1.RecordTransactionRequest) {
			r.Status = finchv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
		}},
		{"malformed date", func(r *finchv1.RecordTransactionRequest) { r.Date = "15-01-2025" }},
		{"zero amount", func(r *finchv1.RecordTransactionRequest) { r.Amount = 0 }},
		{"amount one past the cap", func(r *finchv1.RecordTransactionRequest) { r.Amount = core.MaxAmountCents + 1 }},
		{"amount one past the negative cap", func(r *finchv1.RecordTransactionRequest) { r.Amount = -core.MaxAmountCents - 1 }},
		{"math.MaxInt64 amount", func(r *finchv1.RecordTransactionRequest) { r.Amount = math.MaxInt64 }},
		{"math.MinInt64 amount", func(r *finchv1.RecordTransactionRequest) { r.Amount = math.MinInt64 }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid()
			tc.spoil(req)
			_, err := client.RecordTransaction(ctx, req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v (err: %v)", status.Code(err), err)
			}
		})
	}
}

// The counterpart to the rejections above: the cap itself is a storable amount, and an
// ordinary signed amount is untouched. Without this, over-gating would pass unnoticed.
func TestRecordTransactionAcceptsAmountsAtTheCap(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	accountID := createRulesAccount(t, client, ctx)

	for _, amount := range []int64{core.MaxAmountCents, -core.MaxAmountCents, -5000} {
		resp, err := client.RecordTransaction(ctx, &finchv1.RecordTransactionRequest{
			AccountId: accountID, Date: "2025-01-15", Amount: amount, Name: "Coffee",
			Status: finchv1.TransactionStatus_TRANSACTION_STATUS_RECONCILED,
		})
		if err != nil {
			t.Fatalf("amount %d: RecordTransaction: %v", amount, err)
		}
		if resp.Transaction.Amount != amount {
			t.Errorf("amount %d: stored as %d", amount, resp.Transaction.Amount)
		}
	}
}

// The rule paths carry the same bound. A non-transfer rule is the case with no prior
// guard at all — the existing positive-magnitude check applies only to transfers.
func TestCreateRecurringRuleAcceptsAmountsAtTheCap(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	accountID := createRulesAccount(t, client, ctx)

	for _, amount := range []int64{core.MaxAmountCents, -core.MaxAmountCents} {
		resp, err := client.CreateRecurringRule(ctx, &finchv1.CreateRecurringRuleRequest{
			AccountId: accountID, Name: "Rent", Amount: amount,
			Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
			DayOfMonth: 1,
		})
		if err != nil {
			t.Fatalf("amount %d: CreateRecurringRule: %v", amount, err)
		}
		if resp.Rule.Amount != amount {
			t.Errorf("amount %d: stored as %d", amount, resp.Rule.Amount)
		}
	}
}

// The base rule is deliberately NOT a transfer: TestUpdateRecurringRuleAmountTransferRejectsNonPositive
// already covers transfers, and a transfer rule here would let the pre-existing positive-magnitude
// check satisfy every assertion without the magnitude bound existing at all.
func TestUpdateRecurringRuleAmountRejectsOutOfRangeAmount(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	accountID := createRulesAccount(t, client, ctx)

	rule, err := client.CreateRecurringRule(ctx, &finchv1.CreateRecurringRuleRequest{
		AccountId: accountID, Name: "Rent", Amount: -150000,
		Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	for _, amount := range []int64{0, core.MaxAmountCents + 1, -core.MaxAmountCents - 1, math.MaxInt64, math.MinInt64} {
		_, err := client.UpdateRecurringRuleAmount(ctx, &finchv1.UpdateRecurringRuleAmountRequest{
			RuleId: rule.Rule.Id, NewAmount: amount, EffectiveDate: "2025-02-01",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("amount %d: expected InvalidArgument, got %v (err: %v)", amount, status.Code(err), err)
		}
	}
}

// Over-gating counterpart for the edit path: a non-transfer rule keeps its free sign at
// the cap in both directions.
func TestUpdateRecurringRuleAmountAcceptsAmountsAtTheCap(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	accountID := createRulesAccount(t, client, ctx)

	rule, err := client.CreateRecurringRule(ctx, &finchv1.CreateRecurringRuleRequest{
		AccountId: accountID, Name: "Rent", Amount: -150000,
		Frequency: finchv1.Frequency_FREQUENCY_MONTHLY, StartDate: "2025-01-01",
		DayOfMonth: 1,
	})
	if err != nil {
		t.Fatalf("CreateRecurringRule: %v", err)
	}

	for _, amount := range []int64{core.MaxAmountCents, -core.MaxAmountCents} {
		if _, err := client.UpdateRecurringRuleAmount(ctx, &finchv1.UpdateRecurringRuleAmountRequest{
			RuleId: rule.Rule.Id, NewAmount: amount, EffectiveDate: "2025-02-01",
		}); err != nil {
			t.Errorf("amount %d: UpdateRecurringRuleAmount: %v", amount, err)
		}
	}
}

// A transfer stacks two rules: the universal magnitude bound and its own positive-magnitude
// requirement. The blank-name case is here because core rejects it while this handler did
// not, so it reached the caller as Internal rather than InvalidArgument.
func TestCreateTransferValidationReturnsInvalidArgument(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	source, dest := createTransferPair(t, client, ctx)

	valid := func() *finchv1.CreateTransferRequest {
		return &finchv1.CreateTransferRequest{
			SourceAccountId: source, DestinationAccountId: dest, Amount: 100000,
			Date: "2025-01-15", Name: "To Savings",
			Status: finchv1.TransactionStatus_TRANSACTION_STATUS_RECONCILED,
		}
	}

	cases := []struct {
		name  string
		spoil func(r *finchv1.CreateTransferRequest)
	}{
		{"zero amount", func(r *finchv1.CreateTransferRequest) { r.Amount = 0 }},
		{"negative amount", func(r *finchv1.CreateTransferRequest) { r.Amount = -100000 }},
		{"amount one past the cap", func(r *finchv1.CreateTransferRequest) { r.Amount = core.MaxAmountCents + 1 }},
		{"math.MaxInt64 amount", func(r *finchv1.CreateTransferRequest) { r.Amount = math.MaxInt64 }},
		{"empty name", func(r *finchv1.CreateTransferRequest) { r.Name = "" }},
		{"whitespace name", func(r *finchv1.CreateTransferRequest) { r.Name = "   " }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid()
			tc.spoil(req)
			_, err := client.CreateTransfer(ctx, req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v (err: %v)", status.Code(err), err)
			}
		})
	}
}

// Both accounts must exist for the write to land — foreign keys are on. The amount check
// itself runs before any database access, so this proves the cap is storable, not merely
// that it passes validation.
func TestCreateTransferAcceptsAmountAtTheCap(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()
	source, dest := createTransferPair(t, client, ctx)

	if _, err := client.CreateTransfer(ctx, &finchv1.CreateTransferRequest{
		SourceAccountId: source, DestinationAccountId: dest, Amount: core.MaxAmountCents,
		Date: "2025-01-15", Name: "To Savings",
		Status: finchv1.TransactionStatus_TRANSACTION_STATUS_RECONCILED,
	}); err != nil {
		t.Fatalf("CreateTransfer at the cap: %v", err)
	}
}

// A missing transaction is a NotFound, not a daemon failure — the same distinction the
// recurring-rule operations already draw. Core names the condition; the daemon must carry it.
func TestUpdateTransactionStatusUnknownTransactionReturnsNotFound(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	_, err := client.UpdateTransactionStatus(ctx, &finchv1.UpdateTransactionStatusRequest{
		TransactionId: "00000000-0000-0000-0000-000000000000",
		NewStatus:     finchv1.TransactionStatus_TRANSACTION_STATUS_RECONCILED,
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err: %v)", status.Code(err), err)
	}
}
