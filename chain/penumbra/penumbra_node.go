package penumbra

import (
	"context"
	"strings"
	"sync"

	dockerclient "github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v7/chain/internal/tendermint"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"go.uber.org/zap"
)

type PenumbraNode struct {
	TendermintNode      *tendermint.TendermintNode
	PenumbraAppNode     *PenumbraAppNode
	PenumbraClientNodes map[string]*PenumbraClientNode
	clientsMu           sync.Locker
}

type PenumbraNodes []PenumbraNode

// NewChainNode returns a penumbra chain node with tendermint and penumbra nodes
// with docker volumes created.
func NewPenumbraNode(
	ctx context.Context,
	i int,
	c *PenumbraChain,
	dockerClient *dockerclient.Client,
	networkID string,
	testName string,
	tendermintImage ibc.DockerImage,
	penumbraImage ibc.DockerImage,
) (PenumbraNode, error) {
	tn, err := tendermint.NewTendermintNode(ctx, c.log, i, c, dockerClient, networkID, testName, tendermintImage)
	if err != nil {
		return PenumbraNode{}, err
	}

	pn, err := NewPenumbraAppNode(ctx, c.log, c, i, testName, dockerClient, networkID, penumbraImage)
	if err != nil {
		return PenumbraNode{}, err
	}

	return PenumbraNode{
		TendermintNode:      tn,
		PenumbraAppNode:     pn,
		PenumbraClientNodes: make(map[string]*PenumbraClientNode),
		clientsMu:           &sync.Mutex{},
	}, nil
}

func (p *PenumbraNode) CreateClientNode(
	ctx context.Context,
	log *zap.Logger,
	dockerClient *dockerclient.Client,
	networkID string,
	image ibc.DockerImage,
	testName string,
	index int,
	keyName string,
	spendKey string,
	fullViewingKey string,
) error {
	p.clientsMu.Lock()
	clientNode, err := NewClientNode(
		ctx,
		log,
		p.PenumbraAppNode.Chain,
		keyName,
		index,
		testName,
		image,
		dockerClient,
		networkID,
	)
	if err != nil {
		p.clientsMu.Unlock()
		return err
	}
	p.PenumbraClientNodes[keyName] = clientNode
	p.clientsMu.Unlock()

	if err := clientNode.Initialize(ctx, spendKey, fullViewingKey); err != nil {
		return err
	}

	if err := clientNode.CreateNodeContainer(
		ctx,
		"tcp://"+p.PenumbraAppNode.HostName()+":"+strings.Split(grpcPort, "/")[0],
	); err != nil {
		return err
	}

	if err := clientNode.StartContainer(ctx); err != nil {
		return err
	}

	return nil
}
