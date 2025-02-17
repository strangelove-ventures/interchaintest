package namada

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types"
	dockerclient "github.com/moby/moby/client"

	// To use a legacy tendermint client.
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	"go.uber.org/zap"

	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
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

	log *zap.Logger

	containerLifecycle *dockerutil.ContainerLifecycle

	// Ports set during StartContainer.
	hostP2PPort string
	hostRPCPort string
}

// Collection of NamadaNode.
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
		UidGid:     image.UIDGID,
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

func (n *NamadaNode) NewRPCClient(addr string) error {
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

func (n *NamadaNode) CreateContainer(ctx context.Context) error {
	setConfigDir := fmt.Sprintf("NAMADA_NETWORK_CONFIGS_DIR=%s", n.HomeDir())

	joinNetworkCmd := fmt.Sprintf(`%s %s client --base-dir %s utils join-network --add-persistent-peers --chain-id %s --allow-duplicate-ip`, setConfigDir, n.Chain.Config().Bin, n.HomeDir(), n.Chain.Config().ChainID)
	if n.Validator {
		joinNetworkCmd += " --genesis-validator " + fmt.Sprintf("validator-%d", n.Index)
	}

	mvCmd := "echo 'starting a validator node'"
	if !n.Validator {
		baseDir := fmt.Sprintf("%s/%s", n.HomeDir(), n.Chain.Config().ChainID)
		mvCmd = fmt.Sprintf(`mv %s/wallet.toml %s && sed -i 's/127.0.0.1:26657/0.0.0.0:26657/g' %s/config.toml`, n.HomeDir(), baseDir, baseDir)
	}

	ledgerCmd := fmt.Sprintf(`%s node --base-dir %s ledger`, n.Chain.Config().Bin, n.HomeDir())

	cmd := []string{
		"sh",
		"-c",
		fmt.Sprintf(`%s && %s && %s`, joinNetworkCmd, mvCmd, ledgerCmd),
	}

	exposedPorts := nat.PortMap{
		nat.Port(p2pPort): {},
		nat.Port(rpcPort): {},
	}

	netAddr, err := n.netAddress(ctx)
	if err != nil {
		return err
	}
	ipAddr := strings.Split(netAddr, ":")[0]
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
	err = n.NewRPCClient(fmt.Sprintf("tcp://%s", rpcPort))
	if err != nil {
		return err
	}

	n.hostP2PPort, n.hostRPCPort = hostPorts[0], hostPorts[1]

	time.Sleep(5 * time.Second)
	err = n.WaitMaspFileDownload(ctx)
	if err != nil {
		return fmt.Errorf("failed to download MASP files: %v", err)
	}

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

func (n *NamadaNode) WaitMaspFileDownload(ctx context.Context) error {
	maspDir := ".masp-params"
	requiredFiles := []string{
		"masp-spend.params",
		"masp-output.params",
		"masp-convert.params",
	}

	fr := dockerutil.NewFileRetriever(n.logger(), n.DockerClient, n.TestName)
	for _, file := range requiredFiles {
		relPath := filepath.Join(maspDir, file)
		timeout := 5 * time.Minute
		timeoutChan := time.After(timeout)
		size := -1
		completed := false
		for !completed {
			select {
			case <-timeoutChan:
				return fmt.Errorf("downloading masp files isn't completed")
			default:
				f, err := fr.SingleFileContent(ctx, n.VolumeName, relPath)
				if err != nil {
					time.Sleep(2 * time.Second)
					continue
				}
				if size != len(f) {
					size = len(f)
					time.Sleep(2 * time.Second)
					continue
				}
				completed = true
			}
		}
	}

	return nil
}

func (n *NamadaNode) netAddress(ctx context.Context) (string, error) {
	var index int
	if n.Validator {
		index = n.Index + 128
	} else {
		index = n.Index + 192
	}
	networkResource, err := n.DockerClient.NetworkInspect(ctx, n.NetworkID, types.NetworkInspectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get the network resource: %v", err)
	}
	for _, config := range networkResource.IPAM.Config {
		if config.Subnet != "" {
			ip, ipNet, err := net.ParseCIDR(config.Subnet)
			if err != nil {
				return "", fmt.Errorf("failed to parse the subnet: %v", err)
			}

			ip = ip.To4()
			if ip == nil {
				return "", fmt.Errorf("subnet is not IPv4")
			}

			ones, bits := ipNet.Mask.Size()
			if index < 0 || index >= 1<<uint(bits-ones) {
				return "", fmt.Errorf("index is invalid: %d", index)
			}

			ip[3] += byte(index)
			return fmt.Sprintf("%s:%s", ip, strings.Split(p2pPort, "/")[0]), nil
		}
	}

	return "", fmt.Errorf("failed to get the net address")
}

func (n *NamadaNode) ReadFile(ctx context.Context, relPath string) ([]byte, error) {
	fr := dockerutil.NewFileRetriever(n.logger(), n.DockerClient, n.TestName)
	gen, err := fr.SingleFileContent(ctx, n.VolumeName, relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file at %s: %w", relPath, err)
	}
	return gen, nil
}

func (n *NamadaNode) writeFile(ctx context.Context, destPath string, file []byte) error {
	fw := dockerutil.NewFileWriter(n.logger(), n.DockerClient, n.TestName)
	return fw.WriteFile(ctx, n.VolumeName, destPath, file)
}
