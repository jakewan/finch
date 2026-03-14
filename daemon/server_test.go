package main

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"github.com/jakewan/finch/core"
	finchv1 "github.com/jakewan/finch/daemon/gen/finch/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	finchv1.RegisterFinchServiceServer(srv, &finchServer{db: db})
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
	if resp.Version != version {
		t.Fatalf("expected version %s, got %s", version, resp.Version)
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
