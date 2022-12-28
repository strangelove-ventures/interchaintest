package gravity

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/ory/dockertest/docker"
	"github.com/strangelove-ventures/ibctest/v6/dockerutil"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"go.uber.org/zap"
	"path"
	"strings"
	"sync"
)

type OrchestratorNode struct {
	VolumeName   string
	Index        int
	Chain        ibc.Chain
	NetworkID    string
	DockerClient *dockerclient.Client
	Client       rpcclient.Client
	TestName     string
	Image        ibc.DockerImage

	lock sync.Mutex
	log  *zap.Logger

	containerID string

	// Ports set during StartContainer.
	hostRPCPort  string
	hostGRPCPort string
}

type OrchestratorNodes []*OrchestratorNode

func (on *OrchestratorNode) Name() string {
	return fmt.Sprintf("%s-orch-%d-%s", on.Chain.Config().ChainID, on.Index, dockerutil.SanitizeContainerName(on.TestName))
}

func (on *OrchestratorNode) HostName() string {
	return dockerutil.CondenseHostName(on.Name())
}

func (on *OrchestratorNode) logger() *zap.Logger {
	return on.log.With(
		zap.String("chain_id", on.Chain.Config().ChainID),
		zap.String("test", on.TestName),
	)
}

func (on *OrchestratorNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", on.VolumeName, on.HomeDir())}
}

func (on *OrchestratorNode) HomeDir() string {
	return path.Join("/var/cosmos-chain", on.Chain.Config().Name)
}

func (on *OrchestratorNode) CreateContainer(ctx context.Context) error {
	on.logger().Info("Creating Container",
		zap.String("container", on.Name()),
		zap.String("image", on.Image.Ref()),
	)
	cc, err := on.DockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: on.Image.Ref(),

			Entrypoint: []string{
				"sh",
				"-c",
				"chmod +x /root/gorc/gorc_bootstrap.sh && /root/gorc/gorc_bootstrap.sh",
			},
			Hostname: on.HostName(),
			Env: []string{
				fmt.Sprintf("CHAIN_IDS=%s", strings.Join(chainIDStrings(), ";")),
				fmt.Sprintf("ORCH_MNEMONIC=%s", orch.mnemonic),
				fmt.Sprintf("ETH_PRIV_KEY=%s", val.ethereumKey.privateKey),
				"RUST_BACKTRACE=full",
				"RUST_LOG=debug",
			},

			Labels: map[string]string{dockerutil.CleanupLabel: on.TestName},
		},
		&container.HostConfig{
			Binds:           on.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
			DNS:             []string{},
			RestartPolicy:   container.RestartPolicy(docker.RestartPolicy{Name: "no"}),
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				on.NetworkID: {},
			},
		},
		nil,
		on.Name(),
	)
	if err != nil {
		return err
	}
	on.containerID = cc.ID
	return nil
}

func (on *OrchestratorNode) StartContainer(ctx context.Context) error {
	panic("unimplemented")
}
