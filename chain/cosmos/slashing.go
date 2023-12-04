package cosmos

import (
	"context"

	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// SlashingUnJail unjails a validator.
func (tn *ChainNode) SlashingUnJail(ctx context.Context, keyName string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "slashing", "unjail",
	)
	return err
}

// SlashingGetParams returns slashing params
func (c *CosmosChain) SlashingGetParams(ctx context.Context) (*slashingtypes.Params, error) {
	grpcConn, err := grpc.Dial(
		c.GetNode().hostGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	defer grpcConn.Close()

	res, err := slashingtypes.NewQueryClient(grpcConn).
		Params(ctx, &slashingtypes.QueryParamsRequest{})
	return &res.Params, err
}

// SlashingSigningInfo returns signing info for a validator
func (c *CosmosChain) SlashingSigningInfo(ctx context.Context, consAddress string) (*slashingtypes.ValidatorSigningInfo, error) {
	grpcConn, err := grpc.Dial(
		c.GetNode().hostGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	defer grpcConn.Close()

	res, err := slashingtypes.NewQueryClient(grpcConn).
		SigningInfo(ctx, &slashingtypes.QuerySigningInfoRequest{ConsAddress: consAddress})
	return &res.ValSigningInfo, err
}

// SlashingSigningInfos returns all signing infos
func (c *CosmosChain) SlashingSigningInfos(ctx context.Context) ([]slashingtypes.ValidatorSigningInfo, error) {
	grpcConn, err := grpc.Dial(
		c.GetNode().hostGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	defer grpcConn.Close()

	res, err := slashingtypes.NewQueryClient(grpcConn).
		SigningInfos(ctx, &slashingtypes.QuerySigningInfosRequest{})
	return res.Info, err
}
