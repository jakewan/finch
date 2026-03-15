package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AccountType int

const (
	AccountTypeUnspecified  AccountType = 0
	AccountTypeChecking     AccountType = 1
	AccountTypeSavings      AccountType = 2
	AccountTypeCreditCard   AccountType = 3
	AccountTypeAutoLoan     AccountType = 4
	AccountTypePersonalLoan AccountType = 5
	AccountTypeLineOfCredit AccountType = 6
	AccountTypeBrokerage    AccountType = 7
)

const aggregateTypeAccount = "account"

// Account event types.
const (
	EventAccountCreated     = "AccountCreated"
	EventAccountRenamed     = "AccountRenamed"
	EventAccountTypeChanged = "AccountTypeChanged"
)

type Account struct {
	ID        string
	Name      string
	Type      AccountType
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AccountCreatedPayload is the event payload for account creation.
type AccountCreatedPayload struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

// AccountRenamedPayload is the event payload for renaming an account.
type AccountRenamedPayload struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

// AccountTypeChangedPayload is the event payload for changing an account's type.
type AccountTypeChangedPayload struct {
	OldType int `json:"old_type"`
	NewType int `json:"new_type"`
}

func ValidAccountType(t AccountType) bool {
	return t >= AccountTypeChecking && t <= AccountTypeBrokerage
}

// CreateAccount validates input, appends an AccountCreated event, and updates
// the read model — all within a single transaction.
func (db *DB) CreateAccount(ctx context.Context, name string, accountType AccountType) (*Account, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("account name must not be empty")
	}
	if !ValidAccountType(accountType) {
		return nil, fmt.Errorf("invalid account type: %d", accountType)
	}

	id := uuid.New().String()
	payload, err := json.Marshal(AccountCreatedPayload{
		Name: name,
		Type: int(accountType),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}

	events, err := appendEvents(ctx, tx, aggregateTypeAccount, id, []NewEvent{
		{EventType: EventAccountCreated, Payload: payload},
	})
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("append event: %w", err)
	}

	now := events[0].RecordedAt
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO accounts (id, name, type, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		id, name, int(accountType), now.Unix(), now.Unix(),
	); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("insert account read model: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Account{
		ID:        id,
		Name:      name,
		Type:      accountType,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ListAccounts returns all accounts from the read model, ordered by name.
func (db *DB) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := db.conn.QueryContext(ctx, "SELECT id, name, type, created_at, updated_at FROM accounts ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("query accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAccounts(rows)
}

// RenameAccount changes the name of an existing account.
func (db *DB) RenameAccount(ctx context.Context, accountID, newName string) error {
	if accountID == "" {
		return errors.New("account_id must not be empty")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("account name must not be empty")
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	var oldName string
	var acctType int
	if err := tx.QueryRowContext(ctx,
		"SELECT name, type FROM accounts WHERE id = ?", accountID).Scan(&oldName, &acctType); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("account %s not found", accountID)
		}
		return fmt.Errorf("check account exists: %w", err)
	}

	if oldName == newName {
		_ = tx.Rollback()
		return errors.New("new name is the same as the current name")
	}

	payload, err := json.Marshal(AccountRenamedPayload{
		OldName: oldName,
		NewName: newName,
	})
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("marshal payload: %w", err)
	}

	if _, err := appendEvents(ctx, tx, aggregateTypeAccount, accountID, []NewEvent{
		{EventType: EventAccountRenamed, Payload: payload},
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("append event: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE accounts SET name = ?, updated_at = ? WHERE id = ?",
		newName, time.Now().UTC().Unix(), accountID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update read model: %w", err)
	}

	return tx.Commit()
}

// UpdateAccountType changes the type of an existing account.
func (db *DB) UpdateAccountType(ctx context.Context, accountID string, newType AccountType) error {
	if accountID == "" {
		return errors.New("account_id must not be empty")
	}
	if !ValidAccountType(newType) {
		return fmt.Errorf("invalid account type: %d", newType)
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	var name string
	var oldType int
	if err := tx.QueryRowContext(ctx,
		"SELECT name, type FROM accounts WHERE id = ?", accountID).Scan(&name, &oldType); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("account %s not found", accountID)
		}
		return fmt.Errorf("check account exists: %w", err)
	}

	if AccountType(oldType) == newType {
		_ = tx.Rollback()
		return errors.New("new type is the same as the current type")
	}

	payload, err := json.Marshal(AccountTypeChangedPayload{
		OldType: oldType,
		NewType: int(newType),
	})
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("marshal payload: %w", err)
	}

	if _, err := appendEvents(ctx, tx, aggregateTypeAccount, accountID, []NewEvent{
		{EventType: EventAccountTypeChanged, Payload: payload},
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("append event: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE accounts SET type = ?, updated_at = ? WHERE id = ?",
		int(newType), time.Now().UTC().Unix(), accountID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update read model: %w", err)
	}

	return tx.Commit()
}

// GetAccountHistory returns the event history for a specific account.
func (db *DB) GetAccountHistory(ctx context.Context, accountID string) ([]Event, error) {
	return db.LoadEvents(ctx, aggregateTypeAccount, accountID)
}

func scanAccounts(rows *sql.Rows) ([]Account, error) {
	var accounts []Account
	for rows.Next() {
		var a Account
		var createdUnix, updatedUnix int64
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &createdUnix, &updatedUnix); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		a.CreatedAt = time.Unix(createdUnix, 0).UTC()
		a.UpdatedAt = time.Unix(updatedUnix, 0).UTC()
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}
