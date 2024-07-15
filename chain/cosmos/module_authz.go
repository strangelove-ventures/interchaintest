package cosmos

import (
	"context"
	"fmt"
	"path"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

// AuthzGrant grants a message as a permission to an account.
func (tn *ChainNode) AuthzGrant(ctx context.Context, granter ibc.Wallet, grantee, authType string, extraFlags ...string) (*sdk.TxResponse, error) {

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

		extraFlags[msgTypeIndex+1] = PrefixMsgTypeIfRequired(extraFlags[msgTypeIndex+1])
	}

	cmd = append(cmd, extraFlags...)

	txHash, err := tn.ExecTx(ctx, granter.KeyName(),
		append(cmd, "--output", "json")...,
	)
	if err != nil {
		return nil, err
	}

	return tn.TxHashToResponse(ctx, txHash)
}

// AuthzExec executes an authz MsgExec transaction with a single nested message.
func (tn *ChainNode) AuthzExec(ctx context.Context, grantee ibc.Wallet, nestedMsgCmd []string) (*sdk.TxResponse, error) {
	fileName := "authz.json"
	if err := createAuthzJSON(ctx, tn, fileName, nestedMsgCmd); err != nil {
		return nil, err
	}

	txHash, err := tn.ExecTx(ctx, grantee.KeyName(),
		"authz", "exec", path.Join(tn.HomeDir(), fileName),
	)
	if err != nil {
		return nil, err
	}

	return tn.TxHashToResponse(ctx, txHash)
}

// AuthzRevoke revokes a message as a permission to an account.
func (tn *ChainNode) AuthzRevoke(ctx context.Context, granter ibc.Wallet, grantee string, msgType string) (*sdk.TxResponse, error) {
	msgType = PrefixMsgTypeIfRequired(msgType)

	txHash, err := tn.ExecTx(ctx, granter.KeyName(),
		"authz", "revoke", grantee, msgType,
	)
	if err != nil {
		return nil, err
	}

	return tn.TxHashToResponse(ctx, txHash)
}

// AuthzQueryGrants queries all grants for a given granter and grantee.
func (c *CosmosChain) AuthzQueryGrants(ctx context.Context, granter string, grantee string, msgType string, extraFlags ...string) ([]*authz.Grant, error) {
	res, err := authz.NewQueryClient(c.GetNode().GrpcConn).Grants(ctx, &authz.QueryGrantsRequest{
		Granter:    granter,
		Grantee:    grantee,
		MsgTypeUrl: msgType,
	})
	return res.Grants, err
}

// AuthzQueryGrantsByGrantee queries all grants for a given grantee.
func (c *CosmosChain) AuthzQueryGrantsByGrantee(ctx context.Context, grantee string, extraFlags ...string) ([]*authz.GrantAuthorization, error) {
	res, err := authz.NewQueryClient(c.GetNode().GrpcConn).GranteeGrants(ctx, &authz.QueryGranteeGrantsRequest{
		Grantee: grantee,
	})
	return res.Grants, err
}

// AuthzQueryGrantsByGranter returns all grants for a granter.
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

	res, resErr, err := node.Exec(ctx, genMsgCmd, node.Chain.Config().Env)
	if resErr != nil {
		return fmt.Errorf("failed to generate msg: %s", resErr)
	}
	if err != nil {
		return err
	}

	return node.WriteFile(ctx, res, filePath)
}

func PrefixMsgTypeIfRequired(msgType string) string {
	if !strings.HasPrefix(msgType, "/") {
		msgType = "/" + msgType
	}
	return msgType
}
