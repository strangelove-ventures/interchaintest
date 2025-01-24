package penumbra

import (
	"context"
	"strings"
	"sync"

	dockerclient "github.com/docker/docker/client"
	"go.uber.org/zap"

	"github.com/strangelove-ventures/interchaintest/v9/chain/internal/tendermint"
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
)

// PenumbraNode reporesents a node in the Penumbra network which consists of one instance of Tendermint,
// an instance of pcli, and zero or more instances of pclientd.
type PenumbraNode struct {
	TendermintNode      *tendermint.TendermintNode
	PenumbraAppNode     *PenumbraAppNode
	PenumbraClientNodes map[string]*PenumbraClientNode
	clientsMu           sync.Locker
	addrString          string
}

// PenumbraNodes is a slice of pointers that point to instances of PenumbraNode in memory.
type PenumbraNodes []*PenumbraNode

// NewPenumbraNode returns a penumbra chain node with tendermint and penumbra nodes, along with docker volumes created.
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

// CreateClientNode initializes a new instance of pclientd, with the specified FullViewingKey and CustodyKey,
// before attempting to create and start pclientd in a new Docker container.
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
	addr, err := p.PenumbraAppNode.GetAddress(ctx, keyName)
	if err != nil {
		return err
	}

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
		addr,
		string(addr),
	)
	if err != nil {
		p.clientsMu.Unlock()
		return err
	}
	p.PenumbraClientNodes[keyName] = clientNode
	p.clientsMu.Unlock()

	pdAddr := "tcp://" + p.PenumbraAppNode.HostName() + ":" + strings.Split(grpcPort, "/")[0]
	if err := clientNode.Initialize(ctx, pdAddr, spendKey, fullViewingKey); err != nil {
		return err
	}

	if err := clientNode.CreateNodeContainer(ctx); err != nil {
		return err
	}

	if err := clientNode.StartContainer(ctx); err != nil {
		return err
	}

	return nil
}
