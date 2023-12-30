package cosmos

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

// AuthzGrant grants a message as a permission to an account.
func AuthzGrant(ctx context.Context, chain *CosmosChain, granter ibc.Wallet, grantee, authType string, extraFlags ...string) (*sdk.TxResponse, error) {

	allowed := "send|generic|delegate|unbond|redelegate"
	if !strings.Contains(allowed, authType) {
		return nil, fmt.Errorf("invalid auth type: %s allowed: %s", authType, allowed)
	}

	cmd := []string{"authz", "grant", grantee, authType}

	// when using the generic type, you must specify a --msg-type flag
	if authType == "generic" {
		msgTypeIndex := -1
		for i, flag := range extraFlags {
			if flag == "--msg-type" {
				msgTypeIndex = i
				break
			}
		}

		if msgTypeIndex == -1 {
			return nil, fmt.Errorf("missing --msg-type flag when granting generic authz")
		}

		msgType := extraFlags[msgTypeIndex+1]
		if !strings.HasPrefix(msgType, "/") {
			extraFlags[msgTypeIndex+1] = "/" + msgType
		}
	}

	cmd = append(cmd, extraFlags...)

	txHash, err := chain.GetNode().ExecTx(ctx, granter.KeyName(),
		append(cmd, "--output", "json")...,
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

func (c *CosmosChain) AuthzQueryGrants(ctx context.Context, granter string, grantee string, msgType string, extraFlags ...string) ([]*authz.Grant, error) {
	res, err := authz.NewQueryClient(c.GetNode().GrpcConn).Grants(ctx, &authz.QueryGrantsRequest{
		Granter:    granter,
		Grantee:    grantee,
		MsgTypeUrl: msgType,
	})
	return res.Grants, err
}

func (c *CosmosChain) AuthzQueryGrantsByGrantee(ctx context.Context, grantee string, extraFlags ...string) ([]*authz.GrantAuthorization, error) {
	res, err := authz.NewQueryClient(c.GetNode().GrpcConn).GranteeGrants(ctx, &authz.QueryGranteeGrantsRequest{
		Grantee: grantee,
	})
	return res.Grants, err
}

func (c *CosmosChain) AuthzQueryGrantsByGranter(ctx context.Context, granter string, extraFlags ...string) ([]*authz.GrantAuthorization, error) {
	res, err := authz.NewQueryClient(c.GetNode().GrpcConn).GranterGrants(ctx, &authz.QueryGranterGrantsRequest{
		Granter: granter,
	})
	return res.Grants, err
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
