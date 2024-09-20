package consensus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos/cli"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
)

var _ Client = (*GordianClient)(nil)

type GordianClient struct {
	addr   string
	client *http.Client
}

func NewGordianClient(addr string, client *http.Client) *GordianClient {
	addr = strings.Replace(addr, "tcp://", "http://", 1)

	return &GordianClient{
		addr:   addr,
		client: client,
	}
}

// ClientType implements Client.
func (g *GordianClient) ClientType() ClientType {
	return Gordian
}

// Block implements Client.
func (g *GordianClient) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	return &coretypes.ResultBlock{}, nil
}

// BlockResults implements Client.
func (g *GordianClient) BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	return &coretypes.ResultBlockResults{}, nil
}

// Height implements Client.
func (g *GordianClient) Height(ctx context.Context) (int64, error) {
	type GordianCurrentBlockResponse struct {
		VotingHeight *uint64 `protobuf:"varint,1,opt,name=voting_height,json=votingHeight,proto3,oneof" json:"voting_height,omitempty"`
	}

	// TODO: get hostname query to work
	endpoint := fmt.Sprintf("%s/blocks/watermark", g.addr)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	q := req.URL.Query()

	// make request as JSON
	req.Header.Set("Content-Type", "application/json")
	req.URL.RawQuery = q.Encode()

	// client := &http.Client{}
	resp, err := g.client.Do(req)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var watermark GordianCurrentBlockResponse
	if err := json.NewDecoder(resp.Body).Decode(&watermark); err != nil {
		log.Print(err)
		os.Exit(1)
	}

	return int64(*watermark.VotingHeight), nil
}

// IsClient implements Client.
func (g *GordianClient) IsClient(ctx context.Context, img *dockerutil.Image, bin string) bool {
	res := img.Run(ctx, []string{bin, "gordian"}, dockerutil.ContainerOptions{})
	return cli.HasCommand(res.Err)
}

// IsSynced implements Client.
func (g *GordianClient) IsSynced(ctx context.Context) error {
	// TODO:
	h, err := g.Height(ctx)
	if err != nil {
		return fmt.Errorf("failed to get height: %w", err)
	}

	if h > 0 {
		return nil
	}

	return fmt.Errorf("height is 0")
}

// StartFlags implements Client.
func (g *GordianClient) StartFlags(context.Context) string {
	return ""
}

// Status implements Client.
func (g *GordianClient) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	return &coretypes.ResultStatus{}, nil
}
