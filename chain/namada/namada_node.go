package namada

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	libclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	volumetypes "github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"go.uber.org/zap"
)

type NamadaNode struct {
	Index        int
	Validator    bool
	TestName     string
	Chain        ibc.Chain
	DockerClient *dockerclient.Client
	Client       rpcclient.Client
	Image        ibc.DockerImage
	VolumeName   string
	NetworkID    string

	lock sync.Mutex
	log  *zap.Logger

	containerLifecycle *dockerutil.ContainerLifecycle

	// Ports set during StartContainer.
	hostP2PPort string
	hostRPCPort string
}

// Collection of NamadaNode
type NamadaNodes []*NamadaNode

const (
	p2pPort = "26656/tcp"
	rpcPort = "26657/tcp"
)

func NewNamadaNode(
	ctx context.Context,
	log *zap.Logger,
	chain *NamadaChain,
	index int,
	validator bool,
	testName string,
	dockerClient *dockerclient.Client,
	networkID string,
	image ibc.DockerImage,
) (*NamadaNode, error) {
	nn := &NamadaNode{
		Index:        index,
		Validator:    validator,
		TestName:     testName,
		Chain:        chain,
		DockerClient: dockerClient,
		Image:        image,
		NetworkID:    networkID,

		log: log.With(
			zap.Bool("validator", validator),
			zap.Int("i", index),
		),
	}

	nn.containerLifecycle = dockerutil.NewContainerLifecycle(log, dockerClient, nn.Name())

	nv, err := dockerClient.VolumeCreate(ctx, volumetypes.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel:   testName,
			dockerutil.NodeOwnerLabel: nn.Name(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating namada volume: %w", err)
	}

	nn.VolumeName = nv.Name
	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: log,

		Client: dockerClient,

		VolumeName: nn.VolumeName,
		ImageRef:   nn.Image.Ref(),
		TestName:   nn.TestName,
		UidGid:     image.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set namada volume owner: %w", err)
	}

	return nn, nil
}

func (n *NamadaNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(n.logger(), n.DockerClient, n.NetworkID, n.TestName, n.Image.Repository, n.Image.Version)
	opts := dockerutil.ContainerOptions{
		Env:   env,
		Binds: n.Bind(),
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (n *NamadaNode) logger() *zap.Logger {
	return n.log.With(
		zap.String("chain_id", n.Chain.Config().ChainID),
		zap.String("test", n.TestName),
	)
}

// Name of the test node container.
func (n *NamadaNode) Name() string {
	return fmt.Sprintf("%s-%s-%d-%s", n.Chain.Config().ChainID, n.NodeType(), n.Index, n.TestName)
}

func (n *NamadaNode) HostName() string {
	return dockerutil.CondenseHostName(n.Name())
}

func (n *NamadaNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", n.VolumeName, n.HomeDir())}
}

// Home directory in the Docker filesystem.
func (n *NamadaNode) HomeDir() string {
	return "/home/namada"
}

func (n *NamadaNode) NodeType() string {
	nodeType := "fn"
	if n.Validator {
		nodeType = "val"
	}
	return nodeType
}

func (n *NamadaNode) NewRpcClient(addr string) error {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return err
	}

	httpClient.Timeout = 10 * time.Second
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return err
	}

	n.Client = rpcClient
	return nil
}

func (n *NamadaNode) Height(ctx context.Context) (int64, error) {
	stat, err := n.Client.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("tendermint client status: %w", err)
	}
	return stat.SyncInfo.LatestBlockHeight, nil
}

func (n *NamadaNode) CreateContainer(ctx context.Context, hostBaseDir string) error {
	archiveFile := fmt.Sprintf("%s.tar.gz", n.Chain.Config().ChainID)
	archivePath := filepath.Join(hostBaseDir, archiveFile)
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		return err
	}
	fw := dockerutil.NewFileWriter(n.logger(), n.DockerClient, n.TestName)
	err = fw.WriteFile(ctx, n.VolumeName, archiveFile, archive)
	if err != nil {
		return err
	}

	if n.Validator {
		validatorAlias := fmt.Sprintf("validator-%d", n.Index)
		relPath := filepath.Join("pre-genesis", validatorAlias, "validator-wallet.toml")
		validatorWalletPath := filepath.Join(hostBaseDir, relPath)
		wallet, err := os.ReadFile(validatorWalletPath)
		if err != nil {
			return err
		}
		err = fw.WriteFile(ctx, n.VolumeName, relPath, wallet)
		if err != nil {
			return err
		}
	}

	setConfigDir := fmt.Sprintf("NAMADA_NETWORK_CONFIGS_DIR=%s", n.HomeDir())

	joinNetworkCmd := fmt.Sprintf(`%s namadac --base-dir %s utils join-network --add-persistent-peers --chain-id %s --allow-duplicate-ip`, setConfigDir, n.HomeDir(), n.Chain.Config().ChainID)
	if n.Validator {
		joinNetworkCmd += " --genesis-validator " + fmt.Sprintf("validator-%d", n.Index)
	}

	updateCmd := fmt.Sprintf(`sed -i s/127.0.0.1:26657/0.0.0.0:26657/g %s/%s/config.toml`, n.HomeDir(), n.Chain.Config().ChainID)

	ledgerCmd := fmt.Sprintf(`namadan --base-dir %s ledger`, n.HomeDir())

	cmd := []string{
		"sh",
		"-c",
		fmt.Sprintf(`%s && %s && %s`, joinNetworkCmd, updateCmd, ledgerCmd),
	}

	exposedPorts := nat.PortMap{
		nat.Port(p2pPort): {},
		nat.Port(rpcPort): {},
	}

	ipAddr := strings.Split(n.netAddress(), ":")[0]
	return n.containerLifecycle.CreateContainer(ctx, n.TestName, n.NetworkID, n.Image, exposedPorts, ipAddr, n.Bind(), nil, n.HostName(), cmd, n.Chain.Config().Env, []string{})
}

func (n *NamadaNode) StartContainer(ctx context.Context) error {
	if err := n.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	hostPorts, err := n.containerLifecycle.GetHostPorts(ctx, p2pPort, rpcPort)
	if err != nil {
		return err
	}
	rpcPort := hostPorts[1]
	err = n.NewRpcClient(fmt.Sprintf("tcp://%s", rpcPort))
	if err != nil {
		return err
	}

	n.hostP2PPort, n.hostRPCPort = hostPorts[0], hostPorts[1]

	time.Sleep(5 * time.Second)
	return retry.Do(func() error {
		stat, err := n.Client.Status(ctx)
		if err != nil {
			return err
		}
		if stat != nil && stat.SyncInfo.CatchingUp {
			return fmt.Errorf("still catching up: height(%d) catching-up(%t)",
				stat.SyncInfo.LatestBlockHeight, stat.SyncInfo.CatchingUp)
		}
		return nil
	}, retry.Context(ctx), retry.DelayType(retry.BackOffDelay))
}

func (n *NamadaNode) CheckMaspFiles(ctx context.Context) error {
	maspDir := ".masp-params"
	requiredFiles := []string{
		"masp-spend.params",
		"masp-output.params",
		"masp-convert.params",
	}

	fr := dockerutil.NewFileRetriever(n.logger(), n.DockerClient, n.TestName)
	for _, file := range requiredFiles {
		relPath := filepath.Join(maspDir, file)
		if _, err := fr.SingleFileContent(ctx, n.VolumeName, relPath); err != nil {
			return err
		}
	}

	return nil
}

func (n *NamadaNode) netAddress() string {
  var index int
  if n.Validator {
    index = n.Index + 2
  } else {
    index = n.Index + 128
  }
	return fmt.Sprintf("172.18.0.%d:%s", index, strings.Split(p2pPort, "/")[0])
}
