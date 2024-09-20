package consensus

import (
	"context"
	"fmt"
	"net/http"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"google.golang.org/grpc"
)

type Client interface {
	Name() string
	StartFlags() string
	IsSynced(ctx context.Context) error

	Status(ctx context.Context) (*ctypes.ResultStatus, error)
	BlockResults(ctx context.Context, height *int64) (*ctypes.ResultBlockResults, error)
	Block(ctx context.Context, height *int64) (*ctypes.ResultBlock, error)
	Height(ctx context.Context) (int64, error)

	GrpcClient() *grpc.ClientConn
}

func NewClientFactory(remote string, client *http.Client, grpcConn *grpc.ClientConn) Client {
	cbftClient, err := NewCometBFTClient(remote, client, grpcConn)
	if err != nil {
		panic(err)
	}

	if cbftClient != nil {
		fmt.Println("Using CometBFT client") // TODO: logger
		return cbftClient
	}

	panic("NewClientFactory: No client available")
}
