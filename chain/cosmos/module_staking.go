package cosmos

import (
	"context"
	"fmt"

	stakingtypes "cosmossdk.io/x/staking/types"
)

// StakingCancelUnbond cancels an unbonding delegation.
func (tn *ChainNode) StakingCancelUnbond(ctx context.Context, keyName, validatorAddr, coinAmt string, creationHeight int64) error {
	_, err := tn.ExecTx(ctx,
		keyName, "staking", "cancel-unbond", validatorAddr, coinAmt, fmt.Sprintf("%d", creationHeight),
	)
	return err
}

// StakingCreateValidator creates a new validator.
func (tn *ChainNode) StakingCreateValidator(ctx context.Context, keyName, valFilePath string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "staking", "create-validator", valFilePath,
	)
	return err
}

// StakingDelegate delegates tokens to a validator.
func (tn *ChainNode) StakingDelegate(ctx context.Context, keyName, validatorAddr, amount string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "staking", "delegate", validatorAddr, amount,
	)
	return err
}

// StakingUnbond unstakes tokens from a validator.
func (tn *ChainNode) StakingUnbond(ctx context.Context, keyName, validatorAddr, amount string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "staking", "unbond", validatorAddr, amount,
	)
	return err
}

// StakingEditValidator edits an existing validator.
func (tn *ChainNode) StakingEditValidator(ctx context.Context, keyName string, flags ...string) error {
	cmd := []string{"staking", "edit-validator"}
	cmd = append(cmd, flags...)

	_, err := tn.ExecTx(ctx,
		keyName, cmd...,
	)
	return err
}

// StakingRedelegate redelegates tokens from one validator to another.
func (tn *ChainNode) StakingRedelegate(ctx context.Context, keyName, srcValAddr, dstValAddr, amount string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "staking", "redelegate", srcValAddr, dstValAddr, amount,
	)
	return err
}

// StakingCreateValidatorFile creates a new validator file for use in `StakingCreateValidator`.
func (tn *ChainNode) StakingCreateValidatorFile(
	ctx context.Context, filePath string,
	pubKeyJSON, amount, moniker, identity, website, security, details, commissionRate, commissionMaxRate, commissionMaxChangeRate, minSelfDelegation string,
) error {
	j := fmt.Sprintf(`{
	"pubkey": %s,
	"amount": "%s",
	"moniker": "%s",
	"identity": "%s",
	"website": "%s",
	"security": "%s",
	"details": "%s",
	"commission-rate": "%s",
	"commission-max-rate": "%s",
	"commission-max-change-rate": "%s",
	"min-self-delegation": "%s"
}`, pubKeyJSON, amount, moniker, identity, website, security, details, commissionRate, commissionMaxRate, commissionMaxChangeRate, minSelfDelegation)

	return tn.WriteFile(ctx, []byte(j), filePath)
}

// StakingQueryDelegation returns a delegation.
func (c *CosmosChain) StakingQueryDelegation(ctx context.Context, valAddr string, delegator string) (*stakingtypes.DelegationResponse, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		Delegation(ctx, &stakingtypes.QueryDelegationRequest{DelegatorAddr: delegator, ValidatorAddr: valAddr})
	return res.DelegationResponse, err
}

// StakingQueryDelegations returns all delegations for a delegator.
func (c *CosmosChain) StakingQueryDelegations(ctx context.Context, delegator string) ([]stakingtypes.DelegationResponse, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegatorDelegations(ctx, &stakingtypes.QueryDelegatorDelegationsRequest{DelegatorAddr: delegator, Pagination: nil})
	return res.DelegationResponses, err
}

// StakingQueryDelegationsTo returns all delegations to a validator.
func (c *CosmosChain) StakingQueryDelegationsTo(ctx context.Context, validator string) ([]*stakingtypes.DelegationResponse, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		ValidatorDelegations(ctx, &stakingtypes.QueryValidatorDelegationsRequest{ValidatorAddr: validator})

	var delegations []*stakingtypes.DelegationResponse
	for _, d := range res.DelegationResponses {
		delegations = append(delegations, &d)
	}

	return delegations, err
}

// StakingQueryDelegatorValidator returns a validator for a delegator.
func (c *CosmosChain) StakingQueryDelegatorValidator(ctx context.Context, delegator string, validator string) (*stakingtypes.Validator, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegatorValidator(ctx, &stakingtypes.QueryDelegatorValidatorRequest{DelegatorAddr: delegator, ValidatorAddr: validator})
	return &res.Validator, err
}

// StakingQueryDelegatorValidators returns all validators for a delegator.
func (c *CosmosChain) StakingQueryDelegatorValidators(ctx context.Context, delegator string) ([]stakingtypes.Validator, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegatorValidators(ctx, &stakingtypes.QueryDelegatorValidatorsRequest{DelegatorAddr: delegator})
	return res.Validators, err
}

// StakingQueryHistoricalInfo returns the historical info at the given height.
func (c *CosmosChain) StakingQueryHistoricalInfo(ctx context.Context, height int64) (*stakingtypes.HistoricalInfo, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		HistoricalInfo(ctx, &stakingtypes.QueryHistoricalInfoRequest{Height: height})
	return res.Hist, err
}

// StakingQueryParams returns the staking parameters.
func (c *CosmosChain) StakingQueryParams(ctx context.Context) (*stakingtypes.Params, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		Params(ctx, &stakingtypes.QueryParamsRequest{})
	return &res.Params, err
}

// StakingQueryPool returns the current staking pool values.
func (c *CosmosChain) StakingQueryPool(ctx context.Context) (*stakingtypes.Pool, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		Pool(ctx, &stakingtypes.QueryPoolRequest{})
	return &res.Pool, err
}

// StakingQueryRedelegation returns a redelegation.
func (c *CosmosChain) StakingQueryRedelegation(ctx context.Context, delegator string, srcValAddr string, dstValAddr string) ([]stakingtypes.RedelegationResponse, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		Redelegations(ctx, &stakingtypes.QueryRedelegationsRequest{DelegatorAddr: delegator, SrcValidatorAddr: srcValAddr, DstValidatorAddr: dstValAddr})
	return res.RedelegationResponses, err
}

// StakingQueryUnbondingDelegation returns an unbonding delegation.
func (c *CosmosChain) StakingQueryUnbondingDelegation(ctx context.Context, delegator string, validator string) (*stakingtypes.UnbondingDelegation, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		UnbondingDelegation(ctx, &stakingtypes.QueryUnbondingDelegationRequest{DelegatorAddr: delegator, ValidatorAddr: validator})
	return &res.Unbond, err
}

// StakingQueryUnbondingDelegations returns all unbonding delegations for a delegator.
func (c *CosmosChain) StakingQueryUnbondingDelegations(ctx context.Context, delegator string) ([]stakingtypes.UnbondingDelegation, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		DelegatorUnbondingDelegations(ctx, &stakingtypes.QueryDelegatorUnbondingDelegationsRequest{DelegatorAddr: delegator})
	return res.UnbondingResponses, err
}

// StakingQueryUnbondingDelegationsFrom returns all unbonding delegations from a validator.
func (c *CosmosChain) StakingQueryUnbondingDelegationsFrom(ctx context.Context, validator string) ([]stakingtypes.UnbondingDelegation, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		ValidatorUnbondingDelegations(ctx, &stakingtypes.QueryValidatorUnbondingDelegationsRequest{ValidatorAddr: validator})
	return res.UnbondingResponses, err
}

// StakingQueryValidator returns a validator.
func (c *CosmosChain) StakingQueryValidator(ctx context.Context, validator string) (*stakingtypes.Validator, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		Validator(ctx, &stakingtypes.QueryValidatorRequest{ValidatorAddr: validator})
	return &res.Validator, err
}

// StakingQueryValidators returns all validators.
func (c *CosmosChain) StakingQueryValidators(ctx context.Context, status string) ([]stakingtypes.Validator, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).Validators(ctx, &stakingtypes.QueryValidatorsRequest{
		Status: status,
	})
	return res.Validators, err
}
