package core_test

import (
	"context"
	"testing"

	"github.com/jakewan/finch/core"
)

func TestRecordAndListTransactions(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	txn, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: acct.ID,
		Date:      date(2025, 1, 15),
		Amount:    -5000,
		Name:      "Grocery Store",
		Status:    core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}
	if txn.ID == "" {
		t.Fatal("expected non-empty transaction ID")
	}
	if txn.Amount != -5000 {
		t.Fatalf("expected amount -5000, got %d", txn.Amount)
	}

	txns, err := db.ListTransactions(ctx, acct.ID)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(txns) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txns))
	}
}

func TestRecordTransactionValidation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	tests := []struct {
		name   string
		params core.RecordTransactionParams
	}{
		{"empty name", core.RecordTransactionParams{AccountID: acct.ID, Date: date(2025, 1, 1), Amount: 100}},
		{"empty account", core.RecordTransactionParams{Name: "Test", Date: date(2025, 1, 1), Amount: 100}},
		{"invalid status", core.RecordTransactionParams{AccountID: acct.ID, Name: "Test", Date: date(2025, 1, 1), Amount: 100, Status: 99}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.RecordTransaction(ctx, tc.params); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestUpdateTransactionStatus(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	txn, err := db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID: acct.ID,
		Date:      date(2025, 2, 1),
		Amount:    -150000,
		Name:      "Rent",
		Status:    core.TransactionStatusProjected,
	})
	if err != nil {
		t.Fatalf("RecordTransaction: %v", err)
	}

	if err := db.UpdateTransactionStatus(ctx, txn.ID, core.TransactionStatusScheduled); err != nil {
		t.Fatalf("UpdateTransactionStatus: %v", err)
	}

	txns, err := db.ListTransactions(ctx, acct.ID)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if txns[0].Status != core.TransactionStatusScheduled {
		t.Fatalf("expected scheduled status, got %d", txns[0].Status)
	}
}

func TestCreateTransfer(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	checking := createTestAccount(t, db)
	savings, err := db.CreateAccount(ctx, "Savings", core.AccountTypeSavings)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	err = db.CreateTransfer(ctx, core.CreateTransferParams{
		SourceAccountID:      checking.ID,
		DestinationAccountID: savings.ID,
		Amount:               25000, // $250
		Date:                 date(2025, 1, 15),
		Name:                 "Savings Transfer",
		Status:               core.TransactionStatusReconciled,
	})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	// Source account should have negative transaction.
	srcTxns, err := db.ListTransactions(ctx, checking.ID)
	if err != nil {
		t.Fatalf("ListTransactions source: %v", err)
	}
	if len(srcTxns) != 1 {
		t.Fatalf("expected 1 source transaction, got %d", len(srcTxns))
	}
	if srcTxns[0].Amount != -25000 {
		t.Fatalf("expected source amount -25000, got %d", srcTxns[0].Amount)
	}

	// Destination account should have positive transaction.
	destTxns, err := db.ListTransactions(ctx, savings.ID)
	if err != nil {
		t.Fatalf("ListTransactions dest: %v", err)
	}
	if len(destTxns) != 1 {
		t.Fatalf("expected 1 dest transaction, got %d", len(destTxns))
	}
	if destTxns[0].Amount != 25000 {
		t.Fatalf("expected dest amount 25000, got %d", destTxns[0].Amount)
	}
}

func TestCreateTransferValidation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	acct := createTestAccount(t, db)

	tests := []struct {
		name   string
		params core.CreateTransferParams
	}{
		{"missing source", core.CreateTransferParams{DestinationAccountID: acct.ID, Amount: 100, Date: date(2025, 1, 1), Name: "Test"}},
		{"missing dest", core.CreateTransferParams{SourceAccountID: acct.ID, Amount: 100, Date: date(2025, 1, 1), Name: "Test"}},
		{"zero amount", core.CreateTransferParams{SourceAccountID: acct.ID, DestinationAccountID: "other", Amount: 0, Date: date(2025, 1, 1), Name: "Test"}},
		{"negative amount", core.CreateTransferParams{SourceAccountID: acct.ID, DestinationAccountID: "other", Amount: -100, Date: date(2025, 1, 1), Name: "Test"}},
		{"empty name", core.CreateTransferParams{SourceAccountID: acct.ID, DestinationAccountID: "other", Amount: 100, Date: date(2025, 1, 1)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.CreateTransfer(ctx, tc.params); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
