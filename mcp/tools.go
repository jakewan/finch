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
	ID   int64  `json:"id"`
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
