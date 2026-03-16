package core_test

import (
	"context"
	"encoding/json"
	"errors"
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
	if acct.ID == "" {
		t.Fatal("expected non-empty ID")
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

func TestCreateAccountStoresEvent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	acct, err := db.CreateAccount(ctx, "Test Account", core.AccountTypeChecking)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	events, err := db.GetAccountHistory(ctx, acct.ID)
	if err != nil {
		t.Fatalf("GetAccountHistory: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "AccountCreated" {
		t.Fatalf("expected AccountCreated, got %s", events[0].EventType)
	}

	var payload core.AccountCreatedPayload
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Name != "Test Account" {
		t.Fatalf("expected name Test Account in payload, got %s", payload.Name)
	}
	if payload.Type != int(core.AccountTypeChecking) {
		t.Fatalf("expected type %d in payload, got %d", core.AccountTypeChecking, payload.Type)
	}
}

func TestRenameAccount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	acct, err := db.CreateAccount(ctx, "Old Name", core.AccountTypeChecking)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	if err := db.RenameAccount(ctx, acct.ID, "New Name"); err != nil {
		t.Fatalf("RenameAccount: %v", err)
	}

	accounts, err := db.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Name != "New Name" {
		t.Fatalf("expected name 'New Name', got %q", accounts[0].Name)
	}

	events, err := db.GetAccountHistory(ctx, acct.ID)
	if err != nil {
		t.Fatalf("GetAccountHistory: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].EventType != "AccountRenamed" {
		t.Fatalf("expected AccountRenamed, got %s", events[1].EventType)
	}

	var payload core.AccountRenamedPayload
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.OldName != "Old Name" || payload.NewName != "New Name" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestRenameAccountValidation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	acct, err := db.CreateAccount(ctx, "Test", core.AccountTypeChecking)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	if err := db.RenameAccount(ctx, "", "New"); err == nil {
		t.Fatal("expected error for empty ID")
	}
	if err := db.RenameAccount(ctx, acct.ID, ""); err == nil {
		t.Fatal("expected error for empty name")
	}
	if err := db.RenameAccount(ctx, acct.ID, "   "); err == nil {
		t.Fatal("expected error for whitespace-only name")
	}
	if err := db.RenameAccount(ctx, "nonexistent-id", "New"); err == nil {
		t.Fatal("expected error for nonexistent account")
	}
	if err := db.RenameAccount(ctx, acct.ID, "Test"); err == nil {
		t.Fatal("expected error for same name")
	} else if !errors.Is(err, core.ErrNoChange) {
		t.Fatalf("expected ErrNoChange, got: %v", err)
	}
}

func TestUpdateAccountType(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	acct, err := db.CreateAccount(ctx, "Test", core.AccountTypeChecking)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	if err := db.UpdateAccountType(ctx, acct.ID, core.AccountTypeSavings); err != nil {
		t.Fatalf("UpdateAccountType: %v", err)
	}

	accounts, err := db.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Type != core.AccountTypeSavings {
		t.Fatalf("expected type Savings, got %d", accounts[0].Type)
	}

	events, err := db.GetAccountHistory(ctx, acct.ID)
	if err != nil {
		t.Fatalf("GetAccountHistory: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].EventType != "AccountTypeChanged" {
		t.Fatalf("expected AccountTypeChanged, got %s", events[1].EventType)
	}

	var payload core.AccountTypeChangedPayload
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.OldType != int(core.AccountTypeChecking) || payload.NewType != int(core.AccountTypeSavings) {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestUpdateAccountTypeValidation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	acct, err := db.CreateAccount(ctx, "Test", core.AccountTypeChecking)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	if err := db.UpdateAccountType(ctx, "", core.AccountTypeSavings); err == nil {
		t.Fatal("expected error for empty ID")
	}
	if err := db.UpdateAccountType(ctx, acct.ID, core.AccountTypeUnspecified); err == nil {
		t.Fatal("expected error for unspecified type")
	}
	if err := db.UpdateAccountType(ctx, acct.ID, core.AccountType(99)); err == nil {
		t.Fatal("expected error for invalid type")
	}
	if err := db.UpdateAccountType(ctx, "nonexistent-id", core.AccountTypeSavings); err == nil {
		t.Fatal("expected error for nonexistent account")
	}
	if err := db.UpdateAccountType(ctx, acct.ID, core.AccountTypeChecking); err == nil {
		t.Fatal("expected error for same type")
	} else if !errors.Is(err, core.ErrNoChange) {
		t.Fatalf("expected ErrNoChange, got: %v", err)
	}
}

func TestGetAccountHistoryEmpty(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	events, err := db.GetAccountHistory(ctx, "nonexistent-id")
	if err != nil {
		t.Fatalf("GetAccountHistory: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}
