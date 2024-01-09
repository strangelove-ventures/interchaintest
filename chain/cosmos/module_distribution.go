package cosmos

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// DistributionFundCommunityPool funds the community pool with the specified amount of coins.
func (tn *ChainNode) DistributionFundCommunityPool(ctx context.Context, keyName, amount string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "distribution", "fund-community-pool", amount,
	)
	return err
}

func (tn *ChainNode) DistributionFundValidatorRewardsPool(ctx context.Context, keyName, valAddr, amount string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "distribution", "fund-validator-rewards-pool", valAddr, amount,
	)
	return err
}

// DistributionSetWithdrawAddr change the default withdraw address for rewards associated with an address.
func (tn *ChainNode) DistributionSetWithdrawAddr(ctx context.Context, keyName, withdrawAddr string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "distribution", "set-withdraw-addr", withdrawAddr,
	)
	return err
}

// DistributionWithdrawAllRewards withdraws all delegations rewards for a delegator.
func (tn *ChainNode) DistributionWithdrawAllRewards(ctx context.Context, keyName string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "distribution", "withdraw-all-rewards",
	)
	return err
}

// DistributionWithdrawValidatorRewards withdraws all delegations rewards for a delegator.
// If includeCommission is true, it also withdraws the validator's commission.
func (tn *ChainNode) DistributionWithdrawValidatorRewards(ctx context.Context, keyName, valAddr string, includeCommission bool) error {
	cmd := []string{"distribution", "withdraw-rewards", valAddr}

	if includeCommission {
		cmd = append(cmd, "--commission")
	}

	_, err := tn.ExecTx(ctx,
		keyName, cmd...,
	)
	return err
}

// DistributionCommission returns the validator's commission
func (c *CosmosChain) DistributionQueryCommission(ctx context.Context, valAddr string) (*distrtypes.ValidatorAccumulatedCommission, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		ValidatorCommission(ctx, &distrtypes.QueryValidatorCommissionRequest{
			ValidatorAddress: valAddr,
		})
	return &res.Commission, err
}

// DistributionCommunityPool returns the community pool
func (c *CosmosChain) DistributionQueryCommunityPool(ctx context.Context) (*sdk.DecCoins, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		CommunityPool(ctx, &distrtypes.QueryCommunityPoolRequest{})
	return &res.Pool, err
}

// DistributionDelegationTotalRewards returns the delegator's total rewards
func (c *CosmosChain) DistributionQueryDelegationTotalRewards(ctx context.Context, delegatorAddr string) (*distrtypes.QueryDelegationTotalRewardsResponse, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegationTotalRewards(ctx, &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: delegatorAddr})
	return res, err
}

// DistributionDelegatorValidators returns the delegator's validators
func (c *CosmosChain) DistributionQueryDelegatorValidators(ctx context.Context, delegatorAddr string) (*distrtypes.QueryDelegatorValidatorsResponse, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegatorValidators(ctx, &distrtypes.QueryDelegatorValidatorsRequest{DelegatorAddress: delegatorAddr})
	return res, err
}

// DistributionDelegatorWithdrawAddress returns the delegator's withdraw address
func (c *CosmosChain) DistributionQueryDelegatorWithdrawAddress(ctx context.Context, delegatorAddr string) (string, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegatorWithdrawAddress(ctx, &distrtypes.QueryDelegatorWithdrawAddressRequest{DelegatorAddress: delegatorAddr})
	return res.WithdrawAddress, err
}

// DistributionParams returns the distribution params
func (c *CosmosChain) DistributionQueryParams(ctx context.Context) (*distrtypes.Params, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		Params(ctx, &distrtypes.QueryParamsRequest{})
	return &res.Params, err
}

// DistributionRewards returns the delegator's rewards
func (c *CosmosChain) DistributionQueryRewards(ctx context.Context, delegatorAddr, valAddr string) (sdk.DecCoins, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegationRewards(ctx, &distrtypes.QueryDelegationRewardsRequest{
			DelegatorAddress: delegatorAddr,
			ValidatorAddress: valAddr,
		})
	return res.Rewards, err
}

// DistributionValidatorSlashes returns the validator's slashes
func (c *CosmosChain) DistributionQueryValidatorSlashes(ctx context.Context, valAddr string) ([]distrtypes.ValidatorSlashEvent, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		ValidatorSlashes(ctx, &distrtypes.QueryValidatorSlashesRequest{ValidatorAddress: valAddr})
	return res.Slashes, err
}

// DistributionValidatorDistributionInfo returns the validator's distribution info
func (c *CosmosChain) DistributionQueryValidatorDistributionInfo(ctx context.Context, valAddr string) (*distrtypes.QueryValidatorDistributionInfoResponse, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		ValidatorDistributionInfo(ctx, &distrtypes.QueryValidatorDistributionInfoRequest{ValidatorAddress: valAddr})
	return res, err
}

// DistributionValidatorOutstandingRewards returns the validator's outstanding rewards
func (c *CosmosChain) DistributionQueryValidatorOutstandingRewards(ctx context.Context, valAddr string) (*distrtypes.ValidatorOutstandingRewards, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		ValidatorOutstandingRewards(ctx, &distrtypes.QueryValidatorOutstandingRewardsRequest{ValidatorAddress: valAddr})
	return &res.Rewards, err
}
