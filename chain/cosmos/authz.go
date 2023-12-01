package cosmos

import (
	"context"
	"fmt"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

// TODO: Convert to SDK v50.

// Available Commands:
//   exec        execute tx on behalf of granter account
//   grant       Grant authorization to an address
//   revoke      revoke authorization

// AuthzGrant grants a message as a permission to an account.
func AuthzGrant(ctx context.Context, chain *CosmosChain, granter ibc.Wallet, grantee string, msgType string) (*sdk.TxResponse, error) {
	if !strings.HasPrefix(msgType, "/") {
		msgType = "/" + msgType
	}

	txHash, err := chain.GetNode().ExecTx(ctx, granter.KeyName(),
		"authz", "grant", grantee, "generic", "--msg-type", msgType, "--output=json",
	)
	if err != nil {
		return nil, err
	}

	return chain.GetNode().TxHashToResponse(ctx, txHash)
}

// AuthzExec executes an authz MsgExec transaction with a single nested message.
func AuthzExec(ctx context.Context, chain *CosmosChain, grantee ibc.Wallet, nestedMsgCmd []string) (*sdk.TxResponse, error) {
	fileName := "authz.json"
	node := chain.GetNode()
	if err := createAuthzJSON(ctx, node, fileName, nestedMsgCmd); err != nil {
		return nil, err
	}

	txHash, err := chain.getFullNode().ExecTx(ctx, grantee.KeyName(),
		"authz", "exec", node.HomeDir()+"/"+fileName,
	)
	if err != nil {
		return nil, err
	}

	return chain.GetNode().TxHashToResponse(ctx, txHash)
}

// AuthzRevoke revokes a message as a permission to an account.
func AuthzRevoke(ctx context.Context, chain *CosmosChain, granter ibc.Wallet, grantee string, msgType string) (*sdk.TxResponse, error) {
	if !strings.HasPrefix(msgType, "/") {
		msgType = "/" + msgType
	}

	txHash, err := chain.getFullNode().ExecTx(ctx, granter.KeyName(),
		"authz", "revoke", grantee, msgType,
	)
	if err != nil {
		return nil, err
	}

	return chain.GetNode().TxHashToResponse(ctx, txHash)
}

// authz.QueryGrantsResponse
type QueryAuthzGrantsResponse struct {
	Grants []struct {
		Authorization struct {
			Type string `json:"@type"`
			Msg  string `json:"msg"`
		} `json:"authorization"`
		Expiration any `json:"expiration"`
	} `json:"grants"`
	Pagination struct {
		NextKey any    `json:"next_key"`
		Total   string `json:"total"`
	} `json:"pagination"`
}

// authz.QueryGranteeGrantsResponse & QueryGranterGrantsResponse
type QueryAuthzGrantsByResponse struct {
	Grants []struct {
		Granter       string `json:"granter"`
		Grantee       string `json:"grantee"`
		Authorization struct {
			Type string `json:"@type"`
			Msg  string `json:"msg"`
		} `json:"authorization"`
		Expiration time.Time `json:"expiration"`
	} `json:"grants"`
	Pagination struct {
		NextKey any    `json:"next_key"`
		Total   string `json:"total"`
	} `json:"pagination"`
}

func AuthzQueryGrants(ctx context.Context, chain *CosmosChain, granter string, grantee string, msgType string, extraFlags ...string) (*QueryAuthzGrantsResponse, error) {
	cmd := []string{"authz", "grants", granter, grantee, msgType}
	cmd = append(cmd, extraFlags...)

	var res QueryAuthzGrantsResponse
	return &res, chain.ExecQueryToResponse(ctx, chain, cmd, &res)
}

func AuthzQueryGrantsByGrantee(ctx context.Context, chain *CosmosChain, grantee string, extraFlags ...string) (*QueryAuthzGrantsByResponse, error) {
	cmd := []string{"authz", "grants-by-grantee", grantee}
	cmd = append(cmd, extraFlags...)

	var res QueryAuthzGrantsByResponse
	return &res, chain.ExecQueryToResponse(ctx, chain, cmd, &res)
}

func AuthzQueryGrantsByGranter(ctx context.Context, chain *CosmosChain, granter string, extraFlags ...string) (*QueryAuthzGrantsByResponse, error) {
	cmd := []string{"authz", "grants-by-granter", granter}
	cmd = append(cmd, extraFlags...)

	var res QueryAuthzGrantsByResponse
	return &res, chain.ExecQueryToResponse(ctx, chain, cmd, &res)
}

// createAuthzJSON creates a JSON file with a single generated message.
func createAuthzJSON(ctx context.Context, node *ChainNode, filePath string, genMsgCmd []string) error {
	if !strings.Contains(strings.Join(genMsgCmd, " "), "--generate-only") {
		genMsgCmd = append(genMsgCmd, "--generate-only")
	}

	res, resErr, err := node.Exec(ctx, genMsgCmd, nil)
	if resErr != nil {
		return fmt.Errorf("failed to generate msg: %s", resErr)
	}
	if err != nil {
		return err
	}

	return node.WriteFile(ctx, res, filePath)
}
