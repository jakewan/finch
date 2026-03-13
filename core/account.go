package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
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

type Account struct {
	ID        int64
	Name      string
	Type      AccountType
	CreatedAt time.Time
	UpdatedAt time.Time
}

func ValidAccountType(t AccountType) bool {
	return t >= AccountTypeChecking && t <= AccountTypeBrokerage
}

func (db *DB) CreateAccount(ctx context.Context, name string, accountType AccountType) (*Account, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("account name must not be empty")
	}
	if !ValidAccountType(accountType) {
		return nil, fmt.Errorf("invalid account type: %d", accountType)
	}
	now := time.Now().UTC()
	result, err := db.conn.ExecContext(ctx,
		"INSERT INTO accounts (name, type, created_at, updated_at) VALUES (?, ?, ?, ?)",
		name, int(accountType), now.Unix(), now.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("insert account: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return &Account{
		ID:        id,
		Name:      name,
		Type:      accountType,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (db *DB) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := db.conn.QueryContext(ctx, "SELECT id, name, type, created_at, updated_at FROM accounts ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("query accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAccounts(rows)
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
