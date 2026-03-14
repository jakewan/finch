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

const aggregateTypeTransaction = "transaction"

// Transaction event types.
const (
	EventTransactionRecorded      = "TransactionRecorded"
	EventTransactionStatusChanged = "TransactionStatusChanged"
	EventTransferCreated          = "TransferCreated"
)

// TransactionStatus represents the lifecycle state of a transaction.
type TransactionStatus int

const (
	TransactionStatusUnspecified TransactionStatus = 0
	TransactionStatusProjected  TransactionStatus = 1
	TransactionStatusScheduled  TransactionStatus = 2
	TransactionStatusReconciled TransactionStatus = 3
)

func ValidTransactionStatus(s TransactionStatus) bool {
	return s >= TransactionStatusProjected && s <= TransactionStatusReconciled
}

// Transaction represents the read model for a financial transaction.
type Transaction struct {
	ID              string
	AccountID       string
	Date            time.Time
	Amount          int64 // cents
	Name            string
	Description     string
	Status          TransactionStatus
	RecurringRuleID string
}

// TransactionRecordedPayload is the event data for recording a transaction.
type TransactionRecordedPayload struct {
	AccountID       string `json:"account_id"`
	Date            string `json:"date"`
	Amount          int64  `json:"amount"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Status          int    `json:"status"`
	RecurringRuleID string `json:"recurring_rule_id,omitempty"`
}

// TransactionStatusChangedPayload records a status transition.
type TransactionStatusChangedPayload struct {
	OldStatus int `json:"old_status"`
	NewStatus int `json:"new_status"`
}

// TransferCreatedPayload records both sides of a transfer.
type TransferCreatedPayload struct {
	SourceAccountID      string `json:"source_account_id"`
	DestinationAccountID string `json:"destination_account_id"`
	RecurringRuleID      string `json:"recurring_rule_id,omitempty"`
	Amount               int64  `json:"amount"`
	Date                 string `json:"date"`
	SourceTransactionID  string `json:"source_transaction_id"`
	DestTransactionID    string `json:"dest_transaction_id"`
}

// RecordTransactionParams holds validated input for recording a transaction.
type RecordTransactionParams struct {
	AccountID       string
	Date            time.Time
	Amount          int64
	Name            string
	Description     string
	Status          TransactionStatus
	RecurringRuleID string
}

// RecordTransaction creates a new transaction with event sourcing.
func (db *DB) RecordTransaction(ctx context.Context, params RecordTransactionParams) (*Transaction, error) {
	params.Name = strings.TrimSpace(params.Name)
	if params.Name == "" {
		return nil, errors.New("transaction name must not be empty")
	}
	if params.AccountID == "" {
		return nil, errors.New("account_id must not be empty")
	}
	if !ValidTransactionStatus(params.Status) {
		return nil, fmt.Errorf("invalid transaction status: %d", params.Status)
	}

	id := uuid.New().String()

	payload, err := json.Marshal(TransactionRecordedPayload{
		AccountID:       params.AccountID,
		Date:            params.Date.Format(time.DateOnly),
		Amount:          params.Amount,
		Name:            params.Name,
		Description:     params.Description,
		Status:          int(params.Status),
		RecurringRuleID: params.RecurringRuleID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}

	if _, err := appendEvents(ctx, tx, aggregateTypeTransaction, id, []NewEvent{
		{EventType: EventTransactionRecorded, Payload: payload},
	}); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("append event: %w", err)
	}

	var recurringRuleID *string
	if params.RecurringRuleID != "" {
		recurringRuleID = &params.RecurringRuleID
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO transactions (id, account_id, date, amount, name, description, status, recurring_rule_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, params.AccountID, params.Date.Format(time.DateOnly), params.Amount,
		params.Name, nilIfEmpty(params.Description), int(params.Status), recurringRuleID,
	); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("insert transaction read model: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Transaction{
		ID:              id,
		AccountID:       params.AccountID,
		Date:            params.Date,
		Amount:          params.Amount,
		Name:            params.Name,
		Description:     params.Description,
		Status:          params.Status,
		RecurringRuleID: params.RecurringRuleID,
	}, nil
}

// UpdateTransactionStatus transitions a transaction to a new status.
func (db *DB) UpdateTransactionStatus(ctx context.Context, txnID string, newStatus TransactionStatus) error {
	if txnID == "" {
		return errors.New("transaction_id must not be empty")
	}
	if !ValidTransactionStatus(newStatus) {
		return fmt.Errorf("invalid transaction status: %d", newStatus)
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	var oldStatus int
	row := tx.QueryRowContext(ctx, "SELECT status FROM transactions WHERE id = ?", txnID)
	if err := row.Scan(&oldStatus); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("transaction %s not found", txnID)
		}
		return fmt.Errorf("read current status: %w", err)
	}

	payload, err := json.Marshal(TransactionStatusChangedPayload{
		OldStatus: oldStatus,
		NewStatus: int(newStatus),
	})
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("marshal payload: %w", err)
	}

	if _, err := appendEvents(ctx, tx, aggregateTypeTransaction, txnID, []NewEvent{
		{EventType: EventTransactionStatusChanged, Payload: payload},
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("append event: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE transactions SET status = ? WHERE id = ?", int(newStatus), txnID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update read model: %w", err)
	}

	return tx.Commit()
}

// CreateTransferParams holds input for creating a transfer between accounts.
type CreateTransferParams struct {
	SourceAccountID      string
	DestinationAccountID string
	Amount               int64 // positive amount, will be negated for source
	Date                 time.Time
	Name                 string
	Description          string
	Status               TransactionStatus
	RecurringRuleID      string
}

// CreateTransfer records a transfer as paired transactions on source and destination accounts.
func (db *DB) CreateTransfer(ctx context.Context, params CreateTransferParams) error {
	if params.SourceAccountID == "" || params.DestinationAccountID == "" {
		return errors.New("both source and destination account_id must be provided")
	}
	if params.Amount <= 0 {
		return errors.New("transfer amount must be positive")
	}
	params.Name = strings.TrimSpace(params.Name)
	if params.Name == "" {
		return errors.New("transfer name must not be empty")
	}

	sourceID := uuid.New().String()
	destID := uuid.New().String()
	dateStr := params.Date.Format(time.DateOnly)

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	// Source transaction (negative amount — money leaving).
	sourcePayload, _ := json.Marshal(TransactionRecordedPayload{
		AccountID:       params.SourceAccountID,
		Date:            dateStr,
		Amount:          -params.Amount,
		Name:            params.Name,
		Status:          int(params.Status),
		RecurringRuleID: params.RecurringRuleID,
	})
	if _, err := appendEvents(ctx, tx, aggregateTypeTransaction, sourceID, []NewEvent{
		{EventType: EventTransactionRecorded, Payload: sourcePayload},
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("append source event: %w", err)
	}

	// Destination transaction (positive amount — money arriving).
	destPayload, _ := json.Marshal(TransactionRecordedPayload{
		AccountID:       params.DestinationAccountID,
		Date:            dateStr,
		Amount:          params.Amount,
		Name:            params.Name,
		Status:          int(params.Status),
		RecurringRuleID: params.RecurringRuleID,
	})
	if _, err := appendEvents(ctx, tx, aggregateTypeTransaction, destID, []NewEvent{
		{EventType: EventTransactionRecorded, Payload: destPayload},
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("append dest event: %w", err)
	}

	// Transfer event linking both sides.
	transferPayload, _ := json.Marshal(TransferCreatedPayload{
		SourceAccountID:      params.SourceAccountID,
		DestinationAccountID: params.DestinationAccountID,
		RecurringRuleID:      params.RecurringRuleID,
		Amount:               params.Amount,
		Date:                 dateStr,
		SourceTransactionID:  sourceID,
		DestTransactionID:    destID,
	})
	// Record the transfer event on the source transaction aggregate for traceability.
	if _, err := appendEvents(ctx, tx, aggregateTypeTransaction, sourceID, []NewEvent{
		{EventType: EventTransferCreated, Payload: transferPayload},
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("append transfer event: %w", err)
	}

	// Insert read model rows.
	var recurringRuleID *string
	if params.RecurringRuleID != "" {
		recurringRuleID = &params.RecurringRuleID
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO transactions (id, account_id, date, amount, name, description, status, recurring_rule_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sourceID, params.SourceAccountID, dateStr, -params.Amount,
		params.Name, nilIfEmpty(params.Description), int(params.Status), recurringRuleID,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert source transaction: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO transactions (id, account_id, date, amount, name, description, status, recurring_rule_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		destID, params.DestinationAccountID, dateStr, params.Amount,
		params.Name, nilIfEmpty(params.Description), int(params.Status), recurringRuleID,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert dest transaction: %w", err)
	}

	return tx.Commit()
}

// ListTransactions returns transactions for an account, ordered by date.
func (db *DB) ListTransactions(ctx context.Context, accountID string) ([]Transaction, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, account_id, date, amount, name, description, status, recurring_rule_id
		 FROM transactions WHERE account_id = ? ORDER BY date, name`, accountID)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanTransactions(rows)
}

func scanTransactions(rows *sql.Rows) ([]Transaction, error) {
	var txns []Transaction
	for rows.Next() {
		var t Transaction
		var dateStr string
		var description sql.NullString
		var recurringRuleID sql.NullString
		if err := rows.Scan(&t.ID, &t.AccountID, &dateStr, &t.Amount, &t.Name,
			&description, &t.Status, &recurringRuleID); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		d, err := time.Parse(time.DateOnly, dateStr)
		if err != nil {
			return nil, fmt.Errorf("parse date: %w", err)
		}
		t.Date = d
		if description.Valid {
			t.Description = description.String
		}
		if recurringRuleID.Valid {
			t.RecurringRuleID = recurringRuleID.String
		}
		txns = append(txns, t)
	}
	return txns, rows.Err()
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
