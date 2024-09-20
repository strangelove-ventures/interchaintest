package cosmos

import (
	"context"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// StakingDelegate delegates tokens to a validator.
func (tn *ChainNode) StakingDelegate(ctx context.Context, keyName, validatorAddr, amount string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "staking", "delegate", validatorAddr, amount,
	)
	return err
}

// StakingQueryDelegationsTo returns all delegations to a validator.
func (c *CosmosChain) StakingQueryDelegationsTo(ctx context.Context, validator string) ([]*stakingtypes.DelegationResponse, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).
		ValidatorDelegations(ctx, &stakingtypes.QueryValidatorDelegationsRequest{ValidatorAddr: validator})

	var delegations []*stakingtypes.DelegationResponse
	for i := range res.DelegationResponses {
		delegations = append(delegations, &res.DelegationResponses[i])
	}

	return delegations, err
}

// StakingQueryValidators returns all validators.
func (c *CosmosChain) StakingQueryValidators(ctx context.Context, status string) ([]stakingtypes.Validator, error) {
	res, err := stakingtypes.NewQueryClient(c.GetNode().GrpcConn).Validators(ctx, &stakingtypes.QueryValidatorsRequest{
		Status: status,
	})
	return res.Validators, err
}
