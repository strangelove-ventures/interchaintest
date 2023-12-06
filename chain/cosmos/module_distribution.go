package cosmos

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
func (c *CosmosChain) DistributionCommission(ctx context.Context) (*distrtypes.ValidatorAccumulatedCommission, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		ValidatorCommission(ctx, &distrtypes.QueryValidatorCommissionRequest{})
	return &res.Commission, err
}

// DistributionCommunityPool returns the community pool
func (c *CosmosChain) DistributionCommunityPool(ctx context.Context) (*sdk.DecCoins, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		CommunityPool(ctx, &distrtypes.QueryCommunityPoolRequest{})
	return &res.Pool, err
}

// DistributionDelegationTotalRewards returns the delegator's total rewards
func (c *CosmosChain) DistributionDelegationTotalRewards(ctx context.Context, delegatorAddr string) (*distrtypes.QueryDelegationTotalRewardsResponse, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegationTotalRewards(ctx, &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: delegatorAddr})
	return res, err
}

// DistributionDelegatorValidators returns the delegator's validators
func (c *CosmosChain) DistributionDelegatorValidators(ctx context.Context, delegatorAddr string) (*distrtypes.QueryDelegatorValidatorsResponse, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegatorValidators(ctx, &distrtypes.QueryDelegatorValidatorsRequest{DelegatorAddress: delegatorAddr})
	return res, err
}

// DistributionDelegatorWithdrawAddress returns the delegator's withdraw address
func (c *CosmosChain) DistributionDelegatorWithdrawAddress(ctx context.Context, delegatorAddr string) (*distrtypes.QueryDelegatorWithdrawAddressResponse, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegatorWithdrawAddress(ctx, &distrtypes.QueryDelegatorWithdrawAddressRequest{DelegatorAddress: delegatorAddr})
	return res, err
}

// DistributionParams returns the distribution params
func (c *CosmosChain) DistributionParams(ctx context.Context) (*distrtypes.Params, error) {
	res, err := distrtypes.NewQueryClient(c.GetNode().GrpcConn).
		Params(ctx, &distrtypes.QueryParamsRequest{})
	return &res.Params, err
}

// DistributionRewards returns the delegator's rewards
func (c *CosmosChain) DistributionRewards(ctx context.Context, delegatorAddr string) (sdk.DecCoins, error) {
	grpcConn, err := grpc.Dial(
		c.GetNode().hostGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	defer grpcConn.Close()

	res, err := distrtypes.NewQueryClient(grpcConn).
		DelegationRewards(ctx, &distrtypes.QueryDelegationRewardsRequest{DelegatorAddress: delegatorAddr})
	return res.Rewards, err
}

// DistributionValidatorSlashes returns the validator's slashes
func (c *CosmosChain) DistributionValidatorSlashes(ctx context.Context, valAddr string) ([]distrtypes.ValidatorSlashEvent, error) {
	grpcConn, err := grpc.Dial(
		c.GetNode().hostGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	defer grpcConn.Close()

	res, err := distrtypes.NewQueryClient(grpcConn).
		ValidatorSlashes(ctx, &distrtypes.QueryValidatorSlashesRequest{ValidatorAddress: valAddr})
	return res.Slashes, err
}

// DistributionValidatorDistributionInfo returns the validator's distribution info
func (c *CosmosChain) DistributionValidatorDistributionInfo(ctx context.Context, valAddr string) (*distrtypes.QueryValidatorDistributionInfoResponse, error) {
	grpcConn, err := grpc.Dial(
		c.GetNode().hostGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	defer grpcConn.Close()

	res, err := distrtypes.NewQueryClient(grpcConn).
		ValidatorDistributionInfo(ctx, &distrtypes.QueryValidatorDistributionInfoRequest{ValidatorAddress: valAddr})
	return res, err
}

// DistributionValidatorOutstandingRewards returns the validator's outstanding rewards
func (c *CosmosChain) DistributionValidatorOutstandingRewards(ctx context.Context, valAddr string) (*distrtypes.ValidatorOutstandingRewards, error) {
	grpcConn, err := grpc.Dial(
		c.GetNode().hostGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	defer grpcConn.Close()

	res, err := distrtypes.NewQueryClient(grpcConn).
		ValidatorOutstandingRewards(ctx, &distrtypes.QueryValidatorOutstandingRewardsRequest{ValidatorAddress: valAddr})
	return &res.Rewards, err
}
