package consensus

import (
	"context"
	"net/http"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"go.uber.org/zap"
)

// create an enum for the different client types (gordian and cometbft)
type ClientType string

const (
	Gordian  ClientType = "gordian"
	CometBFT ClientType = "cometbft"
)

type Client interface {
	ClientType() ClientType
	StartFlags(context.Context) string
	IsSynced(ctx context.Context) error
	IsClient(ctx context.Context, img *dockerutil.Image, bin string) bool

	Status(ctx context.Context) (*ctypes.ResultStatus, error)
	BlockResults(ctx context.Context, height *int64) (*ctypes.ResultBlockResults, error)
	Block(ctx context.Context, height *int64) (*ctypes.ResultBlock, error)
	Height(ctx context.Context) (int64, error)
}

// GetBlankClientByName returns a blank client so non state logic (like startup params) can be used.
func NewBlankClient(ctx context.Context, logger *zap.Logger, img *dockerutil.Image, bin string) Client {
	clients := []Client{
		&CometBFTClient{},
		&GordianClient{},
	}

	for _, client := range clients {
		if client.IsClient(ctx, img, bin) {
			logger.Info("NewBlankClient: Found client", zap.String("client", string(client.ClientType())))
			return client
		}
	}

	logger.Info("NewBlankClient: No client found. Defaulting to CometBFT")
	return &CometBFTClient{}
}

// consensus is gathered from `NewBlankClient` on startup of the node.
func NewClientFactory(consensus ClientType, remote string, client *http.Client) Client {
	switch consensus {
	case CometBFT:
		cbft, err := NewCometBFTClient(remote, client)
		if err != nil {
			panic(err)
		}

		if cbft != nil {
			return cbft
		}

		panic("NewClientFactory: No client available for " + CometBFT)
	case Gordian:
		gordian := NewGordianClient(remote, client)
		if gordian != nil {
			return gordian
		}

		panic("NewClientFactory: No client available for " + Gordian)

	default:
		panic("NewClientFactory: No client available")
	}
}
