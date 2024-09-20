package consensus

import (
	"context"
	"fmt"
	"net/http"

	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"google.golang.org/grpc"
)

var _ Client = (*CometBFTClient)(nil)

type CometBFTClient struct {
	Client   rpcclient.Client
	GrpcConn *grpc.ClientConn
}

// NewCometBFTClient creates a new CometBFTClient.
func NewCometBFTClient(remote string, client *http.Client, grpcConn *grpc.ClientConn) (*CometBFTClient, error) {
	rpcClient, err := rpchttp.NewWithClient(remote, "/websocket", client)
	if err != nil {
		return nil, fmt.Errorf("failed to create CometBFT client: %w", err)
	}

	if rpcClient == nil {
		return nil, fmt.Errorf("failed to create CometBFT client: rpc client is nil")
	}

	return &CometBFTClient{
		Client:   rpcClient,
		GrpcConn: grpcConn,
	}, nil
}

// IsSynced implements Client.
func (c *CometBFTClient) IsSynced(ctx context.Context) error {
	stat, err := c.Client.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if stat != nil && stat.SyncInfo.CatchingUp {
		return fmt.Errorf("still catching up: height(%d) catching-up(%t)", stat.SyncInfo.LatestBlockHeight, stat.SyncInfo.CatchingUp)
	}

	return nil
}

// StartupFlags implements Client.
func (c *CometBFTClient) StartFlags() string {
	return "--x-crisis-skip-assert-invariants"
}

// Height implements Client.
func (c *CometBFTClient) Height(ctx context.Context) (int64, error) {
	s, err := c.Client.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("tendermint rpc client status: %w", err)
	}

	return s.SyncInfo.LatestBlockHeight, nil
}

// Name implements Client.
func (c *CometBFTClient) Name() string {
	return "cometbft"
}

// GrpcClient implements Client.
func (c *CometBFTClient) GrpcClient() *grpc.ClientConn {
	return c.GrpcConn
}

// Block implements Client.
func (c *CometBFTClient) Block(ctx context.Context, height *int64) (*ctypes.ResultBlock, error) {
	return c.Client.Block(ctx, height)
}

// BlockResults implements Client.
func (c *CometBFTClient) BlockResults(ctx context.Context, height *int64) (*ctypes.ResultBlockResults, error) {
	return c.Client.BlockResults(ctx, height)
}

// Status implements Client.
func (c *CometBFTClient) Status(ctx context.Context) (*ctypes.ResultStatus, error) {
	return c.Client.Status(ctx)
}
