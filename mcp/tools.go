package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	finchv1 "github.com/jakewan/finch/daemon/gen/finch/v1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerTools(server *mcp.Server, client finchv1.FinchServiceClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ping",
		Description: "Check that the Finch daemon is running and return its version.",
	}, newPingHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_accounts",
		Description: "List all financial accounts.",
	}, newListAccountsHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_account",
		Description: "Create a new financial account.",
	}, newCreateAccountHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_account_history",
		Description: "Get the event history for an account, showing all changes over time.",
	}, newGetAccountHistoryHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_recurring_rule",
		Description: "Create a recurring transaction rule (template). Amount is in cents. Frequency: WEEKLY, BIWEEKLY, SEMI_MONTHLY, MONTHLY, YEARLY.",
	}, newCreateRecurringRuleHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_recurring_rules",
		Description: "List recurring transaction rules, optionally filtered by account.",
	}, newListRecurringRulesHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_recurring_rule_amount",
		Description: "Change the amount of a recurring rule with an effective date and optional reason.",
	}, newUpdateRecurringRuleAmountHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_recurring_rule_history",
		Description: "Get the event history for a recurring rule, showing all changes over time.",
	}, newGetRecurringRuleHistoryHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "record_transaction",
		Description: "Record a financial transaction. Amount is in cents (negative for withdrawals). Status: PROJECTED, SCHEDULED, or RECONCILED.",
	}, newRecordTransactionHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_transaction_status",
		Description: "Update the status of a transaction (PROJECTED → SCHEDULED → RECONCILED).",
	}, newUpdateTransactionStatusHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_transfer",
		Description: "Create a transfer between two accounts. Amount is in cents (positive, will be negated for source).",
	}, newCreateTransferHandler(client))
}

// Ping

type PingInput struct{}
type PingOutput struct {
	Version string `json:"version"`
}

func newPingHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, PingInput) (*mcp.CallToolResult, PingOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ PingInput) (*mcp.CallToolResult, PingOutput, error) {
		resp, err := client.Ping(ctx, &finchv1.PingRequest{})
		if err != nil {
			return nil, PingOutput{}, fmt.Errorf("ping: %w", err)
		}
		return nil, PingOutput{Version: resp.Version}, nil
	}
}

// ListAccounts

type ListAccountsInput struct{}
type AccountInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}
type ListAccountsOutput struct {
	Accounts []AccountInfo `json:"accounts"`
}

func newListAccountsHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, ListAccountsInput) (*mcp.CallToolResult, ListAccountsOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ ListAccountsInput) (*mcp.CallToolResult, ListAccountsOutput, error) {
		resp, err := client.ListAccounts(ctx, &finchv1.ListAccountsRequest{})
		if err != nil {
			return nil, ListAccountsOutput{}, fmt.Errorf("list accounts: %w", err)
		}
		accounts := make([]AccountInfo, len(resp.Accounts))
		for i, a := range resp.Accounts {
			accounts[i] = AccountInfo{
				ID:   a.Id,
				Name: a.Name,
				Type: a.Type.String(),
			}
		}
		return nil, ListAccountsOutput{Accounts: accounts}, nil
	}
}

// CreateAccount

type CreateAccountInput struct {
	Name string `json:"name" jsonschema:"the name of the account"`
	Type string `json:"type" jsonschema:"account type: CHECKING, SAVINGS, CREDIT_CARD, AUTO_LOAN, PERSONAL_LOAN, LINE_OF_CREDIT, or BROKERAGE"`
}
type CreateAccountOutput struct {
	Account AccountInfo `json:"account"`
}

var accountTypeMap = map[string]finchv1.AccountType{
	"CHECKING":       finchv1.AccountType_ACCOUNT_TYPE_CHECKING,
	"SAVINGS":        finchv1.AccountType_ACCOUNT_TYPE_SAVINGS,
	"CREDIT_CARD":    finchv1.AccountType_ACCOUNT_TYPE_CREDIT_CARD,
	"AUTO_LOAN":      finchv1.AccountType_ACCOUNT_TYPE_AUTO_LOAN,
	"PERSONAL_LOAN":  finchv1.AccountType_ACCOUNT_TYPE_PERSONAL_LOAN,
	"LINE_OF_CREDIT": finchv1.AccountType_ACCOUNT_TYPE_LINE_OF_CREDIT,
	"BROKERAGE":      finchv1.AccountType_ACCOUNT_TYPE_BROKERAGE,
}

func newCreateAccountHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, CreateAccountInput) (*mcp.CallToolResult, CreateAccountOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CreateAccountInput) (*mcp.CallToolResult, CreateAccountOutput, error) {
		normalized := strings.ToUpper(strings.TrimSpace(input.Type))
		// Accept both "CHECKING" and "ACCOUNT_TYPE_CHECKING" forms.
		normalized = strings.TrimPrefix(normalized, "ACCOUNT_TYPE_")
		acctType, ok := accountTypeMap[normalized]
		if !ok {
			valid := make([]string, 0, len(accountTypeMap))
			for k := range accountTypeMap {
				valid = append(valid, k)
			}
			slices.Sort(valid)
			return nil, CreateAccountOutput{}, fmt.Errorf("unknown account type %q; valid types: %s", input.Type, strings.Join(valid, ", "))
		}
		resp, err := client.CreateAccount(ctx, &finchv1.CreateAccountRequest{
			Name: input.Name,
			Type: acctType,
		})
		if err != nil {
			return nil, CreateAccountOutput{}, fmt.Errorf("create account: %w", err)
		}
		return nil, CreateAccountOutput{
			Account: AccountInfo{
				ID:   resp.Account.Id,
				Name: resp.Account.Name,
				Type: resp.Account.Type.String(),
			},
		}, nil
	}
}

// GetAccountHistory

type GetAccountHistoryInput struct {
	AccountID string `json:"account_id" jsonschema:"the UUID of the account"`
}
type AccountEventInfo struct {
	ID         int64  `json:"id"`
	EventType  string `json:"event_type"`
	Payload    string `json:"payload"`
	RecordedAt int64  `json:"recorded_at"`
	Sequence   int64  `json:"sequence"`
}
type GetAccountHistoryOutput struct {
	Events []AccountEventInfo `json:"events"`
}

func newGetAccountHistoryHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, GetAccountHistoryInput) (*mcp.CallToolResult, GetAccountHistoryOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input GetAccountHistoryInput) (*mcp.CallToolResult, GetAccountHistoryOutput, error) {
		resp, err := client.GetAccountHistory(ctx, &finchv1.GetAccountHistoryRequest{
			AccountId: input.AccountID,
		})
		if err != nil {
			return nil, GetAccountHistoryOutput{}, fmt.Errorf("get account history: %w", err)
		}
		return nil, GetAccountHistoryOutput{Events: protoEventsToInfo(resp.Events)}, nil
	}
}

// CreateRecurringRule

type CreateRecurringRuleInput struct {
	AccountID              string  `json:"account_id" jsonschema:"UUID of the account"`
	Name                   string  `json:"name" jsonschema:"name of the recurring rule"`
	Amount                 int64   `json:"amount" jsonschema:"amount in cents"`
	Frequency              string  `json:"frequency" jsonschema:"WEEKLY, BIWEEKLY, SEMI_MONTHLY, MONTHLY, or YEARLY"`
	StartDate              string  `json:"start_date" jsonschema:"ISO 8601 date (YYYY-MM-DD)"`
	EndDate                string  `json:"end_date,omitempty" jsonschema:"optional end date (YYYY-MM-DD)"`
	DayOfMonth             int32   `json:"day_of_month,omitempty" jsonschema:"day of month for monthly rules (1-31)"`
	SemiMonthlyDays        []int32 `json:"semi_monthly_days,omitempty" jsonschema:"two days for semi-monthly rules"`
	IsTransfer             bool    `json:"is_transfer,omitempty" jsonschema:"whether this rule represents a transfer"`
	TransferTargetAccountID string `json:"transfer_target_account_id,omitempty" jsonschema:"UUID of the target account for transfers"`
}

type RecurringRuleInfo struct {
	ID                     string  `json:"id"`
	AccountID              string  `json:"account_id"`
	Name                   string  `json:"name"`
	Amount                 int64   `json:"amount"`
	Frequency              string  `json:"frequency"`
	StartDate              string  `json:"start_date"`
	EndDate                string  `json:"end_date,omitempty"`
	DayOfMonth             int32   `json:"day_of_month,omitempty"`
	SemiMonthlyDays        []int32 `json:"semi_monthly_days,omitempty"`
	IsTransfer             bool    `json:"is_transfer,omitempty"`
	TransferTargetAccountID string `json:"transfer_target_account_id,omitempty"`
	Paused                 bool    `json:"paused"`
}

type CreateRecurringRuleOutput struct {
	Rule RecurringRuleInfo `json:"rule"`
}

var frequencyMap = map[string]finchv1.Frequency{
	"WEEKLY":       finchv1.Frequency_FREQUENCY_WEEKLY,
	"BIWEEKLY":     finchv1.Frequency_FREQUENCY_BIWEEKLY,
	"SEMI_MONTHLY": finchv1.Frequency_FREQUENCY_SEMI_MONTHLY,
	"MONTHLY":      finchv1.Frequency_FREQUENCY_MONTHLY,
	"YEARLY":       finchv1.Frequency_FREQUENCY_YEARLY,
}

func newCreateRecurringRuleHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, CreateRecurringRuleInput) (*mcp.CallToolResult, CreateRecurringRuleOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CreateRecurringRuleInput) (*mcp.CallToolResult, CreateRecurringRuleOutput, error) {
		normalized := strings.ToUpper(strings.TrimSpace(input.Frequency))
		normalized = strings.TrimPrefix(normalized, "FREQUENCY_")
		freq, ok := frequencyMap[normalized]
		if !ok {
			valid := make([]string, 0, len(frequencyMap))
			for k := range frequencyMap {
				valid = append(valid, k)
			}
			slices.Sort(valid)
			return nil, CreateRecurringRuleOutput{}, fmt.Errorf("unknown frequency %q; valid: %s", input.Frequency, strings.Join(valid, ", "))
		}

		resp, err := client.CreateRecurringRule(ctx, &finchv1.CreateRecurringRuleRequest{
			AccountId:              input.AccountID,
			Name:                   input.Name,
			Amount:                 input.Amount,
			Frequency:              freq,
			StartDate:              input.StartDate,
			EndDate:                input.EndDate,
			DayOfMonth:             input.DayOfMonth,
			SemiMonthlyDays:        input.SemiMonthlyDays,
			IsTransfer:             input.IsTransfer,
			TransferTargetAccountId: input.TransferTargetAccountID,
		})
		if err != nil {
			return nil, CreateRecurringRuleOutput{}, fmt.Errorf("create recurring rule: %w", err)
		}
		return nil, CreateRecurringRuleOutput{Rule: protoRuleToInfo(resp.Rule)}, nil
	}
}

// ListRecurringRules

type ListRecurringRulesInput struct {
	AccountID string `json:"account_id,omitempty" jsonschema:"optional account UUID to filter by"`
}
type ListRecurringRulesOutput struct {
	Rules []RecurringRuleInfo `json:"rules"`
}

func newListRecurringRulesHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, ListRecurringRulesInput) (*mcp.CallToolResult, ListRecurringRulesOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input ListRecurringRulesInput) (*mcp.CallToolResult, ListRecurringRulesOutput, error) {
		resp, err := client.ListRecurringRules(ctx, &finchv1.ListRecurringRulesRequest{
			AccountId: input.AccountID,
		})
		if err != nil {
			return nil, ListRecurringRulesOutput{}, fmt.Errorf("list recurring rules: %w", err)
		}
		rules := make([]RecurringRuleInfo, len(resp.Rules))
		for i, r := range resp.Rules {
			rules[i] = protoRuleToInfo(r)
		}
		return nil, ListRecurringRulesOutput{Rules: rules}, nil
	}
}

// UpdateRecurringRuleAmount

type UpdateRecurringRuleAmountInput struct {
	RuleID        string `json:"rule_id" jsonschema:"UUID of the recurring rule"`
	NewAmount     int64  `json:"new_amount" jsonschema:"new amount in cents"`
	EffectiveDate string `json:"effective_date" jsonschema:"when the new amount takes effect (YYYY-MM-DD)"`
	Reason        string `json:"reason,omitempty" jsonschema:"optional reason for the change"`
}
type UpdateRecurringRuleAmountOutput struct{}

func newUpdateRecurringRuleAmountHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, UpdateRecurringRuleAmountInput) (*mcp.CallToolResult, UpdateRecurringRuleAmountOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input UpdateRecurringRuleAmountInput) (*mcp.CallToolResult, UpdateRecurringRuleAmountOutput, error) {
		_, err := client.UpdateRecurringRuleAmount(ctx, &finchv1.UpdateRecurringRuleAmountRequest{
			RuleId:        input.RuleID,
			NewAmount:     input.NewAmount,
			EffectiveDate: input.EffectiveDate,
			Reason:        input.Reason,
		})
		if err != nil {
			return nil, UpdateRecurringRuleAmountOutput{}, fmt.Errorf("update recurring rule amount: %w", err)
		}
		return nil, UpdateRecurringRuleAmountOutput{}, nil
	}
}

// GetRecurringRuleHistory

type GetRecurringRuleHistoryInput struct {
	RuleID string `json:"rule_id" jsonschema:"UUID of the recurring rule"`
}
type GetRecurringRuleHistoryOutput struct {
	Events []AccountEventInfo `json:"events"`
}

func newGetRecurringRuleHistoryHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, GetRecurringRuleHistoryInput) (*mcp.CallToolResult, GetRecurringRuleHistoryOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input GetRecurringRuleHistoryInput) (*mcp.CallToolResult, GetRecurringRuleHistoryOutput, error) {
		resp, err := client.GetRecurringRuleHistory(ctx, &finchv1.GetRecurringRuleHistoryRequest{
			RuleId: input.RuleID,
		})
		if err != nil {
			return nil, GetRecurringRuleHistoryOutput{}, fmt.Errorf("get recurring rule history: %w", err)
		}
		return nil, GetRecurringRuleHistoryOutput{Events: protoEventsToInfo(resp.Events)}, nil
	}
}

// RecordTransaction

type RecordTransactionInput struct {
	AccountID       string `json:"account_id" jsonschema:"UUID of the account"`
	Date            string `json:"date" jsonschema:"transaction date (YYYY-MM-DD)"`
	Amount          int64  `json:"amount" jsonschema:"amount in cents (negative for withdrawals)"`
	Name            string `json:"name" jsonschema:"transaction name"`
	Description     string `json:"description,omitempty" jsonschema:"optional description"`
	Status          string `json:"status" jsonschema:"PROJECTED, SCHEDULED, or RECONCILED"`
	RecurringRuleID string `json:"recurring_rule_id,omitempty" jsonschema:"optional recurring rule UUID"`
}

type TransactionInfo struct {
	ID              string `json:"id"`
	AccountID       string `json:"account_id"`
	Date            string `json:"date"`
	Amount          int64  `json:"amount"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Status          string `json:"status"`
	RecurringRuleID string `json:"recurring_rule_id,omitempty"`
}

type RecordTransactionOutput struct {
	Transaction TransactionInfo `json:"transaction"`
}

var transactionStatusMap = map[string]finchv1.TransactionStatus{
	"PROJECTED":  finchv1.TransactionStatus_TRANSACTION_STATUS_PROJECTED,
	"SCHEDULED":  finchv1.TransactionStatus_TRANSACTION_STATUS_SCHEDULED,
	"RECONCILED": finchv1.TransactionStatus_TRANSACTION_STATUS_RECONCILED,
}

func newRecordTransactionHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, RecordTransactionInput) (*mcp.CallToolResult, RecordTransactionOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input RecordTransactionInput) (*mcp.CallToolResult, RecordTransactionOutput, error) {
		normalized := strings.ToUpper(strings.TrimSpace(input.Status))
		normalized = strings.TrimPrefix(normalized, "TRANSACTION_STATUS_")
		txnStatus, ok := transactionStatusMap[normalized]
		if !ok {
			valid := make([]string, 0, len(transactionStatusMap))
			for k := range transactionStatusMap {
				valid = append(valid, k)
			}
			slices.Sort(valid)
			return nil, RecordTransactionOutput{}, fmt.Errorf("unknown status %q; valid: %s", input.Status, strings.Join(valid, ", "))
		}

		resp, err := client.RecordTransaction(ctx, &finchv1.RecordTransactionRequest{
			AccountId:       input.AccountID,
			Date:            input.Date,
			Amount:          input.Amount,
			Name:            input.Name,
			Description:     input.Description,
			Status:          txnStatus,
			RecurringRuleId: input.RecurringRuleID,
		})
		if err != nil {
			return nil, RecordTransactionOutput{}, fmt.Errorf("record transaction: %w", err)
		}
		return nil, RecordTransactionOutput{
			Transaction: protoTxnToInfo(resp.Transaction),
		}, nil
	}
}

// UpdateTransactionStatus

type UpdateTransactionStatusInput struct {
	TransactionID string `json:"transaction_id" jsonschema:"UUID of the transaction"`
	NewStatus     string `json:"new_status" jsonschema:"PROJECTED, SCHEDULED, or RECONCILED"`
}
type UpdateTransactionStatusOutput struct{}

func newUpdateTransactionStatusHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, UpdateTransactionStatusInput) (*mcp.CallToolResult, UpdateTransactionStatusOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input UpdateTransactionStatusInput) (*mcp.CallToolResult, UpdateTransactionStatusOutput, error) {
		normalized := strings.ToUpper(strings.TrimSpace(input.NewStatus))
		normalized = strings.TrimPrefix(normalized, "TRANSACTION_STATUS_")
		txnStatus, ok := transactionStatusMap[normalized]
		if !ok {
			valid := make([]string, 0, len(transactionStatusMap))
			for k := range transactionStatusMap {
				valid = append(valid, k)
			}
			slices.Sort(valid)
			return nil, UpdateTransactionStatusOutput{}, fmt.Errorf("unknown status %q; valid: %s", input.NewStatus, strings.Join(valid, ", "))
		}

		_, err := client.UpdateTransactionStatus(ctx, &finchv1.UpdateTransactionStatusRequest{
			TransactionId: input.TransactionID,
			NewStatus:     txnStatus,
		})
		if err != nil {
			return nil, UpdateTransactionStatusOutput{}, fmt.Errorf("update transaction status: %w", err)
		}
		return nil, UpdateTransactionStatusOutput{}, nil
	}
}

// CreateTransfer

type CreateTransferInput struct {
	SourceAccountID      string `json:"source_account_id" jsonschema:"UUID of the source account"`
	DestinationAccountID string `json:"destination_account_id" jsonschema:"UUID of the destination account"`
	Amount               int64  `json:"amount" jsonschema:"transfer amount in cents (positive)"`
	Date                 string `json:"date" jsonschema:"transfer date (YYYY-MM-DD)"`
	Name                 string `json:"name" jsonschema:"transfer name"`
	Description          string `json:"description,omitempty" jsonschema:"optional description"`
	Status               string `json:"status" jsonschema:"PROJECTED, SCHEDULED, or RECONCILED"`
	RecurringRuleID      string `json:"recurring_rule_id,omitempty" jsonschema:"optional recurring rule UUID"`
}
type CreateTransferOutput struct{}

func newCreateTransferHandler(client finchv1.FinchServiceClient) func(context.Context, *mcp.CallToolRequest, CreateTransferInput) (*mcp.CallToolResult, CreateTransferOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CreateTransferInput) (*mcp.CallToolResult, CreateTransferOutput, error) {
		normalized := strings.ToUpper(strings.TrimSpace(input.Status))
		normalized = strings.TrimPrefix(normalized, "TRANSACTION_STATUS_")
		txnStatus, ok := transactionStatusMap[normalized]
		if !ok {
			valid := make([]string, 0, len(transactionStatusMap))
			for k := range transactionStatusMap {
				valid = append(valid, k)
			}
			slices.Sort(valid)
			return nil, CreateTransferOutput{}, fmt.Errorf("unknown status %q; valid: %s", input.Status, strings.Join(valid, ", "))
		}

		_, err := client.CreateTransfer(ctx, &finchv1.CreateTransferRequest{
			SourceAccountId:      input.SourceAccountID,
			DestinationAccountId: input.DestinationAccountID,
			Amount:               input.Amount,
			Date:                 input.Date,
			Name:                 input.Name,
			Description:          input.Description,
			Status:               txnStatus,
			RecurringRuleId:      input.RecurringRuleID,
		})
		if err != nil {
			return nil, CreateTransferOutput{}, fmt.Errorf("create transfer: %w", err)
		}
		return nil, CreateTransferOutput{}, nil
	}
}

// Mapping helpers

func protoEventsToInfo(events []*finchv1.AccountEvent) []AccountEventInfo {
	result := make([]AccountEventInfo, len(events))
	for i, e := range events {
		result[i] = AccountEventInfo{
			ID:         e.Id,
			EventType:  e.EventType,
			Payload:    e.Payload,
			RecordedAt: e.RecordedAt,
			Sequence:   e.Sequence,
		}
	}
	return result
}

func protoRuleToInfo(r *finchv1.RecurringRule) RecurringRuleInfo {
	return RecurringRuleInfo{
		ID:                     r.Id,
		AccountID:              r.AccountId,
		Name:                   r.Name,
		Amount:                 r.Amount,
		Frequency:              r.Frequency.String(),
		StartDate:              r.StartDate,
		EndDate:                r.EndDate,
		DayOfMonth:             r.DayOfMonth,
		SemiMonthlyDays:        r.SemiMonthlyDays,
		IsTransfer:             r.IsTransfer,
		TransferTargetAccountID: r.TransferTargetAccountId,
		Paused:                 r.Paused,
	}
}

func protoTxnToInfo(t *finchv1.Transaction) TransactionInfo {
	return TransactionInfo{
		ID:              t.Id,
		AccountID:       t.AccountId,
		Date:            t.Date,
		Amount:          t.Amount,
		Name:            t.Name,
		Description:     t.Description,
		Status:          t.Status.String(),
		RecurringRuleID: t.RecurringRuleId,
	}
}
