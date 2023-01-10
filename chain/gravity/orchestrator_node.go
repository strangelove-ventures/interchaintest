package gravity

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/ethereum/go-ethereum/common/hexutil"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/ory/dockertest/docker"
	"github.com/ory/dockertest/docker/types"
	"github.com/strangelove-ventures/ibctest/v6/dockerutil"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"go.uber.org/zap"
)

type OrchestratorNode struct {
	VolumeName   string
	Index        int
	Chain        *GravityChain
	NetworkID    string
	DockerClient *dockerclient.Client
	Client       rpcclient.Client
	TestName     string
	Image        ibc.DockerImage

	lock sync.Mutex
	log  *zap.Logger

	containerID string

	// evm signing info
	mnemonic string

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

type ethereumKey struct {
	publicKey  string
	privateKey string
	address    string
}

const DerivationPath = "m/44'/60'/0'/0/0"

func ethereumKeyFromMnemonic(mnemonic string) (*ethereumKey, error) {
	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		return nil, err
	}

	derivationPath, err := hdwallet.ParseDerivationPath(DerivationPath)
	if err != nil {
		return nil, err
	}

	account, err := wallet.Derive(derivationPath, false)
	if err != nil {
		return nil, err
	}

	privateKeyBytes, err := wallet.PrivateKeyBytes(account)
	if err != nil {
		return nil, err
	}

	publicKeyBytes, err := wallet.PublicKeyBytes(account)
	if err != nil {
		return nil, err
	}

	return &ethereumKey{
		privateKey: hexutil.Encode(privateKeyBytes),
		publicKey:  hexutil.Encode(publicKeyBytes),
		address:    account.Address.String(),
	}, nil
}
func (on *OrchestratorNode) CreateContainer(ctx context.Context) error {
	on.logger().Info("Creating Container",
		zap.String("container", on.Name()),
		zap.String("image", on.Image.Ref()),
	)
	ethereumKey, err := ethereumKeyFromMnemonic(on.mnemonic)
	if err != nil {
		return err
	}

	chainIDStrings := make([]string, len(on.Chain.evmChains))
	for i, c := range on.Chain.evmChains {
		chainIDStrings[i] = strconv.Itoa(int(c.ID))
	}

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
				fmt.Sprintf("CHAIN_IDS=%s", strings.Join(chainIDStrings, ";")),
				fmt.Sprintf("ORCH_MNEMONIC=%s", on.mnemonic),
				fmt.Sprintf("ETH_PRIV_KEY=%s", ethereumKey.privateKey),
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
	if err := dockerutil.StartContainer(ctx, on.DockerClient, on.containerID); err != nil {
		return err
	}

	rc, err := on.DockerClient.ContainerLogs(ctx, on.containerID, dockertypes.ContainerLogsOptions(types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	}))
	if err != nil {
		return err
	}

	return retry.Do(func() error {
		on.logger().Info("waiting for orchestrator to be healthy:", zap.String("container", on.Name()))

		var logsBuf []byte
		if _, err := rc.Read(logsBuf); err != nil {
			return err
		}

		if strings.Contains(string(logsBuf), "No unsigned batches! Everything good!") {
			return nil
		}
		return fmt.Errorf("container still starting, container %s", on.Name())
	}, retry.Context(ctx), retry.Attempts(40), retry.Delay(3*time.Second), retry.DelayType(retry.FixedDelay))
}

func (on *OrchestratorNode) StopContainer(ctx context.Context) error {
	timeout := 30 * time.Second
	return on.DockerClient.ContainerStop(ctx, on.containerID, &timeout)
}

func (on *OrchestratorNode) RemoveContainer(ctx context.Context) error {
	err := on.DockerClient.ContainerRemove(ctx, on.containerID, dockertypes.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("remove container %s: %w", on.Name(), err)
	}
	return nil
}
