package cosmos

import (
	"context"

	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
)

// ConsensusQueryParams queries the chain consensus parameters via grpc.
func (c *CosmosChain) ConsensusQueryParams(ctx context.Context) (*consensustypes.QueryParamsResponse, error) {
	res, err := consensustypes.NewQueryClient(c.GetNode().GrpcConn).Params(ctx, &consensustypes.QueryParamsRequest{})
	return res, err
}
