package finchd

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jakewan/finch/core"
	finchv1 "github.com/jakewan/finch/daemon/gen/finch/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Version is the daemon version reported by the Ping RPC.
const Version = "0.1.0"

// Server implements the FinchService gRPC interface.
type Server struct {
	finchv1.UnimplementedFinchServiceServer
	db *core.DB
}

// NewServer returns a FinchServiceServer backed by the given database.
func NewServer(db *core.DB) finchv1.FinchServiceServer {
	return &Server{db: db}
}

func (s *Server) Ping(_ context.Context, _ *finchv1.PingRequest) (*finchv1.PingResponse, error) {
	return &finchv1.PingResponse{Version: Version}, nil
}

func (s *Server) ListAccounts(ctx context.Context, _ *finchv1.ListAccountsRequest) (*finchv1.ListAccountsResponse, error) {
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

func (s *Server) CreateAccount(ctx context.Context, req *finchv1.CreateAccountRequest) (*finchv1.CreateAccountResponse, error) {
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

func (s *Server) GetAccountHistory(ctx context.Context, req *finchv1.GetAccountHistoryRequest) (*finchv1.GetAccountHistoryResponse, error) {
	if strings.TrimSpace(req.AccountId) == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id must not be empty")
	}
	events, err := s.db.GetAccountHistory(ctx, req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get account history: %v", err)
	}
	return eventsToProto(events), nil
}

func (s *Server) RenameAccount(ctx context.Context, req *finchv1.RenameAccountRequest) (*finchv1.RenameAccountResponse, error) {
	if strings.TrimSpace(req.AccountId) == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id must not be empty")
	}
	if strings.TrimSpace(req.NewName) == "" {
		return nil, status.Error(codes.InvalidArgument, "new_name must not be empty")
	}
	if err := s.db.RenameAccount(ctx, req.AccountId, req.NewName); err != nil {
		return nil, accountError("rename account", err)
	}
	return &finchv1.RenameAccountResponse{}, nil
}

func (s *Server) UpdateAccountType(ctx context.Context, req *finchv1.UpdateAccountTypeRequest) (*finchv1.UpdateAccountTypeResponse, error) {
	if strings.TrimSpace(req.AccountId) == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id must not be empty")
	}
	if req.NewType == finchv1.AccountType_ACCOUNT_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "new_type must be specified")
	}
	if err := s.db.UpdateAccountType(ctx, req.AccountId, core.AccountType(req.NewType)); err != nil {
		return nil, accountError("update account type", err)
	}
	return &finchv1.UpdateAccountTypeResponse{}, nil
}

func (s *Server) CreateRecurringRule(ctx context.Context, req *finchv1.CreateRecurringRuleRequest) (*finchv1.CreateRecurringRuleResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, status.Error(codes.InvalidArgument, "name must not be empty")
	}
	if strings.TrimSpace(req.AccountId) == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id must not be empty")
	}
	if req.Frequency == finchv1.Frequency_FREQUENCY_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "frequency must be specified")
	}
	if req.Frequency < finchv1.Frequency_FREQUENCY_WEEKLY || req.Frequency > finchv1.Frequency_FREQUENCY_YEARLY {
		return nil, status.Errorf(codes.InvalidArgument, "invalid frequency: %d", req.Frequency)
	}

	// Validate the frequency-conditional day fields at the boundary so a bad request
	// surfaces as InvalidArgument. Core re-validates these (defense in depth), but core
	// returns untyped errors, so without this check they would reach the caller as
	// Internal. The exactly-two check also guards the [2]int mapping below.
	switch req.Frequency {
	case finchv1.Frequency_FREQUENCY_MONTHLY:
		if req.DayOfMonth < 1 || req.DayOfMonth > 31 {
			return nil, status.Errorf(codes.InvalidArgument, "day_of_month must be 1-31 for monthly frequency, got %d", req.DayOfMonth)
		}
	case finchv1.Frequency_FREQUENCY_SEMI_MONTHLY:
		if len(req.SemiMonthlyDays) != 2 {
			return nil, status.Errorf(codes.InvalidArgument, "semi_monthly frequency requires exactly two semi_monthly_days, got %d", len(req.SemiMonthlyDays))
		}
		for _, d := range req.SemiMonthlyDays {
			if d < 1 || d > 31 {
				return nil, status.Errorf(codes.InvalidArgument, "semi_monthly_days must each be 1-31, got %d", d)
			}
		}
	}

	// Transfer fields, checked syntactically here so a malformed request never
	// reaches core. Core re-validates and additionally verifies the target account
	// exists — that check needs a database read, which belongs in core, so it maps
	// back through ruleError rather than being duplicated at this boundary.
	if req.IsTransfer {
		if strings.TrimSpace(req.TransferTargetAccountId) == "" {
			return nil, status.Error(codes.InvalidArgument, "transfer_target_account_id must not be empty for a transfer rule")
		}
		if req.TransferTargetAccountId == req.AccountId {
			return nil, status.Error(codes.InvalidArgument, "transfer_target_account_id must differ from account_id")
		}
		if req.Amount <= 0 {
			return nil, status.Errorf(codes.InvalidArgument, "transfer rule amount must be positive, got %d", req.Amount)
		}
	}

	if err := checkAmountMagnitude("amount", req.Amount); err != nil {
		return nil, err
	}

	startDate, err := time.Parse(time.DateOnly, req.StartDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid start_date: %v", err)
	}

	params := core.CreateRecurringRuleParams{
		AccountID:               req.AccountId,
		Name:                    req.Name,
		Amount:                  req.Amount,
		Frequency:               core.Frequency(req.Frequency),
		StartDate:               startDate,
		DayOfMonth:              int(req.DayOfMonth),
		IsTransfer:              req.IsTransfer,
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
		return nil, ruleError("create recurring rule", err)
	}

	return &finchv1.CreateRecurringRuleResponse{
		Rule: recurringRuleToProto(rule),
	}, nil
}

func (s *Server) ListRecurringRules(ctx context.Context, req *finchv1.ListRecurringRulesRequest) (*finchv1.ListRecurringRulesResponse, error) {
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

func (s *Server) UpdateRecurringRuleAmount(ctx context.Context, req *finchv1.UpdateRecurringRuleAmountRequest) (*finchv1.UpdateRecurringRuleAmountResponse, error) {
	if req.RuleId == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id must not be empty")
	}
	// Unlike CreateRecurringRule and CreateTransfer, this handler cannot let the narrower
	// transfer rule answer first: whether the rule is a transfer is a property of the stored
	// row, and reading it belongs in core. So a zero on a transfer rule reports the generic
	// message here rather than "must be positive". Deliberate, not an oversight in the
	// transfer-first ordering the other two follow.
	if err := checkAmountMagnitude("new_amount", req.NewAmount); err != nil {
		return nil, err
	}
	effectiveDate, err := time.Parse(time.DateOnly, req.EffectiveDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid effective_date: %v", err)
	}
	if err := s.db.UpdateRecurringRuleAmount(ctx, req.RuleId, req.NewAmount, effectiveDate, req.Reason); err != nil {
		return nil, ruleError("update recurring rule amount", err)
	}
	return &finchv1.UpdateRecurringRuleAmountResponse{}, nil
}

func (s *Server) PauseRecurringRule(ctx context.Context, req *finchv1.PauseRecurringRuleRequest) (*finchv1.PauseRecurringRuleResponse, error) {
	if req.RuleId == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id must not be empty")
	}
	if err := s.db.PauseRecurringRule(ctx, req.RuleId); err != nil {
		return nil, ruleError("pause recurring rule", err)
	}
	return &finchv1.PauseRecurringRuleResponse{}, nil
}

func (s *Server) ResumeRecurringRule(ctx context.Context, req *finchv1.ResumeRecurringRuleRequest) (*finchv1.ResumeRecurringRuleResponse, error) {
	if req.RuleId == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id must not be empty")
	}
	if err := s.db.ResumeRecurringRule(ctx, req.RuleId); err != nil {
		return nil, ruleError("resume recurring rule", err)
	}
	return &finchv1.ResumeRecurringRuleResponse{}, nil
}

func (s *Server) EndRecurringRule(ctx context.Context, req *finchv1.EndRecurringRuleRequest) (*finchv1.EndRecurringRuleResponse, error) {
	if req.RuleId == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id must not be empty")
	}
	endDate, err := time.Parse(time.DateOnly, req.EndDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid end_date: %v", err)
	}
	if err := s.db.EndRecurringRule(ctx, req.RuleId, endDate); err != nil {
		return nil, ruleError("end recurring rule", err)
	}
	return &finchv1.EndRecurringRuleResponse{}, nil
}

func (s *Server) GetRecurringRuleHistory(ctx context.Context, req *finchv1.GetRecurringRuleHistoryRequest) (*finchv1.GetRecurringRuleHistoryResponse, error) {
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

func (s *Server) RecordTransaction(ctx context.Context, req *finchv1.RecordTransactionRequest) (*finchv1.RecordTransactionResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, status.Error(codes.InvalidArgument, "name must not be empty")
	}
	if strings.TrimSpace(req.AccountId) == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id must not be empty")
	}
	if req.Status == finchv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "transaction status must be specified")
	}
	if err := checkAmountMagnitude("amount", req.Amount); err != nil {
		return nil, err
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
		return nil, txnError("record transaction", err)
	}

	return &finchv1.RecordTransactionResponse{
		Transaction: transactionToProto(txn),
	}, nil
}

func (s *Server) ListTransactions(ctx context.Context, req *finchv1.ListTransactionsRequest) (*finchv1.ListTransactionsResponse, error) {
	if strings.TrimSpace(req.AccountId) == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id must not be empty")
	}
	txns, err := s.db.ListTransactions(ctx, req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list transactions: %v", err)
	}
	resp := &finchv1.ListTransactionsResponse{}
	for _, t := range txns {
		resp.Transactions = append(resp.Transactions, transactionToProto(&t))
	}
	return resp, nil
}

func (s *Server) UpdateTransactionStatus(ctx context.Context, req *finchv1.UpdateTransactionStatusRequest) (*finchv1.UpdateTransactionStatusResponse, error) {
	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id must not be empty")
	}
	if req.NewStatus == finchv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "new_status must be specified")
	}
	if err := s.db.UpdateTransactionStatus(ctx, req.TransactionId, core.TransactionStatus(req.NewStatus)); err != nil {
		return nil, txnError("update transaction status", err)
	}
	return &finchv1.UpdateTransactionStatusResponse{}, nil
}

func (s *Server) CreateTransfer(ctx context.Context, req *finchv1.CreateTransferRequest) (*finchv1.CreateTransferResponse, error) {
	if req.SourceAccountId == "" || req.DestinationAccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "both source and destination account_id must be provided")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	// The zero branch of the check below is unreachable behind the guard above; it stays
	// because the bound is one rule, not a per-handler assembly.
	if err := checkAmountMagnitude("amount", req.Amount); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, status.Error(codes.InvalidArgument, "name must not be empty")
	}
	if req.Status == finchv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "transaction status must be specified")
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
		return nil, txnError("create transfer", err)
	}
	return &finchv1.CreateTransferResponse{}, nil
}

func (s *Server) ProjectBalances(ctx context.Context, req *finchv1.ProjectBalancesRequest) (*finchv1.ProjectBalancesResponse, error) {
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
				IsTransfer:      t.IsTransfer,
				Status:          finchv1.TransactionStatus(t.Status),
				IsProjected:     t.IsProjected,
			})
		}
		resp.Balances = append(resp.Balances, pb)
	}
	return resp, nil
}

func (s *Server) ProjectBalanceOnDate(ctx context.Context, req *finchv1.ProjectBalanceOnDateRequest) (*finchv1.ProjectBalanceOnDateResponse, error) {
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

func (s *Server) GetMonthlyCashFlow(ctx context.Context, req *finchv1.GetMonthlyCashFlowRequest) (*finchv1.GetMonthlyCashFlowResponse, error) {
	if req.FromMonth == "" || req.ToMonth == "" {
		return nil, status.Error(codes.InvalidArgument, "from_month and to_month must not be empty")
	}
	if _, err := time.Parse("2006-01", req.FromMonth); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid from_month %q: expected YYYY-MM format", req.FromMonth)
	}
	if _, err := time.Parse("2006-01", req.ToMonth); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid to_month %q: expected YYYY-MM format", req.ToMonth)
	}
	months, err := s.db.GetMonthlyCashFlow(ctx, req.FromMonth, req.ToMonth, req.AccountIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get monthly cash flow: %v", err)
	}
	resp := &finchv1.GetMonthlyCashFlowResponse{}
	for _, m := range months {
		resp.Months = append(resp.Months, &finchv1.MonthlyCashFlow{
			Month:     m.Month,
			Income:    m.Income,
			Expenses:  m.Expenses,
			Net:       m.Net,
			Transfers: m.Transfers,
		})
	}
	return resp, nil
}

func (s *Server) GetBalanceTimeSeries(ctx context.Context, req *finchv1.GetBalanceTimeSeriesRequest) (*finchv1.GetBalanceTimeSeriesResponse, error) {
	fromDate, err := time.Parse(time.DateOnly, req.FromDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid from_date: %v", err)
	}
	toDate, err := time.Parse(time.DateOnly, req.ToDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid to_date: %v", err)
	}
	switch req.Interval {
	case finchv1.TimeSeriesInterval_TIME_SERIES_INTERVAL_DAILY,
		finchv1.TimeSeriesInterval_TIME_SERIES_INTERVAL_WEEKLY,
		finchv1.TimeSeriesInterval_TIME_SERIES_INTERVAL_MONTHLY:
		// Valid.
	default:
		return nil, status.Error(codes.InvalidArgument, "interval must be DAILY, WEEKLY, or MONTHLY")
	}

	points, err := s.db.GetBalanceTimeSeries(ctx, fromDate, toDate, core.TimeSeriesInterval(req.Interval), req.AccountIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get balance time series: %v", err)
	}

	resp := &finchv1.GetBalanceTimeSeriesResponse{}
	for _, p := range points {
		tp := &finchv1.BalanceTimePoint{
			Date: p.Date.Format(time.DateOnly),
		}
		for _, b := range p.Balances {
			tp.Balances = append(tp.Balances, &finchv1.AccountBalance{
				AccountId: b.AccountID,
				Balance:   b.Balance,
			})
		}
		resp.Points = append(resp.Points, tp)
	}
	return resp, nil
}

// accountError maps core account errors to gRPC status codes.
func accountError(op string, err error) error {
	if strings.Contains(err.Error(), "not found") {
		return status.Errorf(codes.NotFound, "%s: %v", op, err)
	}
	if errors.Is(err, core.ErrNoChange) {
		return status.Errorf(codes.InvalidArgument, "%s: %v", op, err)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}

// ruleError maps core recurring rule errors to gRPC status codes. Typed errors are
// checked first; the string match is a legacy fallback for paths in core that do not
// yet wrap a sentinel.
//
// The ordering is load-bearing, not incidental: an unknown transfer target is
// InvalidArgument (a bad field value in the request, not a missing rule aggregate),
// and its message contains the words "not found" — so the string fallback would
// misclassify it as NotFound if it ran first.
func ruleError(op string, err error) error {
	if errors.Is(err, core.ErrInvalidInput) {
		return status.Errorf(codes.InvalidArgument, "%s: %v", op, err)
	}
	if errors.Is(err, core.ErrNotFound) || strings.Contains(err.Error(), "not found") {
		return status.Errorf(codes.NotFound, "%s: %v", op, err)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}

// checkAmountMagnitude mirrors core's money invariant at the boundary, so a bad amount
// surfaces as InvalidArgument naming the offending proto field rather than reaching the
// caller as Internal. Core re-validates and remains the authority for a non-gRPC caller;
// the bound itself is read from core rather than restated here, so the two cannot drift.
func checkAmountMagnitude(field string, amount int64) error {
	if amount == 0 {
		return status.Errorf(codes.InvalidArgument, "%s must not be zero", field)
	}
	if amount > core.MaxAmountCents || amount < -core.MaxAmountCents {
		return status.Errorf(codes.InvalidArgument,
			"%s magnitude must not exceed %d cents, got %d", field, core.MaxAmountCents, amount)
	}
	return nil
}

// txnError maps core transaction errors to gRPC status codes, by error identity only.
// ruleError carries an additional substring fallback for rule paths in core that predate
// the sentinels; the transaction paths have none, so matching on message text here would
// guard an empty set while standing ready to misclassify any future error whose wording
// happens to contain the phrase — the trap ruleError documents and event-sourcing.md
// forbids. A new transaction error wraps a sentinel instead.
func txnError(op string, err error) error {
	if errors.Is(err, core.ErrInvalidInput) {
		return status.Errorf(codes.InvalidArgument, "%s: %v", op, err)
	}
	if errors.Is(err, core.ErrNotFound) {
		return status.Errorf(codes.NotFound, "%s: %v", op, err)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
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
		Id:                      r.ID,
		AccountId:               r.AccountID,
		Name:                    r.Name,
		Amount:                  r.Amount,
		Frequency:               finchv1.Frequency(r.Frequency),
		StartDate:               r.StartDate.Format(time.DateOnly),
		DayOfMonth:              int32(r.DayOfMonth),
		IsTransfer:              r.IsTransfer,
		TransferTargetAccountId: r.TransferTargetAccountID,
		Paused:                  r.Paused,
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
		IsTransfer:      t.IsTransfer,
	}
}
