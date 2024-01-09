package cosmos

import (
	"context"
	"strings"
	"time"

	"cosmossdk.io/x/feegrant"
)

// FeeGrant grants a fee grant.
func (tn *ChainNode) FeeGrant(ctx context.Context, granterKey, grantee, spendLimit string, allowedMsgs []string, expiration time.Time, extraFlags ...string) error {
	cmd := []string{"feegrant", "grant", granterKey, grantee, "--spend-limit", spendLimit}

	if len(allowedMsgs) > 0 {
		msgs := make([]string, len(allowedMsgs))
		for i, msg := range allowedMsgs {
			msg = PrefixMsgTypeIfRequired(msg)
			msgs[i] = msg
		}

		cmd = append(cmd, "--allowed-messages", strings.Join(msgs, ","))
	}

	if expiration.After(time.Now()) {
		cmd = append(cmd, "--expiration", expiration.Format(time.RFC3339))
	}

	cmd = append(cmd, extraFlags...)

	_, err := tn.ExecTx(ctx, granterKey, cmd...)
	return err
}

// FeeGrantRevoke revokes a fee grant.
func (tn *ChainNode) FeeGrantRevoke(ctx context.Context, keyName, granterAddr, granteeAddr string) error {
	_, err := tn.ExecTx(ctx, keyName, "feegrant", "revoke", granterAddr, granteeAddr)
	return err
}

// FeeGrantGetAllowance returns the allowance of a granter and grantee pair.
func (c *CosmosChain) FeeGrantQueryAllowance(ctx context.Context, granter, grantee string) (*feegrant.Grant, error) {
	res, err := feegrant.NewQueryClient(c.GetNode().GrpcConn).Allowance(ctx, &feegrant.QueryAllowanceRequest{
		Granter: granter,
		Grantee: grantee,
	})
	return res.Allowance, err
}

// FeeGrantGetAllowances returns all allowances of a grantee.
func (c *CosmosChain) FeeGrantQueryAllowances(ctx context.Context, grantee string) ([]*feegrant.Grant, error) {
	res, err := feegrant.NewQueryClient(c.GetNode().GrpcConn).Allowances(ctx, &feegrant.QueryAllowancesRequest{
		Grantee: grantee,
	})
	return res.Allowances, err
}

// FeeGrantGetAllowancesByGranter returns all allowances of a granter.
func (c *CosmosChain) FeeGrantQueryAllowancesByGranter(ctx context.Context, granter string) ([]*feegrant.Grant, error) {
	res, err := feegrant.NewQueryClient(c.GetNode().GrpcConn).AllowancesByGranter(ctx, &feegrant.QueryAllowancesByGranterRequest{
		Granter: granter,
	})
	return res.Allowances, err
}
