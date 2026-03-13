package main

import (
	"context"
	"strings"

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
