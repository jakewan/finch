package main

import (
	"context"
	"strings"
	"time"

	"github.com/jakewan/finch/core"
	finchv1 "github.com/jakewan/finch/daemon/gen/finch/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const version = "0.1.0"

type finchServer struct {
	finchv1.UnimplementedFinchServiceServer
	db *core.DB
}

func (s *finchServer) Ping(_ context.Context, _ *finchv1.PingRequest) (*finchv1.PingResponse, error) {
	return &finchv1.PingResponse{Version: version}, nil
}

func (s *finchServer) ListAccounts(ctx context.Context, _ *finchv1.ListAccountsRequest) (*finchv1.ListAccountsResponse, error) {
	accounts, err := s.db.ListAccounts(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list accounts: %v", err)
	}
	resp := &finchv1.ListAccountsResponse{}
	for _, a := range accounts {
		resp.Accounts = append(resp.Accounts, &finchv1.Account{
			Id:   a.ID,
			Name: a.Name,
			Type: finchv1.AccountType(a.Type),
		})
	}
	return resp, nil
}

func (s *finchServer) CreateAccount(ctx context.Context, req *finchv1.CreateAccountRequest) (*finchv1.CreateAccountResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, status.Error(codes.InvalidArgument, "account name must not be empty")
	}
	if req.Type == finchv1.AccountType_ACCOUNT_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "account type must be specified")
	}
	acct, err := s.db.CreateAccount(ctx, req.Name, core.AccountType(req.Type))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create account: %v", err)
	}
	return &finchv1.CreateAccountResponse{
		Account: &finchv1.Account{
			Id:   acct.ID,
			Name: acct.Name,
			Type: finchv1.AccountType(acct.Type),
		},
	}, nil
}

func (s *finchServer) GetAccountHistory(ctx context.Context, req *finchv1.GetAccountHistoryRequest) (*finchv1.GetAccountHistoryResponse, error) {
	if strings.TrimSpace(req.AccountId) == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id must not be empty")
	}
	events, err := s.db.GetAccountHistory(ctx, req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get account history: %v", err)
	}
	return eventsToProto(events), nil
}

func (s *finchServer) CreateRecurringRule(ctx context.Context, req *finchv1.CreateRecurringRuleRequest) (*finchv1.CreateRecurringRuleResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, status.Error(codes.InvalidArgument, "name must not be empty")
	}
	if req.AccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id must not be empty")
	}
	if req.Frequency == finchv1.Frequency_FREQUENCY_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "frequency must be specified")
	}

	startDate, err := time.Parse(time.DateOnly, req.StartDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid start_date: %v", err)
	}

	params := core.CreateRecurringRuleParams{
		AccountID:              req.AccountId,
		Name:                   req.Name,
		Amount:                 req.Amount,
		Frequency:              core.Frequency(req.Frequency),
		StartDate:              startDate,
		DayOfMonth:             int(req.DayOfMonth),
		IsTransfer:             req.IsTransfer,
		TransferTargetAccountID: req.TransferTargetAccountId,
	}

	if req.EndDate != "" {
		endDate, err := time.Parse(time.DateOnly, req.EndDate)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid end_date: %v", err)
		}
		params.EndDate = &endDate
	}

	if len(req.SemiMonthlyDays) == 2 {
		params.SemiMonthlyDays = [2]int{int(req.SemiMonthlyDays[0]), int(req.SemiMonthlyDays[1])}
	}

	rule, err := s.db.CreateRecurringRule(ctx, params)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create recurring rule: %v", err)
	}

	return &finchv1.CreateRecurringRuleResponse{
		Rule: recurringRuleToProto(rule),
	}, nil
}

func (s *finchServer) ListRecurringRules(ctx context.Context, req *finchv1.ListRecurringRulesRequest) (*finchv1.ListRecurringRulesResponse, error) {
	rules, err := s.db.ListRecurringRules(ctx, req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list recurring rules: %v", err)
	}
	resp := &finchv1.ListRecurringRulesResponse{}
	for _, r := range rules {
		resp.Rules = append(resp.Rules, recurringRuleToProto(&r))
	}
	return resp, nil
}

func (s *finchServer) UpdateRecurringRuleAmount(ctx context.Context, req *finchv1.UpdateRecurringRuleAmountRequest) (*finchv1.UpdateRecurringRuleAmountResponse, error) {
	if req.RuleId == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id must not be empty")
	}
	effectiveDate, err := time.Parse(time.DateOnly, req.EffectiveDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid effective_date: %v", err)
	}
	if err := s.db.UpdateRecurringRuleAmount(ctx, req.RuleId, req.NewAmount, effectiveDate, req.Reason); err != nil {
		return nil, status.Errorf(codes.Internal, "update recurring rule amount: %v", err)
	}
	return &finchv1.UpdateRecurringRuleAmountResponse{}, nil
}

func (s *finchServer) GetRecurringRuleHistory(ctx context.Context, req *finchv1.GetRecurringRuleHistoryRequest) (*finchv1.GetRecurringRuleHistoryResponse, error) {
	if req.RuleId == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id must not be empty")
	}
	events, err := s.db.GetRecurringRuleHistory(ctx, req.RuleId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get recurring rule history: %v", err)
	}
	proto := eventsToProto(events)
	return &finchv1.GetRecurringRuleHistoryResponse{Events: proto.Events}, nil
}

func (s *finchServer) RecordTransaction(ctx context.Context, req *finchv1.RecordTransactionRequest) (*finchv1.RecordTransactionResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, status.Error(codes.InvalidArgument, "name must not be empty")
	}
	if req.AccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id must not be empty")
	}
	txnDate, err := time.Parse(time.DateOnly, req.Date)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid date: %v", err)
	}

	txn, err := s.db.RecordTransaction(ctx, core.RecordTransactionParams{
		AccountID:       req.AccountId,
		Date:            txnDate,
		Amount:          req.Amount,
		Name:            req.Name,
		Description:     req.Description,
		Status:          core.TransactionStatus(req.Status),
		RecurringRuleID: req.RecurringRuleId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "record transaction: %v", err)
	}

	return &finchv1.RecordTransactionResponse{
		Transaction: transactionToProto(txn),
	}, nil
}

func (s *finchServer) UpdateTransactionStatus(ctx context.Context, req *finchv1.UpdateTransactionStatusRequest) (*finchv1.UpdateTransactionStatusResponse, error) {
	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id must not be empty")
	}
	if err := s.db.UpdateTransactionStatus(ctx, req.TransactionId, core.TransactionStatus(req.NewStatus)); err != nil {
		return nil, status.Errorf(codes.Internal, "update transaction status: %v", err)
	}
	return &finchv1.UpdateTransactionStatusResponse{}, nil
}

func (s *finchServer) CreateTransfer(ctx context.Context, req *finchv1.CreateTransferRequest) (*finchv1.CreateTransferResponse, error) {
	if req.SourceAccountId == "" || req.DestinationAccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "both source and destination account_id must be provided")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	txnDate, err := time.Parse(time.DateOnly, req.Date)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid date: %v", err)
	}

	if err := s.db.CreateTransfer(ctx, core.CreateTransferParams{
		SourceAccountID:      req.SourceAccountId,
		DestinationAccountID: req.DestinationAccountId,
		Amount:               req.Amount,
		Date:                 txnDate,
		Name:                 req.Name,
		Description:          req.Description,
		Status:               core.TransactionStatus(req.Status),
		RecurringRuleID:      req.RecurringRuleId,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "create transfer: %v", err)
	}
	return &finchv1.CreateTransferResponse{}, nil
}

func (s *finchServer) ProjectBalances(ctx context.Context, req *finchv1.ProjectBalancesRequest) (*finchv1.ProjectBalancesResponse, error) {
	fromDate, err := time.Parse(time.DateOnly, req.FromDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid from_date: %v", err)
	}
	toDate, err := time.Parse(time.DateOnly, req.ToDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid to_date: %v", err)
	}

	balances, err := s.db.ProjectBalances(ctx, fromDate, toDate, req.AccountIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "project balances: %v", err)
	}

	resp := &finchv1.ProjectBalancesResponse{}
	for _, b := range balances {
		pb := &finchv1.DailyBalance{
			Date:      b.Date.Format(time.DateOnly),
			AccountId: b.AccountID,
			Balance:   b.Balance,
		}
		for _, t := range b.Transactions {
			pb.Transactions = append(pb.Transactions, &finchv1.ProjectedTransaction{
				Date:            t.Date.Format(time.DateOnly),
				Amount:          t.Amount,
				Name:            t.Name,
				AccountId:       t.AccountID,
				RecurringRuleId: t.RecurringRuleID,
				Status:          finchv1.TransactionStatus(t.Status),
				IsProjected:     t.IsProjected,
			})
		}
		resp.Balances = append(resp.Balances, pb)
	}
	return resp, nil
}

func (s *finchServer) ProjectBalanceOnDate(ctx context.Context, req *finchv1.ProjectBalanceOnDateRequest) (*finchv1.ProjectBalanceOnDateResponse, error) {
	if req.AccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id must not be empty")
	}
	targetDate, err := time.Parse(time.DateOnly, req.Date)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid date: %v", err)
	}

	balance, err := s.db.ProjectBalanceOnDate(ctx, targetDate, req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "project balance on date: %v", err)
	}
	return &finchv1.ProjectBalanceOnDateResponse{Balance: balance}, nil
}

// Mapping helpers

func eventsToProto(events []core.Event) *finchv1.GetAccountHistoryResponse {
	resp := &finchv1.GetAccountHistoryResponse{}
	for _, e := range events {
		resp.Events = append(resp.Events, &finchv1.AccountEvent{
			Id:         e.ID,
			EventType:  e.EventType,
			Payload:    string(e.Payload),
			RecordedAt: e.RecordedAt.Unix(),
			Sequence:   e.Sequence,
		})
	}
	return resp
}

func recurringRuleToProto(r *core.RecurringRule) *finchv1.RecurringRule {
	rule := &finchv1.RecurringRule{
		Id:                       r.ID,
		AccountId:                r.AccountID,
		Name:                     r.Name,
		Amount:                   r.Amount,
		Frequency:                finchv1.Frequency(r.Frequency),
		StartDate:                r.StartDate.Format(time.DateOnly),
		DayOfMonth:               int32(r.DayOfMonth),
		IsTransfer:               r.IsTransfer,
		TransferTargetAccountId:  r.TransferTargetAccountID,
		Paused:                   r.Paused,
	}
	if r.EndDate != nil {
		rule.EndDate = r.EndDate.Format(time.DateOnly)
	}
	if r.SemiMonthlyDays[0] != 0 {
		rule.SemiMonthlyDays = []int32{int32(r.SemiMonthlyDays[0]), int32(r.SemiMonthlyDays[1])}
	}
	return rule
}

func transactionToProto(t *core.Transaction) *finchv1.Transaction {
	return &finchv1.Transaction{
		Id:              t.ID,
		AccountId:       t.AccountID,
		Date:            t.Date.Format(time.DateOnly),
		Amount:          t.Amount,
		Name:            t.Name,
		Description:     t.Description,
		Status:          finchv1.TransactionStatus(t.Status),
		RecurringRuleId: t.RecurringRuleID,
	}
}
