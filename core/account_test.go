package core_test

import (
	"context"
	"testing"

	"github.com/jakewan/finch/core"
)

func TestCreateAndListAccounts(t *testing.T) {
	db := openTestDB(t)

	ctx := context.Background()

	acct, err := db.CreateAccount(ctx, "Checking", core.AccountTypeChecking)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if acct.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if acct.Name != "Checking" {
		t.Fatalf("expected name Checking, got %s", acct.Name)
	}
	if acct.Type != core.AccountTypeChecking {
		t.Fatalf("expected type Checking, got %d", acct.Type)
	}

	_, err = db.CreateAccount(ctx, "Savings", core.AccountTypeSavings)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	accounts, err := db.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}

	// ListAccounts orders by name, so Checking comes first.
	if accounts[0].Name != "Checking" {
		t.Fatalf("expected first account Checking, got %s", accounts[0].Name)
	}
	if accounts[1].Name != "Savings" {
		t.Fatalf("expected second account Savings, got %s", accounts[1].Name)
	}
}

func TestCreateAccountValidation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.CreateAccount(ctx, "", core.AccountTypeChecking); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := db.CreateAccount(ctx, "   ", core.AccountTypeChecking); err == nil {
		t.Fatal("expected error for whitespace-only name")
	}
	if _, err := db.CreateAccount(ctx, "Test", core.AccountTypeUnspecified); err == nil {
		t.Fatal("expected error for unspecified account type")
	}
	if _, err := db.CreateAccount(ctx, "Test", core.AccountType(99)); err == nil {
		t.Fatal("expected error for invalid account type")
	}
}
