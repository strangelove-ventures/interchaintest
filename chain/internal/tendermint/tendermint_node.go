package tendermint

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	libclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	volumetypes "github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-version"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"go.uber.org/zap"
)

// TendermintNode represents a node in the test network that is being created
type TendermintNode struct {
	Log *zap.Logger

	VolumeName   string
	Index        int
	Chain        ibc.Chain
	NetworkID    string
	DockerClient *dockerclient.Client
	Client       rpcclient.Client
	TestName     string
	Image        ibc.DockerImage

	containerLifecycle *dockerutil.ContainerLifecycle
}

func NewTendermintNode(
	ctx context.Context,
	log *zap.Logger,
	i int,
	c ibc.Chain,
	dockerClient *dockerclient.Client,
	networkID string,
	testName string,
	image ibc.DockerImage,
) (*TendermintNode, error) {
	tn := &TendermintNode{Log: log, Index: i, Chain: c,
		DockerClient: dockerClient, NetworkID: networkID, TestName: testName, Image: image}

	tn.containerLifecycle = dockerutil.NewContainerLifecycle(log, dockerClient, tn.Name())

	tv, err := dockerClient.VolumeCreate(ctx, volumetypes.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel:   testName,
			dockerutil.NodeOwnerLabel: tn.Name(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating tendermint volume: %w", err)
	}
	tn.VolumeName = tv.Name
	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: log,

		Client: dockerClient,

		VolumeName: tn.VolumeName,
		ImageRef:   tn.Image.Ref(),
		TestName:   tn.TestName,
		UidGid:     tn.Image.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set tendermint volume owner: %w", err)
	}

	return tn, nil
}

// TendermintNodes is a collection of TendermintNode
type TendermintNodes []*TendermintNode

const (
	// BlockTimeSeconds (in seconds) is approx time to create a block
	BlockTimeSeconds = 2

	p2pPort     = "26656/tcp"
	rpcPort     = "26657/tcp"
	grpcPort    = "9090/tcp"
	apiPort     = "1317/tcp"
	privValPort = "1234/tcp"
)

var (
	sentryPorts = nat.PortMap{
		nat.Port(p2pPort):     {},
		nat.Port(rpcPort):     {},
		nat.Port(grpcPort):    {},
		nat.Port(apiPort):     {},
		nat.Port(privValPort): {},
	}
)

// NewClient creates and assigns a new Tendermint RPC client to the TendermintNode
func (tn *TendermintNode) NewClient(addr string) error {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return err
	}

	httpClient.Timeout = 10 * time.Second
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return err
	}

	tn.Client = rpcClient
	return nil
}

// Name is the hostname of the test node container
func (tn *TendermintNode) Name() string {
	return fmt.Sprintf("node-%d-%s-%s", tn.Index, tn.Chain.Config().ChainID, dockerutil.SanitizeContainerName(tn.TestName))
}

func (tn *TendermintNode) HostName() string {
	return dockerutil.CondenseHostName(tn.Name())
}

func (tn *TendermintNode) GenesisFileContent(ctx context.Context) ([]byte, error) {
	gen, err := tn.ReadFile(ctx, "config/genesis.json")
	if err != nil {
		return nil, fmt.Errorf("getting genesis.json content: %w", err)
	}

	return gen, nil
}

// ReadFile reads the contents of a single file at the specified path in the docker filesystem.
// relPath describes the location of the file in the docker volume relative to the home directory.
func (tn *TendermintNode) ReadFile(ctx context.Context, relPath string) ([]byte, error) {
	fr := dockerutil.NewFileRetriever(tn.logger(), tn.DockerClient, tn.TestName)
	gen, err := fr.SingleFileContent(ctx, tn.VolumeName, relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file at %s: %w", relPath, err)
	}
	return gen, nil
}

func (tn *TendermintNode) OverwriteGenesisFile(ctx context.Context, content []byte) error {
	fw := dockerutil.NewFileWriter(tn.logger(), tn.DockerClient, tn.TestName)
	if err := fw.WriteFile(ctx, tn.VolumeName, "config/genesis.json", content); err != nil {
		return fmt.Errorf("overwriting genesis.json: %w", err)
	}

	return nil
}

type PrivValidatorKey struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type PrivValidatorKeyFile struct {
	Address string           `json:"address"`
	PubKey  PrivValidatorKey `json:"pub_key"`
	PrivKey PrivValidatorKey `json:"priv_key"`
}

// Bind returns the home folder bind point for running the node
func (tn *TendermintNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", tn.VolumeName, tn.HomeDir())}
}

func (tn *TendermintNode) HomeDir() string {
	return path.Join("/var/tendermint", tn.Chain.Config().Name)
}

// SetConfigAndPeers modifies the config for a validator node to start a chain
func (tn *TendermintNode) SetConfigAndPeers(ctx context.Context, peers string) error {
	c := make(testutil.Toml)

	sep, err := tn.GetConfigSeparator()
	if err != nil {
		return err
	}

	// Set Log Level to info
	c[fmt.Sprintf("log%slevel", sep)] = "info"

	p2p := make(testutil.Toml)

	// Allow p2p strangeness
	p2p[fmt.Sprintf("allow%sduplicate%sip", sep, sep)] = true
	p2p[fmt.Sprintf("addr%sbook%sstrict", sep, sep)] = false
	p2p[fmt.Sprintf("persistent%speers", sep)] = peers

	c["p2p"] = p2p

	consensus := make(testutil.Toml)

	blockT := (time.Duration(BlockTimeSeconds) * time.Second).String()
	consensus[fmt.Sprintf("timeout%scommit", sep)] = blockT
	consensus[fmt.Sprintf("timeout%spropose", sep)] = blockT

	c["consensus"] = consensus

	rpc := make(testutil.Toml)

	// Enable public RPC
	rpc["laddr"] = "tcp://0.0.0.0:26657"
	if tn.Chain.Config().UsesCometMock() {
		rpc["laddr"] = "tcp://0.0.0.0:22331"
	}

	c["rpc"] = rpc

	return testutil.ModifyTomlConfigFile(
		ctx,
		tn.logger(),
		tn.DockerClient,
		tn.TestName,
		tn.VolumeName,
		"config/config.toml",
		c,
	)
}

// Tenderment deprecate snake_case in config for hyphen-case in v0.34.1
// https://github.com/cometbft/cometbft/blob/main/CHANGELOG.md#v0341
func (tn *TendermintNode) GetConfigSeparator() (string, error) {
	var sep = "_"

	currentTnVersion, err := version.NewVersion(tn.Image.Version[1:])
	if err != nil {
		return "", err
	}
	tnVersion34_1, err := version.NewVersion("0.34.1")
	if err != nil {
		return "", err
	}
	// if currentVersion >= 0.34.1
	if tnVersion34_1.GreaterThanOrEqual(currentTnVersion) {
		sep = "-"
	}
	return sep, nil
}

func (tn *TendermintNode) Height(ctx context.Context) (int64, error) {
	stat, err := tn.Client.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("tendermint client status: %w", err)
	}
	return stat.SyncInfo.LatestBlockHeight, nil
}

// InitHomeFolder initializes a home folder for the given node
func (tn *TendermintNode) InitHomeFolder(ctx context.Context, mode string) error {
	command := []string{tn.Chain.Config().Bin, "init", mode,
		"--home", tn.HomeDir(),
	}
	_, _, err := tn.Exec(ctx, command, tn.Chain.Config().Env)
	return err
}

func (tn *TendermintNode) CreateNodeContainer(ctx context.Context, additionalFlags ...string) error {
	chainCfg := tn.Chain.Config()
	cmd := []string{chainCfg.Bin, "start", "--home", tn.HomeDir()}
	cmd = append(cmd, additionalFlags...)

	return tn.containerLifecycle.CreateContainer(ctx, tn.TestName, tn.NetworkID, tn.Image, sentryPorts, tn.Bind(), nil, tn.HostName(), cmd, nil, []string{})
}

func (tn *TendermintNode) StopContainer(ctx context.Context) error {
	return tn.containerLifecycle.StopContainer(ctx)
}

func (tn *TendermintNode) StartContainer(ctx context.Context) error {
	if err := tn.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	hostPorts, err := tn.containerLifecycle.GetHostPorts(ctx, rpcPort)
	if err != nil {
		return err
	}
	rpcPort := hostPorts[0]

	err = tn.NewClient(fmt.Sprintf("tcp://%s", rpcPort))
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	return retry.Do(func() error {
		stat, err := tn.Client.Status(ctx)
		if err != nil {
			// tn.t.Log(err)
			return err
		}
		// TODO: re-enable this check, having trouble with it for some reason
		if stat != nil && stat.SyncInfo.CatchingUp {
			return fmt.Errorf("still catching up: height(%d) catching-up(%t)",
				stat.SyncInfo.LatestBlockHeight, stat.SyncInfo.CatchingUp)
		}
		return nil
	}, retry.Context(ctx), retry.DelayType(retry.BackOffDelay))
}

// InitValidatorFiles creates the node files and signs a genesis transaction
func (tn *TendermintNode) InitValidatorFiles(ctx context.Context) error {
	return tn.InitHomeFolder(ctx, "validator")
}

func (tn *TendermintNode) InitFullNodeFiles(ctx context.Context) error {
	return tn.InitHomeFolder(ctx, "full")
}

// NodeID returns the node of a given node
func (tn *TendermintNode) NodeID(ctx context.Context) (string, error) {
	// This used to call p2p.LoadNodeKey against the file on the host,
	// but because we are transitioning to operating on Docker volumes,
	// we only have to tmjson.Unmarshal the raw content.
	j, err := tn.ReadFile(ctx, "config/node_key.json")
	if err != nil {
		return "", fmt.Errorf("getting genesis.json content: %w", err)
	}

	var nk p2p.NodeKey
	if err := tmjson.Unmarshal(j, &nk); err != nil {
		return "", fmt.Errorf("unmarshaling node_key.json: %w", err)
	}

	return string(nk.ID()), nil
}

// PeerString returns the string for connecting the nodes passed in
func (tn TendermintNodes) PeerString(ctx context.Context, node *TendermintNode) string {
	addrs := make([]string, len(tn))
	for i, n := range tn {
		if n == node {
			// don't peer with ourself
			continue
		}
		id, err := n.NodeID(ctx)
		if err != nil {
			// TODO: would this be better to panic?
			// When would NodeId return an error?
			break
		}
		hostName := n.HostName()
		ps := fmt.Sprintf("%s@%s:26656", id, hostName)
		fmt.Printf("{%s} peering (%s)\n", hostName, ps)
		addrs[i] = ps
	}
	return strings.Join(addrs, ",")
}

// LogGenesisHashes logs the genesis hashes for the various nodes
func (tn TendermintNodes) LogGenesisHashes(ctx context.Context) error {
	for _, n := range tn {
		gen, err := n.GenesisFileContent(ctx)
		if err != nil {
			return err
		}
		n.logger().Info("Genesis", zap.String("hash", fmt.Sprintf("%X", sha256.Sum256(gen))))
	}
	return nil
}

func (tn *TendermintNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(tn.Log, tn.DockerClient, tn.NetworkID, tn.TestName, tn.Image.Repository, tn.Image.Version)
	opts := dockerutil.ContainerOptions{
		Env:   env,
		Binds: tn.Bind(),
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (tn *TendermintNode) logger() *zap.Logger {
	return tn.Log.With(
		zap.String("chain_id", tn.Chain.Config().ChainID),
		zap.String("test", tn.TestName),
	)
}
