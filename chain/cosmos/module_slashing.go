package cosmos

import (
	"context"

	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

// SlashingUnJail unjails a validator.
func (tn *ChainNode) SlashingUnJail(ctx context.Context, keyName string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "slashing", "unjail",
	)
	return err
}

// SlashingGetParams returns slashing params
func (c *CosmosChain) SlashingQueryParams(ctx context.Context) (*slashingtypes.Params, error) {
	res, err := slashingtypes.NewQueryClient(c.GetNode().GrpcConn).
		Params(ctx, &slashingtypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}
	return &res.Params, nil
}

// SlashingSigningInfo returns signing info for a validator
func (c *CosmosChain) SlashingQuerySigningInfo(ctx context.Context, consAddress string) (*slashingtypes.ValidatorSigningInfo, error) {
	res, err := slashingtypes.NewQueryClient(c.GetNode().GrpcConn).
		SigningInfo(ctx, &slashingtypes.QuerySigningInfoRequest{ConsAddress: consAddress})
	if err != nil {
		return nil, err
	}
	return &res.ValSigningInfo, nil
}

// SlashingSigningInfos returns all signing infos
func (c *CosmosChain) SlashingQuerySigningInfos(ctx context.Context) ([]slashingtypes.ValidatorSigningInfo, error) {
	res, err := slashingtypes.NewQueryClient(c.GetNode().GrpcConn).
		SigningInfos(ctx, &slashingtypes.QuerySigningInfosRequest{})
	if err != nil {
		return nil, err
	}
	return res.Info, nil
}
