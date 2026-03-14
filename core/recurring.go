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

const aggregateTypeRecurringRule = "recurring_rule"

// Recurring rule event types.
const (
	EventRecurringRuleCreated       = "RecurringRuleCreated"
	EventRecurringRuleAmountChanged = "RecurringRuleAmountChanged"
	EventRecurringRulePaused        = "RecurringRulePaused"
	EventRecurringRuleResumed       = "RecurringRuleResumed"
	EventRecurringRuleEnded         = "RecurringRuleEnded"
)

// RecurringRule represents the read model for a recurring transaction template.
type RecurringRule struct {
	ID                     string
	AccountID              string
	Name                   string
	Amount                 int64 // cents
	Frequency              Frequency
	StartDate              time.Time
	EndDate                *time.Time
	DayOfMonth             int
	SemiMonthlyDays        [2]int
	IsTransfer             bool
	TransferTargetAccountID string
	Paused                 bool
}

// RecurringRuleCreatedPayload is the event data for rule creation.
type RecurringRuleCreatedPayload struct {
	AccountID              string `json:"account_id"`
	Name                   string `json:"name"`
	Amount                 int64  `json:"amount"`
	Frequency              int    `json:"frequency"`
	StartDate              string `json:"start_date"`
	EndDate                string `json:"end_date,omitempty"`
	DayOfMonth             int    `json:"day_of_month,omitempty"`
	SemiMonthlyDays        []int  `json:"semi_monthly_days,omitempty"`
	IsTransfer             bool   `json:"is_transfer,omitempty"`
	TransferTargetAccountID string `json:"transfer_target_account_id,omitempty"`
}

// RecurringRuleAmountChangedPayload records an amount change with context.
type RecurringRuleAmountChangedPayload struct {
	OldAmount     int64  `json:"old_amount"`
	NewAmount     int64  `json:"new_amount"`
	EffectiveDate string `json:"effective_date"`
	Reason        string `json:"reason,omitempty"`
}

// RecurringRulePausedPayload records when a rule was paused.
type RecurringRulePausedPayload struct{}

// RecurringRuleResumedPayload records when a rule was resumed.
type RecurringRuleResumedPayload struct{}

// RecurringRuleEndedPayload records the end date for a rule.
type RecurringRuleEndedPayload struct {
	EndDate string `json:"end_date"`
}

// CreateRecurringRuleParams holds validated input for creating a recurring rule.
type CreateRecurringRuleParams struct {
	AccountID              string
	Name                   string
	Amount                 int64
	Frequency              Frequency
	StartDate              time.Time
	EndDate                *time.Time
	DayOfMonth             int
	SemiMonthlyDays        [2]int
	IsTransfer             bool
	TransferTargetAccountID string
}

// CreateRecurringRule validates input, appends a RecurringRuleCreated event,
// and updates the read model within a single transaction.
func (db *DB) CreateRecurringRule(ctx context.Context, params CreateRecurringRuleParams) (*RecurringRule, error) {
	params.Name = strings.TrimSpace(params.Name)
	if params.Name == "" {
		return nil, errors.New("recurring rule name must not be empty")
	}
	if params.AccountID == "" {
		return nil, errors.New("account_id must not be empty")
	}
	if !ValidFrequency(params.Frequency) {
		return nil, fmt.Errorf("invalid frequency: %d", params.Frequency)
	}
	if params.Frequency == FrequencyMonthly && (params.DayOfMonth < 1 || params.DayOfMonth > 31) {
		return nil, fmt.Errorf("day_of_month must be 1-31 for monthly frequency, got %d", params.DayOfMonth)
	}
	if params.Frequency == FrequencySemiMonthly {
		if params.SemiMonthlyDays[0] < 1 || params.SemiMonthlyDays[0] > 31 ||
			params.SemiMonthlyDays[1] < 1 || params.SemiMonthlyDays[1] > 31 {
			return nil, fmt.Errorf("semi_monthly_days must each be 1-31")
		}
	}

	id := uuid.New().String()

	p := RecurringRuleCreatedPayload{
		AccountID:              params.AccountID,
		Name:                   params.Name,
		Amount:                 params.Amount,
		Frequency:              int(params.Frequency),
		StartDate:              params.StartDate.Format(time.DateOnly),
		IsTransfer:             params.IsTransfer,
		TransferTargetAccountID: params.TransferTargetAccountID,
	}
	if params.EndDate != nil {
		p.EndDate = params.EndDate.Format(time.DateOnly)
	}
	if params.DayOfMonth > 0 {
		p.DayOfMonth = params.DayOfMonth
	}
	if params.Frequency == FrequencySemiMonthly {
		p.SemiMonthlyDays = []int{params.SemiMonthlyDays[0], params.SemiMonthlyDays[1]}
	}

	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}

	if _, err := appendEvents(ctx, tx, aggregateTypeRecurringRule, id, []NewEvent{
		{EventType: EventRecurringRuleCreated, Payload: payload},
	}); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("append event: %w", err)
	}

	var endDateStr *string
	if params.EndDate != nil {
		s := params.EndDate.Format(time.DateOnly)
		endDateStr = &s
	}
	var semiMonthlyJSON *string
	if params.Frequency == FrequencySemiMonthly {
		b, _ := json.Marshal([]int{params.SemiMonthlyDays[0], params.SemiMonthlyDays[1]})
		s := string(b)
		semiMonthlyJSON = &s
	}
	var transferTarget *string
	if params.TransferTargetAccountID != "" {
		transferTarget = &params.TransferTargetAccountID
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO recurring_rules (id, account_id, name, amount, frequency, start_date, end_date,
		 day_of_month, semi_monthly_days, is_transfer, transfer_target_account_id, paused)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		id, params.AccountID, params.Name, params.Amount, int(params.Frequency),
		params.StartDate.Format(time.DateOnly), endDateStr,
		params.DayOfMonth, semiMonthlyJSON,
		boolToInt(params.IsTransfer), transferTarget,
	); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("insert recurring_rules read model: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &RecurringRule{
		ID:                     id,
		AccountID:              params.AccountID,
		Name:                   params.Name,
		Amount:                 params.Amount,
		Frequency:              params.Frequency,
		StartDate:              params.StartDate,
		EndDate:                params.EndDate,
		DayOfMonth:             params.DayOfMonth,
		SemiMonthlyDays:        params.SemiMonthlyDays,
		IsTransfer:             params.IsTransfer,
		TransferTargetAccountID: params.TransferTargetAccountID,
		Paused:                 false,
	}, nil
}

// ListRecurringRules returns recurring rules, optionally filtered by account.
func (db *DB) ListRecurringRules(ctx context.Context, accountID string) ([]RecurringRule, error) {
	var rows *sql.Rows
	var err error
	if accountID != "" {
		rows, err = db.conn.QueryContext(ctx,
			`SELECT id, account_id, name, amount, frequency, start_date, end_date,
			 day_of_month, semi_monthly_days, is_transfer, transfer_target_account_id, paused
			 FROM recurring_rules WHERE account_id = ? ORDER BY name`, accountID)
	} else {
		rows, err = db.conn.QueryContext(ctx,
			`SELECT id, account_id, name, amount, frequency, start_date, end_date,
			 day_of_month, semi_monthly_days, is_transfer, transfer_target_account_id, paused
			 FROM recurring_rules ORDER BY name`)
	}
	if err != nil {
		return nil, fmt.Errorf("query recurring_rules: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanRecurringRules(rows)
}

// UpdateRecurringRuleAmount changes a rule's amount and records the change event.
func (db *DB) UpdateRecurringRuleAmount(ctx context.Context, ruleID string, newAmount int64, effectiveDate time.Time, reason string) error {
	if ruleID == "" {
		return errors.New("rule_id must not be empty")
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	var oldAmount int64
	row := tx.QueryRowContext(ctx, "SELECT amount FROM recurring_rules WHERE id = ?", ruleID)
	if err := row.Scan(&oldAmount); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("recurring rule %s not found", ruleID)
		}
		return fmt.Errorf("read current amount: %w", err)
	}

	payload, err := json.Marshal(RecurringRuleAmountChangedPayload{
		OldAmount:     oldAmount,
		NewAmount:     newAmount,
		EffectiveDate: effectiveDate.Format(time.DateOnly),
		Reason:        reason,
	})
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("marshal payload: %w", err)
	}

	if _, err := appendEvents(ctx, tx, aggregateTypeRecurringRule, ruleID, []NewEvent{
		{EventType: EventRecurringRuleAmountChanged, Payload: payload},
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("append event: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE recurring_rules SET amount = ? WHERE id = ?", newAmount, ruleID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update read model: %w", err)
	}

	return tx.Commit()
}

// PauseRecurringRule marks a rule as paused.
func (db *DB) PauseRecurringRule(ctx context.Context, ruleID string) error {
	return db.setRecurringRulePaused(ctx, ruleID, true, EventRecurringRulePaused)
}

// ResumeRecurringRule marks a rule as active.
func (db *DB) ResumeRecurringRule(ctx context.Context, ruleID string) error {
	return db.setRecurringRulePaused(ctx, ruleID, false, EventRecurringRuleResumed)
}

func (db *DB) setRecurringRulePaused(ctx context.Context, ruleID string, paused bool, eventType string) error {
	if ruleID == "" {
		return errors.New("rule_id must not be empty")
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	payload, _ := json.Marshal(struct{}{})
	if _, err := appendEvents(ctx, tx, aggregateTypeRecurringRule, ruleID, []NewEvent{
		{EventType: eventType, Payload: payload},
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("append event: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE recurring_rules SET paused = ? WHERE id = ?", boolToInt(paused), ruleID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update read model: %w", err)
	}

	return tx.Commit()
}

// GetRecurringRuleHistory returns the event history for a specific recurring rule.
func (db *DB) GetRecurringRuleHistory(ctx context.Context, ruleID string) ([]Event, error) {
	return db.LoadEvents(ctx, aggregateTypeRecurringRule, ruleID)
}

func scanRecurringRules(rows *sql.Rows) ([]RecurringRule, error) {
	var rules []RecurringRule
	for rows.Next() {
		var r RecurringRule
		var startDateStr string
		var endDateStr sql.NullString
		var semiMonthlyStr sql.NullString
		var transferTarget sql.NullString
		var isTransfer, paused int

		if err := rows.Scan(&r.ID, &r.AccountID, &r.Name, &r.Amount, &r.Frequency,
			&startDateStr, &endDateStr, &r.DayOfMonth, &semiMonthlyStr,
			&isTransfer, &transferTarget, &paused); err != nil {
			return nil, fmt.Errorf("scan recurring_rule: %w", err)
		}

		sd, err := time.Parse(time.DateOnly, startDateStr)
		if err != nil {
			return nil, fmt.Errorf("parse start_date: %w", err)
		}
		r.StartDate = sd

		if endDateStr.Valid {
			ed, err := time.Parse(time.DateOnly, endDateStr.String)
			if err != nil {
				return nil, fmt.Errorf("parse end_date: %w", err)
			}
			r.EndDate = &ed
		}

		if semiMonthlyStr.Valid {
			var days []int
			if err := json.Unmarshal([]byte(semiMonthlyStr.String), &days); err == nil && len(days) == 2 {
				r.SemiMonthlyDays = [2]int{days[0], days[1]}
			}
		}

		r.IsTransfer = isTransfer != 0
		r.Paused = paused != 0
		if transferTarget.Valid {
			r.TransferTargetAccountID = transferTarget.String
		}

		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
