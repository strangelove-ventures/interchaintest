package penumbra

import (
	"context"
	"fmt"
	"strings"
	"sync"

	volumetypes "github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v7/chain/internal/tendermint"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
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
	tn := tendermint.NewTendermintNode(c.log, i, c, dockerClient, networkID, testName, tendermintImage)

	tv, err := dockerClient.VolumeCreate(ctx, volumetypes.VolumeCreateBody{
		Labels: map[string]string{
			dockerutil.CleanupLabel: testName,

			dockerutil.NodeOwnerLabel: tn.Name(),
		},
	})
	if err != nil {
		return PenumbraNode{}, fmt.Errorf("creating tendermint volume: %w", err)
	}
	tn.VolumeName = tv.Name
	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: c.log,

		Client: dockerClient,

		VolumeName: tn.VolumeName,
		ImageRef:   tn.Image.Ref(),
		TestName:   tn.TestName,
		UidGid:     tn.Image.UidGid,
	}); err != nil {
		return PenumbraNode{}, fmt.Errorf("set tendermint volume owner: %w", err)
	}

	pn := NewPenumbraAppNode(c.log, c, i, testName, dockerClient, networkID, penumbraImage)

	pv, err := dockerClient.VolumeCreate(ctx, volumetypes.VolumeCreateBody{
		Labels: map[string]string{
			dockerutil.CleanupLabel: testName,

			dockerutil.NodeOwnerLabel: pn.Name(),
		},
	})
	if err != nil {
		return PenumbraNode{}, fmt.Errorf("creating penumbra volume: %w", err)
	}
	pn.VolumeName = pv.Name
	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: c.log,

		Client: dockerClient,

		VolumeName: pn.VolumeName,
		ImageRef:   pn.Image.Ref(),
		TestName:   pn.TestName,
		UidGid:     tn.Image.UidGid,
	}); err != nil {
		return PenumbraNode{}, fmt.Errorf("set penumbra volume owner: %w", err)
	}

	return PenumbraNode{
		TendermintNode:      tn,
		PenumbraAppNode:     pn,
		PenumbraClientNodes: make(map[string]*PenumbraClientNode),
		clientsMu:           &sync.Mutex{},
	}, nil
}

func (p *PenumbraNode) CreateClientNode(ctx context.Context, keyName string, spendKey string, fullViewingKey []byte) error {
	p.clientsMu.Lock()
	clientNode := NewClientNode(p.PenumbraAppNode.log, p.PenumbraAppNode.Chain, len(p.PenumbraClientNodes), p.PenumbraAppNode.TestName, p.PenumbraAppNode.Image)
	p.PenumbraClientNodes[keyName] = clientNode
	p.clientsMu.Unlock()

	if err := clientNode.Initialize(ctx, spendKey, fullViewingKey); err != nil {
		return err
	}

	if err := clientNode.CreateNodeContainer(
		ctx,
		p.PenumbraAppNode.HostName()+":"+strings.Split(grpcPort, "/")[0],
		p.TendermintNode.HostName()+":"+strings.Split(rpcPort, "/")[0],
	); err != nil {
		return err
	}

	if err := clientNode.StartContainer(ctx); err != nil {
		return err
	}

	return nil
}
